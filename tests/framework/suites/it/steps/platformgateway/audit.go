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
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/cucumber/godog"

	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
)

// Where a feature reads audit records: gateway-controller's audit table, the audit lines on its
// stdout, and the testbench webhook service its http output posts each record to.
const (
	auditPlaceTable    = "gateway audit table"
	auditPlaceLog      = "gateway-controller log"
	auditPlaceReceiver = "audit receiver"
)

// auditFields are the record fields a feature can compare, named as in the audit line, with the
// audit table column that holds each.
var auditFields = []struct{ field, column string }{
	{"gatewayId", "gateway_id"},
	{"correlationId", "correlation_id"},
	{"action", "action"},
	{"resourceType", "resource_type"},
	{"resourceId", "resource_uuid"},
	{"resourceName", "resource_name"},
	{"outcome", "outcome"},
	{"performedBy", "performed_by"},
}

func (g *Gateway) registerAuditSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the (gateway audit table|gateway-controller log|audit receiver) should have exactly these audit records for "([^"]*)":$`,
		g.auditRecordsAre)
}

// auditRecordsAre waits until the records for a resource, matched by its name or its id, are
// exactly the table's rows. Only the table's columns are compared, and order is ignored, so a
// change the gateway should not record fails the step by never letting the records match.
func (g *Gateway) auditRecordsAre(ctx context.Context, place, resource string, table *godog.Table) error {
	key, err := stepscommon.Expand(ctx, resource)
	if err != nil {
		return err
	}
	want, err := auditExpectations(ctx, table)
	if err != nil {
		return err
	}
	var last []map[string]string
	err = retry.Await(ctx, retry.Options{},
		func(ctx context.Context) ([]map[string]string, error) {
			records, rerr := g.auditRecords(ctx, place, key)
			if rerr != nil {
				return nil, retry.Transient(rerr)
			}
			last = records
			return records, nil
		},
		func(records []map[string]string) bool { return want.match(records) == nil },
		fmt.Sprintf("waiting for the %s to hold the expected audit records for %q", place, key))
	if err != nil {
		return fmt.Errorf("%w: %v", err, want.match(last))
	}
	return nil
}

// auditRecords reads the records for key from one place.
func (g *Gateway) auditRecords(ctx context.Context, place, key string) ([]map[string]string, error) {
	var records []map[string]string
	switch place {
	case auditPlaceTable:
		return g.auditTableRecords(ctx, key)
	case auditPlaceLog:
		stack, _, err := g.topo.ServiceControl("gateway-controller")
		if err != nil {
			return nil, err
		}
		records = auditLogRecords(stack.Logs(ctx))
	case auditPlaceReceiver:
		url, err := g.serviceURL(ctx, "webhook", "/test/deliveries")
		if err != nil {
			return nil, err
		}
		response, err := g.funnel.Client().Do(ctx, httpx.Request{Method: http.MethodGet, URL: url}, 0, 0)
		if err != nil {
			return nil, err
		}
		if !response.Succeeded() {
			return nil, fmt.Errorf("webhook deliveries returned %s", response.Describe())
		}
		var deliveries []struct {
			Body json.RawMessage `json:"body"`
		}
		if err := json.Unmarshal(response.Body, &deliveries); err != nil {
			return nil, fmt.Errorf("decoding webhook deliveries: %w", err)
		}
		for _, delivery := range deliveries {
			if record, ok := decodeAuditRecord(delivery.Body); ok {
				records = append(records, record)
			}
		}
	default:
		return nil, fmt.Errorf("unknown audit record place %q", place)
	}
	return slices.DeleteFunc(records, func(record map[string]string) bool {
		return record["resourceName"] != key && record["resourceId"] != key
	}), nil
}

