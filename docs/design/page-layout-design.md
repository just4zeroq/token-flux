# AI Platform 用户端页面布局设计

> 设计系统：Flat Design | 简约科技风 | Dark First
> 生成工具：UI/UX Pro Max Design Intelligence

---

## 设计系统总览

### Style
**Flat Design** — 2D、极简、无阴影、线条干净、排版驱动、图标轻盈。

### Color Palette (Dark First)

| Token | Value | 用途 |
|-------|-------|------|
| `--color-primary` | `#8B5CF6` | 品牌主色、按钮、链接、激活态 |
| `--color-primary-light` | `#A78BFA` | Hover、次要强调 |
| `--color-accent` | `#FBBF24` | CTA 按钮、价格/收益高亮 |
| `--color-bg` | `#0F0F23` | 页面背景（深色） |
| `--color-surface` | `#1E1E3A` | 卡片、面板、表单背景 |
| `--color-muted` | `#27273B` | 次要背景、分割线 |
| `--color-border` | `#4C1D95` | 边框 |
| `--color-fg` | `#F8FAFC` | 正文 |
| `--color-fg-muted` | `#A0A0B8` | 辅助文字 |
| `--color-destructive` | `#EF4444` | 错误、删除 |

> **Light Mode**: 背景调为白底，primary 保持 #8B5CF6，surface 用 gray-50，文字用 slate-900。

### Typography

| Level | Font | Weight | Size |
|-------|------|--------|------|
| Display | `Geist Variable` / `Space Grotesk` | 700 | 4rem / 3rem |
| Heading 1 | `Geist Variable` | 600 | 2.5rem |
| Heading 2 | `Geist Variable` | 600 | 2rem |
| Heading 3 | `Geist Variable` | 600 | 1.5rem |
| Body | `Geist Variable` | 400 | 1rem |
| Small | `Geist Variable` | 400 | 0.875rem |
| Mono | `Geist Mono` / tabular-nums | 400 | 0.875rem |

> 现有项目已使用 Geist Variable，保留即可，无需替换。

### Effects
- 无阴影、无渐变 — 纯色块 + 间距驱动层次
- Hover：颜色/透明度变化 150ms ease-out
- 圆角：`rounded-lg` (0.5rem) 统一
- Icons：Lucide, 24px, stroke=2

### Layout System
- 容器宽度：max-w-7xl (desktop), px-4 mobile
- 间距：4/8/16/24/32/48/64 递进
- 断点：375 / 768 / 1024 / 1440

---

## 页面布局

### 1. 公共布局 (Shared Layout)

```
+--------------------------------------------------+
| Navbar (sticky, h-16)                            |
| [Logo]  [Models] [Providers] [Pricing]  [Console] |
|                              [Login / 用户名▼]    |
+--------------------------------------------------+
|                                                    |
|                  <Outlet />                        |
|                                                    |
+--------------------------------------------------+
| Footer                                            |
| [AI Platform ©2026]  [Docs] [API] [About]        |
+--------------------------------------------------+
```

**Navbar**:
- Dark bg `--color-bg`, bottom border `--color-border`
- Logo 左侧：SVG icon + "AI Platform" text
- 导航居中/左侧：Models, Providers, Pricing, Console
- 右侧：已登录 → 用户名 + Avatar + 下拉菜单；未登录 → Login 按钮
- Mobile: hamburger menu drawer

**Footer**:
- Dark bg, 上分割线
- 三列：产品（Models/Pricing）、资源（Docs/API/Status）、公司（About/Terms）
- Bottom bar：版权 + Social icons

---

### 2. 落地页 (Landing Page) — `/`

