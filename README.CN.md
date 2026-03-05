<p align="center">
  <img src="assets/harbor-banner.jpeg" alt="Harbor" width="800" />
</p>

<p align="center">
  <strong>让你的 AI Agent 从任意 API 获取结构化上下文。</strong><br/>
  标准化、精选、管控流入 LLM 的数据 —— 任何来源，任何密度，由你掌控。
</p>

<p align="center">
  <a href="#快速开始">快速开始</a> &middot;
  <a href="#为什么选择-harbor">为什么选择 Harbor</a> &middot;
  <a href="#mcp-集成">MCP 集成</a> &middot;
  <a href="#创建连接器">构建连接器</a> &middot;
  <a href="LICENSE">许可证</a> &middot;
  <a href="README.md">English</a>
</p>

---

## 30 秒演示

```bash
$ harbor install coingecko
Installing coingecko v0.2.0... done.

$ harbor get coingecko.prices --param ids=bitcoin --param vs_currencies=usd
```

```json
{
  "data": [
    { "id": "bitcoin", "price_usd": 67234.12, "market_cap_usd": 1320000000000 }
  ],
  "meta": {
    "source": "coingecko",
    "schema": "crypto.prices.v1",
    "fetched_at": "2026-02-14T12:00:00Z",
    "request_id": "a1b2c3d4-..."
  },
  "errors": []
}
```

每个响应都是自描述的：来源、Schema 版本、时间戳、数据溯源。Agent 不需要浪费 token 去*搞清楚自己在看什么* —— 直接开始推理。

---

## 快速开始

### 安装

```bash
curl -fsSL https://harbor.oseaitic.com/install | bash
```

> **无需任何运行时。** 为当前平台（Linux/macOS、amd64/arm64）下载预编译 binary。Go、Node、Python 仅在开发连接器时需要。

或从源码安装：`go install github.com/oseaitic/harbor/cmd/harbor@latest`

### 试一试

```bash
# 安装连接器并获取数据
harbor install coingecko
harbor get coingecko.prices --param ids=bitcoin --param vs_currencies=usd

# 导出工具 Schema —— 直接传入任何支持函数调用的 Agent
harbor tools export
harbor tools export --format openai

# 查看可用连接器
harbor list
```

### 在 Claude Code / Cursor 中使用（MCP）

添加到 MCP 配置（`claude_desktop_config.json` 或 `.cursor/mcp.json`）：

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

Agent 立刻获得对所有已安装连接器的结构化访问。Schema 学习、记忆和检索通过 MCP 工具自动工作。

---

## 为什么选择 Harbor？

Agent 框架关注的是*编排* —— 调用哪个工具、何时循环、如何规划。没有人关注工具调用返回**之后**发生的事：落入上下文窗口的数据的质量、安全性和相关性。

Harbor 关注的就是这件事。三大支柱：

### 标准化 —— 一种格式，任何来源

原始 API 返回的数据形态千差万别。CoinGecko: `{"bitcoin":{"usd":67234.12}}`。Stripe: `{"data":[{"id":"txn_1abc","amount":4999}]}`。每一个都迫使模型去猜测结构、猜测语义、用不同的方式处理错误。

Harbor 将一切标准化为统一信封：`data[]` + `meta{}` + `errors[]`。Agent 只解析一种格式，始终知道数据从哪里来，永远不会静默失败。

### 精选 —— 按需选择信息密度

不是每个任务都需要每个字段。Agent 调用 `harbor_learn_schema` 告诉 Harbor 哪些字段重要 —— Harbor 永久记住。后续所有调用只返回相关字段。

| 层级 | 内容 | 用途 |
|------|------|------|
| `raw` | 原始 API 响应 | 需要完整保真度时 |
| `normalized` | 结构化 `data[]` | 常规 Agent 推理 |
| `compact` | 仅摘要字段 | token 受限的上下文 |
| `summary` | 自然语言一行摘要 | 快速浏览、规划 |

Agent 选择密度。漂移检测监控上游 API —— 如果字段结构变化，Harbor 自动适应。

### 管控 —— Agent 只看到它该看到的

一个"读取发票"的调用可能在财务摘要旁边返回员工 PII。原始 API 不尊重角色边界。

