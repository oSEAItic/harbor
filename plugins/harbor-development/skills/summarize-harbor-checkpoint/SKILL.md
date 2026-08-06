---
name: summarize-harbor-checkpoint
description: Summarize and persist a stable Git commit range as an auditable Harbor checkpoint. Use when the Harbor development hook reports a HEAD transition, when an Agent has committed coherent work for a Harbor Feature, or when the user asks what a development checkpoint accomplished.
---

# Summarize Harbor Checkpoint

Record what a completed Git range accomplished without treating transcripts, diffs, or file lists as the product narrative.

## Workflow

1. Use the `feature_id`, `base_sha`, `head_sha`, and repository path supplied by the Harbor hook. If they are absent, call `harbor_feature_context` and inspect `git rev-parse HEAD`; do not create or guess a Feature.
2. Confirm `HEAD` still matches the intended range. Extend the head only when the additional commits belong to the same coherent checkpoint.
3. Read durable facts first:
   - Harbor Feature title, scope decisions, and prior checkpoint summaries
   - `git log --format=fuller <base>..<head>`
   - commit bodies and trailers
   - verification commands and results actually observed in the current work
   Inspect the diff only when these sources cannot explain the outcome.
4. Produce:
   - `outcome`: one concise programmer-readable explanation of what changed and why it matters
   - `decisions`: only durable product or architecture choices
   - `verification`: only checks that actually ran and their meaningful result
   - `remaining`: known unfinished, deferred, blocked, or risky work
5. Call `harbor_checkpoint_finalize`. Include the current session id, source, and model when available. The operation is idempotent for the same Feature, repository, base, and head.

Use the CLI only when the MCP tool is unavailable:

```bash
harbor feature checkpoint finalize <feature-id> \
  --repo <repo> --base <base-sha> --head <head-sha> \
  --outcome "<outcome>" \
  --decision "<decision>" \
  --verification "<actual check>" \
  --remaining "<known remaining work>" \
  --source codex --session <session-id> --model <model>
```

Omit empty repeated fields. Never claim verification from a planned command, store raw prompts or responses, automatically overwrite a PR body, or block the user's coding session when Harbor is unavailable.