```
+--------------------------------------------------+
| HERO SECTION (全屏, 居中)                         |
|                                                    |
|    ____  ____  ____  ____                         |
|   /    \/    \/    \/    \                        |
|  | AI  | LLM | MCP | Agent|  ← Bento Grid Hero  |
|   \____/\____/\____/\____/                        |
|                                                    |
|   "Build the Next Generation of AI Applications"  |
|   一句话副标题：API keys、模型托管、结算一站式     |
|                                                    |
|       [Get Started]  [View Models ▸]              |
|                                                    |
+--------------------------------------------------+
| BENTO GRID — 能力展示                              |
|                                                    |
| ┌─────────┐ ┌─────────┐ ┌─────────┐              |
| │ LLM API  │ │ MCP     │ │ Agent   │              |
| │ Key托管  │ │ 协议网关 │ │ 运行时  │              |
| └─────────┘ └─────────┘ └─────────┘              |
| ┌─────────┐ ┌─────────┐ ┌─────────┐              |
| │ 双记账   │ │ 支付宝  │ │ 开发者  │              |
| │ 结算系统 │ │ 微信支付 │ │ 控制台  │              |
| └─────────┘ └─────────┘ └─────────┘              |
|                                                    |
|  每个卡片：icon + 标题 + 一句话说明               |
+--------------------------------------------------+
| MODEL SHOWCASE                                     |
|                                                    |
|  "Available Models"  (水平滚动 / 网格)             |
|                                                    |
| ┌──────────────┐ ┌──────────────┐ ┌──────────────┐|
| │ GPT-4o       │ │ Claude 4     │ │ DeepSeek     ||
| │ input ¥x/1K  │ │ input ¥x/1K  │ │ input ¥x/1K  ||
| │ output ¥x/1K │ │ output ¥x/1K │ │ output ¥x/1K ||
| │ [Details ▸]  │ │ [Details ▸]  │ │ [Details ▸]  ||
| └──────────────┘ └──────────────┘ └──────────────┘|
+--------------------------------------------------+
| CTA SECTION                                        |
|                                                    |
|   "Ready to Ship?"                                 |
|   注册即得 100 信用分，立即体验                     |
|        [Create Free Account]                       |
+--------------------------------------------------+
```

**Layout Notes**:
- Hero: 大号 display text (4rem) + Bento grid SVG hero graphic
- Bento Grid: 3×2 或 2×3 卡片网格，hover 轻微上移 + 边框高亮
- Model Showcase: 从后端获取热门模型列表，水平 scroll snap
- CTA: 纯色 purple bg + 白字按钮

---

### 3. 模型工厂 (Model Factory) — `/models`

```
+--------------------------------------------------+
| Navbar (shared)                                    |
+--------------------------------------------------+
|                                                    |
| 模型工厂                                            |
| search bar [________] [▼ 分类] [▼ 能力] [▼ 排序]   |
|                                                    |
| ┌──────┬──────┬──────┬──────┬──────┐              |
| │ GPT  │ Claude│DeepS │Gemini│ Mist │  ← 模型卡片 |
| │ 4o   │ 4     │ eek   │ 2.5  │ ral  |   网格    |
| │tags  │ tags  │ tags  │ tags  │ tags |           |
| │price │ price │ price │ price │ price|           |
| └──────┴──────┴──────┴──────┴──────┘              |
| [1] [2] [3] ...  pagination (if needed)           |
|                                                    |
+--------------------------------------------------+
| MODEL DETAIL (Dialog / Sheet)                      |
|                                                    |
| [模型名 / 提供商 / 状态标签]                        |
| ┌────────────┬────────────────────────────────┐    |
| │ 基本信息     │ 能力图标列表                     │    |
| │ 模型代码     │ Chat / Completion / Embedding  │    |
| │ 上下文窗口   │ Streaming / Vision / Tools     │    |
| │ 开发商      │                                │    |
| ├────────────┴────────────────────────────────┤    |
| │ 定价表                                       │    |
| │ Capability │ Cache Hit │ Cache Miss │ Output │    |
| │ chat       │ ¥x/1K     │ ¥x/1K      │ ¥x/1K  │    |
| │ embedding  │ -         │ -          │ -      │    |
| ├─────────────────────────────────────────────┤    |
| │ [Try in Console ▸]                           │    |
| └─────────────────────────────────────────────┘    |
+--------------------------------------------------+
```

**Layout Notes**:
- 搜索栏全宽，筛选行在下方（水平 inline）
- 模型卡片网格：responsive 1/2/3/4 列（mobile→desktop）
- 每张卡片：模型名 + 提供商 logo + 能力标签 + 最低价格
- 点击卡片打开右侧抽屉(Drawer)或 Dialog 展示详情
- 定价表在详情中展示

