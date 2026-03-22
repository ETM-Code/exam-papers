package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ETM-Code/exam-papers/pkg/examapi"
)

func usage() {
	fmt.Fprintf(os.Stderr, `exam-papers - Fetch Irish State Examination papers and marking schemes

Usage:
  exam-papers                                     Interactive mode
  exam-papers fetch [flags]                        Download papers (non-interactive)
  exam-papers list subjects [flags]                List available subjects
  exam-papers list years [flags]                   List available years
  exam-papers list papers [flags]                  List papers without downloading
  exam-papers serve [--port 8080]                  Run as HTTP API

Fetch flags:
  --year, -y <year>         Exam year (e.g. 2024)
  --exam, -e <code>         Exam type: lc, jc, lb (default: lc)
  --subject, -s <code>      Subject code (use 'list subjects' to find codes)
  --type, -t <type>         Material type: exampapers, markingschemes,
                            deferredexams, deferredmarkingschemes (default: exampapers)
  --level, -l <level>       Filter by level: higher, ordinary, foundation
  --lang <lang>             Filter by language: english, irish (default: all)
  --out, -o <dir>           Output directory (default: ./papers)
  --json                    Output as JSON instead of human-readable

Examples:
  exam-papers fetch -y 2024 -e lc -s 3                     # LC Maths 2024
  exam-papers fetch -y 2024 -e lc -s 3 -l higher           # Higher level only
  exam-papers fetch -y 2024 -e lc -s 3 -t markingschemes   # Marking schemes
  exam-papers list subjects -y 2024 -e lc                   # List LC subjects
  exam-papers list papers -y 2024 -e jc -s 2 --json        # JSON output
`)
}

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		interactive()
		return
	}

	switch args[0] {
	case "fetch":
		cmdFetch(args[1:])
	case "list":
		if len(args) < 2 {
			fatal("Usage: exam-papers list <subjects|years|papers> [flags]")
		}
		switch args[1] {
		case "subjects":
			cmdListSubjects(args[2:])
		case "years":
			cmdListYears(args[2:])
		case "papers":
			cmdListPapers(args[2:])
		default:
			fatal("Unknown list command: %s", args[1])
		}
	case "serve":
		cmdServe(args[1:])
	case "--help", "-h", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", args[0])
		usage()
		os.Exit(1)
	}
}

type flags struct {
	year         string
	exam         string
	subject      string
	materialType string
	level        string
	lang         string
	outDir       string
	jsonOutput   bool
	port         string
}

func parseFlags(args []string) flags {
	f := flags{
		exam:         "lc",
		materialType: "exampapers",
		outDir:       "./papers",
		port:         "8080",
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--year", "-y":
			i++
			if i < len(args) {
				f.year = args[i]
			}
		case "--exam", "-e":
			i++
			if i < len(args) {
				f.exam = args[i]
			}
		case "--subject", "-s":
			i++
			if i < len(args) {
				f.subject = args[i]
			}
		case "--type", "-t":
			i++
			if i < len(args) {
				f.materialType = args[i]
			}
		case "--level", "-l":
			i++
			if i < len(args) {
				f.level = strings.ToLower(args[i])
			}
		case "--lang":
			i++
			if i < len(args) {
				f.lang = strings.ToLower(args[i])
			}
		case "--out", "-o":
			i++
			if i < len(args) {
				f.outDir = args[i]
			}
		case "--json":
			f.jsonOutput = true
		case "--port", "-p":
			i++
			if i < len(args) {
				f.port = args[i]
			}
		}
	}

	return f
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func filterPapers(papers []examapi.Paper, level, lang string) []examapi.Paper {
	if level == "" && lang == "" {
		return papers
	}
	var filtered []examapi.Paper
	for _, p := range papers {
		if level != "" && strings.ToLower(p.Level()) != level {
			continue
		}
		if lang != "" && strings.ToLower(p.Language()) != lang {
			continue
		}
		filtered = append(filtered, p)
	}
	return filtered
}

