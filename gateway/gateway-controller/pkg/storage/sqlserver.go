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
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"regexp"
	"strings"
	"time"

	mssql "github.com/microsoft/go-mssqldb"
)

const (
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

// sqlServerSemicolonPasswordRe matches the password key in ADO/ODBC-style
// (semicolon-separated) DSNs, e.g. "server=h;password=secret;..." — both
// "password" and "pwd", any case — so the value can be redacted before logging.
var sqlServerSemicolonPasswordRe = regexp.MustCompile(`(?i)\b(password|pwd)\s*=[^;]*`)

func sanitizeSQLServerDSN(dsn string) string {
	// go-mssqldb also accepts ADO ("server=...;password=...") and ODBC
	// ("odbc:...;pwd=...") DSNs, which url.Parse does not understand. Redact the
	// password token directly for those so it never reaches the logs.
	if strings.Contains(dsn, ";") || !strings.Contains(dsn, "://") {
		return sqlServerSemicolonPasswordRe.ReplaceAllString(dsn, "${1}=****")
	}
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
	// go-mssqldb also accepts the password as a URL query parameter
	// (e.g. sqlserver://host?user id=sa&password=secret), which is not part of
	// the userinfo above — redact those too.
	if q := u.Query(); len(q) > 0 {
		changed := false
		for key := range q {
			switch strings.ToLower(key) {
			case "password", "pwd":
				q.Set(key, "****")
				changed = true
			}
		}
		if changed {
			u.RawQuery = q.Encode()
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