---

### 4. 供应商页面 (Providers) — `/providers`

```
+--------------------------------------------------+
| Navbar (shared)                                    |
+--------------------------------------------------+
|                                                    |
| 供应商生态                                           |
|                                                    |
| ┌──────────────────────────────────────────────┐   |
| │ "成为供应商，把你的模型带给全球开发者"          │   |
| │ [申请入驻 ▸] (primary CTA)                    │   |
| └──────────────────────────────────────────────┘   |
|                                                    |
| ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐             |
| │ 已接入供应商列表（卡片网格）                     │
| │Logo + 名 │ 模型数  │  API  │  评分  │           |
| │ 字        │         │  可用性│         │          |
| └──────┘ └──────┘ └──────┘ └──────┘              |
|                                                    |
| 入驻流程说明（三步卡片）                            |
| ┌───┐    ┌───┐    ┌───┐                          |
| │ 1 │ →  │ 2 │ →  │ 3 │                          |
| │提交│    │审核│    │上线│                          |
| │申请│    │配置│    │运营│                          |
| └───┘    └───┘    └───┘                          |
+--------------------------------------------------+
```

**Layout Notes**:
- Hero banner 带入驻 CTA
- 供应商卡片网格（同模型工厂风格）
- 入驻流程水平步骤条
- 底部 FAQ 折叠面板

---

### 5. 供应商入驻表单 (Provider Application) — `/providers/apply`

```
+--------------------------------------------------+
| Navbar (shared)                                    |
+--------------------------------------------------+
|                                                    |
| 申请成为供应商                                      |
|                                                    |
| ┌──────────────────────────────────────────────┐   |
| │ Step 1 of 3: 基本信息                        │   |
| │                                                │   |
| │ 公司名称    [________________] *               │   |
| │ 联系人      [________________] *               │   |
| │ 邮箱        [________________] *               │   |
| │ 网站        [________________]                 │   |
| │ 简介        [________________]                 │   |
| │              textarea                          │   |
| │                                                │   |
| │              [Next Step ▸]                     │   |
| └──────────────────────────────────────────────┘   |
+--------------------------------------------------+
```

- Multi-step form, 3 steps: 基本信息 → 技术信息 → 提交确认
- Progress indicator 在顶部
- 每步有 back/next 按钮
- 提交后跳转成功页

---

### 6. 控制台 (Console) — `/console`

控制台是已登录用户的主要工作区，包含侧边栏 + 内容区。

```
+--------------------------------------------------+
| Navbar (simplified, no auth buttons)              |
| [Logo]  [Home]  [Console]  [username▼]           |
+---------------------------------+----------------+
| Sidebar (w-64, sticky)          | Content Area   |
|                                 |                |
| ┌─────────────────────────┐     |  [路由出口]     |
| │ 余额卡片                 │     |                |
| │ ¥ 1,234.56  (credits)   │     |                |
| │ [Recharge ▸]            │     |                |
| └─────────────────────────┘     |                |
|                                 |                |
| ● 概览 Overview                |                |
| ● API Keys                     |                |
| ● Usage 用量                   |                |
| ● Orders 订单                   |                |
| ● Transactions 交易记录          |                |
| ● Settings 设置                 |                |
|                                 |                |
| ┌─────────────────────────┐     |                |
| │ Provider Mode Switch    │     |                |
| │ [Provider] [Consumer]   │     |                |
| └─────────────────────────┘     |                |
+---------------------------------+----------------+
```

**Sidebar**:
- 顶部余额卡片：紫色底 + 白色文字 + 充值按钮
- 导航项：icon + 标签，当前项高亮
- Provider/Consumer 切换（角色切换）
- Mobile: 变为底部 Tab Bar

---

### 7. 控制台 - 概览 (Console Overview) — `/console`

