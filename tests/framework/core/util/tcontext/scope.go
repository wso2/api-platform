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

// contextKey prevents collisions with application context keys.
type contextKey struct{ name string }

var (
	sharedKey = contextKey{"shared"}
	localKey  = contextKey{"local"}
)

// Shared is block-scoped state populated during boot and read by its runners.
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

// Local is runner-scoped state shared by scenarios in that runner.
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

// List returns a copy of a named list.
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

// Resolve, Get, and Contains provide required-value, nullable-value, and presence-only
// lookups. All lookups search local scope before shared scope.

// Resolve returns a required value or an error. Optional angle brackets around the key
// are accepted.
func Resolve(ctx context.Context, key string) (any, error) {
	lookup := strings.TrimSuffix(strings.TrimPrefix(key, "<"), ">")

	if v, ok := Get(ctx, lookup); ok {
		return v, nil
	}
	return nil, fmt.Errorf("no value in context for key %q (available: %s)",
		lookup, strings.Join(Keys(ctx), ", "))
}

// ResolveString resolves a required string value.
func ResolveString(ctx context.Context, key string) (string, error) {
	v, err := Resolve(ctx, key)
	if err != nil {
		return "", err
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("context key %q holds %T, not a string", key, v)
	}
	return s, nil
}

// Get returns a value and whether it is present.
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

// Set stores a value in local scope.
func Set(ctx context.Context, key string, value any) error {
	l, ok := LocalOf(ctx)
	if !ok {
		return fmt.Errorf("no local scope in context: cannot set %q", key)
	}
	l.Set(key, value)
	return nil
}

// Remove deletes a key from local scope.
func Remove(ctx context.Context, key string) {
	if l, ok := LocalOf(ctx); ok {
		l.Remove(key)
	}
}

// Keys returns visible keys in sorted order for diagnostics.
func Keys(ctx context.Context) []string {
	var out []string
	if l, ok := LocalOf(ctx); ok {
		out = append(out, l.keys()...)
	}
	if s, ok := SharedOf(ctx); ok {
		for _, k := range s.keys() {
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
