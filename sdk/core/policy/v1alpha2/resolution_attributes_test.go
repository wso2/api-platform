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

package policyv1alpha2

import (
	"sort"
	"testing"
)

// The zero value is what a route resolved by the route itself carries, which is
// every API kind shipping today. It has to answer correctly without any caller
// testing for it first — that is what removes the nil check from every consumer.
func TestResolutionAttributes_ZeroValueIsTheEmptySet(t *testing.T) {
	var attrs ResolutionAttributes

	if got := attrs.Get("a2a.context.id"); got != "" {
		t.Errorf("Get on the zero value = %q, want %q", got, "")
	}
	if value, ok := attrs.Lookup("a2a.context.id"); ok || value != "" {
		t.Errorf("Lookup on the zero value = (%q, %v), want (\"\", false)", value, ok)
	}
	if got := attrs.Len(); got != 0 {
		t.Errorf("Len on the zero value = %d, want 0", got)
	}
	attrs.Iterate(func(name, value string) {
		t.Errorf("Iterate on the zero value visited %q = %q", name, value)
	})
}

// A resolver that produced nothing hands over a nil map. That must behave as the
// zero value rather than as a distinct third state.
func TestNewResolutionAttributes_NilMapBehavesAsTheZeroValue(t *testing.T) {
	attrs := NewResolutionAttributes(nil)

	if got := attrs.Len(); got != 0 {
		t.Errorf("Len = %d, want 0", got)
	}
	if got := attrs.Get("anything"); got != "" {
		t.Errorf("Get = %q, want %q", got, "")
	}
	if _, ok := attrs.Lookup("anything"); ok {
		t.Error("Lookup reported a value on an empty set")
	}
}

// Identifier-shaped attributes are absent, not empty, when a request did not carry
// them — so a consumer that has to tell those apart needs Lookup, and Get alone
// cannot serve it.
func TestResolutionAttributes_LookupDistinguishesAbsentFromEmpty(t *testing.T) {
	attrs := NewResolutionAttributes(map[string]string{
		"a2a.context.id": "",
		"a2a.task.id":    "task-1",
	})

	if value, ok := attrs.Lookup("a2a.context.id"); !ok || value != "" {
		t.Errorf("Lookup of a recorded empty value = (%q, %v), want (\"\", true)", value, ok)
	}
	if value, ok := attrs.Lookup("a2a.message.id"); ok || value != "" {
		t.Errorf("Lookup of an unrecorded name = (%q, %v), want (\"\", false)", value, ok)
	}

	// Get collapses the two, which is why it is documented as doing so.
	if attrs.Get("a2a.context.id") != attrs.Get("a2a.message.id") {
		t.Error("Get is expected to report a recorded empty value and an absent one alike")
	}
}

func TestResolutionAttributes_GetAndLen(t *testing.T) {
	attrs := NewResolutionAttributes(map[string]string{
		"a2a.operation":        "SendMessage",
		"a2a.transport":        "JSONRPC",
		"a2a.protocol.version": "1.0",
	})

	if got := attrs.Len(); got != 3 {
		t.Errorf("Len = %d, want 3", got)
	}
	for name, want := range map[string]string{
		"a2a.operation":        "SendMessage",
		"a2a.transport":        "JSONRPC",
		"a2a.protocol.version": "1.0",
	} {
		if got := attrs.Get(name); got != want {
			t.Errorf("Get(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestResolutionAttributes_IterateVisitsEveryAttribute(t *testing.T) {
	source := map[string]string{
		"a2a.operation":  "SendMessage",
		"a2a.transport":  "JSONRPC",
		"a2a.context.id": "ctx-1",
	}

	seen := map[string]string{}
	NewResolutionAttributes(source).Iterate(func(name, value string) {
		seen[name] = value
	})

	if len(seen) != len(source) {
		t.Fatalf("Iterate visited %d attributes, want %d", len(seen), len(source))
	}
	names := make([]string, 0, len(seen))
	for name, value := range seen {
		names = append(names, name)
		if source[name] != value {
			t.Errorf("Iterate reported %q = %q, want %q", name, value, source[name])
		}
	}
	sort.Strings(names)
	t.Logf("visited: %v", names)
}

// Iterate is handed to callers that may pass a nil callback through a helper; it
// must not panic on one.
func TestResolutionAttributes_IterateToleratesANilCallback(t *testing.T) {
	NewResolutionAttributes(map[string]string{"a2a.transport": "JSONRPC"}).Iterate(nil)
}

// SharedContext exposes the type, not a map, so a policy holding a request's
// context has no route to the kernel's data. The compiler enforces that; this
// asserts the reachable half of the contract — that reading through the context
// works and reports what the kernel put there.
func TestSharedContext_ResolutionAttributesAreReadableAndEmptyByDefault(t *testing.T) {
	var unresolved SharedContext
	if got := unresolved.ResolutionAttributes.Len(); got != 0 {
		t.Errorf("a directly-resolved route carries %d attributes, want 0", got)
	}
	if got := unresolved.ResolvedOperation; got != "" {
		t.Errorf("a directly-resolved route names operation %q, want %q", got, "")
	}

	resolved := SharedContext{
		APIKind:           APIKindAgent,
		ResolvedOperation: "SendMessage",
		ResolutionAttributes: NewResolutionAttributes(map[string]string{
			"a2a.transport":  "JSONRPC",
			"a2a.context.id": "ctx-1",
		}),
	}
	if got := resolved.ResolutionAttributes.Get("a2a.context.id"); got != "ctx-1" {
		t.Errorf("Get(a2a.context.id) = %q, want %q", got, "ctx-1")
	}
	if got := resolved.ResolutionAttributes.Len(); got != 2 {
		t.Errorf("Len = %d, want 2", got)
	}
}
