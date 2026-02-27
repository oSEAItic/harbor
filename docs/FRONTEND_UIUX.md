# Harbor Console — UI/UX Design Spec

> 本文档定义 Harbor Console 的视觉语言、交互模式、动效规范和完整页面设计。
> 与 `FRONTEND_DESIGN.md`（技术架构）互补，本文聚焦于**用户看到和触摸到的一切**。

---

## 1. 设计理念

### 1.1 设计关键词

**Terminal Elegance** — 将终端的信息密度和精确性，与现代 UI 的可发现性和愉悦感结合。

Harbor 的用户是开发者，他们习惯终端，但需要终端做不到的东西：
- 看到数据的**形状**（JSON 树形结构）
- 看到数据的**流向**（raw → normalized → compact → summary）
- 看到系统的**脉搏**（指标趋势图）
- 做**对比**（4 层 memory 并排、新旧 schema diff）

### 1.2 设计原则

| 原则 | 具体表现 |
|------|---------|
| **信息密度优先** | 每屏尽可能多展示信息，不做无意义留白。宁可紧凑也不空洞 |
| **数据即装饰** | 不使用纯装饰性插图，用真实数据本身作为视觉元素（JSON 树、压缩率条、sparkline） |
| **状态一目了然** | 任何数据都带状态标识：fresh/stale、connected/disconnected、drift/stable |
| **操作可逆** | 写入操作（install、schema edit、rollback）都有确认步骤和撤销能力 |
| **键盘友好** | 支持快捷键导航，核心操作不需要鼠标 |

---

## 2. Design System

### 2.1 色彩体系

#### 深色主题（默认）

```
                    ┌─────────────────────────────────┐
  Background        │  #09090B   (Zinc 950)           │  全局底色
  Surface           │  #18181B   (Zinc 900)           │  卡片、面板
  Surface Elevated  │  #27272A   (Zinc 800)           │  悬浮、下拉、弹窗
  Border            │  #3F3F46   (Zinc 700)           │  分割线、输入框边框
  Border Subtle     │  #27272A   (Zinc 800)           │  微弱分割
                    └─────────────────────────────────┘

                    ┌─────────────────────────────────┐
  Text Primary      │  #FAFAFA   (Zinc 50)            │  标题、关键数据
  Text Secondary    │  #A1A1AA   (Zinc 400)           │  描述、标签
  Text Muted        │  #71717A   (Zinc 500)           │  占位符、禁用态
  Text Inverse      │  #09090B   (Zinc 950)           │  亮色按钮上的文字
                    └─────────────────────────────────┘

                    ┌─────────────────────────────────┐
  Accent (Harbor)   │  #3B82F6   (Blue 500)           │  主操作、链接、选中态
  Accent Hover      │  #2563EB   (Blue 600)           │  悬停
  Accent Muted      │  #1E3A5F   (Blue 500/15%)       │  选中行背景
                    └─────────────────────────────────┘

  语义色：
  ┌──────────────────────────────────────────────────┐
  │  Fresh / Success   │  #22C55E  (Green 500)       │
  │  Fresh bg          │  #22C55E/10%                │
  │  Stale / Warning   │  #EAB308  (Yellow 500)      │
  │  Stale bg          │  #EAB308/10%                │
  │  Error / Danger    │  #EF4444  (Red 500)         │
  │  Error bg          │  #EF4444/10%                │
  │  Schema / Info     │  #8B5CF6  (Violet 500)      │
  │  Schema bg         │  #8B5CF6/10%                │
  └──────────────────────────────────────────────────┘
```

#### 浅色主题

```
  Background        #FFFFFF
  Surface           #F4F4F5   (Zinc 100)
  Surface Elevated  #FFFFFF
  Border            #E4E4E7   (Zinc 200)
  Text Primary      #09090B   (Zinc 950)
  Text Secondary    #52525B   (Zinc 600)
  Text Muted        #A1A1AA   (Zinc 400)
  Accent            #2563EB   (Blue 600)
```

#### JSON 语法色

在两种主题下保持一致的 JSON 值类型颜色：

```
  String            #22C55E   (Green 500)     "hello"
  Number            #3B82F6   (Blue 500)      42, 3.14
  Boolean           #A855F7   (Purple 500)    true, false
  Null              #71717A   (Zinc 500)      null
  Key               #E4E4E7   (Zinc 200)      深色下
                    #52525B   (Zinc 600)      浅色下
  Bracket           #71717A   (Zinc 500)      { } [ ]
  Highlight         #FBBF24/20% (Amber)       搜索匹配高亮
```

### 2.2 字体

```
  UI Text           Inter, -apple-system, sans-serif
  Code / Data       JetBrains Mono, Menlo, Consolas, monospace
  Dashboard 数字     JetBrains Mono (tabular-nums, 让数字对齐)
```

| 用途 | 字号 | 字重 | 行高 |
|------|------|------|------|
| Page title | 24px / 1.5rem | 600 (Semibold) | 32px |
| Section title | 18px / 1.125rem | 600 | 28px |
| Card title | 14px / 0.875rem | 500 (Medium) | 20px |
| Body | 14px / 0.875rem | 400 (Regular) | 20px |
| Small / Label | 12px / 0.75rem | 500 | 16px |
| Code / JSON | 13px / 0.8125rem | 400 | 20px |
| Stat number | 32px / 2rem | 700 (Bold) | 40px |
| Stat label | 12px / 0.75rem | 500 | 16px |

### 2.3 间距系统

基于 4px 网格：

```
  xs    4px     组件内微间距
  sm    8px     同组元素间距
  md    12px    卡片内边距、相关元素间
  lg    16px    段落间距、卡片间
  xl    24px    区块间距
  2xl   32px    页面级间距
  3xl   48px    大区块分隔
```

### 2.4 圆角

```
  Small      4px     Badge、Tag、小按钮
  Default    6px     输入框、卡片、普通按钮
  Medium     8px     弹窗、大卡片
  Large     12px     面板、模态框
  Full       9999px  圆形头像、Pill badge
```

