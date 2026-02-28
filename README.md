<p align="center">
  <img src="assets/harbor-banner.jpeg" alt="Harbor" width="800" />
</p>

<p align="center">
  <strong>The context infrastructure for AI agents.</strong><br/>
  Normalize, compress, and govern the data flowing into your agents — any source, any density, your control.
</p>

<p align="center">
  <a href="#install">Install</a> &middot;
  <a href="#quick-start">Quick Start</a> &middot;
  <a href="#why-harbor">Why Harbor</a> &middot;
  <a href="#creating-a-connector">Build a Connector</a> &middot;
  <a href="#architecture">Architecture</a> &middot;
  <a href="LICENSE">License</a> &middot;
  <a href="README.CN.md">中文</a>
</p>

---

## The Story

**oSEAItic** believes data should flow like the ocean — open, boundless, alive. Every API, every database, every real-time feed is a current in that ocean. AI agents should be able to reach into any current, anywhere, and pull out exactly what they need.

But they can't. Not yet.

Every time an agent connects to a data source — crypto prices, payment records, weather feeds, internal databases — it receives a different shape of data. Different field names, different nesting, different error formats, no provenance, no schema version. The model burns context window just *figuring out what it's looking at* before it can start reasoning.

That is the first problem: **inconsistency**. Agents waste intelligence decoding format instead of reasoning about content.

The second: **waste**. A 200-field API response might contain 6 fields the agent actually needs. The rest is noise — consuming tokens, diluting attention, inflating cost. Hand-writing compression logic for every source means the context engineering layer grows larger than the agent itself.

The third, and most dangerous: **leakage**. When an agent calls a financial API, the response might include employee salaries, SSNs, bank account numbers. The agent's role says "analyst" — it should see revenue and margins, not personal data. But raw API responses don't respect role boundaries. Once data enters the context window, it's exposed — to the model, to the logs, to prompt injection attacks. **What an agent can see defines what an agent can do.** And today, agents see everything.

Three problems. One insight: *agents should never touch raw data.*

That insight became **Harbor**.

The name is the metaphor. Ships of every shape arrive from every ocean — CoinGecko, Stripe, Notion, internal databases. The harbor receives them all at the same dock. Cargo is unloaded into a standard format, inspected, compressed to the density you need, filtered by what you're allowed to see, and dispatched. The agent never asks *"what am I looking at?"* — the schema, source, version, and timestamp are already there. And data that shouldn't be in the context window never enters it.

Harbor is not a wrapper. It is a **self-improving information supply chain** — normalizing, compressing, governing, and remembering every piece of data flowing into AI agents. Agents teach Harbor what fields matter. Harbor detects when upstream APIs change shape and adapts. Every call is cached into a 4-layer memory system. The system gets smarter with every interaction, and every agent benefits from what any agent has learned.

We believe the missing piece in agent infrastructure isn't better models — it's better context. Context that is structured, efficient, governed, and alive.

We open-sourced Harbor because everyone building agents needs this.

**Data flows like the ocean. Harbor is where agents meet that ocean.**

---

## Why Harbor?

Agent frameworks focus on *orchestration* — which tool to call, when to loop, how to plan. Nobody focuses on what happens **after** the tool call returns: the quality, efficiency, and safety of the data landing in the context window.

Harbor does. Three pillars:

### Normalize — One format, any source

```
// What CoinGecko returns
{"bitcoin":{"usd":67234.12}}

// What Stripe returns
{"data":[{"id":"txn_1abc","amount":4999,"currency":"usd","created":1707900000}]}
```

An agent receiving these raw responses has to guess structure, guess semantics, handle errors differently per source, and track nothing about freshness or provenance.

Harbor gives the agent this instead:

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

Every response is **self-describing**. The agent knows what source produced it, what schema to parse against, when it was fetched, and whether errors occurred. Zero tokens wasted on format — all tokens on reasoning.

### Compress — Right density for the task

Not every task needs every field. Harbor maintains 4 layers of the same data:

