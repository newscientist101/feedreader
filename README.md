# FeedReader

A self-hosted feed reader built with Go and SQLite. Supports RSS/Atom feeds,
custom web scrapers for sites without feeds, and AI-powered scraper generation.

![FeedReader screenshot](screenshot.png)

## Features

- **RSS/Atom feeds** — subscribe to standard feeds with conditional GET support
- **Custom scrapers** — CSS-selector-based scrapers for sites without RSS
- **AI scraper generation** — paste a URL and let Claude generate the scraper config
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
- **Usenet newsgroups** — read-only text newsgroup access via Eternal September (optional, operator-enabled)

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

FeedReader expects authentication headers on each request:

| Header | Description |
|---|---|
| `X-Exedev-Userid` | Unique user identifier |
| `X-Exedev-Email` | User's email address |

In production, a reverse proxy injects these headers. For local development,
use a proxy like [mitmdump](https://mitmproxy.org/) to add them:

```bash
mitmdump \
  --mode reverse:http://localhost:8000 \
  --listen-port 3000 \
  --set modify_headers='/~q/X-Exedev-Email/you@example.com' \
  --set modify_headers='/~q/X-Exedev-Userid/user-1'
```

Then open `http://localhost:3000/`.

A new user record is created automatically on first login.

### Configuration

Optional environment variables (or `.env` file):

| Variable | Description |
|---|---|
| `SHELLEY_URL` | Shelley API URL for AI scraper generation (default: `http://localhost:9999`) |
| `USENET_ENABLED` | Set to `true` to enable the Usenet newsgroup reader. Requires `USENET_CREDENTIAL_KEY`. |
| `USENET_CREDENTIAL_KEY` | Base64-encoded 32-byte key for encrypting per-user NNTP credentials (required when `USENET_ENABLED=true`). **Never commit this value to git.** See [Usenet Setup](#usenet-newsgroup-reader) for operator instructions. |

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
  auth.go                  Auth middleware
  filter.go                Folder exclusion-rule filtering
  content_filter.go        Per-feed content transform filters
  category_tree.go         Nested folder tree builder
  retention.go             Data retention / cleanup
  ai_scraper.go            Claude API integration for scraper generation
  feeds/                   RSS/Atom fetcher and parser
  scrapers/                CSS-selector and JSON API scrapers
  huggingface/             Hugging Face model feed source
  opml/                    OPML import/export
  email/                   Newsletter SMTP receiver
  nntp/                    NNTP protocol client (no DB dependency)
  usenet/                  Usenet helpers: crypto, group/credential validation, article mapping
  handler_usenet.go        Usenet API handler methods
  templates/               Server-rendered HTML templates
  static/                  CSS, JS, icons
    app.js                 JS entry point
    modules/               ES modules (each with a .test.js companion)
    style.css              Styles
db/
  db.go                    Database setup, migrations, pragmas
  migrations/              Numbered SQL migrations (001–024)
  queries/                 SQL queries for sqlc
  dbgen/                   Generated Go code (do not edit)
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

### AI-Powered Generation

1. Ensure the Shelley API is running (or set `SHELLEY_URL` in your `.env` file)
2. Go to **Scrapers** → click **AI Generate**
3. Enter the URL and describe what to extract
4. Review and save the generated configuration

### Manual Configuration

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
- `POST /api/ai/generate-scraper` — AI generate config
- `GET /api/ai/status` — AI availability

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

### Usenet (requires `USENET_ENABLED=true`)
- `GET /api/usenet/credentials` — credential status (never returns password)
- `PUT /api/usenet/credentials` — save username/password
- `DELETE /api/usenet/credentials` — remove credentials
- `GET /api/usenet/groups` — list subscribed newsgroups
- `POST /api/usenet/groups` — subscribe to a newsgroup
- `DELETE /api/usenet/groups/{feed_id}` — unsubscribe
- `GET /api/usenet/articles/{article_id}/thread` — thread context for a Usenet article

</details>

## Deployment

FeedReader is a single binary + SQLite database. Deploy however you like.

Example with systemd:

```bash
# Copy and edit the service file
sudo cp srv.service /etc/systemd/system/feedreader.service
sudo systemctl daemon-reload
sudo systemctl enable --now feedreader

# Restart after updates
make build && sudo systemctl restart feedreader
```

Put a reverse proxy (nginx, Caddy, etc.) in front to handle TLS and
inject the authentication headers.

## Usenet Newsgroup Reader

FeedReader optionally supports read-only Usenet newsgroup access via
[Eternal September](https://www.eternal-september.org/). Per-user NNTP
credentials are encrypted at rest using an operator-managed key.

### Features and Limitations

- **Read-only** — articles can only be read, not posted or replied to.
- **Text-only** — only `text/plain` articles are ingested. Binary posts,
  multipart posts, attachments, yEnc-encoded content, and base64-encoded
  bodies are rejected automatically at ingestion time.
- **No binary groups** — newsgroups with `binary` or `binaries` in any
  segment, and all `alt.binaries.*` groups, are rejected at subscription time.
- **Manual subscription** — groups are added by exact lowercase name
  (e.g. `comp.lang.go`). There is no group browser or search.
- **Fixed provider** — [Eternal September](https://www.eternal-september.org/)
  is the only supported provider (`news.eternal-september.org:563`, TLS).
- **First-fetch limit** — the first fetch for a new group imports at most the
  latest 100 articles. Subsequent fetches pick up from where the previous
  one left off, capped at 500 articles per run.
- **Thread context** — Message-ID / References headers are parsed to build
  parent/root relationships. The article page shows all other messages in
  the same thread. Article lists show a reply indicator for non-root messages.
- **Integrated experience** — Usenet newsgroups appear alongside RSS feeds
  in folders, the sidebar, and article lists. Read/unread, star, queue,
  history, search, exclusion filters, and retention all work for Usenet
  articles without any extra setup.

### Using Usenet (as a user)

1. **Configure credentials** — go to **Settings** and find the
   *Usenet (Eternal September)* section. Enter your Eternal September
   username and password and click **Save**. Credentials are stored
   encrypted; the password is never returned by the API.

2. **Subscribe to a newsgroup** — go to **Feeds** and find the
   *Usenet Newsgroups* section. Enter an exact group name (e.g.
   `comp.lang.go`), optionally choose a folder, and click **Subscribe**.
   The background worker will import recent articles on the next scheduled
   run (or immediately if the feed is manually refreshed).

3. **Read articles** — subscribed newsgroups appear in the sidebar and
   article list alongside your RSS feeds. Usenet articles display their
   content inside a `<pre>` block to preserve plain-text formatting.
   Threading context (other messages in the same thread) is shown on the
   individual article page.

4. **Remove a newsgroup** — click the **×** next to a group in the
   *Usenet Newsgroups* section on the Feeds page, or use the standard
   feed delete flow.

### Troubleshooting

| Symptom | Likely cause |
|---|---|
| Usenet sections not visible in Settings or Feeds | `USENET_ENABLED` is not set to `true`, or the server was not restarted after the change. |
| "Usenet is not enabled on this server" (503) | Same as above — check the service environment. |
| "Usenet credentials not configured" on group subscription | Save your Eternal September credentials in Settings first. |
| No articles appear after subscribing | The background fetch may not have run yet. Use the manual **Refresh** action on the feed. Also check the feed's error status via the feed list. |
| Authentication error on fetch | Your Eternal September username or password is incorrect, or the account is not activated. Verify credentials at eternal-september.org. |
| Group rejected with "binary groups are not allowed" | The group name contains `binary`/`binaries` or matches `alt.binaries.*`. FeedReader does not support binary groups. |
| Articles not appearing despite successful fetch | The articles may have been rejected as binary content (binary subjects, non-text MIME types, yEnc markers). Check the server logs for ingestion details. |

### Enabling Usenet (operator)

Set `USENET_ENABLED=true` in the service environment. The application will
**refuse to start** unless `USENET_CREDENTIAL_KEY` is also set to a valid
32-byte base64-encoded key.

### Generating the Credential Encryption Key

The key protects per-user Eternal September passwords stored in the database.
It is an operator/server secret — different in nature from per-user secrets
like the YouTube API key in user settings.

**Generate a new key (do this once on the server):**

```bash
# Generate 32 random bytes and base64-encode them
openssl rand -base64 32
# or with Python:
python3 -c "import secrets, base64; print(base64.b64encode(secrets.token_bytes(32)).decode())"
```

Example output (do not use this value — generate your own):
```
K7gNU3sdo+OL0wNhqoVWhr3g6s1xYv72ol/pe/Unols=
```

### Storing the Key Securely (systemd EnvironmentFile)

Store the key in a root-readable file outside of the repository. **Do not
add it to `.env` or any file committed to git.**

```bash
# Create the secrets file (readable only by root)
sudo install -m 600 -o root -g root /dev/null /etc/feedreader/secrets

# Add the key (replace the value with your generated key)
echo 'USENET_CREDENTIAL_KEY=K7gNU3sdo+OL0wNhqoVWhr3g6s1xYv72ol/pe/Unols=' \
  | sudo tee /etc/feedreader/secrets > /dev/null

# Also add USENET_ENABLED
echo 'USENET_ENABLED=true' | sudo tee -a /etc/feedreader/secrets > /dev/null
```

Reference the file from a systemd drop-in so the service reads it:

```bash
# Create a drop-in override for the feedreader service
sudo mkdir -p /etc/systemd/system/feedreader.service.d
sudo tee /etc/systemd/system/feedreader.service.d/usenet.conf > /dev/null <<'EOF'
[Service]
EnvironmentFile=/etc/feedreader/secrets
EOF

sudo systemctl daemon-reload
sudo systemctl restart feedreader
```

> **Note:** The root `.env` file referenced in `srv.service` uses the `-` prefix
> (`EnvironmentFile=-/home/exedev/feedreader/.env`) so a missing file is
> non-fatal. The secrets drop-in omits `-` so a missing file causes a startup
> failure — intentional, to avoid silently running without the key.

### Key Version and Rotation

The current credential schema uses `key_version = "v1"`. Key rotation is a
manual process and is not yet automated:

1. Generate a new key following the steps above.
2. Re-encrypt all credentials in the database using the new key (no automated
   tooling yet — a future task will add a migration helper).
3. Replace the key in `/etc/feedreader/secrets` and restart the service.

Until rotation tooling is available, treat the initial key as long-lived and
store it durably (e.g. in a password manager).

## License

[CC0](LICENSE)
