# FeedReader

A self-hosted feed reader built with Go and SQLite. Supports RSS/Atom feeds
and custom web scrapers for sites without feeds.

![FeedReader screenshot](screenshot.png)

## Features

- **RSS/Atom feeds** — subscribe to standard feeds with conditional GET support
- **Custom scrapers** — CSS-selector-based scrapers for sites without RSS
- **Folders & categories** — organize feeds into nested folders
- **Multiple views** — card, list, magazine, and expanded layouts
- **Reading queue** — save articles to read later
- **Reading history** — track recently read articles
- **Exclusion rules** — filter articles by keyword or author per folder
- **OPML import/export** — migrate to and from other feed readers
- **Newsletter support** — receive newsletters as feed items via SMTP
- **Data retention** — automatic cleanup of old articles (starred items preserved)
- **Multi-user** — each user gets their own feeds, folders, and settings
- **Responsive UI** — works on desktop, tablet, and mobile
- **Offline support** — service worker for offline reading

## Quick Start

### Prerequisites

- Go 1.22+
- Node.js 18+ (for JS tests and linting only)

### Build and Run

```bash
# Install JS dev dependencies (for tests/linting)
npm install

# Build the binary
make build

# Run (listens on port 8000)
./feedreader
```

The database (`db.sqlite3`) is created automatically on first run with all
migrations applied.

### Authentication

FeedReader uses pluggable auth providers. Configure one via `config.toml` or
the interactive setup UI (runs automatically on first start with no config).

Supported providers:

| Provider | Description |
|---|---|
| `proxy` | Generic reverse proxy headers (configurable header names) |
| `tailscale` | Tailscale Serve/Funnel identity headers |
| `cloudflare` | Cloudflare Access with JWT validation |
| `authelia` | Authelia forward auth headers |
| `oauth2_proxy` | OAuth2 Proxy forwarded headers |
| `exedev` | Legacy exe.dev platform (backward compat) |

Example `config.toml`:

```toml
[auth]
provider = "proxy"

[auth.proxy]
user_id_header = "Remote-User"
email_header = "Remote-Email"
```

For local development, run the interactive config wizard:

```bash
./feedreader init
```

A new user record is created automatically on first login.

### Configuration

Configure via `config.toml`, CLI flags, or environment variables.

| Flag | Config key | Default | Description |
|---|---|---|---|
| `--listen` | `listen` | `:8000` | Listen address |
| `--db` | `db` | `db.sqlite3` | SQLite database path |
| `--email-domain` | `email_domain` | (hostname) | Email domain for newsletters |
| `--config` / `CONFIG_FILE` | — | `config.toml` | Config file path |

## Tech Stack

- **Go** standard library `net/http` router — no web framework
- **SQLite** via [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go, no CGO)
- **sqlc** for type-safe SQL queries
- **Server-rendered HTML** with `html/template`
- **Vanilla JS** frontend — native ES modules, no bundler, no framework
- **goquery** for CSS-selector scraping

## Project Structure

```
cmd/srv/main.go            Entry point
srv/
  server.go                HTTP handlers
  auth.go                  Auth middleware (pluggable providers)
  auth_*.go                Auth provider implementations
  setup_page.go            First-run setup UI
  filter.go                Folder exclusion-rule filtering
  content_filter.go        Per-feed content transform filters
  category_tree.go         Nested folder tree builder
  retention.go             Data retention / cleanup
  feeds/                   RSS/Atom fetcher and parser
  scrapers/                CSS-selector and JSON API scrapers
  huggingface/             Hugging Face model feed source
  opml/                    OPML import/export
  email/                   Newsletter SMTP receiver
  templates/               Server-rendered HTML templates
  static/                  CSS, JS, icons
    app.js                 JS entry point
    modules/               ES modules (each with a .test.js companion)
    style.css              Styles
config/
  config.go                TOML config file parsing
db/
  db.go                    Database setup, migrations, pragmas
  migrations/              Numbered SQL migrations (001–015)
  queries/                 SQL queries for sqlc
  dbgen/                   Generated Go code (do not edit)
Dockerfile                 Multi-stage Docker build
docker-compose.yml         Example deployment with OAuth2 Proxy
```

## Development

### Validation

Run all checks before committing:

```bash
make check
```

This runs, in order:

