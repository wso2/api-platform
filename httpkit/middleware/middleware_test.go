package middleware_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2/go-httpkit/middleware"
)

func okHandler(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

// ---- Chain ----

func TestChain_OrderIsOuterToInner(t *testing.T) {
	var order []string
	mk := func(name string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name+"-before")
				next.ServeHTTP(w, r)
				order = append(order, name+"-after")
			})
		}
	}
	h := middleware.Chain(mk("A"), mk("B"), mk("C"))(http.HandlerFunc(okHandler))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))

	want := []string{"A-before", "B-before", "C-before", "C-after", "B-after", "A-after"}
	for i, v := range want {
		if order[i] != v {
			t.Fatalf("order[%d] = %q, want %q", i, order[i], v)
		}
	}
}

// ---- CorrelationID ----

func TestCorrelationIDMiddleware_GeneratesID(t *testing.T) {
	mw := middleware.CorrelationIDMiddleware(slog.Default())
	var got string
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = middleware.GetCorrelationID(r)
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))

	if got == "" {
		t.Fatal("expected a generated correlation ID, got empty string")
	}
	if rr.Header().Get(middleware.CorrelationIDHeader) != got {
		t.Fatalf("response header %q = %q, want %q", middleware.CorrelationIDHeader, rr.Header().Get(middleware.CorrelationIDHeader), got)
	}
}

func TestCorrelationIDMiddleware_PreservesExistingID(t *testing.T) {
	mw := middleware.CorrelationIDMiddleware(slog.Default())
	const existingID = "test-id-123"
	var got string
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = middleware.GetCorrelationID(r)
	}))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(middleware.CorrelationIDHeader, existingID)
	h.ServeHTTP(httptest.NewRecorder(), req)

	if got != existingID {
		t.Fatalf("got %q, want %q", got, existingID)
	}
}

// ---- Recovery ----

func TestRecoveryMiddleware_CatchesPanic(t *testing.T) {
	mw := middleware.RecoveryMiddleware(slog.Default())
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

// ---- CORS ----

func TestCORSMiddleware_SetsAllowOrigin(t *testing.T) {
	opts := middleware.CORSOptions{AllowedOrigins: []string{"https://example.com"}}
	mw := middleware.CORSMiddleware(opts)
	h := mw(http.HandlerFunc(okHandler))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Fatalf("Allow-Origin = %q, want %q", got, "https://example.com")
	}
}

func TestCORSMiddleware_RejectsUnknownOrigin(t *testing.T) {
	opts := middleware.CORSOptions{AllowedOrigins: []string{"https://example.com"}}
	mw := middleware.CORSMiddleware(opts)
	h := mw(http.HandlerFunc(okHandler))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://evil.com")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no Allow-Origin header, got %q", got)
	}
}

func TestWithCORS_PreflightReturns204(t *testing.T) {
	opts := middleware.CORSOptions{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST"},
	}
	_, h := middleware.WithCORS("GET /test", okHandler, opts)
	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
}
