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

package agentproto

import (
	"slices"
	"testing"
)

// TestV1_0OperationsAreTheCanonicalEleven spells the table out a second time,
// by hand, so a silent edit to the registry fails here rather than at a
// customer's gateway.
//
// This package is the control plane's restatement of the gateway's own
// agentproto table, and nothing links the two at compile time: an operation
// renamed on one side and not the other produces an Agent proxy that validates
// here and has no policy chain there. The names below are the contract; they
// are not derived from the registry, which is the point.
func TestV1_0OperationsAreTheCanonicalEleven(t *testing.T) {
	want := []Operation{
		"SendMessage",
		"SendStreamingMessage",
		"GetTask",
		"ListTasks",
		"CancelTask",
		"SubscribeToTask",
		"CreateTaskPushNotificationConfig",
		"GetTaskPushNotificationConfig",
		"ListTaskPushNotificationConfigs",
		"DeleteTaskPushNotificationConfig",
		"GetExtendedAgentCard",
	}

	got, ok := Operations(V1_0)
	if !ok {
		t.Fatalf("Operations(%q) reported the version as unregistered", V1_0)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("A2A 1.0 operations drifted.\n got: %v\nwant: %v", got, want)
	}
}

func TestVersionRegistry(t *testing.T) {
	if !IsSupportedVersion(V1_0) {
		t.Fatalf("%q must be a registered version", V1_0)
	}
	for _, unsupported := range []ProtocolVersion{"", "1", "1.1", "2.0", "v1.0", " 1.0"} {
		if IsSupportedVersion(unsupported) {
			t.Fatalf("%q must not be a registered version", unsupported)
		}
	}
	if got := Versions(); !slices.Equal(got, []ProtocolVersion{V1_0}) {
		t.Fatalf("Versions() = %v, want exactly [%q]", got, V1_0)
	}
}

// TestIsOperationIsExact pins case sensitivity: A2A operation names are
// case-sensitive, so a near-miss is a configuration error to report rather
// than a value to normalise into the name the author probably meant.
func TestIsOperationIsExact(t *testing.T) {
	if !IsOperation(V1_0, "SendMessage") {
		t.Fatalf("SendMessage must be an A2A 1.0 operation")
	}
	for _, name := range []string{"sendmessage", "SENDMESSAGE", "SendMessage ", "Send-Message", "", "Unknown"} {
		if IsOperation(V1_0, name) {
			t.Fatalf("%q must not be accepted as an A2A 1.0 operation", name)
		}
	}
	// An unregistered version has no operations at all, so nothing matches
	// rather than falling back to another version's set.
	if IsOperation("2.0", "SendMessage") {
		t.Fatalf("an unregistered version must define no operations")
	}
	if _, ok := Operations("2.0"); ok {
		t.Fatalf("Operations must report an unregistered version as unregistered")
	}
}

// TestOperationsReturnsACopy guards the registry against a caller that sorts or
// truncates the slice it was handed.
func TestOperationsReturnsACopy(t *testing.T) {
	first, _ := Operations(V1_0)
	first[0] = "Mutated"

	second, _ := Operations(V1_0)
	if second[0] == "Mutated" {
		t.Fatalf("Operations handed out the registry's own slice")
	}
}
