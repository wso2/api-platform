package pythonbridge

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	gproto "google.golang.org/protobuf/proto"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/pythonbridge/proto"
)

type testStreamHandler func(req *proto.StreamRequest, responses chan<- *proto.StreamResponse)

type testExecutorHarness struct {
	listener *bufconn.Listener
	server   *grpc.Server
	service  *testPythonExecutorServer
}

type testPythonExecutorServer struct {
	proto.UnimplementedPythonExecutorServiceServer

	handler testStreamHandler

	mu       sync.Mutex
	requests []*proto.StreamRequest
	cancels  []*proto.StreamRequest
}

func (s *testPythonExecutorServer) ExecuteStream(stream proto.PythonExecutorService_ExecuteStreamServer) error {
	responses := make(chan *proto.StreamResponse, 16)
	sendErr := make(chan error, 1)
	done := make(chan struct{})

	go func() {
		defer close(done)
		for resp := range responses {
			if err := stream.Send(resp); err != nil {
				select {
				case sendErr <- err:
				default:
				}
				return
			}
		}
	}()

	defer func() {
		close(responses)
		<-done
	}()

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		cloned := gproto.Clone(req).(*proto.StreamRequest)
		s.mu.Lock()
		if _, ok := req.GetPayload().(*proto.StreamRequest_CancelExecution); ok {
			s.cancels = append(s.cancels, cloned)
		} else {
			s.requests = append(s.requests, cloned)
		}
		s.mu.Unlock()

		if s.handler != nil {
			s.handler(cloned, responses)
		}

		select {
		case err := <-sendErr:
			return err
		default:
		}
	}
}

func (s *testPythonExecutorServer) InitPolicy(context.Context, *proto.InitPolicyRequest) (*proto.InitPolicyResponse, error) {
	return &proto.InitPolicyResponse{Success: true}, nil
}

func (s *testPythonExecutorServer) DestroyPolicy(context.Context, *proto.DestroyPolicyRequest) (*proto.DestroyPolicyResponse, error) {
	return &proto.DestroyPolicyResponse{Success: true}, nil
}

func (s *testPythonExecutorServer) HealthCheck(context.Context, *proto.HealthCheckRequest) (*proto.HealthCheckResponse, error) {
	return &proto.HealthCheckResponse{Ready: true}, nil
}

func (s *testPythonExecutorServer) cancelCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.cancels)
}

func (s *testPythonExecutorServer) lastCancel() *proto.StreamRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.cancels) == 0 {
		return nil
	}
	return s.cancels[len(s.cancels)-1]
}

func startTestPythonExecutorServer(t testing.TB, handler testStreamHandler) *testExecutorHarness {
	t.Helper()

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	service := &testPythonExecutorServer{handler: handler}
	proto.RegisterPythonExecutorServiceServer(server, service)

	go func() {
		_ = server.Serve(listener)
	}()

	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	return &testExecutorHarness{
		listener: listener,
		server:   server,
		service:  service,
	}
}

func newNeedsMoreRequest(requestID string) *proto.StreamRequest {
	return &proto.StreamRequest{
		RequestId:     requestID,
		InstanceId:    "instance-1",
		PolicyName:    "demo-policy",
		PolicyVersion: "v1.0.0",
		ExecutionMetadata: &proto.ExecutionMetadata{
			Phase: proto.Phase_PHASE_NEEDS_MORE_REQUEST_DATA,
		},
		Payload: &proto.StreamRequest_NeedsMoreRequestData{
			NeedsMoreRequestData: &proto.NeedsMoreRequestDataPayload{
				Accumulated: []byte("chunk"),
			},
		},
	}
}

