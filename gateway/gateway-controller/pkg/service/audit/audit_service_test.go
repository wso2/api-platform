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

package audit

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
)

type auditRow struct {
	id                                                             string
	performedAt                                                    time.Time
	action, resourceUUID, resourceType, performedBy, correlationID string
}

type recordingAuditTable struct {
	rows []auditRow
	err  error
}

func (t *recordingAuditTable) SaveAudit(id string, performedAt time.Time, action, resourceUUID, resourceType, performedBy, correlationID string) error {
	t.rows = append(t.rows, auditRow{id, performedAt, action, resourceUUID, resourceType, performedBy, correlationID})
	return t.err
}

func TestAuditServiceRecordsToDBAndOutput(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"insert succeeds", nil},
		{"insert fails", errors.New("insert failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := &recordingAuditTable{err: tc.err}
			var out bytes.Buffer
			s := &AuditService{enabled: true, store: db, gatewayID: "gw-1", out: slog.NewJSONHandler(&out, nil),
				logger: slog.New(slog.NewTextHandler(io.Discard, nil))}

			s.Record("CREATE", "res-1", "rest_api", "admin", "corr-1")

			if len(db.rows) != 1 {
				t.Fatalf("db rows = %d, want 1", len(db.rows))
			}
			row := db.rows[0]
			if row.id == "" || row.performedAt.Location() != time.UTC {
				t.Fatalf("row id = %q, performedAt = %v; want an id and a UTC time", row.id, row.performedAt)
			}
			if want := (auditRow{row.id, row.performedAt, "CREATE", "res-1", "rest_api", "admin", "corr-1"}); row != want {
				t.Fatalf("db row = %+v, want %+v", row, want)
			}

			var line map[string]any
			if err := json.Unmarshal(out.Bytes(), &line); err != nil {
				t.Fatalf("output line is not one JSON object: %v: %q", err, out.String())
			}
			for key, value := range map[string]string{
				"time":          row.performedAt.Format(time.RFC3339Nano),
				"id":            row.id,
				"logType":       "audit",
				"gatewayId":     "gw-1",
				"correlationId": "corr-1",
				"action":        "CREATE",
				"resourceType":  "rest_api",
				"resourceId":    "res-1",
				"performedBy":   "admin",
			} {
				if line[key] != value {
					t.Errorf("%s = %v, want %q", key, line[key], value)
				}
			}
		})
	}
}

func TestAuditServiceNoOutput(t *testing.T) {
	db := &recordingAuditTable{}
	s, err := NewAuditService(db, "gw-1", config.AuditConfig{Enabled: true}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}

	s.Record("DELETE", "res-1", "rest_api", "admin", "corr-1")

	if len(db.rows) != 1 {
		t.Fatalf("db rows = %d, want 1", len(db.rows))
	}
	if s.out != nil {
		t.Fatal("output handler built although audit.output is unset")
	}
}

func TestDisabledAuditServiceRecordsNothing(t *testing.T) {
	db := &recordingAuditTable{}
	s, err := NewAuditService(db, "gw-1", config.AuditConfig{Output: "stdout"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}

	s.Record("CREATE", "res-1", "rest_api", "admin", "corr-1")

	if len(db.rows) != 0 {
		t.Fatalf("db rows = %d, want 0 while audit is disabled", len(db.rows))
	}
}

func TestNilAuditServiceRecordsNothing(t *testing.T) {
	var s *AuditService
	s.Record("CREATE", "res-1", "rest_api", "admin", "corr-1") // must not panic
}
