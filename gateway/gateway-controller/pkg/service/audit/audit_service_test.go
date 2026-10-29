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

package audit_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/audit"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// auditRecorder is a store that keeps every row the DB sink writes; every other
// Storage method is unused.
type auditRecorder struct {
	storage.Storage
	rows chan *models.AuditLog
}

// newAuditRecorder returns an enabled AuditService whose DB sink writes to the returned recorder.
func newAuditRecorder() (*audit.AuditService, *auditRecorder) {
	r := &auditRecorder{rows: make(chan *models.AuditLog, 32)}
	return audit.NewAuditService(config.AuditConfig{Enabled: true}, "test-gateway", r, slog.New(slog.NewTextHandler(io.Discard, nil))), r
}

func (r *auditRecorder) CreateAuditLog(row *models.AuditLog) error {
	r.rows <- row
	return nil
}

// next returns the next recorded row. Record writes asynchronously, so it waits briefly.
func (r *auditRecorder) next(t *testing.T) *models.AuditLog {
	t.Helper()
	select {
	case e := <-r.rows:
		return e
	case <-time.After(2 * time.Second):
		t.Fatal("expected an audit event, got none")
		return nil
	}
}

// none fails the test when an event is recorded within a short wait.
func (r *auditRecorder) none(t *testing.T) {
	t.Helper()
	select {
	case e := <-r.rows:
		t.Fatalf("expected no audit event, got %s %s %s", e.Action, e.ResourceType, e.Outcome)
	case <-time.After(200 * time.Millisecond):
	}
}

func event() *audit.AuditEvent {
	return &audit.AuditEvent{
		Action:       models.AuditActionCreate,
		ResourceType: models.AuditResourceTypeRestAPI,
		Outcome:      models.AuditOutcomeSucceeded,
		ResourceName: "orders-v1",
	}
}

func TestAuditDiff(t *testing.T) {
	before := map[string]any{"displayName": "Orders", "version": "v1", "limit": nil, "tags": []string{"a"}}

	t.Run("records only changed fields as from/to", func(t *testing.T) {
		after := map[string]any{"displayName": "Orders 2", "version": "v1", "limit": 100, "tags": []string{"a"}}
		assert.Equal(t, map[string]any{
			"displayName": map[string]any{"from": "Orders", "to": "Orders 2"},
			"limit":       map[string]any{"from": nil, "to": 100},
		}, audit.AuditDiff(before, after))
	})

	t.Run("returns nil when nothing changed", func(t *testing.T) {
		assert.Nil(t, audit.AuditDiff(before, map[string]any{"displayName": "Orders", "version": "v1", "limit": nil, "tags": []string{"a"}}))
	})
}

func TestRecord_FillsServiceFields(t *testing.T) {
	svc, rec := newAuditRecorder()

	// The correlation middleware puts the request's X-Correlation-ID on the context.
	handler := middleware.CorrelationIDMiddleware(slog.New(slog.NewTextHandler(io.Discard, nil)))(
		http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { svc.Record(r.Context(), event()) }))
	req := httptest.NewRequest(http.MethodPost, "/rest-apis", nil)
	req.Header.Set("X-Correlation-ID", "corr-from-request")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	e := rec.next(t)
	assert.Equal(t, "test-gateway", e.GatewayID)
	assert.NotEmpty(t, e.UUID)
	assert.False(t, e.PerformedAt.IsZero())
	assert.Equal(t, "corr-from-request", e.CorrelationID)
}

func TestRecord_KeepsCallerCorrelationID(t *testing.T) {
	svc, rec := newAuditRecorder()
	e := event()
	e.CorrelationID = "corr-from-params"
	svc.Record(context.Background(), e)
	assert.Equal(t, "corr-from-params", rec.next(t).CorrelationID)
}

func TestRecord_SkipsWithoutAudit(t *testing.T) {
	svc, rec := newAuditRecorder()
	svc.Record(audit.WithoutAudit(context.Background()), event())
	rec.none(t)
}

func TestRecord_DropsEventsMissingRequiredFields(t *testing.T) {
	svc, rec := newAuditRecorder()
	for _, e := range []*audit.AuditEvent{
		{ResourceType: models.AuditResourceTypeRestAPI, Outcome: models.AuditOutcomeSucceeded},
		{Action: models.AuditActionCreate, Outcome: models.AuditOutcomeSucceeded},
		{Action: models.AuditActionCreate, ResourceType: models.AuditResourceTypeRestAPI},
	} {
		svc.Record(context.Background(), e)
	}
	rec.none(t)
}

func TestRecord_DisabledOrNilIsANoOp(t *testing.T) {
	_, rec := newAuditRecorder()
	disabled := audit.NewAuditService(config.AuditConfig{}, "test-gateway", rec, slog.New(slog.NewTextHandler(io.Discard, nil)))
	disabled.Record(context.Background(), event())
	rec.none(t)

	var nilService *audit.AuditService
	assert.NotPanics(t, func() { nilService.Record(context.Background(), event()) })
}

// captureStore records the row DBSink writes; every other Storage method is unused.
type captureStore struct {
	storage.Storage
	row *models.AuditLog
}

func (c *captureStore) CreateAuditLog(row *models.AuditLog) error {
	c.row = row
	return nil
}

