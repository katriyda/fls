---
name: frontend-design
description: 高品质前端界面设计指导，适用于本项目的 Go html/template 页面、HTMX 交互、自定义 CSS。当用户要求构建或美化 web 页面、组件、UI 时使用。避免 AI 味的通用设计。
license: Apache-2.0
---

此 Skill 指导创建具有鲜明设计风格的高品质前端界面。本项目前端基于 Go `html/template` + HTMX + CSS，模板位于 `web/templates/`，静态文件位于 `web/static/custom.css`。

用户可能提出：美化管理后台、改进分享页面样式、重新设计登录页、创建自定义错误页面等。

## 设计思路

动手前，先理解上下文并确定鲜明的美学方向：

- **目的**：这个界面解决什么问题？用户是谁？
- **风格**：选择一个极端方向 — 极简、粗野主义、复古未来、精致奢华、趣味玩具风、编辑杂志风、柔和粉彩、工业实用风等。避免模棱两可。
- **约束**：本项目使用 Go `html/template` + HTMX，**CSP 限制 `script-src` 必须包含 `'unsafe-inline'` 和 `'unsafe-hashes'`**，所有样式写在 `custom.css` 中。
- **差异化**：什么让这个设计令人难忘？选一个核心记忆点。

**关键**：选择一个清晰的概念方向并精确执行。大胆的极繁主义和克制的极简主义都有效 — 关键在于意图明确，而非强度。

## 前端美学指南

### 排版
- 选择有个性、有特色的字体。避免 Inter、Arial、系统字体等泛泛之选。
- 字体文件存放在 `web/static/fonts/` 目录下（woff2 格式），通过 `@font-face` 在 `custom.css` 中引入。当前使用 DM Sans（正文）和 JetBrains Mono（等宽）。
- 用有表现力的展示字体搭配精致的正文字体。

### 色彩与主题
- 使用 CSS 变量 (`:root`) 保持一致性。
- 主色调占据主导，搭配锐利强调色 — 比均匀分布的配色更有冲击力。
- 本项目已通过 `custom.css` 管理全局样式。

### 动效
- 优先使用纯 CSS 动画（Go 模板中嵌入更简单）。
- HTMX 本身支持 `hx-swap` 动画效果。
- 聚焦高冲击时刻：一个精心编排的加载动画（`animation-delay` 交错呈现）比分散的微交互更讨喜。
- 悬停效果和状态切换要给人惊喜感。

### 空间构图
- 非对称布局、重叠元素、对角线流动、破网格的元素。
- 慷慨的留白或有控制的密度。

### 背景与细节
- 用 CSS 渐变、杂色纹理（`background-image` + SVG data URI）、几何图案营造氛围。
- 避免纯白色背景。
- 可考虑：分层透明度、戏剧性阴影、颗粒覆盖层。

**永远不要**使用 AI 通用审美：Inter/Roboto/Arial、紫白渐变配色、可预测的布局模式。

## 本项目前端文件结构

```
web/
├── templates/         # *.html 模板（Go html/template）
│   ├── layout.html    # 基础布局 + 导航
│   ├── login.html     # 登录页
│   ├── dashboard.html # 管理后台首页
│   ├── files.html     # 文件列表
│   ├── shares.html    # 分享列表
│   ├── share-detail.html  # 新建/编辑分享
│   ├── config.html    # 系统配置
│   ├── download*.html # 公开分享页面（6 种变体）
│   └── error.html     # 404/500 错误页
└── static/
    └── custom.css     # 自定义样式（所有样式在此）
```

### 设计注意事项
- 所有页面继承 `layout.html`，导航栏通过 `.Authenticated` 控制显示
- CSRF token 通过 `{{ .CSRFToken }}` 注入
- HTMX 请求使用 `HX-Redirect` 头进行重定向
- 错误页面使用 `ErrorData` 结构体，必须包含 `Authenticated` 字段
