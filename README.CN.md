<p align="center">
  <img src="assets/harbor-banner.jpeg" alt="Harbor" width="800" />
</p>

<p align="center">
  <strong>AI Agent 的上下文基础设施。</strong><br/>
  标准化、压缩、管控流入 Agent 的数据 —— 任何来源，任何密度，由你掌控。
</p>

<p align="center">
  <a href="#安装">安装</a> &middot;
  <a href="#快速开始">快速开始</a> &middot;
  <a href="#为什么选择-harbor">为什么选择 Harbor</a> &middot;
  <a href="#创建连接器">构建连接器</a> &middot;
  <a href="#架构">架构</a> &middot;
  <a href="LICENSE">许可证</a> &middot;
  <a href="README.md">English</a>
</p>

---

## 故事

**oSEAItic** 相信数据应该像海洋一样流动 —— 开放、无界、生生不息。每一个 API、每一个数据库、每一条实时数据流都是这片海洋中的一道洋流。AI Agent 应该能够探入任何一道洋流，随时随地取出它需要的东西。

但它们做不到。还做不到。

每当一个 Agent 连接到一个数据源 —— 加密货币价格、支付记录、天气数据、内部数据库 —— 它收到的数据形态都不一样。字段名不同、嵌套结构不同、错误格式不同，没有数据溯源，没有 Schema 版本。模型不得不浪费上下文窗口去*搞清楚自己在看什么*，然后才能开始推理。

这是第一个问题：**不一致**。Agent 把智能浪费在解码格式上，而不是对内容进行推理。

第二个：**浪费**。一个 200 字段的 API 响应中，Agent 真正需要的可能只有 6 个字段。其余的都是噪声 —— 消耗 token、分散注意力、推高成本。为每个数据源手写压缩逻辑，意味着上下文工程层本身变得比 Agent 还要庞大。

第三个，也是最危险的：**泄漏**。当一个 Agent 调用财务 API 时，响应可能包含员工薪资、身份证号、银行账户。Agent 的角色是"分析师" —— 它应该看到营收和利润率，而不是个人数据。但原始 API 响应不尊重角色边界。数据一旦进入上下文窗口，就意味着暴露 —— 暴露给模型、暴露给日志、暴露给 prompt 注入攻击。**Agent 能看到什么，就决定了它能做什么。** 而今天，Agent 看到了一切。

三个问题，一个洞察：*Agent 永远不应该直接触碰原始数据。*

这个洞察催生了 **Harbor**。

名字就是隐喻。不同形态的船只从不同的海域驶来 —— CoinGecko、Stripe、Notion、内部数据库。港口统一接收。货物被卸载为标准格式，接受检验，按需压缩到你需要的信息密度，根据权限过滤你被允许看到的部分，然后分发出去。Agent 永远不需要问*"我在看什么？"* —— Schema、来源、版本和时间戳都已经在那里了。而不应该出现在上下文窗口里的数据，从一开始就不会进入。

Harbor 不是一个封装器。它是一条**自我进化的信息供应链** —— 标准化、压缩、管控、记忆每一份流入 AI Agent 的数据。Agent 教 Harbor 哪些字段重要。Harbor 检测到上游 API 的数据结构变化时自动适应。每次调用都缓存到 4 层记忆系统。系统随着每一次交互变得更智能，每一个 Agent 都受益于任何 Agent 所教会的东西。

我们相信，Agent 基础设施缺失的那一块不是更好的模型 —— 而是更好的上下文。结构化的、高效的、受管控的、活着的上下文。

我们开源 Harbor，因为每一个构建 Agent 的人都需要这个。

**数据像海洋一样流动。Harbor 是 Agent 与海洋相遇的地方。**

---

## 为什么选择 Harbor？

Agent 框架关注的是*编排* —— 调用哪个工具、何时循环、如何规划。没有人关注工具调用返回**之后**发生的事：落入上下文窗口的数据的质量、效率和安全性。

Harbor 关注的就是这件事。三大支柱：

### 标准化 —— 一种格式，任何来源

