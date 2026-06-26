package httputil_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2/go-httpkit/httputil"
)

// ---- WriteJSON ----

func TestWriteJSON_SetsContentTypeAndStatus(t *testing.T) {
	rr := httptest.NewRecorder()
	httputil.WriteJSON(rr, http.StatusCreated, map[string]string{"key": "value"})
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	var got map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got["key"] != "value" {
		t.Fatalf("body key = %q, want value", got["key"])
	}
}

// ---- WriteError ----

func TestWriteError_Body(t *testing.T) {
	rr := httptest.NewRecorder()
	httputil.WriteError(rr, http.StatusBadRequest, "invalid_input", "bad request")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	var got httputil.ErrorBody
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Code != "invalid_input" || got.Message != "bad request" {
		t.Fatalf("unexpected body: %+v", got)
	}
}

// ---- WriteNoContent ----

func TestWriteNoContent(t *testing.T) {
	rr := httptest.NewRecorder()
	httputil.WriteNoContent(rr)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
}

// ---- DecodeJSON ----

func TestDecodeJSON_ValidBody(t *testing.T) {
	type Payload struct {
		Name string `json:"name"`
	}
	body := `{"name":"alice"}`
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	got, err := httputil.DecodeJSON[Payload](req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "alice" {
		t.Fatalf("Name = %q, want alice", got.Name)
	}
}

func TestDecodeJSON_EmptyBody(t *testing.T) {
	type Payload struct{ Name string }
	req := httptest.NewRequest("POST", "/", strings.NewReader(""))
	_, err := httputil.DecodeJSON[Payload](req)
	if err == nil {
		t.Fatal("expected error for empty body")
	}
}

func TestDecodeJSON_MalformedJSON(t *testing.T) {
	type Payload struct{ Name string }
	req := httptest.NewRequest("POST", "/", strings.NewReader("{bad json}"))
	_, err := httputil.DecodeJSON[Payload](req)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

// ---- PathParam / QueryParam ----

func TestPathParam(t *testing.T) {
	mux := http.NewServeMux()
	var got string
	mux.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, r *http.Request) {
		got = httputil.PathParam(r, "id")
	})
	req := httptest.NewRequest("GET", "/items/42", nil)
	mux.ServeHTTP(httptest.NewRecorder(), req)
	if got != "42" {
		t.Fatalf("PathParam = %q, want 42", got)
	}
}

func TestQueryParam(t *testing.T) {
	req := httptest.NewRequest("GET", "/items?limit=10", nil)
	if got := httputil.QueryParam(req, "limit"); got != "10" {
		t.Fatalf("QueryParam = %q, want 10", got)
	}
}
