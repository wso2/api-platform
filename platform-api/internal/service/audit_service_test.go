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

package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
)

type auditRow struct {
	id                                                       string
	performedAt                                              time.Time
	action, resourceUUID, resourceType, orgUUID, performedBy string
}

type recordingAuditTable struct {
	rows []auditRow
	err  error
}

func (t *recordingAuditTable) Record(id string, performedAt time.Time, action, resourceUUID, resourceType, orgUUID, performedBy string) error {
	t.rows = append(t.rows, auditRow{id, performedAt, action, resourceUUID, resourceType, orgUUID, performedBy})
	return t.err
}

func TestAuditServiceRecordsToDBAndOutput(t *testing.T) {
	insertErr := errors.New("insert failed")
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"insert succeeds", nil},
		{"insert fails", insertErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := &recordingAuditTable{err: tc.err}
			var out bytes.Buffer
			s := &AuditService{repo: db, out: slog.NewJSONHandler(&out, nil), logger: slog.New(slog.NewTextHandler(io.Discard, nil))}

			err := s.Record("CREATE", "res-1", "rest_api", "org-1", "user-1")
			if !errors.Is(err, tc.err) {
				t.Fatalf("Record error = %v, want %v", err, tc.err)
			}

			if len(db.rows) != 1 {
				t.Fatalf("db rows = %d, want 1", len(db.rows))
			}
			row := db.rows[0]
			if row.id == "" || row.performedAt.IsZero() || row.performedAt.Location() != time.UTC {
				t.Fatalf("row id = %q, performedAt = %v; want an id and a UTC time", row.id, row.performedAt)
			}
			if want := (auditRow{row.id, row.performedAt, "CREATE", "res-1", "rest_api", "org-1", "user-1"}); row != want {
				t.Fatalf("db row = %+v, want %+v", row, want)
			}

			var line map[string]any
			if err := json.Unmarshal(out.Bytes(), &line); err != nil {
				t.Fatalf("output line is not one JSON object: %v: %q", err, out.String())
			}
			for key, value := range map[string]string{
				"time":           row.performedAt.Format(time.RFC3339Nano),
				"id":             row.id,
				"logType":        "audit",
				"action":         "CREATE",
				"resourceType":   "rest_api",
				"resourceId":     "res-1",
				"organizationId": "org-1",
				"performedBy":    "user-1",
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
	s, err := NewAuditService(db, config.Audit{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Record("DELETE", "res-1", "project", "org-1", "user-1"); err != nil {
		t.Fatalf("Record error = %v", err)
	}
	if len(db.rows) != 1 {
		t.Fatalf("db rows = %d, want 1", len(db.rows))
	}
	if s.out != nil {
		t.Fatal("output handler built although audit.output is unset")
	}
}
