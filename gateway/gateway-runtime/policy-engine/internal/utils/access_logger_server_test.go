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

package utils

import (
	"context"
	"io"
	"testing"

	accesslogv3 "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
	v3 "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
)

// =============================================================================
// Mock Stream Implementation
// =============================================================================

type mockAccessLogStream struct {
	messages    []*v3.StreamAccessLogsMessage
	recvIdx     int
	recvErr     error
	ctx         context.Context
	sendCalled  bool
	closeCalled bool
}

func (m *mockAccessLogStream) Send(*v3.StreamAccessLogsResponse) error {
	m.sendCalled = true
	return nil
}

func (m *mockAccessLogStream) SendAndClose(*v3.StreamAccessLogsResponse) error {
	m.closeCalled = true
	return nil
}

func (m *mockAccessLogStream) Recv() (*v3.StreamAccessLogsMessage, error) {
	if m.recvErr != nil {
		return nil, m.recvErr
	}
	if m.recvIdx >= len(m.messages) {
		return nil, io.EOF
	}
	msg := m.messages[m.recvIdx]
	m.recvIdx++
	return msg, nil
}

func (m *mockAccessLogStream) SetHeader(metadata.MD) error  { return nil }
func (m *mockAccessLogStream) SendHeader(metadata.MD) error { return nil }
func (m *mockAccessLogStream) SetTrailer(metadata.MD)       {}
func (m *mockAccessLogStream) Context() context.Context     { return m.ctx }
func (m *mockAccessLogStream) SendMsg(interface{}) error    { return nil }
func (m *mockAccessLogStream) RecvMsg(interface{}) error    { return nil }

// =============================================================================
// Test Helper Functions
// =============================================================================

func createTestConfig() *config.Config {
	return &config.Config{
		Analytics: config.AnalyticsConfig{
			Enabled:           false,
			EnabledPublishers: []string{},
			Publishers:        config.AnalyticsPublishersConfig{},
		},
	}
}

// =============================================================================
// newAccessLogServiceServer Tests
// =============================================================================

func TestNewAccessLogServiceServer(t *testing.T) {
	cfg := createTestConfig()

	server := newAccessLogServiceServer(cfg, nil, nil)

	require.NotNil(t, server)
	assert.NotNil(t, server.cfg)
	assert.NotNil(t, server.analytics)
}

// =============================================================================
// StreamAccessLogs Tests
// =============================================================================

func TestStreamAccessLogs_EmptyStream(t *testing.T) {
	cfg := createTestConfig()
	server := newAccessLogServiceServer(cfg, nil, nil)

	stream := &mockAccessLogStream{
		messages: []*v3.StreamAccessLogsMessage{},
		ctx:      context.Background(),
	}

	err := server.StreamAccessLogs(stream)

	assert.NoError(t, err) // EOF is expected, not an error
}

