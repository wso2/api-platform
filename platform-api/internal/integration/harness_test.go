//go:build integration

/*
 *  Copyright (c) 2026, WSO2 LLC.
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

// Package integration holds cross-database integration tests for platform-api.
// They run against a real database engine (SQLite, PostgreSQL or SQL Server)
// selected by the IT_DB environment variable and exercise the real schema and
// data-access behavior — pagination, multi-table writes and delete cascades —
// so backend-specific bugs (e.g. SQL Server LIMIT/cascade-path issues) are
// caught instead of being hidden behind the SQLite unit-test path.
//
//
//go:build integration

package integration

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/database"
)

func TestMain(m *testing.M) {
	// Allow GetConfig() to auto-provision an encryption key so tests that exercise
	// subscription_repository.go don't fail at startup.
	os.Setenv("APIP_DEMO_MODE", "true")
	os.Exit(m.Run())
}

// itDB describes the database engine under test.
type itDB struct {
	driver string
	db     *database.DB
}

// openITDB opens the database selected by IT_DB (default: sqlite) and applies
// the matching schema. Supported values: sqlite, postgres, sqlserver.
func openITDB(t *testing.T) *itDB {
	t.Helper()
	driver := envOr("IT_DB", "sqlite")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	var cfg *config.Database
	switch driver {
	case "sqlite", "sqlite3":
		dir := t.TempDir()
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
		ensureSQLServerDB(t, logger, name)
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
		t.Fatalf("unsupported IT_DB %q (want sqlite|postgres|sqlserver)", driver)
	}

	db := connectITDB(t, cfg, logger)
	t.Cleanup(func() { db.Close() })
	if err := db.InitSchema("../database/schema.sql", logger); err != nil {
		t.Fatalf("InitSchema (%s) failed: %v", driver, err)
	}
	return &itDB{driver: driver, db: db}
}

func connectITDB(t *testing.T, cfg *config.Database, logger *slog.Logger) *database.DB {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		db, err := database.NewConnection(cfg, logger)
		if err == nil {
			return db
		}
		lastErr = err
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("could not connect to %s within timeout: %v", cfg.Driver, lastErr)
	return nil
}

// validDBName guards the database name that is interpolated into the
// CREATE DATABASE statement below (identifiers cannot be bound as parameters).
// The name comes from IT_DB_NAME; restrict it to a safe identifier charset.
var validDBName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,62}$`)

// ensureSQLServerDB drops and recreates the SQL Server test database (via master).
// A fresh database on every test run prevents stale schema from causing FK failures
// when constraints are added to existing tables between runs.
func ensureSQLServerDB(t *testing.T, logger *slog.Logger, name string) {
	t.Helper()
	if !validDBName.MatchString(name) {
		t.Fatalf("invalid IT_DB_NAME %q: must match %s", name, validDBName.String())
	}
	master := connectITDB(t, &config.Database{
		Driver: "sqlserver", Host: envOr("IT_DB_HOST", "localhost"), Port: atoiOr("IT_DB_PORT", 1433),
		Name: "master", User: envOr("IT_DB_USER", "sa"), Password: os.Getenv("IT_DB_PASSWORD"),
		SSLMode: "disable", MaxOpenConns: 2, MaxIdleConns: 1,
	}, logger)
	defer master.Close()
	// Terminate open connections before dropping so the DROP succeeds.
	if _, err := master.Exec(fmt.Sprintf(
		"IF DB_ID(N'%s') IS NOT NULL ALTER DATABASE [%s] SET SINGLE_USER WITH ROLLBACK IMMEDIATE",
		name, name)); err != nil {
		t.Fatalf("failed to set sqlserver database %q to single-user: %v", name, err)
	}
	if _, err := master.Exec(fmt.Sprintf(
		"IF DB_ID(N'%s') IS NOT NULL DROP DATABASE [%s]", name, name)); err != nil {
		t.Fatalf("failed to drop sqlserver database %q: %v", name, err)
	}
	if _, err := master.Exec(fmt.Sprintf("CREATE DATABASE [%s]", name)); err != nil {
		t.Fatalf("failed to create sqlserver database %q: %v", name, err)
	}
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

// count returns the number of rows in table matching the org filter.
func (it *itDB) count(t *testing.T, table, col, val string) int {
	t.Helper()
	var n int
	q := it.db.Rebind(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = ?", table, col))
	if err := it.db.QueryRow(q, val).Scan(&n); err != nil {
		t.Fatalf("count(%s) failed: %v", table, err)
	}
	return n
}

// exec runs a `?`-placeholder statement, rebinding for the active driver.
func (it *itDB) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := it.db.Exec(it.db.Rebind(query), args...); err != nil {
		t.Fatalf("exec failed on %s: %v\nquery: %s", it.driver, err, query)
	}
}
