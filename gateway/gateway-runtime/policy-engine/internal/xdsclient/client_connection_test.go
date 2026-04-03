/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package xdsclient

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"testing"
	"time"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/anypb"
)

// TestClient_Dial_InsecureConnection tests dialing with insecure credentials
func TestClient_Dial_InsecureConnection(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()
	config.TLSEnabled = false
	config.ServerAddress = "invalid-server:99999"
	config.ConnectTimeout = 100 * time.Millisecond

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)

	// Attempt to dial (will fail due to invalid server, but we test the path)
	conn, err := client.dial()

	// Should get an error due to invalid server
	assert.Error(t, err)
	assert.Nil(t, conn)
}

// TestClient_Dial_TLSConnectionWithValidCerts tests dialing with TLS enabled
func TestClient_Dial_TLSConnectionWithValidCerts(t *testing.T) {
	// Create temp directory for test certs
	tmpDir := t.TempDir()

	// Generate test certificates
	ca, caPrivKey := generateTestCA(t)
	cert, certPrivKey := generateTestCert(t, ca, caPrivKey)

	// Write certificates to files
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")
	caPath := filepath.Join(tmpDir, "ca.pem")

	writeCertToFile(t, cert, certPath)
	writeKeyToFile(t, certPrivKey, keyPath)
	writeCertToFile(t, ca, caPath)

	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()
	config.TLSEnabled = true
	config.TLSCertPath = certPath
	config.TLSKeyPath = keyPath
	config.TLSCAPath = caPath
	config.ServerAddress = "invalid-server:99999"
	config.ConnectTimeout = 100 * time.Millisecond

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)

	// Attempt to dial (will fail due to invalid server, but TLS config should load)
	conn, err := client.dial()

	// Should get an error due to invalid server
	assert.Error(t, err)
	assert.Nil(t, conn)
}

// TestClient_Dial_TLSConnectionWithInvalidCerts tests error handling with invalid TLS certs
func TestClient_Dial_TLSConnectionWithInvalidCerts(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()
	config.TLSEnabled = true
	config.TLSCertPath = "/nonexistent/cert.pem"
	config.TLSKeyPath = "/nonexistent/key.pem"
	config.TLSCAPath = "/nonexistent/ca.pem"
	config.ServerAddress = "invalid-server:99999"
	config.ConnectTimeout = 100 * time.Millisecond

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)

	// Attempt to dial
	conn, err := client.dial()

	// Should get TLS config error
	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "failed to load TLS config")
}

// TestClient_Dial_TimeoutApplied tests that connection timeout is applied
func TestClient_Dial_TimeoutApplied(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()
	config.ServerAddress = "192.0.2.1:99999" // Non-routable IP (TEST-NET-1)
	config.ConnectTimeout = 200 * time.Millisecond

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)

	start := time.Now()
	conn, err := client.dial()
	elapsed := time.Since(start)

	// Should timeout within reasonable bounds
	assert.Error(t, err)
	assert.Nil(t, conn)
	// Elapsed time should be close to timeout (with some margin for overhead)
	assert.Less(t, elapsed, 500*time.Millisecond, "Should timeout within configured duration")
}

// TestClient_Dial_ContextCancellation tests dial respects context cancellation
func TestClient_Dial_ContextCancellation(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()
	config.ServerAddress = "192.0.2.1:99999" // Non-routable IP
	config.ConnectTimeout = 10 * time.Second // Long timeout

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)

	// Cancel context before dial
	client.cancel()

	conn, err := client.dial()

	// Should fail due to cancelled context
	assert.Error(t, err)
	assert.Nil(t, conn)
}

// TestClient_SendDiscoveryRequest_AllTypes tests sending discovery requests for all resource types
func TestClient_SendDiscoveryRequest_AllTypes(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)

	// Create a mock stream
	mockStream := &mockADSStream{
		sentRequests: make([]*discoveryv3.DiscoveryRequest, 0),
	}

	client.mu.Lock()
	client.stream = mockStream
	client.mu.Unlock()

	// Send discovery requests
	err = client.sendDiscoveryRequest("1.0", "nonce-123")
	require.NoError(t, err)

	// Should have sent 5 requests (PolicyChain, APIKey, LazyResource, SubscriptionState, RouteConfig)
	assert.Len(t, mockStream.sentRequests, 5)

	// Verify request types
	typeURLs := make(map[string]bool)
	for _, req := range mockStream.sentRequests {
		typeURLs[req.TypeUrl] = true
		assert.Equal(t, "policy-engine", req.Node.Id)
		assert.Equal(t, "policy-engine-cluster", req.Node.Cluster)
		assert.Equal(t, "nonce-123", req.ResponseNonce)
	}

	assert.True(t, typeURLs[PolicyChainTypeURL], "Should send PolicyChain request")
	assert.True(t, typeURLs[APIKeyStateTypeURL], "Should send APIKey request")
	assert.True(t, typeURLs[LazyResourceTypeURL], "Should send LazyResource request")
	assert.True(t, typeURLs[SubscriptionStateTypeURL], "Should send SubscriptionState request")
	assert.True(t, typeURLs[RouteConfigTypeURL], "Should send RouteConfig request")
}

