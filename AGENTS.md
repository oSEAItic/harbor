# Harbor — Agent Instructions

> This file is designed for AI agents (Claude Code, Codex, Gemini, Kuse, Manus etc.) that interact with Harbor. It provides structured, LLM-optimized instructions for every capability.
>
> Website: https://harbor.oseaitic.com | GitHub: https://github.com/oSEAItic/harbor

## What is Harbor

Harbor is context infrastructure for AI agents. It sits between you and external data sources (APIs, databases, MCP servers) and ensures every response is:

- **Normalized**: consistent `data[]` + `meta{}` + `errors[]` envelope
- **Curated**: only relevant fields, at the density you choose
- **Governed**: field-level access control based on caller identity
- **Remembered**: cached across sessions with 4-layer recall

You interact with Harbor in two ways: **CLI commands** (via shell) or **MCP tools** (if Harbor is your MCP server).

---

## CLI Commands

### Fetching data

```bash
# Standard fetch — returns curated, context-compiled output
harbor get <connector.resource> [--param key=value]...

# Examples
harbor get coingecko.prices --param ids=bitcoin --param vs_currencies=usd
harbor get yahoo.quote --param symbol=AAPL

# Shorthand: if a resource has one required param, pass it positionally
harbor get coingecko.prices bitcoin

# Full uncompiled output (for debugging)
harbor get <connector.resource> --full

# Raw upstream response alongside normalized data
harbor raw <connector.resource> --param key=value
```

### Auth-proxy HTTP fetching

Call any API through Harbor's credential store — no connector needed.

```bash
# GET with auth
harbor fetch https://api.github.com/repos/oSEAItic/harbor --auth github-pat

# POST with body
harbor fetch https://api.example.com/data -X POST -d '{"key":"val"}' --auth my-api

# Custom auth header (e.g. X-API-Key instead of Authorization: Bearer)
harbor fetch https://api.coingecko.com/api/v3/pro/coins --auth coingecko-pro --auth-header "x-cg-pro-api-key"
```

Responses go through the full pipeline: memory, schema learning, context injection.

### Memory and recall

```bash
# Browse recent memories
harbor recall --list

# Search memories by keyword
harbor recall --search "bitcoin"

# Retrieve a specific memory at a given density layer
harbor recall <connector.resource> --layer compact
# Layers: raw | normalized | compact | summary

# Save analysis conclusions (persists across sessions and devices)
harbor remember <connector> "Your analysis summary here"
harbor remember <connector> "summary" --refs mem_abc123,mem_def456
```

### Feature delivery worklog

Track feature cycle time, agent sessions, blocking, verification, and scope changes
in a local SQLite database. Worklog data stays local and is not part of Harbor
Cloud memory sync.

```bash
harbor feature start "Shopee reconciliation" --project oseaitic-erp --type integration --size M --budget 2d
harbor feature bind feat_abc123 --session "$HARBOR_SESSION" --source codex --model <model-name> --external-session <conversation-id>
harbor feature block feat_abc123 --note "waiting for staging"
harbor feature resume feat_abc123
harbor feature verify feat_abc123 --note "acceptance tests pass"
harbor feature ship feat_abc123

# Make scope changes explicit instead of silently expanding the feature.
harbor feature scope feat_abc123 "support automatic refunds" --decision defer

harbor worklog report --since 7d
harbor worklog estimate --project oseaitic-erp --type integration --size M
harbor worklog serve                    # http://127.0.0.1:4737
```

Set `HARBOR_MODEL` to record the active model automatically when binding a
session, or pass `--model` explicitly when multiple models share one client.

Only mark a feature `verify` after checking its acceptance criteria. A feature
must be verified before Harbor allows it to be shipped.

### Harbor Farm

```bash
harbor farm status
harbor farm plant 0 wheat
harbor farm harvest 0
harbor farm telemetry serve
```

