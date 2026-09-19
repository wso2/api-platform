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

package utils

import (
	"encoding/json"
	"flag"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// updateGolden regenerates the golden files instead of asserting against them.
// It exists so the goldens can be captured from the PRE-change build: running
// it after a behavioural change would make this test assert the change rather
// than the baseline it is here to protect.
var updateGolden = flag.Bool("update-golden", false, "regenerate compat golden files")

const compatDir = "testdata/compat"

// TestLLMProviderTransformer_Compat is the regression guard for the whole
// feature: every fixture under testdata/compat is transformed and compared
// byte-for-byte against a golden captured before any production code changed.
// A diff here means an existing proxy would deploy differently — stop and
// raise it rather than refreshing the golden.
func TestLLMProviderTransformer_Compat(t *testing.T) {
	fixtures, err := filepath.Glob(filepath.Join(compatDir, "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, fixtures, "no compat fixtures found")

	for _, fixture := range fixtures {
		name := filepath.Base(fixture)
		t.Run(name, func(t *testing.T) {
			transformer, _ := newCompatEnvironment(t)

			raw, err := os.ReadFile(fixture)
			require.NoError(t, err)
			var proxy api.LLMProxyConfiguration
			require.NoError(t, json.Unmarshal(raw, &proxy))

			result, err := transformer.Transform(&proxy, &api.RestAPI{})
			require.NoError(t, err)

			actual, err := json.MarshalIndent(result, "", "  ")
			require.NoError(t, err)
			actual = append(actual, '\n')

			goldenPath := filepath.Join(compatDir, "golden", name)
			if *updateGolden {
				require.NoError(t, os.WriteFile(goldenPath, actual, 0o644))
				t.Logf("golden written: %s", goldenPath)
				return
			}

			expected, err := os.ReadFile(goldenPath)
			require.NoError(t, err, "golden missing — regenerate with -update-golden on the pre-change build")
			require.Equal(t, string(expected), string(actual),
				"transformed output differs from the pre-change baseline for %s", name)
		})
	}
}

// newCompatEnvironment builds the fixed world the fixtures are written
// against: three deployed providers on three templates, and a resolver that
// knows the policies they reference.
func newCompatEnvironment(t *testing.T) (*LLMProviderTransformer, storage.Storage) {
	t.Helper()
	store := storage.NewConfigStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db := newTestSQLiteStorage(t, logger)

	for i, handle := range []string{"openai", "anthropic", "gemini"} {
		require.NoError(t, db.SaveLLMProviderTemplate(&models.StoredLLMProviderTemplate{
			UUID: "0000-compat-template-" + handle,
			Configuration: api.LLMProviderTemplate{
				ApiVersion: api.LLMProviderTemplateApiVersionGatewayApiPlatformWso2Comv1,
				Kind:       api.LLMProviderTemplateKindLlmProviderTemplate,
				Metadata:   api.Metadata{Name: handle},
				Spec:       api.LLMProviderTemplateData{DisplayName: handle},
			},
		}))
		_ = i
	}

	saveProvider := func(name, template string) {
		require.NoError(t, db.SaveConfig(&models.StoredConfig{
			UUID:        name + "-compat-uuid",
			Kind:        string(api.LLMProviderConfigurationKindLlmProvider),
			Handle:      name,
			DisplayName: name,
			Version:     "v1.0",
			SourceConfiguration: api.LLMProviderConfiguration{
				ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1,
				Kind:       api.LLMProviderConfigurationKindLlmProvider,
				Metadata:   api.Metadata{Name: name},
				Spec: api.LLMProviderConfigData{
					DisplayName:   name,
					Version:       "v1.0",
					Context:       stringPtr("/" + name),
					Template:      template,
					Upstream:      api.LLMProviderConfigData_Upstream{Url: stringPtr("https://example.com")},
					AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
				},
			},
			DesiredState: models.StateDeployed,
		}))
	}
	saveProvider("openai-provider", "openai")
	saveProvider("anthropic-provider", "anthropic")
	saveProvider("gemini-provider", "gemini")

	resolver := NewStaticPolicyVersionResolver(map[string]string{
		constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME: testSetHeadersVersion,
		constants.ACCESS_CONTROL_DENY_POLICY_NAME:  testRespondVersion,
		constants.UPSTREAM_AUTH_OAUTH2_POLICY_NAME: testOAuth2AuthenticationVersion,
		"llm-header-router":                        "v1.0.0",
		"openai-to-anthropic-transformer":          "v0.9.2",
	})

	return NewLLMProviderTransformer(store, db, &config.RouterConfig{ListenerPort: 8080}, resolver), db
}
