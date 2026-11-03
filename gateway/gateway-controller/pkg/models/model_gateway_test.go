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

package models

import (
	"testing"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

func TestModelEndpointUUID(t *testing.T) {
	// Derived, so every replica agrees on it without coordinating — and fixed: the
	// UUID is the endpoint's API id on the data plane, which its keys are fed under.
	if got := ModelEndpointUUID("wso2"); got != ModelEndpointUUID("wso2") {
		t.Fatalf("not deterministic: %s", got)
	}
	if ModelEndpointUUID("wso2") == ModelEndpointUUID("partners") {
		t.Fatal("two handles derived one UUID")
	}
	if got, want := ModelEndpointUUID("wso2"), "0546d6c2-957b-5997-99f2-6da2752a3933"; got != want {
		t.Fatalf("ModelEndpointUUID(wso2) = %s, want %s: the derivation must never change", got, want)
	}
}

func TestModelEndpointHelpers(t *testing.T) {
	spec := api.ModelConfigData{Endpoints: []string{"wso2", " partners ", "wso2", ""}}
	got := ModelEndpointHandles(spec)
	if len(got) != 2 || got[0] != "wso2" || got[1] != "partners" {
		t.Fatalf("ModelEndpointHandles = %v", got)
	}
	if !ModelNamesEndpoint(spec, "partners") || ModelNamesEndpoint(spec, "other") {
		t.Fatal("ModelNamesEndpoint")
	}

	if EffectiveInboundTemplate(api.ModelEndpointConfigData{}) != "openai" {
		t.Fatal("the default inbound template is openai")
	}
	tmpl := "anthropic"
	if EffectiveInboundTemplate(api.ModelEndpointConfigData{InboundTemplate: &tmpl}) != "anthropic" {
		t.Fatal("EffectiveInboundTemplate")
	}
	if ModelEndpointProviderName("wso2") == ModelEndpointProviderName("partners") {
		t.Fatal("each endpoint has its own provider name")
	}
	if IsControlPlaneSyncedKind(KindModel) || IsControlPlaneSyncedKind(KindModelEndpoint) || !IsControlPlaneSyncedKind(KindLlmProxy) {
		t.Fatal("IsControlPlaneSyncedKind")
	}
}
