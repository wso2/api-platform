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
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

//go:embed gateway-controller-db.postgres.sql
var postgresSchemaSQL string

const (
	postgresSchemaVersion = 1
	postgresSchemaLockID  = int64(749251473)
	pgUniqueViolationCode = "23505"
)

// PostgresConnectionConfig holds PostgreSQL-specific connection settings.
type PostgresConnectionConfig struct {
	DSN             string
	Host            string
	Port            int
	Database        string
	User            string
	Password        string
	SSLMode         string
	ConnectTimeout  time.Duration
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	ApplicationName string
}

// PostgresStorage implements the Storage interface using PostgreSQL.
type PostgresStorage struct {
	db     *sql.DB
	logger *slog.Logger
}

// newPostgresStorage creates a new PostgreSQL storage instance.
func newPostgresStorage(cfg PostgresConnectionConfig, logger *slog.Logger) (*PostgresStorage, error) {
	cfg = withDefaultPostgresConfig(cfg)
	dsn, err := buildPostgresDSN(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to build postgres dsn: %w", err)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres database: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	storage := &PostgresStorage{
		db:     db,
		logger: logger,
	}

	pingTimeout := cfg.ConnectTimeout
	if pingTimeout <= 0 {
		pingTimeout = 5 * time.Second
	}
	pingCtx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping postgres database: %w", err)
	}

	if err := storage.initSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	logger.Info("PostgreSQL storage initialized",
		slog.String("host", cfg.Host),
		slog.Int("port", cfg.Port),
		slog.String("database", cfg.Database),
		slog.String("sslmode", cfg.SSLMode),
		slog.Int("max_open_conns", cfg.MaxOpenConns),
		slog.Int("max_idle_conns", cfg.MaxIdleConns),
		slog.Duration("conn_max_lifetime", cfg.ConnMaxLifetime),
		slog.Duration("conn_max_idle_time", cfg.ConnMaxIdleTime),
		slog.String("dsn", sanitizePostgresDSN(dsn)))

	return storage, nil
}

