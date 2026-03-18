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
	Type        string
	SQLitePath  string
	Postgres    PostgresConnectionConfig
	GatewayID   string
}

// NewStorage creates the configured persistent storage backend.
func NewStorage(cfg BackendConfig, logger *slog.Logger) (Storage, error) {
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
		store.isConfigUniqueViolation = isUniqueConstraintError
		store.isCertificateUniqueViolation = isCertificateUniqueConstraintError
		store.isTemplateUniqueViolation = isTemplateUniqueConstraintError
		store.isAPIKeyUniqueViolation = isAPIKeyUniqueConstraintError
		store.isSubscriptionUniqueViolation = isSubscriptionUniqueConstraintError
		store.isSubscriptionPlanUniqueViolation = isSubscriptionPlanUniqueConstraintError
		return store, nil

	case "postgres":
		backend, err := newPostgresStorage(cfg.Postgres, logger)
		if err != nil {
			return nil, err
		}

		store := newSQLStore(backend.db, backend.logger, "postgres", cfg.GatewayID)
		store.rebindQuery = func(query string) string { return sqlx.Rebind(sqlx.DOLLAR, query) }
		store.isConfigUniqueViolation = isPostgresUniqueConstraintError
		store.isCertificateUniqueViolation = isPostgresCertificateUniqueConstraintError
		store.isTemplateUniqueViolation = isPostgresTemplateUniqueConstraintError
		store.isAPIKeyUniqueViolation = isPostgresAPIKeyUniqueConstraintError
		store.isSubscriptionUniqueViolation = isPostgresSubscriptionUniqueConstraintError
		store.isSubscriptionPlanUniqueViolation = isPostgresSubscriptionPlanUniqueConstraintError
		return store, nil

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedStorageType, cfg.Type)
	}
}
