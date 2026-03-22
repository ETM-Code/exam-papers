// Package examapi provides a client for the examinations.ie exam material archive.
//
// The website uses a multi-step server-rendered form with obfuscated URL parameters.
// Each dropdown selection POSTs accumulated form fields to reveal the next dropdown.
// File download links use the same encoding and 302-redirect to PDFs hosted at
// https://www.examinations.ie/archive/...
//
// The obfuscation: dot-separated numbers where the last number is a key.
// offset = key - 98; each preceding number + offset = ASCII code of the character.
package examapi

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	BaseURL    = "https://www.examinations.ie"
	ArchiveURL = BaseURL + "/exammaterialarchive/"
	PDFURL     = BaseURL + "/archive"
)

// Material types accepted by the form.
var MaterialTypes = map[string]string{
	"exampapers":              "Exam Papers",
	"markingschemes":          "Marking Schemes",
	"deferredexams":           "Deferred Exam Papers",
	"deferredmarkingschemes":  "Deferred Exam Marking Schemes",
}

// Examination codes.
var Examinations = map[string]string{
	"lc": "Leaving Certificate",
	"jc": "Junior Cycle",
	"lb": "Leaving Certificate Applied",
}

// Level codes embedded in file IDs.
var Levels = map[byte]string{
	'A': "Higher",
	'G': "Ordinary",
	'B': "Foundation",
	'C': "Common",
}

// Paper represents a single downloadable exam paper or marking scheme.
type Paper struct {
	Description  string `json:"description"`
	FileID       string `json:"file_id"`
	EncodedLink  string `json:"encoded_link"`
	Size         string `json:"size"`
	Year         string `json:"year"`
	Exam         string `json:"exam"`
	SubjectCode  string `json:"subject_code"`
	MaterialType string `json:"material_type"`
}

// DirectURL returns the direct PDF URL, bypassing the obfuscated link.
func (p Paper) DirectURL() string {
	return fmt.Sprintf("%s/%s/%s/%s", PDFURL, p.MaterialType, p.Year, p.FileID)
}

// Level extracts the level from the file ID (e.g. "Higher", "Ordinary").
func (p Paper) Level() string {
	re := regexp.MustCompile(`(?i)[A-Z]{2}\d{3}([A-Z])LP`)
	m := re.FindStringSubmatch(p.FileID)
	if len(m) > 1 {
		ch := strings.ToUpper(m[1])[0]
		if name, ok := Levels[ch]; ok {
			return name
		}
		return m[1]
	}
	return "Unknown"
}

// Language extracts the language from the file ID.
func (p Paper) Language() string {
	upper := strings.ToUpper(p.FileID)
	if strings.HasSuffix(upper, "EV.PDF") {
		return "English"
	}
	if strings.HasSuffix(upper, "IV.PDF") {
		return "Irish"
	}
	return "Unknown"
}

// Subject holds a subject code and display name.
type Subject struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// Client fetches exam papers from examinations.ie.
type Client struct {
	http      *http.Client
	formState url.Values
	// nextActions stores the onChange action values parsed from each HTML response.
	// The server generates these with random offsets and validates them on the next POST.
	nextActions map[string]string
}

// NewClient creates a new exam papers client.
func NewClient() *Client {
	return &Client{
		http:      &http.Client{},
		formState: url.Values{},
	}
}

func encodeParam(text string, offset int) string {
	key := 98 + offset
	parts := make([]string, 0, len(text)+1)
	for _, c := range text {
		parts = append(parts, strconv.Itoa(int(c)-offset))
	}
	parts = append(parts, strconv.Itoa(key))
	return strings.Join(parts, ".")
}

func decodeParam(encoded string) string {
	parts := strings.Split(encoded, ".")
	if len(parts) < 2 {
		return ""
	}
	key, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return ""
	}
	offset := key - 98
	var b strings.Builder
	for _, p := range parts[:len(parts)-1] {
		v, err := strconv.Atoi(p)
		if err != nil {
			continue
		}
		b.WriteByte(byte(v + offset))
	}
	return b.String()
}

var onChangeRe = regexp.MustCompile(`onChange=SubmitForm\("([^"]+)"\);[^>]*name="([^"]+)"`)
var onClickRe = regexp.MustCompile(`onClick=SubmitForm\("([^"]+)"\);[^>]*name="([^"]+)"`)