```
// CoinGecko 返回的
{"bitcoin":{"usd":67234.12}}

// Stripe 返回的
{"data":[{"id":"txn_1abc","amount":4999,"currency":"usd","created":1707900000}]}
```

Agent 收到这些原始响应后，不得不猜测结构、猜测语义、为每个数据源使用不同的错误处理，对数据新鲜度和来源一无所知。

Harbor 给 Agent 的是这个：

```json
{
  "data": [
    { "id": "bitcoin", "price_usd": 67234.12, "market_cap_usd": 1320000000000 }
  ],
  "meta": {
    "source": "coingecko",
    "connector_version": "0.1.0",
    "schema": "crypto.prices.v1",
    "tool_schema_version": "harbor.tools.v1",
    "fetched_at": "2026-02-14T12:00:00Z",
    "request_id": "a1b2c3d4-..."
  },
  "raw": null,
  "errors": []
}
```

每个响应都是**自描述的**。Agent 知道数据从哪里来、该按什么 Schema 解析、什么时候获取的、是否有错误。零个 token 浪费在格式上 —— 所有 token 都用于推理。

### 压缩 —— 按需选择信息密度

不是每个任务都需要每个字段。Harbor 为同一份数据维护 4 个层级：

| 层级 | 内容 | 何时使用 |
|------|------|---------|
| `raw` | 原始 API 响应 | 需要完整保真度时 |
| `normalized` | 结构化 `data[]` 数组 | 常规 Agent 推理 |
| `compact` | 仅摘要字段 | token 受限的上下文 |
| `summary` | 自然语言一行摘要 | 快速浏览、规划 |

Agent 选择密度。规划阶段用摘要。深度分析用标准化数据。调试会话用原始数据。同一份数据，四个细节层级，大幅节省 token。

Schema 学习让这一切自动化：Agent 调用 `harbor_learn_schema` 告诉 Harbor 哪些字段重要。Harbor 永久记住。后续所有调用自动压缩。漂移检测监控字段使用率 —— 如果上游 API 变更了数据结构，Harbor 自动回滚并重新学习。

### 管控 —— Agent 只看到它该看到的

Agent 感知到什么，就决定了它能做什么。原始 API 响应不尊重角色边界 —— 一个"读取发票"的调用可能在财务摘要旁边返回员工 PII。

Harbor 守在边界上。它根据请求者的身份控制哪些字段进入上下文窗口。Agent 角色不该看到的数据，永远不会到达模型、日志或攻击面。这不是 API 级别的访问控制（"你能不能调用这个端点？"）—— 这是**上下文级别的访问控制**（"你调用之后看到什么？"）。

区别至关重要。一个看不到某个字段的 Agent，就不会基于它推理、不会泄漏它、不会被诱骗说出它。

### 工具发现

```bash
harbor tools export
harbor tools export --format mcp
```

基于 Harbor 内部 Tool IR 导出工具 Schema。支持 OpenAI 函数调用和 MCP 两种格式 —— 每个连接器资源一个工具。传入任何支持函数调用的 Agent，模型立刻知道有哪些数据源可用、接受什么参数、返回什么结果。无需手动编写工具定义。文档和实际能力永远不会不同步。

```json
[
  {
    "type": "function",
    "function": {
      "name": "coingecko_prices",
      "description": "获取加密货币当前价格",
      "parameters": {
        "type": "object",
        "properties": {
          "ids": { "type": "string", "description": "逗号分隔的币种 ID" },
          "vs_currencies": { "type": "string", "description": "目标货币" }
        },
        "required": ["ids", "vs_currencies"]
      }
    }
  }
]
```

---

## 安装

```bash
# 从源码安装
go install github.com/oseaitic/harbor/cmd/harbor@latest

# 或本地构建
make build
./bin/harbor version
```

可选：覆盖 Harbor 的本地数据路径（连接器、记忆、Schema、指标）：

```bash
export HARBOR_HOME="$PWD/.harbor"
```

## 快速开始

