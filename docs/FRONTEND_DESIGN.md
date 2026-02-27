# Harbor Frontend Design Document

> 设计目标：为 Harbor 提供一个可视化的 Web 控制台，让用户可以管理 connectors、探索数据、监控系统状态、调试工具调用，而无需记忆 CLI 命令。

---

## 1. 产品定位与核心原则

### 1.1 定位

Harbor Frontend 是 Harbor 的 **运维与开发控制台**，不是面向终端用户的产品界面。主要用户是：

- **AI 应用开发者** — 构建 agent 时需要了解有哪些数据源、调试 connector 输出
- **平台运维** — 监控 proxy 压缩率、memory 使用、schema drift 等指标
- **Connector 开发者** — 测试 connector、查看 tool schema、验证输出格式

### 1.2 设计原则

| 原则 | 说明 |
|------|------|
| **CLI-first, UI-second** | 前端不替代 CLI，而是提供 CLI 不擅长的可视化能力（图表、JSON 树、diff） |
| **只读优先** | 大部分操作是查看和探索，写入操作（install、learn schema）需要确认 |
| **实时感知** | 数据应有新鲜度标识（fresh/stale），指标应可自动刷新 |
| **JSON 原生** | Harbor 的核心是 JSON envelope，前端必须把 JSON 浏览做到极致 |
| **渐进式复杂度** | 首页是 Dashboard 概览，深入点击才展开细节 |

---

## 2. 技术选型

### 2.1 推荐方案

| 层面 | 选择 | 理由 |
|------|------|------|
| **框架** | Next.js 14+ (App Router) | RSC 支持、API routes 与 Gateway 对接、SSR 首屏性能 |
| **语言** | TypeScript | 与 Harbor SDK/connector 生态统一 |
| **UI 组件库** | shadcn/ui + Radix | 无厂商锁定、可定制、质量高 |
| **样式** | Tailwind CSS | 与 shadcn/ui 天然配合 |
| **状态管理** | TanStack Query (React Query) | 专为 server state 设计，自动缓存/刷新/polling |
| **图表** | Recharts | 轻量、React 原生、满足 metrics 展示 |
| **JSON 浏览** | react-json-view-lite 或自研 Tree | 支持折叠、语法高亮、层级筛选 |
| **表格** | TanStack Table | 排序、筛选、虚拟滚动 |
| **代码编辑** | Monaco Editor (可选) | Schema 编辑、JSON params 输入 |
| **包管理** | pnpm | 快速、磁盘效率高 |

### 2.2 项目结构

```
harbor/
├── frontend/                    # 前端独立目录
│   ├── package.json
│   ├── next.config.ts
│   ├── tailwind.config.ts
│   ├── tsconfig.json
│   ├── public/
│   │   └── harbor-logo.svg
│   ├── src/
│   │   ├── app/                 # Next.js App Router
│   │   │   ├── layout.tsx       # 根布局（侧栏 + 顶栏）
│   │   │   ├── page.tsx         # Dashboard 首页
│   │   │   ├── connectors/
│   │   │   │   ├── page.tsx     # Connector 列表
│   │   │   │   └── [id]/
│   │   │   │       └── page.tsx # Connector 详情
│   │   │   ├── explorer/
│   │   │   │   └── page.tsx     # 数据探索 Playground
│   │   │   ├── memory/
│   │   │   │   ├── page.tsx     # Memory 浏览与搜索
│   │   │   │   └── [id]/
│   │   │   │       └── page.tsx # Memory 对象详情（4层对比）
│   │   │   ├── schemas/
│   │   │   │   └── page.tsx     # Learned schemas 管理
│   │   │   ├── tools/
│   │   │   │   └── page.tsx     # Tool 目录 & Schema 导出
│   │   │   ├── metrics/
│   │   │   │   └── page.tsx     # Proxy 指标仪表盘
│   │   │   └── settings/
│   │   │       └── page.tsx     # 系统配置
│   │   ├── components/
│   │   │   ├── ui/              # shadcn/ui 组件
│   │   │   ├── layout/
│   │   │   │   ├── sidebar.tsx
│   │   │   │   ├── topbar.tsx
│   │   │   │   └── breadcrumb.tsx
│   │   │   ├── json-viewer.tsx  # JSON 树形浏览器
│   │   │   ├── response-envelope.tsx  # Harbor Response 专用展示
│   │   │   ├── freshness-badge.tsx    # fresh/stale 标识
│   │   │   ├── connector-card.tsx
│   │   │   ├── memory-timeline.tsx
│   │   │   ├── metrics-chart.tsx
│   │   │   ├── tool-schema-card.tsx
│   │   │   ├── layer-switcher.tsx     # Memory 4层切换
│   │   │   └── param-editor.tsx       # connector params 输入表单
│   │   ├── lib/
│   │   │   ├── api.ts           # Gateway HTTP client
│   │   │   ├── types.ts         # 前端类型定义（镜像 Go types）
│   │   │   └── utils.ts
│   │   └── hooks/
│   │       ├── use-connectors.ts
│   │       ├── use-memory.ts
│   │       ├── use-metrics.ts
│   │       └── use-tools.ts
│   └── tests/
```

---

