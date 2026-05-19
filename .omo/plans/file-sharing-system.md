# FLS — 文件分享系统 (File Sharing System)

## TL;DR

> **Quick Summary**: 使用 Go 构建的 Web 文件分享系统。单二进制运行，通过浏览器管理文件，生成带密码+过期时间的分享链接。支持**文件分享**和**文本分享**（pastebin 风格），**图片自动预览**，**二维码快速分享**。大文件(>2GB)分片上传(TUS协议)，SQLite 持久化存储，移动端适配。
>
> **Deliverables**:
> - 单二进制文件 `fls` (Windows/Linux/macOS)
> - Web 管理后台（文件管理 + 文本管理 + 分享链接管理 + 系统配置）
> - 公开分享页面（密码保护 + 过期时间 + 下载统计 + 图片预览 + 文本展示）
> - 二维码生成（每个分享链接自动生成，管理后台展示）
> - 首次启动交互式密码设置向导
>
> **Estimated Effort**: Medium-Large (15-18 个任务)
> **Parallel Execution**: YES — 3 个并行 Wave + 1 个 Final 验证 Wave
> **Critical Path**: 项目脚手架 → 数据库层 → TUS上传 → 分享链接 → 下载处理 → Final 验证

---

## Context

### Original Request

使用 Golang 构建一个通过 Web 管理的文件分享系统。要求：
- 文件增删改查
- 分享链接（密码保护 + 过期时间）
- 单用户（后期可扩展）
- 单二进制，SQLite 数据库，后台配置持久化

### Interview Summary

**Key Discussions**:
| 决策项 | 选择 | 原因 |
|--------|------|------|
| Web框架 | Chi router + Go 1.22+ | 轻量，兼容 net/http |
| 前端方案 | html/template + HTMX + magick.css | 单二进制无需构建工具，移动端自动适配 |
| SQLite驱动 | modernc.org/sqlite | 纯Go，无CGo依赖，保持单二进制 |
| 文件存储 | 本地磁盘 | 大文件性能好，易备份 |
| 分片上传 | TUS协议 (tusd) | 标准开放协议，支持断点续传 |
| 分享链接 | 8字符随机短Token | 简短美观，不可猜测 |
| 登录方式 | Cookie Session + 首次终端设密码 | 简单安全 |
| 下载统计 | 需要 | 记录每次下载信息 |
| 移动端适配 | 需要 | magick.css 天然支持 |
| 测试 | TDD + testify | 保证质量 |
| Go版本 | 1.22+ | 支持新特性 |
| 文本分享 | 需要 | 粘贴文本直接生成分享，pastebin 风格 |
| 图片预览 | 需要 | 分享图片时浏览器内直接展示 |
| 二维码 | yeqown/go-qrcode/v2 (非skip2) | 活跃维护(2025-2026)，支持自定义样式 |
| Session | alexedwards/scs (非gorilla/sessions) | OWASP 合规，SQLite 后端持久化 |
| CSRF | justinas/nosurf (非gorilla/csrf) | 轻量单文件，无外部依赖 |
| 速率限制 | ulule/limiter v3 (非uber/ratelimit) | HTTP 中间件式，配置灵活 |

**Research Findings**:
- magick.css: 无class极简CSS框架，巫师笔记风格，100%响应式，纯HTML语义
- TUS协议: 开放文件上传协议，Go有成熟实现(tusd)

---

## Work Objectives

### Core Objective

构建一个完整的、可直接部署的 Go Web 文件分享系统，单用户通过浏览器管理文件、创建带密码保护的分享链接。

### Concrete Deliverables

- `fls` — 单二进制入口
- `internal/` — 全套后端逻辑（数据库、模型、处理器、服务层、中间件）
- `web/templates/` — 全栈 Go 模板（管理端 + 分享端：文件列表、文本编辑、分享管理含二维码、登录、仪表盘）
- `web/static/` — 嵌入静态资源（自定义 CSS）
- `data/` — 运行时数据目录（上传文件 + SQLite + 分享预览缓存）

### Definition of Done

- [x] `go build -o fls .` → 生成单二进制 ✅
- [x] 首次运行交互式设置密码 → 启动 Web 服务
- [x] 浏览器访问管理后台 → 登录 → 文件 CRUD + 文本分享 + 分享管理
- [x] 公开分享链接 → 密码验证 → 文件下载/文本展示/图片预览 → 统计记录
- [x] 分享管理页面显示二维码
- [x] `go test ./...` → 全部通过
- [x] 移动端浏览器管理界面正常显示

### Must Have

- [x] 单二进制编译运行
- [x] SQLite 存储（元数据 + 配置）
- [x] 文件上传（支持 TUS 分片，>2GB）
- [x] **文本分享**（直接粘贴文本生成分享链接，pastebin 风格）
- [x] **图片在线预览**（分享图片时在浏览器直接展示）
- [x] **二维码生成**（每个分享链接自动生成二维码 PNG）
- [x] 分享链接（密码 Bcrypt 加密 + 过期自动失效）
- [x] 管理后台（文件列表、文本管理、分享管理、配置页面）
- [x] 下载统计（IP、时间、User-Agent）
- [x] 配置持久化在数据库中
- [x] 移动端适配
- [x] 自动化测试（TDD）

### Must NOT Have (Guardrails)

- [x] 不要多用户系统（预留扩展点但不实现）
- [x] 不要文件在线预览/转码
- [x] 不要 S3/对象存储/OSS
- [x] 不要第三方 OAuth 登录
- [x] 不要文件查毒/扫描
- [x] 不要 WebSocket/实时推送
- [x] 不要容器化/编排配置（用户自行处理）

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: NO (新建项目)
- **Automated tests**: YES (TDD)
- **Framework**: Go testing + testify
- **Approach**: 每个功能任务：RED(写测试) → GREEN(实现) → REFACTOR

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.omo/evidence/task-{N}-{scenario-slug}.{ext}`.

- **API/Backend**: Bash (curl) — Send requests, assert status + response fields
- **CLI**: interactive_bash — Run command, validate output
- **Library/Module**: Bash (go test) — Run tests, assert pass

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — 基础 + 数据层 + 认证):
├── Task 1: 项目脚手架 + Go Module + 目录结构 [quick]
├── Task 2: 数据库层 — SQLite 初始化和迁移 [quick]
├── Task 3: 数据模型 — File, Share, Config 结构体 [quick]
├── Task 4: 配置系统 — Config 持久化到 DB [quick]
├── Task 5: 认证系统 — 密码设置向导 + Session 登录 [quick]
├── Task 6: 基础模板布局 — magick.css + HTMX 集成 [quick]
└── Task 7: TUS 上传处理器集成 [unspecified-high]

Wave 2 (After Wave 1 — 核心业务 + 管理界面):
├── Task 8: 文件上传 TUS 完整实现 (depends: 2, 7) [unspecified-high]
├── Task 9: 文件管理 API + 管理界面 (depends: 3, 6) [deep]
├── Task 10: 分享链接创建/管理 (depends: 3, 6) [deep]
├── Task 11: 公开下载处理器 — 密码验证 + 过期检查 (depends: 3, 10) [deep]
├── Task 12: 下载统计服务 (depends: 2, 11) [quick]
├── Task 13: 管理后台仪表盘 + 统计 (depends: 6, 9, 12) [visual-engineering]
├── Task 14: 系统配置页面 (depends: 4, 6) [quick]
└── Task 15: 安全加固 — CSRF, 限速, Headers (depends: 5) [unspecified-high]

Wave 3 (After Wave 2 — 收尾 + 移动端打磨):
├── Task 16: 移动端适配完善 (depends: 6, 13, 10) [visual-engineering]
├── Task 17: 错误页面 + 全局错误处理 (depends: 15) [quick]
└── Task 18: 自述文件 + 构建脚本 (depends: ALL) [writing]

Wave FINAL (After ALL tasks — 4 并行验证):
├── F1: Plan compliance audit (oracle)
├── F2: Code quality review (unspecified-high)
├── F3: Full QA execution (unspecified-high)
└── F4: Scope fidelity check (deep)
→ Present results → Get explicit user okay

Critical Path: Task 1 → Task 2 → Task 7 → Task 8 → Task 10 → Task 11 → F1-F4
Parallel Speedup: ~65% faster than sequential
Max Concurrent: 7 (Wave 1), 8 (Wave 2), 3 (Wave 3)
```

