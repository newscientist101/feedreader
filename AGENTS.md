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

Look up real user credentials with:
```bash
sqlite3 db.sqlite3 "SELECT external_id, email FROM users;"
```

<!-- BEGIN BEADS INTEGRATION -->
## Issue Tracking with bd (beads)

**IMPORTANT**: This project uses **bd (beads)** for ALL issue tracking. Do NOT use markdown TODOs, task lists, or other tracking methods.

### Why bd?

- Dependency-aware: Track blockers and relationships between issues
- Git-friendly: Dolt-powered version control with native sync
- Agent-optimized: JSON output, ready work detection, discovered-from links
- Prevents duplicate tracking systems and confusion

### Quick Start

**Check for ready work:**

```bash
bd ready --json
```

**Create new issues:**

```bash
bd create "Issue title" --description="Detailed context" -t bug|feature|task -p 0-4 --json
bd create "Issue title" --description="What this issue is about" -p 1 --deps discovered-from:bd-123 --json
```

**Claim and update:**

```bash
bd update <id> --claim --json
bd update bd-42 --priority 1 --json
```

**Complete work:**

```bash
bd close bd-42 --reason "Completed" --json
```

### Issue Types

- `bug` - Something broken
- `feature` - New functionality
- `task` - Work item (tests, docs, refactoring)
- `epic` - Large feature with subtasks
- `chore` - Maintenance (dependencies, tooling)

### Priorities

- `0` - Critical (security, data loss, broken builds)
- `1` - High (major features, important bugs)
- `2` - Medium (default, nice-to-have)
- `3` - Low (polish, optimization)
- `4` - Backlog (future ideas)

### Workflow for AI Agents

1. **Check ready work**: `bd ready` shows unblocked issues
2. **Claim your task atomically**: `bd update <id> --claim`
3. **Work on it**: Implement, test, document
4. **Discover new work?** Create linked issue:
   - `bd create "Found bug" --description="Details about what was found" -p 1 --deps discovered-from:<parent-id>`
5. **Complete**: `bd close <id> --reason "Done"`

### Auto-Sync

bd automatically syncs via Dolt:

- Each write auto-commits to Dolt history
- Use `bd dolt push`/`bd dolt pull` for remote sync
- No manual export/import needed!

### Important Rules

- ✅ Use bd for ALL task tracking
- ✅ Always use `--json` flag for programmatic use
- ✅ Link discovered work with `discovered-from` dependencies
- ✅ Check `bd ready` before asking "what should I work on?"
- ❌ Do NOT create markdown TODO lists
- ❌ Do NOT use external issue trackers
- ❌ Do NOT duplicate tracking systems

For more details, see README.md and docs/QUICKSTART.md.

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
   bd sync
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
