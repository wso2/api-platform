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

// StreamManager manages the persistent bidirectional stream to the Python executor.
type StreamManager struct {
	socketPath     string
	dialContext    func(context.Context, string) (net.Conn, error)
	conn           *grpc.ClientConn
	client         proto.PythonExecutorServiceClient
	stream         proto.PythonExecutorService_ExecuteStreamClient
	streamCancel   context.CancelFunc
	sendMu         sync.Mutex
	pendingMu      sync.RWMutex
	pendingReqs    map[string]chan *proto.StreamResponse
	suppressedReqs map[string]struct{}
	connected      bool
	connectMu      sync.Mutex
	connID         atomic.Uint64
}

// NewStreamManager creates a StreamManager for the given Unix-domain socket.
func NewStreamManager(socketPath string) *StreamManager {
	return &StreamManager{
		socketPath:     socketPath,
		pendingReqs:    make(map[string]chan *proto.StreamResponse),
		suppressedReqs: make(map[string]struct{}),
	}
}

// Connect establishes the gRPC connection and starts the receive loop.
func (sm *StreamManager) Connect(ctx context.Context) error {
	sm.connectMu.Lock()
	defer sm.connectMu.Unlock()

	if sm.connected {
		return nil
	}

	slogger := slog.With("component", "pythonbridge", "socket", sm.socketPath)
	slogger.InfoContext(ctx, "Connecting to Python Executor")

	if sm.streamCancel != nil {
		sm.streamCancel()
		sm.streamCancel = nil
	}
	if sm.conn != nil {
		_ = sm.conn.Close()
		sm.conn = nil
	}
	sm.stream = nil

	dialContext := sm.dialContext
	if dialContext == nil {
		dialContext = func(ctx context.Context, addr string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", addr)
		}
	}
	conn, err := grpc.DialContext(
		ctx,
		sm.socketPath,
		grpc.WithContextDialer(dialContext),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("dial Python Executor: %w", err)
	}

	client := proto.NewPythonExecutorServiceClient(conn)
	streamCtx, streamCancel := context.WithCancel(context.Background())
	stream, err := client.ExecuteStream(streamCtx)
	if err != nil {
		streamCancel()
		_ = conn.Close()
		return fmt.Errorf("start ExecuteStream: %w", err)
	}

	sm.conn = conn
	sm.client = client
	sm.stream = stream
	sm.streamCancel = streamCancel
	sm.connected = true

	connID := sm.connID.Add(1)
	go sm.receiveLoop(stream, conn, connID)

	slogger.InfoContext(ctx, "Connected to Python Executor")
	return nil
}

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

		sm.pendingMu.RLock()
		ch, ok := sm.pendingReqs[resp.GetRequestId()]
		_, suppressed := sm.suppressedReqs[resp.GetRequestId()]
		sm.pendingMu.RUnlock()

		if !ok {
			if suppressed {
				sm.pendingMu.Lock()
				delete(sm.suppressedReqs, resp.GetRequestId())
				sm.pendingMu.Unlock()
				slogger.Debug("Dropped late Python response after cancellation", "request_id", resp.GetRequestId())
				continue
			}

			slogger.Warn("Received response for unknown request", "request_id", resp.GetRequestId())
			continue
		}

		select {
		case ch <- resp:
		default:
			slogger.Error("Response channel full, dropping response", "request_id", resp.GetRequestId())
		}
	}
}

