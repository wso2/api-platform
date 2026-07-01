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

import "testing"

func TestComputeDataVersion(t *testing.T) {
	tests := []struct {
		name       string
		kind       ArtifactKind
		apiVersion string
		want       string
	}{
		{"v1 rest api", KindRestApi, "gateway.api-platform.wso2.com/v1", "1.0"},
		{"v1 llm proxy", KindLlmProxy, "gateway.api-platform.wso2.com/v1", "1.0"},
		{"v1alpha2 strips qualifier", KindRestApi, "gateway.api-platform.wso2.com/v1alpha2", "1.0"},
		{"v2 major", KindRestApi, "gateway.api-platform.wso2.com/v2", "2.0"},
		{"v10 multi-digit major", KindRestApi, "gateway.api-platform.wso2.com/v10", "10.0"},
		{"empty apiVersion falls back", KindRestApi, "", "1.0"},
		{"unparseable falls back", KindRestApi, "not-a-version", "1.0"},
		{"unknown kind defaults minor 0", ArtifactKind("Bogus"), "gateway.api-platform.wso2.com/v3", "3.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ComputeDataVersion(tt.kind, tt.apiVersion); got != tt.want {
				t.Errorf("ComputeDataVersion(%q, %q) = %q, want %q", tt.kind, tt.apiVersion, got, tt.want)
			}
		})
	}
}

func TestComputeDataVersionRespectsMinorBump(t *testing.T) {
	// Simulate a backward-compatible data-shape bump for one kind and confirm the
	// computed data_version reflects it, while other kinds stay at .0.
	orig := dataMinorVersions[KindRestApi]
	defer func() { dataMinorVersions[KindRestApi] = orig }()

	dataMinorVersions[KindRestApi] = 1
	if got := ComputeDataVersion(KindRestApi, "gateway.api-platform.wso2.com/v1"); got != "1.1" {
		t.Errorf("after minor bump, got %q, want %q", got, "1.1")
	}
	if got := ComputeDataVersion(KindWebSubApi, "gateway.api-platform.wso2.com/v1"); got != "1.0" {
		t.Errorf("unbumped kind got %q, want %q", got, "1.0")
	}
}

// TestDataMinorVersionsExhaustive guards the §7.3 decision: every ArtifactKind
// must have an explicit entry in dataMinorVersions, so a newly added kind can't
// silently fall back to minor 0.
func TestDataMinorVersionsExhaustive(t *testing.T) {
	allKinds := []ArtifactKind{
		KindRestApi,
		KindWebSubApi,
		KindWebBrokerApi,
		KindMcp,
		KindLlmProxy,
		KindLlmProvider,
	}
	for _, k := range allKinds {
		if _, ok := dataMinorVersions[k]; !ok {
			t.Errorf("ArtifactKind %q has no entry in dataMinorVersions; register a minor version", k)
		}
	}
}
