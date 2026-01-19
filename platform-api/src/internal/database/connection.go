/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"platform-api/src/config"

	_ "github.com/lib/pq"           // PostgreSQL driver
	_ "github.com/mattn/go-sqlite3" // SQLite3 driver
)

// DB holds the database connection
type DB struct {
	*sql.DB
	driver string // Database driver name (sqlite3, postgres, etc.)
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
	case "postgres", "postgresql":
		// Build PostgreSQL DSN from config
		dsn := fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name, cfg.SSLMode,
		)

		db, err = sql.Open("postgres", dsn)
		if err != nil {
			return nil, fmt.Errorf("failed to open postgres database: %w", err)
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

	return &DB{DB: db, driver: cfg.Driver}, nil
}

// InitSchema initializes the database schema
// Automatically selects the appropriate schema file based on the database driver.
// If dbSchemaPath is provided (e.g., "./internal/database/schema.sql"), it will
// be used to derive the directory and then select schema.{driver}.sql
func (db *DB) InitSchema(dbSchemaPath string) error {
	var schemaPath string

	// Determine schema file path based on driver
	// Replace "schema.sql" with "schema.{driver}.sql" in the path
	if dbSchemaPath != "" {
		// Extract directory from provided path
		dir := filepath.Dir(dbSchemaPath)

		// Determine driver-specific schema filename
		var schemaFile string
		switch db.driver {
		case "sqlite3":
			schemaFile = "schema.sqlite.sql"
		case "postgres", "postgresql":
			schemaFile = "schema.postgres.sql"
		default:
			return fmt.Errorf("unsupported database driver for schema initialization: %s", db.driver)
		}

		schemaPath = filepath.Join(dir, schemaFile)
	} else {
		// Fallback: construct path from driver
		switch db.driver {
		case "sqlite3":
			schemaPath = "./internal/database/schema.sqlite.sql"
		case "postgres", "postgresql":
			schemaPath = "./internal/database/schema.postgres.sql"
		default:
			return fmt.Errorf("unsupported database driver for schema initialization: %s", db.driver)
		}
	}

	// Read the schema SQL from the external file
	schemaSQL, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)
	}

	// Execute the schema SQL
	_, err = db.Exec(string(schemaSQL))
	if err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	return nil
}

// Rebind converts a SQL query with `?` placeholders to the appropriate format
// for the current database driver. For PostgreSQL, converts `?` to `$1, $2, ...`.
// For SQLite, leaves `?` as-is.
func (db *DB) Rebind(query string) string {
	if db.driver == "postgres" || db.driver == "postgresql" {
		// Convert ? placeholders to $1, $2, $3, etc.
		parts := strings.Split(query, "?")
		if len(parts) == 1 {
			return query // No placeholders
		}

		var result strings.Builder
		for i, part := range parts {
			if i > 0 {
				result.WriteString(fmt.Sprintf("$%d", i))
			}
			result.WriteString(part)
		}
		return result.String()
	}
	// For SQLite and other drivers, return as-is
	return query
}
