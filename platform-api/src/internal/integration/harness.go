//go:build integration

/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied. See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

// Package integration holds cross-database integration tests for platform-api,
// written as a godog (Cucumber) suite. The scenarios run against a real database
// engine (SQLite, PostgreSQL or SQL Server) selected by the IT_DB environment
// variable and exercise the real schema and data-access behavior — pagination,
// multi-table writes and delete cascades — so backend-specific bugs (e.g. SQL
// Server LIMIT/cascade-path issues) are caught instead of being hidden behind
// the SQLite unit-test path.
//
// Build-tagged `integration` so it is excluded from the default `go test ./...`.
//

package integration

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/google/uuid"

	"platform-api/src/config"
	"platform-api/src/internal/database"
)

// itDB describes the database engine under test for a single scenario.
type itDB struct {
	driver  string
	db      *database.DB
	cleanup func() // closes the connection and removes any temp files
}

// openITDB opens the database selected by IT_DB (default: sqlite) and applies
// the matching schema. Supported values: sqlite, postgres, sqlserver. The caller
// must invoke the returned itDB's cleanup when the scenario finishes.
func openITDB() (*itDB, error) {
	driver := envOr("IT_DB", "sqlite")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	var cfg *config.Database
	cleanup := func() {}
	switch driver {
	case "sqlite", "sqlite3":
		dir, err := os.MkdirTemp("", "platform-api-it")
		if err != nil {
			return nil, fmt.Errorf("create sqlite temp dir: %w", err)
		}
		cleanup = func() { os.RemoveAll(dir) }
		cfg = &config.Database{Driver: "sqlite3", Path: dir + "/it.db", MaxOpenConns: 1, MaxIdleConns: 1}
		driver = "sqlite"
	case "postgres", "postgresql":
		cfg = &config.Database{
			Driver:       "postgres",
			Host:         envOr("IT_DB_HOST", "localhost"),
			Port:         atoiOr("IT_DB_PORT", 5432),
			Name:         envOr("IT_DB_NAME", "platform_api_it"),
			User:         envOr("IT_DB_USER", "postgres"),
			Password:     os.Getenv("IT_DB_PASSWORD"),
			SSLMode:      envOr("IT_DB_SSLMODE", "disable"),
			MaxOpenConns: 10, MaxIdleConns: 5,
		}
		driver = "postgres"
	case "sqlserver", "mssql":
		name := envOr("IT_DB_NAME", "platform_api_it")
		if err := ensureSQLServerDB(logger, name); err != nil {
			return nil, err
		}
		cfg = &config.Database{
			Driver:       "sqlserver",
			Host:         envOr("IT_DB_HOST", "localhost"),
			Port:         atoiOr("IT_DB_PORT", 1433),
			Name:         name,
			User:         envOr("IT_DB_USER", "sa"),
			Password:     os.Getenv("IT_DB_PASSWORD"),
			SSLMode:      envOr("IT_DB_SSLMODE", "disable"),
			MaxOpenConns: 10, MaxIdleConns: 5,
		}
		driver = "sqlserver"
	default:
		cleanup()
		return nil, fmt.Errorf("unsupported IT_DB %q (want sqlite|postgres|sqlserver)", driver)
	}

	db, err := connectITDB(cfg, logger)
	if err != nil {
		cleanup()
		return nil, err
	}
	if err := db.InitSchema("../database/schema.sql", logger); err != nil {
		db.Close()
		cleanup()
		return nil, fmt.Errorf("InitSchema (%s) failed: %w", driver, err)
	}
	return &itDB{
		driver:  driver,
		db:      db,
		cleanup: func() { db.Close(); cleanup() },
	}, nil
}

func connectITDB(cfg *config.Database, logger *slog.Logger) (*database.DB, error) {
	deadline := time.Now().Add(90 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		db, err := database.NewConnection(cfg, logger)
		if err == nil {
			return db, nil
		}
		lastErr = err
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("could not connect to %s within timeout: %w", cfg.Driver, lastErr)
}

// validDBName guards the database name that is interpolated into the
// CREATE DATABASE statement below (identifiers cannot be bound as parameters).
// The name comes from IT_DB_NAME; restrict it to a safe identifier charset.
var validDBName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,62}$`)

// ensureSQLServerDB drops and recreates the SQL Server test database (via master).
// A fresh database on every test run prevents stale schema from causing FK failures
// when constraints are added to existing tables between runs.
func ensureSQLServerDB(logger *slog.Logger, name string) error {
	if !validDBName.MatchString(name) {
		return fmt.Errorf("invalid IT_DB_NAME %q: must match %s", name, validDBName.String())
	}
	master, err := connectITDB(&config.Database{
		Driver: "sqlserver", Host: envOr("IT_DB_HOST", "localhost"), Port: atoiOr("IT_DB_PORT", 1433),
		Name: "master", User: envOr("IT_DB_USER", "sa"), Password: os.Getenv("IT_DB_PASSWORD"),
		SSLMode: "disable", MaxOpenConns: 2, MaxIdleConns: 1,
	}, logger)
	if err != nil {
		return err
	}
	defer master.Close()
	// Terminate open connections before dropping so the DROP succeeds.
	if _, err := master.Exec(fmt.Sprintf(
		"IF DB_ID(N'%s') IS NOT NULL ALTER DATABASE [%s] SET SINGLE_USER WITH ROLLBACK IMMEDIATE",
		name, name)); err != nil {
		return fmt.Errorf("failed to set sqlserver database %q to single-user: %w", name, err)
	}
	if _, err := master.Exec(fmt.Sprintf(
		"IF DB_ID(N'%s') IS NOT NULL DROP DATABASE [%s]", name, name)); err != nil {
		return fmt.Errorf("failed to drop sqlserver database %q: %w", name, err)
	}
	if _, err := master.Exec(fmt.Sprintf("CREATE DATABASE [%s]", name)); err != nil {
		return fmt.Errorf("failed to create sqlserver database %q: %w", name, err)
	}
	return nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func atoiOr(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// count returns the number of rows in table matching the column filter.
func (it *itDB) count(table, col, val string) (int, error) {
	var n int
	q := it.db.Rebind(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = ?", table, col))
	if err := it.db.QueryRow(q, val).Scan(&n); err != nil {
		return 0, fmt.Errorf("count(%s) failed on %s: %w", table, it.driver, err)
	}
	return n, nil
}

// exec runs a `?`-placeholder statement, rebinding for the active driver.
func (it *itDB) exec(query string, args ...any) error {
	if _, err := it.db.Exec(it.db.Rebind(query), args...); err != nil {
		return fmt.Errorf("exec failed on %s: %w\nquery: %s", it.driver, err, query)
	}
	return nil
}

func id() string { return uuid.NewString() }
