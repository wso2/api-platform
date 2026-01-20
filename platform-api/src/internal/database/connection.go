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
	"regexp"
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

// Driver returns the underlying database driver name (e.g., sqlite3, postgres).
func (db *DB) Driver() string {
	return db.driver
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

	// For PostgreSQL, we need to execute statements individually
	// because PostgreSQL driver doesn't handle multi-statement Exec() well
	if db.driver == "postgres" || db.driver == "postgresql" {
		return db.initSchemaPostgres(string(schemaSQL))
	}

	// For SQLite, execute as a single statement (it handles multi-statement well)
	_, err = db.Exec(string(schemaSQL))
	if err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	return nil
}

// initSchemaPostgres splits SQL statements and executes them individually within a transaction
// This ensures all tables are created before foreign key constraints are validated
func (db *DB) initSchemaPostgres(schemaSQL string) error {
	// Split SQL statements by semicolon, but be careful with semicolons in strings/comments
	statements := splitSQLStatements(schemaSQL)

	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute each statement individually within the transaction
	executedCount := 0
	for i, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		// Skip pure comment statements (but keep statements that contain comments)
		// A pure comment statement would start with -- and have no actual SQL
		if strings.HasPrefix(stmt, "--") && !strings.Contains(strings.ToUpper(stmt), "CREATE") && !strings.Contains(strings.ToUpper(stmt), "ALTER") && !strings.Contains(strings.ToUpper(stmt), "DROP") && !strings.Contains(strings.ToUpper(stmt), "INSERT") && !strings.Contains(strings.ToUpper(stmt), "UPDATE") && !strings.Contains(strings.ToUpper(stmt), "DELETE") && !strings.Contains(strings.ToUpper(stmt), "SELECT") {
			continue
		}
		if strings.HasPrefix(stmt, "/*") {
			continue
		}

		executedCount++
		_, err := tx.Exec(stmt)
		if err != nil {
			// Show more context about what we're executing
			firstLine := stmt
			if idx := strings.Index(stmt, "\n"); idx > 0 {
				firstLine = stmt[:idx]
			}
			return fmt.Errorf("failed to execute schema statement %d (executed %d/%d): %w\nFirst line: %s\nStatement preview: %s", i+1, executedCount, len(statements), err, firstLine, stmt[:min(len(stmt), 300)])
		}
	}

	// Commit the transaction - this is when foreign keys are validated
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit schema transaction: %w", err)
	}

	return nil
}

// splitSQLStatements splits SQL by semicolons, handling comments and strings
// Properly handles multi-line statements like CREATE TABLE
func splitSQLStatements(sql string) []string {
	// Remove block comments /* ... */ (multiline, non-greedy)
	blockCommentRe := regexp.MustCompile(`(?s)/\*.*?\*/`)
	sql = blockCommentRe.ReplaceAllString(sql, "\n")

	var statements []string
	current := strings.Builder{}
	inString := false
	escapeNext := false

	// Process character by character to properly handle strings and comments
	for _, r := range sql {
		if escapeNext {
			current.WriteRune(r)
			escapeNext = false
			continue
		}

		// Handle escape sequences
		if r == '\\' {
			escapeNext = true
			current.WriteRune(r)
			continue
		}

		// Track string literals - everything inside single quotes is literal
		if r == '\'' {
			inString = !inString
			current.WriteRune(r)
			continue
		}

		// Only split on semicolons that are outside of string literals
		if !inString && r == ';' {
			stmt := strings.TrimSpace(current.String())
			// Only add non-empty statements that aren't pure comments
			if stmt != "" {
				// Remove leading line comments but keep the statement
				stmt = removeLeadingComments(stmt)
				if stmt != "" {
					statements = append(statements, stmt)
				}
			}
			current.Reset()
			continue
		}

		current.WriteRune(r)
	}

	// Handle remaining statement (if any)
	if remaining := strings.TrimSpace(current.String()); remaining != "" {
		remaining = removeLeadingComments(remaining)
		if remaining != "" {
			statements = append(statements, remaining)
		}
	}

	return statements
}

// removeLeadingComments removes leading comment lines from a statement
// but preserves inline comments within the statement
func removeLeadingComments(stmt string) string {
	lines := strings.Split(stmt, "\n")
	var cleanedLines []string
	inStatement := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip pure comment lines
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			// Only skip if we haven't started the actual statement yet
			if !inStatement {
				continue
			}
			// If we're in a statement, preserve the line (it might be part of multi-line formatting)
			cleanedLines = append(cleanedLines, line)
			continue
		}

		// We've found non-comment content
		inStatement = true

		// Handle inline comments within the line
		if idx := strings.Index(line, "--"); idx >= 0 {
			beforeComment := line[:idx]
			// Only remove comment if it's not inside a string
			if strings.Count(beforeComment, "'")%2 == 0 {
				trimmedBefore := strings.TrimRight(beforeComment, " \t")
				if trimmedBefore != "" {
					cleanedLines = append(cleanedLines, trimmedBefore)
				}
				continue
			}
		}

		cleanedLines = append(cleanedLines, line)
	}

	result := strings.Join(cleanedLines, "\n")
	return strings.TrimSpace(result)
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
