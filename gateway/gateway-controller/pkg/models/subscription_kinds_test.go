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

package models

import (
	"reflect"
	"testing"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

func TestTakesSubscriptions(t *testing.T) {
	for kind, want := range map[string]bool{
		KindRestApi: true, KindMcp: true, KindGraphQLApi: false, KindLlmProxy: false, KindAgent: false, "": false,
	} {
		if got := TakesSubscriptions(kind); got != want {
			t.Errorf("TakesSubscriptions(%q) = %v, want %v", kind, got, want)
		}
	}
}

func TestSubscriptionPlanNames(t *testing.T) {
	plans := &[]string{"Gold", "Silver"}
	rest := api.RestAPI{Spec: api.APIConfigData{SubscriptionPlans: plans}}
	mcp := api.MCPProxyConfiguration{Spec: api.MCPProxyConfigData{SubscriptionPlans: plans}}
	var nilMCP *api.MCPProxyConfiguration
	var nilREST *api.RestAPI

	for name, tc := range map[string]struct {
		cfg  *StoredConfig
		want []string
	}{
		"rest value":        {&StoredConfig{Kind: KindRestApi, Configuration: rest}, []string{"Gold", "Silver"}},
		"rest pointer":      {&StoredConfig{Kind: KindRestApi, Configuration: &rest}, []string{"Gold", "Silver"}},
		"rest nil pointer":  {&StoredConfig{Kind: KindRestApi, Configuration: nilREST}, nil},
		"mcp value":         {&StoredConfig{Kind: KindMcp, SourceConfiguration: mcp}, []string{"Gold", "Silver"}},
		"mcp pointer":       {&StoredConfig{Kind: KindMcp, SourceConfiguration: &mcp}, []string{"Gold", "Silver"}},
		"mcp nil pointer":   {&StoredConfig{Kind: KindMcp, SourceConfiguration: nilMCP}, nil},
		"mcp without plans": {&StoredConfig{Kind: KindMcp, SourceConfiguration: api.MCPProxyConfiguration{}}, nil},
		"other kind":        {&StoredConfig{Kind: KindLlmProxy, Configuration: rest}, nil},
		"nil config":        {nil, nil},
	} {
		if got := SubscriptionPlanNames(tc.cfg); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: SubscriptionPlanNames = %v, want %v", name, got, tc.want)
		}
	}
}