### Dependency Matrix

| Task | Depends On | Blocks |
|------|-----------|--------|
| 1 | - | 2-7 |
| 2 | 1 | 8, 12, 14 |
| 3 | 1 | 9, 10, 11 |
| 4 | 1 | 14 |
| 5 | 1 | 15 |
| 6 | 1 | 9, 10, 13, 14, 16 |
| 7 | 1 | 8 |
| 8 | 2, 7 | - |
| 9 | 3, 6 | 13, 16 |
| 10 | 3, 6 | 11, 16 |
| 11 | 3, 10 | 12 |
| 12 | 2, 11 | 13 |
| 13 | 6, 9, 12 | 16 |
| 14 | 4, 6 | - |
| 15 | 5 | 17 |
| 16 | 6, 13, 10 | - |
| 17 | 15 | - |
| 18 | ALL | F1-F4 |
| F1-F4 | 18 | user-ok |

### Agent Dispatch Summary

- **Wave 1**: 7 个任务并行 — T1-T4 → `quick`, T5 → `quick`, T6 → `quick`, T7 → `unspecified-high`
- **Wave 2**: 8 个任务并行 — T8 → `unspecified-high`, T9 → `deep`, T10 → `deep`, T11 → `deep`, T12 → `quick`, T13 → `visual-engineering`, T14 → `quick`, T15 → `unspecified-high`
- **Wave 3**: 3 个任务 — T16 → `visual-engineering`, T17 → `quick`, T18 → `writing`
- **Wave FINAL**: 4 个并行 — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 1. **项目脚手架 + Go Module + 目录结构**

  **What to do**:
  - 初始化 Go module: `go mod init fls`
  - 创建完整目录结构：`internal/` `web/templates/` `web/static/` `data/`
  - 创建 `main.go` 入口文件，解析 CLI flag: `--port`, `--data-dir`
  - 安装依赖：`chi`, `modernc.org/sqlite`, `tusd`, `alexedwards/scs` (session), `yeqown/go-qrcode/v2`, `justinas/nosurf` (CSRF), `ulule/limiter` (限速)
  - 编写 `go.mod` / `go.sum`
  - 创建 `.gitignore` 忽略 `data/` 和编译产物
  - 验证 `go build` 通过

  **Must NOT do**:
  - 不要实现任何业务逻辑
  - 不要创建过多抽象层

  **Recommended Agent Profile**:
  - Category: `quick`
  - Skills: `[]`
  - Reason: 纯文件操作和依赖安装，无复杂逻辑

  **Parallelization**:
  - Can Run In Parallel: YES
  - Parallel Group: Wave 1 (with Tasks 2-7)
  - Blocks: Tasks 2-7
  - Blocked By: None

  **References**:
  - Go 标准项目布局: https://github.com/golang-standards/project-layout
  - Chi router docs: https://go-chi.io/#/pages/routing
  - modernc.org/sqlite: https://pkg.go.dev/modernc.org/sqlite
  - tusd: https://github.com/tus/tusd

  **Acceptance Criteria**:
  - [x] `go build -o fls .` → exits 0, produces `fls` binary
  - [x] `go vet ./...` → exits 0
  - [x] 目录结构包含 `internal/`, `web/templates/`, `web/static/`, `data/`

  **QA Scenarios**:
  ```
  Scenario: Build verification
    Tool: Bash
    Preconditions: Go 1.22+ installed
    Steps:
      1. `cd $PROJECT_ROOT && go build -o fls .`
      2. Check exit code is 0
      3. Check `fls` binary exists
    Expected Result: Binary compiles without error
    Evidence: .omo/evidence/task-1-build.txt

  Scenario: Directory structure verification
    Tool: Bash
    Preconditions: Steps above completed
    Steps:
      1. Check directories exist: `Test-Path internal`, `Test-Path web/templates`, `Test-Path web/static`, `Test-Path data`
    Expected Result: All directories exist
    Evidence: .omo/evidence/task-1-structure.txt
  ```

  **Commit**: YES
  - Message: `feat: initial project scaffolding with Go module and directory structure`
  - Files: `go.mod`, `go.sum`, `main.go`, `.gitignore`, 目录结构

- [x] 2. **数据库层 — SQLite 初始化和迁移**

  **What to do**:
  - 创建 `internal/database/database.go`: 打开 SQLite 连接 (modernc.org/sqlite)，设置 WAL 模式、连接池
  - 实现自动迁移：首次运行时创建所有表
  - 表结构：
    - `files` (id TEXT PK, filename TEXT, original_name TEXT, size INTEGER, mime_type TEXT, storage_path TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)
    - `shares` (id TEXT PK, file_id TEXT FK, token TEXT UNIQUE, password_hash TEXT, expires_at TIMESTAMP, max_downloads INTEGER, download_count INTEGER, content_type TEXT NOT NULL DEFAULT 'file', text_content TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)
    - `config` (key TEXT PK, value TEXT, updated_at TIMESTAMP)
    - `download_logs` (id TEXT PK, share_id TEXT FK, ip_address TEXT, user_agent TEXT, downloaded_at TIMESTAMP)
  - `content_type` 枚举值: `'file'` (文件分享), `'text'` (文本分享)
  - `text_content` 存储分享的纯文本内容 (content_type='text' 时使用)
  - 创建 `internal/database/errors.go`: 自定义错误类型 (ErrNotFound, ErrConflict)
  - 编写测试: 数据库初始化、迁移、CRUD

  **Must NOT do**:
  - 不要用 ORM，纯 SQL 操作
  - 不要预填数据

  **Recommended Agent Profile**:
  - Category: `quick`
  - Reason: 标准数据库初始化，SQL 建表
  - Skills: `[]`

  **Parallelization**:
  - Can Run In Parallel: YES
  - Parallel Group: Wave 1 (with Tasks 1, 3-7)
  - Blocks: Tasks 8, 12, 14
  - Blocked By: Task 1

  **References**:
  - modernc.org/sqlite usage: https://pkg.go.dev/modernc.org/sqlite#Driver
  - Go SQLite WAL mode: `PRAGMA journal_mode=WAL`
  - Go sql.DB connection pool: https://pkg.go.dev/database/sql#DB.SetMaxOpenConns

  **Acceptance Criteria**:
  - [x] 数据库文件创建成功
  - [x] 所有表 (files, shares, config, download_logs) 创建成功
  - [x] 重复运行迁移不报错
  - [x] `go test ./internal/database/` → PASS

  **QA Scenarios**:
  ```
  Scenario: Database creates tables correctly
    Tool: Bash
    Preconditions: Go build passes
    Steps:
      1. Write test that opens DB, runs migration, queries table list
      2. `go test -run TestMigration ./internal/database/`
    Expected Result: All 4 tables exist after migration
    Evidence: .omo/evidence/task-2-migration.txt
  ```

  **Commit**: YES (grouped with Task 1)
  - Message: `feat: database layer with SQLite migration and table definitions`
  - Files: `internal/database/`