// TestClient_SendDiscoveryRequest_NoStream tests error when stream is not available
func TestClient_SendDiscoveryRequest_NoStream(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)

	// No stream set
	err = client.sendDiscoveryRequest("", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stream is not available")
}

// TestClient_SendDiscoveryRequestForType_PolicyChain tests sending request for specific type
func TestClient_SendDiscoveryRequestForType_PolicyChain(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)

	mockStream := &mockADSStream{
		sentRequests: make([]*discoveryv3.DiscoveryRequest, 0),
	}

	client.mu.Lock()
	client.stream = mockStream
	client.mu.Unlock()

	// Send request for PolicyChain type
	err = client.sendDiscoveryRequestForType(PolicyChainTypeURL, "1.0", "nonce-456")
	require.NoError(t, err)

	assert.Len(t, mockStream.sentRequests, 1)
	req := mockStream.sentRequests[0]
	assert.Equal(t, PolicyChainTypeURL, req.TypeUrl)
	assert.Equal(t, "1.0", req.VersionInfo)
	assert.Equal(t, "nonce-456", req.ResponseNonce)
	assert.Equal(t, "policy-engine", req.Node.Id)
}

// TestClient_SendDiscoveryRequestForType_APIKey tests sending request for API key type
func TestClient_SendDiscoveryRequestForType_APIKey(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)

	mockStream := &mockADSStream{
		sentRequests: make([]*discoveryv3.DiscoveryRequest, 0),
	}

	client.mu.Lock()
	client.stream = mockStream
	client.mu.Unlock()

	err = client.sendDiscoveryRequestForType(APIKeyStateTypeURL, "2.0", "nonce-789")
	require.NoError(t, err)

	assert.Len(t, mockStream.sentRequests, 1)
	req := mockStream.sentRequests[0]
	assert.Equal(t, APIKeyStateTypeURL, req.TypeUrl)
	assert.Equal(t, "2.0", req.VersionInfo)
	assert.Equal(t, "nonce-789", req.ResponseNonce)
}

// TestClient_SendDiscoveryRequestForType_LazyResource tests sending request for lazy resource type
func TestClient_SendDiscoveryRequestForType_LazyResource(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)

	mockStream := &mockADSStream{
		sentRequests: make([]*discoveryv3.DiscoveryRequest, 0),
	}

	client.mu.Lock()
	client.stream = mockStream
	client.mu.Unlock()

	err = client.sendDiscoveryRequestForType(LazyResourceTypeURL, "3.0", "nonce-abc")
	require.NoError(t, err)

	assert.Len(t, mockStream.sentRequests, 1)
	req := mockStream.sentRequests[0]
	assert.Equal(t, LazyResourceTypeURL, req.TypeUrl)
	assert.Equal(t, "3.0", req.VersionInfo)
	assert.Equal(t, "nonce-abc", req.ResponseNonce)
}

// TestClient_SendDiscoveryRequestForType_NoStream tests error when stream is nil
func TestClient_SendDiscoveryRequestForType_NoStream(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)

	// No stream set
	err = client.sendDiscoveryRequestForType(PolicyChainTypeURL, "1.0", "nonce")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stream is not available")
}

// TestClient_ProcessStream_ErrorHandling_Timeout tests stream processing with Recv timeout
func TestClient_ProcessStream_ErrorHandling_Timeout(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)

	mockStream := &mockADSStream{
		recvError: context.DeadlineExceeded,
	}

	err = client.processStream(mockStream)
	assert.Error(t, err)
}

// TestClient_ProcessStream_ErrorHandling_EOF tests stream processing with EOF
func TestClient_ProcessStream_ErrorHandling_EOF(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)

	mockStream := &mockADSStream{
		recvError: io.EOF,
	}

	err = client.processStream(mockStream)
	assert.Equal(t, io.EOF, err)
}

// TestClient_ProcessStream_ErrorHandling_NetworkError tests stream processing with network error
func TestClient_ProcessStream_ErrorHandling_NetworkError(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)

	networkErr := fmt.Errorf("network connection lost")
	mockStream := &mockADSStream{
		recvError: networkErr,
	}

	err = client.processStream(mockStream)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error receiving from stream")
}

