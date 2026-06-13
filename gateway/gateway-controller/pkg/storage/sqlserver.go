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

	mssql "github.com/microsoft/go-mssqldb"
)

//go:embed gateway-controller-db.sqlserver.sql
var sqlserverSchemaSQL string

const (
	// sqlserverSchemaLockResource names the application lock used to serialize
	// concurrent schema initialization across controller replicas.
	sqlserverSchemaLockResource = "gateway-controller-schema-init"
	// SQL Server error numbers for unique-constraint / duplicate-key violations.
	sqlserverUniqueConstraintErr = 2627 // PRIMARY KEY / UNIQUE constraint violation
	sqlserverDuplicateKeyErr     = 2601 // unique index violation
	// defaultSQLServerEncrypt is the connection-level fallback for the "encrypt"
	// option, applied when the caller leaves it unset (e.g. storage used directly
	// in tests). The config layer normalizes/validates the value before this for
	// config-driven startup; this keeps the storage layer self-sufficient.
	defaultSQLServerEncrypt = "true"
)

// SQLServerConnectionConfig holds SQL Server-specific connection settings.
type SQLServerConnectionConfig struct {
	DSN                    string
	Host                   string
	Port                   int
	Database               string
	User                   string
	Password               string
	Encrypt                string // "disable", "false", "true", or "strict"
	TrustServerCertificate bool
	ConnectTimeout         time.Duration
	MaxOpenConns           int
	MaxIdleConns           int
	ConnMaxLifetime        time.Duration
	ConnMaxIdleTime        time.Duration
	ApplicationName        string
}

// SQLServerStorage implements the Storage interface using Microsoft SQL Server.
type SQLServerStorage struct {
	db     *sql.DB
	logger *slog.Logger
}

// newSQLServerStorage creates a new SQL Server storage instance.
func newSQLServerStorage(cfg SQLServerConnectionConfig, logger *slog.Logger) (*SQLServerStorage, error) {
	cfg = withDefaultSQLServerConfig(cfg)
	dsn, err := buildSQLServerDSN(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to build sqlserver dsn: %w", err)
	}

	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlserver database: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	storage := &SQLServerStorage{
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
		return nil, fmt.Errorf("failed to ping sqlserver database: %w", err)
	}

	if err := storage.initSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	logger.Info("SQLServer storage initialized",
		slog.String("host", cfg.Host),
		slog.Int("port", cfg.Port),
		slog.String("database", cfg.Database),
		slog.String("encrypt", cfg.Encrypt),
		slog.Bool("trust_server_certificate", cfg.TrustServerCertificate),
		slog.Int("max_open_conns", cfg.MaxOpenConns),
		slog.Int("max_idle_conns", cfg.MaxIdleConns),
		slog.Duration("conn_max_lifetime", cfg.ConnMaxLifetime),
		slog.Duration("conn_max_idle_time", cfg.ConnMaxIdleTime),
		slog.String("dsn", sanitizeSQLServerDSN(dsn)))

	return storage, nil
}

// initSchema creates the database schema if it doesn't exist. The embedded
// schema is idempotent (every object is guarded by IF NOT EXISTS); an
// application lock serializes concurrent initialization across replicas.
func (s *SQLServerStorage) initSchema() (retErr error) {
	ctx := context.Background()

	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire sqlserver connection for schema init: %w", err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			if retErr != nil {
				retErr = fmt.Errorf("%w; failed to close schema init connection: %v", retErr, closeErr)
			} else {
				retErr = fmt.Errorf("failed to close schema init connection: %w", closeErr)
			}
		}
	}()

	// Serialize schema init across replicas with a session-scoped application lock.
	// sp_getapplock reports failure (e.g. a lock-wait timeout) only through its
	// integer return code, not via a driver error, so capture and inspect it.
	// Non-negative means granted (0 = granted, 1 = granted after waiting);
	// negative codes (-1 timeout, -2 cancelled, -3 deadlock, -999 other) are failures.
	var lockStatus mssql.ReturnStatus
	if _, err := conn.ExecContext(ctx,
		"EXEC sp_getapplock @Resource = @p1, @LockMode = 'Exclusive', @LockOwner = 'Session', @LockTimeout = 30000",
		sqlserverSchemaLockResource, &lockStatus); err != nil {
		return fmt.Errorf("failed to acquire schema init lock: %w", err)
	}
	if lockStatus < 0 {
		return fmt.Errorf("failed to acquire schema init lock: sp_getapplock returned %d", int(lockStatus))
	}
	defer func() {
		if _, unlockErr := conn.ExecContext(ctx,
			"EXEC sp_releaseapplock @Resource = @p1, @LockOwner = 'Session'",
			sqlserverSchemaLockResource); unlockErr != nil {
			if retErr != nil {
				retErr = fmt.Errorf("%w; failed to release schema init lock: %v", retErr, unlockErr)
			} else {
				retErr = fmt.Errorf("failed to release schema init lock: %w", unlockErr)
			}
		}
	}()

	s.logger.Info("Initializing SQLServer schema")
	if _, err := conn.ExecContext(ctx, sqlserverSchemaSQL); err != nil {
		return fmt.Errorf("failed to execute sqlserver schema: %w", err)
	}

	s.logger.Info("SQLServer schema initialized")
	return nil
}

func withDefaultSQLServerConfig(cfg SQLServerConnectionConfig) SQLServerConnectionConfig {
	if cfg.Port == 0 {
		cfg.Port = 1433
	}
	if cfg.Encrypt == "" {
		cfg.Encrypt = defaultSQLServerEncrypt
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

func buildSQLServerDSN(cfg SQLServerConnectionConfig) (string, error) {
	if strings.TrimSpace(cfg.DSN) != "" {
		return cfg.DSN, nil
	}
	if cfg.Host == "" || cfg.Database == "" || cfg.User == "" {
		return "", fmt.Errorf("host, database and user are required when dsn is not provided")
	}
	u := &url.URL{
		Scheme: "sqlserver",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
	}
	q := u.Query()
	q.Set("database", cfg.Database)
	if cfg.Encrypt != "" {
		q.Set("encrypt", cfg.Encrypt)
	}
	q.Set("TrustServerCertificate", strconv.FormatBool(cfg.TrustServerCertificate))
	timeoutSec := int(cfg.ConnectTimeout.Seconds())
	if timeoutSec <= 0 {
		timeoutSec = 5
	}
	q.Set("connection timeout", strconv.Itoa(timeoutSec))
	q.Set("dial timeout", strconv.Itoa(timeoutSec))
	if cfg.ApplicationName != "" {
		q.Set("app name", cfg.ApplicationName)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func sanitizeSQLServerDSN(dsn string) string {
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

// isSQLServerUniqueConstraintError reports whether err is a unique-constraint or
// duplicate-key violation reported by SQL Server.
func isSQLServerUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	var mssqlErr mssql.Error
	if errors.As(err, &mssqlErr) {
		return mssqlErr.Number == sqlserverUniqueConstraintErr || mssqlErr.Number == sqlserverDuplicateKeyErr
	}
	return false
}