## 3. Gateway API 扩展

当前 Gateway 已有 `/health`、`/run`、`/tools`、`/recall` 端点。前端需要额外的 API：

### 3.1 需要新增的端点

| 方法 | 路径 | 说明 | 对应 CLI |
|------|------|------|----------|
| `GET` | `/connectors` | 列出已安装和目录中的 connectors | `harbor list` |
| `GET` | `/connectors/:id` | 单个 connector 详情 + tool schemas | `harbor list` + `--describe` |
| `POST` | `/connectors/:id/install` | 安装 connector | `harbor install` |
| `DELETE` | `/connectors/:id` | 卸载 connector | `harbor uninstall` |
| `GET` | `/memory` | 列出 memory index entries（分页/搜索） | `harbor recall --list` |
| `GET` | `/memory/:id` | 获取完整 memory object（含 4 层） | `harbor recall <id>` |
| `GET` | `/memory/search?q=...` | 关键词搜索 memory | `harbor recall --search` |
| `GET` | `/schemas` | 列出所有 learned schemas | `harbor schema list` |
| `GET` | `/schemas/:toolName` | 获取 schema 详情 + 版本历史 | `harbor schema show` |
| `PUT` | `/schemas/:toolName` | 手动更新 schema | `harbor_learn_schema` |
| `POST` | `/schemas/:toolName/rollback` | 回滚 schema | 内部逻辑 |
| `GET` | `/metrics` | 获取聚合 metrics | `harbor metrics report --json` |
| `GET` | `/metrics/raw` | 获取原始 JSONL 事件流 | 直读文件 |
| `GET` | `/doctor` | 系统健康检查 | `harbor doctor --json` |

### 3.2 API 响应格式

所有 Gateway API 应遵循统一格式：

```typescript
interface APIResponse<T> {
  data: T;
  meta: {
    timestamp: string;
    request_id: string;
  };
  errors: Array<{ code: string; message: string }>;
}
```

---

## 4. 页面详细设计

### 4.1 Dashboard（首页）

**目标**：一目了然地了解 Harbor 实例的整体状态。

```
┌──────────────────────────────────────────────────────────┐
│  Harbor Console                              [Settings]  │
├──────────┬───────────────────────────────────────────────┤
│          │                                               │
│ Dashboard│  ┌─────────┐ ┌─────────┐ ┌─────────┐         │
│ Connectors│ │ 4       │ │ 127     │ │ 68%     │         │
│ Explorer │ │Connectors│ │Memories │ │Hit Rate │         │
│ Memory   │ └─────────┘ └─────────┘ └─────────┘         │
│ Schemas  │                                               │
│ Tools    │  ┌─────────┐ ┌─────────┐ ┌─────────┐         │
│ Metrics  │  │ 12,340  │ │ 0.23    │ │ 0       │         │
│ Settings │  │Tok Saved│ │Avg Ratio│ │ Drift   │         │
│          │  └─────────┘ └─────────┘ └─────────┘         │
│          │                                               │
│          │  Recent Activity                              │
│          │  ┌───────────────────────────────────────┐    │
│          │  │ 12:03  coingecko.prices  fresh  320B  │    │
│          │  │ 11:58  yahoo.quote       stale  1.2K  │    │
│          │  │ 11:45  proxy:list_files  fresh  890B  │    │
│          │  └───────────────────────────────────────┘    │
│          │                                               │
│          │  Compression Over Time (24h)                  │
│          │  ┌───────────────────────────────────────┐    │
│          │  │  📊 折线图: raw bytes vs compact bytes │    │
│          │  └───────────────────────────────────────┘    │
│          │                                               │
└──────────┴───────────────────────────────────────────────┘
```

**卡片组件 (StatCard)**：
- Installed Connectors — 数量 + 点击跳转到 /connectors
- Memory Objects — 数量 + fresh/stale 比例环形图
- Memory Hit Rate — 百分比 + 24h 趋势 sparkline
- Tokens Saved — 累计数 + 今日增量
- Avg Compression Ratio — 越低越好（绿色 < 0.3）
- Schema Drift — 最近 24h 内的 drift 次数（0 = 绿，>0 = 黄/红）

**Recent Activity**：
- 取 memory index 最近 20 条
- 每行显示：时间、connector.resource、fresh/stale badge、compact size
- 点击进入 memory 详情

**Compression Chart**：
- X 轴：时间（24h / 7d / 30d 切换）
- Y 轴：bytes
- 两条线：raw bytes（灰色）、compact bytes（蓝色）
- 阴影区域 = tokens saved

### 4.2 Connectors 页面

#### 4.2.1 列表页 `/connectors`