### 2.5 阴影

深色主题下阴影效果弱化，改用边框区分层级：

```
  深色主题：
    Elevated    border: 1px solid Zinc-700 + 背景色提亮 (Zinc-800)
    Popup       border: 1px solid Zinc-700 + background Zinc-800
    Overlay     background rgba(0,0,0,0.6)  (遮罩层)

  浅色主题：
    Elevated    box-shadow: 0 1px 3px rgba(0,0,0,0.1)
    Popup       box-shadow: 0 4px 16px rgba(0,0,0,0.12)
    Overlay     background rgba(0,0,0,0.4)
```

---

## 3. 布局系统

### 3.1 全局布局

```
┌─────────────────────────────────────────────────────────┐
│ Topbar (h: 48px)                                        │
├────────────┬────────────────────────────────────────────┤
│            │                                            │
│  Sidebar   │  Main Content                              │
│  (w: 220px)│  (flex: 1, padding: 24px)                  │
│            │                                            │
│            │  ┌─ Page Header ───────────────────────┐   │
│            │  │ Title + Breadcrumb + Actions         │   │
│            │  └─────────────────────────────────────┘   │
│            │                                            │
│            │  ┌─ Content Area ──────────────────────┐   │
│            │  │                                      │   │
│            │  │  (page-specific layout)               │   │
│            │  │                                      │   │
│            │  └─────────────────────────────────────┘   │
│            │                                            │
│  可折叠     │  max-width: 1440px, margin: 0 auto        │
│  → 64px    │                                            │
└────────────┴────────────────────────────────────────────┘
```

### 3.2 Topbar

```
┌─────────────────────────────────────────────────────────┐
│  [≡]  ⚓ Harbor Console          🟢 Gateway Connected  │
│        ↑ 折叠sidebar              ↑ 实时状态              │
│                              [⌘K Search] [◐ Theme] [⚙] │
└─────────────────────────────────────────────────────────┘
```

- 高度 48px，背景 Surface，底边框 Border
- 左侧：Sidebar toggle + Logo（⚓ 锚 + "Harbor Console" 文字，折叠时只显示锚）
- 中间：Gateway 连接状态指示灯（绿/红 dot + 文字）
- 右侧：
  - `⌘K` 全局搜索触发器（搜索 connectors、memory、tools）
  - 主题切换（☀/◐ 图标按钮）
  - 设置齿轮

### 3.3 Sidebar

```
┌──────────────┐
│  ⚓ Harbor    │  ← Logo 区域
├──────────────┤
│              │
│  ◉ Dashboard │  ← 当前页高亮 (Accent Muted bg + Accent 左边框)
│  ◎ Connectors│
│  ◎ Explorer  │
│  ◎ Memory    │
│  ◎ Schemas   │
│  ◎ Tools     │
│  ◎ Metrics   │
│              │
├──────────────┤
│  ◎ Settings  │  ← 底部固定
│  v0.3.0      │  ← 版本号，Muted 色
└──────────────┘

折叠态 (64px):
┌─────┐
│  ⚓  │
├─────┤
│  ◉  │  ← 只显示图标, tooltip 显示名称
│  ◎  │
│  ◎  │
│  ◎  │
│  ◎  │
│  ◎  │
│  ◎  │
├─────┤
│  ◎  │
│     │
└─────┘
```

**导航项规格**：
- 高度 36px，左内边距 12px
- 图标 18px + 文字 14px，间距 10px
- 当前页：左边框 2px Accent，背景 Accent Muted，文字 Text Primary
- 非当前：无边框，背景透明，文字 Text Secondary
- Hover：背景 Zinc-800（深色）/ Zinc-100（浅色）
- 图标使用 Lucide Icons（与 shadcn/ui 一致）：
  - Dashboard: `LayoutDashboard`
  - Connectors: `Plug`
  - Explorer: `FlaskConical`
  - Memory: `Database`
  - Schemas: `Braces`
  - Tools: `Wrench`
  - Metrics: `BarChart3`
  - Settings: `Settings`

### 3.4 Page Header

每个页面顶部统一结构：

```
┌──────────────────────────────────────────────────────┐
│  Dashboard / Connectors / coingecko    ← 面包屑       │
│                                                       │
│  CoinGecko                        [Action Button]     │
│  Cryptocurrency prices and data    ← 可选描述          │
└──────────────────────────────────────────────────────┘
```

- 面包屑：12px, Text Muted，用 `/` 分隔，可点击
- 标题：24px Semibold
- 描述：14px Text Secondary（可选）
- 右侧：页面级操作按钮

---

## 4. 组件设计

### 4.1 Stat Card

Dashboard 使用的统计卡片。

```
┌─────────────────────────┐
│  Installed Connectors   │  ← label: 12px, Text Muted, uppercase tracking-wide
│                         │
│  4                      │  ← value: 32px, Bold, monospace
│  ━━━━━━━━━━             │  ← sparkline (可选): 32px height, Accent 色
│  +1 this week           │  ← trend: 12px, Green/Red
└─────────────────────────┘

尺寸: 固定高度 120px, 宽度 flex (grid 等分)
背景: Surface
边框: 1px Border
圆角: 8px
内边距: 16px
```

**变体**：
- 默认：数字 + 可选 sparkline + 可选 trend
- 百分比：数字带 % + 环形进度条（替代 sparkline 位置）
- 状态：数字 + 语义色（绿=好, 黄=注意, 红=异常）

```
  例：
  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐
  │ Memory Hit %  │  │ Tokens Saved  │  │ Schema Drift  │
  │               │  │               │  │               │
  │   68.2%       │  │   12,340      │  │   0           │
  │   [===◯  ]    │  │   ▁▂▃▅▇█▆▄▃  │  │               │
  │    ↑ 环形     │  │   +2,100 24h  │  │   All stable  │
  │               │  │               │  │   (Green)     │
  └───────────────┘  └───────────────┘  └───────────────┘
```

