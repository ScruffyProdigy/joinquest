---
name: finishing-a-task
description: Use when a task's pull request has merged and its acceptance criteria are met, before picking up the next ticket — or when cleaning up after any completed JoinQuest work.
---

# Finishing a task

Merged is not finished. A task leaves behind a worktree, a Docker stack, a local branch and an open ticket, and none of them clear themselves.

**Order matters: tear the Docker stack down _before_ removing the worktree.** `scripts/db.sh` derives `COMPOSE_PROJECT_NAME` from the working copy's path, so once the directory is gone you can no longer ask the repo what its stack was called. That is how stacks get orphaned.

## Check it is actually done

- [ ] Every acceptance criterion on the ticket is met — not just "the PR merged"
- [ ] Tests pass, and you ran them rather than assuming

```bash
p=.claude/worktrees/<slug>
git fetch origin                                                    # merge state is only as fresh as your last fetch
git merge-base --is-ancestor "$(git -C "$p" rev-parse HEAD)" origin/main && echo merged
git -C "$p" status --porcelain                                      # must be empty
```

Both checks must pass. Uncommitted work in a worktree is invisible once the directory is gone.

## Tear down, in this order

```bash
# 1. Docker stack — FROM INSIDE the worktree, while it still exists
(cd "$p" && docker compose -p "$(./scripts/db.sh compose-project)" down -v)

# 2. Worktree
git worktree unlock "$p" 2>/dev/null; git worktree remove "$p"

# 3. Local branch
git branch -d <branch>
```

`-v` drops the stack's volumes. Test databases are disposable; without it every finished ticket leaves a Postgres volume behind forever.

## Then the ticket

Move it to Done — but **check first**, don't assume either way. The GitHub integration usually closes tickets on merge, and re-closing a closed ticket is noise. Equally, don't assume it fired.

## Orphan recovery

If the worktree is already gone and the stack is still running, the project name is still on the containers:

```bash
docker ps -a --format '{{.Label "com.docker.compose.project"}}' | sort -u | grep '^lobby-'
docker compose -p <project> down -v
```

## Red flags — stop

- Removing a worktree before tearing down its Docker stack
- Concluding "merged" without a fresh `git fetch`
- Marking a ticket Done because the PR merged, without checking the acceptance criteria
- `docker ps -a` listing `lobby-*` projects whose worktrees no longer exist
