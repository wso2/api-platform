package testbench

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

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