### 4.2 Badge 系统

```
  ┌─────────────────────────────────────────────────┐
  │  状态 Badge (Freshness)                          │
  │                                                  │
  │  [🟢 fresh]    bg: Green/10%, text: Green 500    │
  │  [🟡 near-stale] bg: Yellow/10%, text: Yellow 500│
  │  [🔴 stale]    bg: Red/10%, text: Red 500        │
  │                                                  │
  │  类型 Badge                                       │
  │                                                  │
  │  [node]        bg: Zinc-800, text: Zinc-400      │
  │  [python]      bg: Zinc-800, text: Zinc-400      │
  │  [local]       bg: Violet/10%, text: Violet 500  │
  │  [v0.1.0]     bg: Zinc-800, text: Zinc-400      │
  │                                                  │
  │  Schema Badge                                     │
  │                                                  │
  │  [crypto.prices.v1] bg: Blue/10%, text: Blue 500 │
  │                                                  │
  └─────────────────────────────────────────────────┘

  规格: 高度 22px, 圆角 4px, padding 0 8px, font-size 12px, font-weight 500
```

### 4.3 JSON Viewer

Harbor 最核心的组件，需要极致的浏览体验。

```
┌─ JSON Viewer ──────────────────────────────────────┐
│  [🔍 Search fields...] [Expand All] [Collapse All] [Copy] │
│────────────────────────────────────────────────────│
│                                                    │
│  ▼ [                              ← Bracket: Zinc 500
│      ▼ {                          ← 缩进: 每层 20px
│          "id":        "bitcoin",  ← Key: Zinc 200, String: Green
│          "price_usd": 67234.12,   ← Number: Blue
│          "market_cap": 1.32e+12,
│          "active":    true,       ← Boolean: Purple
│          "delisted":  null,       ← Null: Zinc 500, italic
│          ▶ "tags":    [...] (3)   ← 折叠态: 显示 item count
│        },
│      ▶ { ... }                    ← 折叠的对象: 显示 "..."
│    ]
│                                                    │
│  Hover 效果:                                        │
│  ┌──────────────────────────────────────┐          │
│  │  "price_usd": 67234.12      [📋]   │ ← 行 hover │
│  └──────────────────────────────────────┘ ← 显示复制按钮
│                                                    │
└────────────────────────────────────────────────────┘
```

**交互细节**：

| 交互 | 行为 |
|------|------|
| 点击 ▼/▶ | 展开/折叠该节点，200ms ease-out 动画 |
| 点击 key 名 | 复制该 key 的 JSON path（如 `data[0].price_usd`） |
| 行 hover | 行背景变为 Zinc-800，右侧出现复制按钮 |
| 点击复制按钮 | 复制该节点的值，显示 "Copied!" toast |
| 搜索输入 | 实时过滤，匹配的 key 和 value 高亮（Amber/20% 背景） |
| 大数组 (>50 items) | 虚拟滚动，底部显示 "Showing 50 of 1,234 items" |
| ⌘F | 聚焦搜索框 |
| ↑↓ 方向键 | 在展开的节点间导航 |

**高亮字段**（用于 summary_fields）：
- 被标记为 summary_field 的 key 名前显示蓝色竖线标记
- 可选模式：只显示 summary_fields（dimming 其他字段）

### 4.4 Response Envelope

专门展示 Harbor 协议 Response 的复合组件。

```
┌─ Response ─────────────────────────────────────────┐
│                                                     │
│  ┌────────────────────────────────────────────────┐ │
│  │  ✅ Success  340ms  Cache: miss  mem_a1b2c3d4  │ │  ← Status bar
│  │  Schema: crypto.prices.v1  Source: coingecko    │ │
│  └────────────────────────────────────────────────┘ │
│                                                     │
│  ┌──────────────────────────────────────────────┐   │
│  │ [Data ●] [Meta] [Summary] [Raw] [Errors (0)] │   │  ← Tab bar
│  └──────────────────────────────────────────────┘   │
│                                                     │
│  ┌─ Data ──────────────────────────────────────┐    │
│  │                                              │    │
│  │  Overview: 2 items                           │    │  ← Overview bar
│  │  Fields: id, price_usd, market_cap_usd       │    │
│  │                                              │    │
│  │  (JSON Viewer showing data[])                │    │
│  │                                              │    │
│  └──────────────────────────────────────────────┘    │
│                                                     │
└─────────────────────────────────────────────────────┘
```

**Status Bar 设计**：
- 成功：绿色 ✅ + "Success"
- 有错误：红色 ❌ + 第一个 error message
- 来自 memory：蓝色 💾 + "From memory (3m ago)"
- 背景色根据状态变化（Green/10%, Red/10%, Blue/10%）

**Tab Bar**：
- 活动 tab：底部 2px Accent 边框 + Text Primary
- 非活动：无底边 + Text Muted
- Errors tab：如果有错误，显示红色数字 badge `(2)`
- Tab 切换无页面跳转，内容区 fade 过渡 150ms

### 4.5 Layer Comparison（Memory 4 层对比）

这是 Harbor 前端的**标志性组件**。

