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
	"fmt"
	"log/slog"
	"strings"

	"github.com/jmoiron/sqlx"
)

// BackendConfig contains the minimal storage backend configuration required by NewStorage.
type BackendConfig struct {
	Type       string
	SQLitePath string
	Postgres   PostgresConnectionConfig
	SQLServer  SQLServerConnectionConfig
	GatewayID  string
}

// NewStorage creates the configured persistent storage backend and, for the network
// databases, confirms the database answers.
func NewStorage(cfg BackendConfig, logger *slog.Logger) (Storage, error) {
	return newStorage(cfg, logger, true)
}

// Open creates the configured persistent storage backend without connecting to a
// network database (PostgreSQL, SQL Server), so only configuration errors fail here;
// whether the database answers is left to the caller, which can then wait for it.
// SQLite is a local file and is opened exactly as NewStorage does.
func Open(cfg BackendConfig, logger *slog.Logger) (Storage, error) {
	return newStorage(cfg, logger, false)
}

// newStorage builds the storage backend; connect selects whether a network database
// must answer before it returns.
func newStorage(cfg BackendConfig, logger *slog.Logger, connect bool) (Storage, error) {
	switch cfg.Type {
	case "sqlite":
		backend, err := newSQLiteStorage(cfg.SQLitePath, logger)
		if err != nil {
			if strings.Contains(err.Error(), "database is locked") {
				return nil, fmt.Errorf("%w: %w", ErrDatabaseLocked, err)
			}
			return nil, err
		}

		store := newSQLStore(backend.db, backend.logger, "sqlite", cfg.GatewayID)
		store.rebindQuery = func(query string) string { return query }
		store.isUniqueViolation = isSQLiteUniqueConstraintError
		return store, nil

	case "postgres":
		var backend *PostgresStorage
		var err error
		if connect {
			backend, err = newPostgresStorage(cfg.Postgres, logger)
		} else {
			backend, _, err = openPostgresStorage(cfg.Postgres, logger)
		}
		if err != nil {
			return nil, err
		}

		store := newSQLStore(backend.db, backend.logger, "postgres", cfg.GatewayID)
		store.rebindQuery = func(query string) string { return sqlx.Rebind(sqlx.DOLLAR, query) }
		store.isUniqueViolation = isPostgresUniqueConstraintError
		return store, nil

	case "sqlserver":
		var backend *SQLServerStorage
		var err error
		if connect {
			backend, err = newSQLServerStorage(cfg.SQLServer, logger)
		} else {
			backend, _, err = openSQLServerStorage(cfg.SQLServer, logger)
		}
		if err != nil {
			return nil, err
		}

		store := newSQLStore(backend.db, backend.logger, "sqlserver", cfg.GatewayID)
		store.rebindQuery = func(query string) string { return sqlx.Rebind(sqlx.AT, query) }
		store.isUniqueViolation = isSQLServerUniqueConstraintError
		return store, nil

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedStorageType, cfg.Type)
	}
}
