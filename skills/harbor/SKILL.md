---
name: harbor
description: >-
  Give your AI agent persistent cross-agent memory, credential isolation,
  and schema learning via Harbor. Use when: the user wants agents to remember
  context across sessions, share knowledge between different AI models,
  connect to external APIs without exposing credentials, or reduce noisy
  API responses to only relevant fields. Also use when the user mentions
  Harbor, MCP middleware, agent memory, cross-agent context, or schema learning.
license: Apache-2.0
compatibility: Requires macOS or Linux (amd64/arm64). Works with any MCP-compatible agent.
metadata:
  author: oseaitic
  version: "0.4.0"
  website: https://harbor.oseaitic.com
  repository: https://github.com/oSEAItic/harbor
---

# Harbor — Persistent Memory for AI Agents

You now have access to Harbor, agent middleware that gives you persistent memory across sessions, credential isolation, and schema learning.

## Quick setup

If Harbor is not installed, run the setup script:

```bash
bash scripts/setup.sh
```

This installs the Harbor CLI and configures it as an MCP server for the current agent.

If Harbor is already installed and configured as your MCP server, skip to **Using Harbor**.

## Using Harbor

### You have these MCP tools available

| Tool | What it does |
|------|-------------|
| `harbor_remember` | Save context that persists across sessions and agents |
| `harbor_recall` | Search and retrieve past context |
| `harbor_http` | Auth-proxy HTTP — call any API without exposing credentials |
| `harbor_learn_schema` | Teach Harbor which API fields matter — reduces noise permanently |
| `harbor_get` | Query a connector (API) with auto-recall |
| Connector tools | Each installed connector exposes tools like `coingecko_prices` |

### When you receive data from Harbor

1. **Check `meta.context`** — if present, another agent (or you in a past session) left analysis. Read it before reasoning.
2. **Check for `[Harbor:]` hints** — if you see one, call `harbor_learn_schema` immediately with 3-6 key fields.
3. **After analysis, save your findings** — call `harbor_remember` with a comprehensive summary.

### Auth-proxy HTTP (harbor_http)

Call any API through Harbor's credential store — your agent never sees raw API keys.

```json
{
  "url": "https://api.github.com/repos/oSEAItic/harbor",
  "auth": "github-pat",
  "auth_header": "Authorization: Bearer"
}
```

- `auth` — credential name in Harbor's keychain (set via `harbor auth <name>`)
- `auth_header` — how to inject the credential (default: `Authorization: Bearer`)
- Responses go through Harbor's full pipeline: memory, schema learning, context injection

### Saving context (harbor_remember) — Topic-First

Notes are organized by **topic**, not connector. Connector is optional scope:

```json
{
  "topic": "market-trends",
  "note": "BTC dominance rising to 58%. SOL underperforming vs ETH.",
  "connector": "coingecko",
  "author": "Claude Code",
  "refs": ["mem_abc123"]
}
```

Rules:
- **Always pass your name** in `author` (e.g. "Claude Code", "Gemini CLI", "Cursor")
- Write a comprehensive summary: what you analyzed, patterns found, conclusions, recommendations
- Use `refs` to link to memory IDs your analysis builds upon — creates a knowledge graph
- This note appears in `meta.context` on every future call to this connector — for you and every other agent

### Teaching schemas (harbor_learn_schema)

When an API returns too many fields:

```json
{
  "tool_name": "coingecko_prices",
  "summary_fields": ["price", "change_24h", "market_cap"],
  "summary_template": "{id}: ${price} ({change_24h}%) mcap ${market_cap}"
}
```

Pick 3-6 fields. This is permanent — all future calls by any agent get curated data.

### Recalling past context (harbor_recall)

```json
{ "query": "bitcoin" }
{ "connector": "coingecko" }
{ "id": "mem_abc123", "layer": "raw" }
{ "since": "1h" }
```

Usually you don't need to call this — Harbor auto-injects relevant context via `meta.context`.

## Decision tree

```
Received data from Harbor?
├── Has [Harbor:] hint? → Call harbor_learn_schema (pick 3-6 fields)
├── Has meta.context? → Read it first, it's previous analysis
├── No meta.context? → After your analysis, call harbor_remember
└── Has errors[]? → Check error code, see troubleshooting below
```

## Response format

Every Harbor response follows this envelope:

```json
{
  "data": [{ "id": "bitcoin", "price": 67234 }],
  "meta": {
    "source": "coingecko",
    "context": { "summary": "...", "author": "Claude Code", "age": "33m" },
    "recalls": [{ "id": "mem_abc", "summary": "..." }]
  },
  "errors": []
}
```

- `meta.context` — auto-injected previous analysis (read first)
- `meta.recalls` — related past data (use `harbor_recall` with ID for full content)
- `meta.source` — cite this when attributing data

## Troubleshooting

| Error | Fix |
|-------|-----|
| "connector not found" | Run `harbor install <name>` |
| "auth required" / 401 | Run `harbor auth <connector>` |
| "no schema for tool" | Expected on first use — call `harbor_learn_schema` |
| Empty `data[]` | Check params. Run `harbor doctor --json` for diagnostics |

## CLI commands (via shell)

If MCP tools aren't available, use the CLI directly:

```bash
harbor get coingecko.prices --param ids=bitcoin
harbor fetch https://api.github.com/repos/oSEAItic/harbor --auth github-pat
harbor remember coingecko "BTC dominance rising to 58%" --refs mem_abc123
harbor recall --list
harbor recall --search "bitcoin"
harbor install <connector>
harbor auth <connector>
harbor doctor --json
```

For full CLI and MCP tool reference, see [references/tools.md](references/tools.md).