```
┌─ Layer Comparison ─────────────────────────────────┐
│                                                     │
│  View: [◉ Single] [◎ Side-by-Side]                  │
│                                                     │
│  ┌────────────────────────────────────────────────┐ │
│  │ [Raw        ] [Normalized] [Compact ●] [Summary]│ │
│  │  4,200 B       1,890 B      320 B       42 B   │ │  ← 字节大小
│  └────────────────────────────────────────────────┘ │
│                                                     │
│  ┌─ Compact (320 bytes) ──────────────────────────┐ │
│  │                                                 │ │
│  │  [ { "id": "bitcoin",                          │ │
│  │      "price_usd": 67234.12,                    │ │
│  │      "market_cap_usd": 1320000000000 } ]       │ │
│  │                                                 │ │
│  └─────────────────────────────────────────────────┘ │
│                                                     │
│  Compression: ━━━━━━━━━━━━━━━━━━━━━░░░░░             │
│               Raw 4.2K ──────────→ Compact 320B     │
│               92.4% reduced | ~970 tokens saved     │
│                                                     │
└─────────────────────────────────────────────────────┘

Side-by-Side 模式:
┌─────────────────────────────────────────────────────┐
│                                                      │
│  ┌─ Raw (4,200B) ──────┐  ┌─ Compact (320B) ──────┐ │
│  │                      │  │                        │ │
│  │ {"bitcoin":{         │  │ [{"id":"bitcoin",     │ │
│  │   "usd":67234.12,    │  │   "price_usd":        │ │
│  │   "usd_market_cap"   │  │     67234.12,          │ │
│  │     :132000...,      │  │   "market_cap_usd":    │ │
│  │   "usd_24h_vol"      │  │     1.32e+12}]         │ │
│  │     :28000...,       │  │                        │ │
│  │   "usd_24h_change"   │  │                        │ │
│  │     :2.34,           │  │                        │ │
│  │   "last_updated_at"  │  │                        │ │
│  │     :1708...,        │  │                        │ │
│  │   ... (20+ fields)   │  │                        │ │
│  │ }}                   │  │                        │ │
│  └──────────────────────┘  └────────────────────────┘ │
│                                                      │
└──────────────────────────────────────────────────────┘
```

**压缩率条设计**：
- 全宽 bar，高度 8px，圆角 4px
- 填充部分 = compact/raw 比例，颜色 = Accent
- 空白部分 = 节省的比例，颜色 = Zinc-800
- 下方文字显示百分比和 token 节省估算

**Tab 上的字节大小**：
- 直接在 tab 名称下方显示灰色小字
- 最小的层会有绿色高亮（表示最高效）
- Summary 层用字符数而非字节数

### 4.6 Param Editor（参数编辑器）

根据 tool schema 自动生成的表单。

```
┌─ Parameters ───────────────────────────────────────┐
│                                                     │
│  ids *                                              │
│  ┌──────────────────────────────────────────────┐   │
│  │  bitcoin                                      │   │
│  └──────────────────────────────────────────────┘   │
│  Comma-separated coin IDs (e.g. bitcoin,ethereum)   │  ← description
│                                                     │
│  vs_currencies *                                    │
│  ┌──────────────────────────────────────────────┐   │
│  │  usd                                          │   │
│  └──────────────────────────────────────────────┘   │
│  Target currencies                                  │
│                                                     │
│  ┌──────────────────────────────────────────────┐   │
│  │ + Add optional parameter                      │   │
│  └──────────────────────────────────────────────┘   │
│                                                     │
│  [Switch to JSON mode]                              │
│                                                     │
└─────────────────────────────────────────────────────┘

JSON 模式:
┌─ Parameters (JSON) ────────────────────────────────┐
│                                                     │
│  ┌──────────────────────────────────────────────┐   │
│  │  {                                            │   │
│  │    "ids": "bitcoin",                          │   │
│  │    "vs_currencies": "usd"                     │   │
│  │  }                                            │   │
│  └──────────────────────────────────────────────┘   │
│                                                     │
│  [Switch to form mode]                              │
│                                                     │
└─────────────────────────────────────────────────────┘
```

**表单字段规格**：
- Label: 14px Medium, Text Primary
- Required 标记: * 号，Red 500
- Input: 高度 36px，背景 Zinc-900，边框 Zinc-700，圆角 6px
- Focus: 边框变为 Accent，outer glow (0 0 0 2px Blue/25%)
- Description: 12px Text Muted，在输入框下方

### 4.7 Data Table

Memory 列表、Tool 列表等使用的表格。

```
┌─────────────────────────────────────────────────────┐
│  ID          Source              Age      Status Size│
│─────────────────────────────────────────────────────│
│  mem_a1b2    coingecko.prices    3m ago   🟢     320B│  ← hover: Zinc-800 bg
│  mem_c3d4    coingecko.trending  12m ago  🔴    1.2K │
│  mem_e5f6    yahoo.quote         1h ago   🔴     890B│
│  mem_g7h8    proxy:list_files    5m ago   🟢    4.5K │
│─────────────────────────────────────────────────────│
│  Showing 1-20 of 127               [← 1 2 3 ... →] │
└─────────────────────────────────────────────────────┘
```

**表格规格**：
- Header: 12px uppercase, Text Muted, 背景 Surface, 底边框
- Row: 高度 44px, 14px monospace (数据列), 底边框 Border Subtle
- Hover: 行背景变 Zinc-800
- Click: 整行可点击，进入详情
- 排序: Header 可点击排序，显示 ↑↓ 箭头
- 选中行: 左边框 2px Accent + Accent Muted 背景
- Monospace 列: ID, Size, Age 使用 monospace 字体确保对齐
- Status 列: 状态点 (8px 圆，语义色)

### 4.8 Connector Card

```
┌─ Connector Card ──────────────────────────────────┐
│                                                    │
│  ┌────┐  CoinGecko                  [🟢 Installed]│
│  │ 🪙 │  Cryptocurrency prices, trending coins    │
│  └────┘  and coin details from CoinGecko          │
│                                                    │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐           │
│  │ [v0.1.0] │ │ [node]   │ │ [3 res]  │           │
│  └──────────┘ └──────────┘ └──────────┘           │
│                                                    │
│  Schemas: crypto.prices.v1, crypto.coin.v1, ...    │
│                                                    │
│  12 calls today | Last: 3m ago                     │
│                                                    │
│  [View Details]                [Try in Explorer →]  │
│                                                    │
└────────────────────────────────────────────────────┘

尺寸: 宽度 100% (在 grid 中), 内边距 20px
背景: Surface
边框: 1px Border, hover 时 Border → Zinc-600
圆角: 8px
过渡: border-color 150ms ease
```

---

## 5. 页面详细 UX 设计

### 5.1 Dashboard

**布局**：CSS Grid