```
┌─────────────────────────────────────────────────┐
│  Connectors                     [Install New]   │
├─────────────────────────────────────────────────┤
│                                                  │
│  Installed (3)                                   │
│  ┌────────────────────────────────────────────┐  │
│  │ 🟢 coingecko    Cryptocurrency prices...   │  │
│  │    v0.1.0 | node | 3 resources | 12 calls  │  │
│  ├────────────────────────────────────────────┤  │
│  │ 🟢 yahoo        Stock quotes, company...   │  │
│  │    v0.1.0 | node | 4 resources | 8 calls   │  │
│  ├────────────────────────────────────────────┤  │
│  │ 🟢 kuse-hive    Internal trace data...     │  │
│  │    local  | node | 2 resources | 45 calls   │  │
│  └────────────────────────────────────────────┘  │
│                                                  │
│  Available in Catalog (1)                        │
│  ┌────────────────────────────────────────────┐  │
│  │ ○ postgresql    PostgreSQL queries...       │  │
│  │    v0.1.0 | node          [Install]         │  │
│  └────────────────────────────────────────────┘  │
│                                                  │
└─────────────────────────────────────────────────┘
```

每个 connector card 显示：
- 状态指示灯（已安装 = 绿，目录可安装 = 灰）
- ID + description
- 版本号 / runtime / resource 数量 / 最近调用次数
- 快捷操作：View Details / Uninstall / Try in Explorer

#### 4.2.2 详情页 `/connectors/[id]`

```
┌─────────────────────────────────────────────────┐
│  ← Connectors / coingecko                       │
├─────────────────────────────────────────────────┤
│                                                  │
│  CoinGecko                              v0.1.0   │
│  Cryptocurrency prices, trending, coin details   │
│  Runtime: node | Schemas: crypto.prices.v1, ...  │
│                                                  │
│  ┌─ Resources ─────────────────────────────────┐ │
│  │                                              │ │
│  │  coingecko.prices                            │ │
│  │  Get current prices for cryptocurrencies     │ │
│  │  Params:                                     │ │
│  │    ids (required): string — Coin IDs         │ │
│  │    vs_currencies (required): string           │ │
│  │  Summary fields: [id, price_usd, market_cap] │ │
│  │                          [Try in Explorer]    │ │
│  │                                              │ │
│  │  coingecko.trending                          │ │
│  │  Get trending coins on CoinGecko             │ │
│  │  Params: (none)                              │ │
│  │                          [Try in Explorer]    │ │
│  │                                              │ │
│  │  coingecko.coin                              │ │
│  │  Get detailed info for a specific coin       │ │
│  │  Params:                                     │ │
│  │    id (required): string — Coin ID           │ │
│  │                          [Try in Explorer]    │ │
│  └──────────────────────────────────────────────┘ │
│                                                  │
│  ┌─ Tool Schema (OpenAI format) ───────────────┐ │
│  │  { JSON viewer with syntax highlighting }    │ │
│  │                     [Copy] [Export as MCP]    │ │
│  └──────────────────────────────────────────────┘ │
│                                                  │
│  ┌─ Recent Memories ───────────────────────────┐ │
│  │  mem_a1b2 | prices | 3m ago | fresh         │ │
│  │  mem_c3d4 | trending | 12m ago | stale      │ │
│  └──────────────────────────────────────────────┘ │
│                                                  │
└─────────────────────────────────────────────────┘
```

### 4.3 Data Explorer（Playground）

**核心功能**：交互式地调用 connector resource，查看规范化后的 Harbor Response。

```
┌─────────────────────────────────────────────────┐
│  Data Explorer                                   │
├─────────────────────────────────────────────────┤
│                                                  │
│  ┌─ Request ───────────────────────────────────┐ │
│  │  Connector: [coingecko    ▼]                │ │
│  │  Resource:  [prices       ▼]                │ │
│  │                                              │ │
│  │  Parameters:                                 │ │
│  │  ┌───────────┬──────────────────────────┐    │ │
│  │  │ ids       │ bitcoin                  │    │ │
│  │  ├───────────┼──────────────────────────┤    │ │
│  │  │vs_curren..│ usd                      │    │ │
│  │  └───────────┴──────────────────────────┘    │ │
│  │                                              │ │
│  │  Options:                                    │ │
│  │  ☐ Full (no compilation)                     │ │
│  │  ☐ Include raw upstream                      │ │
│  │  ☐ Force refresh (bypass memory)             │ │
│  │  Budget: [____] tokens                       │ │
│  │                                              │ │
│  │         [Execute]                             │ │
│  └──────────────────────────────────────────────┘ │
│                                                  │
│  ┌─ Response ──────────────────────────────────┐ │
│  │  Status: ✅ 200  |  Time: 340ms  |  Cache: miss │
│  │  Memory ID: mem_e5f6  |  Schema: crypto.prices.v1 │
│  │                                              │ │
│  │  [Data] [Meta] [Errors] [Raw] [Summary]      │ │
│  │  ┌──────────────────────────────────────┐    │ │
│  │  │  ▼ [0]                               │    │ │
│  │  │    "id": "bitcoin"                   │    │ │
│  │  │    "price_usd": 67234.12             │    │ │
│  │  │    "market_cap_usd": 1320000000000   │    │ │
│  │  │  ▼ [1]                               │    │ │
│  │  │    "id": "ethereum"                  │    │ │
│  │  │    ...                               │    │ │
│  │  └──────────────────────────────────────┘    │ │
│  │                                              │ │
│  │  Summary:                                    │ │
│  │  "bitcoin: $67,234.12 (mcap $1.32T);         │ │
│  │   ethereum: $3,456.78 (mcap $415B)"          │ │
│  │                                              │ │
│  │  Overview: 2 items, fields: [id, price_usd,  │ │
│  │            market_cap_usd, ...]               │ │
│  └──────────────────────────────────────────────┘ │
│                                                  │
└─────────────────────────────────────────────────┘
```

