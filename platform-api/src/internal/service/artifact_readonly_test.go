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

package service

import (
	"testing"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/utils"
)

func wantReadOnly(t *testing.T, got *bool, want bool, kind string) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s: readOnly is nil, want %v", kind, want)
	}
	if *got != want {
		t.Errorf("%s: readOnly = %v, want %v", kind, *got, want)
	}
}

// TestReadOnlyReflectsOrigin verifies every artifact GET mapper sets readOnly=true
// for DP-originated artifacts and false for CP-originated ones.
func TestReadOnlyReflectsOrigin(t *testing.T) {
	const projectUUID = "11111111-1111-1111-1111-111111111111"
	apiUtil := &utils.APIUtil{}

	for _, tc := range []struct {
		origin string
		want   bool
	}{
		{constants.OriginDP, true},
		{constants.OriginCP, false},
		{"", false}, // unset origin defaults to not read-only
	} {
		rest, err := apiUtil.ModelToRESTAPI(&model.API{
			Handle: "h", Name: "n", Version: "v1.0", ProjectID: projectUUID, Origin: tc.origin,
		}, "test-project")
		if err != nil {
			t.Fatalf("ModelToRESTAPI(%q): %v", tc.origin, err)
		}
		wantReadOnly(t, rest.ReadOnly, tc.want, "RESTAPI/"+tc.origin)

		prov := mapProviderModelToAPI(&model.LLMProvider{ID: "p", Name: "n", Version: "v1.0", Origin: tc.origin}, "tmpl")
		wantReadOnly(t, prov.ReadOnly, tc.want, "LLMProvider/"+tc.origin)

		proxy := mapProxyModelToAPI(&model.LLMProxy{ID: "px", Name: "n", Version: "v1.0", Origin: tc.origin})
		wantReadOnly(t, proxy.ReadOnly, tc.want, "LLMProxy/"+tc.origin)

		tmpl := mapTemplateModelToAPI(&model.LLMProviderTemplate{ID: "t", Name: "n", Origin: tc.origin})
		wantReadOnly(t, tmpl.ReadOnly, tc.want, "LLMProviderTemplate/"+tc.origin)

		mcp := mapMCPProxyModelToAPI(&model.MCPProxy{Handle: "m", Name: "n", Version: "v1.0", Origin: tc.origin})
		wantReadOnly(t, mcp.ReadOnly, tc.want, "MCPProxy/"+tc.origin)
	}
}
