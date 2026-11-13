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

package templateengine

import (
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine/funcs"
)

// TestMain admits the variable names the package's env-based render tests use;
// production is deny-all until the controller installs its configured policy.
func TestMain(m *testing.M) {
	policy, err := funcs.NewEnvAccessPolicy([]string{"TEST_*", "NONEXISTENT_*"})
	if err != nil {
		panic(err)
	}
	funcs.SetEnvAccessPolicy(policy)
	os.Exit(m.Run())
}

// An API author must not be able to pull the controller's own configuration
// (here, the control-plane token) into a policy param or upstream.
func TestRenderSpec_RefusesControllerEnvironment(t *testing.T) {
	t.Setenv("APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN", "cp-token-value")

	cfg := &models.StoredConfig{
		Kind:   "RestApi",
		Handle: "exfil-api",
		Configuration: testConfig{
			ApiVersion: "v1",
			Kind:       "RestApi",
			Metadata:   testMetadata{Name: "exfil-api"},
			Spec: testSpec{
				DisplayName: "Exfil",
				Upstream:    testUpstream{URL: `{{ env "APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN" }}`},
			},
		},
	}

	err := RenderSpec(cfg, &mockResolver{}, slog.Default())

	var renderErr *RenderError
	require.True(t, errors.As(err, &renderErr))
	var envErr *funcs.EnvAccessError
	assert.True(t, errors.As(err, &envErr))
	assert.NotContains(t, err.Error(), "cp-token-value")
	assert.Empty(t, cfg.SensitiveValues)
}
