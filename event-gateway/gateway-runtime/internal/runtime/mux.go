/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package runtime

import (
	"net/http"
	"sync"
)

// DynamicMux is an HTTP request multiplexer that supports adding and removing
// handlers dynamically. Unlike http.ServeMux, it does not panic on duplicate
// pattern registration — it overwrites the existing handler instead.
type DynamicMux struct {
	mu       sync.RWMutex
	handlers map[string]http.Handler
}

// NewDynamicMux creates a new DynamicMux.
func NewDynamicMux() *DynamicMux {
	return &DynamicMux{
		handlers: make(map[string]http.Handler),
	}
}

// Handle registers the handler for the given pattern.
// If a handler already exists for the pattern, it is replaced.
func (m *DynamicMux) Handle(pattern string, handler http.Handler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[pattern] = handler
}

// HandleFunc registers the handler function for the given pattern.
func (m *DynamicMux) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	m.Handle(pattern, http.HandlerFunc(handler))
}

// Remove deregisters the handler for the given pattern.
func (m *DynamicMux) Remove(pattern string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.handlers, pattern)
}

// ServeHTTP dispatches the request to the handler whose pattern matches the
// request URL path. Exact-match only; returns 404 if no match.
func (m *DynamicMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	handler, ok := m.handlers[r.URL.Path]
	m.mu.RUnlock()

	if ok {
		handler.ServeHTTP(w, r)
		return
	}
	http.NotFound(w, r)
}
