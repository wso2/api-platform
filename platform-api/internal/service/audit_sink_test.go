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
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

func testAuditEvent() *AuditEvent {
	return &AuditEvent{
		OrganizationUUID: "org-1",
		UUID:             "01a1-test",
		Action:           "create",
		ResourceType:     "RestApi",
		Outcome:          "succeeded",
		ResourceName:     "orders-v1",
		EventData:        map[string]any{"version": "v1"},
	}
}

func TestFileSink_WritesCamelCaseJSONLines(t *testing.T) {
	dir := t.TempDir()
	sink, err := NewFileSink(dir)
	require.NoError(t, err)
	require.NoError(t, sink.Write(context.Background(), testAuditEvent()))
	require.NoError(t, sink.Close())

	data, err := os.ReadFile(filepath.Join(dir, "audit.log"))
	require.NoError(t, err)
	var line map[string]any
	require.NoError(t, json.Unmarshal(data, &line))
	assert.Equal(t, "01a1-test", line["id"])
	assert.Equal(t, "org-1", line["organizationId"])
	assert.Equal(t, "create", line["action"])
	assert.Equal(t, map[string]any{"version": "v1"}, line["eventData"])
	assert.NotContains(t, line, "logType")
}

func TestStdoutSink_TagsLines(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	stdout := os.Stdout
	os.Stdout = w
	sink := NewStdoutSink()
	os.Stdout = stdout

	require.NoError(t, sink.Write(context.Background(), testAuditEvent()))
	require.NoError(t, w.Close())
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	var line map[string]any
	require.NoError(t, json.Unmarshal(data, &line))
	assert.Equal(t, "audit", line["logType"])
	assert.Equal(t, "create", line["action"])
	assert.Equal(t, "org-1", line["organizationId"])
}

func TestNewAuditService_Sinks(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	fileSink := func(path string) config.AuditSinks {
		return config.AuditSinks{File: config.AuditFileSink{Enabled: true, Path: path}}
	}

	t.Run("disabled builds nothing and creates no file", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "logs")
		assert.Empty(t, NewAuditService(config.Audit{Sinks: fileSink(dir)}, nil, logger).sinks)
		assert.NoDirExists(t, dir)
	})

	t.Run("database only by default", func(t *testing.T) {
		sinks := NewAuditService(config.Audit{Enabled: true}, nil, logger).sinks
		require.Len(t, sinks, 1)
		assert.IsType(t, &DBSink{}, sinks[0])
	})

	t.Run("file and stdout when enabled", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "nested", "logs")
		cfg := config.Audit{Enabled: true, Sinks: fileSink(dir)}
		cfg.Sinks.Stdout.Enabled = true
		sinks := NewAuditService(cfg, nil, logger).sinks
		require.Len(t, sinks, 3)
		assert.IsType(t, &DBSink{}, sinks[0])
		assert.IsType(t, &FileSink{}, sinks[1])
		assert.IsType(t, &StdoutSink{}, sinks[2])
		assert.FileExists(t, filepath.Join(dir, "audit.log"))
		require.NoError(t, sinks[1].(*FileSink).Close())
	})

	t.Run("empty path uses ./data/logs", func(t *testing.T) {
		t.Chdir(t.TempDir())
		sinks := NewAuditService(config.Audit{Enabled: true, Sinks: fileSink("")}, nil, logger).sinks
		require.Len(t, sinks, 2)
		assert.FileExists(t, filepath.Join("data", "logs", "audit.log"))
		require.NoError(t, sinks[1].(*FileSink).Close())
	})

	t.Run("file sink that cannot open is skipped", func(t *testing.T) {
		notADir := filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(notADir, nil, 0o600))
		sinks := NewAuditService(config.Audit{Enabled: true, Sinks: fileSink(notADir)}, nil, logger).sinks
		require.Len(t, sinks, 1)
		assert.IsType(t, &DBSink{}, sinks[0])
	})
}

// auditLogRecorder keeps audit_log rows in memory, for tests that assert on what a
// service recorded.
type auditLogRecorder struct {
	mu   sync.Mutex
	rows []*model.AuditLog
}

func (r *auditLogRecorder) Create(row *model.AuditLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows = append(r.rows, row)
	return nil
}

func (r *auditLogRecorder) List(repository.AuditLogFilter, int, int) ([]*model.AuditLog, error) {
	return nil, nil
}

func (r *auditLogRecorder) Count(repository.AuditLogFilter) (int, error) { return 0, nil }

// wait returns the rows once at least n are recorded (Record writes asynchronously),
// or what is there after a short timeout.
func (r *auditLogRecorder) wait(n int) []*model.AuditLog {
	for deadline := time.Now().Add(2 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		r.mu.Lock()
		rows := append([]*model.AuditLog(nil), r.rows...)
		r.mu.Unlock()
		if len(rows) >= n || time.Now().After(deadline) {
			return rows
		}
	}
}

// newRecordingAuditService returns an enabled AuditService whose only sink is an
// in-memory recorder.
func newRecordingAuditService() (*AuditService, *auditLogRecorder) {
	rec := &auditLogRecorder{}
	return NewAuditService(config.Audit{Enabled: true}, rec, slog.New(slog.NewTextHandler(io.Discard, nil))), rec
}
