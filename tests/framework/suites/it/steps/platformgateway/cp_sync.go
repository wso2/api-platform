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

package platformgateway

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"

	"github.com/wso2/api-platform/tests/framework/core/components"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
)

// The artifacts-table columns gateway-controller writes when it pushes a gateway-originated
// artifact to the control plane. No gateway API returns them, so the steps read the row.
const (
	cpSyncStatusColumn = "cp_sync_status"
	cpArtifactIDColumn = "cp_artifact_id"
)

// assertCPSyncStatus waits until gateway-controller has recorded the given control-plane sync
// status (pending, success or failed) on a gateway-originated artifact.
func (g *Gateway) assertCPSyncStatus(ctx context.Context, status, kind, handle string) error {
	return g.awaitArtifactColumn(ctx, cpSyncStatusColumn, kind, handle,
		fmt.Sprintf("control-plane sync status %q", status),
		func(value string) bool { return strings.EqualFold(value, status) })
}

// assertCPArtifactID waits until gateway-controller has recorded the identifier the control
// plane assigned to a gateway-originated artifact, which it does only after a successful push.
func (g *Gateway) assertCPArtifactID(ctx context.Context, kind, handle string) error {
	return g.awaitArtifactColumn(ctx, cpArtifactIDColumn, kind, handle,
		"a control-plane artifact id",
		func(value string) bool { return strings.TrimSpace(value) != "" })
}

// awaitArtifactColumn polls one column of an artifact's row until accept holds for its value.
func (g *Gateway) awaitArtifactColumn(
	ctx context.Context, column, kind, handle, want string, accept func(string) bool,
) error {
	resolvedKind, err := stepscommon.Expand(ctx, kind)
	if err != nil {
		return err
	}
	resolvedHandle, err := stepscommon.Expand(ctx, handle)
	if err != nil {
		return err
	}
	var last string
	err = retry.Await(ctx, retry.Options{},
		func(ctx context.Context) (string, error) {
			value, qerr := g.queryArtifactColumn(ctx, column, resolvedKind, resolvedHandle)
			if qerr != nil {
				return "", retry.Transient(qerr)
			}
			last = value
			return value, nil
		},
		accept,
		fmt.Sprintf("waiting for gateway-controller to record %s for the %s artifact %q",
			want, resolvedKind, resolvedHandle))
	if err != nil {
		return fmt.Errorf("%w (last recorded %s: %q)", err, column, last)
	}
	return nil
}

// queryArtifactColumn reads one column of an artifact's row in gateway-controller's own
// database, opening a fresh connection per call because the store may be a point-in-time
// snapshot (see Topology.OpenComponentDB). A NULL column reads as "".
func (g *Gateway) queryArtifactColumn(ctx context.Context, column, kind, handle string) (string, error) {
	store, ok := g.topo.Storage.Plan.StoreFor("platform-gateway", 0)
	if !ok {
		return "", fmt.Errorf("platform-gateway has no assigned store")
	}
	query, err := artifactColumnQuery(store.Type, column)
	if err != nil {
		return "", err
	}
	db, closeDB, err := g.topo.OpenComponentDB(ctx, "platform-gateway", "gateway-controller")
	if err != nil {
		return "", err
	}
	defer func() { _ = closeDB() }()

	var value sql.NullString
	if err := db.QueryRowContext(ctx, query, kind, handle).Scan(&value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("no %s artifact row found for handle %q", kind, handle)
		}
		return "", fmt.Errorf("querying %s of %s %q: %w", column, kind, handle, err)
	}
	return value.String, nil
}

// artifactColumnQuery returns the query that reads one control-plane sync column of an
// artifact by kind and handle. Only the known columns are accepted, so the column name can be
// written into the query text.
func artifactColumnQuery(engine components.DBType, column string) (string, error) {
	switch column {
	case cpSyncStatusColumn, cpArtifactIDColumn:
	default:
		return "", fmt.Errorf("unsupported artifacts column %q", column)
	}
	p1, p2 := sqlPlaceholders(engine)
	return fmt.Sprintf("SELECT %s FROM artifacts WHERE kind = %s AND handle = %s", column, p1, p2), nil
}

// sqlPlaceholders returns the first two bind placeholders in the engine's own form. SQLite
// accepts "?"; Postgres requires "$N"; SQL Server requires "@pN" unless the driver's query-text
// preprocessing is enabled, which sql.Open("sqlserver", ...) does not do (see go-mssqldb's
// NewConnectorWithProcessQueryText doc comment).
func sqlPlaceholders(engine components.DBType) (string, string) {
	switch engine {
	case components.Postgres:
		return "$1", "$2"
	case components.SQLServer:
		return "@p1", "@p2"
	default:
		return "?", "?"
	}
}
