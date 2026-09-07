---
name: starting-a-task
description: Use when beginning work on a JoinQuest ticket or any code change in the lobby repo, before making the first edit — especially when several agents may be working at the same time, or when picking up a Linear issue.
---

# Starting a task

**Every task gets its own git worktree, branched from `origin/main`.**

Several agents work in `~/Projects/lobby` at once. It is a shared clone: uncommitted changes there can be destroyed by another agent without warning, and have been. Nothing you are told about "the repo" makes it safe to edit directly.

## Before the first edit

```bash
cd ~/Projects/lobby
git fetch origin                                   # local main goes stale within minutes
git worktree add -b <branch> .claude/worktrees/<slug> origin/main
cd .claude/worktrees/<slug>
```

Branch from **`origin/main`**, never from local `main`. PRs merge continuously here; a local `main` that looked current an hour ago is not.

For a Linear ticket, use the issue's `gitBranchName` field as `<branch>` — it is the name Linear will match against the PR. Use a short slug for the directory (`jq-124-handoff-identity`).

`.claude/worktrees/` is gitignored, so worktrees never appear in a commit.

## Verify isolation before working

```bash
git rev-parse --show-toplevel    # must end in .claude/worktrees/<slug>
git status --short               # must be empty
```

If the first command prints `/Users/coleryan/Projects/lobby`, you are in the shared clone. Stop and make a worktree.

## While working

| Concern | What to do |
|---|---|
| Tests | Isolated per working copy — ports are auto-assigned. Just run `./scripts/test-backend.sh` |
| Migrations | `ls backend/migrations \| tail -1` before adding one. Parallel agents pick the same number; renumber if yours collides |
| Plans, specs, session notes | Write to Notion (JoinQuest HQ), never the tree. `docs/superpowers/` is gitignored |
| Documentation for engineers | Belongs in the repo, keep it current, and it does **not** need a PR — commit to `main` directly |
| Deploying | Load the `deploy-joinquest` skill; each destructive step needs its own approval |

Commit early. Work sitting uncommitted is work at risk, in a worktree or out of it.

## After it merges

```bash
p=.claude/worktrees/<slug>
git fetch origin                                                  # merge state is only as fresh as your last fetch
git merge-base --is-ancestor "$(git -C $p rev-parse HEAD)" origin/main && echo merged
git -C $p status --porcelain                                      # must be empty
git worktree unlock $p && git worktree remove $p
```

Check merged **and** clean before removing. Do not conclude "not merged" from a stale fetch.

## Red flags — stop

- You are about to edit a file and `pwd` is `~/Projects/lobby`
- You branched from `main` rather than `origin/main`
- `git status` in the shared clone shows changes you made
- You are writing a spec or plan into `docs/`
- You are about to claim something is merged without having fetched first