```bash
# 安装连接器
harbor install coingecko

# 获取数据 —— 标准化、Schema 版本化、带溯源追踪
harbor get coingecko.prices --param ids=bitcoin --param vs_currencies=usd

# 导出工具 Schema —— 直接传入 Agent 的函数调用
harbor tools export
harbor tools export --format openai --with-examples

# 为付费连接器配置认证
harbor auth stripe

# 获取原始上游响应（连同标准化数据）
harbor raw coingecko.prices --param ids=bitcoin --param vs_currencies=usd

# 列出所有已安装和可用的连接器
harbor list

# Agent 可读的能力发现
harbor capabilities --json
harbor doctor --json
harbor agent bootstrap --json

# 从记忆中检索数据
harbor recall coingecko.prices --layer summary
harbor recall --list
harbor recall --search "bitcoin"
```

## Agent 引导（无需代码仓库上下文）

对于只有 CLI 访问权限（没有源代码树/文档）的 Agent，使用：

```bash
harbor agent bootstrap --json
```

返回命令模板、已安装连接器/资源的能力描述、示例调用、常见错误恢复提示和环境诊断信息，一个 payload 全部搞定。

## MCP Proxy —— 包装任意 MCP 服务器

Harbor 可以作为透明代理，挡在任何现有 MCP 服务器（Notion、GitHub、文件系统、Slack 等）前面。一行配置。无需 API Key。无需改代码。Harbor 自动发现上游服务器的工具，由 **Agent 自身** 教会 Harbor 如何压缩每个工具的输出。

**Harbor 负责结构化。Agent 负责思考。**

```json
{
  "mcpServers": {
    "notion": {
      "command": "harbor",
      "args": ["proxy", "notion-mcp-server"]
    }
  }
}
```

### Schema 学习的工作原理

Harbor 内部不调用任何 LLM。连接到 Harbor 的 Agent **本身就是** LLM —— 它已经具备推理能力来判断哪些字段重要。学习流程如下：

1. **首次调用** —— Harbor 代理工具调用并返回原始上游输出，附带一个提示：
   ```
   [Harbor: No compression schema for "list_files". Call harbor_learn_schema
   with tool_name, summary_fields, and summary_template to enable compression.]
   ```
2. **Agent 教学** —— Agent 阅读原始输出，决定哪些字段重要，然后调用 `harbor_learn_schema`：
   ```json
   {
     "tool_name": "list_files",
     "summary_fields": ["name", "size", "type"],
     "summary_template": "{name} ({type}, {size} bytes)"
   }
   ```
3. **Schema 永久存储** —— 该工具的所有后续调用自动压缩。同一台机器上的所有 Agent 共享任何 Agent 教会的 Schema。
4. **字段清单** —— 压缩输出附带省略字段列表，Agent 知道哪些数据存在但被隐藏。如果某个任务需要额外字段，Agent 可以更新 Schema，避免因缺失数据而给出自信但错误的答案。
5. **漂移检测** —— 如果上游 API 变更了数据结构，Harbor 检测到字段命中率下降，自动回滚 Schema，并请求 Agent 重新教学。

这适用于任何 Agent —— Claude、GPT-4、Cursor、Copilot、本地模型 —— 因为提示是纯文本，任何 LLM 都能读取并据此行动。无需特殊集成。

### 凭证注入

第三方 MCP 服务器需要 API Key。Harbor 可以从操作系统钥匙串注入，密钥不会出现在配置文件中：

```bash
# 将 API Key 存入操作系统钥匙串
harbor auth github-pat

# Proxy 从钥匙串读取并注入到上游服务器的环境变量
harbor proxy --credential GITHUB_TOKEN=github-pat \
  npx @modelcontextprotocol/server-github
```

Claude Code / Claude Desktop 配置：
```json
{
  "mcpServers": {
    "github": {
      "command": "harbor",
      "args": ["proxy", "--credential", "GITHUB_TOKEN=github-pat",
               "npx", "@modelcontextprotocol/server-github"]
    }
  }
}
```

