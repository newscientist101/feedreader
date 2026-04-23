# Configuration Reference

FeedReader is configured through a TOML file, CLI flags, and environment
variables. This document covers every option.

---

## Configuration File

**Default location:** `config.toml` in the working directory.

Override with the `--config` flag or `CONFIG_FILE` environment variable.

Start from the example:

```bash
cp config.example.toml config.toml
$EDITOR config.toml
```

Or generate a config interactively:

```bash
./feedreader init
```

---

## Precedence

From highest to lowest priority:

1. **CLI flags** (`--listen`, `--db`, `--email-domain`)
2. **Environment variables** (`CONFIG_FILE`)
3. **Config file** (`config.toml`)
4. **Defaults** (listed below)

Flags only override config values when explicitly set on the command line.

---

## Environment Variable Overrides

| Variable | Description |
|---|---|
| `CONFIG_FILE` | Path to the TOML config file (overrides `--config` flag) |
| `DEV` | Set to any non-empty value to skip authentication (development only) |

---

## Config Reference

### Top-level

| Field | CLI flag | Type | Default | Description |
|---|---|---|---|---|
| `listen` | `--listen` | string | `:8000` | TCP address to listen on (e.g. `0.0.0.0:8000`) |
| `db` | `--db` | string | `db.sqlite3` | Path to the SQLite database file |
| `email_domain` | `--email-domain` | string | hostname | Email domain for newsletter addresses |

```toml
listen = ":8000"
db = "/data/feedreader.db"
email_domain = "feeds.example.com"
```

---

### `[auth]`

| Field | Type | Description |
|---|---|---|
| `provider` | string | Auth provider name (see table below) |

```toml
[auth]
provider = "authelia"
```

Supported provider names:

| Value | Description |
|---|---|
| `proxy` | Generic reverse proxy headers (configurable) |
| `tailscale` | Tailscale Serve/Funnel |
| `cloudflare` | Cloudflare Tunnel + Access |
| `authelia` | Authelia forward auth |
| `oauth2_proxy` | OAuth2 Proxy |
| `exedev` | Legacy exe.dev platform |

If no provider is configured, the first-run setup UI is shown.

---

### `[auth.proxy]`

Used when `provider = "proxy"`. Configure the header names your reverse proxy
injects.

| Field | Type | Default | Description |
|---|---|---|---|
| `user_id_header` | string | `Remote-User` | Header containing the stable user identifier |
| `email_header` | string | `Remote-Email` | Header containing the user's email address |

```toml
[auth]
provider = "proxy"

[auth.proxy]
user_id_header = "Remote-User"
email_header = "Remote-Email"
```

---

### `[auth.cloudflare]`

Used when `provider = "cloudflare"`. Validates the Cloudflare Access JWT.

| Field | Type | Required | Description |
|---|---|---|---|
| `team_domain` | string | yes | Cloudflare team domain (e.g. `myteam` for `myteam.cloudflareaccess.com`) |
| `audience` | string | no | Application audience (AUD) tag for JWT validation |

```toml
[auth]
provider = "cloudflare"

[auth.cloudflare]
team_domain = "myteam"
audience = "abc123..."
```

---

### `[newsletter]`

| Field | Type | Default | Description |
|---|---|---|---|
| `webhook_secret` | string | `""` | Shared secret for the HTTP webhook ingest endpoint |

```toml
[newsletter]
webhook_secret = "a-random-secret"
```

---

### `[newsletter.smtp]`

Built-in SMTP server for receiving newsletters via email delivery.

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | bool | `false` | Start the built-in SMTP server |
| `listen` | string | `:2525` | Address for the SMTP server to listen on |

```toml
[newsletter.smtp]
enabled = true
listen = ":2525"
```

The SMTP server has no authentication or TLS — it is intended for
localhost delivery from a local MTA (e.g. Postfix, nullmailer) or
Docker-internal networks only.

---

## Auth Provider Setup Guides

### `proxy` — Generic Reverse Proxy

Use this with any reverse proxy that injects identity headers after
authentication (Nginx, Caddy, Traefik, HAProxy, etc.).