func TestDBSink_StoresEventDataAsJSON(t *testing.T) {
	tests := []struct {
		name      string
		eventData any
		want      string
	}{
		{"map", map[string]any{"version": "v1"}, `{"version":"v1"}`},
		{"nil", nil, ""},
		{"empty diff", audit.AuditDiff(map[string]any{"a": 1}, map[string]any{"a": 1}), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &captureStore{}
			e := event()
			e.EventData = tt.eventData
			require.NoError(t, audit.NewDBSink(store).Write(context.Background(), e))
			assert.Equal(t, tt.want, store.row.EventData)
			assert.Equal(t, "orders-v1", store.row.ResourceName)
		})
	}
}

func TestFileSink_WritesCamelCaseJSONLines(t *testing.T) {
	dir := t.TempDir()
	sink, err := audit.NewFileSink(dir)
	require.NoError(t, err)

	e := event()
	e.UUID = "01a1-test"
	e.GatewayID = "gw-1"
	e.CorrelationID = "corr-1"
	e.EventData = map[string]any{"version": "v1"}
	require.NoError(t, sink.Write(context.Background(), e))
	require.NoError(t, sink.Close())

	data, err := os.ReadFile(filepath.Join(dir, "audit.log"))
	require.NoError(t, err)
	var line map[string]any
	require.NoError(t, json.Unmarshal(data, &line))
	assert.Equal(t, "01a1-test", line["id"])
	assert.Equal(t, "gw-1", line["gatewayId"])
	assert.Equal(t, "corr-1", line["correlationId"])
	assert.Equal(t, "create", line["action"])
	assert.Equal(t, "RestApi", line["resourceType"])
	assert.Equal(t, "orders-v1", line["resourceName"])
	assert.Equal(t, map[string]any{"version": "v1"}, line["eventData"])
	assert.NotContains(t, line, "UUID")
}

func TestStdoutSink_TagsLines(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	stdout := os.Stdout
	os.Stdout = w
	sink := audit.NewStdoutSink()
	os.Stdout = stdout

	require.NoError(t, sink.Write(context.Background(), event()))
	require.NoError(t, w.Close())
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	var line map[string]any
	require.NoError(t, json.Unmarshal(data, &line))
	assert.Equal(t, "audit", line["logType"])
	assert.Equal(t, "create", line["action"])
	assert.Equal(t, "orders-v1", line["resourceName"])
}

func TestNewAuditService_Sinks(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	fileAt := func(path string) config.AuditSinksConfig {
		return config.AuditSinksConfig{File: config.AuditFileSinkConfig{Enabled: true, Path: path}}
	}
	newStore := func() *auditRecorder { return &auditRecorder{rows: make(chan *models.AuditLog, 32)} }

	t.Run("disabled records nothing and creates no file", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "logs")
		store := newStore()
		audit.NewAuditService(config.AuditConfig{Sinks: fileAt(dir)}, "test-gateway", store, logger).Record(context.Background(), event())
		store.none(t)
		assert.NoDirExists(t, dir)
	})

	t.Run("database only by default", func(t *testing.T) {
		t.Chdir(t.TempDir())
		store := newStore()
		audit.NewAuditService(config.AuditConfig{Enabled: true}, "test-gateway", store, logger).Record(context.Background(), event())
		assert.Equal(t, "orders-v1", store.next(t).ResourceName)
		assert.NoDirExists(t, filepath.Join("data", "logs"))
	})

	t.Run("file sink writes audit.log in the configured directory", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "nested", "logs")
		store := newStore()
		audit.NewAuditService(config.AuditConfig{Enabled: true, Sinks: fileAt(dir)}, "test-gateway", store, logger).Record(context.Background(), event())
		store.next(t)
		assert.Eventually(t, func() bool {
			data, _ := os.ReadFile(filepath.Join(dir, "audit.log"))
			return strings.Contains(string(data), `"resourceName":"orders-v1"`)
		}, 2*time.Second, 20*time.Millisecond)
	})

	t.Run("empty path uses ./data/logs", func(t *testing.T) {
		t.Chdir(t.TempDir())
		audit.NewAuditService(config.AuditConfig{Enabled: true, Sinks: fileAt("")}, "test-gateway", newStore(), logger)
		assert.FileExists(t, filepath.Join("data", "logs", "audit.log"))
	})

	t.Run("stdout sink writes a tagged line", func(t *testing.T) {
		r, w, err := os.Pipe()
		require.NoError(t, err)
		stdout := os.Stdout
		os.Stdout = w
		cfg := config.AuditConfig{Enabled: true}
		cfg.Sinks.Stdout.Enabled = true
		svc := audit.NewAuditService(cfg, "test-gateway", newStore(), logger)
		os.Stdout = stdout

		svc.Record(context.Background(), event())
		require.NoError(t, r.SetReadDeadline(time.Now().Add(2*time.Second)))
		line, err := bufio.NewReader(r).ReadString('\n')
		require.NoError(t, err)
		var got map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &got))
		assert.Equal(t, "audit", got["logType"])
		assert.Equal(t, "orders-v1", got["resourceName"])
	})

	t.Run("file sink that cannot open is skipped, database still records", func(t *testing.T) {
		notADir := filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(notADir, nil, 0o600))
		store := newStore()
		audit.NewAuditService(config.AuditConfig{Enabled: true, Sinks: fileAt(notADir)}, "test-gateway", store, logger).Record(context.Background(), event())
		assert.Equal(t, "orders-v1", store.next(t).ResourceName)
	})
}
