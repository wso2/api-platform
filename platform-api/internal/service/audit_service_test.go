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
	"log/slog"
	"testing"

	"github.com/wso2/api-platform/platform-api/config"
)

func TestAuditServiceRecordsToDBAndStdout(t *testing.T) {
	insertErr := errors.New("insert failed")
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"insert succeeds", nil},
		{"insert fails", insertErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := &recordingAuditRepo{err: tc.err}
			var out bytes.Buffer
			s := &AuditService{repo: db, stdout: slog.New(slog.NewJSONHandler(&out, nil))}

			err := s.Record("CREATE", "res-1", "rest_api", "org-1", "user-1")
			if !errors.Is(err, tc.err) {
				t.Fatalf("Record error = %v, want %v", err, tc.err)
			}

			want := auditCall{"CREATE", "res-1", "rest_api", "org-1", "user-1"}
			if len(db.calls) != 1 || db.calls[0] != want {
				t.Fatalf("db calls = %+v, want [%+v]", db.calls, want)
			}

			var line map[string]any
			if err := json.Unmarshal(out.Bytes(), &line); err != nil {
				t.Fatalf("stdout line is not one JSON object: %v: %q", err, out.String())
			}
			for key, value := range map[string]string{
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
			if _, ok := line["time"]; !ok {
				t.Error("line has no time")
			}
		})
	}
}

func TestAuditServiceStdoutOff(t *testing.T) {
	db := &recordingAuditRepo{}
	s := NewAuditService(db, config.Audit{})

	if err := s.Record("DELETE", "res-1", "project", "org-1", "user-1"); err != nil {
		t.Fatalf("Record error = %v", err)
	}
	if len(db.calls) != 1 {
		t.Fatalf("db calls = %d, want 1", len(db.calls))
	}
	if s.stdout != nil {
		t.Fatal("stdout logger built although audit.stdout is off")
	}
}
