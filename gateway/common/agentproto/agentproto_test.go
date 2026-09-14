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

package agentproto

import (
	"strings"
	"testing"
	"unicode"
)

// unsupportedVersion is a well-formed version string that is deliberately not
// registered, for asserting that lookups reject rather than fall back.
const unsupportedVersion ProtocolVersion = "9.9"

// TestRegisteredVersions pins the supported set. Registering a version is a
// deliberate act — it needs a vendored proto directory, a binding table, and a
// decision about card validation and signing — so one appearing without those
// should fail here first.
func TestRegisteredVersions(t *testing.T) {
	versions := Versions()
	if len(versions) != 1 || versions[0] != V1_0 {
		t.Errorf("Versions() = %v, want exactly [%s]", versions, V1_0)
	}
	if !IsSupportedVersion(V1_0) {
		t.Errorf("IsSupportedVersion(%s) = false", V1_0)
	}
	if IsSupportedVersion(unsupportedVersion) {
		t.Errorf("IsSupportedVersion(%s) = true for an unregistered version", unsupportedVersion)
	}
	if IsSupportedVersion("") {
		t.Error(`IsSupportedVersion("") = true; an Agent that names no version must be rejected, not defaulted`)
	}
}

// TestUnsupportedVersionIsRejected asserts every lookup refuses an unregistered
// version rather than falling back to a default. A fallback would enforce a
// different operation set than the Agent Card advertises, which is silent.
func TestUnsupportedVersionIsRejected(t *testing.T) {
	for _, version := range []ProtocolVersion{unsupportedVersion, "", "1", "v1.0", "1.0.0"} {
		if ops, ok := Operations(version); ok || ops != nil {
			t.Errorf("Operations(%q) = (%v, %v), want (nil, false)", version, ops, ok)
		}
		if bindings, ok := HTTPJSONBindings(version, SendMessage); ok || bindings != nil {
			t.Errorf("HTTPJSONBindings(%q, SendMessage) = (%v, %v), want (nil, false)", version, bindings, ok)
		}
		if IsOperation(version, string(SendMessage)) {
			t.Errorf("IsOperation(%q, SendMessage) = true for an unregistered version", version)
		}
	}
}

// TestV1_0OperationSetIsClosed guards the size and consistency of the 1.0 set.
// A2A 1.0 defines exactly eleven operations, and the controller builds one
// policy chain per operation per routing partition, so a twelfth arriving
// unnoticed would produce an operation with no chain — reachable, but running
// nobody's policies.
//
// Eleven is asserted here, against version 1.0, and nowhere as a package-level
// constant: it is a fact about this version, not about A2A.
func TestV1_0OperationSetIsClosed(t *testing.T) {
	const wantCount = 11

	ops, ok := Operations(V1_0)
	if !ok {
		t.Fatalf("Operations(%s) reported the version as unregistered", V1_0)
	}
	if len(ops) != wantCount {
		t.Errorf("version %s has %d operations, want %d", V1_0, len(ops), wantCount)
	}

	seen := make(map[Operation]bool, len(ops))
	for _, op := range ops {
		if seen[op] {
			t.Errorf("operation %q listed twice in version %s", op, V1_0)
		}
		seen[op] = true
		if _, ok := HTTPJSONBindings(V1_0, op); !ok {
			t.Errorf("operation %q has no HTTP+JSON binding in version %s", op, V1_0)
		}
	}
	for op := range v1_0Bindings {
		if !seen[op] {
			t.Errorf("operation %q has a binding but is missing from version %s's operation list", op, V1_0)
		}
	}
}

// TestLookupsReturnCopies makes sure a caller cannot reorder, blank out, or
// re-point every other caller's view of the registry.
func TestLookupsReturnCopies(t *testing.T) {
	ops, _ := Operations(V1_0)
	ops[0] = "mutated"
	if again, _ := Operations(V1_0); again[0] != SendMessage {
		t.Errorf("mutating the result of Operations() changed the registry: first operation is now %q", again[0])
	}

	bindings, _ := HTTPJSONBindings(V1_0, SendMessage)
	bindings[0] = HTTPBinding{Method: "TRACE", PathTemplate: "/mutated"}
	again, _ := HTTPJSONBindings(V1_0, SendMessage)
	if again[0].Method != "POST" || again[0].PathTemplate != "/message:send" {
		t.Errorf("mutating the result of HTTPJSONBindings() changed the registry: binding is now %s %s", again[0].Method, again[0].PathTemplate)
	}
}

// TestOperationNamesAreValidChainKeyComponents checks the names can be used as
// a chain-key component. Chain keys join apiID, vhost, and operation with the
// ASCII unit separator (0x1f), so an operation name containing one — or any
// other control character — would silently split into a different key.
func TestOperationNamesAreValidChainKeyComponents(t *testing.T) {
	const chainKeySeparator = '\x1f'

	for _, version := range Versions() {
		ops, _ := Operations(version)
		for _, op := range ops {
			name := string(op)
			if name == "" {
				t.Errorf("version %s: empty operation name", version)
				continue
			}
			if strings.ContainsRune(name, chainKeySeparator) {
				t.Errorf("version %s: operation %q contains the chain-key separator (0x1f)", version, name)
			}
			for _, r := range name {
				if unicode.IsControl(r) {
					t.Errorf("version %s: operation %q contains control character %U", version, name, r)
				}
				if r > unicode.MaxASCII {
					t.Errorf("version %s: operation %q contains non-ASCII character %U", version, name, r)
				}
			}
		}
	}
}

