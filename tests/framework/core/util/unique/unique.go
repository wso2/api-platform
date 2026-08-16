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

// Package unique generates collision-free resource identifiers.
//
// Hardcoded resource names are the single largest obstacle to running scenarios
// concurrently. Two scenarios that both create "TestAPI" collide the moment they overlap —
// and the failure is rarely legible: one scenario's create fails as a duplicate, or worse,
// one deletes the other's resource mid-flight and the victim fails somewhere unrelated.
//
// So names are unique BY CONSTRUCTION rather than by convention. A feature author never
// writes a literal name; they ask for one.
package unique

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// suffixKey holds the per-runner suffix in local scope.
const suffixKey = "__nameSuffix"

// Generator produces unique names within one runner.
//
// The suffix is per RUNNER, not per name: every resource a runner creates shares it, which
// makes a stray resource attributable to the runner that leaked it. Without that, a leftover
// named with a fresh random token tells you nothing about where it came from.
type Generator struct {
	suffix  string
	counter atomic.Uint64
}

// NewGenerator returns a generator with a fresh suffix.
func NewGenerator() (*Generator, error) {
	suffix, err := randomSuffix()
	if err != nil {
		return nil, err
	}
	return &Generator{suffix: suffix}, nil
}

// Suffix is the runner's shared token, exposed so a cleanup sweep can recognise its own
// resources by convention when a registry entry was somehow lost.
func (g *Generator) Suffix() string { return g.suffix }

// Unique returns a name derived from base.
//
// Shape is base_suffix_counter: the base keeps it readable in a failing test's output, the
// suffix attributes it to a runner, and the counter separates repeated calls within one
// runner — a scenario that creates three APIs needs three distinct names.
func (g *Generator) Unique(base string) string {
	n := g.counter.Add(1)
	return fmt.Sprintf("%s_%s_%d", sanitize(base), g.suffix, n)
}

// UniqueContext returns a name that is also a legal API context path.
//
// Separate from Unique because a context has different legality rules from a display name:
// it becomes part of a URL, so it must be lowercase and free of characters that would need
// escaping.
func (g *Generator) UniqueContext(base string) string {
	n := g.counter.Add(1)
	return fmt.Sprintf("/%s-%s-%d", strings.ToLower(sanitizeDashes(base)), strings.ToLower(g.suffix), n)
}

// randomSuffix returns a short, filename-and-URL-safe token.
//
// base32 without padding, lowercased: unlike base64 it has no characters that need escaping
// in a URL, a TOML value or a shell, so one suffix is safe everywhere a name might be used.
func randomSuffix() (string, error) {
	raw := make([]byte, 5)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("names: generating a suffix: %w", err)
	}
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)), nil
}

// sanitize keeps a base readable while removing anything a product might reject.
func sanitize(base string) string {
	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "res"
	}
	return out
}

// sanitizeDashes is sanitize for path segments, where a dash reads better than underscore.
func sanitizeDashes(base string) string {
	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "res"
	}
	return out
}

// ── Context integration ──────────────────────────────────────────────────────────

var installMu sync.Mutex

// Install attaches a generator to a runner's local scope. Called once per runner.
func Install(ctx context.Context, g *Generator) error {
	return tcontext.Set(ctx, suffixKey, g)
}

// Of returns the runner's generator, creating one lazily if none was installed.
//
// Lazy creation means a step never fails merely because a generator was not wired up — but
// it is guarded, because two concurrent scenarios in one runner racing to create the first
// generator would otherwise produce two suffixes and lose the attribution property.
func Of(ctx context.Context) (*Generator, error) {
	if v, ok := tcontext.Get(ctx, suffixKey); ok {
		if g, ok := v.(*Generator); ok {
			return g, nil
		}
		return nil, fmt.Errorf("names: context key %q holds %T, not a *Generator", suffixKey, v)
	}

	installMu.Lock()
	defer installMu.Unlock()

	// Re-check under the lock: another scenario may have installed one while we waited.
	if v, ok := tcontext.Get(ctx, suffixKey); ok {
		if g, ok := v.(*Generator); ok {
			return g, nil
		}
	}

	g, err := NewGenerator()
	if err != nil {
		return nil, err
	}
	if err := Install(ctx, g); err != nil {
		return nil, err
	}
	return g, nil
}

// Unique returns a unique name using the runner's generator.
func Unique(ctx context.Context, base string) (string, error) {
	g, err := Of(ctx)
	if err != nil {
		return "", err
	}
	return g.Unique(base), nil
}

