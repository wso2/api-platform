// Measures the marginal cost of re-sending a request body to the policy engine on each failover
// attempt.
//
// The per-attempt chain currently asks Envoy for a BUFFERED body whenever a provider has a
// transformer, so a request that fails over N times carries its payload to the engine N times
// instead of once. The alternative — holding the post-downstream body in engine memory and
// replacing it from the headers phase — trades that bandwidth for resident memory per in-flight
// request, so it is only worth building if the bandwidth actually costs something.
//
// This measures one ext_proc round trip carrying a body of size N over loopback gRPC, which is
// exactly the marginal cost of one extra attempt.
package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extproc "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// echoServer stands in for the policy engine: it receives a body and returns a mutation, which
// is what a transformer does.
type echoServer struct {
	extproc.UnimplementedExternalProcessorServer
}

func (e *echoServer) Process(stream extproc.ExternalProcessor_ProcessServer) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		body := req.GetRequestBody().GetBody()
		resp := &extproc.ProcessingResponse{
			Response: &extproc.ProcessingResponse_RequestBody{
				RequestBody: &extproc.BodyResponse{
					Response: &extproc.CommonResponse{
						// A transformer rewrites the payload, so the response carries a
						// body too — measuring a request-only round trip would understate
						// the real cost.
						Status:       extproc.CommonResponse_CONTINUE_AND_REPLACE,
						BodyMutation: &extproc.BodyMutation{Mutation: &extproc.BodyMutation_Body{Body: body}},
					},
				},
			},
		}
		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}

func startServer(tb testing.TB) *grpc.ClientConn {
	tb.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatal(err)
	}
	srv := grpc.NewServer()
	extproc.RegisterExternalProcessorServer(srv, &echoServer{})
	go func() { _ = srv.Serve(lis) }()
	tb.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = conn.Close() })
	return conn
}

// realisticPrompt builds a body of roughly n bytes shaped like a chat completion request, so the
// protobuf and any compression see representative content rather than incompressible noise.
func realisticPrompt(n int) []byte {
	prefix := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"`)
	suffix := []byte(`"}]}`)
	filler := make([]byte, 0, n)
	phrase := []byte("the quick brown fox jumps over the lazy dog and then considers the nature of language models ")
	for len(filler) < n-len(prefix)-len(suffix) {
		filler = append(filler, phrase...)
	}
	return append(append(prefix, filler...), suffix...)
}

func benchmarkBody(b *testing.B, size int) {
	conn := startServer(b)
	client := extproc.NewExternalProcessorClient(conn)
	body := realisticPrompt(size)

	b.SetBytes(int64(len(body)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream, err := client.Process(context.Background())
		if err != nil {
			b.Fatal(err)
		}
		req := &extproc.ProcessingRequest{
			Request: &extproc.ProcessingRequest_RequestBody{
				RequestBody: &extproc.HttpBody{Body: body, EndOfStream: true},
			},
		}
		if err := stream.Send(req); err != nil {
			b.Fatal(err)
		}
		if _, err := stream.Recv(); err != nil {
			b.Fatal(err)
		}
		_ = stream.CloseSend()
	}
}

func BenchmarkBodyRoundTrip(b *testing.B) {
	for _, size := range []int{1 << 10, 10 << 10, 100 << 10, 1 << 20} {
		b.Run(fmt.Sprintf("%dKB", size/1024), func(b *testing.B) { benchmarkBody(b, size) })
	}
}

// The headers-only round trip is what an attempt costs when its provider has no transformer —
// the floor the body sizes should be compared against.
func BenchmarkHeadersOnlyRoundTrip(b *testing.B) {
	conn := startServer(b)
	client := extproc.NewExternalProcessorClient(conn)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream, err := client.Process(context.Background())
		if err != nil {
			b.Fatal(err)
		}
		req := &extproc.ProcessingRequest{
			Request: &extproc.ProcessingRequest_RequestBody{
				RequestBody: &extproc.HttpBody{Body: nil, EndOfStream: true},
			},
		}
		if err := stream.Send(req); err != nil {
			b.Fatal(err)
		}
		if _, err := stream.Recv(); err != nil {
			b.Fatal(err)
		}
		_ = stream.CloseSend()
	}
}

var _ = corev3.HeaderValue{}
