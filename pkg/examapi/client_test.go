package examapi

import (
	"testing"
)

func TestEncodeParam(t *testing.T) {
	tests := []struct {
		input  string
		offset int
		want   string
	}{
		{"agree", 8, "89.95.106.93.93.106"},
		{"type", 8, "108.113.104.93.106"},
		{"year", 8, "113.93.89.106.106"},
	}

	for _, tt := range tests {
		got := encodeParam(tt.input, tt.offset)
		if got != tt.want {
			t.Errorf("encodeParam(%q, %d) = %q, want %q", tt.input, tt.offset, got, tt.want)
		}
	}
}

func TestDecodeParam(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// File path: archive/exampapers/2024/LC003ALP100EV.pdf
		{"89.106.91.96.97.110.93.39.93.112.89.101.104.89.104.93.106.107.39.42.40.42.44.39.68.59.40.40.43.57.68.72.41.40.40.61.78.38.104.92.94.106", "archive/exampapers/2024/LC003ALP100EV.pdf"},
		// Another file with different offset
		{"93.110.95.100.101.114.97.43.97.116.93.105.108.93.108.97.110.111.43.46.44.46.48.43.72.63.44.44.47.61.72.76.45.44.44.69.82.42.108.96.98.102", "archive/exampapers/2024/LC003ALP100IV.pdf"},
	}

	for _, tt := range tests {
		got := decodeParam(tt.input)
		if got != tt.want {
			t.Errorf("decodeParam(%q) = %q, want %q", tt.input[:40]+"...", got, tt.want)
		}
	}
}

func TestEncodeDecodeRoundtrip(t *testing.T) {
	inputs := []string{
		"agree",
		"type",
		"year",
		"exam",
		"subject",
		"archive/exampapers/2024/LC003ALP100EV.pdf",
		"archive/markingschemes/2023/JC002GLP000EV.pdf",
	}

	for _, input := range inputs {
		for _, offset := range []int{3, 5, 8, 12} {
			encoded := encodeParam(input, offset)
			decoded := decodeParam(encoded)
			if decoded != input {
				t.Errorf("roundtrip failed for %q (offset %d): got %q", input, offset, decoded)
			}
		}
	}
}

func TestPaperDirectURL(t *testing.T) {
	p := Paper{
		FileID:       "LC003ALP100EV.pdf",
		Year:         "2024",
		MaterialType: "exampapers",
	}
	want := "https://www.examinations.ie/archive/exampapers/2024/LC003ALP100EV.pdf"
	if got := p.DirectURL(); got != want {
		t.Errorf("DirectURL() = %q, want %q", got, want)
	}
}

func TestPaperLevel(t *testing.T) {
	tests := []struct {
		fileID string
		want   string
	}{
		{"LC003ALP100EV.pdf", "Higher"},
		{"LC003GLP100EV.pdf", "Ordinary"},
		{"LC003BLP000EV.pdf", "Foundation"},
	}

	for _, tt := range tests {
		p := Paper{FileID: tt.fileID}
		if got := p.Level(); got != tt.want {
			t.Errorf("Level() for %q = %q, want %q", tt.fileID, got, tt.want)
		}
	}
}

func TestPaperLanguage(t *testing.T) {
	tests := []struct {
		fileID string
		want   string
	}{
		{"LC003ALP100EV.pdf", "English"},
		{"LC003ALP100IV.pdf", "Irish"},
	}

	for _, tt := range tests {
		p := Paper{FileID: tt.fileID}
		if got := p.Language(); got != tt.want {
			t.Errorf("Language() for %q = %q, want %q", tt.fileID, got, tt.want)
		}
	}
}