func (sm *StreamManager) handleDisconnect(connID uint64, conn *grpc.ClientConn) {
	sm.connectMu.Lock()
	defer sm.connectMu.Unlock()

	if sm.connID.Load() != connID || sm.conn != conn {
		slog.Info("Stale receiveLoop detected, skipping disconnect handling",
			"component", "pythonbridge",
			"conn_id", connID,
			"current_conn_id", sm.connID.Load(),
		)
		return
	}

	sm.connected = false
	if sm.streamCancel != nil {
		sm.streamCancel()
		sm.streamCancel = nil
	}
	if sm.conn != nil {
		_ = sm.conn.Close()
		sm.conn = nil
	}
	sm.stream = nil

	sm.pendingMu.Lock()
	pendingCopied := make(map[string]chan *proto.StreamResponse, len(sm.pendingReqs))
	for reqID, ch := range sm.pendingReqs {
		pendingCopied[reqID] = ch
	}
	clear(sm.pendingReqs)
	clear(sm.suppressedReqs)
	sm.pendingMu.Unlock()

	for reqID, ch := range pendingCopied {
		select {
		case ch <- &proto.StreamResponse{
			RequestId: reqID,
			Payload: &proto.StreamResponse_Error{
				Error: &proto.ExecutionError{
					Message:       "Python Executor disconnected",
					ErrorType:     "disconnect",
					PolicyName:    "unknown",
					PolicyVersion: "unknown",
				},
			},
		}:
		default:
		}
	}
}

// Execute sends a request on the shared stream and waits for its correlated response.
func (sm *StreamManager) Execute(ctx context.Context, req *proto.StreamRequest) (*proto.StreamResponse, error) {
	if !sm.IsConnected() {
		if err := sm.Connect(ctx); err != nil {
			return nil, fmt.Errorf("connect Python Executor: %w", err)
		}
	}

	ctx, cancel := withDefaultTimeout(ctx, getTimeout())
	defer cancel()

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	respCh := make(chan *proto.StreamResponse, 1)
	sm.pendingMu.Lock()
	sm.pendingReqs[req.GetRequestId()] = respCh
	delete(sm.suppressedReqs, req.GetRequestId())
	sm.pendingMu.Unlock()

	cleanupPending := func(suppressLate bool) {
		sm.pendingMu.Lock()
		delete(sm.pendingReqs, req.GetRequestId())
		if suppressLate {
			sm.suppressedReqs[req.GetRequestId()] = struct{}{}
		}
		sm.pendingMu.Unlock()
	}

	sm.connectMu.Lock()
	stream := sm.stream
	connected := sm.connected
	sm.connectMu.Unlock()
	if !connected || stream == nil {
		cleanupPending(false)
		return nil, fmt.Errorf("python executor stream is not connected")
	}

	if err := ctx.Err(); err != nil {
		cleanupPending(false)
		return nil, err
	}

	sm.sendMu.Lock()
	err := stream.Send(req)
	sm.sendMu.Unlock()
	if err != nil {
		cleanupPending(false)
		return nil, fmt.Errorf("send stream request: %w", err)
	}

	select {
	case resp := <-respCh:
		cleanupPending(false)
		return resp, nil
	case <-ctx.Done():
		cleanupPending(true)
		sm.sendCancelAsync(req, ctx.Err())
		return nil, ctx.Err()
	}
}

func (sm *StreamManager) sendCancelAsync(req *proto.StreamRequest, cause error) {
	cancelReq := &proto.StreamRequest{
		RequestId:     req.GetRequestId(),
		InstanceId:    req.GetInstanceId(),
		PolicyName:    req.GetPolicyName(),
		PolicyVersion: req.GetPolicyVersion(),
		ExecutionMetadata: &proto.ExecutionMetadata{
			Phase: proto.Phase_PHASE_CANCEL,
		},
		Payload: &proto.StreamRequest_CancelExecution{
			CancelExecution: &proto.CancelExecutionPayload{
				TargetPhase: extractPhase(req),
				Reason:      errorReason(cause),
			},
		},
	}
	if req.GetExecutionMetadata() != nil {
		cancelReq.ExecutionMetadata.RouteName = req.GetExecutionMetadata().GetRouteName()
	}

	go func() {
		sm.connectMu.Lock()
		stream := sm.stream
		connected := sm.connected
		sm.connectMu.Unlock()
		if !connected || stream == nil {
			return
		}

		sm.sendMu.Lock()
		err := stream.Send(cancelReq)
		sm.sendMu.Unlock()
		if err != nil {
			slog.Debug("Failed to send cancellation to Python executor",
				"component", "pythonbridge",
				"request_id", req.GetRequestId(),
				"error", err,
			)
		}
	}()
}

