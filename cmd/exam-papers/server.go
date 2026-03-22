package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/ETM-Code/exam-papers/pkg/examapi"
)

func cmdServe(args []string) {
	f := parseFlags(args)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/years", handleYears)
	mux.HandleFunc("GET /api/examinations", handleExaminations)
	mux.HandleFunc("GET /api/subjects", handleSubjects)
	mux.HandleFunc("GET /api/papers", handlePapers)
	mux.HandleFunc("GET /api/download", handleDownload)
	mux.HandleFunc("GET /", handleIndex)

	addr := ":" + f.port
	fmt.Fprintf(os.Stderr, "Listening on http://localhost%s\n", addr)
	fmt.Fprintf(os.Stderr, "\nEndpoints:\n")
	fmt.Fprintf(os.Stderr, "  GET /api/years?type=exampapers\n")
	fmt.Fprintf(os.Stderr, "  GET /api/examinations?type=exampapers&year=2024\n")
	fmt.Fprintf(os.Stderr, "  GET /api/subjects?type=exampapers&year=2024&exam=lc\n")
	fmt.Fprintf(os.Stderr, "  GET /api/papers?type=exampapers&year=2024&exam=lc&subject=3\n")
	fmt.Fprintf(os.Stderr, "  GET /api/download?type=exampapers&year=2024&exam=lc&subject=3&level=higher&lang=english\n")
	fmt.Fprintf(os.Stderr, "\n")

	if err := http.ListenAndServe(addr, mux); err != nil {
		fatal("Server error: %v", err)
	}
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"name":    "exam-papers",
		"version": "1.0.0",
		"endpoints": []string{
			"/api/years?type=exampapers",
			"/api/examinations?type=exampapers&year=2024",
			"/api/subjects?type=exampapers&year=2024&exam=lc",
			"/api/papers?type=exampapers&year=2024&exam=lc&subject=3",
			"/api/download?type=exampapers&year=2024&exam=lc&subject=3",
		},
	})
}

func handleYears(w http.ResponseWriter, r *http.Request) {
	materialType := r.URL.Query().Get("type")
	if materialType == "" {
		materialType = "exampapers"
	}

	client := examapi.NewClient()
	years, err := client.GetYears(materialType)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	writeJSON(w, map[string]any{"years": years, "type": materialType})
}

func handleExaminations(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	materialType := q.Get("type")
	if materialType == "" {
		materialType = "exampapers"
	}
	year := q.Get("year")
	if year == "" {
		writeError(w, 400, "year parameter is required")
		return
	}

	client := examapi.NewClient()
	exams, err := client.GetExaminations(materialType, year)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	writeJSON(w, map[string]any{"examinations": exams, "type": materialType, "year": year})
}

func handleSubjects(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	materialType := q.Get("type")
	if materialType == "" {
		materialType = "exampapers"
	}
	year := q.Get("year")
	if year == "" {
		writeError(w, 400, "year parameter is required")
		return
	}
	exam := q.Get("exam")
	if exam == "" {
		exam = "lc"
	}

	client := examapi.NewClient()
	subjects, err := client.GetSubjects(materialType, year, exam)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	writeJSON(w, map[string]any{"subjects": subjects, "type": materialType, "year": year, "exam": exam})
}

func handlePapers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	materialType := q.Get("type")
	if materialType == "" {
		materialType = "exampapers"
	}
	year := q.Get("year")
	if year == "" {
		writeError(w, 400, "year parameter is required")
		return
	}
	exam := q.Get("exam")
	if exam == "" {
		exam = "lc"
	}
	subject := q.Get("subject")
	if subject == "" {
		writeError(w, 400, "subject parameter is required")
		return
	}

	client := examapi.NewClient()
	papers, err := client.GetPapers(materialType, year, exam, subject)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	papers = filterPapers(papers, q.Get("level"), q.Get("lang"))

	// Add direct_url to JSON output
	type paperJSON struct {
		examapi.Paper
		DirectURL string `json:"direct_url"`
		Level     string `json:"level"`
		Language  string `json:"language"`
	}
	out := make([]paperJSON, len(papers))
	for i, p := range papers {
		out[i] = paperJSON{Paper: p, DirectURL: p.DirectURL(), Level: p.Level(), Language: p.Language()}
	}

	writeJSON(w, map[string]any{"papers": out, "type": materialType, "year": year, "exam": exam, "subject": subject})
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	materialType := q.Get("type")
	if materialType == "" {
		materialType = "exampapers"
	}
	year := q.Get("year")
	if year == "" {
		writeError(w, 400, "year parameter is required")
		return
	}
	exam := q.Get("exam")
	if exam == "" {
		exam = "lc"
	}
	subject := q.Get("subject")
	if subject == "" {
		writeError(w, 400, "subject parameter is required")
		return
	}

	client := examapi.NewClient()
	papers, err := client.GetPapers(materialType, year, exam, subject)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	papers = filterPapers(papers, q.Get("level"), q.Get("lang"))

	if len(papers) == 0 {
		writeError(w, 404, "no papers found matching criteria")
		return
	}

	// If single paper, redirect to direct URL
	if len(papers) == 1 {
		http.Redirect(w, r, papers[0].DirectURL(), http.StatusFound)
		return
	}

	// Multiple papers: return JSON with direct URLs
	type downloadInfo struct {
		Description string `json:"description"`
		DirectURL   string `json:"direct_url"`
		Size        string `json:"size"`
		FileID      string `json:"file_id"`
	}
	out := make([]downloadInfo, len(papers))
	for i, p := range papers {
		out[i] = downloadInfo{
			Description: p.Description,
			DirectURL:   p.DirectURL(),
			Size:        p.Size,
			FileID:      p.FileID,
		}
	}

	writeJSON(w, map[string]any{"papers": out, "count": len(out)})
}
