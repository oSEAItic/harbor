<p align="center">
  <img src="assets/harbor-banner.jpeg" alt="Harbor" width="800" />
</p>

<p align="center">
  <strong>Claude analyzes. Gemini continues. Harbor remembers.</strong><br/>
  Shared context for AI agents — any model, any client, one protocol.
</p>

<p align="center">
  <a href="#quick-start">Quick Start</a> &middot;
  <a href="#the-problem">The Problem</a> &middot;
  <a href="#cross-agent-memory">Cross-Agent Memory</a> &middot;
  <a href="#mcp-integration">MCP Integration</a> &middot;
  <a href="#creating-a-connector">Build a Connector</a> &middot;
  <a href="AGENTS.md">Agent Docs</a> &middot;
  <a href="README.CN.md">中文</a>
</p>

<p align="center">
  Works with <strong>Claude Code</strong> &middot; <strong>Gemini CLI</strong> &middot; <strong>Codex</strong> &middot; <strong>Cursor</strong> &middot; <strong>Minimax</strong> &middot;<strong>Any MCP client</strong> &middot; <strong>Any Function Calling LLMs</strong>
</p>

<p align="center">
  <sub>Harbor is an open-source agent context protocol by <a href="https://github.com/oseaitic">oSEAItic</a>. Not affiliated with CNCF Harbor (container registry).</sub>
</p>

---

## Cross-Agent Memory

Claude Code analyzes your crypto portfolio. You close the session. Later, a different agent picks up where Claude left off — no copy-paste, no re-prompting.

<p align="center">
  <img src="assets/cross-agent-openclaw.png" alt="Another agent reads Claude Code's analysis from Harbor memory" width="900" />
</p>

<p align="center">
  <em>"Market Snapshot (from Claude Code's analysis 17 mins ago)" — a completely different agent, reading Claude's work from Harbor memory. Zero re-prompting.</em>
</p>

Under the hood:

<p align="center">
  <img src="assets/download.png" alt="Claude saves analysis via harbor remember → next agent reads it via meta.context" width="900" />
</p>

<p align="center">
  <em>Right: Claude Code saves its analysis via <code>harbor remember</code>. Left: the next agent automatically receives it as <code>meta.context</code>.</em>
</p>

Every agent on your machine shares the same memory layer. When one agent saves an insight, every future call to that connector carries it forward — across sessions, across models, across tools.

---

## Quick Start

```bash
curl -fsSL https://harbor.oseaitic.com/install | bash
```

> Pre-compiled binary for Linux/macOS (amd64/arm64). No runtime required.
> Or from source: `go install github.com/oseaitic/harbor/cmd/harbor@latest`

**Or — paste this into your agent** (Claude Code, Cursor, Gemini, any MCP client):

```
Read github.com/oSEAItic/harbor and set up Harbor for me.
```

The agent reads [AGENTS.md](AGENTS.md), installs Harbor, configures MCP, and starts using it — no manual setup needed.

<details>
<summary>Manual MCP config</summary>

Add to your MCP config (`claude_desktop_config.json`, `.cursor/mcp.json`, etc.):

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

Try a connector:

```bash
harbor install coingecko
harbor get coingecko.prices --param ids=bitcoin --param vs_currencies=usd
```

</details>

### Proxy any existing MCP server

Already using an MCP server? Wrap it with Harbor — one line, no code changes:

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

Harbor re-discovers upstream tools. The agent teaches curation via plain text hints. Every future call is curated automatically.

---

## The Problem

Agent frameworks focus on *orchestration* — which tool to call, when to loop, how to plan. Nobody focuses on what happens **after** the tool call returns.

Three things go wrong:

**Inconsistency.** Every API returns a different shape. The model wastes intelligence decoding format instead of reasoning about content.

**Noise.** A 200-field response might contain 6 fields the agent needs. The rest dilutes attention and inflates cost.

**Amnesia.** Agent A analyzes data, then the session ends. Agent B starts fresh — re-fetching, re-analyzing, re-reasoning. There's no shared memory between agents, sessions, or models.

Harbor solves all three. Three pillars:

### Normalize — One format, any source

Every response becomes `data[]` + `meta{}` + `errors[]`. The agent parses one format, always knows where data came from, and never fails silently.

### Curate — Right density for the task