```
┌──────────────────────────────────────────────────────┐
│  Dashboard                                            │
│──────────────────────────────────────────────────────│
│                                                      │
│  ┌─ Stats Grid (3 columns) ────────────────────────┐ │
│  │ ┌──────────┐ ┌──────────┐ ┌──────────┐          │ │
│  │ │Connectors│ │ Memories │ │ Hit Rate │          │ │
│  │ │    4     │ │   127    │ │  68.2%   │          │ │
│  │ └──────────┘ └──────────┘ └──────────┘          │ │
│  │ ┌──────────┐ ┌──────────┐ ┌──────────┐          │ │
│  │ │Tok Saved │ │Avg Ratio │ │  Drift   │          │ │
│  │ │  12,340  │ │   0.23   │ │    0     │          │ │
│  │ └──────────┘ └──────────┘ └──────────┘          │ │
│  └──────────────────────────────────────────────────┘ │
│                                                      │
│  ┌─ 2-column layout ──────────────────────────────┐  │
│  │                                                 │  │
│  │  ┌─ Recent Activity ─────┐ ┌─ Quick Actions ──┐│  │
│  │  │                       │ │                   ││  │
│  │  │  (Activity timeline)  │ │ [Explorer]        ││  │
│  │  │                       │ │ [Install]         ││  │
│  │  │  ● coingecko.prices   │ │ [Export Tools]    ││  │
│  │  │    3m ago, fresh      │ │ [View Metrics]    ││  │
│  │  │                       │ │                   ││  │
│  │  │  ● yahoo.quote        │ │ System            ││  │
│  │  │    12m ago, stale     │ │ Gateway: 🟢       ││  │
│  │  │                       │ │ Node.js: v20      ││  │
│  │  │  ● proxy:list_files   │ │ Harbor: v0.3.0    ││  │
│  │  │    5m ago, fresh      │ │                   ││  │
│  │  │                       │ │                   ││  │
│  │  └───────────────────────┘ └───────────────────┘│  │
│  │                                                 │  │
│  └─────────────────────────────────────────────────┘  │
│                                                      │
│  ┌─ Compression Timeline (full width) ─────────────┐ │
│  │  [24h] [7d] [30d]                               │ │
│  │                                                   │ │
│  │  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓░░░░░░░░░░░░░░░    │ │
│  │  ▓ = compact    ░ = saved                         │ │
│  │                                                   │ │
│  └───────────────────────────────────────────────────┘ │
│                                                      │
└──────────────────────────────────────────────────────┘
```

**Recent Activity 列表项**：
```
  ┌─────────────────────────────────────────────┐
  │  ● coingecko.prices              3 min ago  │
  │    ids=bitcoin, vs_currencies=usd           │
  │    [🟢 fresh] [320B] [crypto.prices.v1]     │
  └─────────────────────────────────────────────┘

  ● = 时间线节点: 8px 圆, 连接线 1px Zinc-700
  行高: ~64px
  点击: 跳转到 /memory/{id}
```

**Quick Actions**：
- 大按钮卡片风格，图标 + 文字
- Hover 时背景从 Surface → Zinc-800，左边框出现 Accent 色

### 5.2 Data Explorer

Data Explorer 是使用频率最高的页面，UX 至关重要。

**布局**：上下分割（请求区 / 响应区），可拖拽调整比例。

```
┌──────────────────────────────────────────────────────┐
│  Data Explorer                                        │
│──────────────────────────────────────────────────────│
│                                                      │
│  ┌─ Request Panel ─────────────────────────────────┐ │
│  │                                                  │ │
│  │  ┌─────────────────────┐  ┌──────────────────┐  │ │
│  │  │ Connector            │  │ Resource          │  │ │
│  │  │ [coingecko      ▼]  │  │ [prices       ▼]  │  │ │
│  │  └─────────────────────┘  └──────────────────┘  │ │
│  │                                                  │ │
│  │  Parameters:                                     │ │
│  │  ┌───────────────────────────────────────────┐  │ │
│  │  │ ids *              │ bitcoin               │  │ │
│  │  │ vs_currencies *    │ usd                   │  │ │
│  │  └───────────────────────────────────────────┘  │ │
│  │                                                  │ │
│  │  ┌─ Options (collapsible) ──────────────────┐   │ │
│  │  │ ☐ Full output  ☐ Include raw  ☐ Refresh  │   │ │
│  │  │ Budget: [____]  Layer: [auto ▼]           │   │ │
│  │  └──────────────────────────────────────────┘   │ │
│  │                                                  │ │
│  │     [Execute ▶]                  ⌘↩ to execute  │ │
│  │                                                  │ │
│  └──────────────────────────────────────────────────┘ │
│  ═══════════════════ drag handle ════════════════════ │
│  ┌─ Response Panel ────────────────────────────────┐ │
│  │                                                  │ │
│  │  (见 4.4 Response Envelope)                      │ │
│  │                                                  │ │
│  └──────────────────────────────────────────────────┘ │
│                                                      │
└──────────────────────────────────────────────────────┘
```

**关键 UX 细节**：

1. **Connector 下拉**: 输入可搜索，显示 connector 图标 + 名称 + resource 数量
2. **Resource 下拉**: 选择 connector 后自动加载，显示 resource 名称 + description
3. **参数自动生成**: 选择 resource 后，从 tool schema 的 parameters 自动生成表单字段
4. **Execute 按钮**:
   - 默认：蓝色主按钮 "Execute ▶"
   - Loading：旋转 spinner + "Executing..."，按钮禁用
   - 快捷键 `⌘ Enter`（右下角灰色提示）
5. **Response 出现动画**: 从下方 slide-up 16px + fade-in，200ms ease-out
6. **错误态**: 如果 response.errors 非空，Status bar 变红，Errors tab 自动激活并闪烁一次
7. **缓存标识**: 如果来自 memory，Status bar 显示蓝色 "From memory" + 过期倒计时
8. **历史记录**: 右上角小按钮 "History"，展示最近 10 次请求，可一键重放

