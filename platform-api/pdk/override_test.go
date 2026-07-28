/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package pdk

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func request() *http.Request {
	return httptest.NewRequest(http.MethodGet, "/api/v0.9/gateways/gw-1", nil)
}

// A handler that writes a body without a status is a 200 on the wire, so the
// capture must report the same — a decorator branching on res.Status would
// otherwise treat every such response as an error.
func TestInvoke_StatusDefaultsTo200(t *testing.T) {
	res := Invoke(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"gw-1"}`))
	}), request())

	if res.Status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Status)
	}
	if string(res.Body) != `{"id":"gw-1"}` {
		t.Fatalf("unexpected captured body: %q", res.Body)
	}
}

func TestInvoke_PreservesExplicitStatusAndHeaders(t *testing.T) {
	res := Invoke(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Add("X-Platform-Trace", "abc")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}), request())

	if res.Status != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", res.Status)
	}
	if got := res.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected the handler's Content-Type to survive, got %q", got)
	}
	if got := res.Header.Get("X-Platform-Trace"); got != "abc" {
		t.Fatalf("expected the handler's custom header to survive, got %q", got)
	}
}

// net/http ignores a second WriteHeader; the capture must not diverge, or a
// decorator sees a status the client would never have received.
func TestInvoke_SecondWriteHeaderIsIgnored(t *testing.T) {
	res := Invoke(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.WriteHeader(http.StatusOK)
	}), request())

	if res.Status != http.StatusNotFound {
		t.Fatalf("expected the first status (404) to win, got %d", res.Status)
	}
}

func TestInvoke_HandlerWritingNothingYields200AndNilBody(t *testing.T) {
	res := Invoke(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), request())

	if res.Status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Status)
	}
	if res.Body != nil {
		t.Fatalf("expected a nil body, got %q", res.Body)
	}
}

// A streaming handler must fail visibly under an override rather than silently
// buffering, so the capture writer implements neither optional interface.
func TestInvoke_WriterIsNeitherFlusherNorHijacker(t *testing.T) {
	var isFlusher, isHijacker bool
	Invoke(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, isFlusher = w.(http.Flusher)
		_, isHijacker = w.(http.Hijacker)
	}), request())

	if isFlusher {
		t.Error("capture writer must not implement http.Flusher")
	}
	if isHijacker {
		t.Error("capture writer must not implement http.Hijacker")
	}
}

// The error branch of a decorator passes core's response through untouched, so
// a non-2xx must round-trip byte for byte.
func TestWriteCaptured_RoundTripsANon2xxUnchanged(t *testing.T) {
	body := `{"error":"not_found","message":"Gateway not found."}`
	res := Invoke(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(body))
	}), request())

	rec := httptest.NewRecorder()
	WriteCaptured(rec, res)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
	if rec.Body.String() != body {
		t.Fatalf("expected the body to round-trip unchanged, got %q", rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected Content-Type to be forwarded, got %q", got)
	}
}

// A decorator that rewrote the body would otherwise emit the core handler's
// stale length, truncating or hanging the response.
func TestWriteCaptured_DropsStaleContentLength(t *testing.T) {
	res := Invoke(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "999")
		_, _ = w.Write([]byte("short"))
	}), request())

	rec := httptest.NewRecorder()
	WriteCaptured(rec, res)

	if got := rec.Header().Get("Content-Length"); got != "" {
		t.Fatalf("expected the captured Content-Length to be dropped, got %q", got)
	}
}

func TestWriteCaptured_NilResponseIsANoOp(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteCaptured(rec, nil)

	if rec.Body.Len() != 0 {
		t.Fatalf("expected nothing to be written, got %q", rec.Body.String())
	}
}
