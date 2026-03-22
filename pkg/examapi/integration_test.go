package examapi

import (
	"os"
	"testing"
)

func skipIfNoNetwork(t *testing.T) {
	if os.Getenv("EXAM_INTEGRATION") == "" {
		t.Skip("Set EXAM_INTEGRATION=1 to run integration tests")
	}
}

func TestIntegrationGetYears(t *testing.T) {
	skipIfNoNetwork(t)

	client := NewClient()
	years, err := client.GetYears("exampapers")
	if err != nil {
		t.Fatalf("GetYears: %v", err)
	}
	if len(years) < 20 {
		t.Errorf("Expected 20+ years, got %d", len(years))
	}
	// 2024 should always be available
	found := false
	for _, y := range years {
		if y == "2024" {
			found = true
			break
		}
	}
	if !found {
		t.Error("2024 not in years list")
	}
}

func TestIntegrationGetSubjects(t *testing.T) {
	skipIfNoNetwork(t)

	client := NewClient()
	subjects, err := client.GetSubjects("exampapers", "2024", "lc")
	if err != nil {
		t.Fatalf("GetSubjects: %v", err)
	}
	if len(subjects) < 30 {
		t.Errorf("Expected 30+ LC subjects, got %d", len(subjects))
	}
	// Mathematics (code 3) should be present
	found := false
	for _, s := range subjects {
		if s.Code == "3" && s.Name == "Mathematics" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Mathematics (code 3) not found in subjects")
	}
}

func TestIntegrationGetPapers(t *testing.T) {
	skipIfNoNetwork(t)

	client := NewClient()
	papers, err := client.GetPapers("exampapers", "2024", "lc", "3")
	if err != nil {
		t.Fatalf("GetPapers: %v", err)
	}
	if len(papers) == 0 {
		t.Fatal("No papers returned for LC Maths 2024")
	}

	// Should have Higher, Ordinary, Foundation levels
	levels := map[string]bool{}
	for _, p := range papers {
		levels[p.Level()] = true
	}
	for _, expected := range []string{"Higher", "Ordinary", "Foundation"} {
		if !levels[expected] {
			t.Errorf("Missing %s level papers", expected)
		}
	}

	// Verify direct URLs look correct
	for _, p := range papers {
		url := p.DirectURL()
		if url == "" {
			t.Error("Empty DirectURL")
		}
		if p.FileID == "" {
			t.Error("Empty FileID")
		}
	}
}

func TestIntegrationDownload(t *testing.T) {
	skipIfNoNetwork(t)

	client := NewClient()
	papers, err := client.GetPapers("exampapers", "2024", "lc", "3")
	if err != nil {
		t.Fatalf("GetPapers: %v", err)
	}

	// Download just the first (smallest) paper
	if len(papers) == 0 {
		t.Fatal("No papers to download")
	}

	dir := t.TempDir()
	path, err := client.DownloadPaper(papers[0], dir)
	if err != nil {
		t.Fatalf("DownloadPaper: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size() < 1000 {
		t.Errorf("Downloaded file too small: %d bytes", info.Size())
	}
	t.Logf("Downloaded %s (%d bytes)", papers[0].Description, info.Size())
}
