/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package handler

import (
	"encoding/json"
	"testing"
)

// manifestResponseJSON returns the GetGatewayManifest body for a stored manifest.
func manifestResponseJSON(t *testing.T, stored string) map[string]any {
	t.Helper()
	var raw json.RawMessage
	if stored != "" {
		raw = json.RawMessage(stored)
	}
	body, err := json.Marshal(manifestSyncResponse{Policies: manifestPoliciesForResponse(raw)})
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal response %s: %v", body, err)
	}
	return decoded
}

func TestManifestResponseFlagsOrganizationPoliciesAsCustom(t *testing.T) {
	stored := `[
		{"name":"hmac-signer","version":"v1.0.0","displayName":"HMAC Signer","managedBy":"organization",
		 "policyDefinition":{"parameters":{"type":"object"}}},
		{"name":"cors","version":"v1.2.0","managedBy":"wso2"},
		{"name":"legacy-signer","version":"v1.0.0","managedBy":"customer"}
	]`

	policies, ok := manifestResponseJSON(t, stored)["policies"].([]any)
	if !ok || len(policies) != 3 {
		t.Fatalf("policies = %#v, want three entries", policies)
	}

	custom := policies[0].(map[string]any)
	if custom["isCustomPolicy"] != true {
		t.Errorf("organization policy isCustomPolicy = %v, want true", custom["isCustomPolicy"])
	}
	if custom["managedBy"] != "organization" || custom["displayName"] != "HMAC Signer" {
		t.Errorf("organization policy lost managedBy or displayName: %#v", custom)
	}
	if _, ok := custom["policyDefinition"].(map[string]any); !ok {
		t.Errorf("organization policy lost its policyDefinition: %#v", custom)
	}

	builtIn := policies[1].(map[string]any)
	if builtIn["isCustomPolicy"] != false {
		t.Errorf("wso2 policy isCustomPolicy = %v, want false", builtIn["isCustomPolicy"])
	}

	legacy := policies[2].(map[string]any)
	if legacy["isCustomPolicy"] != true {
		t.Errorf("customer policy isCustomPolicy = %v, want true", legacy["isCustomPolicy"])
	}
}

func TestManifestResponseOmitsPoliciesWhenNoneStored(t *testing.T) {
	if _, present := manifestResponseJSON(t, "")["policies"]; present {
		t.Error("policies present for a gateway that never reported a manifest")
	}
}

func TestManifestResponseReturnsUndecodableManifestAsStored(t *testing.T) {
	got := manifestResponseJSON(t, `{"unexpected":true}`)["policies"]
	if object, ok := got.(map[string]any); !ok || object["unexpected"] != true {
		t.Errorf("policies = %#v, want the stored document unchanged", got)
	}
}
