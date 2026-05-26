# FLS - File Sharing System

## Quick Start

```powershell
# Build
task build

# Run (first run prompts for admin password on stdin)
task run -- --port 8080 --data-dir ./data

# Dev: auto-provides password "admin123", uses test-data/
task dev
```

## Commands

| Action | Command |
|--------|---------|
| Build (current platform) | `task build` |
| Build (Windows cross) | `task build:windows` |
| Build (Linux cross) | `task build:linux` |
| Run tests (all) | `task test` |
| Run tests (single) | `go test ./... -run TestIntegration_Dashboard -v` |
| Dev server (auto password) | `task dev` |
| Start server (manual) | `task run` |
| Clean artifacts | `task clean` |
| List all tasks | `task --list` |

## Architecture

### Entry Point: `main.go`

Wires up: logger → recovery → RealIP → security headers → session → route groups.

### Route Groups (middleware differs per group)

| Group | RateLimit | CSRF | Auth | Routes |
|-------|-----------|------|------|--------|
| Health | No | No | No | `GET /health` |
| Static | No | No | No | `GET /static/*` |
| Login | LoginRate | Yes | No | `GET/POST /login` |
| Public share | APIRate | No | No | `GET/POST /s/{token}` + `/raw`, `/download` |
| Logout | No | No | No | `POST /logout` |
| Admin | APIRate | Yes | Yes | `GET /admin/*`, `/admin/files/*`, `/admin/shares/*`, `/admin/config` |
| API | APIRate | No | Yes | `GET /admin/api/stats` |
| Upload | APIRate | No | Yes | `POST /api/upload/simple` + TUS endpoints |

### Middleware Order (in `main.go`)

```
chimw.Logger → RecoveryMiddleware → chimw.RealIP → SecurityHeadersMiddleware → sessionManager.LoadAndSave
```
Then per-group: RateLimitMiddleware → (optional) CSRFMiddleware → (optional) AuthMiddleware → handler.

### Route ↔ Handler Map

| Path | Handler | Key Method |
|------|---------|------------|
| `/login` | `LoginHandler` | `GetLogin`, `PostLogin` |
| `/admin/` | `DashboardHandler` | `GetDashboard` |
| `/admin/files` | `FileHandler` | `ListFiles`, `GetFile`, `DeleteFile`, `EditFile`, `UpdateFile` |
| `/admin/shares` | `ShareHandler` | `ListShares`, `NewShareForm`, `CreateShare`, `GetShare`, `DeleteShare`, `QRCode` |
| `/admin/config` | `ConfigHandler` | `GetConfig`, `UpdateConfig` |
| `/admin/api/stats` | `StatsHandler` | `GetStats` |
| `/s/{token}*` | `DownloadHandler` | `ServeShare`, `VerifySharePassword`, `RawContent`, `DownloadFile` |
| `/api/upload/*` | `tus.Handler` | `SimpleUpload`, TUS handlers |

### Security Headers

CSP in `internal/middleware/security.go` (line 45). **Must** include `'unsafe-inline'` and `'unsafe-hashes'` in `script-src` — templates use inline `<script>` blocks and HTML event handlers (`onclick`, `onchange`, `ondragover`, etc.). Without these, the upload dropzone and all JS interactions silently fail.

Current CSP directives:
```
default-src 'self'
script-src 'self' https://unpkg.com 'unsafe-inline' 'unsafe-hashes'
style-src 'self' https://unpkg.com https://fonts.googleapis.com
img-src 'self' data:
font-src 'self' https://fonts.gstatic.com
form-action 'self'
```

### HX-Redirect Pattern

For HTMX form submissions, handlers set `HX-Redirect` header. Non-HTMX requests must get a proper `303 See Other` fallback. Used in:
- `CreateShare` and `DeleteShare` in `internal/handler/shares.go`
- `DeleteFile` in `internal/handler/files.go`

### Rate Limits

- `LoginRate`: 200 requests/min (configurable in `security.go`)
- `APIRate`: 600 requests/min (configurable in `security.go`)

## Project Structure