**关键交互**：
- Connector/Resource 下拉从 `/tools` API 动态加载
- 选择 resource 后，自动生成 params 表单（从 tool schema 的 parameters.properties 生成）
- required params 标红星
- Execute 按钮调用 `POST /run`
- Response 区分 5 个 tab：Data（JSON 树）、Meta（元数据卡片）、Errors（错误列表）、Raw（原始上游响应）、Summary（自然语言摘要）
- Data tab 支持：展开/折叠、搜索 field、复制单个值、复制整个 data
- 支持 **对比模式**：左侧 normalized data，右侧 raw data

### 4.4 Memory 页面

#### 4.4.1 列表/搜索 `/memory`

```
┌─────────────────────────────────────────────────┐
│  Memory                                          │
├─────────────────────────────────────────────────┤
│                                                  │
│  Search: [bitcoin_________________] [🔍]         │
│  Filter: [All Connectors ▼] [All Time ▼]        │
│                                                  │
│  127 memories (83 fresh, 44 stale)               │
│                                                  │
│  ┌─ ID ──────── Source ───── Age ─── Size ─────┐ │
│  │ mem_a1b2    coingecko.prices    3m   🟢 320B │ │
│  │ mem_c3d4    coingecko.trending  12m  🔴 1.2K │ │
│  │ mem_e5f6    yahoo.quote         1h   🔴 890B │ │
│  │ mem_g7h8    proxy:list_files    5m   🟢 4.5K │ │
│  │ ...                                          │ │
│  └──────────────────────────────────────────────┘ │
│                                    [1] [2] [3]   │
│                                                  │
│  Storage: 2.3 MB total | Index: 127 entries      │
│                                                  │
└─────────────────────────────────────────────────┘
```

- 🟢 = fresh（在 TTL 内），🔴 = stale
- 搜索框调用 `/memory/search?q=...`
- 可按 connector 筛选、按时间范围筛选
- Size 列显示 compact 层大小
- 底部显示总存储用量

#### 4.4.2 详情页 `/memory/[id]`

**核心特色：4 层对比视图**

```
┌─────────────────────────────────────────────────┐
│  ← Memory / mem_a1b2c3d4                         │
├─────────────────────────────────────────────────┤
│                                                  │
│  coingecko.prices                                │
│  Schema: crypto.prices.v1 | Created: 3m ago      │
│  Params: ids=bitcoin, vs_currencies=usd          │
│  TTL: 300s | Status: 🟢 fresh                    │
│  Content Hash: e5f6a7b8                          │
│                                                  │
│  ┌─ Layer Comparison ──────────────────────────┐ │
│  │                                              │ │
│  │  [Raw ◆] [Normalized] [Compact ●] [Summary] │ │
│  │                                              │ │
│  │  ◆ Raw (4,200 bytes)         ● Compact (320 bytes)  │
│  │  ┌──────────────────┐  ┌──────────────────┐  │ │
│  │  │ {"bitcoin":{     │  │ [{"id":"bitcoin",│  │ │
│  │  │   "usd":67234.12,│  │   "price_usd":   │  │ │
│  │  │   "usd_market_cap│  │     67234.12,    │  │ │
│  │  │   ":1320000000000│  │   "market_cap":  │  │ │
│  │  │   ,"usd_24h_vol" │  │     1.32e+12}]   │  │ │
│  │  │   :28000000000,  │  │                  │  │ │
│  │  │   ...            │  │                  │  │ │
│  │  │ }}               │  │                  │  │ │
│  │  └──────────────────┘  └──────────────────┘  │ │
│  │                                              │ │
│  │  Compression: 4,200B → 320B (92.4% saved)   │ │
│  │  ≈ 970 tokens saved                          │ │
│  │                                              │ │
│  │  Summary:                                    │ │
│  │  "bitcoin: $67,234.12 (mcap $1.32T)"        │ │
│  │                                              │ │
│  └──────────────────────────────────────────────┘ │
│                                                  │
└─────────────────────────────────────────────────┘
```

**功能**：
- 4 层 tab 切换，每层显示 JSON 树 + 字节大小
- **并排对比模式**：可选任意两层左右对比
- 压缩率可视化：raw → compact 的 bar chart
- 自动计算 tokens saved（bytes / 4）
- 快捷操作：Copy Layer、Re-fetch (refresh)、Delete

### 4.5 Schemas 页面

管理 proxy 学习到的压缩 schema。

