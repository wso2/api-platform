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

package it

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	// dbReaderContainer is the sqlite sidecar that mounts the controller's data
	// volume read-only and runs sqlite3 against the gateway-controller database.
	// Present in docker-compose.test.yaml (sqlite mode) only.
	dbReaderContainer = "it-db-reader"

	// postgresContainer is the postgres service used by the postgres compose.
	// Present in docker-compose.test.postgres.yaml only. Has psql built-in.
	postgresContainer = "it-postgres"

	// sqlserverContainer is the SQL Server service used by the sqlserver compose.
	// Present in docker-compose.test.sqlserver.yaml only. Has sqlcmd built-in.
	sqlserverContainer = "it-sqlserver"

	// gatewayDBPath is the SQLite database path inside dbReaderContainer.
	gatewayDBPath = "/data/gateway.db"

	// postgresDB / postgresUser match the credentials in
	// docker-compose.test.postgres.yaml.
	postgresDB   = "gateway_test"
	postgresUser = "gateway"

	// sqlserverDB / sqlserverUser match the credentials in
	// docker-compose.test.sqlserver.yaml. The SA password is read from the
	// MSSQL_SA_PASSWORD env var (set by the workflow/test runner).
	sqlserverDB   = "gateway_test"
	sqlserverUser = "sa"
	sqlcmdPath    = "/opt/mssql-tools18/bin/sqlcmd"

	// defaultDBQueryTimeout caps the time allowed for a query (including
	// retries) so a stuck reader container can't hang a scenario.
	defaultDBQueryTimeout = 10 * time.Second
)

// validSQLIdentifier matches safe table/column identifiers: letters, digits and
// underscores only. Used to guard against SQL identifier injection.
var validSQLIdentifier = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// sqlLiteral escapes s for safe embedding as a single-quoted SQL string
// literal. It doubles any embedded single quotes (SQL-standard escaping).
// Used when the query is sent to a CLI tool (sqlite3 / psql) that doesn't
// support bind parameters.
func sqlLiteral(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// backendCache memoises which DB backend is reachable for the current run.
// cachedDriver is only set when a running container is detected; if detection
// failed the mutex is not held between calls so future calls can retry.
var (
	backendMu    sync.Mutex
	cachedDriver string
)

// detectDBDriver inspects which reader container is actually running and
// returns "sqlite" or "postgres". The result is cached once a running
// container is found; if no container is detected, the cache is left empty
// so subsequent calls will retry detection.
func detectDBDriver(ctx context.Context) string {
	backendMu.Lock()
	if cachedDriver != "" {
		driver := cachedDriver
		backendMu.Unlock()
		return driver
	}
	backendMu.Unlock()

	// Perform container checks outside the lock (they can be slow).
	var detected string
	if containerRunning(ctx, dbReaderContainer) {
		detected = "sqlite"
	} else if containerRunning(ctx, postgresContainer) {
		detected = "postgres"
	} else if containerRunning(ctx, sqlserverContainer) {
		detected = "sqlserver"
	}

	if detected != "" {
		backendMu.Lock()
		if cachedDriver == "" { // another goroutine may have set it first
			cachedDriver = detected
		}
		backendMu.Unlock()
	}
	return detected
}

// containerRunning returns true if `docker inspect` reports the container as
// existing and running. We don't fail on inspect errors — absent containers
// just yield false.
func containerRunning(ctx context.Context, name string) bool {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Running}}", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

// envRuntimeControllerXDS is set (via the Makefile, see test-postgres) for the
// two-controller Postgres topology, where gateway-runtime is fed xDS by
// gateway-controller-xds rather than the management controller.
const envRuntimeControllerXDS = "IT_GATEWAY_CONTROLLER_HA"

// policySnapshotControllerAdminURL returns the admin base URL the policy-chain
// xDS-sync probe should target, or "" to use the default management controller.
// When IT_GATEWAY_CONTROLLER_HA=true it points at the runtime-facing
// controller (host port 9093), whose policy-chain version the policy engine
// echoes. Returning "" lets waitForPolicySnapshotSync fall back to the
// management controller (single-controller topologies and unit tests).
func policySnapshotControllerAdminURL() string {
	if os.Getenv(envRuntimeControllerXDS) == "true" {
		return fmt.Sprintf("http://localhost:%s%s", GatewayControllerRuntimeAdminPort, GatewayAdminAPIBasePath)
	}
	return ""
}

