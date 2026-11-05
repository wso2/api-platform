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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// upstreamWithAPIKeyAuth returns an Agent's deprecated header/value upstream
// auth. The auth field is an anonymous struct, so it is decoded rather than
// spelled out.
func upstreamWithAPIKeyAuth(t *testing.T, header, value string) api.AgentConfigData_Upstream {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"type": "api-key", "header": header, "value": value})
	require.NoError(t, err)
	var upstream api.AgentConfigData_Upstream
	require.NoError(t, json.Unmarshal(raw, &upstream.Auth))
	return upstream
}

// The values below used to be pasted into a single-quoted YAML template: '%'
// was doubled, a quote failed the transform (the Agent was reported deployed
// but never served), and a crafted value added a header of its own (#3791).
func TestAgentUpstreamAuthPolicy_KeepsHeaderVerbatim(t *testing.T) {
	for _, value := range []string{
		"abc%2F",
		"Bearer it's",
		"y'\n    - name: 'X-Injected'\n      value: 'z",
	} {
		t.Run(value, func(t *testing.T) {
			upstream := upstreamWithAPIKeyAuth(t, "X-Up-Auth", value)
			policy := agentUpstreamAuthPolicy(upstream.Auth)
			require.NotNil(t, policy)
			assert.Equal(t, constants.SET_HEADERS_POLICY_NAME, policy.Name)
			require.NotNil(t, policy.Params)
			assert.Equal(t, utils.SetHeadersParams("X-Up-Auth", value), *policy.Params)
		})
	}
}