| Step | Command | What it does |
|---|---|---|
| Format | `make fmt-check` | Verify `goimports` formatting |
| Fix | `make fix-check` | Verify `go fix` has nothing to apply |
| Lint | `make lint` | Go (golangci-lint) + JS (eslint) + CSS (stylelint) + HTML (djlint) + template validation |
| Test | `make test` | Go tests + JS tests (vitest) |
| Vulncheck | `make vulncheck` | Scan dependencies for known vulnerabilities |

To auto-fix Go formatting: `make fmt`

### Database Migrations

Migrations in `db/migrations/` are applied automatically on startup.
Add new ones as sequentially numbered `.sql` files.

### sqlc Workflow

Edit SQL in `db/queries/*.sql`, then regenerate:

```bash
go generate ./db/...
```

## Scraper System

Scraper configs use CSS selectors to extract articles from HTML pages:

```json
{
  "itemSelector": "article.post",
  "titleSelector": "h2.title",
  "urlSelector": "a.permalink",
  "urlAttr": "href",
  "summarySelector": "p.summary",
  "authorSelector": "span.author",
  "imageSelector": "img.thumb",
  "imageAttr": "src",
  "dateSelector": "time",
  "dateAttr": "datetime",
  "baseUrl": "https://example.com"
}
```

Only `itemSelector` is required. All other selectors are optional.

## API

All endpoints require authentication headers.

<details>
<summary>Full API reference</summary>

### Feeds
- `GET /api/feeds/{id}` — feed details
- `GET /api/feeds/{id}/articles` — feed articles
- `GET /api/feeds/{id}/status` — fetch status
- `POST /api/feeds` — create feed
- `PUT /api/feeds/{id}` — update feed
- `DELETE /api/feeds/{id}` — delete feed
- `POST /api/feeds/{id}/refresh` — refresh now
- `POST /api/feeds/{id}/category` — set category
- `POST /api/feeds/{id}/read-all` — mark all read

### Articles
- `GET /api/articles/unread` — unread articles
- `POST /api/articles/{id}/read` — mark read
- `POST /api/articles/{id}/unread` — mark unread
- `POST /api/articles/{id}/star` — toggle star
- `POST /api/articles/batch-read` — batch mark read
- `POST /api/articles/read-all` — mark all read
- `GET /api/search` — search articles

### Categories
- `POST /api/categories` — create
- `PUT /api/categories/{id}` — update
- `DELETE /api/categories/{id}` — delete
- `GET /api/categories/{id}/articles` — category articles
- `POST /api/categories/reorder` — reorder
- `POST /api/categories/{id}/parent` — set parent
- `POST /api/categories/{id}/read-all` — mark all read
- `GET /api/categories/{id}/exclusions` — list exclusion rules
- `POST /api/categories/{id}/exclusions` — create rule
- `DELETE /api/exclusions/{id}` — delete rule

### Queue & History
- `GET /api/queue` — list queued articles
- `POST /api/articles/{id}/queue` — toggle queue
- `DELETE /api/articles/{id}/queue` — remove from queue

### OPML
- `GET /api/opml/export` — export feeds
- `POST /api/opml/import` — import OPML file

### Scrapers
- `GET /api/scrapers/{id}` — get scraper
- `POST /api/scrapers` — create scraper
- `PUT /api/scrapers/{id}` — update scraper
- `DELETE /api/scrapers/{id}` — delete scraper

### Newsletter
- `GET /api/newsletter/address` — get address
- `POST /api/newsletter/generate-address` — generate new address

### Settings & Counts
- `GET /api/settings` — get settings
- `PUT /api/settings` — update settings
- `GET /api/counts` — unread/starred/feed counts
- `GET /api/favicon` — fetch site favicon
- `GET /api/retention/stats` — retention statistics
- `POST /api/retention/cleanup` — run cleanup

</details>

## Deployment

FeedReader is a single binary + SQLite database. Deploy however you like.

### Docker

```bash
# Build the image
docker build -t feedreader .

# Run with a config file
docker run -v ./config.toml:/app/config.toml -v ./data:/app/data \
  -p 8000:8000 feedreader
```

See `docker-compose.yml` for a full example with OAuth2 Proxy.

### Systemd

```bash
# Copy and edit the service file
sudo cp srv.service /etc/systemd/system/feedreader.service
sudo systemctl daemon-reload
sudo systemctl enable --now feedreader

# Restart after updates
make build && sudo systemctl restart feedreader
```

Put a reverse proxy (nginx, Caddy, etc.) in front to handle TLS and
inject authentication headers.

## License

[CC0](LICENSE)
