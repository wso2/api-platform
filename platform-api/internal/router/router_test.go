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

package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// handlerWriting returns a handler that identifies itself in the response body,
// so a test can tell which of two registrations for the same pattern was kept.
func handlerWriting(body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	})
}

// bodyOf drives h with a throwaway request and returns what it wrote.
func bodyOf(t *testing.T, h http.Handler) string {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	return rec.Body.String()
}

// Install order onto the real mux has to reproduce today's registration
// sequence, so the recorder must preserve it rather than iterating a map.
func TestRecorder_PreservesRegistrationOrder(t *testing.T) {
	rec := NewRecorder()
	want := []string{"GET /a", "POST /b", "GET /c/{id}", "DELETE /d"}
	for _, p := range want {
		rec.Handle(p, handlerWriting(p))
	}

	if err := rec.Err(); err != nil {
		t.Fatalf("unexpected registration error: %v", err)
	}
	routes := rec.Routes()
	if len(routes) != len(want) {
		t.Fatalf("expected %d routes, got %d", len(want), len(routes))
	}
	for i, p := range want {
		if routes[i].Pattern != p {
			t.Fatalf("route %d: expected pattern %q, got %q", i, p, routes[i].Pattern)
		}
	}
}

func TestRecorder_HandleFuncWrapsToHandlerFunc(t *testing.T) {
	rec := NewRecorder()
	rec.HandleFunc("GET /a", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("from-func"))
	})

	if err := rec.Err(); err != nil {
		t.Fatalf("unexpected registration error: %v", err)
	}
	routes := rec.Routes()
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	if got := bodyOf(t, routes[0].Handler); got != "from-func" {
		t.Fatalf("recorded handler wrote %q, want %q", got, "from-func")
	}
}

// A duplicate pattern would panic on a real ServeMux. The recorder defers it to
// Err() instead, and must not let the second registration displace the first —
// otherwise the error and the routes that would be installed disagree.
func TestRecorder_DuplicatePatternRecordsErrorAndKeepsFirstHandler(t *testing.T) {
	rec := NewRecorder()
	rec.Handle("GET /a", handlerWriting("first"))
	rec.Handle("GET /a", handlerWriting("second"))

	err := rec.Err()
	if err == nil {
		t.Fatal("expected a duplicate-pattern error, got nil")
	}
	if !strings.Contains(err.Error(), "GET /a") {
		t.Fatalf("error should name the duplicated pattern, got: %v", err)
	}
	routes := rec.Routes()
	if len(routes) != 1 {
		t.Fatalf("expected the duplicate to be dropped, got %d routes", len(routes))
	}
	if got := bodyOf(t, routes[0].Handler); got != "first" {
		t.Fatalf("expected the first handler to be retained, got %q", got)
	}
}

// Err() reports the FIRST failure: a later one must not overwrite it, or the
// startup message points at the wrong route.
func TestRecorder_ErrReportsFirstFailureOnly(t *testing.T) {
	rec := NewRecorder()
	rec.Handle("", handlerWriting("x"))
	rec.Handle("GET /b", nil)

	err := rec.Err()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "empty pattern") {
		t.Fatalf("expected the first (empty pattern) error to be kept, got: %v", err)
	}
}

func TestRecorder_RejectsEmptyPatternAndNilHandler(t *testing.T) {
	tests := []struct {
		name     string
		register func(*Recorder)
	}{
		{"empty pattern", func(rec *Recorder) { rec.Handle("", handlerWriting("x")) }},
		{"nil handler", func(rec *Recorder) { rec.Handle("GET /a", nil) }},
		{"nil handler func", func(rec *Recorder) { rec.HandleFunc("GET /a", nil) }},
		{"empty pattern via HandleFunc", func(rec *Recorder) {
			rec.HandleFunc("", func(http.ResponseWriter, *http.Request) {})
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := NewRecorder()
			tc.register(rec)

			if rec.Err() == nil {
				t.Fatal("expected a registration error, got nil")
			}
			if len(rec.Routes()) != 0 {
				t.Fatalf("expected the bad registration to be a no-op, got %d routes", len(rec.Routes()))
			}
		})
	}
}

// Has decides whether a plugin's override pattern exists in core, so an
// almost-right pattern must not match: it would attach a decorator to a route
// its author did not mean.
func TestRecorder_HasIsAnExactStringMatch(t *testing.T) {
	rec := NewRecorder()
	rec.Handle("GET /api/v0.9/gateways/{gatewayId}", handlerWriting("x"))

	if !rec.Has("GET /api/v0.9/gateways/{gatewayId}") {
		t.Fatal("expected the recorded pattern to be found")
	}
	for _, near := range []string{
		"GET /api/v1/gateways/{gatewayId}",    // wrong version
		"GET /api/v0.9/gateways/{id}",         // wrong wildcard name
		"POST /api/v0.9/gateways/{gatewayId}", // wrong method
		"/api/v0.9/gateways/{gatewayId}",      // no method
		"GET  /api/v0.9/gateways/{gatewayId}", // extra space
	} {
		if rec.Has(near) {
			t.Fatalf("pattern %q must not match the recorded route", near)
		}
	}
}

// Routes() hands out a copy, so a caller mutating the returned slice cannot
// change what gets installed.
func TestRecorder_RoutesReturnsACopy(t *testing.T) {
	rec := NewRecorder()
	rec.Handle("GET /a", handlerWriting("a"))

	routes := rec.Routes()
	routes[0].Pattern = "GET /mutated"
	routes = append(routes, Route{Pattern: "GET /appended", Handler: handlerWriting("b")})
	_ = routes

	again := rec.Routes()
	if len(again) != 1 {
		t.Fatalf("expected the recorder to still hold 1 route, got %d", len(again))
	}
	if again[0].Pattern != "GET /a" {
		t.Fatalf("recorder state was mutated through the returned slice: %q", again[0].Pattern)
	}
}
