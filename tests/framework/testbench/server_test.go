package testbench

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeService struct {
	name      string
	port      int
	handler   http.Handler
	stateful  bool
	partition string
}

func (s *fakeService) Name() string          { return s.name }
func (s *fakeService) Port() int             { return s.port }
func (s *fakeService) Handler() http.Handler { return s.handler }
func (s *fakeService) Stateful() bool        { return s.stateful }
func (s *fakeService) PartitionKey() string  { return s.partition }

type unpartitionedService struct{ fakeService }

func TestServeRejectsInvalidInputs(t *testing.T) {
	registry := &Registry{}
	ctx := context.Background()
	logger := slog.Default()

	tests := []struct {
		name string
		ctx  context.Context
		reg  *Registry
		log  *slog.Logger
		want string
	}{
		{name: "nil context", reg: registry, log: logger, want: "nil context"},
		{name: "nil registry", ctx: ctx, log: logger, want: "nil registry"},
		{name: "nil logger", ctx: ctx, reg: registry, want: "nil logger"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Serve(tt.ctx, tt.reg, tt.log)
			if err == nil || !containsError(err, tt.want) {
				t.Fatalf("Serve() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestServeReturnsCanceledContextBeforeStarting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Serve(ctx, &Registry{}, slog.Default())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Serve() error = %v, want context.Canceled", err)
	}
}

func TestServeReturnsPortConflictImmediately(t *testing.T) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	registry := &Registry{}
	if err := registry.Register(&fakeService{
		name: "conflict", port: port, handler: http.NotFoundHandler(),
	}); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- Serve(context.Background(), registry, slog.Default()) }()
	select {
	case err := <-done:
		if err == nil || !containsError(err, "listening on port") {
			t.Fatalf("Serve() error = %v, want port conflict", err)
		}
	case <-time.After(time.Second):
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
		t.Fatal("Serve() did not return promptly on port conflict")
	}
}

func containsError(err error, want string) bool {
	return err != nil && strings.Contains(err.Error(), want)
}

func TestRegistryRejectsInvalidServices(t *testing.T) {
	handler := http.NotFoundHandler()
	tests := []struct {
		name    string
		service Service
	}{
		{name: "typed nil", service: (*fakeService)(nil)},
		{name: "blank name", service: &fakeService{port: 1, handler: handler}},
		{name: "whitespace name", service: &fakeService{name: "  ", port: 1, handler: handler}},
		{name: "invalid port", service: &fakeService{name: "service", handler: handler}},
		{name: "port above 65535", service: &fakeService{name: "service", port: 65536, handler: handler}},
		{name: "nil handler", service: &fakeService{name: "service", port: 1}},
		{name: "unpartitioned state", service: &unpartitionedService{fakeService{name: "service", port: 1, handler: handler, stateful: true}}},
		{name: "unknown partition", service: &fakeService{name: "service", port: 1, handler: handler, stateful: true, partition: "request"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := (&Registry{}).Register(tt.service); err == nil {
				t.Fatal("Register() accepted invalid service")
			}
		})
	}
}

func TestRegistryValidatesDuplicatesBeforeStatefulness(t *testing.T) {
	registry := &Registry{}
	handler := http.NotFoundHandler()
	if err := registry.Register(&fakeService{name: "first", port: 1, handler: handler}); err != nil {
		t.Fatal(err)
	}
	err := registry.Register(&fakeService{name: "first", port: 1, handler: handler, stateful: true})
	if err == nil || err.Error() != `testbench: services "first" and "first" both claim port 1` {
		t.Fatalf("Register() error = %v", err)
	}
}

func TestRegistryAcceptsPartitionedState(t *testing.T) {
	registry := &Registry{}
	if err := registry.Register(&fakeService{name: "analytics", port: 1, handler: http.NotFoundHandler(), stateful: true, partition: PartitionByBlock}); err != nil {
		t.Fatal(err)
	}
}

func TestRegistrySupportsConcurrentAccess(t *testing.T) {
	registry := &Registry{}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = registry.Register(&fakeService{name: "service-" + string(rune('a'+i)), port: i + 1, handler: http.NotFoundHandler()})
			_ = registry.Services()
		}(i)
	}
	wg.Wait()
	if got := len(registry.Services()); got != 20 {
		t.Fatalf("registered services = %d, want 20", got)
	}
}
