---
name: ui-review
description: 审查前端 UI 样式一致性。当用户要求"检查 UI"、"审查样式"、"ui-review"、"检查前端一致性"、"审查设计系统"时使用。扫描模板和 CSS 中的视觉不一致问题。
license: Apache-2.0
---

# UI 样式一致性审查

对项目前端执行系统性的一致性审查。扫描 `web/templates/*.html` 和 `web/static/custom.css`，找出破坏视觉统一的问题。

## 审查流程

### Step 1: 读取设计系统定义

读取 `web/static/custom.css` 中的 `:root` 变量块，提取：
- 类型比例尺（`--text-*` 变量）
- 间距比例尺（`--space-*` 变量）
- 组件尺寸（`--btn-height*`、`--icon-*` 变量）
- 颜色语义变量（`--color-*`）

这些是设计系统的唯一真实来源。所有偏差都应相对于这些值来判断。

### Step 2: 扫描模板文件

读取 `web/templates/` 下所有 `.html` 文件。对每个模板执行以下 10 项检查。

### Step 3: 输出报告

按严重级别分组输出发现。每个发现包含：类别、文件:行号、问题描述、建议修复。

---

## 检查类别

### 1. Token 合规 (token-compliance)

扫描 CSS 和模板中的硬编码值：

- **颜色**: `#hex`、`rgb()`、`hsl()`、CSS 命名颜色（`red`、`black` 等）— 应使用 `var(--color-*)` 变量
- **字号**: `font-size` 的 `rem`/`px` 字面量 — 应使用 `var(--text-*)` 变量
- **间距**: `margin`、`padding`、`gap` 的 `rem`/`px` 字面量 — 应使用 `var(--space-*)` 变量
- **组件尺寸**: `height`、`min-height` 的 `rem`/`px` 字面量 — 应使用 `var(--btn-height*)`、`var(--icon-*)` 变量

**排除**: `0`、`0px`、`1px`（边框）、`100%`、`auto`、`none`、`transparent` 不算违规。

**严重级别**: 模板中的硬编码值 = warning；CSS 中绕过变量的值 = nit。

### 2. 跨页面视觉一致性 (visual-consistency)

比较同类组件在不同模板中的使用：

- **按钮**: 同一逻辑操作（返回、删除、分页、复制）在不同页面是否使用相同的 `btn`/`btn-sm`/`btn-lg`/`btn-link` + `btn-primary`/`btn-danger`/`btn-warning` 组合
- **卡片**: 所有 `.card` 是否有相同的 padding、border、shadow
- **表格**: `.data-table` 的 `<td>` 是否都有 `data-label` 属性（移动端需要）
- **表单**: `.form-section`、`.form-group` 是否一致使用
- **详情页操作行**: 所有详情页底部的"返回+删除"按钮组是否使用相同的 `btn-sm` + `flex gap-sm` 模式

**检查方法**: 提取每个模板中同类组件的 CSS 类组合，找出不一致的实例。

**严重级别**: 同一操作在不同页面尺寸不同 = critical；同类组件 CSS 类组合不同 = warning。

### 3. 符号一致性 (symbol-consistency)

扫描模板中的可见文本，检查：

- **返回导航**: 是否统一使用纯文字（不加 `←`、`«`、`←` 等箭头符号）
- **分页**: 上一页/下一页是否统一有或无 `«`/`»` 符号
- **连接符**: 范围表示是否统一（`-` vs `–` vs `—` vs `至`）
- **省略号**: 是否统一使用 `…` 或 `...`，不混用
- **引号**: UI 文本中的引号是否统一（`""` vs `""` vs `「」`）

**严重级别**: 同一操作使用不同符号 = warning；不同上下文的符号差异 = nit。

### 4. 内联样式 (inline-styles)

扫描模板中的：

- `style="..."` 属性 — 应改为 CSS 类
- JS 中的 `element.style.*` 赋值 — 应改为 `classList` 操作
- CSS 中的 `!important` — 通常表示特异性问题

**排除**: `display: none` 用于 JS 控制的元素切换是可接受的（但应标注为 JS-controlled）。

**严重级别**: 模板中的内联样式 = warning；JS 中的 `.style` 操作 = nit。

### 5. 缺失/死定义 (orphaned-definitions)

- **模板引用但 CSS 未定义的类**: 提取模板中 `class="..."` 的所有类名，检查 CSS 中是否有对应的规则
- **CSS 定义但模板未使用的类**: 提取 CSS 中所有选择器的类名，检查模板中是否有引用

**严重级别**: 模板引用未定义类 = critical（功能可能缺失）；CSS 死代码 = nit。

### 6. 状态覆盖 (state-coverage)

