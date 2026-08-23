---
name: deploy-joinquest
description: Use when deploying, shipping, or releasing the JoinQuest/lobby app to production (joinquest.cc) — running ./scripts/deploy-joinquest.sh or ./scripts/ship-joinquest.sh, or preparing a release/PR that will be deployed.
---

# Deploy JoinQuest

`docs/lobby-maintenance.md` is canonical — read its "Deploy to joinquest.cc (GKE)" and "Typical ship sequence" sections before deploying. This skill only points you there and flags what gets skipped.

## Before deploying

- `AGENTS.md` gates push/deploy/publish behind explicit user approval — each step, not one blanket yes.
- Run `./scripts/ship-joinquest.sh --check` first.

## After deploying

Run every check under "Post-deploy smoke" in `docs/lobby-maintenance.md` — including the `env.js` cache-control check. That check exists because of JQ-54: nginx's regex-location-wins-over-prefix-location behavior can silently make `env.js` cacheable for a year again with no visible error. Full explanation: `docs/lobby-maintenance.md#envjs-caching-jq-54` and `docs/environment-configuration.md`.

If a fresh visitor still gets stale config after the pod itself serves the right value, purge the Cloudflare cache for the affected path — that clears an already-cached copy from before the fix, not a sign the fix regressed.
