# Agent Instructions

This is a Go web application — a multi-user feed reader — hosted on exe.dev.

See README.md for user-facing docs (features, API endpoints, scraper config, etc.).

## Important

Do **not** use subagents to read all files and output all file contents in full.
This will fail due to context/token limits and should not be attempted.

**Always validate your changes.** After making code changes, run `make check`
before committing. This runs formatting checks, linting, tests, and
vulnerability scanning in one command.

## Tech Stack

- **Go** (module `srv.exe.dev`) with the standard library `net/http` router
- **SQLite** via `modernc.org/sqlite` (pure-Go, no CGO)
- **sqlc** for type-safe query generation (`db/queries/` → `db/dbgen/`)
- **HTML templates** (`html/template`) served server-side
- **Vanilla JS** frontend (no framework) in `srv/static/`
- Runs on port **8000** behind the exe.dev HTTPS proxy

## Code Layout

```
cmd/srv/main.go          Entry point — parses flags, opens DB, starts server
srv/
  server.go              HTTP server, all route handlers (~2600 lines, the bulk of the app)
  auth.go                exe.dev auth middleware (reads X-Exedev-* headers; DEV=1 bypass)
  filter.go              Exclusion-rule filtering (keyword/author per folder)
  content_filter.go      Per-feed content transform filters
  category_tree.go       Nested category/folder tree builder
  retention.go           Data retention / old-article cleanup
  ai_scraper.go          Anthropic API integration for AI scraper config generation
  feeds/
    fetcher.go           RSS/Atom feed fetching with conditional GET support
    parser.go            RSS and Atom XML parser
    fetcher_test.go      Fetcher tests
    parser_test.go       Parser tests
  scrapers/
    runner.go            CSS-selector-based HTML scraper (goquery)
    runner_test.go       Scraper tests
    json_scraper.go      JSON API scraper
  huggingface/
    huggingface.go       Hugging Face-specific feed source
  opml/
    opml.go              OPML import/export
  email/
    email.go             Newsletter email receiving (SMTP)
  templates/             Server-rendered HTML templates
    base.html            Shared layout (nav, head, sidebar)
    index.html           Main article list view
    article.html         Single article view
    feeds.html           Feed management page
    scrapers.html        Scraper management page
    settings.html        User settings page
    queue.html           Reading queue page
    history.html         Reading history page
    category_settings.html  Folder exclusion-rule settings
  static/
    app.js               Main client-side JS (~2300 lines)
    app.test.js          JS unit tests (vitest)
    test-helper.js       JS test helper
    script.js            Service worker registration
    sw.js                Service worker
    style.css            All styles (~3000 lines)
db/
  db.go                  DB open, migrations, pragmas
  sqlc.yaml              sqlc configuration
  migrations/            Numbered SQL migration files (001–015)
  queries/               SQL query files for sqlc
  dbgen/                 Generated Go code from sqlc (do not edit by hand)
```

## Key Patterns

- **Authentication**: All non-static routes go through auth middleware. The
  exe.dev proxy injects `X-Exedev-Userid` and `X-Exedev-Email` headers.
  Set `DEV=1` to skip auth for local development.
- **Database migrations**: Auto-applied on startup in `db.Open()`. Add new
  migrations as sequentially numbered `.sql` files in `db/migrations/`.
- **sqlc workflow**: Edit SQL in `db/queries/*.sql`, then run
  `go generate ./db/...` to regenerate `db/dbgen/`.
- **Single-file server**: Most handler logic lives in `srv/server.go`.
  It's large but flat — each handler is a standalone function on the server struct.
- **Build & validation**: See the Build Workflow section below.
- **Service**: Managed via systemd (`srv.service`). Restart after changes
  with `make build && sudo systemctl restart feedreader`.

## Build Workflow

Run `make check` before committing. It runs all validation steps in order:

| Command          | What it does                                       |
|------------------|----------------------------------------------------|
| `make check`     | **Run all five steps below in sequence**             |
| `make fmt-check` | Fail if any Go files need `goimports` formatting    |
| `make fix-check` | Fail if `go fix` has unapplied modernizations       |
| `make lint`      | `golangci-lint` (Go) + `eslint` (JS) + template lint + `djlint` (HTML) |
| `make test`      | `go test ./...`                                     |
| `make vulncheck` | `govulncheck` — scan deps for known vulnerabilities |

Other useful targets:

| Command      | What it does                                  |
|--------------|-----------------------------------------------|
| `make build` | Compile the `./feedreader` binary             |
| `make fmt`   | Auto-fix Go formatting with `goimports -w .`  |

### Linting details

- **Go** (`.golangci.yml`): errcheck, govet, staticcheck, gocritic
  (all checks enabled),
  misspell, nilerr, errorlint, bodyclose, ineffassign, unused.
  `errcheck`/`bodyclose`/`unused` are suppressed in test files;
  the `std-error-handling` exclusion preset has been removed.
  Generated `db/dbgen/` is excluded entirely.
- **JS** (`eslint.config.mjs`): no-undef, no-unused-vars, eqeqeq, no-eval,
  etc. Functions called from HTML `onclick` attributes are whitelisted in
  `varsIgnorePattern`.
- **CSS** (`.stylelintrc.json`): stylelint with `stylelint-config-standard`.
  Catches duplicate selectors/properties, deprecated values, redundant
  shorthands, and pseudo-element notation. Stylistic color/alpha notation
  rules and `no-descending-specificity` are disabled.
- **Templates** (`cmd/lint-templates/`): Validates Go html/template files
  parse correctly with base.html, checks for mismatched `{{ }}`
  delimiters, unclosed HTML tags, mismatched open/close tags, and
  unused FuncMap entries.
- **HTML** (`djlint`, `.djlintrc`): HTML linting for template files with
  Go template profile. Configured to ignore H006 (img dimensions),
  H019 (javascript: hrefs), H021 (inline styles), H023 (entity refs),
  H031 (meta keywords).

### Fixing formatting

If `make fmt-check` fails, run `make fmt` to auto-fix, then re-run
`make check`.

## Viewing the App During Development

See README.md for full details. Quick options:

1. **With auth proxy**: Use `mitmdump` on port 3000 to inject auth headers,
   then browse `http://localhost:3000/`.

Look up real user credentials with:
```bash
sqlite3 db.sqlite3 "SELECT external_id, email FROM users;"
```
