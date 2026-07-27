---
name: harbor-farm
description: Play, inspect, or manage the authenticated user's Harbor Farm, including session-native mystery crops, normal plots, neighbors, visits, and limited crop foraging. Use when the user mentions Harbor Farm, farming while an agent runs, session crops, Farm codes, visiting friends, or stealing/gathering crops.
---

# Harbor Farm

Harbor owns one account-level Farm ledger shared by Codex, Claude Code, Harbor Studio, and the CLI. Use the Harbor MCP tools; never simulate a separate Farm in chat.

## Interaction loop

1. Call `harbor_farm_open` whenever the user asks to open, view, check, or play with the Farm. It renders the interactive Farm card; do not replace it with a prose status report.
2. Let the user plant, harvest, refresh, visit, and forage directly in the card. Do not narrate data that is already visible in the component.
3. Use `harbor_farm_status` only for headless reasoning or when the host cannot render MCP Apps UI.
4. When acting conversationally, surface only the most useful waiting-time action: harvest a ready plot, plant an empty plot, inspect the latest session crop, or visit a neighbor with ready crops.
5. After a mutation, call `harbor_farm_status` again only when the result does not already contain the updated state.

## Tools

- `harbor_farm_status`: profile, plots, active agents, mystery/session crops, Farm code, and neighbors.
- `harbor_farm_open`: render the playable Farm card with direct actions. Prefer this for human-facing Farm requests.
- `harbor_farm_plant`: plant wheat, carrot, or tomato in plot 0-5.
- `harbor_farm_harvest`: harvest a ready plot.
- `harbor_farm_connect`: connect a friend using an eight-character Farm code.
- `harbor_farm_visit`: inspect a connected neighbor's ready plots and public session crop collection.
- `harbor_farm_forage`: gather one clipping from a ready neighbor plot. A visitor can gather once per crop and the owner keeps at least 80 percent of the harvest.

## Session crops

A connected Agent session begins as a mystery seed, grows from metadata-only lifecycle events, and reveals a deterministic species and genome when the Agent yields or finishes. Explain traits using only the receipt fields returned by Harbor. Do not infer or expose task content.

## Safety and privacy

- Never send prompts, responses, tool arguments, command text, file names, file contents, repo names, provider keys, or raw session IDs to Farm tools.
- Do not configure telemetry, edit Codex or Claude settings, or start background receivers unless the user explicitly asks.
- Do not invoke extra model work merely to earn Farm rewards.
- Waiting and approval states earn no coins.
- Farm coins are separate from paid oSEAItic credits.
- If Harbor Cloud is unavailable, report the cached/offline state and keep the user's coding task primary.
