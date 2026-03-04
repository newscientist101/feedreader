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
    app.js               Entry point — imports modules, runs init (~360 lines)
    script.js            Service worker registration
    sw.js                Service worker
    style.css            All styles (~2900 lines)
    modules/             ES modules (each with a .test.js companion)
      api.js             Fetch wrapper for /api/* endpoints
      article-actions.js Mark read/unread, star, queue, auto-mark-read observer
      articles.js        Article rendering — cards, actions, embeds, user prefs
      counts.js          Unread/starred counts, feed status, error banners
      drag-drop.js       Drag-and-drop reordering for feeds and folders
      dropdown.js        Dropdown toggle and click-outside close
      feed-errors.js     Feed error banner (shared leaf: counts + feeds)
      feeds.js           Feed CRUD, load articles, edit modal
      folders.js         Folder CRUD, category settings page
      icons.js           SVG icon constant strings
      offline.js         Service worker, online/offline handling, cache
      opml.js            OPML import/export
      pagination.js      Cursor-based pagination and infinite scroll
      queue.js           Reading queue page init
      read-button.js     Read/unread toggle button (shared leaf: articles + article-actions)
      scraper-page.js    Scraper management page (tabs, AI, config modal)
      settings-page.js   Settings page init, cleanup, newsletter
      settings.js        User settings (get/save/apply)
      sidebar.js         Sidebar toggle, folder navigation, collapse
      timestamps.js      Tooltip initialization for timestamps
      utils.js           formatTimeAgo, stripHtml, truncateText, etc.
      views.js           View mode switching (cards/list/magazine/expanded)
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
- **ES modules**: Client-side JS uses native ES modules (no bundler).
  `app.js` is the entry point (`<script type="module">`), importing from
  `modules/`. An import map in `base.html` maps module URLs to versioned
  URLs (`?v=hash`) for cache busting. No inline `onclick` handlers — all
  event wiring uses `data-action` attributes with delegated `addEventListener`.
  Circular dependencies are resolved via late-bound setters (e.g.,
  `setArticleActionDeps()`). Each module has a companion `.test.js` file.
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

### Test Users

All testing and development should use **user 1** (`external_id: dev-user-1`,
`email: test@example.com`) unless explicitly asked to use another user.

- **Do not** modify user 4 — this is the real production user.
- **Do not** create additional users.

For the auth proxy, inject these headers:
```bash
mitmdump -p 3000 --mode reverse:http://localhost:8000 \
  --set modify_headers='/~q/X-Exedev-Userid/dev-user-1' \
  --set modify_headers='/~q/X-Exedev-Email/test@example.com'
```

Look up user credentials with:
```bash
sqlite3 db.sqlite3 "SELECT id, external_id, email FROM users;"
```

<!-- BEGIN BEADS INTEGRATION -->
## Issue Tracking with bd (beads)

All task tracking uses **bd** (beads). Do not use markdown TODOs or task lists.
Issue IDs use the `feedreader-` prefix (e.g., `feedreader-fc2`).

The Dolt server runs as a systemd service (`beads-dolt.service`) on port 3307.
A task agent (`task-agent.service`) runs hourly via systemd timer, picks
ready tasks, and works them autonomously. The agent script and systemd units
live in `/home/exedev/ops/` (a local-only git repo, no remote).

A **troubleshooter agent** (`troubleshooter.service`) runs daily at 06:00 UTC.
It scans the feedreader service logs for ERROR-level entries and crashes. If
any are found (after filtering), it launches a Shelley conversation to
investigate and create bd tasks. It does not fix issues — only investigates
and files bugs.

**Known-issues file** (`/home/exedev/ops/troubleshooter-known-issues.txt`):
Patterns listed here are excluded from all log scans. Use this to suppress
fixed issues that are still in the 24hr log window. One fixed-string pattern
per line; lines starting with `#` are comments. Stale entries (patterns that
no longer match any log lines) are pruned automatically on each run.

### Finding and doing work

```bash
bd ready                              # Show unblocked issues
bd show <id>                          # Full details (description, design, notes, acceptance)
bd update <id> --claim                # Claim a task (sets in_progress + assignee)
# ... do the work, run make check ...
bd close <id> -r "what was done"       # Complete it
```

### Creating issues

```bash
# Simple task
bd create "Fix pagination bug" -t bug -p 1 -d "Description of what's broken"

# Task discovered while working on another
bd create "Edge case in feed parser" -t bug --deps discovered-from:feedreader-fc2

# Batch from markdown file (## headings = issue titles, body = description)
bd create -f tasks.md
```

Types: `bug`, `feature`, `task`, `epic`, `chore`, `decision`
Priorities: `0` (critical) through `4` (backlog), default `2`

### Planning with epics

For multi-step work, create an epic with children and dependencies:

```bash
# Create the epic
bd create "Redesign settings page" -t epic -d "Goals and rationale" \
  --design "Technical approach and architecture" \
  --acceptance "Definition of done"

# Create child tasks
bd create "Extract settings components" -t task --parent <epic-id>
bd create "Add validation" -t task --parent <epic-id>

# Set ordering via dependencies
bd dep add <validation-id> <extract-id>   # validation depends on extract
```

Epic fields:
- **description** — why this work exists, goals
- **design** — technical approach, architecture, module structure
- **acceptance** — concrete done-criteria
- **notes** — working memory, append with `bd update <id> --append-notes "..."`

To set long-form fields from a file:
```bash
cat plan.md | EDITOR="cp /dev/stdin" bd edit <id> --design
```

### Dependencies and blocking

```bash
bd dep add <issue> <depends-on>       # issue is blocked by depends-on
bd blocked                            # Show all blocked issues
bd dep tree <id>                      # Visualize dependency graph
```

`bd ready` only shows issues with no open blockers.

### Other useful commands

```bash
bd list                               # Open issues
bd list --all                         # Include closed
bd search "keyword"                   # Full-text search
bd children <epic-id>                 # Show child issues
bd comments add <id> "note"           # Add a comment
bd update <id> --append-notes "..."   # Append to working memory
```

### Rules

- Track all work in bd — no markdown TODOs, no ad-hoc task lists
- Check `bd ready` before starting work
- Claim before working: `bd update <id> --claim`
- Always run `make check` before closing a task
- Link discovered work with `--deps discovered-from:<id>`
- Do NOT use `bd edit` without the stdin pipe pattern (it opens vim)
<!-- END BEADS INTEGRATION -->

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
