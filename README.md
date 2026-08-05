<p align="center">
  <img src="assets/harbor-banner.jpeg" alt="Harbor" width="800" />
</p>

<p align="center">
  <strong>Claude analyzes. Gemini continues. Harbor remembers.</strong><br/>
  Shared context for AI agents — any model, any client, one protocol.
</p>

<p align="center">
  <a href="https://harbor.oseaitic.com">Website</a> &middot;
  <a href="#quick-start">Quick Start</a> &middot;
  <a href="#the-problem">The Problem</a> &middot;
  <a href="#cross-agent-memory">Cross-Agent Memory</a> &middot;
  <a href="#mcp-integration">MCP Integration</a> &middot;
  <a href="#creating-a-connector">Build a Connector</a> &middot;
  <a href="AGENTS.md">Agent Docs</a> &middot;
  <a href="README.CN.md">中文</a>
</p>

<p align="center">
  Works with <strong>Claude Code</strong> &middot; <strong>Gemini CLI</strong> &middot; <strong>Codex</strong> &middot; <strong>Cursor</strong> &middot; <strong>OpenClaw</strong> &middot; <strong>Minimax</strong> &middot; <strong>Any MCP client</strong> &middot; <strong>Any Function Calling LLMs</strong>
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

**Three ways to install — pick whichever fits:**

**1. One-line install:**

```bash
curl -fsSL https://harbor.oseaitic.com/install | bash
```

**2. Agent Skill / Plugin:**

<details>
<summary>Claude Code / Codex / Cursor / Gemini CLI</summary>

```bash
claude plugin marketplace add oSEAItic/harbor && claude plugin install harbor@harbor-marketplace
```

