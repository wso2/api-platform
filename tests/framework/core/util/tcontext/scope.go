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

package tcontext

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// contextKey is unexported so nothing outside this package can put a value where the
// scope lookups expect one, or read one out without going through the intents below.
type contextKey struct{ name string }

var (
	sharedKey = contextKey{"shared"}
	localKey  = contextKey{"local"}
)

// Shared is per-block state, populated during boot and read by every runner and scenario
// in that block.
//
// Writes are expected during boot only; after that it is effectively read-only. It is
// mutex-guarded anyway, because "expected" is not "enforced" and a data race here would
// surface as an unrelated flake rather than as a failure pointing at the write.
type Shared struct {
	mu     sync.RWMutex
	block  string
	values map[string]any
}

// NewShared returns shared state for a block.
func NewShared(block string) *Shared {
	return &Shared{block: block, values: make(map[string]any)}
}

// Block is the owning block's name, used in diagnostics so a value's origin is traceable.
func (s *Shared) Block() string { return s.block }

// Set stores a block-scoped value. Intended for boot; a scenario should be writing to
// local scope instead.
func (s *Shared) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = value
}

func (s *Shared) get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.values[key]
	return v, ok
}

func (s *Shared) keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.values))
	for k := range s.values {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Local is per-runner state: the values one runner's scenarios accumulate, and the way a
// setup feature hands fixtures to the features that follow it in the same runner.
//
// One Local exists per runner, and every scenario in that runner shares it. That is what
// makes the fixture handoff work, and it is safe because scenarios within a runner always
// run sequentially — the framework offers no way to parallelise them.
type Local struct {
	mu     sync.RWMutex
	runner string
	values map[string]any
	lists  map[string][]any
}

// NewLocal returns local state for a runner.
func NewLocal(runner string) *Local {
	return &Local{runner: runner, values: make(map[string]any), lists: make(map[string][]any)}
}

// Runner is the owning runner's name.
func (l *Local) Runner() string { return l.runner }

// Set stores a scenario-scoped value.
func (l *Local) Set(key string, value any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.values[key] = value
}

// Remove deletes a key.
//
// Used by the request funnel to clear a stored response BEFORE issuing a call, so a step
// that throws leaves the key absent rather than holding a previous step's response for the
// next assertion to pass against.
func (l *Local) Remove(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.values, key)
}

// Append adds to a named list, for accumulating created-resource ids.
func (l *Local) Append(key string, value any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lists[key] = append(l.lists[key], value)
}

// List returns a copy of a named list. A copy, so a caller iterating it while cleanup
// appends cannot observe a torn slice.
func (l *Local) List(key string) []any {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return append([]any(nil), l.lists[key]...)
}

// ClearList empties a named list, for a sweep that has finished with it.
func (l *Local) ClearList(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.lists, key)
}

// ListKeys returns every non-empty list key, sorted.
func (l *Local) ListKeys() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]string, 0, len(l.lists))
	for k, v := range l.lists {
		if len(v) > 0 {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func (l *Local) get(key string) (any, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	v, ok := l.values[key]
	return v, ok
}

func (l *Local) keys() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]string, 0, len(l.values))
	for k := range l.values {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// WithShared attaches block-scoped state to a context. Called once per block at boot.
func WithShared(ctx context.Context, s *Shared) context.Context {
	return context.WithValue(ctx, sharedKey, s)
}

// WithLocal attaches runner-scoped state to a context. Called once per runner.
func WithLocal(ctx context.Context, l *Local) context.Context {
	return context.WithValue(ctx, localKey, l)
}

// SharedOf returns the block-scoped state carried by a context.
func SharedOf(ctx context.Context) (*Shared, bool) {
	s, ok := ctx.Value(sharedKey).(*Shared)
	return s, ok
}

// LocalOf returns the runner-scoped state carried by a context.
func LocalOf(ctx context.Context) (*Local, bool) {
	l, ok := ctx.Value(localKey).(*Local)
	return l, ok
}

// ── The three read intents ───────────────────────────────────────────────────────
//
// Choosing between Resolve, Get and Contains IS the statement of intent. They are
// deliberately three functions rather than one with a flag, because the choice records
// what the caller believes about the value's presence, and that belief is exactly what a
// reader of the step needs to know.
//
// Both scopes are searched local-first, so a scenario's own value shadows a block-wide
// default of the same name — which is what lets a runner override a fixture without
// mutating shared state.

// Resolve returns a REQUIRED value, or an error naming the key and listing what is
// present.
//
// This is the default for anything a step receives as an argument. A mistyped key fails
// here, immediately and legibly, instead of surfacing later as a nil dereference in
// unrelated code.
//
// An optional <angle-bracket> wrapper is stripped, so a step can accept either a bare key
// or a Gherkin-style reference without the caller caring which.
func Resolve(ctx context.Context, key string) (any, error) {
	lookup := strings.TrimSuffix(strings.TrimPrefix(key, "<"), ">")

	if v, ok := Get(ctx, lookup); ok {
		return v, nil
	}
	return nil, fmt.Errorf("no value in context for key %q (available: %s)",
		lookup, strings.Join(Keys(ctx), ", "))
}

// ResolveString is Resolve with a string type assertion, since most step arguments are
// ids, tokens or names.
func ResolveString(ctx context.Context, key string) (string, error) {
	v, err := Resolve(ctx, key)
	if err != nil {
		return "", err
	}
	s, ok := v.(string)
	if !ok {
		// A type mismatch is as much a programming error as a missing key, and just as
		// worth failing loudly on.
		return "", fmt.Errorf("context key %q holds %T, not a string", key, v)
	}
	return s, nil
}

// Get returns a NULLABLE value: no error, just a presence flag.
//
// For framework-managed keys the caller checks itself, or where absence is a legitimate
// branch rather than a mistake. Prefer Resolve for anything a step was handed.
func Get(ctx context.Context, key string) (any, bool) {
	if l, ok := LocalOf(ctx); ok {
		if v, found := l.get(key); found {
			return v, true
		}
	}
	if s, ok := SharedOf(ctx); ok {
		if v, found := s.get(key); found {
			return v, true
		}
	}
	return nil, false
}

// Contains reports presence only.
func Contains(ctx context.Context, key string) bool {
	_, ok := Get(ctx, key)
	return ok
}

// Set stores a value in LOCAL scope. A step writes here, never to shared.
func Set(ctx context.Context, key string, value any) error {
	l, ok := LocalOf(ctx)
	if !ok {
		// Reaching this means a scenario ran without runner scope attached, which is a
		// framework wiring bug rather than anything the step author did.
		return fmt.Errorf("no local scope in context: cannot set %q", key)
	}
	l.Set(key, value)
	return nil
}

// Remove deletes a key from local scope. See Local.Remove for why this exists.
func Remove(ctx context.Context, key string) {
	if l, ok := LocalOf(ctx); ok {
		l.Remove(key)
	}
}

// Keys returns every key visible from this context, sorted, for diagnostics. Local keys
// come first because they are the ones a failing step most likely meant.
func Keys(ctx context.Context) []string {
	var out []string
	if l, ok := LocalOf(ctx); ok {
		out = append(out, l.keys()...)
	}
	if s, ok := SharedOf(ctx); ok {
		for _, k := range s.keys() {
			// Shadowed shared keys are not listed twice; local already reported them.
			if !containsString(out, k) {
				out = append(out, k)
			}
		}
	}
	if len(out) == 0 {
		return []string{"(none)"}
	}
	return out
}

func containsString(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}
