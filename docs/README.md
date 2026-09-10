# JoinQuest Documentation

Documentation for **JoinQuest** — the platform that connects players to third-party web games and handles lobby, auth, and (roadmap) commerce so developers can focus on their game.

**New here?** Read **[Product vision](vision.md)** first.

> **Design precedence.** The Figma Make prototype published to
> [demo.joinquest.cc](https://demo.joinquest.cc) is the **newer standard**. Where a document
> in this folder disagrees with the prototype on product or UX design, the prototype wins and
> the document is stale. Design docs here record earlier thinking; they remain authoritative
> for shipped API contracts and implementation detail, not for design intent.
>
> **Plans and specs do not live here.** They are working artefacts and belong in Notion
> (JoinQuest HQ). `docs/superpowers/` is gitignored. Markdown that helps a new engineer get
> productive does belong in this folder, and should be kept current.

## Who should read what

| You are… | Start with |
|----------|------------|
| Evaluating or partnering on a game | [vision.md](vision.md) → [lobby-protocol-handoff.md](lobby-protocol-handoff.md) |
| Implementing game integration | [lobby-protocol-handoff.md](lobby-protocol-handoff.md), [game-catalog-architecture.md](game-catalog-architecture.md), [seat-templates-and-matchmaking.md](seat-templates-and-matchmaking.md) |
| Hacking on this repo | [../AGENTS.md](../AGENTS.md), [lobby-maintenance.md](lobby-maintenance.md), [development.md](development.md), [architecture.md](architecture.md) |

## Documentation structure

### Product & integration
- **[Product vision](vision.md)** — Problem, promise, mental model, principles
- **[End-to-end partner checklist](end-to-end-partner-checklist.md)** — Sign-in → queue → play → return
- **[Match lifecycle callbacks](match-lifecycle-callbacks.md)** — Per-player finish + match result (planned)
- **[Composition & join options](composition-and-join-options.md)** — Role queues vs game-backed eligibility
- **[Rooms & tables](rooms-and-tables.md)** — Private rooms, forming tables, invite/QR, queue exclusion
- **[Spirit animal avatars](spirit-animal-avatars.md)** — Shipped: starter avatars + tarot-guided “Find my spirit animal” flow; optional `avatarKey` on profile update
- **[Game-minted launch URLs](game-minted-launch-urls.md)** — Games return launch URL bases on provision; Lobby attaches JWT
- **[Developer self-service](developer-self-service.md)** — Register games, integration dashboard (19 checks), private testing, public release, MCP
- **[Developer AI setup roadmap](developer-ai-setup-roadmap.md)** — MCP npm, dashboard copy buttons, unified skill+MCP install
- **[Player experience roadmap](player-experience-roadmap.md)** — Planned: queue wait estimates, bottleneck CTAs, richer game cards
- **[Lobby ↔ Game Handoff](lobby-protocol-handoff.md)** — Provision, JWT, and integration rationale
- **[Game Catalog Architecture](game-catalog-architecture.md)** — Modes, queues, manifest sync
- **[Seat templates & LFG](seat-templates-and-matchmaking.md)** — Seat map contract and matchmaking spec

### Getting started (contributors)
- **[AGENTS.md](../AGENTS.md)** — Agent/maintainer quick reference
- **[Lobby maintenance runbook](lobby-maintenance.md)** — Deploy, doc sync, MCP publish
- **[Development Guide](development.md)** — Local setup and day-to-day workflow
- **[Environment Configuration](environment-configuration.md)** — Runtime env injection (local, k8s)
- **[Testing Guide](testing.md)** — Backend, frontend, and E2E tests

### Architecture & API
- **[Architecture Overview](architecture.md)** — System design and components
- **[API Documentation](api.md)** — GraphQL reference and examples
- **[Pub/Sub](pubsub.md)** — Redis queue notifications
- **[Rating backtesting](rating-backtesting.md)** — Scoring rating models against held-out match history
- **[Cold-start seeding](cold-start-seeding.md)** — Seeding a player's rating in an unplayed mode from another mode of the same game

### Operations
- **[Database Migrations](database-migrations.md)** — Schema migrations and CLI
- **[Contributing Guide](contributing.md)** — How to contribute

## Quick start

1. **Prerequisites**: Go 1.25+, Node.js 20+, Docker
2. **Clone**: `git clone https://github.com/scruffyprodigy/joinquest.git` (folder may be `lobby` locally)
3. **Setup**: `./scripts/setup.sh` from the project root
4. **Start**: `./scripts/dev.sh` (frontend `:5173`, GraphQL `:8080/graphql`)

## Project structure

```text
lobby/ (joinquest on GitHub)  # JoinQuest platform — repo root
├── AGENTS.md            # Maintainer / AI agent guide
├── backend/             # Go GraphQL API (Lobby / JoinQuest server)
├── frontend/            # React + Vite — player-facing JoinQuest UI
├── k8s/                 # Kubernetes manifests
├── docs/                # This documentation
├── scripts/             # Dev, test, and deploy scripts
└── .github/workflows/   # CI/CD
```

## External links

- [GraphQL Schema](../backend/graph/schema/)
- [Frontend source](../frontend/src/)
- [Kubernetes configs](../k8s/)
- [GitHub Actions](../.github/workflows/)