```
Agent (Claude/Cursor/任意 LLM)
  ↓ MCP stdio
Harbor Proxy (MCP Server)
  ├── schema 检查 → 有 schema 则压缩
  ├── memory 检查 → 有缓存则直接返回
  ├── harbor_learn_schema → Agent 教会压缩
  ├── harbor_recall → 跨会话记忆搜索
  ↓ MCP stdio (client)
Upstream MCP Server (任意)
```

> **高级用法：** 如果是纯 CLI 工作流（没有 Agent 参与），可以选择设置 `HARBOR_LLM_API_KEY` 让 Harbor 通过外部 LLM API 自动学习 Schema。详见 `internal/schema/llm.go` 的配置说明。这不是推荐路径 —— Agent 驱动的学习效果更好，因为 Agent 拥有任务上下文，而独立的 LLM 调用没有。

## MCP Server —— 内置连接器的原生 MCP 支持

Harbor 也可以将所有已安装的连接器暴露为原生 MCP 工具：

```json
{
  "mcpServers": {
    "harbor": {
      "command": "harbor",
      "args": ["mcp"]
    }
  }
}
```

工具调用经过 Harbor 的完整 pipeline —— 执行、上下文编译、缓存到 4 层记忆 —— Agent 自动获得压缩和记忆功能。

## 记忆与检索

每次 `harbor get` 和每次代理的工具调用都会缓存到 4 层记忆系统：

| 层级 | 内容 | 用途 |
|------|------|------|
| `raw` | 原始 API / 工具输出 | 需要完整保真度时使用 |
| `normalized` | 连接器的 `data[]` 数组 | 结构化记录 |
| `compact` | 仅摘要字段 | 省 token 的上下文 |
| `summary` | 自然语言一行摘要 | 快速浏览 |

通过 CLI 或 MCP 工具检索记忆：

```bash
# 浏览最近记忆
harbor recall --list

# 关键词搜索
harbor recall --search "bitcoin"

# 检索特定记忆
harbor recall coingecko.prices --layer compact
```

通过 MCP 连接的 Agent 可以调用 `harbor_recall` 来跨会话搜索和检索记忆。

### Agent 集成（Python 示例）

```python
import subprocess, json

# 获取上下文工程处理后的数据
result = subprocess.run(
    ["harbor", "get", "coingecko.prices", "--param", "ids=bitcoin", "--param", "vs_currencies=usd"],
    capture_output=True, text=True
)
context = json.loads(result.stdout)

# 喂给 LLM —— 模型收到结构化的、自描述的上下文
messages = [
    {"role": "system", "content": "你是一名金融分析师。使用提供的数据回答问题。"},
    {"role": "user", "content": f"根据以下数据:\n{json.dumps(context, indent=2)}\n\n比特币当前价格是多少？"}
]
```

### Agent 集成（工具调用）

```python
import subprocess, json

# 将 Harbor 工具 Schema 加载到 Agent 中
tools_raw = subprocess.run(["harbor", "tools", "export"], capture_output=True, text=True)
tools = json.loads(tools_raw.stdout)

# 传给 OpenAI / Anthropic / 任何支持函数调用的模型
response = client.chat.completions.create(
    model="gpt-4",
    messages=messages,
    tools=tools,  # Harbor 生成，始终与已安装的连接器保持同步
)
```

## 输出格式

每个连接器返回相同的信封结构 —— 专为 LLM 消费设计：

```json
{
  "data": [...],
  "meta": {
    "source": "coingecko",
    "connector_version": "0.1.0",
    "schema": "crypto.prices.v1",
    "tool_schema_version": "harbor.tools.v1",
    "fetched_at": "2026-02-14T12:00:00Z",
    "request_id": "a1b2c3d4-..."
  },
  "raw": null,
  "errors": []
}
```