func TestStreamManagerExecuteCancellationSuppressesLateResponses(t *testing.T) {
	harness := startTestPythonExecutorServer(t, func(req *proto.StreamRequest, responses chan<- *proto.StreamResponse) {
		if _, ok := req.GetPayload().(*proto.StreamRequest_CancelExecution); ok {
			return
		}

		go func(requestID string) {
			time.Sleep(50 * time.Millisecond)
			responses <- &proto.StreamResponse{
				RequestId: requestID,
				Payload: &proto.StreamResponse_NeedsMoreDecision{
					NeedsMoreDecision: &proto.NeedsMoreDecisionPayload{NeedsMore: true},
				},
			}
		}(req.GetRequestId())
	})

	sm := NewStreamManager("bufconn")
	sm.dialContext = func(context.Context, string) (net.Conn, error) {
		return harness.listener.Dial()
	}
	t.Cleanup(func() {
		_ = sm.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := sm.Execute(ctx, newNeedsMoreRequest("slow-request"))
	require.ErrorIs(t, err, context.DeadlineExceeded)

	require.Eventually(t, func() bool {
		return harness.service.cancelCount() == 1
	}, time.Second, 10*time.Millisecond)

	cancelReq := harness.service.lastCancel()
	require.NotNil(t, cancelReq)
	assert.Equal(t, proto.Phase_PHASE_CANCEL, cancelReq.GetExecutionMetadata().GetPhase())
	assert.Equal(t, proto.Phase_PHASE_NEEDS_MORE_REQUEST_DATA, cancelReq.GetCancelExecution().GetTargetPhase())

	require.Eventually(t, func() bool {
		sm.pendingMu.RLock()
		defer sm.pendingMu.RUnlock()
		return len(sm.pendingReqs) == 0 && len(sm.suppressedReqs) == 0
	}, time.Second, 10*time.Millisecond)
}

func TestStreamManagerReconnectsAfterDisconnect(t *testing.T) {
	harness1 := startTestPythonExecutorServer(t, func(req *proto.StreamRequest, responses chan<- *proto.StreamResponse) {
		responses <- &proto.StreamResponse{
			RequestId: req.GetRequestId(),
			Payload: &proto.StreamResponse_NeedsMoreDecision{
				NeedsMoreDecision: &proto.NeedsMoreDecisionPayload{NeedsMore: false},
			},
		}
	})

	sm := NewStreamManager("bufconn")
	sm.dialContext = func(context.Context, string) (net.Conn, error) {
		return harness1.listener.Dial()
	}
	t.Cleanup(func() {
		_ = sm.Close()
	})

	resp, err := sm.Execute(context.Background(), newNeedsMoreRequest("first-request"))
	require.NoError(t, err)
	decision, err := NewTranslator().ToGoNeedsMoreDecision(resp)
	require.NoError(t, err)
	assert.False(t, decision)

	harness1.server.Stop()
	_ = harness1.listener.Close()

	require.Eventually(t, func() bool {
		return !sm.IsConnected()
	}, time.Second, 10*time.Millisecond)

	harness2 := startTestPythonExecutorServer(t, func(req *proto.StreamRequest, responses chan<- *proto.StreamResponse) {
		responses <- &proto.StreamResponse{
			RequestId: req.GetRequestId(),
			Payload: &proto.StreamResponse_NeedsMoreDecision{
				NeedsMoreDecision: &proto.NeedsMoreDecisionPayload{NeedsMore: true},
			},
		}
	})
	sm.dialContext = func(context.Context, string) (net.Conn, error) {
		return harness2.listener.Dial()
	}

	resp, err = sm.Execute(context.Background(), newNeedsMoreRequest("second-request"))
	require.NoError(t, err)
	decision, err = NewTranslator().ToGoNeedsMoreDecision(resp)
	require.NoError(t, err)
	assert.True(t, decision)
}

func BenchmarkStreamManagerNeedsMoreRoundTrip(b *testing.B) {
	harness := startTestPythonExecutorServer(b, func(req *proto.StreamRequest, responses chan<- *proto.StreamResponse) {
		responses <- &proto.StreamResponse{
			RequestId: req.GetRequestId(),
			Payload: &proto.StreamResponse_NeedsMoreDecision{
				NeedsMoreDecision: &proto.NeedsMoreDecisionPayload{NeedsMore: true},
			},
		}
	})

	sm := NewStreamManager("bufconn")
	sm.dialContext = func(context.Context, string) (net.Conn, error) {
		return harness.listener.Dial()
	}
	b.Cleanup(func() {
		_ = sm.Close()
	})

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := newNeedsMoreRequest(fmt.Sprintf("bench-%d", i))
		resp, err := sm.Execute(ctx, req)
		if err != nil {
			b.Fatalf("execute request: %v", err)
		}
		if _, err := NewTranslator().ToGoNeedsMoreDecision(resp); err != nil {
			b.Fatalf("translate decision: %v", err)
		}
	}
}
