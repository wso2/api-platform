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

// Package dbschema owns the database schema for tables that are specific to
// event-gateway-controller: websub_apis, webbroker_apis, and webhook_secrets.
// These used to live in gateway-controller (core)'s own schema scripts, but
// core has no use for them on its own — only this module's kinds and features
// reference them. Apply runs this module's own idempotent DDL against the
// exact same database connection/backend that core's Storage already opened
// (via Storage.GetDB()), immediately after core's own schema has been
// applied, so both sets of tables end up in the same database without core's
// schema files ever needing to know about these tables.
//
// The embedded .sql files here mirror gateway/gateway-controller/pkg/storage's
// own gateway-controller-db*.sql naming and per-dialect split.
package dbschema

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
)

//go:embed eventgateway-db.sql
var sqliteSchema string

//go:embed eventgateway-db.postgres.sql
var postgresSchema string

//go:embed eventgateway-db.sqlserver.sql
var sqlserverSchema string

// forBackend returns the idempotent DDL for the given storage backend type
// ("sqlite", "postgres", or "sqlserver" — matching storage.BackendConfig.Type).
func forBackend(backendType string) (string, error) {
	switch backendType {
	case "sqlite":
		return sqliteSchema, nil
	case "postgres":
		return postgresSchema, nil
	case "sqlserver":
		return sqlserverSchema, nil
	default:
		return "", fmt.Errorf("unsupported storage backend for event-gateway schema: %s", backendType)
	}
}

// Apply creates the websub_apis, webbroker_apis, and webhook_secrets tables
// (if they don't already exist) against db, using the DDL dialect for
// backendType. Safe to call on every startup — every statement is idempotent.
func Apply(ctx context.Context, db *sql.DB, backendType string) error {
	schema, err := forBackend(backendType)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("failed to apply event-gateway database schema: %w", err)
	}
	return nil
}
