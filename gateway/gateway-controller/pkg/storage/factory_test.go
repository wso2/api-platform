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
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
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
