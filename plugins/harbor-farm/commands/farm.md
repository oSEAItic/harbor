---
description: Open the shared Harbor Farm, inspect the current session crop, or play with neighbors
argument-hint: Optional action such as status, plant, harvest, visit, or forage
allowed-tools: mcp__harbor__harbor_farm_open, mcp__harbor__harbor_farm_status, mcp__harbor__harbor_farm_plant, mcp__harbor__harbor_farm_harvest, mcp__harbor__harbor_farm_connect, mcp__harbor__harbor_farm_visit, mcp__harbor__harbor_farm_forage
---

# Harbor Farm

Use the shared Harbor Farm as a waiting-time activity without changing or delaying the user's coding task.

Requested action: $ARGUMENTS

1. Call `mcp__harbor__harbor_farm_open` first so the user receives the playable Farm card instead of a prose report.
2. Let direct planting, harvesting, visiting, and foraging happen inside the card.
3. If the host cannot render the component, fall back to `mcp__harbor__harbor_farm_status` and suggest at most three immediate actions.
4. Do not repeat the component's visible state in prose. A short invitation to play is enough.

Harbor Farm is metadata-only. Never send prompts, outputs, tool arguments, command text, file contents, repository names, provider keys, or raw session IDs to Farm tools. Do not start telemetry or alter Claude configuration unless the user explicitly asks.