// parseActions extracts the SubmitForm action values from HTML onChange/onClick handlers.
// The server generates these with random offsets and validates them on the next POST.
func parseActions(html string) map[string]string {
	actions := map[string]string{}

	for _, re := range []*regexp.Regexp{onChangeRe, onClickRe} {
		for _, m := range re.FindAllStringSubmatch(html, -1) {
			encoded := m[1]
			name := m[2]
			decoded := decodeParam(encoded)
			actions[decoded] = encoded
			// Also map by field name for lookup
			actions[name] = encoded
		}
	}

	return actions
}

func (c *Client) reset() {
	c.formState = url.Values{}
	c.nextActions = map[string]string{}
}

func (c *Client) doWithRetry(req *http.Request) (*http.Response, error) {
	maxAttempts := 5
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 2s, 4s, 8s, 16s
			delay := time.Duration(1<<uint(attempt)) * time.Second
			time.Sleep(delay)

			// Rebuild the request body if it was consumed
			if req.GetBody != nil {
				body, err := req.GetBody()
				if err != nil {
					return nil, fmt.Errorf("rebuilding request body: %w", err)
				}
				req.Body = body
			}
		}

		resp, err := c.http.Do(req)
		if err != nil {
			if attempt < maxAttempts-1 {
				continue
			}
			return nil, err
		}
		if resp.StatusCode == 403 || resp.StatusCode == 429 {
			resp.Body.Close()
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("request failed after %d attempts (rate limited)", maxAttempts)
}