Harbor 根据请求者身份控制哪些字段进入上下文窗口。Agent 角色不该看到的数据，永远不会到达模型、日志或攻击面。这不是 API 级别的访问控制（"你能不能调用这个端点？"）—— 这是**上下文级别的访问控制**（"你调用之后看到什么？"）。

---

## MCP 集成

### 代理任意 MCP 服务器

包装任何现有 MCP 服务器 —— Notion、GitHub、文件系统、Slack。一行配置。Harbor 自动发现上游工具，Agent 通过纯文本提示教会精选。

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

### Schema 学习流程

Harbor 内部不调用 LLM。连接到 Harbor 的 Agent **本身就是** LLM：

1. **首次调用** —— Harbor 代理并返回原始输出，附带提示：
   `[Harbor: No schema for "list_files". Call harbor_learn_schema to enable curation.]`
2. **Agent 教学** —— 调用 `harbor_learn_schema`，指定 `summary_fields` 和 `summary_template`
3. **永久存储** —— 后续所有调用自动精选。每个 Agent 共享成果。
4. **漂移检测** —— 上游变更时，Harbor 检测并重新学习。

适用于任何 LLM（Claude、GPT-4、Cursor、本地模型）—— 提示是纯文本。

### 凭证注入

从操作系统钥匙串注入 API Key，密钥不会出现在配置文件中：

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

---

## 记忆与检索

每次调用都缓存到 4 层记忆系统。跨会话检索：

```bash
harbor recall --list                              # 浏览最近记忆
harbor recall --search "bitcoin"                  # 关键词搜索
harbor recall coingecko.prices --layer compact    # 检索特定记忆
```

### 持久化上下文 —— `harbor_remember`

将分析结论永久保存，跨会话、跨设备持久化：

```bash
harbor remember coingecko "BTC/ETH 相关性高（r=0.94）。SOL 波动剧烈。市场整体偏多。"
```

该备注会作为 `meta.context` 出现在**未来每一次**访问该连接器时 —— 无论是你自己，还是任何设备上的任何 Agent。登录后自动同步到 Harbor Cloud。

---

## Harbor Cloud

跨设备同步凭据、Schema 和记忆。

```bash
harbor login                          # 登录
harbor auth my-connector              # 存储 API Key（客户端加密）
harbor auth sync my-connector         # 在另一台机器上拉取凭据
harbor publish my-connector           # 发布私有连接器
```

凭据使用 AES-256-GCM 客户端加密。服务器只存储密文。

---

## 创建连接器

连接器是 exec 插件 —— 独立的二进制程序，读取参数并将 JSON 输出到 stdout。可以用任何语言构建。

### TypeScript（使用 harbor-sdk）

```typescript
import { parseArgs, output, buildMeta, handleDescribe } from "@oseaitic/harbor-sdk";

const TOOL_SCHEMAS = [{
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
}];

async function main() {
  if (handleDescribe(TOOL_SCHEMAS)) return;

  const { resource, params, auth } = parseArgs();
  const rawData = await fetchFromAPI(params, auth);

  output({
    data: rawData.items.map((item: any) => ({
      id: item.id,
      title: item.name,
      description: item.desc,
      score: item.relevance,
    })),
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

### 接口约定

```
connector --resource <name> --params '<json>' [--raw] [--describe]
```

| 参数 | 用途 |
|------|------|
| `--resource` | 要获取的资源名称 |
| `--params` | 参数的 JSON 对象 |
| `--raw` | 包含上游原始响应 |
| `--describe` | 输出工具 Schema |

认证通过 `HARBOR_AUTH` 环境变量注入 —— 连接器永远不直接管理凭据。

---

## 架构

```
Agent (Claude, GPT-4, Cursor, 本地 LLM, ...)
  |
  | 工具调用 / MCP
  v
Harbor
  |  标准化 --> 精选 --> 管控
  |  Schema 学习 <--> 漂移检测
  |  记忆存储 <--> 检索
  |
  v
连接器 (coingecko, stripe, your-source, 任意 MCP 服务器)
  |
  v
API / 数据库 / 服务
```

---

## 许可证

Apache 2.0 —— 见 [LICENSE](LICENSE)。

---

<p align="center">
  由 <a href="https://github.com/oseaitic"><strong>oSEAItic</strong></a> 构建<br/>
  <sub>数据像海洋一样流动。Harbor 是 Agent 与海洋相遇的地方。</sub>
</p>