```
┌─────────────────────────────────────────────────┐
│  Learned Schemas                                 │
├─────────────────────────────────────────────────┤
│                                                  │
│  ┌──── Tool ────────── Fields ──── Version ────┐ │
│  │ list_files         name,size,type    v3     │ │
│  │   Template: {name} ({type}, {size} bytes)   │ │
│  │   Learned: 2h ago by agent                  │ │
│  │                   [Edit] [History] [Rollback]│ │
│  ├──────────────────────────────────────────────┤ │
│  │ search_issues     number,title,state  v1    │ │
│  │   Template: #{number} {title} [{state}]     │ │
│  │   Learned: 1d ago by agent                  │ │
│  │                   [Edit] [History] [Rollback]│ │
│  ├──────────────────────────────────────────────┤ │
│  │ get_notifications  title,reason,repo  v2    │ │
│  │   Template: {title} - {reason} in {repo}    │ │
│  │   Learned: 3d ago by llm (gpt-4)           │ │
│  │                   [Edit] [History] [Rollback]│ │
│  └──────────────────────────────────────────────┘ │
│                                                  │
│  Schema Version History (list_files)             │
│  ┌──────────────────────────────────────────────┐ │
│  │ v3 (current) | name,size,type      | 2h ago │ │
│  │ v2           | name,size           | 1d ago │ │
│  │ v1           | name,type,modified  | 3d ago │ │
│  └──────────────────────────────────────────────┘ │
│                                                  │
└─────────────────────────────────────────────────┘
```

**功能**：
- 列出所有 learned schemas，显示 fields + template + version
- 每个 schema 可展开 version history
- Edit 打开表单编辑 summary_fields 和 summary_template
- Rollback 一键回退到指定版本
- 测试按钮：用当前 schema 压缩最近的 raw 数据，预览效果

### 4.6 Tools 页面

展示 Harbor 的 Tool IR 以及导出为不同格式。

```
┌─────────────────────────────────────────────────┐
│  Tools                                           │
├─────────────────────────────────────────────────┤
│                                                  │
│  Export Format: [OpenAI ▼]  [Copy All] [Download]│
│                                                  │
│  4 tools from 2 connectors                       │
│                                                  │
│  ┌──────────────────────────────────────────────┐ │
│  │ coingecko.prices                             │ │
│  │ Get current prices for cryptocurrencies      │ │
│  │ Parameters:                                  │ │
│  │   ids: string (required) — Comma-separated   │ │
│  │   vs_currencies: string (required)            │ │
│  │                                              │ │
│  │ JSON Schema:                                 │ │
│  │ {                                            │ │
│  │   "type": "function",                        │ │
│  │   "function": {                              │ │
│  │     "name": "coingecko.prices",              │ │
│  │     ...                                      │ │
│  │   }                                          │ │
│  │ }                                [Copy]      │ │
│  └──────────────────────────────────────────────┘ │
│  ┌──────────────────────────────────────────────┐ │
│  │ coingecko.trending                           │ │
│  │ ...                                          │ │
│  └──────────────────────────────────────────────┘ │
│                                                  │
└─────────────────────────────────────────────────┘
```

**功能**：
- 格式切换：OpenAI / MCP / Raw IR
- 全量导出：一键复制/下载整个 tool schema 数组
- 单个 tool 的 schema 可独立复制
- 每个 tool 展示人类可读的参数表 + 原始 JSON schema
- 直接跳转到 Explorer 试用该 tool

### 4.7 Metrics 仪表盘

```
┌─────────────────────────────────────────────────┐
│  Metrics                                         │
├─────────────────────────────────────────────────┤
│                                                  │
│  Time Range: [24h] [7d] [30d]  Auto-refresh: [⏸] │
│                                                  │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐│
│  │ 342     │ │ 68.2%   │ │ 78.3%   │ │ 0.23    ││
│  │ Calls   │ │Hit Rate │ │Schema % │ │Avg Ratio││
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘│
│                                                  │
│  ┌─ Compression Timeline ──────────────────────┐ │
│  │                                              │ │
│  │  📊 面积图: raw vs compact over time         │ │
│  │     阴影区域 = bytes saved                    │ │
│  │                                              │ │
│  └──────────────────────────────────────────────┘ │
│                                                  │
│  ┌─ Top Tools by Call Count ───────────────────┐ │
│  │                                              │ │
│  │  Tool           Calls  Hit%  Schema% Ratio   │ │
│  │  list_files     89     72%   85%     0.18    │ │
│  │  search_issues  45     60%   100%    0.21    │ │
│  │  read_file      32     45%   0%      1.00    │ │
│  │  get_notific..  28     80%   100%    0.12    │ │
│  │                                              │ │
│  └──────────────────────────────────────────────┘ │
│                                                  │
│  ┌─ Schema Drift Events ──────────────────────┐  │
│  │                                              │ │
│  │  ⚠ list_files — field hit rate 0.15 — 2h ago│ │
│  │    Auto-rolled back to v2                    │ │
│  │                                              │ │
│  │  (no other drift events in 24h)             │ │
│  └──────────────────────────────────────────────┘ │
│                                                  │
│  Total tokens saved (24h): ~12,340               │
│                                                  │
└─────────────────────────────────────────────────┘
```

**图表类型**：
- **Compression Timeline**: 面积图，X=时间，Y=bytes，两区域（raw, compact）
- **Tool Breakdown**: 水平柱状图，按调用次数排序
- **Hit Rate Trend**: 折线图，memory hit rate over time
- **Schema Drift Log**: 时间线，显示 drift 事件和自动回滚

### 4.8 Settings 页面

