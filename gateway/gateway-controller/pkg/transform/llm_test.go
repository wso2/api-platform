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

package transform

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"
)

func TestWithLLMCost(t *testing.T) {
	withCost := map[string]models.PolicyDefinition{
		"llm-cost|v1.2.0": {Name: "llm-cost", Version: "v1.2.0"},
	}
	names := func(chain []policyenginev1.PolicyInstance) []string {
		var out []string
		for _, p := range chain {
			out = append(out, p.Name+"@"+p.Version)
		}
		return out
	}
	guardrail := policyenginev1.PolicyInstance{Name: "pii-masking", Version: "v1"}

	cases := []struct {
		name  string
		kind  string
		chain []policyenginev1.PolicyInstance
		defs  map[string]models.PolicyDefinition
		want  []string
	}{
		{"provider without a price gets llm-cost last", models.KindLlmProvider,
			[]policyenginev1.PolicyInstance{guardrail}, withCost, []string{"pii-masking@v1", "llm-cost@v1"}},
		{"provider already priced keeps its chain", models.KindLlmProvider,
			[]policyenginev1.PolicyInstance{{Name: "llm-cost", Version: "v1"}, guardrail}, withCost, []string{"llm-cost@v1", "pii-masking@v1"}},
		{"azure pricing counts as priced", models.KindLlmProvider,
			[]policyenginev1.PolicyInstance{{Name: "azure-llm-cost", Version: "v1"}}, withCost, []string{"azure-llm-cost@v1"}},
		{"proxies are left alone", models.KindLlmProxy,
			[]policyenginev1.PolicyInstance{guardrail}, withCost, []string{"pii-masking@v1"}},
		{"REST APIs are left alone", models.KindRestApi,
			nil, withCost, nil},
		{"a build without llm-cost is left alone", models.KindLlmProvider,
			[]policyenginev1.PolicyInstance{guardrail}, map[string]models.PolicyDefinition{}, []string{"pii-masking@v1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := withLLMCost(tc.kind, tc.chain, tc.defs, config.BuildLatestVersionIndex(tc.defs))
			assert.Equal(t, tc.want, names(got))
		})
	}
}