| Layer | What it holds | When to use it |
|-------|--------------|----------------|
| `raw` | Original API response | Full fidelity debugging |
| `normalized` | Structured `data[]` array | Standard agent reasoning |
| `compact` | Summary fields only | Token-constrained contexts |
| `summary` | Natural language one-liner | Quick scanning, planning |

Agents choose the density. A planning step gets summaries. A deep analysis gets normalized. A debug session gets raw. Same data, four levels of detail, massive token savings.

Schema learning makes this automatic: the agent teaches Harbor which fields matter by calling `harbor_learn_schema`. Harbor remembers permanently. Every future call is compressed. Drift detection monitors field usage — if an upstream API changes shape, Harbor rolls back and re-learns.

### Govern — Only what agents should see

What an agent perceives defines what it can do. Raw API responses don't respect role boundaries — a "read invoices" call might return employee PII alongside financial summaries.

Harbor sits at the boundary. It controls which fields enter the context window based on who's asking. Data that an agent's role shouldn't see never reaches the model, the logs, or the attack surface. This is not API-level access control ("can you call this endpoint?") — it is **context-level access control** ("what do you see when you call it?").

The difference matters. An agent that can't see a field can't reason about it, can't leak it, can't be tricked into revealing it.

### Tool discovery

```bash
harbor tools export
harbor tools export --format mcp
```

Emits tool schemas from Harbor's internal Tool IR. OpenAI function-calling and MCP formats are supported — one resource per connector tool. Drop it into any function-calling agent and the model instantly knows what data sources exist, what parameters they accept, and what they return. No manual tool definitions. No drift between docs and reality.

```json
[
  {
    "type": "function",
    "function": {
      "name": "coingecko_prices",
      "description": "Get current prices for cryptocurrencies",
      "parameters": {
        "type": "object",
        "properties": {
          "ids": { "type": "string", "description": "Comma-separated coin IDs" },
          "vs_currencies": { "type": "string", "description": "Target currencies" }
        },
        "required": ["ids", "vs_currencies"]
      }
    }
  }
]
```

---

## Install

```bash
curl -fsSL https://harbor.oseaitic.com/install | bash
```

> **No runtime required.** The installer downloads a pre-compiled static binary for your platform (Linux/macOS, amd64/arm64). Go, Node.js, and Python are **not** needed to run Harbor — only to build connectors.

Or from source (requires Go):
```bash
go install github.com/oseaitic/harbor/cmd/harbor@latest
```

Optional: override Harbor's local data path (connectors, memory, schemas, metrics):

```bash
export HARBOR_HOME="$PWD/.harbor"
```

## Harbor Cloud

Connect to Harbor Cloud to store credentials and publish private connectors — accessible from any machine or agent session.

```bash
# Sign in
harbor login

# Store a connector's API key in the cloud (client-side encrypted, server never sees plaintext)
harbor auth my-connector

# On another machine: pull credentials from cloud into local keychain
harbor auth sync my-connector

# Publish a private connector to your account
harbor publish my-connector

# Check credential status (local + cloud)
harbor auth status
```

Credentials are encrypted client-side with AES-256-GCM before upload. The server stores only ciphertext and cannot decrypt it.

## Quick Start

```bash
# Install a connector
harbor install coingecko

# Fetch data — normalized, schema-versioned, provenance-tracked
harbor get coingecko.prices --param ids=bitcoin --param vs_currencies=usd

# Export tool schemas — drop directly into your agent's function calling
harbor tools export
harbor tools export --format openai --with-examples

# Configure auth for premium connectors
harbor auth stripe

# Get the raw upstream response alongside normalized data
harbor raw coingecko.prices --param ids=bitcoin --param vs_currencies=usd

# List all installed + available connectors
harbor list

# Machine-readable capability discovery for agents
harbor capabilities --json
harbor doctor --json
harbor agent bootstrap --json

# Recall data from memory
harbor recall coingecko.prices --layer summary
harbor recall --list
harbor recall --search "bitcoin"
```

