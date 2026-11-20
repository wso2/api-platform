/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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

// Package counters is the gateway's shared counter service as policies see it:
// named totals that every policy engine of a gateway, and every gateway sharing a
// store, adds to and reads. Budgets count spend with it; a token or request quota
// could count the same way.
//
// Totals are shared with about a second of lag and reads use the engine's local copy,
// so a limit built on them can be overshot by what arrives within that second. Use
// them for long-window limits (per day, week or month), not for strict short-window
// rate limiting, which needs a check against the store on every request.
//
// This package is only the contract. The policy engine implements it: Add keeps the
// amount in memory and the engine sends the sums to its gateway-controller about once
// a second; the controller applies them to the shared store (Redis, or memory when
// none is configured) and sends back the totals of the keys the engine uses. Reads
// never wait on the network. The engine calls Install once at startup.
//
// When nothing is installed (an engine that predates this package, or a unit test)
// a process-local in-memory service is used, so a policy works either way.
package counters

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Key names one counter. Window is the start of the period the counter covers: a
// new window is a new counter, starting at zero. Use the zero time for a counter
// that never resets.
type Key struct {
	// Namespace keeps one policy's counters apart from another's, e.g. "budget".
	Namespace string
	// ID names the counter within its namespace, e.g. a budget allowance UUID.
	ID string
	// Window is the start of the counter's period.
	Window time.Time
}

// Service is implemented by the policy engine.
type Service interface {
	// Add adds delta to the counter. It never blocks on the network.
	Add(key Key, delta float64)
	// Total returns the shared total plus this engine's adds not yet applied to it.
	// ok is false while the shared total is not known yet (the first use of a
	// counter on this engine): the total then counts only this engine's adds, and
	// the shared total is requested.
	Total(key Key) (total float64, ok bool)
}

// Describer is implemented by a Service that reports what its counters are: the
// policy engine forwards the labels to its gateway-controller with the counter's adds,
// so the controller can report the counter's total (budgets report spend per
// allowance to the control plane). Labels are never written to the shared store.
type Describer interface {
	// Describe sets the counter's labels, replacing any it had.
	Describe(key Key, labels map[string]string)
}

// Forgetter is implemented by a Service that caches totals: Forget drops what it holds
// for the counters of a namespace whose IDs start with idPrefix, including adds not yet
// reported, so counters that were cleared in the shared store (a deleted budget's) are
// not judged by, or written back from, a stale copy.
type Forgetter interface {
	Forget(namespace, idPrefix string)
}

// serviceHolder lets atomic.Pointer hold an interface value.
type serviceHolder struct{ svc Service }

var installed atomic.Pointer[serviceHolder]

// Install makes svc the process-wide counter service. Called once by the policy
// engine at startup; not intended to be called by policies. Passing nil uninstalls
// it (tests only).
func Install(svc Service) {
	if svc == nil {
		installed.Store(nil)
		return
	}
	installed.Store(&serviceHolder{svc: svc})
}

// Shared reports whether the installed service shares totals beyond this process.
func Shared() bool {
	return installed.Load() != nil
}

func current() Service {
	if h := installed.Load(); h != nil {
		return h.svc
	}
	return local
}

// Add adds delta to the counter.
func Add(key Key, delta float64) {
	current().Add(key, delta)
}

// Total returns the counter's total; see Service.Total.
func Total(key Key) (float64, bool) {
	return current().Total(key)
}

// Describe labels the counter for reporting when the installed service reports
// counters (Describer); otherwise it does nothing.
func Describe(key Key, labels map[string]string) {
	if d, ok := current().(Describer); ok {
		d.Describe(key, labels)
	}
}

// Forget drops the cached counters of namespace whose IDs start with idPrefix when the
// installed service caches them (Forgetter); otherwise it does nothing.
func Forget(namespace, idPrefix string) {
	if f, ok := current().(Forgetter); ok {
		f.Forget(namespace, idPrefix)
	}
}

// local is the process-local service used when none is installed.
var local Service = NewMemory()

// Memory is an in-memory Service: totals are this process's adds. It is the service
// used when none is installed, and is useful in tests.
type Memory struct {
	mu     sync.Mutex
	totals map[Key]float64
	labels map[Key]map[string]string
}

// NewMemory returns an empty in-memory Service.
func NewMemory() *Memory {
	return &Memory{totals: map[Key]float64{}, labels: map[Key]map[string]string{}}
}

// Add adds delta to the counter.
func (m *Memory) Add(key Key, delta float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totals[key] += delta
}

// Total returns the counter's total; it is always known.
func (m *Memory) Total(key Key) (float64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.totals[key], true
}

// Describe records the counter's labels; there is no controller to report them to.
func (m *Memory) Describe(key Key, labels map[string]string) {
	copied := make(map[string]string, len(labels))
	for k, v := range labels {
		copied[k] = v
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.labels[key] = copied
}

// Labels returns the counter's labels; useful in tests.
func (m *Memory) Labels(key Key) map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.labels[key]
}

// Forget drops the counters of namespace whose IDs start with idPrefix.
func (m *Memory) Forget(namespace, idPrefix string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key := range m.totals {
		if key.Namespace == namespace && strings.HasPrefix(key.ID, idPrefix) {
			delete(m.totals, key)
			delete(m.labels, key)
		}
	}
}
