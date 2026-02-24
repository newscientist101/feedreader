# FeedReader

A modern feed reader with RSS/Atom support and a modular scraper system for non-RSS sources.

## Features

- **RSS/Atom Feeds**: Subscribe to standard RSS and Atom feeds
- **Folders/Categories**: Organize feeds into folders with OPML import/export
- **Custom Scrapers**: Create scrapers for websites without RSS feeds
- **AI-Powered Config**: Let Claude analyze a page and generate scraper configuration
- **Exclusion Rules**: Filter out articles by keyword or author per folder
- **Data Retention**: Automatic cleanup of old articles (starred items preserved)
- **Multiple Views**: Card, List, Magazine, and Expanded views
- **Reading Queue**: Save articles to read later
- **Reading History**: Track recently read articles
- **Newsletter Support**: Receive newsletters as feed items via email
- **Responsive UI**: Works on desktop, tablet, and mobile

## Building and Running

```bash
# Build
make build

# Run
./feedreader
```

The server listens on port 8000 by default.

## Viewing the App

The app requires exe.dev authentication headers (`X-Exedev-Userid` and
`X-Exedev-Email`). Without them, all non-static requests redirect to
`/__exe.dev/login`.

### Via the exe.dev proxy (production)

Access `https://lynx-fairy.exe.xyz:8000/` in a browser where you are
logged into exe.dev. The proxy injects the auth headers automatically.

### Local development (Shelley / browser tool)

The built-in browser tool cannot authenticate through the exe.dev proxy.
Use **mitmdump** as a reverse proxy on a second port to inject the
required headers:

```bash
mitmdump \
  --mode reverse:http://localhost:8000 \
  --listen-port 3000 \
  --set modify_headers='/~q/X-Exedev-Email/dev@localhost' \
  --set modify_headers='/~q/X-Exedev-Userid/dev-user-1'
```

Then browse to `http://localhost:3000/`.

To view as a specific existing user, look up their credentials:

```bash
sqlite3 db.sqlite3 "SELECT external_id, email FROM users;"
```

Then use the matching `external_id` and `email` in the headers:

```bash
mitmdump \
  --mode reverse:http://localhost:8000 \
  --listen-port 3000 \
  --set modify_headers='/~q/X-Exedev-Email/<EMAIL>' \
  --set modify_headers='/~q/X-Exedev-Userid/<EXTERNAL_ID>'
```

**Note:** When viewing the app (e.g. for development or testing), always
use the real user's credentials from the database so you see their actual
feeds, folders, and read state.

## Configuration

Create a `.env` file for environment variables:

```bash
# For AI-powered scraper generation (optional)
ANTHROPIC_API_KEY=your-api-key-here
```

## Running as a systemd service

```bash
# Install the service file
sudo cp srv.service /etc/systemd/system/feedreader.service

# Reload systemd and enable the service
sudo systemctl daemon-reload
sudo systemctl enable feedreader.service

# Start the service
sudo systemctl start feedreader

# Check status
systemctl status feedreader

# View logs
journalctl -u feedreader -f
```

To restart after code changes:

```bash
make build
sudo systemctl restart feedreader
```

## Scraper System

### AI-Powered Generation

1. Set `ANTHROPIC_API_KEY` in your `.env` file
2. Go to Scrapers page
3. Enter the URL and describe what you want to extract
4. Click "Generate Scraper Config"
5. Review and use the generated configuration

### Manual Configuration

Create a JSON config with CSS selectors:

```json
{
  "itemSelector": "article.post",
  "titleSelector": "h2.title",
  "urlSelector": "a.permalink",
  "urlAttr": "href",
  "summarySelector": "p.summary",
  "imageSelector": "img.thumb",
  "imageAttr": "src",
  "dateSelector": "time",
  "dateAttr": "datetime",
  "baseUrl": "https://example.com"
}
```

### Available Selectors