Farm uses the existing Harbor Cloud account. Agent telemetry is opt-in and
metadata-only; never add prompts, responses, tool arguments, or file content to
Farm events. Waiting states do not earn Farm coins.

### Connector management

```bash
# List installed + available connectors
harbor list

# Install a connector from the registry
harbor install <connector-name>

# Uninstall a connector
harbor uninstall <connector-name>

# Export tool schemas for function-calling integration
harbor tools export                     # Harbor format
harbor tools export --format openai     # OpenAI function-calling format
harbor tools export --format mcp        # MCP format
```

### Authentication

```bash
# Store API key for a connector in OS keychain
harbor auth <connector-name>

# Check credential status (local + cloud)
harbor auth status

# Login to Harbor Cloud (syncs credentials, schemas, memories)
harbor login

# Sync credentials from cloud to local keychain
harbor auth sync <connector-name>
```

### Diagnostics

```bash
# Health check — connector status, config, environment
harbor doctor --json

# Full agent bootstrap — capabilities, examples, error recovery hints
harbor agent bootstrap --json

# Schema history for a connector
harbor schema history <connector-name>

# Compression metrics
harbor metrics report
```

---

## MCP Tools

When Harbor runs as your MCP server (`harbor mcp` or `harbor proxy ...`), these tools are available:

### Connector tools

Every installed connector resource becomes an MCP tool. Names follow the pattern `<connector>_<resource>`. Parameters match the connector's schema.

Example: if `coingecko` is installed with a `prices` resource, the tool `coingecko_prices` accepts `ids` and `vs_currencies` parameters.

### harbor_learn_schema

Teach Harbor which fields matter for a given tool. **You MUST call this when you see a `[Harbor:` hint in a tool response.**

```json
{
  "tool_name": "list_files",
  "summary_fields": ["name", "size", "type"],
  "summary_template": "{name} ({type}, {size} bytes)"
}
```

Guidelines:
- Pick the 3-6 most useful fields for reasoning about this data
- Write a one-line template with `{field}` placeholders
- The schema is stored permanently — all future calls are curated
- You can call this again to update the schema if your task needs different fields
- After learning, Harbor returns re-compressed data from cache (no extra API call needed)

### harbor_recall

Search and retrieve data from cross-session memory.

```json
// Browse recent memories
{}

// Search by keyword
{ "query": "bitcoin" }

// Filter by connector
{ "connector": "coingecko" }

// Retrieve specific memory at full fidelity
{ "id": "mem_abc123", "layer": "raw" }

// Time-scoped search
{ "query": "price", "since": "1h" }
```

Parameters:
- `query` (string, optional): keyword search across connector names, resources, params, summaries
- `connector` (string, optional): filter to a specific connector
- `id` (string, optional): retrieve a specific memory by ID
- `layer` (string, optional): `raw` | `normalized` | `compact` | `summary` (default: `compact`)
- `since` (string, optional): time filter, e.g. `"30m"`, `"2h"`, `"1d"`

### harbor_http

Auth-proxy HTTP — call any API through Harbor's credential store. Your agent never sees raw API keys.

```json
{
  "url": "https://api.github.com/repos/oSEAItic/harbor",
  "auth": "github-pat",
  "auth_header": "Authorization: Bearer"
}
```

Parameters:
- `url` (string, required): full URL to fetch
- `method` (string, optional): HTTP method, default GET
- `body` (string, optional): request body for POST/PUT
- `auth` (string, optional): credential name in Harbor keychain (set via `harbor auth <name>`)
- `auth_header` (string, optional): how to inject credential (default: `Authorization: Bearer`). For APIs that use custom headers, pass the header name (e.g. `X-API-Key`)

The response goes through Harbor's full pipeline: memory, schema learning, context injection.

### harbor_remember

Persist your analysis conclusions for future sessions.

```json
{
  "connector": "coingecko",
  "note": "BTC/ETH correlation high (r=0.94). SOL volatile (+40% weekly). Market bullish but elevated risk.",
  "author": "Claude Code",
  "refs": ["mem_abc123"]
}
```

