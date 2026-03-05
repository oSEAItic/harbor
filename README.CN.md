<p align="center">
  <img src="assets/harbor-banner.jpeg" alt="Harbor" width="800" />
</p>

<p align="center">
  <strong>别再给 LLM 喂原始 JSON 了。</strong><br/>
  Harbor 标准化、精选、管控流入 AI Agent 的数据 —— 任何来源，任何密度，由你掌控。
</p>

<p align="center">
  <a href="#快速开始">快速开始</a> &middot;
  <a href="#问题">问题</a> &middot;
  <a href="#mcp-集成">MCP 集成</a> &middot;
  <a href="#创建连接器">构建连接器</a> &middot;
  <a href="AGENTS.md">Agent 文档</a> &middot;
  <a href="LICENSE">许可证</a> &middot;
  <a href="README.md">English</a>
</p>

<p align="center">
  支持 <strong>Claude Code</strong> &middot; <strong>Cursor</strong> &middot; <strong>GPT-4</strong> &middot; <strong>任何 MCP 客户端</strong> &middot; <strong>任何支持函数调用的 LLM</strong>
</p>

---

## Before / After

Agent 调用 CoinGecko，它看到的是这个：

```json
{"bitcoin":{"usd":67234.12,"usd_market_cap":1320984173209,"usd_24h_vol":28394857234,
"usd_24h_change":2.34,"last_updated_at":1707900000},"ethereum":{"usd":3456.78,
"usd_market_cap":415678234567,"usd_24h_vol":12948573456,"usd_24h_change":-0.82,
"last_updated_at":1707900000},"solana":{"usd":134.56,"usd_market_cap":58234567890,
"usd_24h_vol":3948573456,"usd_24h_change":5.67,"last_updated_at":1707900000}}
```

没有 Schema。没有来源归属。不知道哪些字段重要。模型不得不浪费 token 去*搞清楚自己在看什么*，然后才能开始推理。

用 Harbor 之后：

```json
{
  "data": [
    { "id": "bitcoin",  "price_usd": 67234.12, "change_24h": 2.34 },
    { "id": "ethereum", "price_usd": 3456.78,  "change_24h": -0.82 },
    { "id": "solana",   "price_usd": 134.56,   "change_24h": 5.67 }
  ],
  "meta": {
    "source": "coingecko",
    "schema": "crypto.prices.v1",
    "fetched_at": "2026-02-14T12:00:00Z",
    "context": { "summary": "BTC/ETH 相关性高。SOL 波动剧烈。", "age": "2h ago" },
    "recalls": [{ "resource": "coingecko.prices", "age": "1d ago", "summary": "BTC $65k..." }]
  },
  "errors": []
}
```

自描述。精选字段。来源和时间戳。跨会话记忆。零 token 浪费在格式上 —— 全部用于推理。

---

## 快速开始

### 安装

```bash
curl -fsSL https://harbor.oseaitic.com/install | bash
```

> 为当前平台（Linux/macOS、amd64/arm64）下载预编译 binary。无需任何运行时。

或从源码安装：`go install github.com/oseaitic/harbor/cmd/harbor@latest`

### 试一试

```bash
harbor install coingecko
harbor get coingecko.prices --param ids=bitcoin --param vs_currencies=usd
```

### 添加到 Claude Code / Cursor

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

Agent 立刻获得对所有已安装连接器的结构化访问，内置 Schema 学习、记忆和检索。

### 代理任何现有 MCP 服务器

已经在用 MCP 服务器？用 Harbor 包装 —— 一行配置，无需改代码：

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

Harbor 自动发现上游工具。Agent 通过纯文本提示教会精选。后续所有调用自动精选。

---

## 问题

Agent 框架关注的是*编排* —— 调用哪个工具、何时循环、如何规划。没有人关注工具调用返回**之后**发生的事。

三件事会出问题：

**不一致。** 每个 API 返回的形态都不同。模型把智能浪费在解码格式上，而不是对内容推理。

**噪声。** 200 个字段的响应中可能只有 6 个是 Agent 需要的。其余的分散注意力、推高成本。

**泄漏。** "读取发票"的调用在财务摘要旁边返回员工 PII。原始 API 不尊重角色边界。数据一旦进入上下文窗口就意味着暴露 —— 暴露给模型、日志和 prompt 注入攻击。

Harbor 解决这三个问题。三大支柱：

### 标准化 —— 一种格式，任何来源

每个响应都变成 `data[]` + `meta{}` + `errors[]`。Agent 只解析一种格式，始终知道数据从哪里来，永远不会静默失败。

