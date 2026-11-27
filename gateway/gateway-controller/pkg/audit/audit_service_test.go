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
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
)

func TestAuditServiceRecordsToOutput(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "audit.log")
	s, err := NewAuditService("gw-1", config.AuditConfig{
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

	var line map[string]any
	if err := json.Unmarshal(out, &line); err != nil {
		t.Fatalf("output line is not one JSON object: %v: %q", err, out)
	}
	if id, _ := line["id"].(string); id == "" {
		t.Errorf("id = %v, want a record id", line["id"])
	}
	if at, err := time.Parse(time.RFC3339Nano, fmt.Sprint(line["time"])); err != nil || at.Location() != time.UTC {
		t.Errorf("time = %v, want a UTC RFC 3339 time", line["time"])
	}
	for key, value := range map[string]string{
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
}

func TestAuditServiceNoOutput(t *testing.T) {
	s, err := NewAuditService("gw-1", config.AuditConfig{Enabled: true}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewAuditService error = %v", err)
	}

	s.Record("DELETE", "res-1", "petstore", "rest_api", OutcomeSucceeded, "admin", "corr-1") // must not panic

	if len(s.sinks) != 0 {
		t.Fatal("output sinks built although audit.outputs is unset")
	}
}

func TestDisabledAuditServiceRecordsNothing(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "audit.log")
	s, err := NewAuditService("gw-1", config.AuditConfig{
		Outputs: []string{config.AuditOutputFile},
		File:    config.AuditFileConfig{Path: logPath},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewAuditService error = %v", err)
	}

	s.Record("CREATE", "res-1", "petstore", "rest_api", OutcomeSucceeded, "admin", "corr-1")

	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("audit log stat error = %v, want no file while audit is disabled", err)
	}
}

func TestNilAuditServiceRecordsNothing(t *testing.T) {
	var s *AuditService
	s.Record("CREATE", "res-1", "petstore", "rest_api", OutcomeSucceeded, "admin", "corr-1") // must not panic
	if err := s.Close(context.Background()); err != nil {
		t.Fatalf("Close error = %v", err)
	}
}

func TestAuditServiceMasksSensitiveFields(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "audit.log")
	s, err := NewAuditService("gw-1", config.AuditConfig{
		Enabled:             true,
		MaskSensitiveFields: true,
		Outputs:             []string{config.AuditOutputFile},
		File:                config.AuditFileConfig{Path: logPath},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewAuditService error = %v", err)
	}

	s.Record("UPDATE", "res-1", "petstore", "rest_api", OutcomeSucceeded, "admin", "corr-1")
	s.Record("DELETE", "res-2", "petstore", "rest_api", OutcomeSucceeded, "", "corr-2")
	if err := s.Close(context.Background()); err != nil {
		t.Fatalf("Close error = %v", err)
	}

	out, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	var lines []recordJSON
	for _, raw := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		var line recordJSON
		if err := json.Unmarshal([]byte(raw), &line); err != nil {
			t.Fatalf("output line is not a JSON object: %v: %q", err, raw)
		}
		lines = append(lines, line)
	}
	if len(lines) != 2 {
		t.Fatalf("output lines = %d, want 2", len(lines))
	}
	if got := lines[0]; got.PerformedBy != "****" || got.ResourceName != "petstore" || got.ResourceID != "res-1" || got.CorrelationID != "corr-1" {
		t.Errorf("line = %+v, want only performedBy masked", got)
	}
	// An empty value stays empty, so "****" always stands for a real value.
	if got := lines[1].PerformedBy; got != "" {
		t.Errorf("line performedBy = %q, want the empty user left empty", got)
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
			s, err := NewAuditService("gw-1", tc.cfg, logger)
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
