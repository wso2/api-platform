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

// Package agentproto is a version-keyed registry of canonical A2A operations
// and their HTTP+JSON bindings.
//
// It lives in common/ because both sides of the gateway need the same table and
// neither can import the other: the controller generates one policy chain per
// canonical operation and one route per binding, and the policy engine resolves
// an incoming request back to the same canonical operation so that a JSON-RPC
// call and its HTTP+JSON equivalent run the identical chain. A private copy on
// either side would drift, and the failure is silent — a mismatched name simply
// selects no chain, and a mismatched verb simply never matches a route.
//
// # Everything is keyed by protocol version
//
// An Agent selects exactly one A2A protocol version through
// spec.a2a.protocolVersion, and that choice decides its operation set, its
// bindings, its Agent Card model, and its signing rules. Two versions' tables
// are not interchangeable, so there is no version-free way to ask this package
// anything: every lookup takes a ProtocolVersion and reports whether that
// version is registered. There is deliberately no package-global operation
// count — eleven is a fact about 1.0, not about A2A.
//
// Each version's table is transcribed from the vendored protocol definition at
// gateway/gateway-controller/specification/a2a/v<version>/a2a.proto (see the
// SOURCE file beside it); that module's tests check this registry against the
// proto on every run.
//
// The JSON-RPC method name of an operation is its canonical name verbatim, so
// there is no map for it here. The method-name lookup itself belongs to the
// policy engine's resolver, which is the only component that reads a JSON-RPC
// envelope, and it is keyed by the same protocol version.
package agentproto

import "slices"

// ProtocolVersion is an A2A protocol version as it appears in an Agent's
// spec.a2a.protocolVersion.
type ProtocolVersion string

// V1_0 is A2A protocol version 1.0, the only version registered today.
const V1_0 ProtocolVersion = "1.0"

// Operation is a canonical A2A operation name: the binding-independent identity
// of an operation, shared by its JSON-RPC method and its HTTP+JSON route.
//
// Within a protocol version the set is closed. A request that does not map to
// one of that version's operations is not an A2A operation, which is what lets
// a chain miss be classified as a deployment error rather than a client error.
type Operation string

// Canonical A2A operation names.
//
// These are identifiers, not a set: which of them a given protocol version
// defines is answered by Operations. They are declared once because a name that
// survives across versions must mean the same operation in each.
const (
	SendMessage                      Operation = "SendMessage"
	SendStreamingMessage             Operation = "SendStreamingMessage"
	GetTask                          Operation = "GetTask"
	ListTasks                        Operation = "ListTasks"
	CancelTask                       Operation = "CancelTask"
	SubscribeToTask                  Operation = "SubscribeToTask"
	CreateTaskPushNotificationConfig Operation = "CreateTaskPushNotificationConfig"
	GetTaskPushNotificationConfig    Operation = "GetTaskPushNotificationConfig"
	ListTaskPushNotificationConfigs  Operation = "ListTaskPushNotificationConfigs"
	DeleteTaskPushNotificationConfig Operation = "DeleteTaskPushNotificationConfig"
	GetExtendedAgentCard             Operation = "GetExtendedAgentCard"
)

// HTTPBinding is one HTTP+JSON route for an operation: the method and the path
// template, relative to the transport's base path.
//
// Path parameters use the specification document's names ({id}, {configId})
// rather than the proto's ({task_id}, {id}). The names are local to the
// gateway's route templates; only the method and the path shape are
// interop-critical.
type HTTPBinding struct {
	// Method is the uppercase HTTP method.
	Method string
	// PathTemplate is the operation path relative to the transport base path.
	// It always starts with "/" and never ends with one.
	PathTemplate string
}

// versionTable is one protocol version's operation set and bindings.
type versionTable struct {
	// operations is in protocol-definition order, which is also the order the
	// specification's method-mapping table uses. Generated artifacts iterate
	// this rather than ranging over a map, so route and chain ordering is
	// deterministic across runs.
	operations []Operation
	// bindings maps each operation to its HTTP+JSON routes. The value is a
	// slice because one canonical operation may legitimately be reachable by
	// more than one route — an operation whose verb is disputed upstream can be
	// served under both verbs, still as one operation with one policy chain.
	bindings map[Operation][]HTTPBinding
}

// registry holds every supported protocol version. Adding a version means
// adding an entry here and a sibling vendored proto directory; it never means
// editing an existing entry.
var registry = map[ProtocolVersion]versionTable{
	V1_0: v1_0Table,
}

// Versions returns the registered protocol versions, sorted, so callers that
// report or validate against the supported set produce stable output.
func Versions() []ProtocolVersion {
	out := make([]ProtocolVersion, 0, len(registry))
	for v := range registry {
		out = append(out, v)
	}
	// Lexical order. Version strings are short and numeric-dotted, so this is
	// stable and readable for reporting; it is not semantic version ordering,
	// and nothing here should start depending on it as though it were.
	slices.Sort(out)
	return out
}

// IsSupportedVersion reports whether version has a registered table.
//
// Deployment-time validation calls this before anything else: an Agent naming
// an unsupported version must be rejected outright, not fall back to a default
// version, which would silently enforce a different operation set than the one
// its Agent Card advertises.
func IsSupportedVersion(version ProtocolVersion) bool {
	_, ok := registry[version]
	return ok
}

// Operations returns version's canonical operations in protocol order, and
// false if version is not registered.
//
// The returned slice is a copy: a caller sorting or filtering it in place must
// not be able to reorder every other caller's routes.
func Operations(version ProtocolVersion) ([]Operation, bool) {
	table, ok := registry[version]
	if !ok {
		return nil, false
	}
	out := make([]Operation, len(table.operations))
	copy(out, table.operations)
	return out, true
}

// IsOperation reports whether name is a canonical operation of version. The
// comparison is exact — A2A operation names are case-sensitive, and a
// near-match is a configuration error, not a value to normalise.
func IsOperation(version ProtocolVersion, name string) bool {
	table, ok := registry[version]
	if !ok {
		return false
	}
	_, ok = table.bindings[Operation(name)]
	return ok
}

// HTTPJSONBindings returns every HTTP+JSON route for op under version, and
// false if version is not registered or op is not one of its operations.
//
// The returned slice is a copy, for the same reason as Operations. Callers must
// generate a route per element and one chain per operation: multiple bindings
// are alternative ways to reach the same operation, not separate operations.
func HTTPJSONBindings(version ProtocolVersion, op Operation) ([]HTTPBinding, bool) {
	table, ok := registry[version]
	if !ok {
		return nil, false
	}
	bindings, ok := table.bindings[op]
	if !ok {
		return nil, false
	}
	out := make([]HTTPBinding, len(bindings))
	copy(out, bindings)
	return out, true
}
