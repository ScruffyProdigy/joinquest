# Database Migrations

Schema migrations for JoinQuest (users, games, queues, sessions, goods — the platform data model behind [vision](./vision.md)).

## Overview

JoinQuest uses a custom migration system built on top of [golang-migrate](https://github.com/golang-migrate/migrate) to manage database schema changes. The system supports both programmatic and CLI-based migration management.

## Migration Files

Migrations are stored in the `backend/migrations/` directory and follow the naming convention:
- `{version}_{description}.up.sql` - Migration to apply changes
- `{version}_{description}.down.sql` - Migration to rollback changes

### Current Migrations

- `000001_initial_schema.up.sql` - Creates the initial database schema including:
  - Users table with magic link authentication support
  - Magic links table for secure email-based login
  - Games table for available games
  - Game queues table for matchmaking
  - Game sessions table for active games
  - Digital goods table for trading system
  - User inventory table for owned items
- `000002_provisional_display_names.up.sql` - Provisional display names for new users
- `000003_demo_games.up.sql` - Seeds demo games for local/staging
- `000034_developer_self_service.up.sql` - Developer ownership, visibility states, contact URLs, integration check results
- `000035_drop_play_url.up.sql` - Remove catalog `play_url`; launch URLs come from provision only
- `000005_queue_one_waiting_per_user.up.sql` - One active waiting queue entry per user (per game; superseded by 000012)
- `000012_one_waiting_queue_global.up.sql` - One waiting queue row per user across all games
- `000006_magic_link_login_code.up.sql` - Adds `code_hash` on `magic_links` for 6-digit email codes
- `000007_game_catalog.up.sql` - Game catalog: manifest cache on `games`, `game_modes`, `game_mode_seats`, `mode_queues`, and `game_queues.mode_queue_id`
- `000008_mode_queue_matchmaking.up.sql` - Session `mode_id`/`mode_queue_id`, unique waiting row per user per mode queue
- `000009_catalog_only_games.up.sql` - Seeds catalog mode/queue for RPS demo; deactivates Party Lobby
- `000010_drop_legacy_game_fields.up.sql` - Drops legacy `games.game_mode`/`min_players`/`max_players` and `game_modes.best_of`
- `000011_security_hardening.up.sql` - Magic-link token hashing; failed login-code attempt tracking
- `000012_one_waiting_queue_global.up.sql` - One waiting queue row per user across all games
- `000013_player_return_and_match_lifecycle.up.sql` - `return_context` on session participants; match lifecycle timestamps
- `000014_seat_templates.up.sql` - `seat_template` JSONB on modes; affinity/queue metadata on expanded seats
- `000015_queue_path_join.up.sql` - `queue_path` on waiting `game_queues` rows
- `000016_rooms.up.sql` - Chat rooms, members, messages (Step 1)
- `000017_tables.up.sql` - Forming tables and `table_seats` inside rooms (Step 2)
- `000018_avatars.up.sql` - `avatar_key`, `avatar_source` on users
- `000019_spirit_animal_readings.up.sql` - `avatar_readings` table for spirit-animal flow
- `000020_avatar_readings_phase_started.up.sql` - `phase_started_at` on avatar readings
- `000021_forming_matches.up.sql` - LFG Phase B: parties, forming matches, queue linkage
- `000022_party_tree.up.sql` - `party_tree` JSONB on parties
- `000023_lfg_cleanup.up.sql` - Drop deprecated `parties.together`; forming match status cleanup
- `000024_session_launch_urls.up.sql` - `launch_url_base` on `game_session_participants` for stored game-minted URL bases

## CLI Usage

The migration CLI tool is located at `backend/cmd/migrate/main.go` and can be used with the following commands:

### Using Make Commands

```bash
# Run all pending migrations
make migrate-up

# Rollback the last migration
make migrate-down

# Check current migration version
make migrate-version

# Force a specific migration version (use with caution)
make migrate-force VERSION=1
```

### Direct CLI Usage

```bash
# Run migrations up
go run ./cmd/migrate -action=up

# Rollback last migration
go run ./cmd/migrate -action=down

# Run specific number of steps
go run ./cmd/migrate -action=steps -steps=2

# Check current version
go run ./cmd/migrate -action=version

# Force version (use with caution)
go run ./cmd/migrate -action=force -version=1
```

## Programmatic Usage

The migration system can also be used programmatically:

```go
import "github.com/scruffyprodigy/playhub/internal/migrate"

// Create migrator
migrator, err := migrate.NewMigrator(db)
if err != nil {
    return err
}
defer migrator.Close()

// Run all pending migrations
if err := migrator.Up(); err != nil {
    return err
}
```

## Database Schema

### Users Table
- `id` - UUID primary key
- `email` - Unique email address
- `username` - Unique username
- `display_name` - User's display name
- `avatar_url` - Optional avatar image URL
- `is_active` - Whether the user account is active
- `is_verified` - Whether the user has verified their email
- `last_login_at` - Timestamp of last login
- `created_at` - Account creation timestamp
- `updated_at` - Last update timestamp

### Magic Links Table
- `id` - UUID primary key
- `user_id` - Foreign key to users table (nullable for new users)
- `email` - Email address for the magic link
- `token` - Unique token for the magic link
- `code_hash` - SHA-256 hash of the optional 6-digit email sign-in code (nullable on older rows)
- `expires_at` - Expiration timestamp
- `used_at` - When the link was used (nullable)
- `created_at` - Creation timestamp

### Games Table
- `id` - UUID primary key
- `name` - Game name
- `description` - Game description
- `version` - Game version
- `min_players` - Minimum players required
- `max_players` - Maximum players allowed
- `estimated_duration_minutes` - Estimated game duration
- `category` - Game category
- `status` - Game status (active, inactive, maintenance)
- `created_at` - Creation timestamp
- `updated_at` - Last update timestamp

### Game Queues Table
- `id` - UUID primary key
- `game_id` - Foreign key to games table
- `user_id` - Foreign key to users table
- `status` - Queue status (waiting, matched, cancelled, expired)
- `priority` - Queue priority
- `preferences` - JSON preferences
- `joined_at` - When user joined queue
- `matched_at` - When user was matched (nullable)
- `expires_at` - Queue expiration (nullable)

### Game Sessions Table
- `id` - UUID primary key
- `game_id` - Foreign key to games table
- `status` - Session status (active, completed, cancelled)
- `started_at` - Session start time
- `ended_at` - Session end time (nullable)
- `session_data` - JSON session data

### Game Session Participants Table
- `id` - UUID primary key
- `session_id` - Foreign key to game_sessions table
- `user_id` - Foreign key to users table
- `joined_at` - When user joined session
- `left_at` - When user left session (nullable)
- `role` - User's role in the session

### Digital Goods Table
- `id` - UUID primary key
- `name` - Item name
- `description` - Item description
- `category` - Item category
- `rarity` - Item rarity (common, uncommon, rare, epic, legendary)
- `game_id` - Foreign key to games table
- `is_tradeable` - Whether item can be traded
- `created_at` - Creation timestamp
- `updated_at` - Last update timestamp

### User Inventory Table
- `id` - UUID primary key
- `user_id` - Foreign key to users table
- `good_id` - Foreign key to digital_goods table
- `quantity` - Number of items owned
- `acquired_at` - When item was acquired

## Environment Variables

- `DATABASE_URL` - PostgreSQL connection string (required)

## Kubernetes Integration

The migration system is integrated with Kubernetes through the `k8s/jobs/migration.yaml` job, which runs migrations automatically during deployment.

## Testing

Migration tests are located in `backend/internal/migrate/migrate_test.go` and can be run with:

```bash
go test ./internal/migrate -v
```

Integration tests use a separate database (`joinquest_test`) so they never touch dev data:

```bash
./scripts/db.sh test-migrate          # create DB + run migrations
export DATABASE_URL="$(./scripts/db.sh test-url)"
cd backend && go test ./...
```

Or run `./scripts/test-backend.sh` / `./scripts/test.sh`, which set this automatically.

`go test` against `joinquest` is blocked unless `ALLOW_TESTS_ON_DEV_DB=1` is set.

## Best Practices

1. **Always create both up and down migrations** - Every migration should be reversible
2. **Test migrations thoroughly** - Test both up and down migrations in development
3. **Use transactions** - Wrap complex migrations in transactions when possible
4. **Backup before major changes** - Always backup production data before running migrations
5. **Version control** - Keep migration files in version control
6. **Sequential numbering** - Use sequential version numbers for migrations
7. **Descriptive names** - Use clear, descriptive names for migration files

## Troubleshooting

### Migration Stuck in "Dirty" State
If a migration fails and leaves the database in a dirty state:

```bash
# Check current version and dirty status
make migrate-version

# Force the version to the last successful migration
make migrate-force VERSION=1
```

### Connection Issues
Ensure the `DATABASE_URL` environment variable is set correctly:

```bash
export DATABASE_URL="postgres://user:password@host:port/database?sslmode=disable"
```

### Permission Issues
Ensure the database user has sufficient permissions to create tables, indexes, and manage the schema_migrations table.
