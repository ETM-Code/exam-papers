# exam-papers

CLI and API for fetching Irish State Examination papers and marking schemes from examinations.ie.

## Build & Run

```bash
go build -o exam-papers ./cmd/exam-papers/
./exam-papers --help
```

## Project Structure

- `pkg/examapi/` - Core client library. Handles form submissions, HTML parsing, obfuscation encoding/decoding, rate limiting with exponential backoff.
- `cmd/exam-papers/main.go` - CLI with interactive mode (no args) and non-interactive mode (subcommands).
- `cmd/exam-papers/server.go` - HTTP API server (`exam-papers serve`).

## How the API Works

The site uses a multi-step form where each dropdown POST reveals the next. The URL parameters (`?i=` for actions, `?fp=` for files) are obfuscated: dot-separated ASCII codes with a random offset. The last number is the key (offset = key - 98). The server generates new action values with each response, so we must parse onChange handlers from each HTML response and reuse them.

Direct PDF access: `https://www.examinations.ie/archive/{type}/{year}/{fileID}`

## Testing

```bash
go test ./pkg/examapi/ -v -run 'Test[^I]'              # Unit tests
EXAM_INTEGRATION=1 go test ./pkg/examapi/ -v -run TestIntegration  # Integration (hits real API)
```

## Rate Limiting

The site uses Cloudflare. The client has 500ms delays between form submissions, 300ms between downloads, and exponential backoff retry (up to 5 attempts) on 403/429 responses. When bulk downloading across many years, the rate limiter may still kick in.

## Common Subject Codes (LC)

1=Irish, 2=English, 3=Maths, 4=History, 5=Geography, 10=French, 11=German, 12=Spanish, 14=Art, 21=Physics, 22=Chemistry, 25=Biology, 219=Computer Science
