# JoinQuest lobby — agent guide

This repository is the **JoinQuest platform** (player shell + GraphQL API + developer dashboard). It is **not** a game repo. Game logic and provision endpoints live in separate game repositories.

**Maintainers (you):** follow this file and [docs/lobby-maintenance.md](docs/lobby-maintenance.md).

**Game integration:** use [`.agents/skills/joinquest-integration/`](.agents/skills/joinquest-integration/) — typically installed into a *game* repo via `install-joinquest-dev.sh`, not the primary workflow here.

**Design precedence:** the Figma Make prototype at [demo.joinquest.cc](https://demo.joinquest.cc) is the **newer standard**. When a doc in `docs/` disagrees with the prototype about product or UX design, follow the prototype and treat the doc as stale — do not cite it as authority for design intent. This does not override shipped behaviour or API contracts.

**Plans and specs stay out of the tree.** Write them to Notion (JoinQuest HQ), not to `docs/`. `docs/superpowers/` is gitignored. Documentation that helps a new engineer onboard *is* wanted in the repo — keep it current, and it does not need to go through a PR.

## Naming

| Term | Meaning |
|------|---------|
| `lobby` | Local folder / this repo |
| **JoinQuest** | Product name; production at [joinquest.cc](https://joinquest.cc) |
| `playhub` | Legacy internal name, fully retired. The GitHub repo is now [`scruffyprodigy/joinquest`](https://github.com/scruffyprodigy/joinquest) and the Go module path is `github.com/scruffyprodigy/joinquest` |

Public-facing text and package metadata (JQ-167), Docker images and k8s
resources (JQ-169), database and volume names (JQ-170), and the Go module path
(JQ-168) have all been de-`playhub`ed. Nothing in the tree carries the old name;
if `playhub` turns up in a diff, it is a mistake.

## Repo map

```text
backend/                          Go GraphQL API (gqlgen)
  graph/schema/*.graphqls         GraphQL schema (edit here first)
  graph/*.resolvers.go            Resolvers (hand-written)
  internal/store/                 PostgreSQL persistence
  internal/developer/             Embedded agent docs (synced copies — do not edit)
  migrations/                     SQL migrations
frontend/src/                     React + Vite player + developer UI
  lib/developers.js               Developer dashboard GraphQL client
  components/developers/          Developer portal components
mcp/joinquest-integration/        @joinquest/mcp-integration npm package
docs/                             Human docs; some are canonical for agent/MCP embeds
k8s/                              Kubernetes manifests (namespace `joinquest`)
scripts/                          setup, test, deploy, doc sync
.agents/skills/joinquest-integration/   Agent skill source (copied to game repos + plugins)
plugins/joinquest/                Cursor/Claude plugin bundle
```

## Local development

```bash
./scripts/setup.sh
./scripts/dev.sh          # frontend :5173, GraphQL :8080/graphql
./scripts/test.sh         # backend + frontend unit tests
```

- GraphQL codegen: `cd backend && make generate` (required after schema edits)
- Backend tests only: `./scripts/test-backend.sh` (uses an isolated per-run test DB)
- MCP tests: `cd mcp/joinquest-integration && npm test`

See [docs/development.md](docs/development.md) for details.

### Running alongside other agents

Several agents run this repo at once, so the test stack is isolated per working
copy. **You do not need to coordinate, wait, or invent a workaround** — just run
the suite.

- Postgres and Redis publish on **ephemeral** host ports. Never hardcode 5432 or
  6379; ask for the address: `./scripts/db.sh url`, `./scripts/db.sh redis-url`.
- Each working copy gets its own compose project (`./scripts/db.sh compose-project`),
  so `docker compose down` here cannot stop another agent's stack. Any direct
  `docker compose` call must run with that project name — prefer `db.sh`.
- Each suite run gets its own `joinquest_test_<run id>` database, created and
  dropped by the harness. Do not target a bare `joinquest_test`.
- A stray container holding a port is reported by name with the fix. If you hit
  a port or database conflict, read the message — do not add sleeps, retries, or
  a hand-rolled second stack.
- Verify isolation with `./scripts/check-concurrent-test-isolation.sh`.

The dev database (`joinquest`) is intentionally shared and long-lived within a
working copy; only the test database is per-run. See
[docs/development.md](docs/development.md#running-suites-concurrently).

## Change checklists

### Developer dashboard / GraphQL feature

Touch every layer that exposes the same operation:

1. `backend/graph/schema/developer.graphqls`
2. `cd backend && make generate`
3. `backend/internal/store/developer.go` (+ `*_test.go`)
4. `backend/graph/developer.resolvers.go`
5. `frontend/src/lib/developers.js` (+ component tests)
6. `frontend/src/components/developers/*.jsx` as needed
7. `mcp/joinquest-integration/src/graphql.js`
8. `mcp/joinquest-integration/src/tools.js`
9. Bump `mcp/joinquest-integration/package.json` version when MCP surface changes
10. `docs/developer-self-service.md` and/or `docs/developer-agent-playbook.md`
11. `./scripts/sync-developer-docs.sh`
12. Run tests (backend, frontend, MCP)

### Playbook or integration guide only

1. Edit **canonical** file in `docs/`:
   - `docs/developer-agent-playbook.md`
   - `docs/developer-integration-guide.md`
2. Run `./scripts/sync-developer-docs.sh` (updates backend embeds, skill copies, plugin copies)
3. CI enforces sync via `./scripts/check-developer-docs-sync.sh`

Never edit `backend/internal/developer/*.md` or `.agents/.../playbook.md` directly.

### GraphQL schema (any domain)

1. Edit `backend/graph/schema/*.graphqls`
2. `cd backend && make generate`
3. Implement resolver + store changes
4. Add/update tests; CI runs gqlgen drift detection

### Database schema

1. Add migration under `backend/migrations/`
2. `cd backend && make migrate-up` locally
3. Update `backend/internal/store/` + tests

## Shipping

**Open a PR as soon as there is code worth reviewing.** A branch with a PR is how
review happens here, so commit, push the branch, and open the PR yourself rather than
leaving finished work sitting uncommitted in a worktree — do not wait to be asked.
Draft it if it is not ready; say what you verified in the description.

- Code always goes on a branch with a PR — never commit or push code to `main` directly.
- Documentation-only changes are the exception: they do not need a PR (see *Plans and
  specs* above).
- Deploying and publishing are still gated on explicit approval, every time — a merged
  PR is not approval to ship.

Pre-ship checks (human approval required from here down):

```bash
./scripts/ship-joinquest.sh --check
```

Production deploy ([joinquest.cc](https://joinquest.cc)):

```bash
./scripts/build-and-push.sh --push
gcloud container clusters get-credentials joinquest --region us-east1 --project joinquest-demo  # refresh if Unauthorized
./scripts/deploy-joinquest.sh
```

MCP npm publish (only when `mcp/` changed and version bumped):

```bash
./scripts/publish-joinquest-mcp.sh
```

Full runbook: [docs/lobby-maintenance.md](docs/lobby-maintenance.md).

## Conventions

- **Minimal scope** — smallest correct diff; don't refactor unrelated code
- **Match existing style** — Go: `gofmt`, store layer for DB; frontend: JSX + Vitest
- **No secrets** — never commit `.env`, API keys, k8s secrets, or `.DS_Store`
- **Tests** — add store/resolver tests for backend logic; component/lib tests for dashboard changes
- **Human gates** — production deploy and npm publish require explicit user approval. Opening a PR does not: that is how work gets reviewed

## Key docs

| Topic | Doc |
|-------|-----|
| Maintainer runbook | [docs/lobby-maintenance.md](docs/lobby-maintenance.md) |
| Local dev | [docs/development.md](docs/development.md) |
| Architecture | [docs/architecture.md](docs/architecture.md) |
| Testing | [docs/testing.md](docs/testing.md) |
| Developer self-service spec | [docs/developer-self-service.md](docs/developer-self-service.md) |
| Game dev install scripts (manifest) | [scripts/joinquest-setup/README.md](scripts/joinquest-setup/README.md) |
| Agent playbook (canonical) | [docs/developer-agent-playbook.md](docs/developer-agent-playbook.md) |
| Integration guide (canonical) | [docs/developer-integration-guide.md](docs/developer-integration-guide.md) |
