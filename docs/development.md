# Development Guide

This guide will help you set up and run JoinQuest locally for development.

**AI agents / maintainers:** [AGENTS.md](../AGENTS.md) · [lobby-maintenance.md](lobby-maintenance.md)

**Context:** JoinQuest is the shared lobby and integration platform described in **[Product vision](vision.md)**. This repo is the server + player UI; partner games (e.g. `demo-game-rps`) integrate via the handoff protocol.

> **Naming:** product = **JoinQuest**. GitHub repo = `scruffyprodigy/joinquest`. Local folder is often `lobby`. The legacy `playhub` name is fully retired.

## Current Development Status

### ✅ Ready for Development
- **Environment Configuration**: Docker-based runtime environment injection
- **GraphQL API**: Catalog matchmaking (`seatTemplate`, `queuePath`, forming LFG), auth, PostgreSQL-backed resolvers
- **Frontend Foundation**: React app with testing infrastructure
- **Kubernetes Deployment**: Multi-environment deployment scripts
- **Testing Suite**: Comprehensive test coverage
- **Database Integration**: PostgreSQL connection management
- **CI/CD Pipeline**: GitHub Actions workflows with proper linting and testing

### 🚧 In Active Development
- **Digital goods**: Player purchase and trade flows

### 📋 Next Steps
- **E2E partner path**: [end-to-end-partner-checklist.md](./end-to-end-partner-checklist.md)
- **Phase C LFG**: weighted dequeue, `allocations` by affinity — [lfg-phase-b-plan.md](./lfg-phase-b-plan.md), [seat-templates-and-matchmaking.md](./seat-templates-and-matchmaking.md)
- **Game-backed queue options**: [composition-and-join-options.md](./composition-and-join-options.md) — eligibility from game API before join

## Prerequisites

