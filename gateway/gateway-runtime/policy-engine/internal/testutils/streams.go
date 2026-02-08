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

package testutils

import (
	"context"
	"io"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"google.golang.org/grpc/metadata"
)

// MockExtProcStream implements extprocv3.ExternalProcessor_ProcessServer for testing.
type MockExtProcStream struct {
	Requests  []*extprocv3.ProcessingRequest
	Responses []*extprocv3.ProcessingResponse
	RecvIndex int
	RecvErr   error
	SendErr   error
	Ctx       context.Context
}

// NewMockExtProcStream creates a new MockExtProcStream with the given requests.
func NewMockExtProcStream(requests []*extprocv3.ProcessingRequest) *MockExtProcStream {
	return &MockExtProcStream{
		Requests:  requests,
		Responses: make([]*extprocv3.ProcessingResponse, 0),
		Ctx:       context.Background(),
	}
}

// Send records the response and returns any configured error.
func (m *MockExtProcStream) Send(resp *extprocv3.ProcessingResponse) error {
	if m.SendErr != nil {
		return m.SendErr
	}
	m.Responses = append(m.Responses, resp)
	return nil
}

// Recv returns the next request or EOF when exhausted.
func (m *MockExtProcStream) Recv() (*extprocv3.ProcessingRequest, error) {
	if m.RecvErr != nil {
		return nil, m.RecvErr
	}
	if m.RecvIndex >= len(m.Requests) {
		return nil, io.EOF
	}
	req := m.Requests[m.RecvIndex]
	m.RecvIndex++
	return req, nil
}

// SetHeader is a no-op implementation.
func (m *MockExtProcStream) SetHeader(metadata.MD) error { return nil }

// SendHeader is a no-op implementation.
func (m *MockExtProcStream) SendHeader(metadata.MD) error { return nil }

// SetTrailer is a no-op implementation.
func (m *MockExtProcStream) SetTrailer(metadata.MD) {}

// Context returns the stream context.
func (m *MockExtProcStream) Context() context.Context { return m.Ctx }

// SendMsg is a no-op implementation.
func (m *MockExtProcStream) SendMsg(interface{}) error { return nil }

// RecvMsg is a no-op implementation.
func (m *MockExtProcStream) RecvMsg(interface{}) error { return nil }

// WithContext sets a custom context for the stream.
func (m *MockExtProcStream) WithContext(ctx context.Context) *MockExtProcStream {
	m.Ctx = ctx
	return m
}

// WithRecvError configures an error to be returned on Recv.
func (m *MockExtProcStream) WithRecvError(err error) *MockExtProcStream {
	m.RecvErr = err
	return m
}

// WithSendError configures an error to be returned on Send.
func (m *MockExtProcStream) WithSendError(err error) *MockExtProcStream {
	m.SendErr = err
	return m
}
