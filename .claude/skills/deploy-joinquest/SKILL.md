---
name: deploy-joinquest
description: Use when deploying, shipping, or releasing the JoinQuest/lobby app to production (joinquest.cc) — running ./scripts/deploy-joinquest.sh or ./scripts/ship-joinquest.sh, or preparing a release/PR that will be deployed.
---

# Deploy JoinQuest

`docs/lobby-maintenance.md` is canonical — read its "Deploy to joinquest.cc (GKE)" and "Typical ship sequence" sections before deploying. This skill only points you there and flags what gets skipped.

## Before deploying

- `AGENTS.md` gates push/deploy/publish behind explicit user approval — each step, not one blanket yes.
- Run `./scripts/ship-joinquest.sh --check` first.

## Where to run each script

Worktrees make this matter, and the two scripts want opposite working copies.

| Script | Run it from | Why |
|---|---|---|
| `build-and-push.sh` | the working copy holding the code you want live — a worktree is fine | it builds from the working tree, so this is how branch code reaches production before merge |
| `deploy-joinquest.sh` | **the primary clone only** | it applies `k8s/secrets/*.yaml`, which are gitignored; a worktree has only the `.example.yaml` templates |
| `ship-joinquest.sh --check` | the working copy you are shipping | it tests the code you are about to build |

Run `deploy-joinquest.sh` from a worktree and it dies partway with `error: the path
"k8s/secrets/pg-auth.yaml" does not exist` — *after* restarting cert-manager and
reapplying the ClusterIssuer, so it half-applies before failing. Re-run it from the
primary clone; the k8s manifests are the same in both, and the images are already
pushed by then, so nothing is lost.

Note that the Bash tool's working directory persists between calls, so a `cd` into a
worktree earlier in a session is still in effect when you invoke the deploy script.
`cd` to the primary clone explicitly rather than relying on where you think you are.

## After deploying

Run every check under "Post-deploy smoke" in `docs/lobby-maintenance.md` — including the `env.js` cache-control check. That check exists because of JQ-54: nginx's regex-location-wins-over-prefix-location behavior can silently make `env.js` cacheable for a year again with no visible error. Full explanation: `docs/lobby-maintenance.md#envjs-caching-jq-54` and `docs/environment-configuration.md`.

If a fresh visitor still gets stale config after the pod itself serves the right value, purge the Cloudflare cache for the affected path — that clears an already-cached copy from before the fix, not a sign the fix regressed.
