/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package pythonbridge

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/pythonbridge/proto"
)

// StreamManager manages a persistent bidirectional gRPC stream to the Python Executor.
// It is a singleton created at Policy Engine startup.
//
// Thread safety: Multiple goroutines (one per ext_proc stream) call Execute() concurrently.
// The StreamManager uses a pendingRequests map protected by a mutex to correlate
// request_id → response channel.
//
// Flow:
//  1. Execute() generates a unique request_id, creates a response channel, adds to pendingRequests
//  2. Sends the ExecutionRequest on the stream (protected by a send mutex)
//  3. A background goroutine continuously receives from the stream, looks up request_id
//     in pendingRequests, and sends the response on the channel
//  4. Execute() waits on the channel with a timeout, returns the result
type StreamManager struct {
	socketPath   string
	conn         *grpc.ClientConn
	client       proto.PythonExecutorServiceClient
	stream       proto.PythonExecutorService_ExecuteStreamClient
	streamCancel context.CancelFunc                       // cancels the long-lived stream context on Close()
	sendMu       sync.Mutex                               // Protects stream.Send()
	pendingMu    sync.RWMutex                             // Protects pendingRequests
	pendingReqs  map[string]chan *proto.ExecutionResponse // request_id → response channel
	connected    bool
	connectMu    sync.Mutex
	connID       atomic.Uint64 // Monotonically increasing connection ID for stale goroutine detection
}

// NewStreamManager creates a new StreamManager for the given UDS socket path.
func NewStreamManager(socketPath string) *StreamManager {
	return &StreamManager{
		socketPath:  socketPath,
		pendingReqs: make(map[string]chan *proto.ExecutionResponse),
	}
}

// Connect establishes connection to the Python Executor and starts the receive loop.
// This is called lazily on first Execute() if not already connected.
func (sm *StreamManager) Connect(ctx context.Context) error {
	sm.connectMu.Lock()
	defer sm.connectMu.Unlock()

	if sm.connected {
		return nil
	}

	slogger := slog.With("component", "pythonbridge", "socket", sm.socketPath)
	slogger.InfoContext(ctx, "Connecting to Python Executor")

	// Cancel and clear any previous stream/connection before creating new ones,
	// so the old context is terminated and resources are released first.
	if sm.streamCancel != nil {
		sm.streamCancel()
		sm.streamCancel = nil
	}
	if sm.conn != nil {
		sm.conn.Close()
		sm.conn = nil
	}
	sm.stream = nil

	// Dial with UDS
	dialer := func(ctx context.Context, addr string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "unix", addr)
	}
	conn, err := grpc.DialContext(
		ctx,
		sm.socketPath,
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("failed to dial Python Executor: %w", err)
	}

	client := proto.NewPythonExecutorServiceClient(conn)

	// The stream must live for the lifetime of the connection, not for the
	// duration of a single InitPolicy call. Use a dedicated background context
	// that is only cancelled when Close() is explicitly called.
	streamCtx, streamCancel := context.WithCancel(context.Background())
	stream, err := client.ExecuteStream(streamCtx)
	if err != nil {
		streamCancel()
		conn.Close()
		return fmt.Errorf("failed to start ExecuteStream: %w", err)
	}

	sm.conn = conn
	sm.client = client
	sm.stream = stream
	sm.streamCancel = streamCancel
	sm.connected = true

	// Increment connection ID and capture for this connection's receive loop
	connID := sm.connID.Add(1)

	// Start receive loop with captured stream and connection ID for stale goroutine detection.
	go sm.receiveLoop(stream, conn, connID)

	slogger.InfoContext(ctx, "Connected to Python Executor")
	return nil
}