Or point any agent that supports [Agent Skills](https://agentskills.io) at `skills/harbor/SKILL.md`.

</details>

<details>
<summary>OpenClaw</summary>

```bash
# Install Harbor CLI first
go install github.com/oseaitic/harbor/cmd/harbor@latest

# Install the OpenClaw plugin
openclaw plugins install github.com/oSEAItic/harbor/plugins/harbor-openclaw --link
```

The plugin auto-provisions a free cloud account (50 memories) on first use. Adds `harbor_remember` + `harbor_recall` tools to your agents, syncs context to workspace on session start, and captures insights before compaction.

Run `harbor cloud disable` to opt out of cloud sync (local-only mode).

See [plugins/harbor-openclaw/README.md](plugins/harbor-openclaw/README.md) for details.

</details>

**3. Paste into your agent** (zero install — agent does everything):

```
Set up Harbor for this project — instructions at github.com/oSEAItic/harbor/blob/main/AGENTS.md
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

### Auth-proxy any API (no connector needed)

Call any API through Harbor's credential store — your agent never sees raw API keys:

```bash
harbor fetch https://api.github.com/repos/oSEAItic/harbor --auth github-pat
```

Or via MCP tool: `harbor_http(url="...", auth="github-pat")`. Responses go through the full pipeline — memory, schema learning, context injection.

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

### Memory & Recall — Topic-First

Memory is organized by **topic**, not by connector. Agents save what they learned, not where they learned it:

```
harbor_remember(topic="ws-reconnect",         # Save by topic
  note="Root cause: no backoff in ws.go",
  connector="kuse-hive",                       # Optional scope
  refs=["mem_abc123"])                          # Link related notes

harbor_recall(query="websocket")               # Search by keyword
harbor_recall(id="mem_abc123")                 # Retrieve specific note
harbor_remember(topic="billing", note="...")    # Global note (no connector)
```

Notes from the same agent session are automatically grouped by `session_id`. Reference edges between notes form a **knowledge graph** — agents declare which notes are related via `--refs`.

```bash
harbor forget mem_abc123                       # Delete by ID
harbor forget --topic ws-reconnect             # Delete by topic
harbor forget --connector kuse-hive --confirm  # Bulk delete
```

### Local feature worklog

AI makes code generation fast, but delivery time still includes context recovery,
blocking, verification, rework, and scope changes. Harbor can connect agent
sessions to features and measure that full cycle:

```bash
harbor feature start "Shopee reconciliation" \
  --project oseaitic-erp --type integration --size M --budget 2d

harbor feature bind feat_abc123 \
  --session "$HARBOR_SESSION" --source codex --model <model-name> --external-session <conversation-id>

harbor feature block feat_abc123 --note "waiting for staging"
harbor feature resume feat_abc123
harbor feature checkpoint feat_abc123 --commit <git-sha> --note "working checkpoint"
harbor feature verify feat_abc123 --commit <git-sha> --note "acceptance tests pass"
harbor feature ship feat_abc123

harbor worklog report --since 7d
harbor worklog estimate --project oseaitic-erp --type integration --size M
harbor worklog serve                    # http://127.0.0.1:4737
```

Set `HARBOR_MODEL` to attach the active model automatically, or pass `--model`
explicitly. Model identity is stored per session because one feature can span
multiple conversations and models.

`checkpoint` and `verify` can optionally anchor their evidence to a Git commit.
Harbor stores the SHA on the event; it never creates or modifies a commit.

Scope additions are recorded explicitly as `include`, `swap`, `defer`, or
`reject` decisions:

```bash
harbor feature scope feat_abc123 "automatic refunds" --decision defer
```

Worklog data is stored in `~/.harbor/worklog.db` (or `$HARBOR_HOME/worklog.db`).
It is local-only and is never included in Harbor Cloud memory sync.

`harbor worklog serve` opens a read-only, responsive feature calendar. Feature
activity spans its active dates; selecting an entry opens a receipt with cycle,
blocked, verification, session, event, and scope details. Use `--addr` to choose
a different local address.

### Harbor Farm

Use the same Harbor Cloud identity across the CLI, supported agent surfaces,
and presentation clients such as Harbor Studio:

```bash
harbor farm status
harbor farm watch                      # compact view for a second terminal pane
harbor farm plant 0 wheat
harbor farm harvest 0
harbor farm connect A1B2C3D4           # exchange Farm codes with a friend
harbor farm visit A1B2C3D4
harbor farm forage A1B2C3D4 2          # one clipping per visitor and crop
harbor farm telemetry serve           # private OTLP JSON receiver on 127.0.0.1
```

`harbor mcp` also exposes `harbor_farm_open`, an MCP Apps card with six clickable
plots, seed selection, live growth timers, harvesting, Farm codes, neighbor
visits, and foraging. The headless status and mutation tools remain available,
so Claude Code and Codex CLI/IDE/App use the exact same account-owned ledger as
the terminal and Harbor Studio even when a host cannot render the card.

Each connected agent session also grows one session crop. It begins as a
mystery seed, advances from metadata-only lifecycle events, and reveals a
deterministic species and genome when the agent yields or finishes. Farm
receipts contain bucketed usage and generic event counts, never task content.
Friends connect with eight-character Farm codes. A visitor can gather once from
each ready crop, at most three visitors can gather from a crop, and the owner
retains at least 80 percent of the normal harvest.

Agent events are queued under `$HARBOR_HOME/farm-queue` before upload. The event
contract accepts agent/model/status/tool-name/token metadata only; prompts,
outputs, tool arguments, and file content are discarded. Waiting earns no Farm
coins, and the Farm ledger is separate from paid oSEAItic credits.

The repository also includes the cross-client `plugins/harbor-farm` bundle and
marketplace manifests for Codex and Claude Code. Open a new Codex conversation
after installing or updating the plugin, then ask `@Harbor Farm Open my playable
Harbor Farm.` Both integrations launch the same `harbor mcp` process rather than
creating a second Farm or credential store.

### Credential management

Store API keys in Harbor's encrypted keychain — tools never see raw secrets:

```bash
harbor auth tavily                 # Store credential (interactive or browser setup)
harbor auth get tavily             # Retrieve for tool use (stdout)
harbor auth sync                   # Sync cloud → local keychain
harbor auth list                   # List all stored credentials
```

Two modes for tools:

```bash
# Header-based APIs (most REST APIs) — Harbor injects automatically
harbor fetch https://api.github.com/user --auth github-pat

# Body/custom APIs — tool retrieves key, decides injection format
KEY=$(harbor auth get tavily)
curl -X POST api.tavily.com/search -d "{\"api_key\":\"$KEY\",\"query\":\"test\"}"
```

Inject into MCP proxy:

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

Sync credentials, schemas, and memories across machines. **Free tier available — no signup required:**

```bash
harbor cloud enable                   # Auto-provision free account (50 memories, zero config)
harbor cloud status                   # Check connection
harbor cloud disable                  # Opt out (local-only mode)
```

Or sign in for unlimited:

```bash
harbor login                          # Sign in with email
harbor auth my-connector              # Store API key (client-side encrypted)
harbor auth sync my-connector         # Pull credentials on another machine
harbor publish my-connector           # Publish private connector
```

Credentials are encrypted client-side with AES-256-GCM. The server stores only ciphertext.

---

## OpenClaw Plugin

Harbor ships a native [OpenClaw plugin](plugins/harbor-openclaw/) that enhances OpenClaw's memory system:

| Feature | OpenClaw native | + Harbor plugin |
|---------|----------------|-----------------|
| Memory organization | Raw text dump to MEMORY.md | Topic-first, structured, deduplicated |
| Cross-device | Local files only | Cloud sync via Harbor Cloud (free tier: 50 memories) |
| Cross-agent | Per-agent workspace | Shared memory pool across agents |
| Knowledge graph | None | Ref edges between related insights |
| Context loss | Lost on compaction | Auto-captured before compaction |

### What the plugin does

- **`harbor_remember` + `harbor_recall` tools** — registered as native OpenClaw agent tools
- **Session start hook** — writes Harbor's curated context to `memory/harbor-context.md`, auto-indexed by OpenClaw's file watcher
- **Pre-compaction hook** — captures session context via `harbor remember` before it's lost to compaction
- **Auto-provision** — creates a free cloud account on first use (opt out with `harbor cloud disable`)

### Install

```bash
go install github.com/oseaitic/harbor/cmd/harbor@latest
openclaw plugins install github.com/oSEAItic/harbor/plugins/harbor-openclaw --link
```

### Config

In OpenClaw config (`plugins.entries.harbor.config`):

```json
{
  "autoSync": true,
  "autoCapture": true,
  "contextFile": "memory/harbor-context.md"
}
```

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