**Always pass your name in `author`** — e.g. `"Claude Code"`, `"Gemini"`, `"Cursor"`, `"GPT-4"`. This lets the next agent (which may be a different model) know who produced the analysis. If omitted, Harbor attempts auto-detection from environment variables.

Use `refs` to link to memory IDs your analysis builds upon — this creates knowledge graph edges. When any agent recalls a referenced memory, linked memories are automatically surfaced as neighbors.

Your note appears as `meta.context` on every future call to this connector — for you and every other agent. Compose a comprehensive summary covering:
1. What you analyzed and why
2. Patterns or anomalies found
3. Conclusions reached
4. Recommendations made

### Harbor Farm tools

The Harbor MCP server exposes the same account-owned Farm ledger as the CLI and
Harbor Studio:

- `harbor_farm_status` — read Farm profile, plots, usage, and active sessions
- `harbor_farm_plant` — plant `wheat`, `carrot`, or `tomato` in plot `0`-`5`
- `harbor_farm_harvest` — harvest a ready plot

Use `harbor mcp` from Claude Code, Codex CLI, the Codex IDE extension, or the
Codex app. Farm telemetry remains opt-in and metadata-only; MCP tools do not
grant access to prompts, outputs, tool arguments, file content, or provider keys.

---

## Decision Trees

### When you receive data from Harbor

```
Is there a [Harbor:] hint at the end?
  YES --> Call harbor_learn_schema immediately
         Pick 3-6 key fields, write a template
  NO  --> Data is already curated. Proceed with reasoning.

Does meta.context exist?
  YES --> Read it first. It contains your (or another agent's)
         previous analysis of this data source.
  NO  --> After your analysis, call harbor_remember to save
         conclusions for future sessions.

Are there meta.recalls entries?
  YES --> Related past data is available. Use harbor_recall
         with the memory ID if you need full content.
  NO  --> This is the first interaction with this connector.
```

### When you need external data

```
Do you need real-time data from an API?
  YES --> Use harbor get or the connector's MCP tool
  NO  --> Check harbor_recall first (cached data may suffice)

Is the data too verbose / too many fields?
  YES --> Call harbor_learn_schema to curate it
  NO  --> Proceed with the data as-is

Do you need to remember your findings?
  YES --> Call harbor_remember with a comprehensive summary
  NO  --> Data is ephemeral; no action needed
```

### When something goes wrong

```
"connector not found"
  --> Run: harbor install <connector-name>

"no schema for <tool>"
  --> This is expected on first use. Call harbor_learn_schema.

"memory not found"
  --> The memory may have expired. Re-fetch with harbor get.

"auth required" / 401
  --> Run: harbor auth <connector-name>
  --> For cloud sync: harbor login && harbor auth sync <connector-name>

Tool returns empty data[]
  --> Check params. Try harbor doctor --json for diagnostics.
  --> Try harbor raw <connector.resource> to see the upstream response.
```

---

## Response Envelope

Every Harbor response follows this structure:

```json
{
  "data": [],
  "meta": {
    "source": "connector-name",
    "schema": "domain.resource.v1",
    "fetched_at": "ISO-8601 timestamp",
    "request_id": "UUID",
    "context": { "summary": "...", "age": "...", "author": "Claude Code" },
    "recalls": [{ "id": "...", "resource": "...", "age": "...", "summary": "..." }],
    "memory_hint": "Call harbor_remember after analysis"
  },
  "raw": null,
  "errors": [{ "code": "...", "message": "..." }]
}
```

Key fields:
- `meta.source` — cite this when attributing data
- `meta.schema` — tells you what fields to expect
- `meta.fetched_at` — check staleness before reasoning
- `meta.context` — previous analysis (read this first)
- `meta.recalls` — related past data (use `harbor_recall(id=...)` for full content)
- `errors` — never empty-check `data` alone; always check `errors` too