### 5.3 Memory Browser

**搜索体验**：

```
┌─ Search ──────────────────────────────────────────┐
│                                                    │
│  🔍 [Search memories...____________________]       │
│                                                    │
│  ┌─ Filters ─────────────────────────────────────┐ │
│  │ Connector: [All ▼]  Time: [All ▼]  Status: [All ▼] │
│  └───────────────────────────────────────────────┘ │
│                                                    │
│  127 results (83 fresh, 44 stale)                  │
│                                                    │
└────────────────────────────────────────────────────┘
```

- 搜索框：大输入框，自动聚焦，支持 `⌘K` 全局触发
- 搜索防抖：300ms，输入停止后才搜索
- Filter badges：选中的 filter 显示为可移除的 badge
- 结果统计：显示总数 + fresh/stale 分布

**Memory 详情页 UX**：

进入详情后，关键要展示数据从 raw 到 summary 的**压缩旅程**：

```
  Raw (4.2 KB) ──→ Normalized (1.9 KB) ──→ Compact (320 B) ──→ Summary (42 B)
       │                   │                     │                    │
    上游原始            规范化数组           summary fields       自然语言
       │                   │                     │                    │
      100%               45.2%                 7.6%                 1.0%
```

这个流程可视化用**水平步骤条**展示：
- 4 个节点，每节点显示层名 + 大小 + 百分比
- 节点间用渐细的连线连接（表示信息压缩）
- 当前查看的层节点高亮
- 点击任意节点切换到该层的 JSON 视图

### 5.4 Metrics Dashboard

**图表交互规范**：

```
  ┌─ Compression Timeline ────────────────────────┐
  │                                                │
  │  [24h ●] [7d] [30d]      Auto-refresh: [⏸/▶] │
  │                                                │
  │   KB                                           │
  │  8 ┤          ╱╲                               │
  │  6 ┤    ╱╲  ╱    ╲                    ╱        │
  │  4 ┤  ╱    ╲╱      ╲╱╲    ╱╲  ╱╲  ╱╱          │  ← raw (Zinc 400, 虚线)
  │  2 ┤╱                  ╲╱    ╲╱  ╲╱            │
  │  1 ┤━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━       │  ← compact (Accent, 实线)
  │  0 ┼──────────────────────────────────→ Time   │
  │    00:00   06:00   12:00   18:00   24:00       │
  │                                                │
  │   ▓ Tokens saved (shaded area between lines)   │
  │                                                │
  └────────────────────────────────────────────────┘
```

**Hover tooltip**：
```
  ┌──────────────────┐
  │  14:30            │
  │  Raw: 6.2 KB      │
  │  Compact: 980 B    │
  │  Saved: ~1,305 tok │
  │  Ratio: 0.158      │
  └──────────────────┘
```

- 竖直辅助线跟随鼠标
- Tooltip 固定在鼠标上方，避免被裁切
- 阴影区域（saved tokens）使用 Accent/10% 填充

---

## 6. 交互模式

### 6.1 全局搜索（Command Palette）

`⌘K` 打开，搜索一切。

```
┌─────────────────────────────────────────────────┐
│  🔍 Search Harbor...                              │
│─────────────────────────────────────────────────│
│                                                  │
│  Pages                                           │
│  ┌────────────────────────────────────────────┐  │
│  │ 📊  Dashboard                               │  │
│  │ 🔌  Connectors                              │  │
│  │ 🧪  Data Explorer                           │  │
│  └────────────────────────────────────────────┘  │
│                                                  │
│  Connectors                                      │
│  ┌────────────────────────────────────────────┐  │
│  │ 🪙  coingecko — Cryptocurrency prices      │  │
│  │ 📈  yahoo — Stock quotes                    │  │
│  └────────────────────────────────────────────┘  │
│                                                  │
│  Recent Memories                                 │
│  ┌────────────────────────────────────────────┐  │
│  │ 💾  mem_a1b2 — coingecko.prices — 3m ago   │  │
│  └────────────────────────────────────────────┘  │
│                                                  │
│  ↑↓ Navigate  ↩ Select  esc Close               │
└─────────────────────────────────────────────────┘
```

- Overlay + 居中弹窗，宽度 560px
- 实时搜索，分类显示结果
- ↑↓ 键盘导航，Enter 确认，Esc 关闭
- 搜索范围：页面名、connector 名/description、memory summary、tool 名

### 6.2 快捷键

| 快捷键 | 动作 |
|--------|------|
| `⌘K` | 打开全局搜索 |
| `⌘E` | 跳转到 Data Explorer |
| `⌘↩` | 在 Explorer 中执行请求 |
| `⌘B` | 折叠/展开 Sidebar |
| `⌘.` | 切换主题 |
| `Esc` | 关闭弹窗/取消操作 |
| `⌘C` | 复制当前选中的 JSON 节点 |
| `1-7` | 在 Sidebar 中快速切换页面（Dashboard=1, ...） |

### 6.3 确认对话框

写入操作（install、uninstall、rollback、delete）使用统一确认模式：

```
┌──────────────────────────────────────┐
│                                      │
│  Uninstall coingecko?                │
│                                      │
│  This will remove the connector      │
│  binary and all associated data.     │
│  Memory entries will be preserved.   │
│                                      │
│          [Cancel]  [Uninstall]       │
│                     ↑ 红色按钮        │
└──────────────────────────────────────┘
```

- 居中模态框，遮罩层
- 危险操作按钮用 Red 500
- Cancel 用 ghost 样式
- 动画: fade-in 150ms + scale from 0.95

### 6.4 Toast 通知

右下角弹出，自动消失。

```
  ┌────────────────────────────────┐
  │ ✅  Connector installed         │   ← 成功: Green border-left
  │    coingecko v0.1.0            │
  └────────────────────────────────┘

  ┌────────────────────────────────┐
  │ ❌  Execution failed            │   ← 错误: Red border-left
  │    connector_not_found          │
  └────────────────────────────────┘

  ┌────────────────────────────────┐
  │ 📋  Copied to clipboard        │   ← 信息: Blue border-left
  └────────────────────────────────┘
```

