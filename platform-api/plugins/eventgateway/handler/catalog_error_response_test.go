package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// The plugin handlers used to translate internal sentinel errors with errors.Is.
// After the sentinels moved to the catalog, a stale guard would still compile but
// silently fall through to a 500. These lock in the catalog pass-through.
func TestRespondCatalogError_PassesThroughServiceLayerError(t *testing.T) {
	// utils.ValidateHandleImmutable is an internal helper the plugin services call.
	handle := "different-handle"
	err := utils.ValidateHandleImmutable("real-handle", &handle)
	if err == nil {
		t.Fatal("expected an immutability error")
	}

	rec := httptest.NewRecorder()
	if !respondCatalogError(rec, testLogger(), err) {
		t.Fatal("respondCatalogError should have handled a catalog error")
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var body apperror.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not a valid ErrorResponse: %v", err)
	}
	if body.Code != apperror.CodeCommonValidationFailed {
		t.Errorf("code = %q, want %q", body.Code, apperror.CodeCommonValidationFailed)
	}
	// The body must carry the sterile catalog message, never err.Error() (which
	// would prefix the code and append the wrapped cause).
	if got, want := body.Message, "The id is immutable and cannot be changed."; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

func TestRespondCatalogError_IgnoresNonCatalogError(t *testing.T) {
	rec := httptest.NewRecorder()
	if respondCatalogError(rec, testLogger(), errPlain{}) {
		t.Error("respondCatalogError must not handle a non-catalog error")
	}
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Error("respondCatalogError must not write anything for a non-catalog error")
	}
}

// A 5xx must never leave the process unlogged. The plugin does not run behind
// middleware.MapErrors, so respondCatalogError is the only thing standing
// between a server fault and a silent 500 — an earlier version wrote the
// response with no log line and no stack at all.
func TestRespondCatalogError_ServerErrorIsLoggedWithStack(t *testing.T) {
	var buf bytes.Buffer
	slogger := slog.New(slog.NewTextHandler(&buf, nil))

	rec := httptest.NewRecorder()
	err := apperror.Internal.Wrap(errPlain{}).WithLogMessage("db pool exhausted")
	if !respondCatalogError(rec, slogger, err) {
		t.Fatal("respondCatalogError should have handled a catalog error")
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	out := buf.String()
	if !strings.Contains(out, "level=ERROR") {
		t.Errorf("a 5xx must log at ERROR; got: %s", out)
	}
	if !strings.Contains(out, "stack=") {
		t.Errorf("a 5xx must log the origin stack; got: %s", out)
	}
	// The wrapped cause and the internal detail belong in the log, never the body.
	if !strings.Contains(out, "db pool exhausted") {
		t.Errorf("log must carry the internal detail; got: %s", out)
	}
	if !strings.Contains(out, "some driver failure") {
		t.Errorf("log must carry the wrapped cause; got: %s", out)
	}

	var body apperror.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not a valid ErrorResponse: %v", err)
	}
	if body.TrackingID == "" {
		t.Error("a 5xx response must echo a tracking ID for log correlation")
	}
	if strings.Contains(body.Message, "driver") || strings.Contains(body.Message, "pool") {
		t.Errorf("internal detail leaked into the client message: %q", body.Message)
	}
}

// A 4xx is a client outcome, not a system fault: WARN, no stack, no tracking ID.
func TestRespondCatalogError_ClientErrorIsWarnWithoutStack(t *testing.T) {
	var buf bytes.Buffer
	slogger := slog.New(slog.NewTextHandler(&buf, nil))

	rec := httptest.NewRecorder()
	if !respondCatalogError(rec, slogger, apperror.WebSubAPINotFound.New()) {
		t.Fatal("respondCatalogError should have handled a catalog error")
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	out := buf.String()
	if !strings.Contains(out, "level=WARN") {
		t.Errorf("a 4xx must log at WARN; got: %s", out)
	}
	if strings.Contains(out, "stack=") {
		t.Errorf("a 4xx must not log a stack; got: %s", out)
	}
}

type errPlain struct{}

func (errPlain) Error() string { return "some driver failure" }