// TestV1_0HTTPJSONBindings pins every verb and path template against the A2A
// specification's method-mapping reference (section 5.3). A wrong verb or path
// does not fail loudly at runtime: the route simply never matches, the
// operation 404s, and no policy chain ever runs. That makes this table the one
// place worth restating literally rather than deriving.
func TestV1_0HTTPJSONBindings(t *testing.T) {
	want := map[Operation][]HTTPBinding{
		SendMessage:                      {{Method: "POST", PathTemplate: "/message:send"}},
		SendStreamingMessage:             {{Method: "POST", PathTemplate: "/message:stream"}},
		GetTask:                          {{Method: "GET", PathTemplate: "/tasks/{id}"}},
		ListTasks:                        {{Method: "GET", PathTemplate: "/tasks"}},
		CancelTask:                       {{Method: "POST", PathTemplate: "/tasks/{id}:cancel"}},
		SubscribeToTask:                  {{Method: "POST", PathTemplate: "/tasks/{id}:subscribe"}},
		CreateTaskPushNotificationConfig: {{Method: "POST", PathTemplate: "/tasks/{id}/pushNotificationConfigs"}},
		GetTaskPushNotificationConfig:    {{Method: "GET", PathTemplate: "/tasks/{id}/pushNotificationConfigs/{configId}"}},
		ListTaskPushNotificationConfigs:  {{Method: "GET", PathTemplate: "/tasks/{id}/pushNotificationConfigs"}},
		DeleteTaskPushNotificationConfig: {{Method: "DELETE", PathTemplate: "/tasks/{id}/pushNotificationConfigs/{configId}"}},
		GetExtendedAgentCard:             {{Method: "GET", PathTemplate: "/extendedAgentCard"}},
	}

	for op, expected := range want {
		got, ok := HTTPJSONBindings(V1_0, op)
		if !ok {
			t.Errorf("%s: no binding in version %s", op, V1_0)
			continue
		}
		if len(got) != len(expected) {
			t.Errorf("%s: %d bindings, want %d", op, len(got), len(expected))
			continue
		}
		for i := range expected {
			if got[i] != expected[i] {
				t.Errorf("%s: binding %d is %s %s, want %s %s", op, i,
					got[i].Method, got[i].PathTemplate, expected[i].Method, expected[i].PathTemplate)
			}
		}
	}
}

// TestBindingShapes checks the invariants route generation relies on, for every
// registered version: methods are uppercase (a lowercase verb produces an Envoy
// Exact matcher that never fires) and path templates are rooted with no
// trailing slash, so joining them onto a transport base path cannot produce
// "//" or a trailing separator.
func TestBindingShapes(t *testing.T) {
	validMethods := map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}

	for _, version := range Versions() {
		ops, _ := Operations(version)
		for _, op := range ops {
			bindings, ok := HTTPJSONBindings(version, op)
			if !ok || len(bindings) == 0 {
				t.Errorf("version %s: operation %s has no bindings", version, op)
				continue
			}
			for _, b := range bindings {
				if !validMethods[b.Method] {
					t.Errorf("version %s, %s: method %q is not an expected uppercase HTTP method", version, op, b.Method)
				}
				if b.Method != strings.ToUpper(b.Method) {
					t.Errorf("version %s, %s: method %q is not uppercase", version, op, b.Method)
				}
				if !strings.HasPrefix(b.PathTemplate, "/") {
					t.Errorf("version %s, %s: path template %q does not start with /", version, op, b.PathTemplate)
				}
				if len(b.PathTemplate) > 1 && strings.HasSuffix(b.PathTemplate, "/") {
					t.Errorf("version %s, %s: path template %q ends with /", version, op, b.PathTemplate)
				}
				if strings.Contains(b.PathTemplate, "//") {
					t.Errorf("version %s, %s: path template %q contains an empty segment", version, op, b.PathTemplate)
				}
			}
		}
	}
}

// TestNoDuplicateMethodAndPathWithinAVersion asserts no two operations in a
// version share a method and path template. Two that did would be
// indistinguishable at the route layer, so one of them would silently inherit
// the other's policy chain. Duplicates *within* one operation are equally
// pointless — two identical routes for the same operation.
func TestNoDuplicateMethodAndPathWithinAVersion(t *testing.T) {
	for _, version := range Versions() {
		ops, _ := Operations(version)
		seen := make(map[HTTPBinding]Operation, len(ops))
		for _, op := range ops {
			bindings, _ := HTTPJSONBindings(version, op)
			for _, b := range bindings {
				if prev, dup := seen[b]; dup {
					if prev == op {
						t.Errorf("version %s: %s lists the binding %s %s twice", version, op, b.Method, b.PathTemplate)
					} else {
						t.Errorf("version %s: %s and %s share the binding %s %s", version, prev, op, b.Method, b.PathTemplate)
					}
				}
				seen[b] = op
			}
		}
	}
}

func TestIsOperation(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"SendMessage", true},
		{"GetExtendedAgentCard", true},
		// Case-sensitive: A2A operation names are exact, and a near-match is a
		// configuration error rather than something to normalise.
		{"sendmessage", false},
		{"sendMessage", false},
		{" SendMessage", false},
		{"SendMessage ", false},
		{"", false},
		{"tasks/get", false},
		// A JSON-RPC method from an earlier A2A draft: rejected, not aliased.
		{"message/send", false},
	}
	for _, tt := range tests {
		if got := IsOperation(V1_0, tt.name); got != tt.want {
			t.Errorf("IsOperation(%s, %q) = %v, want %v", V1_0, tt.name, got, tt.want)
		}
	}
}