// receiveLoop continuously receives responses from the stream and dispatches them
// to waiting request channels.
// The connID parameter identifies the specific connection this loop is handling,
// preventing stale goroutines from closing newer connections.
func (sm *StreamManager) receiveLoop(stream proto.PythonExecutorService_ExecuteStreamClient, conn *grpc.ClientConn, connID uint64) {
	slogger := slog.With("component", "pythonbridge", "phase", "receiveLoop", "conn_id", connID)
	defer slogger.Info("Receive loop exited")

	for {
		resp, err := stream.Recv()
		if err != nil {
			slogger.Error("Error receiving from stream", "error", err)
			sm.handleDisconnect(connID, conn)
			return
		}

		// Lookup the waiting channel
		sm.pendingMu.RLock()
		ch, ok := sm.pendingReqs[resp.RequestId]
		sm.pendingMu.RUnlock()

		if !ok {
			slogger.Warn("Received response for unknown request", "request_id", resp.RequestId)
			continue
		}

		// Send response to waiter (non-blocking with buffer)
		select {
		case ch <- resp:
		default:
			slogger.Error("Response channel full, dropping response", "request_id", resp.RequestId)
		}
	}
}

// handleDisconnect marks the connection as disconnected and cleans up pending requests.
// The connID parameter identifies the connection that disconnected; if it doesn't match
// the current connection ID, this is a stale goroutine and should not close the new connection.
func (sm *StreamManager) handleDisconnect(connID uint64, conn *grpc.ClientConn) {
	sm.connectMu.Lock()
	defer sm.connectMu.Unlock()

	// Check if this is a stale goroutine trying to close a newer connection
	if sm.connID.Load() != connID || sm.conn != conn {
		slog.Info("Stale receiveLoop detected, skipping disconnect handling",
			"component", "pythonbridge",
			"conn_id", connID,
			"current_conn_id", sm.connID.Load())
		return
	}

	sm.connected = false
	if sm.streamCancel != nil {
		sm.streamCancel()
		sm.streamCancel = nil
	}
	if sm.conn != nil {
		sm.conn.Close()
		sm.conn = nil
	}
	sm.stream = nil

	// Signal error to all pending requests
	sm.pendingMu.Lock()
	pendingCopied := make(map[string]chan *proto.ExecutionResponse, len(sm.pendingReqs))
	for reqID, ch := range sm.pendingReqs {
		pendingCopied[reqID] = ch
	}
	// Clear the original map
	clear(sm.pendingReqs)
	sm.pendingMu.Unlock()

	for reqID, ch := range pendingCopied {
		select {
		case ch <- &proto.ExecutionResponse{
			RequestId: reqID,
			Result: &proto.ExecutionResponse_Error{
				Error: &proto.ExecutionError{
					Message:    "Python Executor disconnected",
					ErrorType:  "disconnect",
					PolicyName: "unknown",
				},
			},
		}:
		default:
		}
	}
}

// Execute sends a request to the Python Executor and waits for the response.
// It handles lazy connection, request correlation, and timeout.
func (sm *StreamManager) Execute(ctx context.Context, req *proto.ExecutionRequest) (*proto.ExecutionResponse, error) {
	// Ensure connected
	if !sm.IsConnected() {
		if err := sm.Connect(ctx); err != nil {
			return nil, fmt.Errorf("failed to connect to Python Executor: %w", err)
		}
	}

	// Create response channel (buffered to prevent blocking receiveLoop)
	respCh := make(chan *proto.ExecutionResponse, 1)

	// Register pending request
	sm.pendingMu.Lock()
	sm.pendingReqs[req.RequestId] = respCh
	sm.pendingMu.Unlock()

	// Cleanup on exit
	defer func() {
		sm.pendingMu.Lock()
		delete(sm.pendingReqs, req.RequestId)
		sm.pendingMu.Unlock()
	}()

	// Capture stream under connectMu to avoid TOCTOU race with handleDisconnect
	sm.connectMu.Lock()
	stream := sm.stream
	connected := sm.connected
	sm.connectMu.Unlock()

	if !connected || stream == nil {
		return nil, fmt.Errorf("python executor stream is not connected")
	}

	// Send request (protected by send mutex)
	sm.sendMu.Lock()
	err := stream.Send(req)
	sm.sendMu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Wait for response with timeout
	timeout := getTimeout()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case resp := <-respCh:
		return resp, nil
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("timeout waiting for Python response after %v", timeout)
		}
		return nil, ctx.Err()
	}
}

