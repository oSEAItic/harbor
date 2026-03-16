# Harbor Tool Reference

## MCP Tools

### harbor_remember

Save analysis context that persists across sessions and agents.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `connector` | string | Yes | Connector name (e.g. "coingecko") |
| `note` | string | Yes | Your analysis summary |
| `author` | string | No | Your agent name (e.g. "Claude Code"). Auto-detected if omitted. |
| `refs` | string[] | No | Memory IDs this note references (creates knowledge graph edges) |

**Behavior:** The note is stored and automatically injected as `meta.context` in every future call to this connector — by any agent, any model, any session. Referenced memories are linked via graph edges and surfaced as neighbors on recall.

---

### harbor_http

Auth-proxy HTTP fetching — call any API through Harbor's credential store. Your agent never sees raw API keys.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `url` | string | Yes | Full URL to fetch |
| `method` | string | No | HTTP method (default: GET) |
| `body` | string | No | Request body (for POST/PUT) |
| `auth` | string | No | Credential name in Harbor keychain |
| `auth_header` | string | No | How to inject credential (default: "Authorization: Bearer") |

**Behavior:** Harbor retrieves the credential from keychain, injects it into the request header, executes the HTTP call, and passes the response through the full pipeline (memory, schema learning, context injection). The agent never touches the raw API key.

---

### harbor_recall

Search and retrieve past context.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `query` | string | No | Keyword search across memories |
| `connector` | string | No | Filter to a specific connector |
| `id` | string | No | Retrieve a specific memory by ID |
| `layer` | string | No | Density layer: `raw`, `normalized`, `compact` (default), `summary` |
| `since` | string | No | Time filter: `"30m"`, `"2h"`, `"1d"` |

**Note:** Usually not needed — Harbor auto-injects relevant context via `meta.context`.

---

### harbor_learn_schema

Teach Harbor which fields matter for a given tool. Permanent — affects all future calls by any agent.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `tool_name` | string | Yes | The tool to learn schema for |
| `summary_fields` | string[] | Yes | 3-6 most useful fields |
| `summary_template` | string | Yes | One-line template with `{field}` placeholders |

**When to call:** When you see a `[Harbor:]` hint in a tool response, or when API data is too verbose.

---

### harbor_set_density

Set response density for a connector.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `connector` | string | Yes | Connector name |
| `layer` | string | Yes | `raw`, `normalized`, `compact`, or `summary` |

**Layers:**
- `raw` — full upstream response
- `normalized` — `data[]` + `meta{}` + `errors[]` envelope, all fields
- `compact` — only learned schema fields (default after learning)
- `summary` — one-line per data item using `summary_template`

---

### harbor_status

Check Harbor status, connected connectors, and memory stats.

**Parameters:** None

---

## CLI Commands

### Data

```bash
harbor get <connector.resource> [--param key=value]...
harbor get coingecko.prices --param ids=bitcoin
harbor raw <connector.resource> --param key=value    # full upstream response
```

### Auth-proxy HTTP

```bash
harbor fetch <url> --auth <credential-name>              # GET with auth
harbor fetch <url> --auth <name> -X POST -d '{"key":"val"}'  # POST
harbor fetch <url> --auth <name> --auth-header "X-API-Key"   # custom header
```

### Memory

```bash
harbor remember <connector> "Your analysis summary"
harbor remember <connector> "note" --refs mem_abc123,mem_def456  # with refs
harbor recall --list                    # browse all memories
harbor recall --search "keyword"        # search memories
harbor recall <connector.resource>      # retrieve specific memory
harbor recall <connector.resource> --layer raw   # full fidelity
```

### Connectors

```bash
harbor list                             # installed + available
harbor install <connector>              # install from registry
harbor uninstall <connector>            # remove connector
harbor tools export                     # export tool schemas
harbor tools export --format openai     # OpenAI function-calling format
```

### Auth

```bash
harbor auth <connector>                 # store API key in OS keychain
harbor auth status                      # check credential status
harbor login                            # login to Harbor Cloud
harbor auth sync <connector>            # sync credentials from cloud
```

### Diagnostics

```bash
harbor doctor --json                    # health check
harbor agent bootstrap --json           # full agent bootstrap
harbor schema history <connector>       # schema change log
harbor whoami                           # cloud login status
harbor sync                             # explicit cloud sync
```

## Response Envelope

Every response:

```json
{
  "data": [],
  "meta": {
    "source": "connector-name",
    "schema": "domain.resource.v1",
    "fetched_at": "ISO-8601",
    "context": { "summary": "...", "author": "...", "age": "..." },
    "recalls": [{ "id": "...", "summary": "...", "age": "..." }],
    "memory_hint": "Call harbor_remember after analysis"
  },
  "errors": [{ "code": "...", "message": "..." }]
}
```
