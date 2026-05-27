# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

| Action | Command |
|--------|---------|
| Build | `task build` |
| Cross (Windows) | `task build:windows` |
| Cross (Linux) | `task build:linux` |
| Run tests (all) | `task test` |
| Single test | `go test ./... -run TestIntegration_Dashboard -v` |
| Dev server | `task dev` (auto-pipes admin password `admin123`) |
| Run (no rebuild) | `task run` |
| Clean artifacts | `task clean` |

Uses [Taskfile](https://taskfile.dev/) (install via `mise install task`). Build injects version/commit/date via ldflags.

## Architecture

Go 1.26 web app — single-binary file sharing system with SQLite storage (pure Go, no CGO via `modernc.org/sqlite`).

**Entry point**: `main.go` wires `chi.NewRouter()` with global middleware: `chimw.Logger → chimw.RealIP → SecurityHeadersMiddleware → sessionManager.LoadAndSave → NewRecoveryMiddleware(sm)`.

**Route groups** (exact middleware per group defined in `main.go`):
| Group | RateLimit | CSRF | Auth |
|-------|-----------|------|------|
| `/health`, `/static/*` | No | No | No |
| `/` (public index) | Yes | No | No |
| `/login` | Login rate | Yes | No |
| `/s/{token}*` | API rate | No | No |
| `/logout` | No | No | No |
| `/admin/*`, `/admin/api/stats` | API rate | Yes | Yes |
| `/api/upload/*` | API rate | No | Yes |

**Key layers**:
- `internal/handler/` — HTTP handlers (login, dashboard, files, shares, config, stats, download, public)
- `internal/service/` — Business logic (auth, share, stats)
- `internal/middleware/` — Auth, CSRF (nosurf), security headers, dynamic rate limiting
- `internal/tus/` — TUS protocol upload handlers (mounted at `/api/upload`)
- `internal/config/` — Config loaded from DB, supports `FLS_*` env overrides
- `internal/database/` — SQLite + migrations + log cleaner
- `internal/model/` — Data models (file, share, config, download_log, errors)

**Templates**: Go `html/template` in `web/templates/`, embedded via `//go:embed` in `web/embed.go`. All templates inherit from `layout.html`. `RenderTemplate(w, name, data)` in `internal/handler/render.go` — data must include `Authenticated bool` and `CSRFToken string`.

**HTMX**: Used for search, delete confirmations, pagination. CSRF token injected via `XSRF-TOKEN` cookie in `htmx:configRequest` event handler (`layout.html`).

## Gotchas

1. **Rate limits are dynamic**: `DynamicRateLimitMiddleware` reads `cfg.RateLimitPerMinute` (default 60). Login = `max(5, RateLimitPerMinute/3)`. Static `LoginRate`/`APIRate` are fallbacks for when cfg is nil (tests only).

2. **SQLite single connection**: `SetMaxOpenConns(1)` — single connection only. No concurrent writes.

3. **CSRF**: Token in non-HttpOnly cookie `XSRF-TOKEN` for JS/HTMX. Retrieve via `middleware.CSRFToken(r)`. Templates inject it into forms.

4. **Error pages**: `renderError` in `error.go` uses `ErrorData` which has `Authenticated bool` — but `renderError` leaves it at zero (false). Authenticated users hitting error pages see unauthenticated nav.

5. **Upload handlers are in `internal/tus/`**, not `internal/handler/` — mounted via `tusHandler.Mount()`.

6. **New share form** requires hidden `file_id` populated by JS (upload or file select). Without it, form submission fails with `"file_id is required for file share"`.

7. **CSP** allows `'unsafe-inline'` and `'unsafe-hashes'` — required because templates use inline `<script>` and HTML event handlers (`onclick`, `onchange`, `ondragover`).

8. **HX-Redirect pattern**: POST handlers check `HX-Request` and set `HX-Redirect` for HTMX, `303 See Other` for regular forms.

## Testing

- `setupTestServer()` in `internal/handler/integration_test.go` creates a full in-memory server with seeded data: 1 admin password (`admin123`), 1 file, 1 share (token `testtokn`).
- Tests bypass CSRF via `/test-login` endpoint (registered only in test setup).
- `loginAsAdmin()` extracts SCS session cookie via cookiejar.
- No external services needed — everything is in-memory.

## Frontend

**Design system**: magick.css-inspired monochrome minimalism. High-contrast black-on-white, border-only form inputs, paper-like cards with subtle shadow, system fonts.

**Styles**: All in `web/static/custom.css` via CSS variables (`:root`). Dark/light mode via `prefers-color-scheme`. Update `custom.css?v=4.0.0` cache-bust version in `layout.html` when modifying CSS.

**Components** (in `custom.css`): `.btn` variants, `.card`, `.stat-card` (4-col grid), `.data-table` + `.responsive-table`, `.form-section`/`.form-group`/`.form-grid`, `.badge` variants, `.dropzone` (upload area with states), `.type-toggle`/`.type-card`, `.detail-table`, `.flash`, `.pagination`, `.empty-state`.

**JS**: File upload via TUS protocol (tus.js), clipboard copy, type toggling, drop handling — all inline `<script>` in templates.

**Responsive**: 900px (2-col stats) and 600px (stacked layout). Touch targets >= 44px for `pointer: coarse`.

## Deployment

CI/CD via `.github/workflows/deploy.yml`: runs tests, builds Linux amd64, deploys via SSH/SCP to VPS, restarts systemd user service. Triggered on push to `master`.