| 字段 | 对 Agent 的意义 |
|------|----------------|
| `data` | 标准化的、符合 Schema 的记录 —— 一次解析，处处可用 |
| `meta.source` | 来源归属 —— Agent 知道数据从哪里来 |
| `meta.schema` | Schema 标识 —— Agent 知道期望哪些字段 |
| `meta.fetched_at` | 时间戳 —— 支持时效性感知推理 |
| `meta.request_id` | 追踪 ID —— 端到端调试和审计 Agent 决策 |
| `raw` | 可选的上游原始响应 —— 当 Agent 需要完整保真度时使用 |
| `errors` | 结构化错误（含错误码） —— 没有静默失败，没有猜测 |

## 创建连接器

连接器是 exec 插件 —— 独立的二进制程序，读取参数并将 JSON 输出到标准输出。可以用任何语言构建。每个连接器将原始 API 转化为 LLM 就绪的上下文。

### TypeScript（使用 harbor-sdk）

```typescript
import { parseArgs, output, buildMeta, handleDescribe } from "harbor-sdk";

const TOOL_SCHEMAS = [
  {
    type: "function",
    function: {
      name: "myconnector_search",
      description: "搜索项目",
      parameters: {
        type: "object",
        properties: {
          query: { type: "string", description: "搜索查询" },
        },
        required: ["query"],
      },
    },
  },
];

async function main() {
  // 当 Agent 询问"有哪些工具可用？"时，输出 Schema
  if (handleDescribe(TOOL_SCHEMAS)) return;

  const { resource, params, auth } = parseArgs();

  // 从上游 API 获取数据
  const rawData = await fetchFromAPI(params, auth);

  // 上下文工程：标准化为 Agent 友好的结构
  const normalized = rawData.items.map((item: any) => ({
    id: item.id,
    title: item.name,           // 统一字段命名
    description: item.desc,     // Agent 期望 "description" 而非 "desc"
    score: item.relevance,      // 语义化字段名
  }));

  output({
    data: normalized,
    meta: buildMeta({
      source: "my-connector",
      connector_version: "0.1.0",
      schema: "mydata.search.v1",
    }),
    raw: null,
    errors: [],
  });
}

main();
```

### Python（使用 harbor-sdk）

```python
from harbor_sdk import parse_args, output, build_meta, handle_describe

TOOL_SCHEMAS = [
    {
        "type": "function",
        "function": {
            "name": "myconnector_search",
            "description": "搜索项目",
            "parameters": {
                "type": "object",
                "properties": {
                    "query": {"type": "string", "description": "搜索查询"},
                },
                "required": ["query"],
            },
        },
    },
]

def main():
    if handle_describe(TOOL_SCHEMAS):
        return

    args = parse_args()
    raw_data = fetch_from_api(args["params"], args["auth"])

    # 上下文工程：为 Agent 标准化
    normalized = [
        {"id": item["id"], "title": item["name"], "description": item["desc"]}
        for item in raw_data["items"]
    ]

    output({
        "data": normalized,
        "meta": build_meta("my-connector", "0.1.0", "mydata.search.v1"),
        "raw": None,
        "errors": [],
    })

main()
```

### 接口约定

```
connector --resource <name> --params '<json>' [--raw] [--describe]
```

| 参数 | 用途 |
|------|------|
| `--resource` | 要获取的资源名称 |
| `--params` | 参数的 JSON 对象 |
| `--raw` | 在标准化数据旁包含上游原始响应 |
| `--describe` | 输出工具 Schema，用于 Agent 发现（`harbor tools export`） |

认证通过 `HARBOR_AUTH` 环境变量注入 —— 连接器永远不直接管理凭据。

## 架构

