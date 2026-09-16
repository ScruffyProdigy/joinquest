# Architecture Overview

JoinQuest is the **player-facing platform** and **integration layer** for third-party web games: accounts, catalog, queues, matchmaking, and provision handoff. Game authors keep their own browser origin and game servers; we connect players via game-minted launch URLs and a stable seat contract.

**Why this exists:** [Product vision](./vision.md).

## Platform vs game responsibilities

| JoinQuest (this repo) | Third-party game |
|----------------------|------------------|
| Sign-in, sessions, player profile | Game rules, rendering, realtime play |
| Catalog, queues, seat assignment (LFG) | Seat manifest, accept/reject roster |
| `POST` provision + JWT link-out | Claim seat, run match |
| Shared commerce / inventory (roadmap) | Optional own storefront |

```text
Browser (JoinQuest UI) ──GraphQL──► Go API ──► PostgreSQL / Redis
                                        │
                                        ├── provision ──► Game API (your origin)
                                        └── redirect ──► Game launch URL + seat JWT
```

## System Architecture

### Development Environment
```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   React Frontend │    │  Go GraphQL API │    │   PostgreSQL    │
│   (Port 5173)   │◄──►│   (Port 8080)   │◄──►│   (Port 5432)   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Vite Dev      │    │   gqlgen        │    │   Database      │
│   Server        │    │   Code Gen      │    │   Migrations    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### Production Environment (Kubernetes)
```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   React Frontend │    │  Go GraphQL API │    │   PostgreSQL    │
│   (Nginx)       │◄──►│   (Container)   │◄──►│   (Container)   │
│   Port 80       │    │   Port 8080     │    │   Port 5432     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Environment   │    │   Kubernetes    │    │   Persistent    │
│   ConfigMaps    │    │   Services      │    │   Volumes       │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## Component Overview

### Frontend (React + Vite) ✅
- **Technology**: React 19, Vite, Vitest, Playwright
- **Purpose**: User interface for gaming lobby
- **Status**: Foundation implemented with testing infrastructure
- **Features**: Basic UI, environment configuration, comprehensive testing
- **Port**: 5173 (development), 80 (production)

### Backend (Go + GraphQL) ✅
- **Technology**: Go 1.25, gqlgen, GraphQL
- **Purpose**: API server and business logic
- **Status**: Catalog matchmaking, auth, handoff, and PostgreSQL persistence
- **Features**: `registerGame`, queue-scoped `joinQueue`, provision to game APIs, Redis subscriptions
- **Port**: 8080

### Database (PostgreSQL) ✅
- **Technology**: PostgreSQL
- **Purpose**: Data persistence
- **Status**: Migrations, users, games, sessions, queues, digital goods

## Data Flow

### User Authentication Flow
```
User → Frontend → GraphQL API → Database
     ← JWT Token ←
```

### Game Queue Flow
```
User → Join Queue → GraphQL API → Database
     ← Queue Status ←
```

### Trading Flow
```
User → Trade Request → GraphQL API → Database
     ← Trade Confirmation ←
```

## API Design

### GraphQL Schema Structure
```
Query {
  version: String!
  healthz: String!
  me: User
  games(limit: Int, offset: Int): [Game!]!
  game(id: ID!): Game
  session(id: ID!): Session
}

Mutation {
  registerGame(input: RegisterGameInput!): RegisterGamePayload!
  joinQueue(queueId: ID!): JoinResult!
  leaveQueue(queueId: ID!): Boolean!
}
```

## Security

### Authentication
- JWT-based authentication
- Secure token storage
- Session management

### Authorization
- Role-based access control
- Resource-level permissions
- API rate limiting

## Deployment

### Development
- Local development servers
- Hot reloading enabled
- Test databases

### Production
- Kubernetes deployment
- Containerized services
- Load balancing
- SSL/TLS termination

## Monitoring & Observability

### Logging
- Structured logging (JSON)
- Log levels (DEBUG, INFO, WARN, ERROR)
- Request tracing

### Metrics
- Application metrics
- Performance monitoring
- Error tracking

### Health Checks
- `/healthz` endpoint
- Database connectivity
- External service status

## Environment Configuration System

JoinQuest uses a Docker-based environment configuration system that allows the same Docker image to work across different environments.

### How It Works

1. **Docker Entrypoint Script**: The frontend Docker image includes a script that generates `env.js` at runtime
2. **Kubernetes ConfigMaps**: Each environment has its own ConfigMap with environment-specific values
3. **Runtime Injection**: When the container starts, it reads environment variables and creates the `env.js` file
4. **Frontend Access**: The frontend loads `/env.js` and accesses variables via `window.env`

### Environment Configurations

#### Local Development
- **Frontend**: `http://localhost:5173`
- **GraphQL**: `http://localhost:8080/graphql`
- **Start**: `./scripts/dev.sh`

#### Production (JoinQuest)
- **Public URL**: `https://joinquest.cc`
- **GraphQL**: `https://joinquest.cc/graphql`
- **Deploy**: `./scripts/deploy-joinquest.sh`

### Benefits

- **Single Docker Image**: Same image works in all environments
- **Runtime Configuration**: No need to rebuild for different environments
- **Kubernetes Native**: Uses ConfigMaps for environment-specific values
- **Secure**: Sensitive values can be stored in Secrets
- **Easy Deployment**: Simple scripts for each environment

## Scalability Considerations

### Horizontal Scaling
- Stateless API design
- Database connection pooling
- Load balancer ready

### Performance
- GraphQL query optimization
- Database indexing
- Caching strategies

### Future Enhancements
- Microservices architecture
- Event-driven communication
- Message queues
- Caching layers