func cmdFetch(args []string) {
	f := parseFlags(args)
	if f.year == "" {
		fatal("--year is required. Use 'exam-papers list years' to see options.")
	}
	if f.subject == "" {
		fatal("--subject is required. Use 'exam-papers list subjects -y %s -e %s' to see codes.", f.year, f.exam)
	}

	client := examapi.NewClient()
	papers, err := client.GetPapers(f.materialType, f.year, f.exam, f.subject)
	if err != nil {
		fatal("Error fetching papers: %v", err)
	}

	papers = filterPapers(papers, f.level, f.lang)

	if len(papers) == 0 {
		fmt.Println("No papers found matching your criteria.")
		return
	}

	if f.jsonOutput {
		type result struct {
			Path  string       `json:"path"`
			Paper examapi.Paper `json:"paper"`
		}
		var results []result
		for _, p := range papers {
			path, err := client.DownloadPaper(p, f.outDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error downloading %s: %v\n", p.FileID, err)
				continue
			}
			results = append(results, result{Path: path, Paper: p})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(results)
		return
	}

	fmt.Printf("Downloading %d papers to %s/\n\n", len(papers), f.outDir)
	for i, p := range papers {
		path, err := client.DownloadPaper(p, f.outDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [%d/%d] FAILED %s: %v\n", i+1, len(papers), p.Description, err)
			continue
		}
		fmt.Printf("  [%d/%d] %s (%s) -> %s\n", i+1, len(papers), p.Description, p.Size, path)
	}
	fmt.Println("\nDone.")
}

func cmdListSubjects(args []string) {
	f := parseFlags(args)
	if f.year == "" {
		f.year = "2024"
	}

	client := examapi.NewClient()
	subjects, err := client.GetSubjects(f.materialType, f.year, f.exam)
	if err != nil {
		fatal("Error: %v", err)
	}

	if f.jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(subjects)
		return
	}

	fmt.Printf("Subjects for %s %s (%s):\n\n", examapi.Examinations[f.exam], f.year, examapi.MaterialTypes[f.materialType])
	for _, s := range subjects {
		fmt.Printf("  %-6s %s\n", s.Code, s.Name)
	}
	fmt.Printf("\n%d subjects available\n", len(subjects))
}

func cmdListYears(args []string) {
	f := parseFlags(args)

	client := examapi.NewClient()
	years, err := client.GetYears(f.materialType)
	if err != nil {
		fatal("Error: %v", err)
	}

	if f.jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(years)
		return
	}

	fmt.Printf("Available years (%s):\n\n", examapi.MaterialTypes[f.materialType])
	for _, y := range years {
		fmt.Printf("  %s\n", y)
	}
}

func cmdListPapers(args []string) {
	f := parseFlags(args)
	if f.year == "" {
		fatal("--year is required")
	}
	if f.subject == "" {
		fatal("--subject is required")
	}

	client := examapi.NewClient()
	papers, err := client.GetPapers(f.materialType, f.year, f.exam, f.subject)
	if err != nil {
		fatal("Error: %v", err)
	}

	papers = filterPapers(papers, f.level, f.lang)

	if f.jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(papers)
		return
	}

	fmt.Printf("Papers for subject %s, %s %s:\n\n", f.subject, f.exam, f.year)
	for _, p := range papers {
		fmt.Printf("  %-45s %-10s %s\n", p.Description, p.Size, p.DirectURL())
	}
	fmt.Printf("\n%d papers available\n", len(papers))
}

// --- Interactive mode ---

