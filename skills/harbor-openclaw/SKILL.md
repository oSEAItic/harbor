---
name: harbor
description: >-
  Persistent cross-session memory, credential isolation, and schema learning
  for your OpenClaw agent. Use Harbor when you need to: remember context across
  sessions, call APIs without exposing credentials, reduce noisy API responses
  to relevant fields, or share knowledge between agents. Also use when the user
  mentions Harbor, agent memory, credential isolation, or schema learning.
metadata:
  openclaw:
    requires:
      bins:
        - harbor
    install:
      - kind: go
        package: github.com/oseaitic/harbor@latest
        bins: [harbor]
    emoji: "\u2693"
    homepage: https://harbor.oseaitic.com
    os: ["macos", "linux"]
---

# Harbor — Persistent Memory & Credential Isolation for OpenClaw

You now have access to Harbor, agent infrastructure that gives you persistent memory across sessions, credential isolation (your skills never see raw API keys), and schema learning.

## Network & data disclosure

- **Local-first**: all credential storage and memory are local by default (`~/.harbor/`)
- **Optional cloud sync**: if enabled (`harbor cloud enable`), memory summaries (not credentials) sync to Harbor Cloud (`harbor-cloud.oseaitic.com`)
- **Credential setup page**: when a credential is missing, Harbor may open a browser to `harbor.oseaitic.com/setup` for guided key entry. The key is encrypted server-side before storage.
- **No telemetry**: Harbor CLI sends no analytics or tracking data

## Setup

If `harbor` is not installed:

```bash
go install github.com/oseaitic/harbor/cmd/harbor@latest
```

Then configure Harbor as an MCP server for OpenClaw (add to `openclaw.json`):

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

If Harbor is already installed, skip to **Using Harbor**.

## Using Harbor

### Available tools

| Tool | What it does |
|------|-------------|
| `harbor_http` | Auth-proxy HTTP — call any API without exposing credentials |
| `harbor_remember` | Save context that persists across sessions |
| `harbor_recall` | Search and retrieve past context |
| `harbor_learn_schema` | Teach Harbor which API fields matter — reduces noise permanently |

### Credential isolation (harbor_http)

**This is the key security feature for OpenClaw skills.** Instead of storing API keys in environment variables where any skill can read them, Harbor holds credentials in its encrypted keychain. Your agent calls APIs through Harbor — never touching raw keys.

```bash
# Store a credential (one-time setup)
harbor auth github-pat
# Agent prompt: "Enter API key for github-pat:"

# Call API through Harbor — agent never sees the key
harbor fetch https://api.github.com/repos/oSEAItic/harbor --auth github-pat
```

Or via MCP tool:

```json
{
  "url": "https://api.github.com/repos/oSEAItic/harbor",
  "auth": "github-pat",
  "auth_header": "Authorization: Bearer"
}
```

- `auth` — credential name in Harbor's keychain
- `auth_header` — how to inject the credential (default: `Authorization: Bearer`). For custom headers: `"x-cg-pro-api-key"`, `"X-API-Key"`, etc.
- Responses go through the full pipeline: memory, schema learning, context injection

### Saving context (harbor_remember) — Topic-First

Notes are organized by **topic**, not connector. Connector is optional scope:

```json
{
  "topic": "github-activity",
  "note": "Harbor repo has 247 stars, 12 open issues. Active development on auth-proxy and memory features.",
  "connector": "github",
  "author": "OpenClaw Agent",
  "refs": ["mem_abc123"]
}
```

Rules:
- **Use descriptive topic keys** — e.g. `"ws-reconnect"`, `"billing-logic"`, `"market-trends"`
- **Always pass `"OpenClaw Agent"` as author** — so other agents know who produced the analysis
- Write comprehensive summaries: what you analyzed, patterns found, conclusions
- Use `refs` to link to memory IDs your analysis builds upon — creates a knowledge graph
- Notes from the same session are auto-grouped by `session_id`

### Recalling past context (harbor_recall)

```json
{ "query": "github" }
{ "connector": "coingecko" }
{ "id": "mem_abc123" }
```

Usually you don't need this — Harbor auto-injects relevant context.

### Teaching schemas (harbor_learn_schema)

When an API returns too many fields:

```json
{
  "tool_name": "github_repos",
  "summary_fields": ["name", "stars", "language", "updated_at"],
  "summary_template": "{name} ({language}) - {stars} stars, updated {updated_at}"
}
```

Pick 3-6 fields. This is permanent — all future calls are curated.

## Decision tree

```
Received data from Harbor?
├── Has meta.context? → Read it first, it's previous analysis
├── Has [Harbor:] hint? → Call harbor_learn_schema (pick 3-6 fields)
├── No meta.context? → After your analysis, call harbor_remember
└── Has errors[]? → Check error code, see troubleshooting below
```

## CLI fallback

If MCP tools aren't available, use the CLI:

```bash
harbor fetch <url> --auth <credential-name>              # Auth-proxy HTTP
harbor get <connector.resource> --param key=value         # Connector fetch
harbor remember <topic> "Your analysis summary"             # Save context
harbor remember --connector <name> <topic> "summary"       # Scoped to connector
harbor forget mem_xxx                                      # Delete memory
harbor recall --search "keyword"                          # Search memory
harbor auth <name>                                        # Store credential
harbor auth get <name>                                    # Retrieve credential (stdout)
harbor auth sync                                          # Sync cloud → local
harbor doctor --json                                      # Diagnostics
```

## Troubleshooting

| Error | Fix |
|-------|-----|
| `harbor: command not found` | Run `go install github.com/oseaitic/harbor/cmd/harbor@latest` |
| "auth required" / 401 | Run `harbor auth <credential-name>` to store the API key |
| Empty `data[]` | Check params. Run `harbor doctor --json` for diagnostics |

## OpenClaw Plugin (recommended)

For deeper integration, install the Harbor OpenClaw plugin:

```bash
openclaw plugins install github.com/oSEAItic/harbor/plugins/harbor-openclaw --link
```

The plugin:
- Registers `harbor_remember` + `harbor_recall` as native OpenClaw agent tools
- Syncs Harbor context to your workspace on session start (auto-indexed by OpenClaw)
- Captures context before compaction (prevents memory loss)
- Auto-provisions a free cloud account (50 memories, opt out with `harbor cloud disable`)

## Build Tools with Harbor (for skill/plugin authors)

Use `harbor fetch` as your HTTP layer — get credential isolation, memory, and schema learning for free. Your tool code never touches raw API keys.

Harbor provides two ways to use credentials in tools:

| Mode | Use when | Command |
|------|----------|---------|
| `harbor auth get` | API key goes in body, query param, or custom format | Tool gets raw key, decides injection |
| `harbor fetch --auth` | API key goes in HTTP header (most REST APIs) | Harbor injects automatically |

### Example: Tavily search (key in body — use `harbor auth get`)

```typescript
export const tavily_search = {
  name: "tavily_search",
  description: "Web search via Tavily (credential-isolated through Harbor)",
  parameters: {
    type: "object",
    required: ["query"],
    properties: { query: { type: "string" } },
  },
  async execute({ query }: { query: string }) {
    const { execSync } = require("node:child_process");
    const key = execSync("harbor auth get tavily", { encoding: "utf-8" });
    const res = await fetch("https://api.tavily.com/search", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ api_key: key, query, max_results: 5 }),
    });
    return res.json();
  },
};
```

### Example: GitHub API (key in header — use `harbor fetch`)

```typescript
export const github_repos = {
  name: "github_repos",
  description: "List GitHub repos (credential-isolated)",
  parameters: { type: "object", properties: {} },
  async execute() {
    const { execSync } = require("node:child_process");
    return JSON.parse(execSync(
      "harbor fetch https://api.github.com/user/repos --auth github-pat",
      { encoding: "utf-8" },
    ));
  },
};
```

### Example: Stripe (key in header, custom format)

```typescript
export const stripe_balance = {
  name: "stripe_balance",
  description: "Check Stripe balance (credential-isolated)",
  parameters: { type: "object", properties: {} },
  async execute() {
    const { execSync } = require("node:child_process");
    const key = execSync("harbor auth get stripe", { encoding: "utf-8" });
    const res = await fetch("https://api.stripe.com/v1/balance", {
      headers: { Authorization: `Bearer ${key}` },
    });
    return res.json();
  },
};
```

User setup (one-time): `harbor auth <name>` → paste key → done.

### Why use Harbor for credentials?

| | Harbor | Raw env vars |
|---|---|---|
| **API key** | Encrypted keychain, never in code | In env var, any skill can read |
| **Access** | `harbor auth get` or `harbor fetch --auth` | `process.env.XXX` |
| **Security** | Per-credential isolation | All skills see all vars |
| **Setup** | `harbor auth <name>` or browser setup page | Edit .env, restart |
| **Cross-device** | Cloud sync | Manual copy |

### Pattern for any API

```bash
# 1. User stores credential (once)
harbor auth <name>

# 2. Tool retrieves key (any injection format)
harbor auth get <name>              # raw key to stdout

# 3. Or let Harbor inject into header automatically
harbor fetch <url> --auth <name>    # header-based APIs
```

## Why Harbor for OpenClaw

OpenClaw skills currently access API keys via environment variables — any installed skill can read any credential. Harbor fixes this:

1. **Credential isolation** — API keys live in Harbor's encrypted keychain, not env vars. Skills call `harbor fetch` and never see raw keys.
2. **Cross-session memory** — Your analysis persists. Next time you (or another skill) access the same data source, previous conclusions are auto-injected.
3. **Schema learning** — APIs return 47 fields, you use 3. Harbor learns and curates permanently.
4. **Tool platform** — Any developer can build credential-isolated tools with `harbor fetch`. One pattern, any API.
