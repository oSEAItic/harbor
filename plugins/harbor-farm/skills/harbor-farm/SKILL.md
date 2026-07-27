---
name: harbor-farm
description: Play, inspect, or manage the authenticated user's Harbor Farm, including session-native mystery crops, normal plots, neighbors, visits, and limited crop foraging. Use when the user mentions Harbor Farm, farming while an agent runs, session crops, Farm codes, visiting friends, or stealing/gathering crops.
---

# Harbor Farm

Harbor owns one account-level Farm ledger shared by Codex, Claude Code, Harbor Studio, and the CLI. Use the Harbor MCP tools; never simulate a separate Farm in chat.

## Interaction loop

1. Call `harbor_farm_status` before recommending or mutating anything.
2. Briefly surface the most useful waiting-time action: harvest a ready plot, plant an empty plot, inspect the latest session crop, or visit a neighbor with ready crops.
3. Use direct Farm tools for the user's chosen action. Planting, harvesting, and foraging change the shared ledger.
4. After a mutation, call `harbor_farm_status` again only when the result does not already contain the updated state.

## Tools

- `harbor_farm_status`: profile, plots, active agents, mystery/session crops, Farm code, and neighbors.
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
