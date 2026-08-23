# JoinQuest

**Build the game. We handle the lobby.**

JoinQuest is a gaming platform for web developers who want to ship a multiplayer title without spending months on accounts, matchmaking, storefronts, and payments first. Players discover games here, queue up, and get sent to **your** game on **your** URL with a signed seat assignment. You keep your own site and codebase; we provide the plumbing — and aim to do it better than a solo dev would have time to.

→ **[Product vision](docs/vision.md)** — why this exists, who it’s for, how integration fits together.

## For game developers

| You build | JoinQuest provides |
|-----------|-------------------|
| Game logic, UI, servers on your domain | Sign-in, catalog, queues, matchmaking (LFG) |
| Seat manifest (`seatTemplate`) | Seat map assignment + provision push + JWT `seatKey` |
| Accept or reject rosters | Player traffic and (roadmap) shop / entitlements |

**Integration docs:** [Developer self-service](docs/developer-self-service.md) · [Lobby ↔ game handoff](docs/lobby-protocol-handoff.md) · [Game catalog](docs/game-catalog-architecture.md) · [Seat templates & LFG](docs/seat-templates-and-matchmaking.md)

**Making a game?** Register and integrate on your timeline — public listing is reviewed when you're ready. See [developer self-service](docs/developer-self-service.md).

Reference game: `demo-game-rps` (sibling repo / deploy scripts in this project).

## For players (today)

Browse the catalog, sign in, join mode queues, and launch into registered third-party games when a match is ready.

## Features

### ✅ Implemented
- **Environment Configuration**: Docker-based runtime environment injection
- **GraphQL API**: Catalog matchmaking, auth, digital goods, game handoff
- **Frontend Foundation**: React application with testing infrastructure
- **Kubernetes Deployment**: Multi-environment deployment scripts
- **Testing Suite**: Comprehensive unit, integration, and E2E tests
- **CI/CD Pipeline**: GitHub Actions workflows for testing and deployment
- **Database Integration**: PostgreSQL setup with connection management
- **Database Migrations**: Complete migration system with CLI and programmatic support
- **Linting & Code Quality**: ESLint configuration with proper test environment setup
- **Developer self-service**: Register games, dashboard, private testing, 19-check integration suite, public release review, integration MCP — see [developer-self-service](docs/developer-self-service.md) · [mcp/joinquest-integration/README.md](mcp/joinquest-integration/README.md)

### 🚧 In Development
- **Digital trading**: Player-facing purchase and trade flows
- **Developer self-service follow-ups**: Scheduled integration re-checks, JWKS rotation remote check

### 📋 Planned
- **Payment Processing**: Integration with payment providers
- **Analytics Dashboard**: Usage and performance metrics

## Architecture

This project consists of:

- **Backend**: Go-based GraphQL API with gqlgen
- **Frontend**: React + Vite application (JoinQuest player shell)
- **Database**: PostgreSQL with connection management

See [Architecture Overview](docs/architecture.md) and [Vision](docs/vision.md).

## For AI agents / maintainers

Maintaining this repo (backend, dashboard, deploy): start with **[AGENTS.md](AGENTS.md)** and **[docs/lobby-maintenance.md](docs/lobby-maintenance.md)**.

Helping a game developer integrate with JoinQuest: use the skill in [`.agents/skills/joinquest-integration/`](.agents/skills/joinquest-integration/) (usually installed into their game repo).

> **Naming:** the product is **JoinQuest** (`joinquest.cc`). This folder is often named `lobby` locally. The GitHub repo and some internals still use the legacy name `playhub` (Docker images, database names).

## Getting Started

### Prerequisites

- Go 1.25+
- Node.js 20+
- Docker (optional)

### Quick Setup

```bash
git clone https://github.com/scruffyprodigy/playhub.git
cd playhub   # or lobby — same repo
./scripts/setup.sh
./scripts/dev.sh
```

### Manual Setup

See [Development Guide](docs/development.md) for detailed setup instructions.

## Development

### Running Tests

```bash
# All tests
./scripts/test.sh

# Backend only
./scripts/test-backend.sh

# Frontend only
./scripts/test-frontend.sh

# With E2E tests
./scripts/test-frontend.sh --e2e
```

### Development Servers

```bash
# Start both frontend and backend
./scripts/dev.sh
```

### Code Generation

The backend uses gqlgen for GraphQL code generation:

```bash
cd backend
go run github.com/99designs/gqlgen@v0.17.81 generate
```

### Database Migrations

The project includes a complete database migration system:

```bash
cd backend

# Run all pending migrations
make migrate-up

# Check migration status
make migrate-version

# Rollback last migration
make migrate-down
```

See [Database Migrations](docs/database-migrations.md) for detailed documentation.

### Deployment

```bash
# Local dev stack (Docker Compose)
./scripts/dev.sh

# Deploy to production (joinquest.cc)
./scripts/build-and-push.sh --push
./scripts/deploy-joinquest.sh
```

See [Environment Configuration](docs/environment-configuration.md) for detailed deployment instructions.

## Documentation

- **[AGENTS.md](AGENTS.md)** — AI agent / maintainer quick reference (repo map, checklists, ship)
- **[Lobby maintenance runbook](docs/lobby-maintenance.md)** — Deploy, doc sync, MCP publish
- **[Product vision](docs/vision.md)** — Why JoinQuest exists; indie dev + integration story
- **[End-to-end partner checklist](docs/end-to-end-partner-checklist.md)** — Verify sign-in → queue → play → return
- **[Development Guide](docs/development.md)** — Setup and development workflow
- **[Architecture Overview](docs/architecture.md)** — System design and components
- **[Game Catalog Architecture](docs/game-catalog-architecture.md)** — Modes, queues, manifest sync
- **[Lobby ↔ Game Handoff](docs/lobby-protocol-handoff.md)** — Provision, JWT, integration protocol
- **[Seat templates & LFG](docs/seat-templates-and-matchmaking.md)** — Manifest, seat map, matchmaking spec
- **[API Documentation](docs/api.md)** — GraphQL API reference
- **[Testing Guide](docs/testing.md)** — Testing strategies and running tests
- **[Database Migrations](docs/database-migrations.md)** — Schema migrations
- **[Pub/Sub](docs/pubsub.md)** — Redis queue notifications
- **[Environment Configuration](docs/environment-configuration.md)** — Deployment environment setup
- **[Contributing Guide](docs/contributing.md)** — How to contribute

## Contributing

See [Contributing Guide](docs/contributing.md) for detailed information.

## License

[Add your license here]
