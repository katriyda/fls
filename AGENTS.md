# FLS - File Sharing System

## Commands

| Action | Command |
|--------|---------|
| Build | `task build` |
| Cross (Windows) | `task build:windows` |
| Cross (Linux) | `task build:linux` |
| Run tests (all) | `task test` |
| Single test | `go test ./... -run TestIntegration_Dashboard -v` |
| Dev server | `task dev` |
| Run (no rebuild) | `task run` |
| Clean artifacts | `task clean` |

## Key Architecture

**Entry point**: `main.go` wires `chi.NewRouter()` with global middleware stack `chimw.Logger → RecoveryMiddleware → chimw.RealIP → SecurityHeadersMiddleware → sessionManager.LoadAndSave`, then per-group middleware.

**Route groups** (see `main.go` for exact middleware per group):
| Group | RateLimit | CSRF | Auth |
|-------|-----------|------|------|
| `/health`, `/static/*` | No | No | No |
| `/login` | Login | Yes | No |
| `/s/{token}*` | API | No | No |
| `/logout` | No | No | No |
| `/admin/*`, `/admin/api/stats` | API | Yes | Yes |
| `/api/upload/*` | API | No | Yes |

**Upload handlers** live in `internal/tus/` (not `internal/handler/`), mounted via `tusHandler.Mount()`.

**Templates**: embedded at compile time (`web/embed.go` via `//go:embed`). Every template depends on `layout.html` which reads `.Authenticated` and `.CSRFToken`. `RenderTemplate(w, name, data)` accepts `interface{}` — handlers pass either typed structs or `map[string]interface{}`, always including `Authenticated bool` and `CSRFToken string`.

**CSP** (`internal/middleware/security.go:59`):
```
default-src 'self'
script-src 'self' https://static.cloudflareinsights.com 'unsafe-inline' 'unsafe-hashes'
style-src 'self' https://fonts.googleapis.com 'unsafe-inline'
img-src 'self' data:
font-src 'self' https://fonts.gstatic.com
connect-src 'self' https://cloudflareinsights.com
form-action 'self'
```
`'unsafe-inline'` and `'unsafe-hashes'` are required — templates use inline `<script>` and HTML event handlers (`onclick`, `onchange`, `ondragover`).

**HX-Redirect pattern**: POST handlers check `HX-Request` and set `HX-Redirect` for HTMX, `303 See Other` for regular forms. Used in `CreateShare`, `DeleteShare` (shares.go), `DeleteFile` (files.go).

## Gotchas

1. **Rate limits are dynamic in production**: `DynamicRateLimitMiddleware` reads `cfg.RateLimitPerMinute` (default 60). Login = `max(5, RateLimitPerMinute/3)`. The static `LoginRate` (200) / `APIRate` (600) from `security.go` are fallbacks when cfg is nil (tests only).

2. **SQLite** via `modernc.org/sqlite` (pure Go, no CGO). `SetMaxOpenConns(1)` — single connection only.

3. **CSRF** via nosurf. Token exposed in non-HttpOnly cookie `XSRF-TOKEN` for JS/HTMX. Retrieve via `middleware.CSRFToken(r)`.

4. **Error pages** (`renderError` in error.go) use `ErrorData` struct which has `Authenticated bool` field — but `renderError` leaves it at zero value (false). If an authenticated user hits 404, they see unauthenticated nav.

5. **New share form** uses hidden `file_id` populated by JS (upload or file select). Without it, form submission fails with `"file_id is required for file share"`.

6. **First-run password wizard** reads from stdin. `task dev` pipes `admin123` automatically.

7. **Env overrides**: config values can be set via `FLS_*` env vars (see `config.go:EnvOverrides`).

## Frontend Design

**Design system**: magick.css-inspired monochrome minimalism. High-contrast black-on-white, border-only form inputs, paper-like cards with subtle shadow, system fonts. All styles in `web/static/custom.css` via CSS variables (`:root`). Dark/light mode via `prefers-color-scheme`.

**Templates** (`web/templates/`): Go `html/template` with `layout.html` inheritance. `.Authenticated` controls nav/footer visibility. Admin pages use `main.admin-mode` (wider max-width).

**Components** (see `custom.css` for exact class names):
| Component | CSS classes | Notes |
|-----------|-------------|-------|
| Buttons | `.btn`, `.btn-primary`, `.btn-danger`, `.btn-sm` | Flat, bordered, monochrome |
| Cards | `.card` | Paper style with border + shadow |
| Stat cards | `.stat-card` grid (4-col) | Dashboard metric display |
| Data table | `.data-table` + `.responsive-table` | Column headers uppercase, responsive on mobile |
| Forms | `.form-section`, `.form-group`, `.form-grid` | 2-col grid layout, bottom-border inputs |
| Badges | `.badge`, `.badge-success`, `.badge-danger`, `.badge-warning` | Status labels |
| Dropzone | `.dropzone`, `.dropzone-dragover`, `.dropzone-uploading`, `.dropzone-error` | File upload area with states |
| Type toggle | `.type-toggle` > `.type-card` | Radio-as-card for share type selection |
| Detail table | `.detail-table` | Key-value layout for share detail |
| Flash messages | `.flash`, `.flash-error` | Success/error notifications |
| Pagination | `.pagination` | Page nav with prev/next |
| Empty state | `.empty-state` | "No data" placeholder |

**HTMX**: Used for search (files), delete confirmations, pagination. CSRF token injected via `XSRF-TOKEN` cookie in `htmx:configRequest` event handler (`layout.html:22`).

**JS** (inline in templates): File upload via TUS protocol (tus.js), clipboard copy, type toggling, drop event handling. All inline `<script>` — requires `'unsafe-inline'` + `'unsafe-hashes'` in CSP `script-src`.

**Fonts**: System font stack only (`-apple-system`, `Segoe UI`, etc. — no Google Fonts loaded despite CSP allowance). Monospace for code/tokens.

**Responsive**: Breakpoints at 900px (2-col stats) and 600px (stacked layout, data tables become labeled rows). Touch targets ≥44px for `pointer: coarse`.

**Build**: Static files (`custom.css`, `htmx.min.js`, `tus.js`) embedded at compile time in `web/embed.go`. Update `custom.css?v=4.0.0` cache-bust version in `layout.html:7` when modifying CSS.

## Testing

- `setupTestServer()` creates full in-memory server with seeded data: 1 admin password (`admin123`), 1 file, 1 share (token `testtokn`).
- Tests bypass CSRF via `/test-login` endpoint (registered only in test setup).
- `loginAsAdmin()` extracts SCS session cookie via cookiejar.
- `noRedirectClient()` returns 3xx without following redirects.
- No external services needed — everything is in-memory.
