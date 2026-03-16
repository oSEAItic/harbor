---
name: harbor
description: >-
  Persistent cross-session memory, credential isolation, and schema learning
  for your OpenClaw agent. Use Harbor when you need to: remember context across
  sessions, call APIs without exposing credentials, reduce noisy API responses
  to relevant fields, or share knowledge between agents. Also use when the user
  mentions Harbor, agent memory, credential isolation, or schema learning.
version: 0.3.15
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

## Setup

If `harbor` is not installed:

```bash
curl -fsSL https://harbor.oseaitic.com/install | bash
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

### Saving context (harbor_remember)

After analyzing data, save your findings so they persist across sessions:

```json
{
  "connector": "github",
  "note": "Harbor repo has 247 stars, 12 open issues. Main contributor is oseaitic. Active development on auth-proxy and memory features.",
  "author": "OpenClaw",
  "refs": ["mem_abc123"]
}
```

Rules:
- **Always pass `"OpenClaw"` as author** — so other agents know who produced the analysis
- Write comprehensive summaries: what you analyzed, patterns found, conclusions
- Use `refs` to link to memory IDs your analysis builds upon — creates a knowledge graph
- This note appears as context on every future call to this connector

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
harbor remember <connector> "Your analysis summary"       # Save context
harbor recall --search "keyword"                          # Search memory
harbor auth <name>                                        # Store credential
harbor doctor --json                                      # Diagnostics
```

## Troubleshooting

| Error | Fix |
|-------|-----|
| `harbor: command not found` | Run `curl -fsSL https://harbor.oseaitic.com/install \| bash` |
| "auth required" / 401 | Run `harbor auth <credential-name>` to store the API key |
| Empty `data[]` | Check params. Run `harbor doctor --json` for diagnostics |

## Why Harbor for OpenClaw

OpenClaw skills currently access API keys via environment variables — any installed skill can read any credential. Harbor fixes this:

1. **Credential isolation** — API keys live in Harbor's encrypted keychain, not env vars. Skills call `harbor_http` and never see raw keys.
2. **Cross-session memory** — Your analysis persists. Next time you (or another skill) access the same data source, previous conclusions are auto-injected.
3. **Schema learning** — APIs return 47 fields, you use 3. Harbor learns and curates permanently.
