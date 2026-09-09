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

package steps

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/components"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// templateLiteralTables maps a gateway artifact kind to its per-kind storage table, joined
// against the shared artifacts table by kind+handle. Mirrors the schemas in
// gateway-controller/pkg/storage/gateway-controller-db.sql.
var templateLiteralTables = map[string]string{
	"RestApi":     "rest_apis",
	"LlmProvider": "llm_providers",
	"LlmProxy":    "llm_proxies",
	"Mcp":         "mcp_proxies",
}

// responseBodyContainsTemplateLiteral asserts the last published response carries a template
// expression (e.g. `{{ secret "x" }}`) verbatim - proving a RestApi/LlmProvider/LlmProxy/Mcp
// spec keeps its template body unrendered in every API response, even though the gateway
// resolves it upstream at request time.
func (g *Gateway) responseBodyContainsTemplateLiteral(ctx context.Context, literal *godog.DocString) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	expanded, err := stepscommon.Expand(ctx, literal.Content)
	if err != nil {
		return err
	}
	expected := strings.TrimSpace(expanded)
	if expected == "" {
		return fmt.Errorf("expected template literal is empty")
	}
	if containsLiteralOrJSONEscaped(resp.Text(), expected) {
		return nil
	}
	return fmt.Errorf("response body does not contain expected template literal %q: %s", expected, resp.Describe())
}

// containsLiteralOrJSONEscaped reports whether haystack contains needle either verbatim or
// with its embedded double quotes JSON-escaped. A template literal like `{{ secret "x" }}`
// appears in a JSON response/stored row with its quotes escaped, since the whole literal sits
// inside a JSON string field; this lets a feature write the natural, unescaped literal either
// way.
func containsLiteralOrJSONEscaped(haystack, needle string) bool {
	if strings.Contains(haystack, needle) {
		return true
	}
	escaped := strings.ReplaceAll(needle, `"`, `\"`)
	return escaped != needle && strings.Contains(haystack, escaped)
}

// assertStoredConfiguration asserts whether gateway-controller's own persisted configuration
// row for a gateway-originated artifact contains a template literal - proving what is
// physically on disk, independent of what any API chooses to serialize back. This is the one
// assertion a REST call cannot make: an API response and the DB row happen to agree today, but
// they are two separate guarantees (see template_functions.feature), and only a direct read
// proves the second one.
func (g *Gateway) assertStoredConfiguration(ctx context.Context, kind, handle string, doc *godog.DocString, want bool) error {
	table, ok := templateLiteralTables[kind]
	if !ok {
		return fmt.Errorf("unknown artifact kind %q for stored-configuration assertions", kind)
	}
	resolvedHandle, err := stepscommon.Expand(ctx, handle)
	if err != nil {
		return err
	}
	expandedLiteral, err := stepscommon.Expand(ctx, doc.Content)
	if err != nil {
		return err
	}
	literal := strings.TrimSpace(expandedLiteral)
	if literal == "" {
		return fmt.Errorf("expected literal is empty")
	}

	verb := "contain"
	if !want {
		verb = "not contain"
	}
	what := fmt.Sprintf("waiting for the stored %s configuration for %q to %s the expected literal",
		kind, resolvedHandle, verb)

	var last string
	err = retry.Await(ctx, retry.Options{},
		func(ctx context.Context) (string, error) {
			row, qerr := g.queryStoredConfiguration(ctx, table, kind, resolvedHandle)
			if qerr != nil {
				return "", retry.Transient(qerr)
			}
			last = row
			return row, nil
		},
		func(row string) bool { return containsLiteralOrJSONEscaped(row, literal) == want },
		what)
	if err != nil {
		return fmt.Errorf("%w (last stored row: %s)", err, last)
	}
	return nil
}

// queryStoredConfiguration reads one artifact's raw configuration column directly from
// gateway-controller's own database - a fresh connection per call, since the underlying store
// may be a point-in-time snapshot (see Topology.OpenComponentDB) that a caller polling for an
// eventually-consistent write must re-take on every attempt.
func (g *Gateway) queryStoredConfiguration(ctx context.Context, table, kind, handle string) (string, error) {
	store, ok := g.topo.Storage.Plan.StoreFor("platform-gateway", 0)
	if !ok {
		return "", fmt.Errorf("platform-gateway has no assigned store")
	}

	db, closeDB, err := g.topo.OpenComponentDB(ctx, "platform-gateway", "gateway-controller")
	if err != nil {
		return "", err
	}
	defer func() { _ = closeDB() }()

	// SQLite accepts "?"; Postgres requires "$N"; SQL Server requires "@pN" unless the
	// driver's query-text preprocessing is enabled, which sql.Open("sqlserver", ...) does not
	// do (see go-mssqldb's NewConnectorWithProcessQueryText doc comment).
	p1, p2 := "?", "?"
	switch store.Type {
	case components.Postgres:
		p1, p2 = "$1", "$2"
	case components.SQLServer:
		p1, p2 = "@p1", "@p2"
	}
	query := fmt.Sprintf(
		"SELECT t.configuration FROM %s t JOIN artifacts a ON t.uuid = a.uuid AND t.gateway_id = a.gateway_id WHERE a.kind = %s AND a.handle = %s",
		table, p1, p2)

	var configuration string
	if err := db.QueryRowContext(ctx, query, kind, handle).Scan(&configuration); err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("no %s row found for handle %q", kind, handle)
		}
		return "", fmt.Errorf("querying stored %s configuration for %q: %w", kind, handle, err)
	}
	return configuration, nil
}
