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

	v3 "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v3"
	accesslogv3 "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
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
			Enabled:    false,
			Publishers: nil,
		},
	}
}

// =============================================================================
// newAccessLogServiceServer Tests
// =============================================================================

func TestNewAccessLogServiceServer(t *testing.T) {
	cfg := createTestConfig()

	server := newAccessLogServiceServer(cfg)

	require.NotNil(t, server)
	assert.NotNil(t, server.cfg)
	assert.NotNil(t, server.analytics)
}

// =============================================================================
// StreamAccessLogs Tests
// =============================================================================

func TestStreamAccessLogs_EmptyStream(t *testing.T) {
	cfg := createTestConfig()
	server := newAccessLogServiceServer(cfg)

	stream := &mockAccessLogStream{
		messages: []*v3.StreamAccessLogsMessage{},
		ctx:      context.Background(),
	}

	err := server.StreamAccessLogs(stream)

	assert.NoError(t, err) // EOF is expected, not an error
}

func TestStreamAccessLogs_RecvError(t *testing.T) {
	cfg := createTestConfig()
	server := newAccessLogServiceServer(cfg)

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
	server := newAccessLogServiceServer(cfg)

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
	server := newAccessLogServiceServer(cfg)

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
	server := newAccessLogServiceServer(cfg)

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