func (c *Client) postForm(action string, extra map[string]string) (string, error) {
	for k, v := range extra {
		c.formState.Set(k, v)
	}

	// Delay between form submissions to avoid Cloudflare rate limits
	time.Sleep(500 * time.Millisecond)

	// Use the server-provided encoded action if available, otherwise encode ourselves
	encoded, ok := c.nextActions[action]
	if !ok {
		encoded = encodeParam(action, 8)
	}
	targetURL := ArchiveURL + "?i=" + encoded

	formData := c.formState.Encode()
	req, err := http.NewRequest("POST", targetURL, strings.NewReader(formData))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	// Allow body to be re-read on retry
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(formData)), nil
	}

	resp, err := c.doWithRetry(req)
	if err != nil {
		return "", fmt.Errorf("posting form: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	html := string(body)
	c.nextActions = parseActions(html)

	return html, nil
}

func (c *Client) acceptTerms() (string, error) {
	// First GET the page to parse the initial action values
	req, err := http.NewRequest("GET", ArchiveURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := c.doWithRetry(req)
	if err != nil {
		return "", fmt.Errorf("loading archive page: %w", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	c.nextActions = parseActions(string(body))

	return c.postForm("agree", map[string]string{
		"MaterialArchive__noTable__cbv__AgreeCheck": "Y",
		"MaterialArchive__noTable__cbh__AgreeCheck": "N",
	})
}

func (c *Client) selectType(materialType string) (string, error) {
	return c.postForm("type", map[string]string{
		"MaterialArchive__noTable__sbv__ViewType": materialType,
		"MaterialArchive__noTable__sbh__ViewType": "id",
	})
}

func (c *Client) selectYear(year string) (string, error) {
	return c.postForm("year", map[string]string{
		"MaterialArchive__noTable__sbv__YearSelect": year,
		"MaterialArchive__noTable__sbh__YearSelect": "id",
	})
}

func (c *Client) selectExam(exam string) (string, error) {
	return c.postForm("exam", map[string]string{
		"MaterialArchive__noTable__sbv__ExaminationSelect": exam,
		"MaterialArchive__noTable__sbh__ExaminationSelect": "id",
	})
}

func (c *Client) selectSubject(subjectCode string) (string, error) {
	return c.postForm("subject", map[string]string{
		"MaterialArchive__noTable__sbv__SubjectSelect": subjectCode,
		"MaterialArchive__noTable__sbh__SubjectSelect": "id",
	})
}

var selectRe = regexp.MustCompile(`name="([^"]+)"[^>]*>(.*?)</select>`)
var optionRe = regexp.MustCompile(`value="([^"]*)"[^>]*>([^<]*)`)

func parseOptions(html, selectName string) []Subject {
	for _, m := range selectRe.FindAllStringSubmatch(html, -1) {
		if m[1] != selectName {
			continue
		}
		var subjects []Subject
		for _, opt := range optionRe.FindAllStringSubmatch(m[2], -1) {
			if opt[1] == "" {
				continue
			}
			subjects = append(subjects, Subject{Code: opt[1], Name: strings.TrimSpace(opt[2])})
		}
		return subjects
	}
	return nil
}

var fileIDRe = regexp.MustCompile(`value='([^']+\.pdf)'`)
var paperEntryRe = regexp.MustCompile(
	`<TD class='materialbody'>([^<]+)</TD>\s*` +
		`<TD class='materialbody'>\s*` +
		`<a href=(\S+)[^>]*>Click Here</a>\s*` +
		`<font class='size'>\s*\[([^\]]+)\]`)

func parsePapers(html, year, exam, subjectCode, materialType string) []Paper {
	fileIDs := fileIDRe.FindAllStringSubmatch(html, -1)
	entries := paperEntryRe.FindAllStringSubmatch(html, -1)

	var papers []Paper
	for i, entry := range entries {
		fileID := ""
		if i < len(fileIDs) {
			fileID = fileIDs[i][1]
		}
		link := strings.TrimSpace(entry[2])
		if strings.HasPrefix(link, "?fp=") {
			link = link[4:]
		}

		// If fileID is empty, extract it from the decoded link
		if fileID == "" && link != "" {
			decoded := decodeParam(link)
			if idx := strings.LastIndex(decoded, "/"); idx >= 0 {
				fileID = decoded[idx+1:]
			}
		}

		papers = append(papers, Paper{
			Description:  strings.TrimSpace(entry[1]),
			FileID:       fileID,
			EncodedLink:  link,
			Size:         strings.TrimSpace(entry[3]),
			Year:         year,
			Exam:         exam,
			SubjectCode:  subjectCode,
			MaterialType: materialType,
		})
	}
	return papers
}

// GetYears returns available years for a material type.
func (c *Client) GetYears(materialType string) ([]string, error) {
	c.reset()
	if _, err := c.acceptTerms(); err != nil {
		return nil, err
	}
	html, err := c.selectType(materialType)
	if err != nil {
		return nil, err
	}
	subjects := parseOptions(html, "MaterialArchive__noTable__sbv__YearSelect")
	years := make([]string, len(subjects))
	for i, s := range subjects {
		years[i] = s.Code
	}
	return years, nil
}

// GetExaminations returns available exam types for a year.
func (c *Client) GetExaminations(materialType, year string) ([]Subject, error) {
	c.reset()
	if _, err := c.acceptTerms(); err != nil {
		return nil, err
	}
	if _, err := c.selectType(materialType); err != nil {
		return nil, err
	}
	html, err := c.selectYear(year)
	if err != nil {
		return nil, err
	}
	return parseOptions(html, "MaterialArchive__noTable__sbv__ExaminationSelect"), nil
}

// GetSubjects returns available subjects for a given exam and year.
func (c *Client) GetSubjects(materialType, year, exam string) ([]Subject, error) {
	c.reset()
	if _, err := c.acceptTerms(); err != nil {
		return nil, err
	}
	if _, err := c.selectType(materialType); err != nil {
		return nil, err
	}
	if _, err := c.selectYear(year); err != nil {
		return nil, err
	}
	html, err := c.selectExam(exam)
	if err != nil {
		return nil, err
	}
	return parseOptions(html, "MaterialArchive__noTable__sbv__SubjectSelect"), nil
}

// GetPapers returns all available papers for a specific subject.
func (c *Client) GetPapers(materialType, year, exam, subjectCode string) ([]Paper, error) {
	c.reset()
	if _, err := c.acceptTerms(); err != nil {
		return nil, err
	}
	if _, err := c.selectType(materialType); err != nil {
		return nil, err
	}
	if _, err := c.selectYear(year); err != nil {
		return nil, err
	}
	if _, err := c.selectExam(exam); err != nil {
		return nil, err
	}
	html, err := c.selectSubject(subjectCode)
	if err != nil {
		return nil, err
	}
	return parsePapers(html, year, exam, subjectCode, materialType), nil
}

// Filename returns a year-prefixed filename to avoid collisions across years.
func (p Paper) Filename() string {
	return p.Year + "_" + p.FileID
}

// DownloadPaper downloads a single paper to destDir, returning the file path.
func (c *Client) DownloadPaper(paper Paper, destDir string) (string, error) {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("creating directory: %w", err)
	}

	destPath := filepath.Join(destDir, paper.Filename())

	time.Sleep(300 * time.Millisecond)

	req, err := http.NewRequest("GET", paper.DirectURL(), nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := c.doWithRetry(req)
	if err != nil {
		return "", fmt.Errorf("downloading: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", fmt.Errorf("writing file: %w", err)
	}

	return destPath, nil
}
