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

package modelgateway

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// Every shipped router is known, with the phase it chooses in: the routers that
// read only headers choose there, every one that can read the body is recorded as
// choosing from it.
func TestRouterInfoFor_EveryBuiltinRouter(t *testing.T) {
	for name, want := range map[string]SelectionPhase{
		"model-round-robin":          PhaseHeaders,
		"model-weighted-round-robin": PhaseHeaders,
		"llm-header-router":          PhaseHeaders,
		"semantic-model-routing":     PhaseBody,
		"intelligent-model-routing":  PhaseBody,
		"typesafe-jev-model-routing": PhaseBody,
		"time-based-model-routing":   PhaseBody,
		"cost-based-model-routing":   PhaseBody,
	} {
		info, ok := RouterInfoFor(name, nil)
		require.True(t, ok, name)
		assert.Equal(t, want, info.Phase, name)
	}
}

func TestRouterInfoFor_CustomRouterDeclaresItself(t *testing.T) {
	params := map[string]interface{}{ModelRoutingSchemaExtension: map[string]interface{}{"phase": "headers"}}
	info, ok := RouterInfoFor("acme-router", &models.PolicyDefinition{Name: "acme-router", Parameters: &params})
	require.True(t, ok)
	assert.Equal(t, RouterInfo{Phase: PhaseHeaders}, info)

	// Phase defaults to body: the conservative answer.
	params = map[string]interface{}{ModelRoutingSchemaExtension: map[string]interface{}{}}
	info, ok = RouterInfoFor("acme-router", &models.PolicyDefinition{Parameters: &params})
	require.True(t, ok)
	assert.Equal(t, PhaseBody, info.Phase)

	params = map[string]interface{}{ModelRoutingSchemaExtension: map[string]interface{}{"phase": "whenever"}}
	info, ok = RouterInfoFor("acme-router", &models.PolicyDefinition{Parameters: &params})
	require.True(t, ok)
	assert.Equal(t, PhaseBody, info.Phase)

	// No extension, or one that is not an object, is not a model router.
	_, ok = RouterInfoFor("set-headers", &models.PolicyDefinition{})
	assert.False(t, ok)
	params = map[string]interface{}{ModelRoutingSchemaExtension: "yes"}
	_, ok = RouterInfoFor("acme-router", &models.PolicyDefinition{Parameters: &params})
	assert.False(t, ok)
	_, ok = RouterInfoFor("acme-router", nil)
	assert.False(t, ok)
}
