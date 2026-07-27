---
description: Open the shared Harbor Farm, inspect the current session crop, or play with neighbors
argument-hint: Optional action such as status, plant, harvest, visit, or forage
allowed-tools: mcp__harbor__harbor_farm_status, mcp__harbor__harbor_farm_plant, mcp__harbor__harbor_farm_harvest, mcp__harbor__harbor_farm_connect, mcp__harbor__harbor_farm_visit, mcp__harbor__harbor_farm_forage
---

# Harbor Farm

Use the shared Harbor Farm as a waiting-time activity without changing or delaying the user's coding task.

Requested action: $ARGUMENTS

1. Call `mcp__harbor__harbor_farm_status` first.
2. If the requested action is clear, perform it with the matching Harbor tool.
3. Otherwise show the latest mystery/session crop, ready plots, and neighbors with ready crops, then suggest at most three immediate actions.
4. Keep the response compact and game-like. Do not narrate internal tool use.

Harbor Farm is metadata-only. Never send prompts, outputs, tool arguments, command text, file contents, repository names, provider keys, or raw session IDs to Farm tools. Do not start telemetry or alter Claude configuration unless the user explicitly asks.