```
┌─────────────────────────────────────────────────┐
│  Settings                                        │
├─────────────────────────────────────────────────┤
│                                                  │
│  Harbor Home                                     │
│  ~/.harbor                                       │
│                                                  │
│  Gateway                                         │
│  URL: http://localhost:8080                       │
│  Status: 🟢 Connected                            │
│                                                  │
│  Directories                                     │
│  ┌──────────────────────────────────────────┐    │
│  │ Connectors: ~/.harbor/connectors  (3)   │    │
│  │ Memory:     ~/.harbor/memory      (2.3MB)│    │
│  │ Schemas:    ~/.harbor/schemas     (5)   │    │
│  │ Metrics:    ~/.harbor/metrics     (480K)│    │
│  └──────────────────────────────────────────┘    │
│                                                  │
│  Maintenance                                     │
│  [Prune Stale Memories]  [Clear Metrics]         │
│                                                  │
│  System Info (harbor doctor)                     │
│  ┌──────────────────────────────────────────┐    │
│  │ Harbor Version: 0.3.0                    │    │
│  │ Go Version: 1.22                         │    │
│  │ Node.js: v20.11.0                        │    │
│  │ OS: darwin arm64                         │    │
│  │ HARBOR_HOME: ~/.harbor                   │    │
│  └──────────────────────────────────────────┘    │
│                                                  │
└─────────────────────────────────────────────────┘
```

---

## 5. 核心组件设计

### 5.1 JSON Viewer

Harbor 的核心价值是 JSON 数据，JSON Viewer 是最重要的组件。

```typescript
interface JSONViewerProps {
  data: unknown;
  maxDepth?: number;        // 默认展开深度
  searchable?: boolean;     // 支持字段搜索
  copyable?: boolean;       // 每个节点可复制
  highlightFields?: string[];  // 高亮特定字段（如 summary_fields）
  diffWith?: unknown;       // 对比模式，高亮差异
}
```

功能要求：
- 折叠/展开，支持全部展开/全部折叠
- 字段值类型颜色区分（string=绿, number=蓝, boolean=紫, null=灰）
- 数组显示 item count
- 大数组虚拟滚动（>100 items）
- 搜索字段名，匹配的节点自动展开并高亮
- 复制按钮：单个值、单个对象、整个 JSON
- Diff 模式：并排显示，增/删/改用颜色标注

### 5.2 Response Envelope

专门展示 Harbor Response 的组件，理解 Harbor 协议结构。

```typescript
interface ResponseEnvelopeProps {
  response: HarborResponse;
  showRaw?: boolean;
  activeTab?: 'data' | 'meta' | 'errors' | 'raw' | 'summary';
}
```

- Data tab: JSON Viewer 展示 `data[]`，如果有 overview 则顶部显示 item count + field list
- Meta tab: 卡片式展示 source, connector_version, schema, fetched_at, request_id, memory_id
- Errors tab: 如果 `errors[]` 非空，红色列表显示 code + message + detail
- Raw tab: JSON Viewer 展示 `raw`（如果非 null）
- Summary tab: 自然语言摘要文本

### 5.3 Freshness Badge

```typescript
interface FreshnessBadgeProps {
  createdAt: string;
  ttlSeconds: number;
}
```

- 计算剩余 TTL
- Fresh: 绿色 badge + 剩余时间
- Stale: 红色 badge + 过期时间
- 接近过期（<20% TTL 剩余）: 黄色 badge

### 5.4 Layer Switcher

Memory 4 层切换组件。

```typescript
interface LayerSwitcherProps {
  layers: {
    raw: unknown;
    normalized: unknown;
    compact: unknown;
    summary: string;
  };
  bytesRaw: number;
  bytesCompact: number;
}
```

- 4 个 tab，每个显示该层的字节大小
- 底部显示压缩率 bar（raw → compact 的可视化）
- 支持 side-by-side 对比任意两层

### 5.5 Param Editor

根据 tool schema 自动生成参数输入表单。

```typescript
interface ParamEditorProps {
  schema: JSONSchema;  // tool.function.parameters
  value: Record<string, string>;
  onChange: (params: Record<string, string>) => void;
}
```

- 从 `properties` 自动生成字段
- `required` 字段标红星
- 字段描述显示为 placeholder 或 tooltip
- 支持 JSON 模式（直接编辑 JSON 字符串）

---

## 6. 数据流与状态管理

### 6.1 API Client

```typescript
// lib/api.ts
const GATEWAY_URL = process.env.NEXT_PUBLIC_GATEWAY_URL || 'http://localhost:8080';

export const harborAPI = {
  // Connectors
  listConnectors: () => fetch(`${GATEWAY_URL}/connectors`).then(r => r.json()),
  getConnector: (id: string) => fetch(`${GATEWAY_URL}/connectors/${id}`).then(r => r.json()),
  installConnector: (id: string) => fetch(`${GATEWAY_URL}/connectors/${id}/install`, { method: 'POST' }),

  // Data
  run: (req: RunRequest) => fetch(`${GATEWAY_URL}/run`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  }).then(r => r.json()),

  // Memory
  listMemory: (params?: MemoryQuery) => fetch(`${GATEWAY_URL}/memory?${qs(params)}`).then(r => r.json()),
  getMemory: (id: string) => fetch(`${GATEWAY_URL}/memory/${id}`).then(r => r.json()),
  searchMemory: (q: string) => fetch(`${GATEWAY_URL}/memory/search?q=${q}`).then(r => r.json()),

  // Tools
  listTools: () => fetch(`${GATEWAY_URL}/tools`).then(r => r.json()),

  // Schemas
  listSchemas: () => fetch(`${GATEWAY_URL}/schemas`).then(r => r.json()),

  // Metrics
  getMetrics: (params?: MetricsQuery) => fetch(`${GATEWAY_URL}/metrics?${qs(params)}`).then(r => r.json()),

  // Health
  health: () => fetch(`${GATEWAY_URL}/health`).then(r => r.json()),
};
```