// IsConnected reports whether the stream is connected and ready.
func (sm *StreamManager) IsConnected() bool {
	sm.connectMu.Lock()
	defer sm.connectMu.Unlock()

	if !sm.connected || sm.conn == nil {
		return false
	}
	return sm.conn.GetState() == connectivity.Ready
}

// Close closes the shared stream and connection.
func (sm *StreamManager) Close() error {
	sm.connectMu.Lock()
	defer sm.connectMu.Unlock()

	if !sm.connected {
		return nil
	}
	sm.connected = false

	if sm.streamCancel != nil {
		sm.streamCancel()
		sm.streamCancel = nil
	}
	if sm.stream != nil {
		_ = sm.stream.CloseSend()
	}
	if sm.conn != nil {
		return sm.conn.Close()
	}
	return nil
}

// InitPolicy creates a Python policy instance via the unary RPC.
func (sm *StreamManager) InitPolicy(ctx context.Context, req *proto.InitPolicyRequest) (*proto.InitPolicyResponse, error) {
	if !sm.IsConnected() {
		if err := sm.Connect(ctx); err != nil {
			return nil, fmt.Errorf("connect Python Executor: %w", err)
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

// DestroyPolicy destroys a Python policy instance via the unary RPC.
func (sm *StreamManager) DestroyPolicy(ctx context.Context, req *proto.DestroyPolicyRequest) (*proto.DestroyPolicyResponse, error) {
	if !sm.IsConnected() {
		if err := sm.Connect(ctx); err != nil {
			return nil, fmt.Errorf("connect Python Executor: %w", err)
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

// HealthCheck checks Python executor readiness via the unary RPC.
func (sm *StreamManager) HealthCheck(ctx context.Context) (*proto.HealthCheckResponse, error) {
	if !sm.IsConnected() {
		if err := sm.Connect(ctx); err != nil {
			return nil, fmt.Errorf("connect Python Executor: %w", err)
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

func withDefaultTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func extractPhase(req *proto.StreamRequest) proto.Phase {
	if req.GetExecutionMetadata() != nil && req.GetExecutionMetadata().GetPhase() != proto.Phase_PHASE_UNSPECIFIED {
		return req.GetExecutionMetadata().GetPhase()
	}
	switch req.GetPayload().(type) {
	case *proto.StreamRequest_RequestHeaders:
		return proto.Phase_PHASE_REQUEST_HEADERS
	case *proto.StreamRequest_RequestBody:
		return proto.Phase_PHASE_REQUEST_BODY
	case *proto.StreamRequest_ResponseHeaders:
		return proto.Phase_PHASE_RESPONSE_HEADERS
	case *proto.StreamRequest_ResponseBody:
		return proto.Phase_PHASE_RESPONSE_BODY
	case *proto.StreamRequest_NeedsMoreRequestData:
		return proto.Phase_PHASE_NEEDS_MORE_REQUEST_DATA
	case *proto.StreamRequest_RequestChunk:
		return proto.Phase_PHASE_REQUEST_BODY_CHUNK
	case *proto.StreamRequest_NeedsMoreResponseData:
		return proto.Phase_PHASE_NEEDS_MORE_RESPONSE_DATA
	case *proto.StreamRequest_ResponseChunk:
		return proto.Phase_PHASE_RESPONSE_BODY_CHUNK
	default:
		return proto.Phase_PHASE_UNSPECIFIED
	}
}

func errorReason(err error) string {
	if err == nil {
		return "cancelled"
	}
	return err.Error()
}

// pythonPolicyTimeout is resolved once at package init.
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
