package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/scruffyprodigy/playhub/internal/migrate"
)

var DB *sql.DB

// Init initializes the database connection
func Init() error {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL environment variable is required")
	}

	var err error
	DB, err = sql.Open("postgres", databaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}

	// Test the connection
	if err := DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(5)
	DB.SetConnMaxLifetime(5 * time.Minute)
	DB.SetConnMaxIdleTime(30 * time.Second)

	log.Println("Database connection established successfully")
	return nil
}

// InitWithMigrations initializes the database connection and runs migrations
func InitWithMigrations() error {
	if err := Init(); err != nil {
		return err
	}

	// Run migrations on a separate connection. golang-migrate closes the *sql.DB
	// passed to WithInstance when the migrator is closed.
	migrationDB, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		return fmt.Errorf("failed to open migration database connection: %w", err)
	}

	migrator, err := migrate.NewMigrator(migrationDB)
	if err != nil {
		migrationDB.Close()
		return fmt.Errorf("failed to create migrator: %w", err)
	}
	defer func() {
		migrator.Close()
		migrationDB.Close()
	}()

	if err := migrator.Up(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	if err := DB.Ping(); err != nil {
		return fmt.Errorf("database connection unavailable after migrations: %w", err)
	}

	log.Println("Database migrations completed successfully")
	return nil
}

// Close closes the database connection
func Close() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}

// GetDB returns the database connection
func GetDB() *sql.DB {
	return DB
}
