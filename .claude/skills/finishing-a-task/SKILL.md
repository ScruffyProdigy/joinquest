---
name: finishing-a-task
description: Use when a task's pull request has merged and its acceptance criteria are met, before picking up the next ticket — or when cleaning up after any completed JoinQuest work.
---

# Finishing a task

Merged is not finished. A task leaves behind a worktree, a Docker stack, a local branch, a branch on GitHub and an open ticket, and none of them clear themselves.

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

# 4. Branch on GitHub — nothing deletes it for you
git push origin --delete <branch>
```

`-v` drops the stack's volumes. Test databases are disposable; without it every finished ticket leaves a Postgres volume behind forever.

`delete_branch_on_merge` is **off** on this repo, so a merged branch stays on GitHub until someone removes it, and step 3 does not touch the remote. Delete it only once the merge check above has passed — the branch is the only copy of the work until then.

## Then the ticket

Move it to Done — but **check first**, don't assume either way. The GitHub integration usually closes tickets on merge, and re-closing a closed ticket is noise. Equally, don't assume it fired.

## Orphan recovery

Three kinds. The second is invisible to the first check, and the third lives on GitHub rather than this machine.

**Stack still exists** — the project name is on the containers:

```bash
docker ps -a --format '{{.Label "com.docker.compose.project"}}' | sort -u | grep '^lobby-'
docker compose -p <project> down -v
```

**Containers gone, volume left behind** — belongs to no project, so it appears in no `docker ps` and no compose listing. Torn down without `-v`, or the containers were pruned separately. These accumulate silently at ~70MB each:

```bash
docker volume ls -q -f dangling=true | grep joinquest      # look first
docker volume ls -q -f dangling=true | grep joinquest | xargs -r docker volume rm
```

The invariant worth checking: surviving `*_joinquest_pgdata` volumes should map one-to-one to live worktrees plus the main clone. Anything else is an orphan.

Volumes created before JQ-170 are named `*_playhub_pgdata` and the grep above will not
find them. Sweep them once with `grep playhub`; every one of them is an orphan, because
no current working copy uses that name.

**Merged branches still on GitHub** — from every task finished before this step existed:

```bash
git fetch origin --prune
git branch -r --merged origin/main --list 'origin/*' | grep -v 'origin/main$'   # look first
```

Keep the `--list 'origin/*'`. It scopes the listing to `origin`, so a second remote's
`main` cannot show up as a merged branch — deletable-looking, and very much not.
(The `playhub` remote this once guarded against was removed in JQ-167.)

Everything left is already in `main`, so deleting it loses nothing. Still, go one
at a time rather than piping the list to `--delete`: a merged branch someone has
since branched off shows up here too.

## Red flags — stop

- Removing a worktree before tearing down its Docker stack
- Concluding "merged" without a fresh `git fetch`
- Marking a ticket Done because the PR merged, without checking the acceptance criteria
- `docker ps -a` listing `lobby-*` projects whose worktrees no longer exist
- Deleting a local branch and assuming the GitHub one went with it