// IsConnected returns true if the stream is connected and ready.
func (sm *StreamManager) IsConnected() bool {
	sm.connectMu.Lock()
	defer sm.connectMu.Unlock()

	if !sm.connected || sm.conn == nil {
		return false
	}

	state := sm.conn.GetState()
	return state == connectivity.Ready
}

// Close gracefully closes the stream and connection.
func (sm *StreamManager) Close() error {
	sm.connectMu.Lock()
	defer sm.connectMu.Unlock()

	if !sm.connected {
		return nil
	}

	sm.connected = false

	// Cancel the stream context first so the receiveLoop unblocks cleanly.
	if sm.streamCancel != nil {
		sm.streamCancel()
		sm.streamCancel = nil
	}

	if sm.stream != nil {
		sm.stream.CloseSend()
	}

	if sm.conn != nil {
		return sm.conn.Close()
	}

	return nil
}

// InitPolicy calls the InitPolicy unary RPC to create a policy instance on the Python side.
// Returns the instance_id assigned by Python.
func (sm *StreamManager) InitPolicy(ctx context.Context, req *proto.InitPolicyRequest) (*proto.InitPolicyResponse, error) {
	if !sm.IsConnected() {
		if err := sm.Connect(ctx); err != nil {
			return nil, fmt.Errorf("failed to connect to Python Executor: %w", err)
		}
	}

	sm.connectMu.Lock()
	client := sm.client
	sm.connectMu.Unlock()

	if client == nil {
		return nil, fmt.Errorf("python executor client is not connected")
	}

	resp, err := client.InitPolicy(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("InitPolicy RPC failed: %w", err)
	}
	return resp, nil
}

// DestroyPolicy calls the DestroyPolicy unary RPC to destroy a policy instance on the Python side.
func (sm *StreamManager) DestroyPolicy(ctx context.Context, req *proto.DestroyPolicyRequest) (*proto.DestroyPolicyResponse, error) {
	if !sm.IsConnected() {
		if err := sm.Connect(ctx); err != nil {
			return nil, fmt.Errorf("failed to connect to Python Executor: %w", err)
		}
	}

	sm.connectMu.Lock()
	client := sm.client
	sm.connectMu.Unlock()

	if client == nil {
		return nil, fmt.Errorf("python executor client is not connected")
	}

	resp, err := client.DestroyPolicy(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("DestroyPolicy RPC failed: %w", err)
	}
	return resp, nil
}

// HealthCheck calls the HealthCheck unary RPC to check Python executor readiness.
func (sm *StreamManager) HealthCheck(ctx context.Context) (*proto.HealthCheckResponse, error) {
	if !sm.IsConnected() {
		if err := sm.Connect(ctx); err != nil {
			return nil, fmt.Errorf("failed to connect to Python Executor: %w", err)
		}
	}

	sm.connectMu.Lock()
	client := sm.client
	sm.connectMu.Unlock()

	if client == nil {
		return nil, fmt.Errorf("python executor client is not connected")
	}

	resp, err := client.HealthCheck(ctx, &proto.HealthCheckRequest{})
	if err != nil {
		return nil, fmt.Errorf("HealthCheck RPC failed: %w", err)
	}
	return resp, nil
}

// pythonPolicyTimeout is the configured timeout, resolved once at package init.
var pythonPolicyTimeout = func() time.Duration {
	if s := os.Getenv("PYTHON_POLICY_TIMEOUT"); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			return d
		}
	}
	return 30 * time.Second
}()

func getTimeout() time.Duration {
	return pythonPolicyTimeout
}

// Singleton management
var (
	globalStreamManager *StreamManager
	streamManagerOnce   sync.Once
)

const pythonExecutorSocketPath = "/var/run/api-platform/python-executor.sock"

// GetStreamManager returns the singleton StreamManager instance.
func GetStreamManager() *StreamManager {
	streamManagerOnce.Do(func() {
		globalStreamManager = NewStreamManager(pythonExecutorSocketPath)
	})
	return globalStreamManager
}
