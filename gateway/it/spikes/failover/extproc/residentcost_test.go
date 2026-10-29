// Compares the two ways of getting a request body to the per-attempt chain.
//
//	WIRE   (today): Envoy re-sends the body on every attempt. Cost is transfer + transient
//	               allocation per attempt.
//	MEMORY (alternative): the post-downstream body is held in engine memory and replaced from
//	               the headers phase. Cost is resident memory for every in-flight request,
//	               held until the request finishes or its TTL expires.
//
// Both pay one downstream body transfer, which already happens today, so only the UPSTREAM
// portion differs and only that is measured here.
package main

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	extproc "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"google.golang.org/grpc"
)

// bodyStore is the in-memory alternative: the same shape as the engine's attemptBaselineStore,
// extended to hold the payload.
type bodyStore struct {
	mu      sync.Mutex
	entries map[string]storedBody
}

type storedBody struct {
	body    []byte
	expires time.Time
}

func newBodyStore() *bodyStore { return &bodyStore{entries: map[string]storedBody{}} }

func (s *bodyStore) Remember(id string, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.entries[id]; ok {
		return
	}
	s.entries[id] = storedBody{body: body, expires: time.Now().Add(5 * time.Minute)}
}

func (s *bodyStore) Lookup(id string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[id]
	return e.body, ok
}

func (s *bodyStore) Forget(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, id)
}

// attemptsPerRequest is a typical failover: primary fails, fallback answers.
const attemptsPerRequest = 2

func roundTrip(b *testing.B, client extproc.ExternalProcessorClient, body []byte) {
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

// WIRE: each attempt carries the body.
func benchWire(b *testing.B, conn *grpc.ClientConn, size int) {
	client := extproc.NewExternalProcessorClient(conn)
	body := realisticPrompt(size)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for a := 0; a < attemptsPerRequest; a++ {
			roundTrip(b, client, body)
		}
	}
}

// MEMORY: the body is stored once, each attempt is a headers-only round trip plus a lookup.
func benchMemory(b *testing.B, conn *grpc.ClientConn, size int) {
	client := extproc.NewExternalProcessorClient(conn)
	body := realisticPrompt(size)
	store := newBodyStore()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := fmt.Sprintf("req-%d", i)
		store.Remember(id, body)
		for a := 0; a < attemptsPerRequest; a++ {
			if _, ok := store.Lookup(id); !ok {
				b.Fatal("body missing")
			}
			roundTrip(b, client, nil) // headers-only: no body on the wire
		}
		store.Forget(id)
	}
}

func BenchmarkPerRequest(b *testing.B) {
	conn := startServer(b)
	for _, size := range []int{10 << 10, 100 << 10, 1 << 20} {
		label := fmt.Sprintf("%dKB", size/1024)
		b.Run("wire/"+label, func(b *testing.B) { benchWire(b, conn, size) })
		b.Run("memory/"+label, func(b *testing.B) { benchMemory(b, conn, size) })
	}
}

// TestResidentMemory reports the heap cost of holding bodies for in-flight requests, which is
// what the memory approach trades bandwidth for. Reported, not asserted: it is a budgeting
// number, and the threshold that matters depends on deployment concurrency.
func TestResidentMemory(t *testing.T) {
	for _, tc := range []struct {
		concurrent int
		size       int
	}{
		{100, 10 << 10},
		{1000, 10 << 10},
		{1000, 100 << 10},
		{5000, 100 << 10},
	} {
		store := newBodyStore()
		runtime.GC()
		var before runtime.MemStats
		runtime.ReadMemStats(&before)

		for i := 0; i < tc.concurrent; i++ {
			// A distinct copy per request, as production would have.
			store.Remember(fmt.Sprintf("req-%d", i), realisticPrompt(tc.size))
		}

		runtime.GC()
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		held := (after.HeapAlloc - before.HeapAlloc) / (1 << 20)
		t.Logf("%5d in-flight x %4dKB body = %4d MB resident",
			tc.concurrent, tc.size/1024, held)
		runtime.KeepAlive(store)
	}
}