- 位置: 右下角, 距边 16px
- 多个 toast 向上堆叠，间距 8px
- 自动消失: 成功 3s, 错误 5s, 信息 2s
- 动画: slide-in from right 200ms + fade-out on dismiss
- 最多同时显示 3 个

---

## 7. 动效规范

### 7.1 过渡时间

| 场景 | 时长 | 缓动 |
|------|------|------|
| Hover 状态变化 | 150ms | ease |
| Tab 切换内容 | 150ms | ease-out |
| 侧栏折叠/展开 | 200ms | ease-in-out |
| 弹窗出现 | 150ms | ease-out |
| 弹窗消失 | 100ms | ease-in |
| 页面路由切换 | 200ms | ease-out |
| JSON 节点展开 | 200ms | ease-out |
| Toast 进场 | 200ms | ease-out |
| Toast 退场 | 150ms | ease-in |
| Skeleton 闪烁 | 1.5s | linear (循环) |

### 7.2 Loading 状态

**Skeleton 加载**：

```
  ┌─────────────────────────────┐
  │  ░░░░░░░░░░░                │   ← 标题 skeleton
  │                             │
  │  ░░░░░░░░░░░░░░░░░░░░░░░   │   ← 内容 skeleton
  │  ░░░░░░░░░░░░░░░░░         │
  │  ░░░░░░░░░░░░░░░░░░░░░░░░░ │
  └─────────────────────────────┘
```

- 用于首次加载数据
- Skeleton 使用 Zinc-800 → Zinc-700 的渐变动画（1.5s 循环）
- 形状匹配实际内容布局

**Execute 按钮 Loading**：
```
  [Executing...  ⟳]   ← 蓝色按钮, spinner 旋转
```

**页面切换**：
- 旧内容 fade-out 100ms
- 新内容 fade-in + slide-up 8px，200ms

### 7.3 空状态

每个列表页面都需要空状态设计：

```
  ┌──────────────────────────────────────┐
  │                                      │
  │           ⚓                          │
  │                                      │
  │     No connectors installed          │
  │                                      │
  │  Install your first connector to     │
  │  start fetching data for agents.     │
  │                                      │
  │     [Browse Catalog →]               │
  │                                      │
  └──────────────────────────────────────┘
```

- 居中，图标 48px（使用页面对应的 Lucide icon，灰色）
- 标题 16px Semibold
- 描述 14px Text Muted
- CTA 按钮引导下一步

每个页面的空状态文案：

| 页面 | 图标 | 标题 | CTA |
|------|------|------|-----|
| Connectors | Plug | No connectors installed | Browse Catalog |
| Memory | Database | No memories yet | Open Explorer |
| Schemas | Braces | No schemas learned | Learn more about proxy |
| Metrics | BarChart3 | No metrics recorded | Set up proxy |

### 7.4 错误状态

```
  ┌──────────────────────────────────────┐
  │                                      │
  │           ⚠                          │
  │                                      │
  │     Gateway unreachable              │
  │                                      │
  │  Cannot connect to Harbor Gateway    │
  │  at http://localhost:8080            │
  │                                      │
  │     [Retry]   [Settings]             │
  │                                      │
  └──────────────────────────────────────┘
```

---

## 8. 响应式设计

### 8.1 断点

| 断点 | 宽度 | 布局变化 |
|------|------|---------|
| Desktop XL | >= 1440px | 全功能布局，Dashboard 3 列 stats |
| Desktop | >= 1024px | 默认布局 |
| Tablet | >= 768px | Sidebar 折叠为图标模式，Dashboard 2 列 stats |
| Mobile | < 768px | Sidebar 变为 hamburger 菜单，单列布局 |

### 8.2 Sidebar 响应式行为

```
  >= 1024px:  展开态 (220px)，可手动折叠
  768-1023px: 自动折叠为图标态 (64px)
  < 768px:    隐藏，通过 hamburger 按钮触发 overlay drawer
```

### 8.3 Explorer 响应式

```
  >= 1024px:  请求/响应上下分割
  < 1024px:   请求/响应切换为 tab 模式
              [Request] [Response]
```

### 8.4 Layer Comparison 响应式

```
  >= 1024px:  Side-by-side 模式可用
  < 1024px:   只有 Single 模式（tab 切换层）
```

---

## 9. 无障碍（Accessibility）

### 9.1 基本要求

| 项目 | 标准 |
|------|------|
| 对比度 | 所有文字满足 WCAG AA (4.5:1 正文, 3:1 大字) |
| 键盘导航 | 所有交互元素可 Tab 聚焦，Enter/Space 触发 |
| Focus visible | 聚焦元素显示 2px Accent outline (offset 2px) |
| ARIA | 表格有 aria-label, 弹窗有 aria-modal, tabs 有 role=tablist |
| 屏幕阅读器 | 状态 badge 有 aria-label ("Status: fresh, expires in 2 minutes") |
| 动效 | 尊重 `prefers-reduced-motion`，减少所有非必要动画 |

### 9.2 Focus 管理

- 打开弹窗 → focus 锁定在弹窗内
- 关闭弹窗 → focus 回到触发按钮
- Tab 切换 → focus 自动移到新 tab panel
- 全局搜索打开 → focus 自动到搜索框

---

## 10. 品牌与情感化设计

### 10.1 Logo 使用

- 主 logo: ⚓ 锚图标 + "Harbor" 文字
- Sidebar 展开态: 锚 + "Harbor Console"
- Sidebar 折叠态: 只显示锚
- 浏览器 tab: ⚓ Harbor Console
- Favicon: 蓝色圆底 + 白色锚

### 10.2 微交互彩蛋

- **首次安装 connector**: confetti 动画（非常小，1 秒消失）
- **压缩率 < 10%**: 数字旁显示闪烁的星星 "✨ 92% saved"
- **Memory 全部 fresh**: 状态栏显示 "All data fresh" 带绿色呼吸效果
- **Schema drift 归零**: drift 计数器从数字回到 0 时播放短暂脉冲动画