## Agent Bootstrap (No Repo Context Required)

For agents that only have CLI access (no source tree/docs), use:

```bash
harbor agent bootstrap --json
```

This returns command templates, installed connector/resource capabilities, example invocations, common error recovery hints, and environment diagnostics in one payload.

## MCP Proxy — Wrap Any MCP Server

Harbor can act as a transparent proxy in front of any existing MCP server (Notion, GitHub, filesystem, Slack, etc.). One config line. No API keys. No code changes. Harbor re-discovers the upstream server's tools, and the **agent itself** teaches Harbor how to compress each tool's output.

**Harbor structures. The agent thinks.**

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

### How schema learning works

Harbor never calls an LLM internally. The agent connected to Harbor **is** the LLM — it already has the reasoning ability to decide what fields matter. The learning flow is:

1. **First call** — Harbor proxies the tool call and returns raw upstream output, appending a hint:
   ```
   [Harbor: No compression schema for "list_files". Call harbor_learn_schema
   with tool_name, summary_fields, and summary_template to enable compression.]
   ```
2. **Agent teaches** — The agent reads the raw output, decides which fields are important, and calls `harbor_learn_schema`:
   ```json
   {
     "tool_name": "list_files",
     "summary_fields": ["name", "size", "type"],
     "summary_template": "{name} ({type}, {size} bytes)"
   }
   ```
3. **Schema stored permanently** — All future calls to that tool are automatically compressed. Every agent on the machine benefits from schemas any agent has taught.
4. **Field inventory** — Compressed output lists omitted fields so the agent knows what data is available but hidden. The agent can update the schema if a task needs additional fields, preventing confidently wrong answers from missing data.
5. **Drift detection** — If the upstream API changes shape, Harbor detects field hit rate drops, rolls back the schema, and asks the agent to re-teach.

This works with any agent — Claude, GPT-4, Cursor, Copilot, local models — because the hint is plain text that any LLM can read and act on. No special integration required.

### Credential injection

Third-party MCP servers need API keys. Harbor can inject them from the OS keychain so secrets never appear in config files:

```bash
# Store API key in OS keychain
harbor auth github-pat

# Proxy reads from keychain and injects into the upstream server's environment
harbor proxy --credential GITHUB_TOKEN=github-pat \
  npx @modelcontextprotocol/server-github
```

In Claude Code / Claude Desktop config:
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
Agent (Claude/Cursor/any LLM)
  ↓ MCP stdio
Harbor Proxy (MCP Server)
  ├── schema check → compress if learned
  ├── memory check → return cached if fresh
  ├── harbor_learn_schema → agent teaches compression
  ├── harbor_recall → cross-session memory search
  ↓ MCP stdio (client)
Upstream MCP Server (any)
```

> **Advanced:** For CLI-only workflows without an agent in the loop, you can optionally set `HARBOR_LLM_API_KEY` to let Harbor auto-learn schemas via an external LLM API. See `internal/schema/llm.go` for configuration. This is not the recommended path — agent-driven learning produces better schemas because the agent has task context that a standalone LLM call does not.

## MCP Server — Native MCP for Built-in Connectors

Harbor also exposes all installed connectors as native MCP tools:

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

Tool calls run through Harbor's pipeline — execute, context-compile, and cache to 4-layer memory — so agents get compression and memory for free.

## Memory & Recall

Every `harbor get` and every proxied tool call is cached into a 4-layer memory system:

| Layer | Content | Use case |
|-------|---------|----------|
| `raw` | Original API / tool output | Full fidelity when needed |
| `normalized` | Connector's `data[]` array | Structured records |
| `compact` | Summary fields only | Token-efficient context |
| `summary` | Natural language one-liner | Quick scanning |

Recall data from memory via CLI or MCP tool:

```bash
# Browse recent memories
harbor recall --list

# Search by keyword
harbor recall --search "bitcoin"