// initSchema creates the database schema if it doesn't exist.
func (s *PostgresStorage) initSchema() (retErr error) {
	ctx := context.Background()

	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire postgres connection for schema migration: %w", err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			if retErr != nil {
				retErr = fmt.Errorf("%w; failed to close schema migration connection: %v", retErr, closeErr)
			} else {
				retErr = fmt.Errorf("failed to close schema migration connection: %w", closeErr)
			}
		}
	}()

	if _, err := conn.ExecContext(ctx, s.rebind(`SELECT pg_advisory_lock(?)`), postgresSchemaLockID); err != nil {
		return fmt.Errorf("failed to acquire schema migration lock: %w", err)
	}
	defer func() {
		if _, unlockErr := conn.ExecContext(ctx, s.rebind(`SELECT pg_advisory_unlock(?)`), postgresSchemaLockID); unlockErr != nil {
			if retErr != nil {
				retErr = fmt.Errorf("%w; failed to release schema migration lock: %v", retErr, unlockErr)
			} else {
				retErr = fmt.Errorf("failed to release schema migration lock: %w", unlockErr)
			}
		}
	}()

	if _, err := conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			id INTEGER PRIMARY KEY,
			version INTEGER NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("failed to ensure schema_migrations table: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO schema_migrations (id, version)
		VALUES (1, 0)
		ON CONFLICT (id) DO NOTHING
	`); err != nil {
		return fmt.Errorf("failed to initialize schema_migrations row: %w", err)
	}

	var version int
	if err := conn.QueryRowContext(ctx, `SELECT version FROM schema_migrations WHERE id = 1`).Scan(&version); err != nil {
		return fmt.Errorf("failed to query schema version: %w", err)
	}

	if version < postgresSchemaVersion {
		s.logger.Info("Initializing PostgreSQL schema", slog.Int("target_version", postgresSchemaVersion))
		if err := s.execSchemaStatements(ctx, conn, postgresSchemaSQL); err != nil {
			return fmt.Errorf("failed to execute postgres schema: %w", err)
		}
		if _, err := conn.ExecContext(ctx, s.rebind(`
			UPDATE schema_migrations
			SET version = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = 1
		`), postgresSchemaVersion); err != nil {
			return fmt.Errorf("failed to update schema_migrations: %w", err)
		}
		version = postgresSchemaVersion
	}

	s.logger.Info("PostgreSQL schema up to date", slog.Int("version", version))
	return nil
}

func (s *PostgresStorage) execSchemaStatements(ctx context.Context, conn *sql.Conn, schema string) error {
	if _, err := conn.ExecContext(ctx, schema); err != nil {
		return err
	}
	return nil
}

func withDefaultPostgresConfig(cfg PostgresConnectionConfig) PostgresConnectionConfig {
	if cfg.Port == 0 {
		cfg.Port = 5432
	}
	if cfg.SSLMode == "" {
		cfg.SSLMode = "require"
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = 5 * time.Second
	}
	if cfg.MaxOpenConns == 0 {
		cfg.MaxOpenConns = 25
	}
	if cfg.MaxIdleConns == 0 {
		cfg.MaxIdleConns = 5
	}
	if cfg.ConnMaxLifetime == 0 {
		cfg.ConnMaxLifetime = 30 * time.Minute
	}
	if cfg.ConnMaxIdleTime == 0 {
		cfg.ConnMaxIdleTime = 5 * time.Minute
	}
	if cfg.ApplicationName == "" {
		cfg.ApplicationName = "gateway-controller"
	}
	return cfg
}

func buildPostgresDSN(cfg PostgresConnectionConfig) (string, error) {
	if strings.TrimSpace(cfg.DSN) != "" {
		return cfg.DSN, nil
	}
	if cfg.Host == "" || cfg.Database == "" || cfg.User == "" {
		return "", fmt.Errorf("host, database and user are required when dsn is not provided")
	}
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Path:   cfg.Database,
	}
	q := u.Query()
	q.Set("sslmode", cfg.SSLMode)
	timeoutSec := int(cfg.ConnectTimeout.Seconds())
	if timeoutSec <= 0 {
		timeoutSec = 5
	}
	q.Set("connect_timeout", strconv.Itoa(timeoutSec))
	if cfg.ApplicationName != "" {
		q.Set("application_name", cfg.ApplicationName)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func sanitizePostgresDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return "<redacted>"
	}
	if u.User != nil {
		username := u.User.Username()
		if username != "" {
			u.User = url.UserPassword(username, "****")
		}
	}
	return u.String()
}

func (s *PostgresStorage) rebind(query string) string {
	return sqlx.Rebind(sqlx.DOLLAR, query)
}

func (s *PostgresStorage) exec(query string, args ...interface{}) (sql.Result, error) {
	return s.db.Exec(s.rebind(query), args...)
}

func (s *PostgresStorage) queryRow(query string, args ...interface{}) *sql.Row {
	return s.db.QueryRow(s.rebind(query), args...)
}

func isPostgresUniqueConstraintError(err error) bool {
	pgErr := extractPgError(err)
	return pgErr != nil && pgErr.Code == pgUniqueViolationCode
}

// isPostgresCertificateUniqueConstraintError checks if the error is a UNIQUE constraint violation for certificates.
func isPostgresCertificateUniqueConstraintError(err error) bool {
	pgErr := extractPgError(err)
	if pgErr == nil || pgErr.Code != pgUniqueViolationCode {
		return false
	}
	switch pgErr.ConstraintName {
	case "certificates_name_key", "certificates_pkey":
		return true
	default:
		return strings.Contains(pgErr.TableName, "certificates")
	}
}

func isPostgresTemplateUniqueConstraintError(err error) bool {
	pgErr := extractPgError(err)
	if pgErr == nil || pgErr.Code != pgUniqueViolationCode {
		return false
	}
	switch pgErr.ConstraintName {
	case "llm_provider_templates_handle_key":
		return true
	default:
		return strings.Contains(pgErr.TableName, "llm_provider_templates")
	}
}

// isPostgresAPIKeyUniqueConstraintError checks if the error is an API key uniqueness violation.
func isPostgresAPIKeyUniqueConstraintError(err error) bool {
	pgErr := extractPgError(err)
	if pgErr == nil || pgErr.Code != pgUniqueViolationCode {
		return false
	}
	// Keep this specific to API key uniqueness in case other unique constraints appear.
	switch pgErr.ConstraintName {
	case "api_keys_api_key_key", "api_keys_pkey", "idx_unique_external_api_key":
		return true
	default:
		return strings.Contains(pgErr.TableName, "api_keys")
	}
}

func extractPgError(err error) *pgconn.PgError {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr
	}
	return nil
}