```
                    ┌──────────────────────────────────────────────┐
                    │                 AI Agent                      │
                    │    (Claude, GPT-4, Cursor, 本地 LLM, ...)     │
                    └─────────┬────────────────────▲────────────────┘
                              │ 工具调用            │ 受管控的上下文
                    ┌─────────▼────────────────────┤────────────────┐
                    │                Harbor                          │
                    │                                                │
                    │  ┌───────────┐ ┌──────────┐ ┌──────────────┐  │
                    │  │  标准化   │→│   压缩   │→│    管控      │  │
                    │  │           │ │          │ │              │  │
                    │  │ 统一信封  │ │ 4 层记忆 │ │ 字段级       │  │
                    │  │ Schema    │ │ Schema   │ │ 访问控制     │  │
                    │  │ 数据溯源  │ │ 自学习   │ │ 基于角色     │  │
                    │  └───────────┘ └──────────┘ └──────────────┘  │
                    │                                                │
                    │  ┌──────────────────────────────────────────┐  │
                    │  │ Schema 学习 ←→ 漂移检测                  │  │
                    │  │ 记忆存储 ←→ 检索                         │  │
                    │  │ 指标 ←→ 压缩分析                         │  │
                    │  └──────────────────────────────────────────┘  │
                    └──┬────────────┬────────────┬──────────────────┘
                       │            │            │
          ┌────────────▼──┐  ┌──────▼─────┐  ┌──▼────────────┐
          │   coingecko   │  │   stripe   │  │  your-source  │
          │   (连接器)     │  │  (连接器)   │  │   (连接器)     │
          └───────┬───────┘  └──────┬─────┘  └──────┬────────┘
                  │                 │               │
          ┌───────▼───────┐  ┌──────▼─────┐  ┌─────▼─────────┐
          │  CoinGecko    │  │  Stripe    │  │  任意 API /    │
          │  API          │  │  API       │  │  数据库        │
          └───────────────┘  └────────────┘  └───────────────┘

数据的海洋自下而上流经 Harbor。
每个连接器在同一个港口停泊。
货物被标准化、压缩、管控，然后分发。
Agent 收到的恰好是它需要的 —— 不多，不少。
```

## 项目结构

```
harbor/
├── assets/              # Logo 和品牌素材
├── cmd/harbor/          # CLI 入口
├── internal/
│   ├── cli/             # 命令实现 (get, recall, proxy, mcp, ...)
│   ├── protocol/        # 请求/响应类型 + 验证
│   ├── connector/       # 插件执行器 + 工具 Schema 导出
│   ├── context/         # 上下文编译器 (字段过滤 + 自然语言摘要)
│   ├── memory/          # 4 层记忆存储 (~/.harbor/memory/)
│   ├── recall/          # harbor_recall MCP 工具 (mcp + proxy 共享)
│   ├── schema/          # 学习到的压缩 Schema (~/.harbor/schemas/)
│   ├── pipeline/        # 执行 → 编译 → 记忆 pipeline
│   ├── proxy/           # 透明 MCP 代理 (server + client)
│   ├── mcpserver/       # 内置连接器的原生 MCP 服务器
│   ├── auth/            # 操作系统钥匙串抽象
│   ├── registry/        # 连接器安装/列表（从注册中心）
│   └── generator/       # 连接器脚手架生成器
├── gateway/             # 可选的 REST 服务器（用于托管 Agent）
├── sdk/
│   ├── typescript/      # TypeScript 连接器 SDK
│   └── python/          # Python 连接器 SDK
├── schemas/             # 规范 JSON Schema
├── connectors/
│   ├── coingecko/       # 参考连接器（加密货币）
│   └── yahoo/           # 参考连接器（金融）
├── docs/                # 连接器规范（Agent 可读）
├── Makefile
└── LICENSE              # Apache 2.0
```

## 网关

可选的网关通过 HTTP 暴露 Harbor 协议 —— 适用于托管 Agent、Web 应用和多租户环境（Agent 通过 API 而非 CLI 调用数据源）。

```bash
make gateway
PORT=8080 ./bin/harbor-gateway
```

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{"connector":"coingecko","resource":"prices","params":{"ids":"bitcoin","vs_currencies":"usd"}}'
```

响应与 CLI 输出完全一致 —— 相同的信封、相同的上下文工程、相同的管控。

## 许可证

Apache 2.0 —— 见 [LICENSE](LICENSE)。

---

<p align="center">
  由 <a href="https://github.com/oseaitic"><strong>oSEAItic</strong></a> 构建<br/>
  <sub>数据应该像海洋一样流动。我们构建 AI 与海洋相遇的基础设施。</sub>
</p>