func TestStreamAccessLogs_RecvError(t *testing.T) {
	cfg := createTestConfig()
	server := newAccessLogServiceServer(cfg, nil, nil)

	expectedErr := io.ErrUnexpectedEOF
	stream := &mockAccessLogStream{
		messages: []*v3.StreamAccessLogsMessage{},
		recvErr:  expectedErr,
		ctx:      context.Background(),
	}

	err := server.StreamAccessLogs(stream)

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestStreamAccessLogs_WithNilHttpLogs(t *testing.T) {
	cfg := createTestConfig()
	server := newAccessLogServiceServer(cfg, nil, nil)

	stream := &mockAccessLogStream{
		messages: []*v3.StreamAccessLogsMessage{
			{}, // Empty message with nil HttpLogs
		},
		ctx: context.Background(),
	}

	err := server.StreamAccessLogs(stream)

	assert.NoError(t, err)
}

func TestStreamAccessLogs_WithHttpLogs(t *testing.T) {
	cfg := createTestConfig()
	server := newAccessLogServiceServer(cfg, nil, nil)

	stream := &mockAccessLogStream{
		messages: []*v3.StreamAccessLogsMessage{
			{
				LogEntries: &v3.StreamAccessLogsMessage_HttpLogs{
					HttpLogs: &v3.StreamAccessLogsMessage_HTTPAccessLogEntries{
						LogEntry: []*accesslogv3.HTTPAccessLogEntry{
							{},
						},
					},
				},
			},
		},
		ctx: context.Background(),
	}

	err := server.StreamAccessLogs(stream)

	assert.NoError(t, err)
}

func TestStreamAccessLogs_MultipleMessages(t *testing.T) {
	cfg := createTestConfig()
	server := newAccessLogServiceServer(cfg, nil, nil)

	stream := &mockAccessLogStream{
		messages: []*v3.StreamAccessLogsMessage{
			{},
			{
				LogEntries: &v3.StreamAccessLogsMessage_HttpLogs{
					HttpLogs: &v3.StreamAccessLogsMessage_HTTPAccessLogEntries{
						LogEntry: []*accesslogv3.HTTPAccessLogEntry{
							{},
							{},
						},
					},
				},
			},
			{},
		},
		ctx: context.Background(),
	}

	err := server.StreamAccessLogs(stream)

	assert.NoError(t, err)
}

// =============================================================================
// StartAccessLogServiceServer Tests
// =============================================================================

func TestStartAccessLogServiceServer_TCP(t *testing.T) {
	cfg := &config.Config{
		Collector: config.CollectorConfig{
			Server: config.AccessLogsServiceConfig{
				Mode:                  "tcp",
				ServerPort:            19001, // Deprecated override; also exercises backward compat
				ALSPlainText:          true,
				ExtProcMaxMessageSize: 1024 * 1024 * 4,
				ExtProcMaxHeaderLimit: 8192,
			},
		},
	}

	// Start the server
	als, err := StartAccessLogServiceServer(cfg, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, als)
	require.NotNil(t, als.Server)
	require.NotNil(t, als.Verdicts,
		"the failover verdict consumer is registered whether or not the collector is enabled")

	// Gracefully stop the server
	als.Server.GracefulStop()
}

// TestStartAccessLogServiceServer_ReturnsErrorRatherThanPanicking pins the behaviour the two
// callers depend on: a deployment that enabled analytics must fail loudly at boot, while one
// that started this receiver only so failover can grade mid-stream failures must degrade
// instead. Both need the error back; a panic would force the first choice on both.
func TestStartAccessLogServiceServer_ReturnsErrorRatherThanPanicking(t *testing.T) {
	cfg := &config.Config{
		Collector: config.CollectorConfig{
			Server: config.AccessLogsServiceConfig{
				Mode:                  "tcp",
				ServerPort:            19002,
				ALSPlainText:          true,
				ExtProcMaxMessageSize: 1024 * 1024 * 4,
				ExtProcMaxHeaderLimit: 0, // never validated when the collector is disabled
			},
		},
	}

	var als *AccessLogService
	var err error
	require.NotPanics(t, func() { als, err = StartAccessLogServiceServer(cfg, nil, nil) })
	require.Error(t, err)
	assert.Nil(t, als)
	assert.Contains(t, err.Error(), "max_header_limit")
}

// TestNewAccessLogServiceServer_ConsumerRegistration covers which consumers run.
//
// Analytics is registered only when the collector is enabled: Process() builds a full analytic
// event before discovering there are no publishers to take it, and logs an error for any entry
// that does not fit its expected shape — so running it in a deployment whose stream exists purely
// for failover grading would spend real work per entry and emit misleading errors.
func TestNewAccessLogServiceServer_ConsumerRegistration(t *testing.T) {
	names := func(s *AccessLogServiceServer) []string {
		got := make([]string, 0, len(s.consumers))
		for _, c := range s.consumers {
			got = append(got, c.Name())
		}
		return got
	}

	collectorOff := &config.Config{}
	require.False(t, collectorOff.IsCollectorEnabled(),
		"both collector inputs default off — the premise FR-030o rests on")
	assert.Equal(t, []string{"failover-attempt-verdict"}, names(newAccessLogServiceServer(collectorOff, nil, nil)),
		"failover grading works with analytics unconfigured")

	collectorOn := &config.Config{Analytics: config.AnalyticsConfig{Enabled: true}}
	require.True(t, collectorOn.IsCollectorEnabled())
	assert.Equal(t, []string{"failover-attempt-verdict", "analytics"}, names(newAccessLogServiceServer(collectorOn, nil, nil)),
		"the collector's consumer joins the one that is always present")
}

func TestStartAccessLogServiceServer_UDS(t *testing.T) {
	// Create a temp directory for the socket
	tmpDir := t.TempDir()
	// Override the default socket path for testing by using TCP fallback
	// Note: UDS mode uses constants.DefaultALSSocketPath which is /var/run/api-platform/gateway-analytics.sock
	// For unit tests, we just verify the server starts and stops cleanly

	cfg := &config.Config{
		Analytics: config.AnalyticsConfig{
			Enabled:           false,
			EnabledPublishers: []string{},
			Publishers:        config.AnalyticsPublishersConfig{},
			AccessLogsServiceCfg: config.AccessLogsServiceConfig{
				Mode:                  "uds",
				ALSPlainText:          true,
				ExtProcMaxMessageSize: 1024 * 1024 * 4,
				ExtProcMaxHeaderLimit: 8192,
			},
		},
	}

	// For UDS, the socket path is /var/run/api-platform/gateway-analytics.sock (constant).
	// In a unit test environment, /var/run/api-platform/ may not exist. We test the TCP path
	// thoroughly and test UDS only if the directory is writable.
	_ = tmpDir
	_ = cfg
	t.Skip("UDS test requires /app/ directory - covered by integration tests")
}

func TestStartAccessLogServiceServer_DefaultMode(t *testing.T) {
	// Empty mode should default to UDS
	cfg := &config.Config{
		Analytics: config.AnalyticsConfig{
			Enabled:           false,
			EnabledPublishers: []string{},
			Publishers:        config.AnalyticsPublishersConfig{},
			AccessLogsServiceCfg: config.AccessLogsServiceConfig{
				Mode:                  "", // Empty defaults to UDS
				ALSPlainText:          true,
				ExtProcMaxMessageSize: 1024 * 1024 * 4,
				ExtProcMaxHeaderLimit: 8192,
			},
		},
	}

	// Same as UDS - requires /app/ directory
	_ = cfg
	t.Skip("Default (UDS) test requires /app/ directory - covered by integration tests")
}

func TestStartAccessLogServiceServer_WithTLS(t *testing.T) {
	// This test would require valid TLS certs, which is complex to set up
	// The main coverage is achieved by the TCP test
	t.Skip("TLS test requires valid certificates - covered by integration tests")
}
