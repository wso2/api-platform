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

// Package router holds the registration sink core route registration writes to.
//
// Core handlers register on a Recorder instead of the real *http.ServeMux so the
// server can install each recorded pattern later — wrapped, where a plugin has
// claimed it with a route override. Nothing here knows about plugins, overrides,
// or the server; it only records what was registered, in order.
package router

import (
	"fmt"
	"net/http"
)

// Router is the subset of *http.ServeMux that route registration uses.
// *http.ServeMux satisfies it, so a caller can register on a real mux or on a
// Recorder without either side knowing which. The public plugin contracts
// (pdk.Plugin, plugin.Plugin) keep *http.ServeMux — this interface exists for
// core route registration only.
type Router interface {
	Handle(pattern string, handler http.Handler)
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

// Route is one recorded registration: the pattern exactly as it was passed in,
// and the handler to serve it.
type Route struct {
	Pattern string
	Handler http.Handler
}

// Recorder records route registrations instead of serving them.
//
// It deliberately has no ServeHTTP: a Recorder is a registration sink, not a
// router, so it is impossible to ship a build where the recorder is serving
// traffic. Registrations are kept in insertion order, so installing them onto a
// real mux reproduces the original registration sequence.
//
// A Recorder is not safe for concurrent use; route registration happens on the
// single startup goroutine.
type Recorder struct {
	routes []Route
	// index maps pattern -> position in routes, for O(1) Has and duplicate
	// detection. Insertion order lives in routes, not here.
	index map[string]int
	// err is the FIRST registration error. Handle cannot return an error (the
	// signature is fixed by *http.ServeMux compatibility) and panicking would
	// only reproduce ServeMux's behaviour with a worse message, so errors are
	// deferred to a single Err() check by the caller. Deferring is safe because
	// nothing serves traffic until the recorded routes are installed.
	err error
}

// NewRecorder returns an empty Recorder.
func NewRecorder() *Recorder {
	return &Recorder{index: make(map[string]int)}
}

// Handle records handler under pattern. A duplicate pattern, an empty pattern,
// or a nil handler records an error (if none is recorded yet) and makes the call
// a no-op, keeping the first handler registered for that pattern.
func (rec *Recorder) Handle(pattern string, handler http.Handler) {
	switch {
	case pattern == "":
		rec.fail(fmt.Errorf("route registered with an empty pattern"))
		return
	case handler == nil:
		rec.fail(fmt.Errorf("route %q registered with a nil handler", pattern))
		return
	}
	if _, dup := rec.index[pattern]; dup {
		rec.fail(fmt.Errorf("route %q registered more than once", pattern))
		return
	}
	rec.index[pattern] = len(rec.routes)
	rec.routes = append(rec.routes, Route{Pattern: pattern, Handler: handler})
}

// HandleFunc records handler under pattern, converting it to an http.Handler the
// same way *http.ServeMux does.
func (rec *Recorder) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	if handler == nil {
		rec.fail(fmt.Errorf("route %q registered with a nil handler", pattern))
		return
	}
	rec.Handle(pattern, http.HandlerFunc(handler))
}

// Has reports whether pattern was recorded. The match is an exact string
// comparison on the full `"METHOD /path/{wildcard}"` pattern: "GET /x" and
// "GET  /x" are distinct patterns to ServeMux too, and a fuzzy match would let a
// route override silently attach to a route its author did not mean.
func (rec *Recorder) Has(pattern string) bool {
	_, ok := rec.index[pattern]
	return ok
}

// Routes returns the recorded routes in registration order. The slice is a copy,
// so a caller cannot append into the recorder's own storage.
func (rec *Recorder) Routes() []Route {
	out := make([]Route, len(rec.routes))
	copy(out, rec.routes)
	return out
}

// Err returns the first registration error, or nil. Callers check it once, after
// all core routes have been registered and before anything is installed.
func (rec *Recorder) Err() error {
	return rec.err
}

// fail records err as the first error, if none is recorded yet.
func (rec *Recorder) fail(err error) {
	if rec.err == nil {
		rec.err = err
	}
}

// compile-time check that a real mux is usable wherever a Router is expected.
var (
	_ Router = (*http.ServeMux)(nil)
	_ Router = (*Recorder)(nil)
)
