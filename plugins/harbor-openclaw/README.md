# Harbor Plugin for OpenClaw

Adds **curated, cross-session, cross-device memory** to OpenClaw agents.

Works alongside `memory-core` — does NOT replace OpenClaw's native memory.

## What it adds

| Feature | OpenClaw native | + Harbor plugin |
|---------|----------------|-----------------|
| Memory storage | Raw text dump to MEMORY.md | Topic-first, structured, deduplicated |
| Cross-device | Local files only | Cloud sync via Harbor Cloud |
| Cross-agent | Per-agent workspace | Shared memory pool |
| Knowledge graph | None | Ref edges between related insights |
| Memory freshness | No TTL | TTL + decay |

## Install

```bash
# Prerequisite: Harbor CLI
go install github.com/oseaitic/harbor/cmd/harbor@latest

# Install plugin
openclaw plugins install ./plugins/harbor-openclaw --link
```

## Agent tools

### `harbor_remember`

Save an analysis conclusion by topic:

```
topic: "ws-reconnect"
note: "Root cause: gateway ws.go line 127 has no backoff"
connector: "kuse-hive"  (optional)
refs: ["mem_abc123"]     (optional)
```

### `harbor_recall`

Search prior insights:

```
query: "websocket"       → keyword search
id: "mem_abc123"         → specific memory
list: true               → recent memories
```

## Auto behaviors

- **Session start**: writes `memory/harbor-context.md` with recent Harbor memories (OpenClaw indexes it automatically)
- **Before compaction**: captures session context before it's lost

## Config

In OpenClaw config (`plugins.entries.harbor.config`):

```json
{
  "autoSync": true,
  "autoCapture": true,
  "contextFile": "memory/harbor-context.md"
}
```