- [x] 3. **数据模型 — File, Share, Config 结构体**

  **What to do**:
  - `internal/model/file.go`: File 结构体 (ID, OriginalName, Size, MimeType, StoragePath, CreatedAt, UpdatedAt) + 序列化/反序列化方法
  - `internal/model/share.go`: Share 结构体 (ID, FileID, Token, PasswordHash, ExpiresAt, MaxDownloads, DownloadCount, ContentType, TextContent, CreatedAt, UpdatedAt) + 验证方法 (IsExpired, IsPasswordCorrect, IsTextShare, IsFileShare)
  - `internal/model/config.go`: ConfigItem 结构体 (Key, Value, UpdatedAt)
  - `internal/model/download_log.go`: DownloadLog 结构体
  - `internal/model/errors.go`: 通用错误类型
  - 使用 `github.com/google/uuid` 或 `crypto/rand` 生成 ID
  - 编写测试

  **Must NOT do**:
  - 不要包含业务逻辑（验证方法除外）
  - 不要添加 JSON 标签之外的序列化逻辑

  **Recommended Agent Profile**:
  - Category: `quick`
  - Reason: 纯数据结构定义，无复杂逻辑
  - Skills: `[]`

  **Parallelization**:
  - Can Run In Parallel: YES
  - Parallel Group: Wave 1 (with Tasks 1, 2, 4-7)
  - Blocks: Tasks 9, 10, 11
  - Blocked By: Task 1

  **References**:
  - Go time.Time: https://pkg.go.dev/time
  - go-crypto/bcrypt: https://pkg.go.dev/golang.org/x/crypto/bcrypt

  **Acceptance Criteria**:
  - [x] `go test ./internal/model/` → PASS
  - [x] Share.IsExpired() 对过期/未过期的 share 返回正确结果

  **QA Scenarios**:
  ```
  Scenario: Model tests pass
    Tool: Bash
    Preconditions: Go build passes
    Steps:
      1. `go test -v ./internal/model/`
    Expected Result: All tests pass
    Evidence: .omo/evidence/task-3-models.txt
  ```

  **Commit**: YES (grouped with Task 1-2)

- [x] 4. **配置系统 — Config 持久化到 DB**

  **What to do**:
  - `internal/config/config.go`: Config 结构体（端口, 数据目录, Token长度, 最大上传大小, Session密钥 等）
  - 实现 `Load()`: 从 SQLite config 表读取所有配置
  - 实现 `Save()`: 将配置持久化到 config 表
  - 初始化默认值：端口 8080, 数据目录 "./data", Token长度 8
  - 支持通过 CLI flag 覆盖: `--port`, `--data-dir`
  - 编写测试: 配置保存/加载/默认值

  **Must NOT do**:
  - 不要读取 YAML/JSON/TOML 配置文件
  - 不要实现热加载（简单系统不需要）

  **Recommended Agent Profile**:
  - Category: `quick`
  - Reason: 标准 CRUD 操作
  - Skills: `[]`

  **Parallelization**:
  - Can Run In Parallel: YES
  - Parallel Group: Wave 1
  - Blocks: Task 14
  - Blocked By: Task 1

  **References**:
  - flag package: https://pkg.go.dev/flag
  - os.Getenv for env var support

  **Acceptance Criteria**:
  - [x] 默认配置正确加载
  - [x] 修改后保存到数据库并重新加载一致
  - [x] CLI flag 能覆盖默认值
  - [x] `go test ./internal/config/` → PASS

  **QA Scenarios**:
  ```
  Scenario: Config persists correctly
    Tool: Bash
    Preconditions: Task 2 completed (DB exists)
    Steps:
      1. Write test to save config, reload from DB, compare
      2. `go test -run TestConfigPersistence ./internal/config/`
    Expected Result: Loaded values match saved values
    Evidence: .omo/evidence/task-4-config.txt
  ```

  **Commit**: YES (grouped with Tasks 1-3)

