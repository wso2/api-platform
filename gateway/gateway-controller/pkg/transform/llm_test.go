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
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils/clusterkey"
)

// TestLLMTransformer_ClusterNameKeyedOnLLMUUID verifies that an LLM config receives the
// same identity-based cluster name as a REST API. LLMTransformer converts the config to a
// RestAPI and delegates to RestAPITransformer; this pins that the LLM config's UUID is
// carried through the extra hop, so the cluster name is clusterkey.HashedName(env, UUID)
// and not keyed on anything LLM-specific.
func TestLLMTransformer_ClusterNameKeyedOnLLMUUID(t *testing.T) {
	// A RestAPI Configuration is supplied directly, so Transform uses it as-is and skips
	// the provider transform; no storage backend is needed for this path.
	base := makeRestAPIWithOps([]api.Operation{
		{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/users")},
	})
	restAPI := base.Configuration.(api.RestAPI)
	cfg := &models.StoredConfig{
		UUID:          "test-llm-api",
		Kind:          "LlmProxy",
		Configuration: restAPI,
	}

	// Construct directly with only restTransformer set: the RestAPI-direct path does not
	// use the provider transformer or storage, so this avoids an unrelated db dependency.
	transformer := &LLMTransformer{
		restTransformer: NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{}),
	}

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)

	mainRoute := rdc.Routes["GET|/test/users|main.local"]
	require.NotNil(t, mainRoute, "main route must exist")
	assert.Equal(t, clusterkey.HashedName("main", cfg.UUID), mainRoute.Upstream.ClusterKey,
		"LLM cluster name must be the identity name keyed on the LLM config UUID")
}