func interactive() {
	client := examapi.NewClient()

	fmt.Println("exam-papers - Irish State Examination Paper Fetcher")
	fmt.Println("===================================================")
	fmt.Println()

	// Step 1: Material type
	fmt.Println("What would you like to download?")
	typeKeys := []string{"exampapers", "markingschemes", "deferredexams", "deferredmarkingschemes"}
	for i, k := range typeKeys {
		fmt.Printf("  %d. %s\n", i+1, examapi.MaterialTypes[k])
	}
	materialType := typeKeys[promptChoice("Choice", len(typeKeys))-1]

	// Step 2: Year
	fmt.Println("\nFetching available years...")
	years, err := client.GetYears(materialType)
	if err != nil {
		fatal("Error: %v", err)
	}
	fmt.Println("Available years:")
	// Show in rows of 10
	for i, y := range years {
		if i > 0 && i%10 == 0 {
			fmt.Println()
		}
		fmt.Printf("  %s", y)
	}
	fmt.Println()
	year := promptString("\nEnter year")

	// Validate year
	valid := false
	for _, y := range years {
		if y == year {
			valid = true
			break
		}
	}
	if !valid {
		fatal("Invalid year: %s", year)
	}

	// Step 3: Exam type
	fmt.Println("\nFetching exam types...")
	exams, err := client.GetExaminations(materialType, year)
	if err != nil {
		fatal("Error: %v", err)
	}
	fmt.Println("Exam types:")
	for i, e := range exams {
		fmt.Printf("  %d. %s (%s)\n", i+1, e.Name, e.Code)
	}
	examIdx := promptChoice("Choice", len(exams)) - 1
	exam := exams[examIdx].Code

	// Step 4: Subject
	fmt.Println("\nFetching subjects...")
	subjects, err := client.GetSubjects(materialType, year, exam)
	if err != nil {
		fatal("Error: %v", err)
	}
	fmt.Println("Subjects:")
	for i, s := range subjects {
		fmt.Printf("  %2d. %-6s %s\n", i+1, s.Code, s.Name)
	}
	subjectIdx := promptChoice("\nChoice", len(subjects)) - 1
	subject := subjects[subjectIdx]

	// Step 5: Fetch and display papers
	fmt.Printf("\nFetching papers for %s...\n", subject.Name)
	papers, err := client.GetPapers(materialType, year, exam, subject.Code)
	if err != nil {
		fatal("Error: %v", err)
	}

	if len(papers) == 0 {
		fmt.Println("No papers found.")
		return
	}

	fmt.Printf("\nFound %d papers:\n", len(papers))
	for i, p := range papers {
		fmt.Printf("  %2d. %-45s %s\n", i+1, p.Description, p.Size)
	}

	// Ask what to download
	fmt.Println("\nDownload options:")
	fmt.Println("  a. Download all")
	fmt.Println("  s. Select specific papers (e.g. 1,3,5)")
	fmt.Println("  q. Quit")
	choice := promptString("Choice")

	var toDownload []examapi.Paper
	switch strings.ToLower(choice) {
	case "a":
		toDownload = papers
	case "q":
		return
	default:
		// Parse comma-separated indices
		for _, part := range strings.Split(choice, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 1 || idx > len(papers) {
				fmt.Fprintf(os.Stderr, "Skipping invalid selection: %s\n", part)
				continue
			}
			toDownload = append(toDownload, papers[idx-1])
		}
	}

	if len(toDownload) == 0 {
		fmt.Println("Nothing to download.")
		return
	}

	outDir := promptStringDefault("Output directory", "./papers")

	fmt.Printf("\nDownloading %d papers to %s/\n\n", len(toDownload), outDir)
	for i, p := range toDownload {
		path, err := client.DownloadPaper(p, outDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [%d/%d] FAILED %s: %v\n", i+1, len(toDownload), p.Description, err)
			continue
		}
		fmt.Printf("  [%d/%d] %s (%s) -> %s\n", i+1, len(toDownload), p.Description, p.Size, path)
	}
	fmt.Println("\nDone.")
}

func promptChoice(prompt string, max int) int {
	for {
		s := promptString(prompt)
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > max {
			fmt.Printf("Please enter a number between 1 and %d.\n", max)
			continue
		}
		return n
	}
}

func promptString(prompt string) string {
	fmt.Printf("%s: ", prompt)
	var s string
	fmt.Scanln(&s)
	return strings.TrimSpace(s)
}

func promptStringDefault(prompt, def string) string {
	fmt.Printf("%s [%s]: ", prompt, def)
	var s string
	fmt.Scanln(&s)
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	return s
}
