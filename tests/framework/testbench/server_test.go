package testbench

import (
	"bytes"
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
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

type unpartitionedService fakeService

func (s *unpartitionedService) Name() string          { return s.name }
func (s *unpartitionedService) Port() int             { return s.port }
func (s *unpartitionedService) Handler() http.Handler { return s.handler }
func (s *unpartitionedService) Stateful() bool        { return s.stateful }

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
		{name: "unpartitioned state", service: &unpartitionedService{name: "service", port: 1, handler: handler, stateful: true}},
		{name: "unknown partition", service: &fakeService{name: "service", port: 1, handler: handler, stateful: true, partition: "request"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (&Registry{}).Register(tt.service)
			if err == nil {
				t.Fatal("Register() accepted invalid service")
			}
			if tt.name == "unpartitioned state" && !containsError(err, "cannot be hosted") {
				t.Fatalf("Register() error = %v, want missing partitioning error", err)
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

// captureLogs runs h against req with the default logger redirected, returning the emitted
// JSON lines so a test can assert on what an operator would actually see.
func captureLogs(t *testing.T, h http.Handler, req *http.Request) (string, *httptest.ResponseRecorder) {
	t.Helper()
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return buf.String(), rec
}

// Every request leaves one access line carrying the fields needed to correlate it with a
// suite step: service, correlation id, method, path, status, duration, and byte counts.
func TestObservabilityEmitsAnAccessLinePerRequest(t *testing.T) {
	h := observability("oauth2", limitBody(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	out, rec := captureLogs(t, h, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/debug/reset", strings.NewReader("{}")))

	require.Contains(t, out, `"msg":"testbench request served"`)
	for _, field := range []string{`"service":"oauth2"`, `"method":"POST"`, `"path":"/debug/reset"`,
		`"status":204`, `"duration_ms"`, `"response_bytes"`, `"request_bytes"`, `"request_id"`} {
		require.Contains(t, out, field)
	}
	// The id is echoed so a client can correlate from its own side.
	require.NotEmpty(t, rec.Header().Get(RequestIDHeader))
}

func TestObservabilityDoesNotLogQueryValues(t *testing.T) {
	h := observability("jwks", limitBody(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	secret := "expected-secret-must-not-be-logged"
	out, _ := captureLogs(t, h, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "/token?expected_secret="+secret, nil))

	require.Contains(t, out, `"has_query":true`)
	require.NotContains(t, out, secret)
	require.NotContains(t, out, `"query"`)
}

// A caller-supplied correlation id is honoured rather than replaced, which is what lets a
// suite step name the exact request it is asserting about.
func TestObservabilityHonoursAnInboundRequestID(t *testing.T) {
	h := observability("echo", limitBody(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", nil)
	req.Header.Set(RequestIDHeader, "suite-step-42")

	out, rec := captureLogs(t, h, req)
	require.Equal(t, "suite-step-42", rec.Header().Get(RequestIDHeader))
	require.Contains(t, out, `"request_id":"suite-step-42"`)
}

// The reason a mock rejected a request is the one thing an access line cannot infer, so the
// error body is captured into the log rather than left only in the client's response.
func TestObservabilityRecordsTheRejectionReason(t *testing.T) {
	h := observability("analytics", limitBody(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "analytics collector: malformed batch payload", http.StatusBadRequest)
	})))
	out, _ := captureLogs(t, h, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/block/v1/events", nil))

	require.Contains(t, out, `"msg":"testbench request rejected"`)
	require.Contains(t, out, "malformed batch payload")
	require.Contains(t, out, `"status":400`)
}

func TestObservabilityOmitsJSONFailureBodies(t *testing.T) {
	h := observability("echo", limitBody(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"secret":"must-not-be-logged"}`))
	})))
	out, _ := captureLogs(t, h, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/status/400", nil))

	require.NotContains(t, out, "must-not-be-logged")
	require.NotContains(t, out, `"reason"`)
}

// A 5xx is surfaced at error level even though the mock returned it deliberately: a suite
// chasing a failure should not have to grep INFO lines for it.
func TestObservabilityLogsServerErrorsAtErrorLevel(t *testing.T) {
	h := observability("backend", limitBody(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "forced upstream failure", http.StatusBadGateway)
	})))
	out, _ := captureLogs(t, h, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/fail", nil))
	require.Contains(t, out, `"level":"ERROR"`)
	require.Contains(t, out, `"msg":"testbench request failed"`)
}

// Thirteen services share one process, so a panic in any handler must be contained and
// reported with a stack rather than taking every concurrent block down with it.
func TestObservabilityRecoversAndReportsAPanic(t *testing.T) {
	h := observability("mcp", limitBody(PartitionRouter(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom in a mock")
	}))))
	out, rec := captureLogs(t, h, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/panic-block/x", nil))

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Contains(t, out, `"msg":"testbench handler panicked"`)
	require.Contains(t, out, "boom in a mock")
	require.Contains(t, out, `"stack"`)
	require.Contains(t, out, `"msg":"testbench request failed"`)
	require.Contains(t, out, `"status":500`)
	require.Contains(t, out, `"response_bytes"`)
	require.Contains(t, out, `"partition":"panic-block"`)
}

func TestObservabilityAbortsAfterResponseStarted(t *testing.T) {
	h := observability("echo", limitBody(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("before panic"))
		w.(http.Flusher).Flush()
		panic("boom after response")
	})))
	server := httptest.NewUnstartedServer(h)
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.Start()
	t.Cleanup(server.Close)

	resp, err := server.Client().Get(server.URL)
	require.NoError(t, err)
	require.NotNil(t, resp)
	body, readErr := io.ReadAll(resp.Body)
	require.NoError(t, resp.Body.Close())
	require.Error(t, readErr)
	require.NotContains(t, string(body), "boom after response")
	require.NotContains(t, string(body), "testbench handler panicked")
}

// The partition is what ties a log line to one block out of the fifty-plus sharing this
// container, so it must appear on the access line and not only inside the router.
func TestObservabilityReportsThePartitionOnce(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, r, http.StatusOK, map[string]string{"path": r.URL.Path})
	})
	h := observability("capture", limitBody(NormalizeMethod(PartitionRouter(inner))))
	out, rec := captureLogs(t, h, httptest.NewRequestWithContext(t.Context(), "get", "/myblock/test/captured", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, out, `"msg":"testbench request served"`)
	require.Contains(t, out, `"partition":"myblock"`)
	// A lowercase method is normalized, and that rewrite is itself recorded.
	require.Contains(t, out, `"msg":"testbench normalized the request method"`)
	// One partition attribute per line, not two.
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		require.LessOrEqual(t, strings.Count(line, `"partition":`), 1, "duplicated attribute in %s", line)
	}
}

// A partitioned service addressed without its prefix answers a bare 400; the log is the only
// place the cause is visible.
func TestPartitionRouterLogsARejectedPath(t *testing.T) {
	h := observability("capture", limitBody(PartitionRouter(http.NotFoundHandler())))
	out, rec := captureLogs(t, h, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, out, `"msg":"testbench rejected a partition path"`)
	require.Contains(t, out, `"raw_path":"/"`)
}

// Fail carries structured detail an access line cannot infer from the response alone.
func TestFailLogsStructuredDetailAlongsideTheResponse(t *testing.T) {
	h := observability("oauth2", limitBody(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Fail(w, r, http.StatusUnauthorized, "invalid client credentials",
			"client_id", "test-client", "auth_style", "basic")
	})))
	out, rec := captureLogs(t, h, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/oauth2/token", nil))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, out, `"client_id":"test-client"`)
	require.Contains(t, out, `"auth_style":"basic"`)
	require.Contains(t, out, `"message":"invalid client credentials"`)
}
