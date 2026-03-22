# exam-papers

Go CLI/API for fetching Irish State Exam papers from examinations.ie.

## Build & Test

- Build: `go build -o exam-papers ./cmd/exam-papers/`
- Unit tests: `go test ./pkg/examapi/ -run 'Test[^I]'`
- Integration tests: `EXAM_INTEGRATION=1 go test ./pkg/examapi/ -run TestIntegration`

## Architecture

- `pkg/examapi/` - Client library. The site uses obfuscated URL params (shifted ASCII, random offset per page load). Action values are parsed from each HTML response's onChange handlers.
- `cmd/exam-papers/main.go` - CLI: interactive (no args) and non-interactive (subcommands).
- `cmd/exam-papers/server.go` - HTTP API server.

## Key Details

- Rate limiting: Cloudflare. 500ms between form POSTs, 300ms between downloads, exponential backoff retry on 403/429.
- Direct PDF URLs: `https://www.examinations.ie/archive/{type}/{year}/{fileID}`
- Common subject codes: 1=Irish, 2=English, 3=Maths, 21=Physics, 22=Chemistry, 25=Biology