// auditTableRecords reads the audit rows whose resource name or id is key, opening a fresh
// connection per call because the store may be a point-in-time snapshot (see
// Topology.OpenComponentDB). A NULL column reads as "".
func (g *Gateway) auditTableRecords(ctx context.Context, key string) ([]map[string]string, error) {
	store, ok := g.topo.Storage.Plan.StoreFor("platform-gateway", 0)
	if !ok {
		return nil, fmt.Errorf("platform-gateway has no assigned store")
	}
	columns := make([]string, len(auditFields))
	for i, f := range auditFields {
		columns[i] = f.column
	}
	p1, p2 := sqlPlaceholders(store.Type)
	query := fmt.Sprintf("SELECT %s FROM audit WHERE resource_name = %s OR resource_uuid = %s",
		strings.Join(columns, ", "), p1, p2)

	db, closeDB, err := g.topo.OpenComponentDB(ctx, "platform-gateway", "gateway-controller")
	if err != nil {
		return nil, err
	}
	defer func() { _ = closeDB() }()
	rows, err := db.QueryContext(ctx, query, key, key)
	if err != nil {
		return nil, fmt.Errorf("querying audit rows for %q: %w", key, err)
	}
	defer func() { _ = rows.Close() }()

	var records []map[string]string
	for rows.Next() {
		values := make([]sql.NullString, len(auditFields))
		dest := make([]any, len(values))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("reading an audit row for %q: %w", key, err)
		}
		record := make(map[string]string, len(auditFields))
		for i, f := range auditFields {
			record[f.field] = values[i].String
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

// auditLogRecords returns the audit records among the JSON lines of a log.
func auditLogRecords(logs string) []map[string]string {
	var records []map[string]string
	for _, line := range strings.Split(logs, "\n") {
		start := strings.IndexByte(line, '{')
		if start < 0 {
			continue
		}
		if record, ok := decodeAuditRecord([]byte(line[start:])); ok {
			records = append(records, record)
		}
	}
	return records
}

// decodeAuditRecord reads one audit line. Anything that is not a JSON object tagged
// "logType":"audit" is not a record.
func decodeAuditRecord(raw []byte) (map[string]string, bool) {
	var fields map[string]any
	if json.Unmarshal(raw, &fields) != nil || fields["logType"] != "audit" {
		return nil, false
	}
	record := make(map[string]string, len(auditFields))
	for _, f := range auditFields {
		if value, ok := fields[f.field].(string); ok {
			record[f.field] = value
		}
	}
	return record, true
}

// auditExpectation is a step table: a header row of record fields and one row per record.
type auditExpectation struct {
	columns []string
	rows    [][]string
}

func auditExpectations(ctx context.Context, table *godog.Table) (auditExpectation, error) {
	if table == nil || len(table.Rows) == 0 {
		return auditExpectation{}, fmt.Errorf("an audit records step needs a table with a header row of record fields")
	}
	var want auditExpectation
	for _, cell := range table.Rows[0].Cells {
		field := strings.TrimSpace(cell.Value)
		if !slices.ContainsFunc(auditFields, func(f struct{ field, column string }) bool { return f.field == field }) {
			return auditExpectation{}, fmt.Errorf("unknown audit record field %q", field)
		}
		if slices.Contains(want.columns, field) {
			return auditExpectation{}, fmt.Errorf("audit record field %q appears twice", field)
		}
		want.columns = append(want.columns, field)
	}
	for _, row := range table.Rows[1:] {
		if len(row.Cells) != len(want.columns) {
			return auditExpectation{}, fmt.Errorf("an audit record row has %d cells, want %d", len(row.Cells), len(want.columns))
		}
		values := make([]string, len(row.Cells))
		for i, cell := range row.Cells {
			value, err := stepscommon.Expand(ctx, strings.TrimSpace(cell.Value))
			if err != nil {
				return auditExpectation{}, err
			}
			values[i] = value
		}
		want.rows = append(want.rows, values)
	}
	return want, nil
}

// match reports how records differ from the expected rows, comparing only the expected columns.
func (want auditExpectation) match(records []map[string]string) error {
	got := make([]string, len(records))
	for i, record := range records {
		values := make([]string, len(want.columns))
		for j, column := range want.columns {
			values[j] = record[column]
		}
		got[i] = strings.Join(values, " | ")
	}
	expected := make([]string, len(want.rows))
	for i, row := range want.rows {
		expected[i] = strings.Join(row, " | ")
	}
	slices.Sort(got)
	slices.Sort(expected)
	if !slices.Equal(got, expected) {
		return fmt.Errorf("audit records (%s) are [%s], want [%s]",
			strings.Join(want.columns, " | "), strings.Join(got, "; "), strings.Join(expected, "; "))
	}
	return nil
}