```
+--------------------------------------------------+
| 概览 (控制台首页)                                  |
|                                                    |
| ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐             |
| │ 信用分 │ │本月用量│ │ API调用 │ │活跃模型│          |
| │ 1,234  │ │ 56.7K │ │ 1,234  │ │ 5     │          |
| └──────┘ └──────┘ └──────┘ └──────┘             |
|                                                    |
| ┌──────────────────────────────────────────────┐   |
| │ 近期用量趋势 (Chart - last 7 days)            │   |
| │  ▁▃▄▆▇▅▇                                    │   |
| └──────────────────────────────────────────────┘   |
|                                                    |
| ┌──────────┐ ┌──────────┐                         |
| │ 快捷操作   │ │ 最近订单   │                        |
| │ [创建Key] │ │ #PAYxxx ¥10│                        |
| │ [充值]    │ │ #PAYxxx ¥20│                        |
| │ [查看模型] │ │ ...        │                        |
| └──────────┘ └──────────┘                         |
+--------------------------------------------------+
```

**Layout Notes**:
- 4 统计卡片行（Stat cards）
- 用量趋势 line chart（recharts）
- 两列下：快捷操作 + 最近订单

---

### 8. 控制台 - 复用页面

以下页面已有实现，直接整合到 Console layout：

| 路由 | 页面 | 现有文件 |
|------|------|----------|
| `/console/keys` | API Key 管理 | `routes/keys.tsx` |
| `/console/usage` | 用量统计 | `routes/usage.tsx` |
| `/console/orders` | 充值订单 | `routes/orders.tsx` |
| `/console/transactions` | 交易记录 | `routes/transactions.tsx` |

需要调整：移除这些页面自带的独立 header，改用 Console sidebar layout。

---

## 路由规划 (Final)

| 路由 | 页面 | 访问控制 |
|------|------|----------|
| `/` | Landing Page | 公开 |
| `/models` | 模型工厂 | 公开 |
| `/models/:id` | 模型详情 (Dialog) | 公开 |
| `/providers` | 供应商列表 | 公开 |
| `/providers/apply` | 供应商入驻 | 公开 |
| `/login` | 登录 | 公开 |
| `/register` | 注册 | 公开 |
| `/console` | Console Layout (sidebar) | JWT |
| `/console/overview` | 概览 | JWT |
| `/console/keys` | API Keys | JWT |
| `/console/usage` | 用量统计 | JWT |
| `/console/orders` | 订单记录 | JWT |
| `/console/transactions` | 交易记录 | JWT |
| `/console/settings` | 设置 | JWT |

---

## 关键组件清单

| 组件 | 用途 | 状态 |
|------|------|------|
| `Navbar` | 全局导航栏 | 新建 |
| `Footer` | 全局底部 | 新建 |
| `Sidebar` | Console 侧边导航 | 新建 |
| `BentoGrid` | Landing 能力展示 | 新建 |
| `ModelCard` | 模型卡片 | 新建 |
| `ModelDetail` | 模型详情弹窗 | 新建 |
| `StatCard` | 概览页统计卡 | 新建 |
| `StepForm` | 入驻多步表单 | 新建 |
| `ProviderCard` | 供应商卡片 | 新建 |
| `BalanceCard` | 余额卡片 (Sidebar) | 新建 |
| `SearchBar` | 模型搜索 | 新建 |

---

## 色值迁移指南

当前项目使用 shadcn CSS 变量 (oklch)。建议调整为紫色主题：

```css
:root {
  --primary: 262.1 83.3% 57.8%;      /* #8B5CF6 */
  --primary-foreground: 0 0% 100%;
  --accent: 45.4 93.4% 47.5%;        /* #FBBF24 */
  --accent-foreground: 0 0% 0%;
  --background: 240 29.6% 9.8%;       /* #0F0F23 (dark) */
  --foreground: 210 40% 98%;          /* #F8FAFC */
  --card: 240 18.2% 17.3%;            /* #1E1E3A */
  --muted: 240 13.5% 19.2%;           /* #27273B */
  --border: 262 40% 35%;              /* #4C1D95 */
}
```

---

## 下一步

1. ✅ 设计系统 & 布局已确定
2. 开始实施：任务拆分（UI 组件 → 页面路由 → 数据接入）