检查交互元素是否有完整状态：

- **按钮** (`.btn*`): 是否有 `:hover` 和 `:active` 状态
- **链接** (`<a>`): 是否有 `:hover` 状态
- **输入框**: 是否有 `:focus` 状态
- **卡片** (有 `transition` 的): hover 效果是否定义

**严重级别**: 主要按钮缺少 hover 状态 = warning；次要元素缺少状态 = nit。

### 7. 响应式一致性 (responsive-consistency)

- **断点值**: 是否统一使用项目定义的断点（900px、600px）
- **固定宽度**: 是否有硬编码的 `width` 值应该用 `max-width` + 百分比
- **移动端表格**: `.data-table` 在 `<600px` 是否都有 `data-label` 支持

**严重级别**: 不一致的断点 = warning；缺少移动端支持 = warning。

### 8. 按钮重量层级 (button-weight-hierarchy)

检查按钮的视觉重量是否正确反映操作的重要性。业界标准层级：

| 操作类型 | 视觉重量 | 允许的样式 |
|---|---|---|
| 主要 CTA（保存、登录、下载） | 最高 | `btn-primary`（填充） |
| 破坏性操作（删除） | 高 | `btn-danger`（填充） |
| 次要操作（查看、返回、编辑） | 中 | `btn-sm`（outline） |
| 内联操作（复制、toggle 开关） | 低 | `btn-link`（文字链接）或 `btn-sm` outline |

**检查规则**:
- `btn-primary`（填充）只用于表单提交、主要 CTA，不应用于 toggle/开关操作
- `btn-danger`（填充）只用于删除等破坏性操作
- Toggle/开关操作（如公开/取消公开）应使用 outline 样式（`btn-sm` 或 `btn-warning`），不应比删除按钮更醒目
- 复制、剪贴板等轻量操作应使用 `btn-link` 样式
- 同一行内的多个按钮，视觉重量应与操作重要性成正比

**检查方法**: 扫描所有 `.btn*` 的使用上下文。如果一个 toggle 操作使用了 `btn-primary`（填充），而同页面的删除操作使用 `btn-danger`（填充），则两者视觉重量相同 — 这是错误的，toggle 应更轻。

**严重级别**: toggle 操作使用填充样式（比删除更重或相同）= critical；次要操作使用填充样式 = warning。

### 9. 内嵌按钮间距 (inline-button-spacing)

检查按钮与相邻内容之间是否有适当的间距。

**检查规则**:
- 按钮与相邻的 `<code>`、`<input>`、文字之间必须有间距类（`ml-sm`、`gap-sm` 等）
- 表格操作列（`<td>` 内）多个按钮之间必须有间距（通过 `.btn + .btn` CSS 规则或 flex gap）
- 按钮不应紧贴（flush）任何相邻元素

**检查方法**: 扫描模板中 `<button` 和 `<a class="btn` 的前一个兄弟元素，如果紧邻 `<code>`、`<input>`、文字且没有 `ml-sm`/`gap-*` 等间距类，则报告。

**严重级别**: 按钮紧贴相邻内容（无间距）= warning。

### 10. 堆叠元素 (stacked-elements)

检查 `<td>` 或其他容器内是否堆叠了多个独立的视觉元素。

**检查规则**:
- 同一个 `<td>` 内不应同时存在 badge + button 等多个独立元素
- 状态指示和操作按钮应合并为单一元素（用按钮样式表达状态，如 warning 色 + 图标前缀）
- 如果一个 `<td>` 内有超过 1 个交互元素（按钮、链接），应考虑合并或简化

**检查方法**: 扫描所有 `<td>` 元素，计算其中的 `button`、`a.btn`、`.badge` 元素数量。超过 1 个交互元素或 badge + button 组合则报告。

**严重级别**: badge + button 堆叠在同一个 td = critical；多个按钮在同一个 td 无分组 = warning。

---

## 输出格式

```
## UI 样式审查报告

### Critical (N 项)

**[类别]** `文件:行号`
问题描述。
→ 建议修复方式。

### Warning (N 项)

**[类别]** `文件:行号`
问题描述。
→ 建议修复方式。

### Nit (N 项)

**[类别]** `文件:行号`
问题描述。
→ 建议修复方式。

---
共发现 N 项问题：N critical, N warning, N nit。
```

## 审查原则

- 只报告真正的不一致，不报告设计选择（如 primary 比 default 视觉更重是有意为之）
- 同一操作在不同页面必须一致（如"返回列表"按钮在所有详情页应相同）
- 如果差异有合理理由，在报告中标注为"可接受"
- 优先报告影响用户体验最大的问题（尺寸不一致 > 符号差异 > 代码风格）
