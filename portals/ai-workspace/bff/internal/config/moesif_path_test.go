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

package config

import "testing"

func TestMoesifPathMapperUnsetLeavesHopUntouched(t *testing.T) {
	for _, raw := range []string{"", "   ", "no-equals-sign", "=/id_token", "/analytics/id-token="} {
		if got := (ControlPlaneConfig{MoesifPathMappings: raw}).MoesifPathMapper(); got != nil {
			t.Errorf("MoesifPathMappings=%q produced a mapper; want nil so the hop forwards unchanged", raw)
		}
	}
}

func TestMoesifPathMapperRewritesConfiguredRoutes(t *testing.T) {
	// The moesif-key case: the SPA asks for /analytics/id-token, the upstream
	// publishes /id_token.
	mapPath := (ControlPlaneConfig{
		MoesifPathMappings: "/analytics/id-token=/id_token",
	}).MoesifPathMapper()
	if mapPath == nil {
		t.Fatal("expected a mapper")
	}

	for _, tc := range []struct{ in, want string }{
		{"/analytics/id-token", "/id_token"},
		{"/analytics/id-token/refresh", "/id_token/refresh"},
		// Whole-segment match only: a longer sibling route must not be caught.
		{"/analytics/id-token-v2", "/analytics/id-token-v2"},
		// Anything unmapped is forwarded as-is.
		{"/analytics/events", "/analytics/events"},
	} {
		if got := mapPath(tc.in); got != tc.want {
			t.Errorf("mapPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMoesifPathMapperNormalisesAndTakesMultiplePairs(t *testing.T) {
	mapPath := (ControlPlaneConfig{
		MoesifPathMappings: " analytics/id-token = /id_token/ , /analytics/keys=collector-keys ",
	}).MoesifPathMapper()
	if mapPath == nil {
		t.Fatal("expected a mapper")
	}
	if got := mapPath("/analytics/id-token"); got != "/id_token" {
		t.Errorf("slash/space normalisation failed: got %q", got)
	}
	if got := mapPath("/analytics/keys"); got != "/collector-keys" {
		t.Errorf("second pair not applied: got %q", got)
	}
}
