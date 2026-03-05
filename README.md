<p align="center">
  <img src="assets/harbor-banner.jpeg" alt="Harbor" width="800" />
</p>

<p align="center">
  <strong>Give your AI agent structured context from any API.</strong><br/>
  Normalize, curate, and govern data flowing into LLMs — any source, any density, your control.
</p>

<p align="center">
  <a href="#quick-start">Quick Start</a> &middot;
  <a href="#why-harbor">Why Harbor</a> &middot;
  <a href="#mcp-integration">MCP Integration</a> &middot;
  <a href="#creating-a-connector">Build a Connector</a> &middot;
  <a href="LICENSE">License</a> &middot;
  <a href="README.CN.md">中文</a>
</p>

---

## 30-Second Demo

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

Every response is self-describing: source, schema version, timestamp, provenance. The agent never wastes tokens figuring out *what it's looking at* — it goes straight to reasoning.

---

## Quick Start

### Install

```bash
curl -fsSL https://harbor.oseaitic.com/install | bash
```

> **No runtime required.** Pre-compiled binary for Linux/macOS (amd64/arm64). Go, Node, Python only needed for building connectors.

Or from source: `go install github.com/oseaitic/harbor/cmd/harbor@latest`

### Try it

```bash
# Install a connector and fetch data
harbor install coingecko
harbor get coingecko.prices --param ids=bitcoin --param vs_currencies=usd

# Export tool schemas — drop into any function-calling agent
harbor tools export
harbor tools export --format openai

# See what's available
harbor list
```

### Use with Claude Code / Cursor (MCP)

Add to your MCP config (`claude_desktop_config.json` or `.cursor/mcp.json`):

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

Your agent now has structured access to every installed connector. Schema learning, memory, and recall work automatically through MCP tools.

---

## Why Harbor?

Agent frameworks focus on *orchestration* — which tool to call, when to loop, how to plan. Nobody focuses on what happens **after** the tool call returns: the quality, safety, and relevance of the data landing in the context window.

Harbor does. Three pillars:

### Normalize — One format, any source

Raw APIs return wildly different shapes. CoinGecko: `{"bitcoin":{"usd":67234.12}}`. Stripe: `{"data":[{"id":"txn_1abc","amount":4999}]}`. Each forces the model to guess structure, guess semantics, handle errors differently.

Harbor normalizes everything into a single envelope: `data[]` + `meta{}` + `errors[]`. The agent parses one format, always knows where data came from, and never fails silently.

### Curate — Right density for the task

Not every task needs every field. The agent teaches Harbor what matters by calling `harbor_learn_schema` — and Harbor remembers permanently. Every future call returns only relevant fields.

| Layer | Content | Use case |
|-------|---------|----------|
| `raw` | Original API response | Full fidelity debugging |
| `normalized` | Structured `data[]` | Standard agent reasoning |
| `compact` | Summary fields only | Token-efficient contexts |
| `summary` | Natural language one-liner | Quick scanning, planning |

Agents choose the density. Drift detection monitors upstream APIs — if fields change shape, Harbor adapts.

### Govern — Only what agents should see

A "read invoices" call might return employee PII alongside financial summaries. Raw APIs don't respect role boundaries.

Harbor controls which fields enter the context window based on who's asking. Data an agent's role shouldn't see never reaches the model, the logs, or the attack surface. This isn't API-level access control ("can you call this endpoint?") — it's **context-level access control** ("what do you see when you call it?").

---

## MCP Integration

### Proxy any MCP server

Wrap any existing MCP server — Notion, GitHub, filesystem, Slack. One config line. Harbor re-discovers upstream tools and the agent teaches compression via plain text hints.

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

### Schema learning flow

Harbor never calls an LLM. The connected agent **is** the LLM:

1. **First call** — Harbor proxies and returns raw output with a hint:
   `[Harbor: No schema for "list_files". Call harbor_learn_schema to enable curation.]`
2. **Agent teaches** — calls `harbor_learn_schema` with `summary_fields` and `summary_template`
3. **Stored permanently** — all future calls are curated. Every agent benefits.
4. **Drift detection** — if upstream changes shape, Harbor detects and re-learns.

Works with any LLM (Claude, GPT-4, Cursor, local models) — the hint is plain text.

### Credential injection

Inject API keys from the OS keychain so secrets never appear in config files:

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

## Memory & Recall

Every call is cached into a 4-layer memory system. Recall across sessions:

```bash
harbor recall --list                              # Browse recent
harbor recall --search "bitcoin"                  # Search by keyword
harbor recall coingecko.prices --layer compact    # Retrieve specific memory
```

### Persistent context — `harbor_remember`

Save analysis conclusions that persist across sessions and devices:

```bash
harbor remember coingecko "BTC/ETH correlation high (r=0.94). SOL volatile. Market bullish."
```

This appears as `meta.context` on **every future call** to that connector — for you, or any agent on any device. Notes sync to Harbor Cloud on login.

---

## Harbor Cloud

Sync credentials, schemas, and memories across machines.

```bash
harbor login                          # Sign in
harbor auth my-connector              # Store API key (client-side encrypted)
harbor auth sync my-connector         # Pull credentials on another machine
harbor publish my-connector           # Publish private connector
```

Credentials are encrypted client-side with AES-256-GCM. The server stores only ciphertext.

---

## Creating a Connector

Connectors are exec plugins — standalone binaries that read arguments and write JSON to stdout. Build in any language.

### TypeScript (using harbor-sdk)

```typescript
import { parseArgs, output, buildMeta, handleDescribe } from "@oseaitic/harbor-sdk";

const TOOL_SCHEMAS = [{
  type: "function",
  function: {
    name: "myconnector_search",
    description: "Search for items",
    parameters: {
      type: "object",
      properties: {
        query: { type: "string", description: "Search query" },
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

### Interface contract

```
connector --resource <name> --params '<json>' [--raw] [--describe]
```

| Flag | Purpose |
|------|---------|
| `--resource` | Which resource to fetch |
| `--params` | JSON object of parameters |
| `--raw` | Include raw upstream response |
| `--describe` | Emit tool schemas for discovery |

Auth is injected via `HARBOR_AUTH` env var — connectors never manage credentials directly.

---

## Architecture

```
Agent (Claude, GPT-4, Cursor, local LLM, ...)
  |
  | tool call / MCP
  v
Harbor
  |  Normalize --> Curate --> Govern
  |  Schema Learning <--> Drift Detection
  |  Memory Store <--> Recall
  |
  v
Connectors (coingecko, stripe, your-source, any MCP server)
  |
  v
APIs / Databases / Services
```

---

## License

Apache 2.0 — see [LICENSE](LICENSE).

---

<p align="center">
  Built by <a href="https://github.com/oseaitic"><strong>oSEAItic</strong></a><br/>
  <sub>Data flows like the ocean. Harbor is where agents meet that ocean.</sub>
</p>
