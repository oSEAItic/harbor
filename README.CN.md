<p align="center">
  <img src="assets/harbor-banner.jpeg" alt="Harbor — 一个 CLI，所有数据源，为 LLM Agent 而生" width="800" />
</p>

<p align="center">
  <strong>一个 CLI，所有数据源，为 LLM Agent 而生。</strong><br/>
  将任意外部 API 转化为 LLM 可读的上下文，内置 Schema 版本管理、数据溯源和工具发现。
</p>

<p align="center">
  <a href="#安装">安装</a> &middot;
  <a href="#快速开始">快速开始</a> &middot;
  <a href="#为什么需要上下文工程">上下文工程</a> &middot;
  <a href="#创建连接器">构建连接器</a> &middot;
  <a href="#架构">架构</a> &middot;
  <a href="LICENSE">许可证</a> &middot;
  <a href="README.md">English</a>
</p>

---

## 故事

**oSEAItic** 为能基于真实世界数据进行推理的 AI 构建基础设施。

一切始于 [Little RAG](https://github.com/oseaitic/little-rag) —— 一个检索增强生成平台。检索部分运作良好，但增强部分出了问题。每次我们接入一个新的数据源 —— 加密货币价格、支付记录、天气数据、内部数据库 —— Agent 收到的数据形态都不一样。字段名不同、嵌套结构不同、错误格式不同，没有数据溯源，没有 Schema 版本。模型不得不浪费上下文窗口去*理解自己在看什么*，然后才能进行推理。

我们在手动做上下文工程 —— 为每个 API 的响应编写定制适配器，把它翻译成 LLM 能真正使用的格式。当数据源增长到几十个时，上下文工程层本身变得比 Agent 还要庞大。

我们问自己：*如果每个数据源天生就是 LLM 可读的呢？*

这个问题催生了 **Harbor**。

这个名字来源于港口的本质 —— 不同形态、不同来源的船只在同一个港口停泊。货物被卸载为标准格式，附上溯源元数据，然后统一调度。无论这艘船是从 CoinGecko、Stripe 还是私有数据库驶来 —— 一旦进入港口，一切都遵循同一套协议。Agent 永远不需要问*"我在看什么？"* —— Schema、来源、版本和时间戳都已经在那里了。

Harbor 是 AI Agent 的上下文工程层。每个连接器将上游数据标准化为一致的 JSON 信封，LLM 无需任何前置解析就能直接使用。`harbor tools export` 输出兼容 OpenAI 的函数 Schema，让 Agent 自动发现它能访问哪些数据。模型不只是获得数据 —— 它获得的是**结构化的、自描述的、带溯源追踪的上下文**，最大化上下文窗口中每个 token 的价值。

我们构建 Harbor 是因为 Agent 基础设施缺失的那一块不是更好的模型 —— 而是更好的上下文。我们开源它，因为每一个构建 Agent 的人都需要这个。

**一个 CLI。所有数据源。为 Agent 上下文工程而生。**

---

## 为什么需要上下文工程？

LLM 的能力上限取决于它收到的上下文质量。大多数 Agent 框架关注的是*编排* —— 调用哪个工具、何时循环、如何规划。Harbor 关注的是工具调用返回**之后**发生的事：落入上下文窗口的数据质量。

### 原始 API 响应的问题

```
// CoinGecko 返回的
{"bitcoin":{"usd":67234.12}}

// Stripe 返回的
{"data":[{"id":"txn_1abc","amount":4999,"currency":"usd","created":1707900000}]}
```

Agent 收到这些数据后必须：
1. 理解每个响应的结构（浪费推理 token）
2. 在没有 Schema 参考的情况下猜测字段语义
3. 对每个数据源使用不同的错误处理逻辑
4. 完全无法追踪数据新鲜度、版本或来源

### Harbor 给 Agent 的是什么

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

每个响应都是**自描述的**。Agent 知道：
- **数据来源** (`meta.source`) —— 知道数据从哪里来
- **期望的 Schema** (`meta.schema`) —— 无需猜测即可解析
- **获取时间** (`meta.fetched_at`) —— 支持时效性感知推理
- **连接器版本** (`meta.connector_version`) —— 可复现性
- **是否有错误** (`errors[]`) —— 没有静默失败

这是协议层面的上下文工程。Agent 花零个 token 理解格式，所有 token 都用于对内容进行推理。

### Agent 的工具发现

```bash
harbor tools export
```

输出一个 OpenAI 兼容的函数 Schema 数组 —— 每个连接器的每个资源一个。把它传入任何支持函数调用的 Agent，模型立刻知道有哪些数据源可用、接受什么参数、返回什么结果。无需手动编写工具定义。文档和实际能力永远不会不同步。

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

## 快速开始

```bash
# 安装连接器
harbor install coingecko

# 获取数据 —— 标准化、Schema 版本化、带溯源追踪
harbor get coingecko.prices --param ids=bitcoin --param vs_currencies=usd

# 导出工具 Schema —— 直接传入 Agent 的函数调用
harbor tools export

# 为付费连接器配置认证
harbor auth stripe

# 获取原始上游响应（连同标准化数据）
harbor raw coingecko.prices --param ids=bitcoin --param vs_currencies=usd

# 列出所有已安装和可用的连接器
harbor list
```

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
                        ┌─────────────────────────────────────────┐
                        │              AI Agent                    │
                        │  (Claude, GPT-4, 本地 LLM 等)            │
                        └──────────┬──────────────────▲───────────┘
                                   │ 函数调用          │ 结构化上下文
                        ┌──────────▼──────────────────┤───────────┐
                        │           Harbor CLI                     │
                        │                                          │
                        │  harbor get <connector>.<resource>       │
                        │  harbor tools export                     │
                        │  harbor auth <connector>                 │
                        └──┬────────────┬────────────┬────────────┘
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

每个连接器：
  1. 从 Harbor 接收参数和认证信息
  2. 从上游获取原始数据
  3. 进行上下文工程，转化为标准化 JSON 信封
  4. 返回自描述的、Schema 版本化的数据给 Agent
```

## 项目结构

```
harbor/
├── assets/              # Logo 和品牌素材
├── cmd/harbor/          # CLI 入口
├── internal/
│   ├── cli/             # 命令实现
│   ├── protocol/        # 请求/响应类型 + 验证
│   ├── connector/       # 插件执行器
│   ├── auth/            # 操作系统钥匙串抽象
│   ├── registry/        # 连接器安装/列表（从注册中心）
│   └── cache/           # 本地响应缓存（带 TTL）
├── gateway/             # 可选的 REST 服务器（用于托管 Agent）
├── sdk/
│   ├── typescript/      # TypeScript 连接器 SDK
│   └── python/          # Python 连接器 SDK
├── schemas/             # 规范 JSON Schema
├── connectors/
│   └── coingecko/       # 参考连接器
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

响应与 CLI 输出完全一致 —— 相同的信封、相同的上下文工程、相同的 Agent 就绪性。

## 许可证

Apache 2.0 —— 见 [LICENSE](LICENSE)。

---

<p align="center">
  由 <a href="https://github.com/oseaitic"><strong>oSEAItic</strong></a> 构建<br/>
  <sub>为 AI Agent 打造的上下文工程基础设施。</sub>
</p>