- `itemSelector` - CSS selector for each item container (required)
- `titleSelector` - Selector for title element (uses text content)
- `urlSelector` - Selector for link element
- `urlAttr` - Attribute to extract URL from (default: `href`)
- `summarySelector` - Selector for description (optional)
- `authorSelector` - Selector for author name (optional)
- `imageSelector` - Selector for image element (optional)
- `imageAttr` - Attribute to extract image URL from (default: `src`)
- `dateSelector` - Selector for date element (optional)
- `dateAttr` - Attribute to extract date from (uses text content if empty)
- `baseUrl` - Base URL for relative links

## Folder Exclusion Rules

Filter unwanted content per folder:

1. Hover over a folder → click ⚙️ gear icon
2. Add keyword or author exclusion rules
3. Supports plain text or regex patterns

## API Endpoints

### Feeds
- `GET /api/feeds/{id}` - Get feed details
- `GET /api/feeds/{id}/articles` - Get feed articles
- `GET /api/feeds/{id}/status` - Get feed fetch status
- `POST /api/feeds` - Create feed
- `PUT /api/feeds/{id}` - Update feed
- `DELETE /api/feeds/{id}` - Delete feed
- `POST /api/feeds/{id}/refresh` - Refresh feed
- `POST /api/feeds/{id}/category` - Set feed category
- `POST /api/feeds/{id}/read-all` - Mark all feed articles read

### Articles
- `GET /api/articles/unread` - Get unread articles
- `POST /api/articles/{id}/read` - Mark as read
- `POST /api/articles/{id}/unread` - Mark as unread
- `POST /api/articles/{id}/star` - Toggle star
- `POST /api/articles/batch-read` - Batch mark as read
- `POST /api/articles/read-all` - Mark all articles read
- `GET /api/search` - Search articles

### Categories
- `POST /api/categories` - Create category
- `PUT /api/categories/{id}` - Update category
- `DELETE /api/categories/{id}` - Delete category
- `GET /api/categories/{id}/articles` - Get category articles
- `POST /api/categories/reorder` - Reorder categories
- `POST /api/categories/{id}/parent` - Set parent category
- `POST /api/categories/{id}/read-all` - Mark all category articles read
- `GET /api/categories/{id}/exclusions` - List exclusion rules
- `POST /api/categories/{id}/exclusions` - Create exclusion rule
- `DELETE /api/exclusions/{id}` - Delete exclusion rule

### Queue & History
- `GET /api/queue` - List queued articles
- `POST /api/articles/{id}/queue` - Toggle queue status
- `DELETE /api/articles/{id}/queue` - Remove from queue

### OPML
- `GET /api/opml/export` - Export as OPML
- `POST /api/opml/import` - Import OPML file

### Scrapers
- `GET /api/scrapers/{id}` - Get scraper
- `POST /api/scrapers` - Create scraper
- `PUT /api/scrapers/{id}` - Update scraper
- `DELETE /api/scrapers/{id}` - Delete scraper
- `POST /api/ai/generate-scraper` - Generate config with AI
- `GET /api/ai/status` - Check AI availability

### Newsletter
- `GET /api/newsletter/address` - Get newsletter email address
- `POST /api/newsletter/generate-address` - Generate new newsletter address

### Settings & Counts
- `GET /api/settings` - Get user settings
- `PUT /api/settings` - Update user settings
- `GET /api/counts` - Get unread/starred/feed counts
- `GET /api/favicon` - Fetch site favicon
- `GET /api/retention/stats` - Retention statistics
- `POST /api/retention/cleanup` - Run cleanup

## Database

SQLite database (`db.sqlite3`) with migrations in `db/migrations/`.

## Code Layout

- `cmd/srv/` - Main binary entrypoint
- `srv/` - HTTP server, handlers, templates
- `srv/feeds/` - RSS/Atom parser and fetcher
- `srv/scrapers/` - Custom scraper engine
- `srv/opml/` - OPML import/export
- `srv/email/` - Newsletter email receiving
- `srv/huggingface/` - Hugging Face feed source
- `srv/templates/` - HTML templates
- `srv/static/` - CSS and JavaScript
- `db/` - Database, migrations, queries