// TestClient_SendDiscoveryRequest_VersionTracking tests that versions are tracked per type
func TestClient_SendDiscoveryRequest_VersionTracking(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)

	mockStream := &mockADSStream{
		sentRequests: make([]*discoveryv3.DiscoveryRequest, 0),
	}

	client.mu.Lock()
	client.stream = mockStream
	client.policyChainVersion = "pc-v1"
	client.apiKeyVersion = "ak-v1"
	client.lazyResourceVersion = "lr-v1"
	client.mu.Unlock()

	// Send discovery request
	err = client.sendDiscoveryRequest("", "")
	require.NoError(t, err)

	// Verify each type uses its own version
	for _, req := range mockStream.sentRequests {
		switch req.TypeUrl {
		case PolicyChainTypeURL:
			assert.Equal(t, "pc-v1", req.VersionInfo)
		case APIKeyStateTypeURL:
			assert.Equal(t, "ak-v1", req.VersionInfo)
		case LazyResourceTypeURL:
			assert.Equal(t, "lr-v1", req.VersionInfo)
		}
	}
}

func TestClient_GetPolicyChainVersion(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)

	client.mu.Lock()
	client.policyChainVersion = "pc-v42"
	client.mu.Unlock()

	assert.Equal(t, "pc-v42", client.GetPolicyChainVersion())
}

// Mock ADS stream for testing
type mockADSStream struct {
	grpc.ClientStream
	sentRequests  []*discoveryv3.DiscoveryRequest
	recvResponses []*discoveryv3.DiscoveryResponse
	recvIndex     int
	recvError     error
	sendError     error
}

func (m *mockADSStream) Send(req *discoveryv3.DiscoveryRequest) error {
	if m.sendError != nil {
		return m.sendError
	}
	m.sentRequests = append(m.sentRequests, req)
	return nil
}

func (m *mockADSStream) Recv() (*discoveryv3.DiscoveryResponse, error) {
	if m.recvError != nil {
		return nil, m.recvError
	}
	if m.recvIndex >= len(m.recvResponses) {
		return nil, io.EOF
	}
	resp := m.recvResponses[m.recvIndex]
	m.recvIndex++
	return resp, nil
}

func (m *mockADSStream) CloseSend() error {
	return nil
}

// TestClient_ProcessStream_SuccessfulResponse tests processing a valid response
func TestClient_ProcessStream_SuccessfulResponse(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)

	// Create a valid but minimal response
	resp := &discoveryv3.DiscoveryResponse{
		TypeUrl:     PolicyChainTypeURL,
		VersionInfo: "v1",
		Nonce:       "nonce-1",
		Resources:   []*anypb.Any{}, // Empty is valid
	}

	mockStream := &mockADSStream{
		recvResponses: []*discoveryv3.DiscoveryResponse{resp},
		sentRequests:  make([]*discoveryv3.DiscoveryRequest, 0),
	}

	client.mu.Lock()
	client.stream = mockStream
	client.mu.Unlock()

	// Process stream in goroutine since it will run until EOF
	done := make(chan error, 1)
	go func() {
		done <- client.processStream(mockStream)
	}()

	// Wait for processing
	select {
	case err := <-done:
		assert.Equal(t, io.EOF, err) // Expected after processing all responses
	case <-time.After(2 * time.Second):
		t.Fatal("processStream should complete")
	}

	// Verify ACK was sent
	assert.NotEmpty(t, mockStream.sentRequests)
	ackSent := false
	for _, req := range mockStream.sentRequests {
		if req.TypeUrl == PolicyChainTypeURL && req.ResponseNonce == "nonce-1" {
			ackSent = true
			break
		}
	}
	assert.True(t, ackSent, "Should send ACK for processed response")
}

// TestClient_ProcessStream_UnknownTypeURL tests handling of unknown resource type
func TestClient_ProcessStream_UnknownTypeURL(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)

	// Response with unknown type URL
	resp := &discoveryv3.DiscoveryResponse{
		TypeUrl:     "type.unknown.com/UnknownType",
		VersionInfo: "v1",
		Nonce:       "nonce-unknown",
		Resources:   []*anypb.Any{},
	}

	mockStream := &mockADSStream{
		recvResponses: []*discoveryv3.DiscoveryResponse{resp},
		sentRequests:  make([]*discoveryv3.DiscoveryRequest, 0),
	}

	client.mu.Lock()
	client.stream = mockStream
	client.mu.Unlock()

	done := make(chan error, 1)
	go func() {
		done <- client.processStream(mockStream)
	}()

	select {
	case err := <-done:
		assert.Equal(t, io.EOF, err)
	case <-time.After(2 * time.Second):
		t.Fatal("processStream should complete")
	}

	// Should send NACK for unknown type
	nackSent := false
	for _, req := range mockStream.sentRequests {
		if req.ResponseNonce == "nonce-unknown" {
			nackSent = true
			break
		}
	}
	assert.True(t, nackSent, "Should send NACK for unknown type")
}

// TestClient_SendDiscoveryRequest_SendError tests error handling when Send fails
func TestClient_SendDiscoveryRequest_SendError(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)

	mockStream := &mockADSStream{
		sendError: fmt.Errorf("send failed"),
	}

	client.mu.Lock()
	client.stream = mockStream
	client.mu.Unlock()

	err = client.sendDiscoveryRequest("", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send")
}
