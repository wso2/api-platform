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
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
)

type auditRow struct {
	id                                                                                    string
	performedAt                                                                           time.Time
	action, resourceUUID, resourceName, resourceType, outcome, performedBy, correlationID string
}

type recordingAuditTable struct {
	rows []auditRow
	err  error
}

func (t *recordingAuditTable) SaveAudit(id string, performedAt time.Time, action, resourceUUID, resourceName, resourceType, outcome, performedBy, correlationID string) error {
	t.rows = append(t.rows, auditRow{id, performedAt, action, resourceUUID, resourceName, resourceType, outcome, performedBy, correlationID})
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
			logPath := filepath.Join(t.TempDir(), "audit.log")
			s, err := NewAuditService(db, "gw-1", config.AuditConfig{
				Enabled: true,
				Outputs: []string{config.AuditOutputFile},
				File:    config.AuditFileConfig{Path: logPath, MaxSizeMB: 100},
			}, slog.New(slog.NewTextHandler(io.Discard, nil)))
			if err != nil {
				t.Fatalf("NewAuditService error = %v", err)
			}

			s.Record("CREATE", "res-1", "petstore", "rest_api", OutcomeSucceeded, "admin", "corr-1")
			if err := s.Close(context.Background()); err != nil {
				t.Fatalf("Close error = %v", err)
			}
			out, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatalf("reading the audit log: %v", err)
			}

			if len(db.rows) != 1 {
				t.Fatalf("db rows = %d, want 1", len(db.rows))
			}
			row := db.rows[0]
			if row.id == "" || row.performedAt.Location() != time.UTC {
				t.Fatalf("row id = %q, performedAt = %v; want an id and a UTC time", row.id, row.performedAt)
			}
			if want := (auditRow{row.id, row.performedAt, "CREATE", "res-1", "petstore", "rest_api", OutcomeSucceeded, "admin", "corr-1"}); row != want {
				t.Fatalf("db row = %+v, want %+v", row, want)
			}

			var line map[string]any
			if err := json.Unmarshal(out, &line); err != nil {
				t.Fatalf("output line is not one JSON object: %v: %q", err, out)
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
				"resourceName":  "petstore",
				"outcome":       OutcomeSucceeded,
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
		t.Fatalf("NewAuditService error = %v", err)
	}

	s.Record("DELETE", "res-1", "petstore", "rest_api", OutcomeSucceeded, "admin", "corr-1")

	if len(db.rows) != 1 {
		t.Fatalf("db rows = %d, want 1", len(db.rows))
	}
	if len(s.sinks) != 0 {
		t.Fatal("output sinks built although audit.outputs is unset")
	}
}

func TestDisabledAuditServiceRecordsNothing(t *testing.T) {
	db := &recordingAuditTable{}
	s, err := NewAuditService(db, "gw-1", config.AuditConfig{Outputs: []string{"stdout"}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewAuditService error = %v", err)
	}

	s.Record("CREATE", "res-1", "petstore", "rest_api", OutcomeSucceeded, "admin", "corr-1")

	if len(db.rows) != 0 {
		t.Fatalf("db rows = %d, want 0 while audit is disabled", len(db.rows))
	}
}

func TestNilAuditServiceRecordsNothing(t *testing.T) {
	var s *AuditService
	s.Record("CREATE", "res-1", "petstore", "rest_api", OutcomeSucceeded, "admin", "corr-1") // must not panic
	if err := s.Close(context.Background()); err != nil {
		t.Fatalf("Close error = %v", err)
	}
}

func TestAuditServiceMasksTheUserInTheLineOnly(t *testing.T) {
	db := &recordingAuditTable{}
	logPath := filepath.Join(t.TempDir(), "audit.log")
	s, err := NewAuditService(db, "gw-1", config.AuditConfig{
		Enabled: true,
		MaskPII: true,
		Outputs: []string{config.AuditOutputFile},
		File:    config.AuditFileConfig{Path: logPath},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewAuditService error = %v", err)
	}

	s.Record("UPDATE", "res-1", "petstore", "rest_api", OutcomeSucceeded, "admin", "corr-1")
	if err := s.Close(context.Background()); err != nil {
		t.Fatalf("Close error = %v", err)
	}

	out, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	var line recordJSON
	if err := json.Unmarshal(out, &line); err != nil {
		t.Fatalf("output line is not one JSON object: %v: %q", err, out)
	}
	if line.PerformedBy != "****" {
		t.Fatalf("line performedBy = %q, want it masked", line.PerformedBy)
	}
	if db.rows[0].performedBy != "admin" {
		t.Fatalf("row performedBy = %q, want the user", db.rows[0].performedBy)
	}
}

func TestAuditServiceTrimsValuesToTheColumnSizes(t *testing.T) {
	db := &recordingAuditTable{}
	s, err := NewAuditService(db, "gw-1", config.AuditConfig{Enabled: true}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewAuditService error = %v", err)
	}

	// The 255-byte cut falls inside the two-byte "é", which is then dropped.
	name := strings.Repeat("a", 254) + "é" + "tail"
	s.Record("CREATE", "res-1", name, "rest_api", OutcomeSucceeded, "admin", strings.Repeat("c", 70))
	s.Record("CREATE", "res-1", "petstore", "rest_api", OutcomeSucceeded, "admin", "corr-\xff")

	if got := db.rows[0].resourceName; got != strings.Repeat("a", 254) {
		t.Errorf("resourceName = %q (%d bytes), want 254 valid bytes", got, len(got))
	}
	if got := db.rows[0].correlationID; got != strings.Repeat("c", 64) {
		t.Errorf("correlationID = %q, want the first 64 bytes", got)
	}
	if got := db.rows[1].correlationID; got != "corr-" {
		t.Errorf("correlationID = %q, want the invalid UTF-8 removed", got)
	}
}

func TestNewAuditServiceFailsOnABadOutput(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, tc := range []struct {
		name    string
		cfg     config.AuditConfig
		wantErr string
	}{
		{"unknown output", config.AuditConfig{Enabled: true, Outputs: []string{"syslog"}}, `unknown output "syslog"`},
		// The file output is built first and must be closed again.
		{"later output fails", config.AuditConfig{
			Enabled: true,
			Outputs: []string{config.AuditOutputFile, config.AuditOutputHTTP},
			File:    config.AuditFileConfig{Path: filepath.Join(t.TempDir(), "audit.log")},
		}, "controller.audit.http"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := NewAuditService(&recordingAuditTable{}, "gw-1", tc.cfg, logger)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("NewAuditService = %v, %v; want an error containing %q", s, err, tc.wantErr)
			}
		})
	}
}

type closeFailingSink struct{ name string }

func (closeFailingSink) Write([]byte) {}

func (closeFailingSink) Close(context.Context) error { return errors.New("still flushing") }

func (c closeFailingSink) Name() string { return c.name }

func TestAuditServiceCloseReportsEveryOutput(t *testing.T) {
	s := &AuditService{sinks: []sink{closeFailingSink{"file"}, closeFailingSink{"http"}}}

	err := s.Close(context.Background())

	for _, want := range []string{"closing the file audit output", "closing the http audit output"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Close error = %v, want it to contain %q", err, want)
		}
	}
}
