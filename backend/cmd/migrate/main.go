package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
	"github.com/scruffyprodigy/joinquest/internal/migrate"
)

func main() {
	var (
		databaseURL = flag.String("database-url", "", "Database connection URL")
		action      = flag.String("action", "up", "Migration action: up, down, steps, version, force")
		steps       = flag.Int("steps", 1, "Number of steps for steps action")
		version     = flag.Int("version", 0, "Version for force action")
	)
	flag.Parse()

	// Get database URL from environment if not provided
	if *databaseURL == "" {
		*databaseURL = os.Getenv("DATABASE_URL")
		if *databaseURL == "" {
			log.Fatal("Database URL is required. Set DATABASE_URL environment variable or use -database-url flag")
		}
	}

	// Connect to database
	db, err := sql.Open("postgres", *databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Create migrator
	migrator, err := migrate.NewMigrator(db)
	if err != nil {
		log.Fatalf("Failed to create migrator: %v", err)
	}
	defer migrator.Close()

	// Execute action
	switch *action {
	case "up":
		if err := migrator.Up(); err != nil {
			log.Fatalf("Failed to run migrations up: %v", err)
		}
	case "down":
		if err := migrator.Down(); err != nil {
			log.Fatalf("Failed to run migrations down: %v", err)
		}
	case "steps":
		if err := migrator.Steps(*steps); err != nil {
			log.Fatalf("Failed to run migration steps: %v", err)
		}
	case "version":
		version, dirty, err := migrator.Version()
		if err != nil {
			log.Fatalf("Failed to get migration version: %v", err)
		}
		if dirty {
			fmt.Printf("Current version: %d (dirty)\n", version)
		} else {
			fmt.Printf("Current version: %d\n", version)
		}
	case "force":
		if *version == 0 {
			log.Fatal("Version is required for force action")
		}
		if err := migrator.Force(*version); err != nil {
			log.Fatalf("Failed to force migration version: %v", err)
		}
	default:
		log.Fatalf("Unknown action: %s. Valid actions: up, down, steps, version, force", *action)
	}
}
