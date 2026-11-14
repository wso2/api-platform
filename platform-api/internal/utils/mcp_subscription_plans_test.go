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

package utils

import (
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/platform-api/internal/model"
)

func TestGenerateMCPDeploymentYAMLSubscriptionPlans(t *testing.T) {
	u := &MCPUtils{}
	proxy := &model.MCPProxy{
		Handle:  "docs-mcp",
		Name:    "Docs MCP",
		Version: "v1.0",
		Configuration: model.MCPProxyConfiguration{
			Upstream:          model.UpstreamConfig{Main: &model.UpstreamEndpoint{URL: "https://example.com/mcp"}},
			SubscriptionPlans: []string{"Gold", "Silver"},
		},
	}

	out, err := u.GenerateMCPDeploymentYAML(proxy)
	if err != nil {
		t.Fatalf("GenerateMCPDeploymentYAML returned error: %v", err)
	}
	var deployment model.MCPProxyDeploymentYAML
	if err := yaml.Unmarshal([]byte(out), &deployment); err != nil {
		t.Fatalf("deployment YAML does not parse: %v\n%s", err, out)
	}
	if !reflect.DeepEqual(deployment.Spec.SubscriptionPlans, []string{"Gold", "Silver"}) {
		t.Errorf("spec.subscriptionPlans = %v, want [Gold Silver]:\n%s", deployment.Spec.SubscriptionPlans, out)
	}

	proxy.Configuration.SubscriptionPlans = nil
	out, err = u.GenerateMCPDeploymentYAML(proxy)
	if err != nil {
		t.Fatalf("GenerateMCPDeploymentYAML returned error: %v", err)
	}
	if strings.Contains(out, "subscriptionPlans") {
		t.Errorf("deployment YAML must omit subscriptionPlans when none are set:\n%s", out)
	}
}