The agent teaches Harbor what matters by calling `harbor_learn_schema`. Harbor remembers permanently. Four layers of the same data:

| Layer | Content | Use case |
|-------|---------|----------|
| `raw` | Original API response | Full fidelity debugging |
| `normalized` | Structured `data[]` | Standard agent reasoning |
| `compact` | Summary fields only | Token-efficient contexts |
| `summary` | Natural language one-liner | Quick scanning, planning |

### Govern — Only what agents should see

Harbor controls which fields enter the context window based on who's asking. This isn't API-level access control ("can you call this endpoint?") — it's **context-level access control** ("what do you see when you call it?"). An agent that can't see a field can't leak it.

---

## MCP Integration

### Schema learning — the agent teaches, Harbor remembers

Harbor never calls an LLM internally. The connected agent **is** the LLM:

1. **First call** — Harbor returns raw output with a hint:
   `[Harbor: No schema for "list_files". Call harbor_learn_schema to enable curation.]`
2. **Agent teaches** — calls `harbor_learn_schema` with `summary_fields` and `summary_template`
3. **Stored permanently** — all future calls are curated. Every agent on the machine benefits.
4. **Drift detection** — if upstream changes shape, Harbor detects and re-learns.

### Memory & Recall

Every call is cached into 4-layer memory. Agents recall across sessions:

```
harbor_recall(query="bitcoin")              # Search by keyword
harbor_recall(id="mem_abc123")              # Retrieve full content
harbor_remember(connector="coingecko",      # Save analysis conclusions
  note="BTC/ETH correlation high...")
```

Notes saved via `harbor_remember` appear as `meta.context` on every future call to that connector — **this is how agents share memory across sessions and across models.**

### Credential injection

Inject API keys from the OS keychain — secrets never appear in config files:

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

## What it looks like

Raw CoinGecko response — no schema, no source, no memory:

```json
{"bitcoin":{"usd":67234.12,"usd_market_cap":1320984173209,"usd_24h_vol":28394857234,
"usd_24h_change":2.34,"last_updated_at":1707900000},"ethereum":{"usd":3456.78,...}}
```

Through Harbor — structured, attributed, with cross-session context:

```json
{
  "data": [
    { "id": "bitcoin",  "price_usd": 67234.12, "change_24h": 2.34 },
    { "id": "ethereum", "price_usd": 3456.78,  "change_24h": -0.82 }
  ],
  "meta": {
    "source": "coingecko",
    "schema": "crypto.prices.v1",
    "fetched_at": "2026-03-05T12:00:00Z",
    "context": { "summary": "BTC dominance rising. SOL volatile." },
    "recalls": [{ "resource": "coingecko.prices", "age": "1d", "summary": "BTC at $71k..." }]
  },
  "errors": []
}
```

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

Connectors are exec plugins — standalone programs that read arguments and write JSON to stdout. Build in any language.

```typescript
import { parseArgs, output, buildMeta, handleDescribe } from "@oseaitic/harbor-sdk";

const TOOL_SCHEMAS = [{
  type: "function",
  function: {
    name: "myapi_search",
    description: "Search for items",
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

See [CONNECTOR_SPEC.md](docs/CONNECTOR_SPEC.md) for the full interface contract.

---

## Agent-Native Documentation

Harbor ships with **[AGENTS.md](AGENTS.md)** — structured instructions that any AI agent can read and act on. It covers every CLI command, every MCP tool, decision trees for common workflows, and error recovery patterns.

If your tool is agent infrastructure, your documentation should be agent-native too.

---

## Architecture

```
Agent A (Claude Code)     Agent B (Gemini)     Agent C (Cursor)
  \                         |                    /
   \                        |                   /
    +----------- MCP / tool call --------------+
                        |
                        v
                     Harbor
      Normalize --> Curate --> Govern
      Schema Learning <--> Drift Detection
      Memory Store <--> Cross-Session Recall
                        |
                        v
          Connectors + MCP Servers (any source)
                        |
                        v
              APIs / Databases / Services
```

---

## License

Apache 2.0 — see [LICENSE](LICENSE).

<p align="center">
  Built by <a href="https://github.com/oseaitic"><strong>oSEAItic</strong></a><br/>
  <sub>Data flows like the ocean. Harbor is where agents meet that ocean.</sub>
</p>
