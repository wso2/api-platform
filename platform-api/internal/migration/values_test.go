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
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestTextOrEmpty(t *testing.T) {
	if got := textOrEmpty(sql.NullString{}); got != "" {
		t.Errorf("NULL should become \"\", got %q", got)
	}
	if got := textOrEmpty(sql.NullString{String: "", Valid: true}); got != "" {
		t.Errorf("empty should stay \"\", got %q", got)
	}
	if got := textOrEmpty(sql.NullString{String: "text", Valid: true}); got != "text" {
		t.Errorf("value should pass through, got %q", got)
	}
}

// recordingQueryer captures every INSERT the transform issues.
type recordingQueryer struct {
	calls []execCall
}

type execCall struct {
	sql  string
	args []any
}

func (r *recordingQueryer) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, errors.New("unexpected query")
}

func (r *recordingQueryer) QueryRowContext(context.Context, string, ...any) *sql.Row { return nil }

func (r *recordingQueryer) ExecContext(_ context.Context, q string, args ...any) (sql.Result, error) {
	r.calls = append(r.calls, execCall{sql: q, args: args})
	return driver.RowsAffected(1), nil
}

// The secrets row an externalized credential produces must carry "" for
// description, not NULL: v2 scans that column into a plain string, and a NULL
// made every migrated secret unreadable by v2 (the 1.1.0 gateway fetch 500).
func TestTransformArtifactConfig_SecretsRowHasEmptyDescription(t *testing.T) {
	k := testKernels(t)
	in := map[string]any{"upstream": map[string]any{"main": map[string]any{
		"auth": map[string]any{"type": "api-key", "value": "sk-plaintext"},
	}}}
	blob, _ := json.Marshal(in)
	ts := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	rec := &recordingQueryer{}

	if _, err := k.transformArtifactConfig(context.Background(), rec, "LlmProvider", "art-1", "org-1", "actor-1", ts, blob, ""); err != nil {
		t.Fatalf("transform: %v", err)
	}
	var secrets *execCall
	for i := range rec.calls {
		if strings.HasPrefix(rec.calls[i].sql, "INSERT INTO secrets ") {
			secrets = &rec.calls[i]
		}
	}
	if secrets == nil {
		t.Fatalf("no secrets row written; calls: %v", rec.calls)
	}
	// Column order: uuid, organization_uuid, handle, display_name, description, ...
	if !strings.Contains(secrets.sql, "(uuid, organization_uuid, handle, display_name, description,") {
		t.Fatalf("unexpected secrets column order: %s", secrets.sql)
	}
	if desc, ok := secrets.args[4].(string); !ok || desc != "" {
		t.Errorf("description bind = %#v, want \"\" (NULL breaks v2's string scan)", secrets.args[4])
	}
	for i, a := range secrets.args[:4] {
		if a == nil {
			t.Errorf("arg %d of the secrets row is NULL", i)
		}
	}
}
