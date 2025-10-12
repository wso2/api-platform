package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite3 driver
	"platform-api/src/config"
)

// DB holds the database connection
type DB struct {
	*sql.DB
}

// NewConnection creates a new database connection using configuration
func NewConnection(cfg *config.Database) (*DB, error) {
	var db *sql.DB
	var err error

	switch cfg.Driver {
	case "sqlite3":
		// Ensure the directory exists for SQLite
		dir := filepath.Dir(cfg.Path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}

		// Open SQLite connection to the api_platform.db file
		db, err = sql.Open("sqlite3", cfg.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to open database: %w", err)
		}

		// Enable foreign key constraints for SQLite
		if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
			return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{db}, nil
}

// InitSchema initializes the database schema
func (db *DB) InitSchema() error {
	// Read the schema SQL from the external file
	schemaPath := filepath.Join("internal", "database", "schema.sql")
	schemaSQL, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	// Execute the schema SQL
	_, err = db.Exec(string(schemaSQL))
	if err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	return nil
}