### 6.2 React Query Hooks

```typescript
// hooks/use-connectors.ts
export function useConnectors() {
  return useQuery({
    queryKey: ['connectors'],
    queryFn: harborAPI.listConnectors,
    staleTime: 30_000,  // 30s cache
  });
}

// hooks/use-metrics.ts
export function useMetrics(since: string = '24h') {
  return useQuery({
    queryKey: ['metrics', since],
    queryFn: () => harborAPI.getMetrics({ since }),
    refetchInterval: 60_000,  // auto-refresh every 60s
  });
}

// hooks/use-memory.ts
export function useMemorySearch(query: string) {
  return useQuery({
    queryKey: ['memory', 'search', query],
    queryFn: () => harborAPI.searchMemory(query),
    enabled: query.length > 0,
  });
}
```

---

## 7. 前端类型定义

镜像 Go 端类型，确保前后端一致。

```typescript
// lib/types.ts

// Harbor Protocol Response
interface HarborResponse {
  data: unknown[];
  meta: HarborMeta;
  raw?: unknown;
  errors: HarborError[];
  overview?: ContextOverview;
  summary?: string;
}

interface HarborMeta {
  source: string;
  connector_version: string;
  schema: string;
  tool_schema_version?: string;
  fetched_at: string;
  request_id: string;
  pagination?: PageInfo;
  memory_id?: string;
  from_memory?: boolean;
}

interface HarborError {
  code: string;
  message: string;
  detail?: string;
}

interface ContextOverview {
  total_items: number;
  fields: string[];
}

interface PageInfo {
  cursor?: string;
  has_more: boolean;
  total?: number;
}

// Memory
interface MemoryObject {
  id: string;
  connector: string;
  resource: string;
  schema: string;
  params?: Record<string, string>;
  created_at: string;
  ttl_seconds: number;
  layers: {
    raw?: unknown;
    normalized?: unknown;
    compact?: unknown;
    summary?: string;
  };
  meta: {
    source: string;
    connector_version: string;
    request_id: string;
    content_hash: string;
    bytes_raw: number;
    bytes_compact: number;
  };
}

interface MemoryIndexEntry {
  id: string;
  connector: string;
  resource: string;
  schema: string;
  params?: Record<string, string>;
  created_at: string;
  ttl_seconds: number;
  bytes_raw: number;
  bytes_compact: number;
  summary?: string;
}

interface MemorySearchResult extends MemoryIndexEntry {
  score: number;
  age: string;
  fresh: boolean;
}

// Tool Schema
interface ToolSchema {
  type: 'function';
  function: {
    name: string;
    description: string;
    parameters: Record<string, unknown>;
    summary_fields?: string[];
    summary_template?: string;
  };
}

// Learned Schema
interface LearnedSchema {
  tool_name: string;
  summary_fields: string[];
  summary_template: string;
  learned_at: string;
  llm_model?: string;
  version: number;
}

// Connector
interface CatalogEntry {
  id: string;
  name: string;
  version: string;
  description: string;
  runtime: string;
  schemas: string[];
}

interface ConnectorInfo extends CatalogEntry {
  installed: boolean;
  tools: ToolSchema[];
  memory_count: number;
  recent_calls: number;
}

// Metrics
interface MetricsSummary {
  path: string;
  since: string;
  total_calls: number;
  window_start: string;
  window_end: string;
  memory_hit_rate: number;
  schema_applied_rate: number;
  drift_rate: number;
  avg_compression_ratio: number;
  approx_tokens_saved_total: number;
  tools: ToolSummary[];
}

interface ToolSummary {
  tool_name: string;
  calls: number;
  memory_hits: number;
  schema_applied_calls: number;
  drift_detected_calls: number;
  avg_compression_ratio: number;
  approx_tokens_saved_total: number;
}
```

---

## 8. 实现路线图

### Phase 1: 基础框架 + 只读视图（MVP）

**目标**：能看、能探索、能调试。

| 任务 | 优先级 | 说明 |
|------|--------|------|
| 项目初始化 | P0 | Next.js + shadcn/ui + Tailwind 脚手架 |
| Layout 框架 | P0 | 侧栏导航 + 顶栏 + 面包屑 |
| Gateway API 扩展 | P0 | 在 gateway/main.go 中新增 /connectors, /memory, /schemas, /metrics 端点 |
| Dashboard 首页 | P0 | 统计卡片 + Recent Activity 列表 |
| Connectors 列表 | P0 | 展示已安装和目录 connectors |
| Data Explorer | P0 | 核心 playground，调用 /run 查看结果 |
| JSON Viewer 组件 | P0 | 折叠、搜索、复制、类型高亮 |
| Response Envelope 组件 | P0 | 5-tab 展示 Harbor Response |

