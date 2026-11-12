/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package storage

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-sqlite3"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage/observe"
)

func TestNewStorage_SQLite(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "factory.db")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	s, err := NewStorage(BackendConfig{Type: "sqlite", SQLitePath: dbPath}, logger)
	if err != nil {
		t.Fatalf("expected sqlite storage, got error: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil storage")
	}
	defer s.Close()
}

func TestNewStorage_Postgres(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN is not set; skipping postgres factory test")
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := NewStorage(BackendConfig{Type: "postgres", Postgres: PostgresConnectionConfig{DSN: dsn}}, logger)
	if err != nil {
		t.Fatalf("expected postgres storage, got error: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil storage")
	}
	defer s.Close()
}

func TestNewStorage_SQLServer(t *testing.T) {
	dsn := os.Getenv("SQLSERVER_TEST_DSN")
	if dsn == "" {
		t.Skip("SQLSERVER_TEST_DSN is not set; skipping sqlserver factory test")
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := NewStorage(BackendConfig{Type: "sqlserver", SQLServer: SQLServerConnectionConfig{DSN: dsn}}, logger)
	if err != nil {
		t.Fatalf("expected sqlserver storage, got error: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil storage")
	}
	defer s.Close()
}

func TestNewStorage_UnsupportedType(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	_, err := NewStorage(BackendConfig{Type: "mysql"}, logger)
	if err == nil {
		t.Fatal("expected error for unsupported storage type")
	}
	if !errors.Is(err, ErrUnsupportedStorageType) {
		t.Fatalf("expected ErrUnsupportedStorageType, got: %v", err)
	}
}

// unreachableBackends returns network backend configurations pointing at a local port
// that nothing listens on, so any connection attempt is refused at once.
func unreachableBackends(t *testing.T) []BackendConfig {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return []BackendConfig{
		{Type: "postgres", Postgres: PostgresConnectionConfig{
			Host: "127.0.0.1", Port: port, Database: "gateway", User: "gateway", Password: "secret",
			SSLMode: "disable", ConnectTimeout: time.Second,
		}},
		{Type: "sqlserver", SQLServer: SQLServerConnectionConfig{
			Host: "127.0.0.1", Port: port, Database: "gateway", User: "gateway", Password: "secret",
			Encrypt: "disable", ConnectTimeout: time.Second,
		}},
	}
}

func TestOpen_NetworkBackendsDoNotConnect(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, cfg := range unreachableBackends(t) {
		t.Run(cfg.Type, func(t *testing.T) {
			s, err := Open(cfg, logger)
			if err != nil {
				t.Fatalf("Open() against an unreachable database error = %v, want nil", err)
			}
			defer s.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := s.GetDB().PingContext(ctx); err == nil {
				t.Fatal("PingContext() succeeded, want an error from the unreachable database")
			}
		})
	}
}

func TestNewStorage_NetworkBackendsRequireTheDatabase(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, cfg := range unreachableBackends(t) {
		t.Run(cfg.Type, func(t *testing.T) {
			_, err := NewStorage(cfg, logger)
			if err == nil || !strings.Contains(err.Error(), "failed to ping") {
				t.Fatalf("NewStorage() error = %v, want a ping failure", err)
			}
		})
	}
}

func TestOpen_SQLite(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	s, err := Open(BackendConfig{Type: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "open.db")}, logger)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer s.Close()
	if err := s.GetDB().PingContext(context.Background()); err != nil {
		t.Fatalf("PingContext() = %v, want a usable SQLite database", err)
	}
}

func TestOpen_UnsupportedType(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	if _, err := Open(BackendConfig{Type: "mysql"}, logger); !errors.Is(err, ErrUnsupportedStorageType) {
		t.Fatalf("Open() error = %v, want ErrUnsupportedStorageType", err)
	}
}

func TestOpen_IncompleteConnectionSettingsFailAtOnce(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, cfg := range []BackendConfig{
		{Type: "postgres", Postgres: PostgresConnectionConfig{Host: "127.0.0.1"}},
		{Type: "sqlserver", SQLServer: SQLServerConnectionConfig{Host: "127.0.0.1"}},
	} {
		t.Run(cfg.Type, func(t *testing.T) {
			for name, open := range map[string]func(BackendConfig, *slog.Logger) (Storage, error){
				"Open": Open, "NewStorage": NewStorage,
			} {
				_, err := open(cfg, logger)
				if err == nil || !strings.Contains(err.Error(), "dsn") {
					t.Fatalf("%s() error = %v, want a configuration error naming the dsn", name, err)
				}
			}
		})
	}
}

func TestOpen_NetworkBackendsRecordFailedConnects(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, cfg := range unreachableBackends(t) {
		t.Run(cfg.Type, func(t *testing.T) {
			cfg.Window = observe.NewWindow(30*time.Second, nil)
			s, err := Open(cfg, logger)
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			defer s.Close()
			if got := cfg.Window.Snapshot(time.Now()).Attempts; got != 0 {
				t.Fatalf("Open() made %d database attempts, want none", got)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := s.GetDB().PingContext(ctx); err == nil {
				t.Fatal("PingContext() succeeded, want an error from the unreachable database")
			}
			snap := cfg.Window.Snapshot(time.Now())
			if snap.Failures == 0 || snap.LastFailure == observe.NoFailure {
				t.Fatalf("window = %+v, want the failed connect counted", snap)
			}
		})
	}
}

func TestOpen_SQLiteThroughTheWindow(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	window := observe.NewWindow(30*time.Second, nil)
	s, err := Open(BackendConfig{Type: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "window.db"), Window: window}, logger)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer s.Close()

	snap := window.Snapshot(time.Now())
	if snap.Attempts == 0 || snap.Failures != 0 || snap.LastActivity.IsZero() {
		t.Fatalf("window after schema setup = %+v, want successful activity", snap)
	}

	// The DSN pragmas still apply on every physical connection opened through the wrapper.
	db := s.GetDB()
	db.SetConnMaxLifetime(time.Nanosecond)
	for i := 0; i < 3; i++ {
		var on int
		if err := db.QueryRow("PRAGMA foreign_keys").Scan(&on); err != nil {
			t.Fatalf("PRAGMA foreign_keys: %v", err)
		}
		if on != 1 {
			t.Fatalf("foreign_keys = %d on connection %d, want 1", on, i)
		}
		time.Sleep(time.Millisecond)
	}

	// A constraint violation reaches the caller unchanged and is not a connection failure.
	_, err = db.Exec("INSERT INTO rest_apis (uuid, gateway_id, configuration) VALUES ('missing-parent', 'gw', '{}')")
	if err == nil {
		t.Fatal("insert violating a foreign key succeeded")
	}
	var liteErr sqlite3.Error
	if !errors.As(err, &liteErr) || liteErr.ExtendedCode != sqlite3.ErrConstraintForeignKey {
		t.Fatalf("error = %v, want a sqlite3.Error foreign key violation", err)
	}
	if got := window.Snapshot(time.Now()).Failures; got != 0 {
		t.Fatalf("window counted %d failures for a constraint violation", got)
	}
}
