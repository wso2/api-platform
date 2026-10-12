/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package migration

import (
	"context"
	"fmt"
	"strings"
)

// insertRow builds and executes a positional INSERT with an optional conflict
// clause, e.g. "ON CONFLICT (uuid) DO NOTHING". This is the workhorse for the
// UUID-PK / natural-key idempotency cases (§B.9). len(cols) must equal len(vals).
//
// For partial-unique targets the conflict clause carries the predicate, e.g.
//
//	ON CONFLICT (organization_uuid, artifact_uuid, application_id)
//	  WHERE application_id IS NOT NULL DO NOTHING
//
// which infers the partial index rather than a plain UUID upsert (§B.9 ⚠).
func insertRow(ctx context.Context, q queryer, table string, cols []string, vals []any, conflict string) error {
	if len(cols) != len(vals) {
		return fmt.Errorf("insertRow %s: %d cols vs %d vals", table, len(cols), len(vals))
	}
	ph := make([]string, len(cols))
	for i := range cols {
		ph[i] = fmt.Sprintf("$%d", i+1)
	}
	sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(cols, ", "), strings.Join(ph, ", "))
	if conflict != "" {
		sql += " " + conflict
	}
	if _, err := q.ExecContext(ctx, sql, vals...); err != nil {
		return fmt.Errorf("insert %s: %w", table, err)
	}
	return nil
}

// deleteByParent removes a parent's child rows before re-inserting them — the
// idempotency toolkit for SERIAL/INTEGER PKs where a synthesized UUID is
// type-invalid (gateway_endpoints.id, §B.9). parentCol is a single FK column.
func deleteByParent(ctx context.Context, q queryer, table, parentCol string, parentVal any) error {
	if _, err := q.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE %s = $1", table, parentCol), parentVal); err != nil {
		return fmt.Errorf("delete %s by %s: %w", table, parentCol, err)
	}
	return nil
}

// Common conflict clauses, named so call sites read clearly.
const (
	conflictUUIDNothing = "ON CONFLICT (uuid) DO NOTHING"
	conflictDoNothing   = "ON CONFLICT DO NOTHING"
)

// conflictCols builds `ON CONFLICT (a, b, c) DO NOTHING` for a natural key.
func conflictCols(cols ...string) string {
	return fmt.Sprintf("ON CONFLICT (%s) DO NOTHING", strings.Join(cols, ", "))
}

// conflictPartial builds `ON CONFLICT (cols) WHERE predicate DO NOTHING` for a
// partial-unique index (§B.9 ⚠).
func conflictPartial(predicate string, cols ...string) string {
	return fmt.Sprintf("ON CONFLICT (%s) WHERE %s DO NOTHING", strings.Join(cols, ", "), predicate)
}
