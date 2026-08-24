package httpclient

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMaxBytesReadCloser_AllowsExactLimit(t *testing.T) {
	body := io.NopCloser(strings.NewReader("0123456789")) // exactly 10 bytes
	m := &maxBytesReadCloser{body: body, max: 10}

	data, err := io.ReadAll(m)
	if err != nil {
		t.Fatalf("expected a body of exactly the limit to read cleanly, got: %v", err)
	}
	if string(data) != "0123456789" {
		t.Fatalf("data = %q", data)
	}
}

func TestMaxBytesReadCloser_ErrorsPastLimit(t *testing.T) {
	body := io.NopCloser(strings.NewReader("0123456789EXTRA"))
	m := &maxBytesReadCloser{body: body, max: 10}

	_, err := io.ReadAll(m)
	if err == nil {
		t.Fatal("expected an error for a body exceeding the configured limit")
	}
}

// TestNew_ResponseBodyIsBoundedByDefault proves DefaultConfig's
// MaxResponseBytes is actually enforced end-to-end through New, not just
// present as an unused config field.
func TestNew_ResponseBodyIsBoundedByDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(make([]byte, 1024))
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.Timeouts.MaxResponseBytes = 100 // well under the 1024-byte response
	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("client.Get: %v", err)
	}
	defer resp.Body.Close()

	if _, err := io.ReadAll(resp.Body); err == nil {
		t.Fatal("expected reading a response body over the configured MaxResponseBytes to fail")
	}
}

func TestNew_ResponseBodyBoundCanBeDisabled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(make([]byte, 1024))
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.Timeouts.MaxResponseBytes = -1 // opt-out
	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("client.Get: %v", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("expected the response to read fully when the bound is disabled, got: %v", err)
	}
	if len(data) != 1024 {
		t.Fatalf("len(data) = %d, want 1024", len(data))
	}
}