# Retrieve a specific memory
harbor recall coingecko.prices --layer compact
```

Agents connected via MCP can call `harbor_recall` to search and retrieve memories across sessions.

### Agent integration (Python example)

```python
import subprocess, json

# Fetch context-engineered data for the agent
result = subprocess.run(
    ["harbor", "get", "coingecko.prices", "--param", "ids=bitcoin", "--param", "vs_currencies=usd"],
    capture_output=True, text=True
)
context = json.loads(result.stdout)

# Feed to LLM — the model receives structured, self-describing context
messages = [
    {"role": "system", "content": "You are a financial analyst. Use the provided data to answer questions."},
    {"role": "user", "content": f"Based on this data:\n{json.dumps(context, indent=2)}\n\nWhat is Bitcoin's current price?"}
]
```

### Agent integration (tool use)

```python
import subprocess, json

# Load Harbor tool schemas into your agent
tools_raw = subprocess.run(["harbor", "tools", "export"], capture_output=True, text=True)
tools = json.loads(tools_raw.stdout)

# Pass to OpenAI / Anthropic / any function-calling model
response = client.chat.completions.create(
    model="gpt-4",
    messages=messages,
    tools=tools,  # Harbor-generated, always in sync with installed connectors
)
```

## Output Format

Every connector returns the same envelope — designed for LLM consumption:

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

| Field | Purpose for agents |
|-------|-------------------|
| `data` | Normalized, schema-conformant records — parse once, use everywhere |
| `meta.source` | Source attribution — the agent knows where data came from |
| `meta.schema` | Schema identifier — the agent knows what fields to expect |
| `meta.fetched_at` | Timestamp — enables staleness-aware reasoning |
| `meta.request_id` | Trace ID — debug and audit agent decisions end-to-end |
| `raw` | Optional upstream response — for when the agent needs full fidelity |
| `errors` | Structured errors with codes — no silent failures, no guessing |

## Creating a Connector

Connectors are exec plugins — standalone binaries that read arguments and write JSON to stdout. Build them in any language. Each connector transforms a raw API into LLM-ready context.

### TypeScript (using harbor-sdk)

```typescript
import { parseArgs, output, buildMeta, handleDescribe } from "harbor-sdk";

const TOOL_SCHEMAS = [
  {
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
  },
];