// UniqueContext returns a unique API context path using the runner's generator.
func UniqueContext(ctx context.Context, base string) (string, error) {
	g, err := Of(ctx)
	if err != nil {
		return "", err
	}
	return g.UniqueContext(base), nil
}

// Placeholder is the marker a feature file uses to request a generated name.
//
// Written as ${UNIQUE:base} in Gherkin, so a feature never contains a literal resource name
// and cannot reintroduce the collision this package exists to prevent.
const Placeholder = "${UNIQUE:"

// ContextPlaceholder is the marker a feature file uses to reference a value the scenario stored.
//
// Written as ${CTX:name} in Gherkin. Resolution is delegated to ContextValue, because which
// names are addressable is a property of the suite, not of this package.
const ContextPlaceholder = "${CTX:"

// ContextValue resolves a ${CTX:name} placeholder. A suite sets this once at start-up; nil means
// the suite exposes nothing, and any ${CTX:} reference is then an error rather than an empty
// substitution.
var ContextValue func(ctx context.Context, name string) (string, error)

func expandContext(ctx context.Context, s string) (string, error) {
	var b strings.Builder
	rest := s
	for {
		start := strings.Index(rest, ContextPlaceholder)
		if start < 0 {
			b.WriteString(rest)
			return b.String(), nil
		}
		b.WriteString(rest[:start])

		after := rest[start+len(ContextPlaceholder):]
		end := strings.Index(after, "}")
		if end < 0 {
			return "", fmt.Errorf("names: unterminated %s placeholder in %q", ContextPlaceholder, s)
		}
		name := after[:end]
		if err := validBase(name); err != nil {
			return "", fmt.Errorf("names: in %q: %w", s, err)
		}
		if ContextValue == nil {
			return "", fmt.Errorf("names: %s%s} used but the suite exposes no context values",
				ContextPlaceholder, name)
		}
		// An empty value must fail here. Substituting "" produces a request that looks well
		// formed and fails much later as an auth rejection, naming neither the placeholder nor
		// the step that was meant to set it.
		value, err := ContextValue(ctx, name)
		if err != nil {
			return "", fmt.Errorf("names: resolving %s%s}: %w", ContextPlaceholder, name, err)
		}
		b.WriteString(value)
		rest = after[end+1:]
	}
}

// Expand replaces every ${UNIQUE:base} in a string with a generated name.
//
// Repeated occurrences of the SAME base within one string resolve to the same name, so a
// step can reference one resource twice in a payload. Different bases resolve differently.
func Expand(ctx context.Context, s string) (string, error) {
	if strings.Contains(s, ContextPlaceholder) {
		expanded, err := expandContext(ctx, s)
		if err != nil {
			return "", err
		}
		s = expanded
	}
	if !strings.Contains(s, Placeholder) {
		return s, nil
	}

	g, err := Of(ctx)
	if err != nil {
		return "", err
	}

	resolved := map[string]string{}
	var b strings.Builder
	rest := s

	for {
		start := strings.Index(rest, Placeholder)
		if start < 0 {
			b.WriteString(rest)
			break
		}
		b.WriteString(rest[:start])

		after := rest[start+len(Placeholder):]
		end := strings.Index(after, "}")
		if end < 0 {
			// Unterminated: an error rather than a silent literal, because a name that
			// looks generated but is not would collide exactly as a hardcoded one does.
			return "", fmt.Errorf("names: unterminated %s placeholder in %q", Placeholder, s)
		}

		base := after[:end]
		if err := validBase(base); err != nil {
			// Without this, a mistyped placeholder like ${UNIQUE:api" inside JSON silently
			// takes everything up to the JSON's own closing brace as the base and produces
			// a plausible-looking name. Rejecting a suspicious base turns that into an
			// error naming the placeholder.
			return "", fmt.Errorf("names: in %q: %w", s, err)
		}

		name, seen := resolved[base]
		if !seen {
			name = g.Unique(base)
			resolved[base] = name
		}
		b.WriteString(name)
		rest = after[end+1:]
	}

	return b.String(), nil
}

// validBase rejects a placeholder base that looks like a typo rather than a name.
//
// Deliberately strict: a base is a short identifier, so anything containing quotes, braces,
// colons or whitespace means the placeholder was almost certainly not closed where the
// author intended.
func validBase(base string) error {
	if base == "" {
		return fmt.Errorf("%s has an empty base", Placeholder)
	}
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.':
		default:
			return fmt.Errorf("%s%s} has an invalid base: %q is not allowed in a name base "+
				"(the placeholder is probably unterminated)", Placeholder, base, r)
		}
	}
	return nil
}