```
fls/
├── main.go                    # Entry point, route wiring
├── internal/
│   ├── config/                # Config management (loaded from SQLite)
│   ├── database/              # SQLite connection + migrations
│   │   ├── database.go        # 4 tables: files, shares, config, download_logs
│   │   └── database_test.go
│   ├── handler/               # HTTP handlers (15 files)
│   │   ├── error.go           # 404, 405, 500 error pages + RecoveryMiddleware
│   │   ├── render.go          # Template rendering + static file serving
│   │   ├── login.go           # Login page + POST handler
│   │   ├── shares.go          # Share CRUD + QR code generation
│   │   ├── files.go           # File CRUD
│   │   ├── download.go        # Public share pages + file download
│   │   ├── dashboard.go       # Admin dashboard
│   │   ├── config.go          # System config page
│   │   ├── stats.go           # Stats API
│   │   └── integration_test.go # Full-stack integration tests
│   ├── middleware/
│   │   ├── auth.go            # SCS session auth + SetAuthenticated/ClearAuthenticated
│   │   └── security.go        # CSP, CSRF (nosurf), rate limiting, path validation
│   ├── model/
│   │   ├── config.go          # ConfigItem
│   │   ├── file.go            # File
│   │   ├── share.go           # Share (with IsExpired, IsPasswordCorrect, IsTextShare, IsFileShare)
│   │   ├── errors.go          # ErrNotFound, ErrInvalidInput, ErrUnauthorized
│   │   └── share_test.go
│   ├── service/
│   │   ├── auth.go            # Password verification + first-run wizard
│   │   ├── share.go           # CreateFileShare, CreateTextShare, ListShares, etc.
│   │   ├── stats.go           # Dashboard stats
│   │   └── *_test.go          # Per-service tests
│   └── tus/                   # Upload handlers (separate package!)
│       ├── handler.go         # SimpleUpload (multipart) + TUS protocol
│       └── tus_handler.go     # TUS protocol implementation
├── web/
│   ├── embed.go               # //go:embed templates/*.html static/*
│   ├── templates/             # 14 .html templates
│   │   ├── layout.html        # Base layout + nav
│   │   ├── login.html         # Login form
│   │   ├── dashboard.html     # Admin dashboard
│   │   ├── share-detail.html  # New share form + share detail
│   │   ├── shares.html        # Share list
│   │   ├── files.html         # File list
│   │   ├── download*.html     # Public share pages (6 variants)
│   │   ├── error.html         # 404/405 error page
│   │   └── config.html        # System config page
│   └── static/
│       └── custom.css         # Custom styles
├── Taskfile.yml               # Taskfile build system (used via `task`)
├── mise.toml                  # mise tool version management
└── AGENTS.md                  # This file
```

## Database

SQLite via `modernc.org/sqlite` (pure Go, no CGO). Connection string:
```
sqlite:<path>?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)
```

**Critical: `SetMaxOpenConns(1)`** — SQLite does not support concurrent writes.

### Tables

- **files**: `id TEXT PK`, `filename`, `original_name`, `size`, `mime_type`, `storage_path`, `created_at`, `updated_at`
- **shares**: `id TEXT PK`, `file_id TEXT FK→files`, `token TEXT UNIQUE`, `password_hash`, `expires_at`, `max_downloads`, `download_count`, `content_type` ('file'|'text'), `text_content`, timestamps
- **config**: `key TEXT PK`, `value TEXT`, `updated_at`
- **download_logs**: `id TEXT PK`, `share_id TEXT FK→shares`, `ip_address`, `user_agent`, `downloaded_at`

Migrations run automatically on startup via `db.Migrate()` in `database.go`.

## Templates

Templates are embedded at compile time via `//go:embed` (`web/embed.go`). Rendering via `RenderTemplate(w, name, data)` in `render.go`. Template data is always `map[string]interface{}`.

**Every template depends on `layout.html`** (defines nav + footer). The `layout.html` template checks `.Authenticated` to show/hide nav links. All admin page templates must pass `"Authenticated": true` in their data map.

**Error pages** use `ErrorData` struct (`internal/handler/error.go`) which **must** include `Authenticated bool`.

**CSRF token** is injected per-request via `middleware.CSRFToken(r)` and passed in template data as `"CSRFToken"`.

## Testing

Tests use a full in-memory server (`setupTestServer()` in `integration_test.go`). Pattern:

```go
client, baseURL, cleanup := setupTestServer(t)
defer cleanup()
var cookie string
loginAsAdmin(t, client, baseURL, &cookie)
// Use client with auto-managed cookies
```

Key testing facts:
- Tests **bypass CSRF** via a `/test-login` endpoint (only registered in test setup, not in production)
- `loginAsAdmin()` helper extracts SCS session cookie for manual request auth
- `noRedirectClient()` helper returns HTTP 3xx status codes instead of following redirects
- Test server seeds: 1 admin password, 1 test file, 1 test share (token `testtokn`)
- Integration tests cover: login, dashboard, stats API, public share page, file download, 404, health check, logout session clearance
- Per-handler tests exist for: `shares_test.go`, `files_test.go`, `download_test.go`, `stats_test.go`, `health_test.go`
- No need for external test services — everything is in-memory

## Gotchas

1. **CSP blocks inline JS out of the box.** Templates use `onclick`, `onchange`, `ondragover` HTML attributes and inline `<script>` blocks. The CSP header in `security.go` must include `'unsafe-inline'` and `'unsafe-hashes'` in `script-src` for these to work. Without this, the upload dropzone, file selection, and type toggling all silently fail.

2. **Upload handlers are in `internal/tus/` package**, not in `internal/handler/`. Routes are mounted via `tusHandler.Mount()`.

3. **Form submissions target both HTMX and non-HTMX.** Handlers that redirect after POST must check `HX-Request` header and either set `HX-Redirect` header (for HTMX) or return 303 (for regular forms).

4. **ErrorData struct needs Authenticated field.** Without it, the layout template aborts mid-render because it accesses `.Authenticated`.

5. **SQLite with `SetMaxOpenConns(1)`.** Concurrent database access from goroutines uses a single connection. Do not open multiple database connections.

6. **New share form uses a hidden `file_id` input** populated by JavaScript (upload or existing file select). Without a file uploaded/selected, the form submission fails with `"file_id is required for file share"`.

7. **First-run password wizard reads from stdin.** For automated dev setup, pipe password into stdin or use `task dev` which auto-provides "admin123".

8. **Session cookie is named `"session"`.** Managed by SCS (`github.com/alexedwards/scs/v2`). Session lifetime is configurable via the admin config page.
