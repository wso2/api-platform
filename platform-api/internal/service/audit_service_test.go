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

type staticIdentity map[string]string

func (m staticIdentity) DisplayIdentity(internalUserID string) (string, error) {
	return m[internalUserID], nil
}

func TestAuditServiceRecordsToTableAndOutput(t *testing.T) {
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
			logPath := filepath.Join(t.TempDir(), "audit.log")
			s, err := NewAuditService(db, staticIdentity{"user-1": "alice"}, config.Audit{
				Enabled: true,
				Outputs: []string{config.AuditOutputFile},
				File:    config.AuditFile{Path: logPath, MaxSizeMB: 100},
			}, slog.New(slog.NewTextHandler(io.Discard, nil)))
			if err != nil {
				t.Fatalf("NewAuditService error = %v", err)
			}

			err = s.Record("CREATE", "res-1", "rest_api", "org-1", "user-1")
			if !errors.Is(err, tc.err) {
				t.Fatalf("Record error = %v, want %v", err, tc.err)
			}
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
			if row.id == "" || row.performedAt.IsZero() || row.performedAt.Location() != time.UTC {
				t.Fatalf("row id = %q, performedAt = %v; want an id and a UTC time", row.id, row.performedAt)
			}
			if want := (auditRow{row.id, row.performedAt, "CREATE", "res-1", "rest_api", "org-1", "user-1"}); row != want {
				t.Fatalf("db row = %+v, want %+v", row, want)
			}

			var line map[string]any
			if err := json.Unmarshal(out, &line); err != nil {
				t.Fatalf("output line is not one JSON object: %v: %q", err, out)
			}
			for key, value := range map[string]string{
				"time":           row.performedAt.Format(time.RFC3339Nano),
				"id":             row.id,
				"logType":        "audit",
				"action":         "CREATE",
				"resourceType":   "rest_api",
				"resourceId":     "res-1",
				"organizationId": "org-1",
				"performedBy":    "alice",
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
	s, err := NewAuditService(db, nil, config.Audit{Enabled: true}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewAuditService error = %v", err)
	}

	if err := s.Record("DELETE", "res-1", "project", "org-1", "user-1"); err != nil {
		t.Fatalf("Record error = %v", err)
	}
	if len(db.rows) != 1 {
		t.Fatalf("db rows = %d, want 1", len(db.rows))
	}
}

type unresolvableIdentity struct{}

func (unresolvableIdentity) DisplayIdentity(string) (string, error) {
	return "", errors.New("user lookup failed")
}

func TestAuditServiceLineHidesTheInternalUserID(t *testing.T) {
	for _, tc := range []struct {
		name     string
		mask     bool
		identity auditIdentity
		wantBy   string
		wantLog  string
	}{
		{"mask_sensitive_fields", true, staticIdentity{"user-1": "alice"}, "****", ""},
		{"identity lookup fails", false, unresolvableIdentity{}, "", "Failed to resolve the audit actor; leaving it empty in the output"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := &recordingAuditTable{}
			logPath := filepath.Join(t.TempDir(), "audit.log")
			logs := &auditLogBuffer{}
			s, err := NewAuditService(db, tc.identity, config.Audit{
				Enabled:             true,
				MaskSensitiveFields: tc.mask,
				Outputs:             []string{config.AuditOutputFile},
				File:                config.AuditFile{Path: logPath},
			}, slog.New(slog.NewTextHandler(logs, nil)))
			if err != nil {
				t.Fatalf("NewAuditService error = %v", err)
			}

			if err := s.Record("UPDATE", "res-1", "rest_api", "org-1", "user-1"); err != nil {
				t.Fatalf("Record error = %v", err)
			}
			if err := s.Close(context.Background()); err != nil {
				t.Fatalf("Close error = %v", err)
			}

			out, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			var line auditRecordJSON
			if err := json.Unmarshal(out, &line); err != nil {
				t.Fatalf("output line is not one JSON object: %v: %q", err, out)
			}
			if line.PerformedBy != tc.wantBy {
				t.Errorf("line performedBy = %q, want %q", line.PerformedBy, tc.wantBy)
			}
			if db.rows[0].performedBy != "user-1" {
				t.Errorf("row performedBy = %q, want the internal user id", db.rows[0].performedBy)
			}
			if tc.wantLog != "" && !strings.Contains(logs.String(), tc.wantLog) {
				t.Errorf("log lacks %q:\n%s", tc.wantLog, logs)
			}
		})
	}
}

func TestDisabledAuditServiceRecordsNothing(t *testing.T) {
	db := &recordingAuditTable{}
	s, err := NewAuditService(db, nil, config.Audit{Outputs: []string{"stdout"}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewAuditService error = %v", err)
	}

	if err := s.Record("CREATE", "res-1", "rest_api", "org-1", "user-1"); err != nil {
		t.Fatalf("Record error = %v", err)
	}
	if len(db.rows) != 0 {
		t.Fatalf("db rows = %d, want 0 while audit is disabled", len(db.rows))
	}
}

func TestNewAuditServiceFailsOnABadOutput(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, tc := range []struct {
		name    string
		cfg     config.Audit
		wantErr string
	}{
		{"unknown output", config.Audit{Enabled: true, Outputs: []string{"syslog"}}, `unknown output "syslog"`},
		// The file output is built first and must be closed again.
		{"later output fails", config.Audit{
			Enabled: true,
			Outputs: []string{config.AuditOutputFile, config.AuditOutputHTTP},
			File:    config.AuditFile{Path: filepath.Join(t.TempDir(), "audit.log")},
		}, "audit.http"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := NewAuditService(&recordingAuditTable{}, nil, tc.cfg, logger)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("NewAuditService = %v, %v; want an error containing %q", s, err, tc.wantErr)
			}
		})
	}
}

type closeFailingAuditSink struct{ name string }

func (closeFailingAuditSink) Write([]byte) {}

func (closeFailingAuditSink) Close(context.Context) error { return errors.New("still flushing") }

func (c closeFailingAuditSink) Name() string { return c.name }

func TestAuditServiceCloseReportsEveryOutput(t *testing.T) {
	s := &AuditService{sinks: []auditSink{closeFailingAuditSink{"file"}, closeFailingAuditSink{"http"}}}

	err := s.Close(context.Background())

	for _, want := range []string{"closing the file audit output", "closing the http audit output"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Close error = %v, want it to contain %q", err, want)
		}
	}
}
