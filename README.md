# FLS - 文件分享系统

文件分享系统（File Sharing System），基于 Go 语言实现，支持大文件断点续传、文件管理、分享链接、二维码生成等功能。

## 功能特性

- 🚀 TUS 断点续传 — 支持 >2GB 大文件上传
- 📁 文件管理 — 上传、查看、重命名、删除
- 🔗 分享链接 — 文件/文本分享，支持密码保护、过期时间
- 📱 图片预览 — 分享图片直接在浏览器预览
- 📄 文本分享 — 粘贴板风格文本分享
- 🎯 QR码 — 每个分享自动生成二维码
- 📊 管理面板 — 仪表盘、文件管理、分享管理
- 🔒 安全 — 会话认证、CSRF 保护、速率限制
- 💾 SQLite 存储 — 单二进制部署，无需外部数据库

## 快速开始

**构建：**

```bash
# Windows
go build -o fls.exe .

# Linux/macOS
make
```

**运行：**

```bash
./fls.exe --port 8080 --data-dir ./data
```

首次运行会自动在终端提示设置管理员密码。访问 `http://localhost:8080/admin/` 登录管理面板。

## 配置

所有配置存储在 SQLite 数据库中，可通过 `/admin/config` 页面修改配置。配置项包括：

- 端口
- 上传大小限制
- 分享过期时间
- 其他系统参数

## 命令行参数

| 参数 | 缩写 | 说明 | 默认值 |
|------|------|------|--------|
| `--port` | `-p` | 监听端口 | `8080` |
| `--data-dir` | `-d` | 数据目录 | `./data` |

## API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/admin/` | 仪表盘 |
| `GET` | `/admin/files` | 文件列表 |
| `GET` | `/admin/files/{id}` | 文件详情 |
| `DELETE` | `/admin/files/{id}` | 删除文件 |
| `POST` | `/api/upload/simple` | 简单上传 |
| `POST` | `/api/upload/tus` | TUS 创建上传 |
| `PATCH` | `/api/upload/tus/{id}` | TUS 上传分片 |
| `HEAD` | `/api/upload/tus/{id}` | TUS 查询进度 |
| `DELETE` | `/api/upload/tus/{id}` | TUS 取消上传 |
| `GET` | `/admin/shares` | 分享列表 |
| `POST` | `/admin/shares` | 创建分享 |
| `GET` | `/admin/shares/{id}` | 分享详情 |
| `DELETE` | `/admin/shares/{id}` | 删除分享 |
| `GET` | `/admin/shares/{id}/qrcode` | 分享 QR码 |
| `GET` | `/admin/config` | 系统配置 |
| `POST` | `/admin/config` | 更新配置 |
| `GET` | `/admin/api/stats` | 统计数据 |
| `GET` | `/s/{token}` | 查看分享 |
| `POST` | `/s/{token}` | 验证分享密码 |
| `GET` | `/s/{token}/raw` | 原始内容 |
| `GET` | `/s/{token}/download` | 下载文件 |

## 技术栈

- **后端：** Go 1.22+, Chi Router, modernc.org/sqlite
- **前端：** Go html/template, HTMX, magick.css
- **认证：** SCS Session, bcrypt
- **二维码：** yeqown/go-qrcode/v2
- **安全：** nosurf (CSRF), ulule/limiter (速率限制)

## 项目结构

```
fls/
├── main.go              # 入口文件，路由注册
├── internal/
│   ├── config/          # 配置管理
│   ├── database/        # SQLite 数据库
│   ├── handler/         # HTTP 处理器
│   ├── middleware/      # 中间件 (Auth, Security)
│   ├── model/           # 数据模型
│   ├── service/         # 业务逻辑层
│   └── tus/             # TUS 协议实现
├── web/
│   ├── static/          # 静态资源 (CSS)
│   ├── templates/       # HTML 模板
│   └── embed.go         # 静态文件嵌入
└── data/                # 数据目录 (自动创建)
```

## 构建

### Windows (PowerShell)

```powershell
.\build.ps1
```

输出到 `./dist/fls.exe`，包含版本号和提交信息。

### Linux/macOS (Make)

```bash
make            # 构建当前平台
make build-linux      # Linux amd64 交叉编译
make build-darwin     # macOS amd64 交叉编译
make build-darwin-arm # macOS ARM64 交叉编译
make build-windows    # Windows amd64 交叉编译
make clean            # 清理构建产物
```

输出到 `./dist/` 目录，文件名包含目标平台信息。