### Phase 2: Memory + Schema 管理

| 任务 | 优先级 | 说明 |
|------|--------|------|
| Memory 列表 + 搜索 | P1 | 浏览、搜索、筛选 memory |
| Memory 4 层对比 | P1 | 核心差异化功能 |
| Layer Switcher 组件 | P1 | 切换 + 压缩率可视化 |
| Schemas 列表 | P1 | 查看所有 learned schemas |
| Schema 版本历史 | P1 | 查看 + 回滚 |
| Schema 编辑 | P2 | 手动编辑 summary_fields / template |
| Schema 测试预览 | P2 | 用 schema 压缩 raw 数据预览效果 |

### Phase 3: Metrics + 高级功能

| 任务 | 优先级 | 说明 |
|------|--------|------|
| Metrics 仪表盘 | P1 | 图表 + 表格 + drift log |
| Compression Timeline 图表 | P1 | raw vs compact 面积图 |
| Token Saved 统计 | P1 | 累计 + 趋势 |
| Connector 安装/卸载 | P2 | 从前端触发 install/uninstall |
| Tools 导出页 | P2 | 多格式导出 OpenAI/MCP |
| Settings 页面 | P2 | 系统信息 + 维护操作 |
| 深色模式 | P3 | 开发者工具标配 |
| 实时 WebSocket 推送 | P3 | Gateway → Frontend 实时事件 |

### Phase 4: 生态集成

| 任务 | 优先级 | 说明 |
|------|--------|------|
| Connector 开发工作流 | P3 | 在前端 scaffold + build + test |
| Schema 可视化编辑器 | P3 | 拖拽字段选择 |
| Agent 调用追踪 | P3 | 端到端展示 agent → harbor → upstream 的调用链 |
| 多实例管理 | P3 | 连接多个 Gateway 实例 |

---

## 9. 启动命令集成

前端应该可以通过 Harbor CLI 启动：

```bash
# 启动 gateway + 前端
harbor serve

# 或者只启动前端（连接已有 gateway）
harbor serve --frontend-only --gateway http://localhost:8080
```

Makefile 新增：

```makefile
frontend:
	cd frontend && pnpm dev

frontend-build:
	cd frontend && pnpm build

serve: gateway frontend
	# 并行启动 gateway 和 frontend
```

生产模式下，前端构建为静态文件，由 Gateway 直接服务（在 `/` 路径下），无需额外的前端服务器。

---

## 10. 设计参考与灵感

| 参考产品 | 借鉴点 |
|----------|--------|
| **Grafana** | Metrics 仪表盘布局、时间范围选择器、图表交互 |
| **Postman** | Data Explorer 的请求/响应布局 |
| **Redis Insight** | Memory 浏览的层级展示 |
| **Prisma Studio** | 简洁的数据浏览体验 |
| **MCP Inspector** | Tool schema 展示、参数输入 |
| **Vercel Dashboard** | 统计卡片 + 简洁 UI 风格 |

---

## 附录 A: 配色方案

```
Primary:     #2563EB (Harbor Blue)
Background:  #0F172A (Slate 900)
Surface:     #1E293B (Slate 800)
Border:      #334155 (Slate 700)
Text:        #F8FAFC (Slate 50)
Text Muted:  #94A3B8 (Slate 400)
Success:     #22C55E (Green 500)  — fresh, healthy
Warning:     #EAB308 (Yellow 500) — near-stale, drift
Danger:      #EF4444 (Red 500)    — stale, error
Accent:      #8B5CF6 (Violet 500) — schema, tools
```

默认深色主题（开发者工具定位），可选浅色模式。

## 附录 B: 关键用户流程

### B.1 开发者首次使用

1. `harbor serve` → 打开浏览器
2. Dashboard 显示 "No connectors installed"
3. 点击 "Install your first connector" → Connectors 页面
4. 点击 coingecko 的 [Install]
5. 安装完成 → 自动跳转到 connector 详情
6. 点击 prices 资源的 [Try in Explorer]
7. 在 Explorer 中输入 `ids=bitcoin`，点击 Execute
8. 查看规范化的 Harbor Response

### B.2 调试 agent 输出质量

1. 在 Metrics 页面发现 `list_files` 的压缩率为 1.0（没有压缩）
2. 发现该工具没有 learned schema
3. 点击 [Teach Schema] → 打开 Schema 编辑器
4. 从 Memory 中加载最近的 raw 输出，选择重要字段
5. 预览压缩效果，满意后保存
6. 下次 agent 调用时自动压缩

### B.3 排查 schema drift

1. Dashboard 显示黄色 "1 drift event"
2. 点击进入 Metrics → Schema Drift Events
3. 看到 `list_files` 发生 drift，field hit rate = 0.15
4. 查看 Schema 历史，对比 v2 → v3 的变化
5. 检查 raw 数据确认是上游 API 结构变化
6. 编辑新 schema 或回滚到之前版本