async function main() {
  // When an agent asks "what tools exist?", emit schemas
  if (handleDescribe(TOOL_SCHEMAS)) return;

  const { resource, params, auth } = parseArgs();

  // Fetch from upstream API
  const rawData = await fetchFromAPI(params, auth);

  // Context engineering: normalize into agent-friendly structure
  const normalized = rawData.items.map((item: any) => ({
    id: item.id,
    title: item.name,           // consistent field naming
    description: item.desc,     // agents expect "description", not "desc"
    score: item.relevance,      // semantic field names
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

### Python (using harbor-sdk)

```python
from harbor_sdk import parse_args, output, build_meta, handle_describe

TOOL_SCHEMAS = [
    {
        "type": "function",
        "function": {
            "name": "myconnector_search",
            "description": "Search for items",
            "parameters": {
                "type": "object",
                "properties": {
                    "query": {"type": "string", "description": "Search query"},
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

    # Context engineering: normalize for the agent
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

### Interface Contract

```
connector --resource <name> --params '<json>' [--raw] [--describe]
```

| Flag | Purpose |
|------|---------|
| `--resource` | Which resource to fetch |
| `--params` | JSON object of parameters |
| `--raw` | Include the raw upstream API response alongside normalized data |
| `--describe` | Emit tool schemas for agent discovery (`harbor tools export`) |

Auth is injected via the `HARBOR_AUTH` environment variable — connectors never manage credentials directly.

## Architecture

```
                    ┌──────────────────────────────────────────────┐
                    │                 AI Agent                      │
                    │    (Claude, GPT-4, Cursor, local LLM, ...)   │
                    └─────────┬────────────────────▲────────────────┘
                              │ tool call          │ governed context
                    ┌─────────▼────────────────────┤────────────────┐
                    │                Harbor                          │
                    │                                                │
                    │  ┌───────────┐ ┌──────────┐ ┌──────────────┐  │
                    │  │ Normalize │→│ Compress │→│   Govern     │  │
                    │  │           │ │          │ │              │  │
                    │  │ Envelope  │ │ 4-layer  │ │ Field-level  │  │
                    │  │ Schema    │ │ Memory   │ │ Access Ctrl  │  │
                    │  │ Provenance│ │ Learning │ │ Role-based   │  │
                    │  └───────────┘ └──────────┘ └──────────────┘  │
                    │                                                │
                    │  ┌──────────────────────────────────────────┐  │
                    │  │ Schema Learning ←→ Drift Detection       │  │
                    │  │ Memory Store ←→ Recall                   │  │
                    │  │ Metrics ←→ Compression Analytics         │  │
                    │  └──────────────────────────────────────────┘  │
                    └──┬────────────┬────────────┬──────────────────┘
                       │            │            │
          ┌────────────▼──┐  ┌──────▼─────┐  ┌──▼────────────┐
          │   coingecko   │  │   stripe   │  │  your-source  │
          │  (connector)  │  │ (connector)│  │  (connector)  │
          └───────┬───────┘  └──────┬─────┘  └──────┬────────┘
                  │                 │               │
          ┌───────▼───────┐  ┌──────▼─────┐  ┌─────▼─────────┐
          │  CoinGecko    │  │  Stripe    │  │  Any API /    │
          │  API          │  │  API       │  │  Database     │
          └───────────────┘  └────────────┘  └───────────────┘

The ocean of data flows upward through Harbor.
Each connector docks at the same port.
Cargo is normalized, compressed, governed, and dispatched.
The agent receives exactly what it needs — nothing more, nothing less.
```

## Project Structure

```
harbor/
├── assets/              # Logo and brand assets
├── cmd/harbor/          # CLI entrypoint
├── internal/
│   ├── cli/             # Command implementations (get, recall, proxy, mcp, ...)
│   ├── protocol/        # Request/response types + validation
│   ├── connector/       # Plugin executor + tool schema export
│   ├── context/         # Context compiler (field filtering + NL summary)
│   ├── memory/          # 4-layer memory store (~/.harbor/memory/)
│   ├── recall/          # harbor_recall MCP tool (shared by mcp + proxy)
│   ├── schema/          # Learned compression schemas (~/.harbor/schemas/)
│   ├── pipeline/        # Execute → compile → memory pipeline
│   ├── proxy/           # Transparent MCP proxy (server + client)
│   ├── mcpserver/       # Native MCP server for built-in connectors
│   ├── auth/            # OS keychain abstraction
│   ├── registry/        # Connector install/list from registry
│   └── generator/       # Connector scaffolding generator
├── gateway/             # Optional REST server for hosted agents
├── sdk/
│   ├── typescript/      # TypeScript connector SDK
│   └── python/          # Python connector SDK
├── schemas/             # Canonical JSON Schemas
├── connectors/
│   ├── coingecko/       # Reference connector (crypto)
│   └── yahoo/           # Reference connector (finance)
├── docs/                # Connector spec (agent-readable)
├── Makefile
└── LICENSE              # Apache 2.0
```

## Gateway

The optional gateway exposes Harbor's protocol over HTTP — for hosted agents, web apps, and multi-tenant environments where agents call data sources via API rather than CLI.

```bash
make gateway
PORT=8080 ./bin/harbor-gateway
```

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{"connector":"coingecko","resource":"prices","params":{"ids":"bitcoin","vs_currencies":"usd"}}'
```

The response is identical to CLI output — same envelope, same context engineering, same governance.

## License

Apache 2.0 — see [LICENSE](LICENSE).

---

<p align="center">
  Built by <a href="https://github.com/oseaitic"><strong>oSEAItic</strong></a><br/>
  <sub>Data should flow like the ocean. We build the infrastructure where AI meets that ocean.</sub>
</p>