### 10.3 文案语气

| 场景 | 风格 | 示例 |
|------|------|------|
| 标题/标签 | 简洁、技术 | "Compression Ratio", "Schema Drift" |
| 空状态 | 引导、友好 | "No memories yet. Data from connectors will appear here." |
| 错误 | 直接、可执行 | "Gateway unreachable. Check if harbor-gateway is running." |
| 成功 | 确认、克制 | "Connector installed." 不过度庆祝 |
| Tooltip | 解释、精确 | "Time since data was fetched. Stale data may be outdated." |

---

## 11. 关键页面视觉稿描述

以下用文字精确描述每个页面的视觉呈现，可直接作为开发参考。

### 11.1 Dashboard — 首屏呈现

打开 Harbor Console，第一眼看到的：

- 深色背景 (#09090B) 给人终端般的专业感
- 顶部 6 个 stat cards 排成 2x3 网格（每个 cards 之间 16px gap）
- 第一行三个卡片的数字用 32px bold monospace，一眼扫到关键指标
- 下方左侧是 Recent Activity 时间线（占 60% 宽度），每条记录前有绿/红圆点
- 右侧是 Quick Actions 卡片（占 40%）+ 系统状态小卡片
- 最底部是全宽的 Compression Timeline 图表
- 图表使用柔和的蓝色调，阴影区域表示节省的空间，给人"一直在优化"的感觉
- 整体信息密度高但不杂乱，因为用了足够的灰度层级来区分主次

### 11.2 Data Explorer — 核心体验

这是用户停留最久的页面：

- 上半部分是请求区，紧凑但不拥挤
- Connector 和 Resource 两个下拉框并排，选完后下方的参数表单平滑展开（200ms slide-down）
- Execute 按钮是页面唯一的蓝色主按钮，视觉焦点
- 点击 Execute 后，按钮变为 loading 状态，下方分隔线出现蓝色进度条（indeterminate）
- 响应返回后，进度条消失，Response 面板从下方 slide-up 出现
- Status bar 根据结果变色：成功=绿底, 缓存命中=蓝底, 错误=红底
- JSON Viewer 中 key 名用浅灰色，值用对应类型色
- 鼠标 hover 到 JSON 的任意行时，右侧浮现一个小小的复制按钮
- 如果来自 memory，Status bar 上有一个蓝色 "From memory" badge + TTL 倒计时

### 11.3 Memory Detail — 压缩旅程

进入一条 memory 的详情：

- 顶部是元信息卡片：connector.resource, schema, age, status badge, params
- 核心区域是**层级流水线**可视化：
  - 水平排列 4 个节点：Raw → Normalized → Compact → Summary
  - 节点之间的连线从粗到细，暗示信息的压缩过程
  - 每个节点下方显示字节大小，字号从左到右递减（也暗示压缩）
  - 被选中的节点放大、高亮，下方展开 JSON 视图
- 底部是压缩率条形图：一个全宽的 bar，填充部分 = compact/raw，视觉上很直观
- 在 Side-by-Side 模式下，左右两个 JSON Viewer 同步滚动（可选）

---

## 12. 开发规范

### 12.1 组件命名

```
  文件名: kebab-case           json-viewer.tsx, stat-card.tsx
  组件名: PascalCase           JSONViewer, StatCard
  CSS class: Tailwind utility   (不使用 CSS modules)
  Hook: camelCase + use 前缀   useConnectors, useMetrics
```

### 12.2 主题实现

使用 CSS 变量 + Tailwind 自定义色，在 `<html>` 上切换 class：

```css
:root {
  --background: 240 10% 3.9%;     /* Zinc 950 */
  --surface: 240 5.9% 10%;        /* Zinc 900 */
  --border: 240 3.7% 15.9%;       /* Zinc 800 */
  --text-primary: 0 0% 98%;       /* Zinc 50 */
  --accent: 217.2 91.2% 59.8%;    /* Blue 500 */
}

.light {
  --background: 0 0% 100%;
  --surface: 240 4.8% 95.9%;
  --border: 240 5.9% 90%;
  --text-primary: 240 10% 3.9%;
  --accent: 217.2 91.2% 59.8%;
}
```

### 12.3 性能约束

| 指标 | 目标 |
|------|------|
| First Contentful Paint | < 1s |
| Time to Interactive | < 2s |
| JSON Viewer (1000 items) | < 100ms render |
| 大表格 (500 rows) | 虚拟滚动，< 16ms frame |
| Bundle size | < 300KB gzipped |
| Lighthouse Performance | >= 90 |

---

## 附录: 图标映射

| 概念 | Lucide 图标 | 用途 |
|------|-------------|------|
| Dashboard | `LayoutDashboard` | Sidebar nav |
| Connector | `Plug` | Sidebar nav, cards |
| Explorer | `FlaskConical` | Sidebar nav |
| Memory | `Database` | Sidebar nav, badges |
| Schema | `Braces` | Sidebar nav, cards |
| Tool | `Wrench` | Sidebar nav, cards |
| Metrics | `BarChart3` | Sidebar nav |
| Settings | `Settings` | Sidebar nav, topbar |
| Fresh | `CircleCheck` | Status badge |
| Stale | `CircleAlert` | Status badge |
| Error | `CircleX` | Error display |
| Copy | `Copy` | Action button |
| Execute | `Play` | Explorer button |
| Search | `Search` | Search bar |
| Expand | `ChevronDown` | JSON tree |
| Collapse | `ChevronRight` | JSON tree |
| External link | `ExternalLink` | Links |
| Install | `Download` | Connector install |
| Uninstall | `Trash2` | Connector remove |
| Refresh | `RefreshCw` | Force re-fetch |
| Theme | `SunMoon` | Theme toggle |
| History | `History` | Version history |
| Rollback | `Undo2` | Schema rollback |
