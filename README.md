# exam-papers

A CLI and API for downloading Irish State Examination papers and marking schemes. Because the official website is, genuinely, one of the worst user experiences I've encountered on the modern internet.

## Why This Exists

Every Irish secondary school student needs past exam papers. The State Examinations Commission publishes them for free on [examinations.ie](https://www.examinations.ie/exammaterialarchive/). Sounds fine.

It is not fine.

The archive is a series of cascading dropdowns. You pick your material type, wait for a page reload, pick your year, page reload, pick your exam, page reload, pick your subject, page reload, and then finally you get a list of papers. Five full page reloads, each one a POST request that carries all your accumulated form state. You click on Higher Level Paper 1, and it opens in a new tab. Grand.

Then you realise you wanted 2023, not 2024. Back you go. Five dropdowns. Five page reloads. Pick the paper. And now you want the marking scheme too? That's a different material type, so start over. From the checkbox. Five reloads. Again.

I think you see the problem. Every student doing exam prep wants to look at the paper, then the marking scheme, then maybe last year's paper, then that marking scheme. That's four trips through the same five-step form. Twenty page reloads to do what should be four clicks.

So you think: fine, I'll just figure out the URL pattern and type them in directly. The PDFs are hosted somewhere, the links point to them, surely there's a logical naming convention. `LC003ALP100EV.pdf` is clearly Leaving Cert, subject 003 (Maths), Higher Level (A), Paper 1, English Version. That much makes sense. So you try hitting `examinations.ie/archive/exampapers/2024/LC003ALP100EV.pdf` and it works.

But the links on the website don't point there. They point to URLs like:

```
?fp=89.106.91.96.97.110.93.39.93.112.89.101.104.89.104.93.106.107.39.42.40.42.44.39.68.59.40.40.43.57.68.72.41.40.40.61.78.38.104.92.94.106
```

I stared at this for a while.

Those are dot-separated numbers. They look like ASCII codes, but shifted. After some fiddling I worked out the scheme: each number is an ASCII code minus a random offset, and the last number in the sequence is always `98 + offset`. So to decode it, you read the last number, subtract 98 to get the offset, and add that offset to every preceding number to recover the original characters. The string above decodes to `archive/exampapers/2024/LC003ALP100EV.pdf`.

The form action URLs use the same encoding. The "agree" action (accepting terms and conditions) becomes `89.95.106.93.93.106`. "type" becomes `108.113.104.93.106`. And the offset is regenerated randomly on every page load, so you can't hardcode them; you have to parse each HTML response, find the onChange handlers, and use the server-provided encoded values for your next POST.

I want to be clear: this is not encryption. The key is literally appended to the ciphertext. It's the security equivalent of locking your front door and leaving the key in the lock. My best theory is that someone was asked to prevent direct-linking to PDFs and this is what they came up with, maybe to make the Terms and Conditions checkbox feel more load-bearing. But the PDFs are accessible at their direct URLs anyway, so it doesn't even accomplish that.

These are public documents. They were paid for by the Irish taxpayer. And somewhere along the way, someone decided they needed to be wrapped in an obfuscation scheme that would take a bored student about an hour to crack, sitting behind a five-step form that has no reason to exist. God knows why.

So I reverse-engineered the whole thing and wrote this tool.

## How the Reverse Engineering Worked

I have a tool called [api-spy](https://github.com/eoghancollins/api-spy) for this kind of thing. You point it at a website, write a script to automate the clicks and form submissions, and it records all the network traffic correlated with each action. Pointed it at the exam archive, clicked through the dropdowns, and captured every POST and response.

From the recordings, I mapped out the form flow, the field names, the encoded action values, and the paper download links. Cracking the encoding took some staring, but once I found the offset pattern it was obvious. The whole "API" is just five POST requests in sequence, accumulating form state, with the server deciding which dropdown to reveal next. Not exactly REST.

## Installation

```bash
go install github.com/ETM-Code/exam-papers/cmd/exam-papers@latest
```

Or build from source:

```bash
git clone https://github.com/ETM-Code/exam-papers
cd exam-papers
go build -o exam-papers ./cmd/exam-papers/
```

## Usage

### Interactive Mode

Run it with no arguments. It walks you through the dropdowns (yes, the irony) but at least you can download everything at once:

```bash
exam-papers
```

### Non-Interactive Mode

The useful bit. Automation, scripting, bulk downloads:

```bash
# Download all LC Maths papers for 2024
exam-papers fetch -y 2024 -e lc -s 3

# Just Higher Level, English
exam-papers fetch -y 2024 -e lc -s 3 -l higher --lang english

# Marking schemes
exam-papers fetch -y 2024 -e lc -s 3 -t markingschemes

# List available subjects (you'll need the codes)
exam-papers list subjects -y 2024 -e lc

# List papers without downloading
exam-papers list papers -y 2024 -e lc -s 21 --json

# Download to a specific directory
exam-papers fetch -y 2024 -e lc -s 21 -o ./physics-papers
```

### Bulk Downloads

The whole point of this tool, really. Grab every Higher Level English paper for the last decade in one go:

```bash
# All Higher Level Maths papers, last 10 years
for year in $(seq 2025 -1 2015); do
  exam-papers fetch -y $year -e lc -s 3 -l higher --lang english -o ./maths-papers
done

# Every Physics paper and marking scheme for 2024
exam-papers fetch -y 2024 -e lc -s 21 -o ./physics
exam-papers fetch -y 2024 -e lc -s 21 -t markingschemes -o ./physics

# All Junior Cycle Science papers, Higher Level only
for year in $(seq 2025 -1 2015); do
  exam-papers fetch -y $year -e jc -s 15 -l higher -o ./jc-science
done
```

The tool handles rate limiting internally (Cloudflare), so you don't need to add sleeps between calls. It'll retry with exponential backoff if it gets throttled.

### As an API

```bash
exam-papers serve --port 8080
```

Then:

```
GET /api/years?type=exampapers
GET /api/subjects?type=exampapers&year=2024&exam=lc
GET /api/papers?type=exampapers&year=2024&exam=lc&subject=3
GET /api/papers?year=2024&exam=lc&subject=21&level=higher&lang=english
```

All JSON. The papers endpoint includes `direct_url` fields you can fetch directly.

## Common Subject Codes

These are the codes you pass to `-s`. The full list is available via `exam-papers list subjects`.

| Code | Subject | Code | Subject |
|------|---------|------|---------|
| 1 | Irish | 14 | Art |
| 2 | English | 21 | Physics |
| 3 | Mathematics | 22 | Chemistry |
| 4 | History | 25 | Biology |
| 5 | Geography | 33 | Business |
| 10 | French | 219 | Computer Science |
| 11 | German | 225 | Physical Education |
| 12 | Spanish | | |

## Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--year` | `-y` | Exam year | (required) |
| `--exam` | `-e` | Exam type: `lc`, `jc`, `lb` | `lc` |
| `--subject` | `-s` | Subject code | (required) |
| `--type` | `-t` | Material: `exampapers`, `markingschemes`, `deferredexams`, `deferredmarkingschemes` | `exampapers` |
| `--level` | `-l` | Filter: `higher`, `ordinary`, `foundation` | all |
| `--lang` | | Filter: `english`, `irish` | all |
| `--out` | `-o` | Output directory | `./papers` |
| `--json` | | JSON output | false |

## Notes

- **Rate limiting**: Cloudflare sits in front. The client handles this with delays and exponential backoff, but bulk-downloading 30 years of papers will involve some retries. It gets there.
- **Older papers** (pre-2007) use slightly different file naming. `ALPO00` instead of `ALP000`. Same papers, the tool handles it.
- **1995** only has Art, English, German, Irish, and French available. Not a bug.
- **Some papers are huge** (Ordinary Level Physics 2021 is 43MB). Likely because the original PDFs were lost at some point and replaced with scans.
- Files are named `{year}_{fileID}` to avoid collisions when downloading across multiple years.

## Agent Instructions

The `agent-instructions/` folder contains instruction files for AI coding assistants:

- `CLAUDE.md` for [Claude Code](https://claude.ai/code)
- `AGENTS.md` for [OpenAI Codex](https://openai.com/codex)

Copy whichever one you need to the project root (or wherever your tool expects it). They contain the project structure, build/test commands, API details, and the subject code reference.

## License

MIT