**How it works:** The proxy authenticates the user and injects two headers:
one containing a stable user ID, one containing the email address.
FeedReader reads these headers to identify the current user.

**Required config:**

```toml
[auth]
provider = "proxy"

[auth.proxy]
user_id_header = "Remote-User"   # or whatever your proxy uses
email_header = "Remote-Email"
```

**Security:** Ensure the proxy strips these headers from incoming requests
before injecting its own, so clients cannot spoof them.

---

### `tailscale` — Tailscale Serve/Funnel

Use when running feedreader behind [Tailscale Serve](https://tailscale.com/kb/1312/serve)
or [Tailscale Funnel](https://tailscale.com/kb/1223/funnel).

**How it works:** Tailscale injects `Tailscale-User-Login`, `Tailscale-User-Name`,
and `Tailscale-User-Profile-Pic` headers for authenticated Tailscale users.

**No additional config needed:**

```toml
[auth]
provider = "tailscale"
```

**Setup:** Run `tailscale serve https / http://localhost:8000` to expose
FeedReader through Tailscale. Users must be members of your tailnet.

---

### `cloudflare` — Cloudflare Tunnel + Access

Use when running feedreader behind [Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/)
protected by [Cloudflare Access](https://developers.cloudflare.com/cloudflare-one/policies/access/).

**How it works:** Cloudflare Access issues a signed JWT on the
`Cf-Access-Jwt-Assertion` header. FeedReader validates the JWT signature
against the JWKS endpoint at `https://<team_domain>.cloudflareaccess.com/cdn-cgi/access/certs`.

**Required config:**

```toml
[auth]
provider = "cloudflare"

[auth.cloudflare]
team_domain = "myteam"             # your Cloudflare Access team name
audience = "abc123def456..."       # AUD tag from the Access application
```

Find the AUD tag in your Cloudflare Zero Trust dashboard under
Access > Applications > your app > Overview.

---

### `authelia` — Authelia Forward Auth

Use when running feedreader behind [Authelia](https://www.authelia.com/).
This is the default in the Docker Compose deployment (`DEPLOY.md`).

**How it works:** Authelia validates the session and injects `Remote-User`
and `Remote-Email` headers. No additional config is needed because Authelia
uses the same default header names as the `proxy` provider.

```toml
[auth]
provider = "authelia"
```

See [DEPLOY.md](DEPLOY.md) for the full Docker Compose setup with Caddy + Authelia.

---

### `oauth2_proxy` — OAuth2 Proxy

Use when running feedreader behind [OAuth2 Proxy](https://oauth2-proxy.github.io/oauth2-proxy/).

**How it works:** OAuth2 Proxy injects `X-Forwarded-User` and
`X-Forwarded-Email` headers after OAuth authentication.

```toml
[auth]
provider = "oauth2_proxy"
```

OAuth2 Proxy supports many upstream providers: Google, GitHub, GitLab,
Microsoft, Keycloak, and more. See the
[OAuth2 Proxy docs](https://oauth2-proxy.github.io/oauth2-proxy/configuration/providers/)
for provider-specific configuration.

---

## Example Configs

### 1. Minimal — Generic proxy headers

For any reverse proxy that injects `Remote-User` / `Remote-Email`:

```toml
listen = ":8000"
db = "db.sqlite3"

[auth]
provider = "proxy"

[auth.proxy]
user_id_header = "Remote-User"
email_header = "Remote-Email"
```

---

### 2. Cloudflare Access

```toml
listen = ":8000"
db = "db.sqlite3"

[auth]
provider = "cloudflare"

[auth.cloudflare]
team_domain = "myteam"
audience = "your-application-aud-tag"
```

---

### 3. Authelia (Docker Compose deployment)

Matches the `deploy/docker-compose.yml` setup from [DEPLOY.md](DEPLOY.md):

```toml
listen = ":8000"
db = "/data/feedreader.db"
email_domain = "feeds.yourdomain.com"

[auth]
provider = "authelia"

[newsletter]
webhook_secret = "your-webhook-secret"

[newsletter.smtp]
enabled = false
```

---

### 4. Full example with all options

See `config.example.toml` for an annotated example covering all options.