### 精选 —— 按需选择信息密度

Agent 调用 `harbor_learn_schema` 告诉 Harbor 哪些字段重要。Harbor 永久记住。同一份数据，四个层级：

| 层级 | 内容 | 用途 |
|------|------|------|
| `raw` | 原始 API 响应 | 需要完整保真度时 |
| `normalized` | 结构化 `data[]` | 常规 Agent 推理 |
| `compact` | 仅摘要字段 | token 受限的上下文 |
| `summary` | 自然语言一行摘要 | 快速浏览、规划 |

漂移检测监控上游 API —— 如果字段结构变化，Harbor 自动适应。

### 管控 —— Agent 只看到它该看到的

Harbor 根据请求者身份控制哪些字段进入上下文窗口。这不是 API 级别的访问控制（"你能不能调用这个端点？"）—— 这是**上下文级别的访问控制**（"你调用之后看到什么？"）。看不到的字段就不会泄漏。

---

## MCP 集成

### Schema 学习 —— Agent 教学，Harbor 记忆

Harbor 内部不调用 LLM。连接到 Harbor 的 Agent **本身就是** LLM：

1. **首次调用** —— Harbor 返回原始输出，附带提示：
   `[Harbor: No schema for "list_files". Call harbor_learn_schema to enable curation.]`
2. **Agent 教学** —— 调用 `harbor_learn_schema`，指定 `summary_fields` 和 `summary_template`
3. **永久存储** —— 后续所有调用自动精选。同一台机器上的所有 Agent 共享成果。
4. **漂移检测** —— 上游变更时，Harbor 检测并重新学习。

### 记忆与检索

每次调用都缓存到 4 层记忆系统。Agent 跨会话检索：

```
harbor_recall(query="bitcoin")              # 关键词搜索
harbor_recall(id="mem_abc123")              # 检索完整内容
harbor_remember(connector="coingecko",      # 保存分析结论
  note="BTC/ETH 相关性高...")
```

备注作为 `meta.context` 出现在未来每一次调用中 —— 跨会话、跨设备的机构记忆。

### 凭证注入

从操作系统钥匙串注入 API Key —— 密钥不会出现在配置文件中：

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

连接器是 exec 插件 —— 独立程序，读取参数并将 JSON 输出到 stdout。可以用任何语言构建。

```typescript
import { parseArgs, output, buildMeta, handleDescribe } from "@oseaitic/harbor-sdk";

const TOOL_SCHEMAS = [{
  type: "function",
  function: {
    name: "myapi_search",
    description: "搜索项目",
    parameters: {
      type: "object",
      properties: { query: { type: "string" } },
      required: ["query"],
    },
  },
}];

async function main() {
  if (handleDescribe(TOOL_SCHEMAS)) return;
  const { resource, params, auth } = parseArgs();
  const raw = await fetch(`https://api.example.com/search?q=${params.query}`);
  const data = await raw.json();

  output({
    data: data.items.map((item: any) => ({
      id: item.id, title: item.name, score: item.relevance,
    })),
    meta: buildMeta({ source: "myapi", connector_version: "0.1.0", schema: "myapi.search.v1" }),
    raw: null,
    errors: [],
  });
}
main();
```

完整接口约定见 [CONNECTOR_SPEC.md](docs/CONNECTOR_SPEC.md)。

---

## Agent 原生文档

Harbor 附带 **[AGENTS.md](AGENTS.md)** —— 结构化指令，任何 AI Agent 都能直接读取并据此行动。涵盖每个 CLI 命令、每个 MCP 工具、常见工作流决策树和错误恢复模式。

这个文件是 Harbor 哲学的具象化：如果你的工具是 Agent 基础设施，你的文档就应该是 Agent 原生的。

---

## 架构

```
Agent (Claude Code, Cursor, GPT-4, 任意 LLM)
  |
  | MCP / 工具调用
  v
Harbor
  |  标准化 --> 精选 --> 管控
  |  Schema 学习 <--> 漂移检测
  |  记忆存储 <--> 跨会话检索
  |
  v
连接器 + MCP 服务器 (任意来源)
  |
  v
API / 数据库 / 服务
```

---

## 许可证

Apache 2.0 —— 见 [LICENSE](LICENSE)。

<p align="center">
  由 <a href="https://github.com/oseaitic"><strong>oSEAItic</strong></a> 构建<br/>
  <sub>数据像海洋一样流动。Harbor 是 Agent 与海洋相遇的地方。</sub>
</p>