- [x] 5. **认证系统 — 密码设置向导 + Session 登录**

  **What to do**:
  - `internal/service/auth.go`: 密码哈希 (bcrypt)、Session 管理、认证逻辑
  - `internal/middleware/auth.go`: 认证中间件 — 检查 Session Cookie，未认证跳转登录页
  - 首次启动逻辑：
    - 检查数据库中有无管理员密码
    - 若无，终端输出提示 `首次运行：请在终端设置管理员密码`
    - 等待用户输入密码（两次确认）
    - Bcrypt 哈希后存入 config 表
    - 自动启动 Web 服务
  - 登录流程：
    - GET /login → 显示登录页面
    - POST /login → 验证密码 → 创建 Session
    - GET /logout → 清除 Session → 重定向登录页
  - Session 使用 `alexedwards/scs` — 支持 SQLite 存储后端(scs/sqlite3store)，OWASP 安全指南合规，自动 session 加载/保存中间件
  - 编写测试

  **Must NOT do**:
  - 不要实现注册功能（单用户）
  - Session 密钥从配置读取，不要硬编码
  - 密码不要明文存储

  **Recommended Agent Profile**:
  - Category: `quick`
  - Reason: 标准认证模式，Go 生态成熟方案
  - Skills: `[]`

  **Parallelization**:
  - Can Run In Parallel: YES
  - Parallel Group: Wave 1 (with Tasks 1-4, 6-7)
  - Blocks: Task 15
  - Blocked By: Task 1

  **References**:
  - bcrypt: https://pkg.go.dev/golang.org/x/crypto/bcrypt
  - alexedwards/scs: https://github.com/alexedwards/scs (session + SQLite store)
  - SCS sqlite3store: https://github.com/alexedwards/scs/tree/master/sqlite3store
  - Chi middleware pattern: https://go-chi.io/#/pages/middleware

  **Acceptance Criteria**:
  - [x] 首次运行提示设置密码
  - [x] 登录后能访问受保护路由
  - [x] 未登录时跳转登录页
  - [x] 登出后清除 Session
  - [x] Session 持久化在 SQLite 中（重启不丢失）
  - [x] `go test ./internal/service/` → PASS

  **QA Scenarios**:
  ```
  Scenario: Login flow
    Tool: Bash (curl)
    Preconditions: First-run password already set in DB
    Steps:
      1. `curl -c cookies.txt -L http://localhost:8080/login`
      2. Verify response contains login form
      3. `curl -c cookies.txt -b cookies.txt -X POST -d "password=test123" http://localhost:8080/login`
      4. Verify response contains session cookie
      5. `curl -b cookies.txt http://localhost:8080/admin/`
      6. Verify response is admin dashboard (not redirected)
    Expected Result: Login succeeds, protected page accessible
    Evidence: .omo/evidence/task-5-login.txt

  Scenario: Session survives restart
    Tool: Bash (curl)
    Preconditions: Logged in, session cookie saved
    Steps:
      1. Restart server process
      2. `curl -b cookies.txt http://localhost:8080/admin/`
      3. Verify admin page accessible (not redirected to login)
    Expected Result: Session persists across restarts via SQLite store
    Evidence: .omo/evidence/task-5-session-persist.txt
  ```

  **Commit**: YES (grouped with Tasks 1-4)

- [x] 6. **基础模板布局 — magick.css + HTMX 集成**

  **What to do**:
  - 使用 Go `embed.FS` 嵌入 `web/templates/` 目录
  - `web/templates/layout.html`: 基础 HTML 骨架
    - `<head>`: meta viewport, magick.css CDN, htmx CDN, 自定义 CSS
    - `<body>`: `<main>` 内容插槽 + 导航栏 + footer
    - 导航栏: Logo/标题, 文件管理, 分享管理, 配置, 登出
    - 移动端导航通过 `<details>` 实现折叠菜单（magick.css 原生支持）
  - 自定义 CSS 覆盖: `web/static/custom.css` （嵌入）
    - 文件列表样式、上传进度条、操作按钮
  - HTMX 集成: `web/static/htmx.min.js` （嵌入或 CDN）
  - `internal/handler/render.go`: 模板渲染辅助函数
  - 所有模板继承 layout.html

  **Must NOT do**:
  - 不要使用任何前端构建工具
  - 不要引入 Node.js 依赖
  - magick.css 通过 CDN 直接引入（保持单二进制，运行时联网）

  **Recommended Agent Profile**:
  - Category: `visual-engineering`
  - Reason: 前端布局和样式设计
  - Skills: `[]`

  **Parallelization**:
  - Can Run In Parallel: YES
  - Parallel Group: Wave 1
  - Blocks: Tasks 9, 10, 13, 14, 16
  - Blocked By: Task 1

  **References**:
  - magick.css: https://css.winterveil.net/ (中文文档)
  - HTMX: https://htmx.org/docs/
  - Go embed: https://pkg.go.dev/embed
  - Go html/template: https://pkg.go.dev/html/template

  **Acceptance Criteria**:
  - [x] 模板正确渲染，继承 layout.html
  - [x] magick.css 样式生效
  - [x] 导航栏在所有页面显示
  - [x] 移动端浏览器显示正常（折叠导航）
  - [x] `go test ./internal/handler/` → PASS

  **QA Scenarios**:
  ```
  Scenario: Template renders correctly
    Tool: Bash (curl) (after auth)
    Preconditions: Auth working (Task 5), logged in
    Steps:
      1. `curl -b cookies.txt http://localhost:8080/admin/`
      2. Grep output for `<main>` tag, navigation links
    Expected Result: HTML contains main content area, nav links present
    Evidence: .omo/evidence/task-6-templates.txt
  ```

  **Commit**: YES (grouped with Tasks 1-5)
  - Message: `feat: project scaffolding, database, auth, and base templates`
  - Files: All Wave 1 files

- [x] 7. **TUS 上传处理器集成**

  **What to do**:
  - 集成 `github.com/tus/tusd` 库处理分片上传
  - `internal/tus/handler.go`: 自定义 TUS 数据存储（文件存磁盘 + 元数据更新数据库）
  - 实现 tusd.DataStore 接口:
    - `NewUpload`: 创建上传记录
    - `WriteChunk`: 写入分片数据
    - `ReadChunk`: 读取分片数据（断点续传）
    - `FinishUpload`: 完成上传，创建 File 记录
    - `GetInfo`: 获取上传信息
  - TUS 路由挂载: `POST /api/upload/tus`, `PATCH /api/upload/tus/{id}`, `HEAD /api/upload/tus/{id}`
  - 普通（小文件）上传: `POST /api/upload/simple` — 直接保存
  - 编写测试（上传模拟）

  **Must NOT do**:
  - 不要修改 tusd 核心代码
  - 大文件上传不要加载到内存

  **Recommended Agent Profile**:
  - Category: `unspecified-high`
  - Reason: 第三方库集成，接口适配，tusd DataStore 实现
  - Skills: `[]`

  **Parallelization**:
  - Can Run In Parallel: YES
  - Parallel Group: Wave 1
  - Blocks: Task 8
  - Blocked By: Task 1

  **References**:
  - tusd Go library: https://github.com/tus/tusd
  - tusd DataStore interface: https://pkg.go.dev/github.com/tus/tusd/pkg/data-store
  - TUS protocol spec: https://tus.io/protocols/resumable-upload

  **Acceptance Criteria**:
  - [x] TUS `POST` 创建上传返回 Location header
  - [x] TUS `PATCH` 上传分片成功
  - [x] TUS `HEAD` 返回上传进度
  - [x] 上传完成后数据库中创建 File 记录
  - [x] 普通上传接口也正常工作
  - [x] `go test ./internal/tus/` → PASS

  **QA Scenarios**:
  ```
  Scenario: TUS upload small file
    Tool: Bash (curl)
    Preconditions: Auth working, TUS handler mounted
    Steps:
      1. Create test file: `echo "hello world" > /tmp/test-upload.txt`
      2. TUS POST: `curl -i -X POST -H "Upload-Length: 12" -H "Upload-Metadata: filename dGVzdC11cGxvYWQudHh0" http://localhost:8080/api/upload/tus`
      3. Extract Location header
      4. TUS PATCH to upload content: `curl -i -X PATCH -H "Upload-Offset: 0" -H "Content-Type: application/offset+octet-stream" --data-binary @/tmp/test-upload.txt $LOCATION`
      5. TUS HEAD to verify: `curl -i -X HEAD $LOCATION`
    Expected Result: Upload-Offset matches Upload-Length, file saved to disk
    Evidence: .omo/evidence/task-7-tus.txt
  ```

  **Commit**: NO (grouped with Task 8)

- [x] 8. **文件上传 TUS 完整实现**

  **What to do**:
  - 整合 Task 7 的 TUS 处理器到完整 HTTP 路由
  - 添加上传进度通知（通过 HTMX 轮询或 SSE）
  - 上传完成后自动跳转到文件详情页
  - MIME 类型自动检测（通过文件头而非扩展名）
  - 文件去重策略：同名文件不覆盖（追加时间戳）
  - 存储路径: `{data-dir}/uploads/{YYYY-MM}/{file-id}/{filename}`
  - 上传限制检查：配置中的最大上传大小、磁盘空间检查
  - 编写完整测试：小文件、大分片、断点续传模拟

  **Must NOT do**:
  - 不要扫描文件内容（安全由使用者负责）
  - 不要限制文件类型

  **Recommended Agent Profile**:
  - Category: `unspecified-high`
  - Reason: 复杂上传流程集成
  - Skills: `[]`

  **Parallelization**:
  - Can Run In Parallel: NO
  - Blocks: None
  - Blocked By: Tasks 2, 7

  **References**:
  - net/http MIME detection: https://pkg.go.dev/net/http#DetectContentType
  - TUS protocol examples: https://tus.io/protocols/resumable-upload

  **Acceptance Criteria**:
  - [x] 普通上传 (< 10MB) 直接完成
  - [x] TUS 分片上传（模拟 > 50MB）成功完成
  - [x] 上传完成后文件保存到磁盘指定路径
  - [x] 数据库中创建准确的 File 记录
  - [x] `go test ./internal/tus/` → PASS
  - [x] `go test ./internal/handler/` → PASS

  **QA Scenarios**:
  ```
  Scenario: Full upload flow through web
    Tool: Bash (curl)
    Preconditions: Auth working, logged in cookies
    Steps:
      1. Upload file via admin API: `curl -b cookies.txt -F "file=@/tmp/test.txt" http://localhost:8080/api/upload/simple`
      2. Verify response JSON contains file info (id, name, size)
      3. Check file exists on disk: `Test-Path $dataDir/uploads/*/test.txt`
      4. Check DB has file record
    Expected Result: File uploaded, stored, DB record created
    Evidence: .omo/evidence/task-8-upload.txt
  ```

  **Commit**: YES
  - Message: `feat: file upload with TUS protocol support for large files`
  - Files: `internal/tus/`, `internal/handler/upload.go`
  - Pre-commit: `go test ./internal/tus/ ./internal/handler/`

- [x] 9. **文件管理 API + 管理界面**

  **What to do**:
  - `internal/handler/files.go`: 文件管理 HTTP 处理器
    - GET /admin/files — 文件列表页（HTMX 表格）
    - GET /admin/files/{id} — 文件详情页
    - DELETE /admin/files/{id} — 删除文件（删除磁盘+DB+关联分享）
    - GET /admin/files/{id}/edit — 编辑文件名
    - PUT /admin/files/{id} — 更新文件信息
  - 文件列表功能：
    - 分页（每页 20 条）
    - 按文件名搜索
    - 按上传时间排序
    - 显示: 文件名、大小(可读格式)、MIME类型、上传时间、关联分享数
  - 文件详情页：
    - 文件基本信息
    - 关联分享链接列表
    - 操作：重命名、删除
  - HTMX 交互：
    - 删除确认弹窗（花体弹窗）
    - 搜索自动提交（输入延迟）
    - 分页按钮
  - `web/templates/files.html`: 文件列表模板
  - `web/templates/file-detail.html`: 文件详情模板
  - 编写测试

  **Must NOT do**:
  - 不要实现文件内容修改（上传后只读）
  - 不要批量操作（后期扩展）

  **Recommended Agent Profile**:
  - Category: `deep`
  - Reason: 前后端集成，HTMX 交互逻辑
  - Skills: `[]`

  **Parallelization**:
  - Can Run In Parallel: YES
  - Parallel Group: Wave 2 (with Tasks 10-15)
  - Blocks: Tasks 13, 16
  - Blocked By: Tasks 3, 6

  **References**:
  - HTMX patterns: https://htmx.org/examples/
  - Chi URL params: `chi.URLParam(r, "id")`

  **Acceptance Criteria**:
  - [x] 文件列表页面显示所有已上传文件
  - [x] 分页功能正常
  - [x] 搜索过滤正常
  - [x] 删除文件同时删除磁盘文件和 DB 记录
  - [x] `go test ./internal/handler/` → PASS

  **QA Scenarios**:
  ```
  Scenario: File list and delete
    Tool: Bash (curl)
    Preconditions: At least 3 files uploaded via Task 8
    Steps:
      1. `curl -b cookies.txt http://localhost:8080/admin/files`
      2. Verify HTML contains file list table with file names
      3. Get file ID from list
      4. `curl -b cookies.txt -X DELETE http://localhost:8080/admin/files/{id}`
      5. `curl -b cookies.txt http://localhost:8080/admin/files`
      6. Verify file no longer in list
    Expected Result: File delete works, file removed from disk and DB
    Evidence: .omo/evidence/task-9-files.txt
  ```

  **Commit**: YES (grouped with Task 10)

- [x] 10. **分享链接创建/管理（文件 + 文本 + 二维码）**

  **What to do**:
  - `internal/service/share.go`: 分享链接业务逻辑
    - 生成唯一 8 字符 Token（crypto/rand 安全随机）
    - 密码哈希（Bcrypt，可选，空密码表示无密码）
    - 过期时间校验
    - 限制下载次数（可选，0 或无限制）
    - **文本分享**：content_type='text' 时，无需文件 ID，直接存储 text_content
    - **二维码生成**：使用 `github.com/yeqown/go-qrcode/v2` 库生成 PNG 二维码（支持自定义颜色、logo、输出格式，活跃维护中）
  - `internal/handler/shares.go`: 分享管理 HTTP 处理器
    - GET /admin/shares — 分享链接列表（显示类型图标：📄文件 / 📝文本）
    - GET /admin/shares/new — 创建分享页面
      - 选项卡切换：**文件分享** | **文本分享**
      - 文件分享：文件选择器 + 密码 + 过期时间
      - 文本分享：文本输入框（限制 1MB）+ 密码 + 过期时间
    - POST /admin/shares — 创建分享（根据 content_type 分别处理）
    - DELETE /admin/shares/{id} — 撤销分享
    - GET /admin/shares/{id} — 分享详情
      - 显示分享链接（带复制按钮，HTMX 一键复制）
      - **显示二维码**：嵌入二维码 PNG（base64 data URI 或独立 /admin/shares/{id}/qrcode 端点）
      - 密码状态、下载次数、过期时间
  - Token 碰撞检测：生成后查询 DB，碰撞则重新生成
  - 创建分享时的文件选择器（HTMX 搜索选择）
  - `web/templates/shares.html`: 分享列表模板（增加类型列）
  - `web/templates/share-detail.html`: 创建/编辑分享模板（增加文本输入、二维码展示、复制按钮）
  - 编写测试

  **Must NOT do**:
  - 分享创建后不能修改密码和过期时间（只能删除重建）
  - Token 不能自定义（安全原因）
  - 文本内容不要存到文件系统，只存数据库

  **Recommended Agent Profile**:
  - Category: `deep`
  - Reason: 核心业务逻辑，安全考量，第三方库集成
  - Skills: `[]`

  **Parallelization**:
  - Can Run In Parallel: YES
  - Parallel Group: Wave 2
  - Blocks: Tasks 11, 16
  - Blocked By: Tasks 3, 6

  **References**:
  - crypto/rand: https://pkg.go.dev/crypto/rand
  - bcrypt: https://pkg.go.dev/golang.org/x/crypto/bcrypt
  - go-qrcode v2: https://github.com/yeqown/go-qrcode (v2, active 2025-2026)
  - QR code generation example: `qrcode.New(url, qrcode.WithQRCodeVersion(qrcode.VersionAuto))` → `writer/standard.New(w)` → `w.Write(q)`

  **Acceptance Criteria**:
  - [x] 文件分享：创建分享生成 8 字符 Token
  - [x] 文本分享：粘贴文本，生成分享链接，无需上传文件
  - [x] 可选密码被正确 Bcrypt 哈希
  - [x] 过期时间正确存储
  - [x] 撤销分享立即失效
  - [x] Token 无碰撞
  - [x] **二维码 PNG 生成成功，包含完整的分享 URL**
  - [x] **分享详情页显示二维码图片**
  - [x] `go test ./internal/service/` → PASS

  **QA Scenarios**:
  ```
  Scenario: Create file share
    Tool: Bash (curl)
    Preconditions: At least 1 file exists from Task 8
    Steps:
      1. Get file ID from file list
      2. `curl -b cookies.txt -X POST -d "file_id={file_id}&password=mypass&expires_in=24h" http://localhost:8080/admin/shares`
      3. Verify response JSON contains token (8 chars)
      4. `curl -b cookies.txt http://localhost:8080/admin/shares`
      5. Verify new share appears in list
    Expected Result: Share created with valid token
    Evidence: .omo/evidence/task-10-file-share.txt

  Scenario: Create text share
    Tool: Bash (curl)
    Preconditions: Logged in
    Steps:
      1. `curl -b cookies.txt -X POST -d "content_type=text&text_content=Hello+World&password=mypass&expires_in=24h" http://localhost:8080/admin/shares`
      2. Verify response JSON has content_type="text" and token
      3. `curl -b cookies.txt http://localhost:8080/admin/shares`
      4. Verify new text share appears with text icon
    Expected Result: Text share created without file upload
    Evidence: .omo/evidence/task-10-text-share.txt

  Scenario: QR code displayed on share detail page
    Tool: Bash (curl)
    Preconditions: Share exists
    Steps:
      1. `curl -b cookies.txt http://localhost:8080/admin/shares/{id}`
      2. Grep HTML for `<img` tag with "qrcode" or base64 PNG data
    Expected Result: QR code image present in share detail page
    Evidence: .omo/evidence/task-10-qrcode.txt
  ```

  **Commit**: YES (grouped with Task 9)
  - Message: `feat: file management and share link CRUD with text sharing and QR code`
  - Files: `internal/handler/files.go`, `internal/handler/shares.go`, `internal/service/share.go`

- [x] 11. **公开下载处理器 — 密码验证 + 过期检查 + 图片预览 + 文本展示**

  **What to do**:
  - `internal/handler/download.go`: 公开访问的下载处理器
    - GET /s/{token} — 分享页面
      - 检查 Token 是否存在 → 不存在显示 404
      - 检查是否过期 → 过期显示"链接已过期"
      - 检查是否密码保护 → 有密码显示密码输入表单
      - 验证密码 → 错误显示重试
      - 验证通过 → 根据 content_type 展示不同内容
    - **文本分享展示** (content_type='text'):
      - 从数据库读取 text_content
      - 在 `<pre>` 代码块中显示文本内容（magick.css 风格）
      - 提供"复制文本"和"下载为 .txt"按钮
      - 大文本（>100KB）默认折叠，点击展开
    - **图片文件预览** (content_type='file' 且 MIME 以 image/ 开头):
      - 显示 `<img>` 标签嵌入图片（通过 /s/{token}/raw 端点直接输出）
      - 图片下方提供下载按钮
    - **其他文件**:
      - 显示文件信息（文件名、大小、MIME）和下载按钮
    - GET /s/{token}/raw — 直接输出原始内容（供 img 标签 src 引用）
    - GET /s/{token}/download — 触发下载（Content-Disposition）
      - 验证 Token（同上面检查）
      - 记录下载日志（IP、User-Agent、时间）
      - 增加下载计数
      - 流式传输文件（支持大文件，io.CopyN）
    - `web/templates/download.html`: 下载页面（magick.css 魔法书风格）
    - `web/templates/download-text.html`: 文本分享展示模板
    - `web/templates/download-image.html`: 图片预览模板
    - `web/templates/download-expired.html`: 过期页面
    - `web/templates/download-password.html`: 密码输入页面
  - 响应头设置：预览用 Content-Type，下载用 Content-Disposition

  **Must NOT do**:
  - 不要将整个文件加载到内存（流式传输）
  - 不要暴露文件真实路径
  - 不要允许目录遍历
  - 图片预览不要缩放原图（直接输出原图）

  **Recommended Agent Profile**:
  - Category: `deep`
  - Reason: 核心分享逻辑，多种内容类型，安全性重要
  - Skills: `[]`

  **Parallelization**:
  - Can Run In Parallel: YES
  - Parallel Group: Wave 2
  - Blocks: Task 12
  - Blocked By: Tasks 3, 10

  **References**:
  - io.CopyN streaming: https://pkg.go.dev/io#CopyN
  - Content-Disposition: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Disposition
  - Go net/http ServeContent: https://pkg.go.dev/net/http#ServeContent
  - magick.css code blocks for text display

  **Acceptance Criteria**:
  - [x] 访问有效文件分享链接显示文件信息和下载按钮
  - [x] **访问文本分享链接直接显示文本内容**（在 `<pre>` 中）
  - [x] **文本分享可下载为 .txt 文件**
  - [x] **访问图片分享链接直接显示图片预览**
  - [x] 密码保护页面正确显示密码输入框
  - [x] 正确密码 → 允许访问
  - [x] 错误密码 → 拒绝并提示
  - [x] 过期链接 → 显示过期页面
  - [x] 流式下载大文件不占用大量内存
  - [x] `go test ./internal/handler/` → PASS

  **QA Scenarios**:
  ```
  Scenario: Download file with password
    Tool: Bash (curl)
    Preconditions: File share created with password "mypass"
    Steps:
      1. GET share page: `curl http://localhost:8080/s/{token}`
      2. Verify HTML contains password form
      3. POST wrong password: `curl -d "password=wrong" http://localhost:8080/s/{token}`
      4. Verify response shows error message
      5. POST correct password: `curl -c dl-cookies.txt -d "password=mypass" http://localhost:8080/s/{token}`
      6. GET download: `curl -b dl-cookies.txt http://localhost:8080/s/{token}/download -o /tmp/downloaded.txt`
      7. Verify file content matches original
    Expected Result: Wrong password rejected, correct password downloads file
    Evidence: .omo/evidence/task-11-download.txt

  Scenario: Text share displays content inline
    Tool: Bash (curl)
    Preconditions: Text share created with content "Hello World"
    Steps:
      1. GET share page: `curl http://localhost:8080/s/{text-token}`
      2. Verify HTML contains `<pre>` with "Hello World"
      3. Verify download link for .txt is present
    Expected Result: Text content displayed inline
    Evidence: .omo/evidence/task-11-text.txt

  Scenario: Image share shows preview
    Tool: Bash (curl)
    Preconditions: Image share created (e.g., test.png)
    Steps:
      1. GET share page after password: `curl -b dl-cookies.txt http://localhost:8080/s/{image-token}`
      2. Verify HTML contains `<img>` tag with src pointing to /s/{token}/raw
      3. GET raw: `curl http://localhost:8080/s/{image-token}/raw -o /tmp/preview.png`
      4. Verify file is a valid PNG (check file header bytes)
    Expected Result: Image preview shown in browser
    Evidence: .omo/evidence/task-11-image.txt
  ```

  **Commit**: YES (grouped with Task 12)

- [x] 12. **下载统计服务**

  **What to do**:
  - `internal/service/stats.go`: 统计服务
    - `RecordDownload(shareID, ip, userAgent)`: 记录到 download_logs 表
    - `GetShareStats(shareID)`: 获取某分享的总下载数、独立IP数、下载趋势
    - `GetGlobalStats()`: 全系统统计（总文件数、总下载量、活跃分享数）
  - 下载日志字段：share_id, ip_address, user_agent, downloaded_at
  - `internal/handler/stats.go`: 统计 API
    - GET /admin/api/stats — 返回 JSON 统计（给仪表盘用）
  - 日志自动清理：配置中可设置保留天数（默认永久）
  - 编写测试

  **Must NOT do**:
  - 不要做 IP 定位
  - 不要记录文件内容

  **Recommended Agent Profile**:
  - Category: `quick`
  - Reason: 标准数据库 CRUD + 简单聚合查询
  - Skills: `[]`

  **Parallelization**:
  - Can Run In Parallel: YES
  - Parallel Group: Wave 2 (with Tasks 8-15)
  - Blocks: Task 13
  - Blocked By: Tasks 2, 11

  **References**:
  - SQL GROUP BY queries for aggregation
  - Go time for trend data

  **Acceptance Criteria**:
  - [x] 每次下载记录 IP 和 User-Agent
  - [x] 分享统计返回总下载数
  - [x] 全局统计返回正确汇总
  - [x] `go test ./internal/service/` → PASS

  **QA Scenarios**:
  ```
  Scenario: Download counted correctly
    Tool: Bash (curl)
    Preconditions: Share exists, downloaded 3 times from Task 11
    Steps:
      1. `curl -b cookies.txt http://localhost:8080/admin/shares/{id}`
      2. Verify download count shows correct number
    Expected Result: Download count matches actual downloads
    Evidence: .omo/evidence/task-12-stats.txt
  ```

  **Commit**: YES (grouped with Task 11)
  - Message: `feat: share download handler with password/expiry and download statistics`
  - Files: `internal/handler/download.go`, `internal/service/stats.go`

- [x] 13. **管理后台仪表盘 + 统计展示**

  **What to do**:
  - `internal/handler/admin.go`: 仪表盘处理器
    - GET /admin/ — 仪表盘首页
  - 仪表盘内容：
    - 概览卡片：文件总数、分享总数、总下载量、活跃分享
    - 最近上传文件列表（5 条）
    - 最近下载记录（10 条）
    - 存储使用量（总大小）
  - `web/templates/dashboard.html`: 仪表盘模板
    - magick.css 卡片布局、数据表格
    - 响应式设计（移动端竖排卡片）
  - 仪表盘作为登录后默认页面

  **Must NOT do**:
  - 不要用图表库（保持单二进制无前端依赖）
  - 不要做实时数据

  **Recommended Agent Profile**:
  - Category: `visual-engineering`
  - Reason: UI 设计 + 数据展示
  - Skills: `[]`

  **Parallelization**:
  - Can Run In Parallel: YES
  - Parallel Group: Wave 2
  - Blocks: Task 16
  - Blocked By: Tasks 6, 9, 12

  **References**:
  - Task 6 template patterns
  - SQL aggregation queries from Task 12

  **Acceptance Criteria**:
  - [x] 仪表盘显示所有统计卡片
  - [x] 最近文件列表正确
  - [x] 最近下载记录正确
  - [x] 移动端显示正常
  - [x] `go test ./internal/handler/` → PASS

  **QA Scenarios**:
  ```
  Scenario: Dashboard loads and shows data
    Tool: Bash (curl)
    Preconditions: Files uploaded, shares created, downloads happened
    Steps:
      1. `curl -b cookies.txt http://localhost:8080/admin/`
      2. Grep for "Files", "Shares", "Downloads" counts
      3. Verify numbers match expected
    Expected Result: Dashboard shows accurate statistics
    Evidence: .omo/evidence/task-13-dashboard.txt
  ```

  **Commit**: YES (grouped with Tasks 14-15)

- [x] 14. **系统配置页面**

  **What to do**:
  - `internal/handler/config.go`: 配置管理处理器
    - GET /admin/config — 配置页面
    - POST /admin/config — 保存配置
  - 可配置项（在管理页面修改）：
    - 端口（需要重启生效，页面提示）
    - 最大上传大小
    - Token 长度
    - 分享默认过期时间
    - Session 超时时间
    - 下载日志保留天数
  - 配置修改后立即保存到 DB
  - `web/templates/config.html`: 配置表单模板
    - 分节显示配置项
    - 保存成功提示（HTMX flash message）
  - 修改端口提示用户手动重启

  **Must NOT do**:
  - 不要实现热重启
  - 不要修改数据库连接等核心配置

  **Recommended Agent Profile**:
  - Category: `quick`
  - Reason: 标准表单 CRUD
  - Skills: `[]`

  **Parallelization**:
  - Can Run In Parallel: YES
  - Parallel Group: Wave 2
  - Blocks: None
  - Blocked By: Tasks 4, 6

  **References**:
  - Task 4 config persistence pattern
  - Task 6 template layout

  **Acceptance Criteria**:
  - [x] 配置页面显示所有配置项
  - [x] 修改后保存到数据库
  - [x] 重启后配置持久化
  - [x] `go test ./internal/handler/` → PASS

  **QA Scenarios**:
  ```
  Scenario: Change config via web
    Tool: Bash (curl)
    Preconditions: Auth working
    Steps:
      1. `curl -b cookies.txt http://localhost:8080/admin/config`
      2. Verify form shows current max upload size
      3. `curl -b cookies.txt -X POST -d "max_upload_size=2147483648&token_length=12&..." http://localhost:8080/admin/config`
      4. `curl -b cookies.txt http://localhost:8080/admin/config`
      5. Verify updated values shown in form
    Expected Result: Config updated and persists
    Evidence: .omo/evidence/task-14-config.txt
  ```

  **Commit**: YES (grouped with Task 13, 15)

- [x] 15. **安全加固 — CSRF, 限速, Headers**

  **What to do**:
  - CSRF 保护：
    - 所有 POST/PUT/DELETE 请求验证 CSRF Token
    - CSRF Token 嵌入表单（隐藏字段）或通过 HTMX header
    - 使用 `justinas/nosurf` — 轻量级 CSRF 中间件，兼容 net/http
  - 速率限制：
    - 登录尝试限速：同一 IP 5 次/分钟
    - 下载限速：同一 IP 按配置限制
    - 上传限速：按配置限制
    - 使用 `ulule/limiter/v3` — 内存后端 + HTTP 中间件
  - 安全 HTTP Headers：
    - X-Content-Type-Options: nosniff
    - X-Frame-Options: DENY
    - Content-Security-Policy: 严格策略
    - Referrer-Policy: no-referrer
    - Strict-Transport-Security (HTTPS 时)
  - 路径遍历防护：验证文件路径在 data-dir 范围内
  - `internal/middleware/security.go`: 安全中间件合集

  **Must NOT do**:
  - 不要使用外部的 WAF
  - 不要实现 HTTPS（交给反向代理）

  **Recommended Agent Profile**:
  - Category: `unspecified-high`
  - Reason: 安全相关，多种中间件组合
  - Skills: `[]`

  **Parallelization**:
  - Can Run In Parallel: YES
  - Parallel Group: Wave 2
  - Blocks: Task 17
  - Blocked By: Task 5

  **References**:
  - OWASP Go security: https://cheatsheetseries.owasp.org/cheatsheets/Go_Security_Cheat_Sheet.html
  - CSP guide: https://developer.mozilla.org/en-US/docs/Web/HTTP/CSP
  - justinas/nosurf: https://github.com/justinas/nosurf (轻量 CSRF)
  - ulule/limiter v3: https://github.com/ulule/limiter (HTTP 速率限制)
  - SCS session store + nosurf integration example

  **Acceptance Criteria**:
  - [x] CSRF Token 在 POST 表单中有效
  - [x] 无 CSRF Token 的 POST 被拒绝 403
  - [x] 登录失败 5 次后临时锁定
  - [x] 安全 Headers 出现在所有响应中
  - [x] 路径遍历攻击被拦截
  - [x] `go test ./internal/middleware/` → PASS

  **QA Scenarios**:
  ```
  Scenario: CSRF protection works
    Tool: Bash (curl)
    Preconditions: Auth working
    Steps:
      1. Try POST without CSRF token: `curl -b cookies.txt -X POST -d "password=newpass" http://localhost:8080/admin/config`
      2. Verify 403 Forbidden
      3. Get CSRF token from login page
      4. POST with token: `curl -b cookies.txt -X POST -d "password=newpass&csrf_token=..." http://localhost:8080/admin/config`
      5. Verify 200 OK
    Expected Result: CSRF blocks forged requests
    Evidence: .omo/evidence/task-15-csrf.txt

  Scenario: Rate limiting on login
    Tool: Bash (curl)
    Preconditions: Server running
    Steps:
      1. POST login 6 times with wrong password in quick succession
      2. 6th attempt returns 429 Too Many Requests
    Expected Result: Rate limiting prevents brute force
    Evidence: .omo/evidence/task-15-ratelimit.txt
  ```

  **Commit**: YES (grouped with Tasks 13, 14)
  - Message: `feat: admin dashboard, config page, and security hardening`
  - Files: `internal/handler/admin.go`, `internal/handler/config.go`, `internal/middleware/security.go`

- [x] 16. **移动端适配完善**

  **What to do**:
  - 检查所有管理页面在移动端（320px - 768px）的显示效果
  - magick.css 本身响应式，但需要：
    - 导航栏：添加手机端折叠菜单（`<details>` 原生实现）
    - 文件/分享列表：窄屏幕时表格转为文字堆叠显示
    - 表单：确保输入框和按钮在手机上触控友好（min-height: 44px）
  - `web/static/custom.css`: 移动端覆盖样式
    - 表格在小屏幕显示优化
    - 上传进度条样式
    - 文件列表卡片样式
  - 测试移动端 curl + 检查视口设置

  **Must NOT do**:
  - 不要引入前端框架
  - 不要破坏桌面端体验

  **Recommended Agent Profile**:
  - Category: `visual-engineering`
  - Reason: 前端 UI 适配
  - Skills: `[]`

  **Parallelization**:
  - Can Run In Parallel: YES
  - Parallel Group: Wave 3 (with Tasks 17, 18)
  - Blocks: None
  - Blocked By: Tasks 6, 13, 10

  **References**:
  - magick.css 本身在所有屏幕尺寸保持美观
  - CSS media queries

  **Acceptance Criteria**:
  - [x] 所有页面在 375px 宽度下正常显示
  - [x] 导航可折叠/展开
  - [x] 文件列表在手机上可读
  - [x] 分享下载页面移动端正常

  **QA Scenarios**:
  ```
  Scenario: Mobile viewport renders correctly
    Tool: Bash (curl) with mobile User-Agent
    Preconditions: All pages functional
    Steps:
      1. `curl -b cookies.txt -H "User-Agent: Mozilla/5.0 (iPhone; CPU iPhone OS 16_0)" http://localhost:8080/admin/files`
      2. Verify HTML contains viewport meta tag
      3. Check nav contains collapsible menu elements
    Expected Result: Mobile-friendly HTML structure
    Evidence: .omo/evidence/task-16-mobile.txt
  ```

  **Commit**: YES (grouped with Tasks 17, 18)

- [x] 17. **错误页面 + 全局错误处理**

  **What to do**:
  - `internal/handler/errors.go`: 自定义错误处理器
    - 404 页面（magick.css 巫师风格 — "这页魔法书不见了"）
    - 500 页面
    - 403 页面（"你没有权限施展这个咒语"）
  - `internal/middleware/recovery.go`: panic 恢复中间件
    - 生产环境 panic → 500 页面 + slog 日志
    - 开发环境 panic → 输出详细信息
  - 全局错误处理中间件：
    - 统一 JSON 错误响应（API 路由）
    - 统一 HTML 错误页面（页面路由）
  - 日志：Go 标准库 `log/slog`，输出到 stderr
  - `web/templates/error-404.html`, `error-500.html`, `error-403.html`

  **Must NOT do**:
  - 不要在错误页面暴露堆栈信息（生产环境）

  **Recommended Agent Profile**:
  - Category: `quick`
  - Reason: 模式固定的错误页面
  - Skills: `[]`

  **Parallelization**:
  - Can Run In Parallel: YES
  - Parallel Group: Wave 3
  - Blocks: None
  - Blocked By: Task 15

  **References**:
  - Go log/slog: https://pkg.go.dev/log/slog

  **Acceptance Criteria**:
  - [x] 404 页面返回自定义错误页
  - [x] 500 页面不暴露堆栈
  - [x] panic 被 recover 不崩溃
  - [x] `go test ./internal/middleware/` → PASS

  **QA Scenarios**:
  ```
  Scenario: 404 page shows custom design
    Tool: Bash (curl)
    Preconditions: Server running
    Steps:
      1. `curl http://localhost:8080/nonexistent-page`
      2. Verify response contains "404" and custom error message
      3. Verify response status code is 404
    Expected Result: Custom 404 page shown
    Evidence: .omo/evidence/task-17-404.txt
  ```

  **Commit**: YES (grouped with Task 16, 18)

- [x] 18. **README + 构建脚本**

  **What to do**:
  - `README.md`: 完整使用文档
    - 项目简介和截图
    - 快速开始（下载、运行、密码设置）
    - 构建方法（`go build -o fls .`）
    - CLI flag 说明
    - 配置方法（Web 页面）
    - 分享链接使用说明
    - 扩展指南
  - `Makefile`:
    - `build` — 编译当前平台
    - `build-all` — 交叉编译（linux-amd64, linux-arm64, darwin-amd64, windows-amd64）
    - `test` — 运行测试
    - `clean` — 清理
  - 交叉编译使用 Go 标准 `GOOS`/`GOARCH` 环境变量

  **Must NOT do**:
  - 不要写过多开发文档

  **Recommended Agent Profile**:
  - Category: `writing`
  - Reason: 文档和构建脚本
  - Skills: `[]`

  **Parallelization**:
  - Can Run In Parallel: YES
  - Parallel Group: Wave 3
  - Blocks: None
  - Blocked By: Task 1-17

  **References**:
  - Go cross-compilation: `GOOS=linux GOARCH=amd64 go build`
  - Makefile best practices

  **Acceptance Criteria**:
  - [x] README 包含安装和使用说明
  - [x] `make build` 编译成功
  - [x] `make test` 通过所有测试
  - [x] `go test ./...` → PASS

  **QA Scenarios**:
  ```
  Scenario: Build from README instructions
    Tool: Bash
    Preconditions: Go 1.22+ installed
    Steps:
      1. `go build -o fls .`
      2. Check binary exists and runs
    Expected Result: README build instructions work
    Evidence: .omo/evidence/task-18-build.txt
  ```

  **Commit**: YES (grouped with Task 16, 17)
  - Message: `feat: mobile responsiveness, error handling, and documentation`
  - Files: `README.md`, `Makefile`, `web/templates/error-*.html`, `internal/middleware/recovery.go`

---

## Final Verification Wave

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
>
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, curl endpoint, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .omo/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go vet ./...` + `golangci-lint` + `go test ./...`. Review all changed files for: `interface{}` (use `any`), `error` handling with blank `_`, `panic` in non-main code, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names.
  Output: `Vet [PASS/FAIL] | Lint [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Full QA Execution** — `unspecified-high`
  Start from clean state (no data dir). Execute first-run setup → login → upload file → create share → download via share → verify password rejection → verify expired share rejection. **Also test: text share creation → inline text display → image share preview → verify QR code generated on share detail page.** Execute EVERY QA scenario from EVERY task. Save to `.omo/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **T1-T7** (Wave 1): `feat: initial project scaffolding with database, auth, and templates`
- **T8-T15** (Wave 2): `feat: core features - upload, management, sharing, download, stats, security`
- **T16-T18** (Wave 3): `feat: mobile polish, error handling, and documentation`
- **F1-F4**: (review only, no code commit)

---

## Success Criteria

### Verification Commands
```bash
go build -o fls .          # Build: exits 0
go vet ./...               # Static analysis: exits 0
go test ./...               # Tests: exits 0, all pass
./fls                       # First-run setup wizard appears
# After setup: serve on :8080
curl http://localhost:8080  # Redirects to login page
```

### Final Checklist
- [x] 单二进制编译成功
- [x] 首次运行密码设置向导正常
- [x] 登录/登出功能正常
- [x] 文件上传（普通 + TUS分片）正常
- [x] 文件列表/详情/删除正常
- [x] **文本分享创建/展示正常**
- [x] **图片分享自动预览正常**
- [x] 分享链接创建（设置密码+过期）正常
- [x] **分享详情页显示二维码**
- [x] 分享密码验证（正确通过/错误拒绝）正常
- [x] 过期分享链接自动拒绝下载
- [x] 下载次数统计正确
- [x] 系统配置修改后持久化
- [x] 移动端浏览正常
- [x] 所有测试通过
