/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
 *
 */

// Package agentproto holds the A2A protocol facts the control plane validates
// against: which wire versions exist, and which canonical operations each one
// defines.
//
// It is the control plane's own copy of the gateway's `agentproto` contract
// (gateway/common/agentproto), deliberately narrowed to the version/operation
// registry — the HTTP+JSON bindings and the chain resolver are data-plane
// concerns and are not repeated here. platform-api does not depend on the
// gateway modules, so the table is restated rather than imported.
//
// That makes it a table with no compile-time link to the gateway's: a protocol
// version registered on one side and not the other, or an operation name that
// differs between them, will not fail a build. Adding or changing a version
// means changing both, and the names must match exactly — the control plane
// stores what the gateway will later match its route chains against, so a
// near-miss here is an Agent proxy that validates at authoring time and has no
// chain at runtime.
package agentproto

import "slices"

// ProtocolVersion is an A2A wire protocol version, as it appears in an Agent
// proxy's a2a.protocolVersion.
type ProtocolVersion string

// V1_0 is A2A protocol version 1.0, the only version registered today.
const V1_0 ProtocolVersion = "1.0"

// Operation is a canonical A2A operation name: the binding-independent identity
// of an operation, shared by its JSON-RPC method and its HTTP+JSON route.
//
// Within a protocol version the set is closed, which is what lets the control
// plane reject an unknown name outright instead of storing an Agent proxy whose
// per-operation configuration could never attach to anything.
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

// v1_0Operations is version 1.0's operation set, in protocol-definition order.
// Order is preserved because it is the order the specification's method-mapping
// table uses, and reporting the accepted set in a stable order keeps validation
// messages deterministic.
var v1_0Operations = []Operation{
	SendMessage,
	SendStreamingMessage,
	GetTask,
	ListTasks,
	CancelTask,
	SubscribeToTask,
	CreateTaskPushNotificationConfig,
	GetTaskPushNotificationConfig,
	ListTaskPushNotificationConfigs,
	DeleteTaskPushNotificationConfig,
	GetExtendedAgentCard,
}

// registry holds every supported protocol version. Adding a version means
// adding an entry here; it never means editing an existing one, because an
// Agent proxy already stored against that version keeps its operation set.
var registry = map[ProtocolVersion][]Operation{
	V1_0: v1_0Operations,
}

// Versions returns the registered protocol versions, sorted, so a caller that
// reports the supported set produces stable output.
//
// The order is lexical. Version strings are short and numeric-dotted, so this
// is readable for reporting; it is not semantic version ordering and nothing
// should start treating it as such.
func Versions() []ProtocolVersion {
	out := make([]ProtocolVersion, 0, len(registry))
	for v := range registry {
		out = append(out, v)
	}
	slices.Sort(out)
	return out
}

// IsSupportedVersion reports whether version has a registered operation table.
//
// An Agent proxy naming an unregistered version is rejected at authoring time
// rather than defaulted to a registered one: defaulting would silently enforce
// a different operation set than the one the author wrote against, and the
// artifact could never deploy anyway.
func IsSupportedVersion(version ProtocolVersion) bool {
	_, ok := registry[version]
	return ok
}

// Operations returns version's canonical operations in protocol order, and
// false if version is not registered.
//
// The returned slice is a copy: a caller sorting or filtering it in place must
// not be able to reorder it for every other caller.
func Operations(version ProtocolVersion) ([]Operation, bool) {
	ops, ok := registry[version]
	if !ok {
		return nil, false
	}
	return slices.Clone(ops), true
}

// IsOperation reports whether name is a canonical operation of version. The
// comparison is exact — A2A operation names are case-sensitive, and a near-miss
// is a configuration error rather than a value to normalise.
func IsOperation(version ProtocolVersion, name string) bool {
	ops, ok := registry[version]
	if !ok {
		return false
	}
	return slices.Contains(ops, Operation(name))
}
