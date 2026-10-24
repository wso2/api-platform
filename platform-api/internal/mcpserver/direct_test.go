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

package mcpserver

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wso2/api-platform/platform-api/api"
)

// TestNewDirect_FailsWhenATypeHasNoAdapter catches a row added to apiTypes
// without the services behind it.
func TestNewDirect_FailsWhenATypeHasNoAdapter(t *testing.T) {
	d, err := newDirect(Deps{})
	assert.NoError(t, err)
	assert.Len(t, d.apis, len(apiTypes))

	apiTypes = append(apiTypes, apiType{name: "new-type"})
	t.Cleanup(func() { apiTypes = apiTypes[:len(apiTypes)-1] })
	_, err = newDirect(Deps{})
	assert.ErrorContains(t, err, "new-type has no direct adapter")
}

// TestCheckUpstream pins the REST handler's rules (handler/api.go), which
// checkUpstream repeats for the direct backend.
func TestCheckUpstream(t *testing.T) {
	url, ref, blank := "https://orders.example.com", "orders-backend", " "
	tests := []struct {
		name    string
		u       api.Upstream
		wantErr string
	}{
		{"main url", api.Upstream{Main: api.UpstreamDefinition{Url: &url}}, ""},
		{"main ref", api.Upstream{Main: api.UpstreamDefinition{Ref: &ref}}, ""},
		{"sandbox only", api.Upstream{Sandbox: &api.UpstreamDefinition{Url: &url}}, ""},
		{"none", api.Upstream{}, "At least one upstream endpoint"},
		{"blank counts as none", api.Upstream{Main: api.UpstreamDefinition{Url: &blank}}, "At least one upstream endpoint"},
		{"main has both", api.Upstream{Main: api.UpstreamDefinition{Url: &url, Ref: &ref}}, "upstream main must specify either"},
		{"sandbox has both", api.Upstream{Main: api.UpstreamDefinition{Url: &url}, Sandbox: &api.UpstreamDefinition{Url: &url, Ref: &ref}}, "upstream sandbox must specify either"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkUpstream(tc.u)
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}