// queryStoredConfiguration runs a SELECT against one of the per-resource-type
// tables (rest_apis / websub_apis / etc.) joined with artifacts to look the row
// up by handle. Returns the raw configuration JSON blob (the unrendered
// SourceConfiguration as persisted by the controller). The SQL is identical
// across sqlite and postgres; the per-backend CLI invocation lives in
// executeQuery.
//
// kind is the artifact kind ("RestApi", "WebSubApi", ...) — used to constrain
// the join so a handle collision across kinds returns the right row. table is
// the per-kind table name (e.g. "rest_apis").
func queryStoredConfiguration(ctx context.Context, kind, table, handle string) (string, error) {
	if !validSQLIdentifier.MatchString(table) {
		return "", fmt.Errorf("invalid SQL table identifier %q: must match [A-Za-z0-9_]+", table)
	}
	query := fmt.Sprintf(
		"SELECT t.configuration FROM %s t JOIN artifacts a ON t.uuid = a.uuid AND t.gateway_id = a.gateway_id WHERE a.kind = '%s' AND a.handle = '%s';",
		table, sqlLiteral(kind), sqlLiteral(handle),
	)

	row, err := executeQuery(ctx, query)
	if err != nil {
		return "", err
	}
	if row == "" {
		return "", fmt.Errorf("no %s row found for handle %q", kind, handle)
	}
	return row, nil
}

// executeQuery runs a single SELECT against whichever DB reader container is
// up (it-db-reader for sqlite, it-postgres for postgres) and returns the
// trimmed first-cell result. Returns "" with no error when the query produced
// zero rows; returns a non-nil error if no reader container is reachable or
// the CLI exited non-zero.
func executeQuery(ctx context.Context, query string) (string, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultDBQueryTimeout)
		defer cancel()
	}
	driver := detectDBDriver(ctx)
	var cmd *exec.Cmd
	switch driver {
	case "sqlite":
		cmd = exec.CommandContext(ctx, "docker", "exec", dbReaderContainer, "sqlite3", "-bail", gatewayDBPath, query)
	case "postgres":
		// -A unaligned, -t tuples-only, -X no .psqlrc — produces just the value.
		cmd = exec.CommandContext(ctx, "docker", "exec", postgresContainer,
			"psql", "-U", postgresUser, "-d", postgresDB, "-AtX", "-c", query)
	case "sqlserver":
		// -h -1 no headers; -y 8000 widens the variable-length column display from
		// sqlcmd's 256-char default to its maximum so the NVARCHAR(MAX) configuration
		// JSON isn't truncated (test configs are far smaller than 8000); -w 65535
		// stops long lines from wrapping; -b exits non-zero on error; -C trusts the
		// self-signed server cert. (-y 0 / -W are rejected alongside -h/-y, and the
		// caller already TrimSpaces, so neither is used.)
		pw := os.Getenv("MSSQL_SA_PASSWORD")
		if pw == "" {
			return "", fmt.Errorf("MSSQL_SA_PASSWORD must be set for sqlserver test queries")
		}
		cmd = exec.CommandContext(ctx, "docker", "exec", sqlserverContainer,
			sqlcmdPath, "-C", "-S", "localhost", "-U", sqlserverUser, "-P", pw,
			"-d", sqlserverDB, "-h", "-1", "-y", "8000", "-w", "65535", "-b",
			"-Q", "SET NOCOUNT ON; "+query)
	default:
		return "", fmt.Errorf("no DB reader container is running (looked for %q, %q and %q)", dbReaderContainer, postgresContainer, sqlserverContainer)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s query failed: %w (output: %s)", driver, err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// GetStoredRestAPISourceConfiguration returns the unrendered SourceConfiguration
// JSON blob for a RestApi handle. Used by IT scenarios to assert the DB persists
// the original templated body, not the rendered one.
func GetStoredRestAPISourceConfiguration(ctx context.Context, handle string) (string, error) {
	return queryStoredConfiguration(ctx, "RestApi", "rest_apis", handle)
}

// GetStoredRestAPISourceConfigurationWithRetry retries a few times to give the
// controller a moment to flush the row to disk after a POST. The controller
// upserts synchronously on the request path, but in CI we occasionally see the
// row not visible to a separate sqlite3 process for a few hundred ms.
func GetStoredRestAPISourceConfigurationWithRetry(ctx context.Context, handle string) (string, error) {
	return GetStoredSourceConfigurationWithRetry(ctx, "RestApi", "rest_apis", handle)
}

// GetStoredSourceConfigurationWithRetry generalises GetStoredRestAPISourceConfigurationWithRetry
// to any artifact kind/table pair so template-rendering ITs can assert DB
// persistence for LlmProvider, LlmProxy, and Mcp in addition to RestApi.
func GetStoredSourceConfigurationWithRetry(ctx context.Context, kind, table, handle string) (string, error) {
	const maxAttempts = 10
	const interval = 200 * time.Millisecond

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		row, err := queryStoredConfiguration(ctx, kind, table, handle)
		if err == nil {
			return row, nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}
	}
	return "", fmt.Errorf("stored configuration not found after %d attempts: %w", maxAttempts, lastErr)
}