- **Go 1.25+** - [Download](https://golang.org/dl/)
- **Node.js 20+** - [Download](https://nodejs.org/)
- **Docker & Docker Compose** - [Download](https://www.docker.com/)
- **Git** - [Download](https://git-scm.com/)

## Quick Setup

1. **Clone the repository**
   ```bash
   git clone https://github.com/scruffyprodigy/joinquest.git
   cd joinquest   # folder may be named lobby locally
   ```

2. **Run the setup script**
   ```bash
   ./scripts/setup.sh
   ```

3. **Start development servers**
   ```bash
   ./scripts/dev.sh
   ```

   `./scripts/dev.sh` starts Postgres and **Redis** (for queue/room pub/sub). Set `REDIS_URL` in `.env` if you run services outside Docker Compose. Optional tracing: `LOBBY_PUBSUB_DEBUG=true` (backend) and `?lobbyDebug=1` or `localStorage.lobbyDebug=1` (frontend) — see [pubsub.md](./pubsub.md).

## Manual Setup

### Backend Setup

1. **Navigate to backend directory**
   ```bash
   cd backend
   ```

2. **Install Go dependencies**
   ```bash
   go mod download
   ```

3. **Generate GraphQL code**
   ```bash
   go run github.com/99designs/gqlgen@v0.17.81 generate
   ```

4. **Run tests**
   ```bash
   go test ./...
   ```

5. **Start the server**
   ```bash
   go run server.go
   ```

The backend will be available at `http://localhost:8080`

### Frontend Setup

1. **Navigate to frontend directory**
   ```bash
   cd frontend
   ```

2. **Install dependencies**
   ```bash
   npm install
   ```

3. **Run tests**
   ```bash
   npm run test:run
   ```

4. **Start development server**
   ```bash
   npm run dev
   ```

The frontend will be available at `http://localhost:5173`

## Development Workflow

### Making Changes

1. **Backend Changes**
   - Modify GraphQL schema in `backend/graph/schema/`
   - Update resolvers in `backend/graph/`
   - Run `go run github.com/99designs/gqlgen@v0.17.81 generate` after schema changes
   - Test with `go test ./...`

2. **Frontend Changes**
   - Modify components in `frontend/src/`
   - Update tests as needed
   - Test with `npm run test:run`

### Code Generation

The backend uses gqlgen for GraphQL code generation:

```bash
cd backend
go run github.com/99designs/gqlgen@v0.17.81 generate
```

### Catalog manifest sync (ops)

Refresh a catalog game's cached modes/seats from its live `api_base_url` (same logic as admin manifest sync):

```bash
export DATABASE_URL="postgres://..."
cd backend && go run ./cmd/syncmanifest <game-uuid>
```

Production deploys also run `k8s/jobs/patch-game-handoff-urls.yaml` via `./scripts/deploy-joinquest.sh` to align handoff URLs and seed demo game rows.

### Testing

- **Backend tests**: `./scripts/test-backend.sh` (uses an isolated per-run test DB; does not touch dev `joinquest`)
- **Frontend unit tests**: `cd frontend && npm run test:run`
- **Frontend E2E tests**: `cd frontend && npm run test:e2e` (uses dev `joinquest` via running backend)
- **All tests**: `./scripts/test.sh`
- **Test DB only**: `./scripts/db.sh test-migrate` then `export DATABASE_URL="$(./scripts/db.sh test-url)"`
- **Concurrency isolation check**: `./scripts/check-concurrent-test-isolation.sh` (add `--docker` to start two real stacks side by side)

#### Running suites concurrently

Several agents or terminals can run the backend suite at the same time. Nothing
needs coordinating — isolation comes from three things, all handled by
[`scripts/lib/db-runtime.sh`](../scripts/lib/db-runtime.sh):

| Layer | How it is isolated |
|-------|--------------------|
| Docker Compose project | `lobby-<dir>-<hash of the working copy path>`, so `docker compose down` in one checkout never tears down another's stack |
| Host ports | Postgres and Redis publish on **ephemeral** ports (`0:5432`, `0:6379`). Discover them with `./scripts/db.sh url` and `./scripts/db.sh redis-url` — never assume 5432 or 6379 |
| Test database | `joinquest_test_<run id>`. `test-backend.sh` and `test.sh` mint a fresh run id per invocation and drop the database when the run exits |

Useful commands:

```bash
./scripts/db.sh url               # dev database URL, with the real published port
./scripts/db.sh redis-url         # Redis URL, with the real published port
./scripts/db.sh test-url          # this working copy's test database URL
./scripts/db.sh test-db           # just the test database name
./scripts/db.sh compose-project   # this working copy's compose project name
./scripts/db.sh test-drop         # drop the test database
```

Standalone `db.sh test-*` commands use a **stable** run id derived from the
working copy path, so `test-url` is repeatable between calls. Set
`LOBBY_TEST_RUN_ID` to force a different database.

Need a fixed address for an external client (a psql GUI, say)? Pin it:

```bash
LOBBY_POSTGRES_HOST_PORT=5432 ./scripts/db.sh up
```

If that port is taken, `db.sh` says so by name and lists the ways out, rather
than surfacing a raw `Bind for 0.0.0.0:5432 failed` from Docker.

The long-lived dev database (`joinquest`) is deliberately **not** per-run — it is
shared dev state, and each working copy already has its own container holding
its own copy of it.
- **JoinQuest integration MCP**: `cd mcp/joinquest-integration && npm test` — see [mcp/joinquest-integration/README.md](../mcp/joinquest-integration/README.md)
- **Developer doc sync check**: `./scripts/check-developer-docs-sync.sh`
- **Pre-ship checks**: `./scripts/ship-joinquest.sh --check`
- **Agent skill**: [`.agents/skills/joinquest-integration/`](../.agents/skills/joinquest-integration/) — copy into game projects; playbook at [docs/developer-agent-playbook.md](./developer-agent-playbook.md)

### JoinQuest integration MCP (developer agents)

The MCP server in `mcp/joinquest-integration/` exposes developer dashboard operations to AI agents (Cursor, Claude, Copilot, Roo, Windsurf, Cline, etc.) via stdio. It calls the same GraphQL API as the portal (defaults to `https://joinquest.cc/graphql`).

**Game developers:** [developer dashboard](https://joinquest.cc/developers) wizard or [`.agents/skills/joinquest-integration/mcp-setup.md`](../.agents/skills/joinquest-integration/mcp-setup.md). MCP runs via `npx -y @joinquest/mcp-integration` (only `JOINQUEST_API_KEY` in env). ChatGPT: no MCP — register at `https://joinquest.cc/developers?path=manual`.

**Lobby contributors** testing against `./scripts/dev.sh`:

1. Generate a developer API key (local dashboard or joinquest.cc).
2. Run `./scripts/install-joinquest-mcp.sh`.
3. Add `.cursor/mcp.json` using [cursor-mcp.example.json](../mcp/joinquest-integration/cursor-mcp.example.json) plus:
   ```json
   "JOINQUEST_API_URL": "http://localhost:8080/graphql"
   ```
   Or use `scripts/run-joinquest-mcp-cursor.sh` as the Cursor `command` with that env override.

Run MCP tests: `cd mcp/joinquest-integration && npm test`.

### Linting

- **Backend**: `cd backend && go vet ./...`
- **Frontend**: `cd frontend && npm run lint`

## Environment Configuration

JoinQuest uses a Docker-based environment configuration system that allows the same Docker image to work across different environments (local, staging, production).

### Environment Variables

#### Backend
- `PORT` - Server port (default: 8080)
- `DATABASE_URL` - PostgreSQL connection string
- `MAGIC_LINK_BASE_URL` - Prefix for email sign-in links (see `.env.example`)
- `SESSION_COOKIE_NAME` / `SESSION_COOKIE_SECURE` - Session cookie settings
- `CORS_ALLOWED_ORIGINS` - Allowed browser origins for GraphQL
- `JWT_PRIVATE_KEY_PEM` / `JWKS_KID` - JWT signing for session cookies and seat tokens (see `.env.example`)

#### Frontend (Runtime Injection)
- `REACT_APP_ENV` - Environment identifier (local, staging, production)
- `REACT_APP_API_BASE_URL` - Backend API URL for the current environment

### Environment-Specific Configurations

#### Local Development
- **Frontend**: `http://localhost:5173`
- **GraphQL**: `http://localhost:8080/graphql`
- **Start**: `./scripts/dev.sh`

#### Production (JoinQuest)
- **Public URL**: `https://joinquest.cc`
- **GraphQL**: `https://joinquest.cc/graphql`
- **Deploy**: `./scripts/deploy-joinquest.sh`

### How Environment Configuration Works

1. **Docker Entrypoint**: The frontend Docker image includes a script that generates `env.js` at runtime
2. **Kubernetes ConfigMaps**: Each environment has its own ConfigMap with environment-specific values
3. **Runtime Injection**: When the container starts, it reads environment variables and creates the `env.js` file
4. **Frontend Access**: The frontend loads `/env.js` and accesses variables via `window.env`

For detailed information, see [Environment Configuration Guide](environment-configuration.md).

## Recent Updates

### GitHub Workflow Fixes (Latest)

The CI/CD pipeline has been updated with the following improvements:

- **ESLint Configuration**: Fixed linting issues with proper environment configuration
- **Test Environment Setup**: Added proper mocking for `window.env` in test environment
- **GraphQL Code Generation**: Ensured generated code is up to date with schema
- **Database Integration**: Added PostgreSQL connection management

All workflows now pass successfully:
- ✅ Frontend Tests (linting, unit tests, E2E tests)
- ✅ Environment Configuration Tests
- ✅ GraphQL Drift Detection

## Troubleshooting

### Common Issues

1. **GraphQL generation fails**
   - Ensure you're in the backend directory
   - Check that all schema files are valid GraphQL
   - Run `make generate` to regenerate code

2. **Frontend tests fail**
   - Clear node_modules: `rm -rf node_modules && npm install`
   - Check that all dependencies are installed
   - Ensure `window.env` is properly mocked in test setup

3. **Linting errors**
   - Run `npm run lint` to check for issues
   - ESLint is configured for Node.js, browser, and test environments

4. **Port conflicts**
   - Backend default: 8080
   - Frontend default: 5173
   - Change ports in respective config files if needed

### Getting Help

- Check the [API Documentation](api.md)
- Review [Architecture Overview](architecture.md)
- Open an issue on GitHub
