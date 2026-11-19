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
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

func validSoapAPI() api.SoapAPI {
	spec := api.SoapAPIData{DisplayName: "StockQuote API", Version: "v1.0", Context: "/stockquote/$version"}
	spec.Upstream.Main = api.Upstream{Url: api.Ptr("http://stockquote-service:8080/services/StockQuote")}
	return api.SoapAPI{
		ApiVersion: api.SoapAPIApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.SoapAPIKindSoapApi,
		Metadata:   api.Metadata{Name: "stockquote-v1.0"},
		Spec:       spec,
	}
}

func TestValidateSoapAPIConfig_Valid(t *testing.T) {
	name, version, errs := validateSoapAPIConfig(validSoapAPI())
	assert.Empty(t, errs)
	assert.Equal(t, "StockQuote API", name)
	assert.Equal(t, "v1.0", version)
}

func TestValidateSoapAPIConfig_Rejects(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*api.SoapAPI)
		field  string
	}{
		{"wrong kind", func(c *api.SoapAPI) { c.Kind = "RestApi" }, "kind"},
		{"bad version", func(c *api.SoapAPI) { c.Spec.Version = "version-one" }, "spec.version"},
		{"context without leading slash", func(c *api.SoapAPI) { c.Spec.Context = "stockquote" }, "spec.context"},
		{"missing upstream url", func(c *api.SoapAPI) { c.Spec.Upstream.Main = api.Upstream{} }, "spec.upstream.main.url"},
		{"non-http upstream", func(c *api.SoapAPI) { c.Spec.Upstream.Main.Url = api.Ptr("ftp://backend/svc") }, "spec.upstream.main.url"},
		// SoapApi has no upstreamDefinitions list for a ref to resolve against.
		{"upstream ref", func(c *api.SoapAPI) { c.Spec.Upstream.Main.Ref = api.Ptr("some-def") }, "spec.upstream.main.ref"},
		{"sandbox ref", func(c *api.SoapAPI) { c.Spec.Upstream.Sandbox = &api.Upstream{Ref: api.Ptr("some-def")} }, "spec.upstream.sandbox.ref"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validSoapAPI()
			tt.mutate(&cfg)
			_, _, errs := validateSoapAPIConfig(cfg)
			assert.True(t, containsFieldError(errs, tt.field), "expected an error on %s, got %+v", tt.field, errs)
		})
	}
}

func TestValidateSoapAPIConfig_WrongType(t *testing.T) {
	_, _, errs := validateSoapAPIConfig(api.RestAPI{})
	assert.True(t, containsFieldError(errs, "config"))
}

func TestResolveVhostSentinels_SoapAPI(t *testing.T) {
	routerCfg := &config.RouterConfig{VHosts: config.VHostsConfig{
		Main:    config.VHostEntry{Default: "gw.example.com"},
		Sandbox: config.VHostEntry{Default: "sandbox.gw.example.com"},
	}}

	t.Run("omitted vhosts take the gateway defaults", func(t *testing.T) {
		var cfg any = validSoapAPI()
		require.NoError(t, resolveVhostSentinels(&cfg, routerCfg))
		vhosts := cfg.(api.SoapAPI).Spec.Vhosts
		require.NotNil(t, vhosts)
		assert.Equal(t, "gw.example.com", vhosts.Main)
		require.NotNil(t, vhosts.Sandbox)
		assert.Equal(t, "sandbox.gw.example.com", *vhosts.Sandbox)
	})

	t.Run("sentinels are replaced, explicit hosts kept", func(t *testing.T) {
		soapAPI := validSoapAPI()
		soapAPI.Spec.Vhosts = &struct {
			Main    string  `json:"main" yaml:"main"`
			Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{Main: constants.VHostGatewayDefault, Sandbox: api.Ptr("sb.soap.example.com")}
		var cfg any = soapAPI
		require.NoError(t, resolveVhostSentinels(&cfg, routerCfg))
		vhosts := cfg.(api.SoapAPI).Spec.Vhosts
		assert.Equal(t, "gw.example.com", vhosts.Main)
		assert.Equal(t, "sb.soap.example.com", *vhosts.Sandbox)
	})
}

// A SoapApi deployed with DesiredState undeployed is stored undeployed, so the
// translator withdraws its routes while the configuration stays; a later
// deploy without a state brings it back.
func TestDeploySoapAPI_HonoursDesiredState(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db := newTestMockDB()
	service := newTestAPIDeploymentService(storage.NewConfigStore(), db, nil, config.NewAPIValidator(), nil)
	body := []byte(`apiVersion: gateway.api-platform.wso2.com/v1
kind: SoapApi
metadata:
  name: stockquote
spec:
  displayName: StockQuote API
  version: v1.0
  context: /stockquote/$version
  upstream:
    main:
      url: http://backend:8080/ws
`)
	deploy := func(apiID string, state models.DesiredState) *APIDeploymentResult {
		result, err := service.DeployAPIConfiguration(APIDeploymentParams{
			APIID:         apiID,
			Data:          body,
			ContentType:   "application/yaml",
			Kind:          string(api.SoapAPIKindSoapApi),
			Origin:        models.OriginGatewayAPI,
			DesiredState:  state,
			CorrelationID: "test-corr",
			Logger:        logger,
		})
		require.NoError(t, err)
		return result
	}

	created := deploy("", models.StateUndeployed)
	assert.Equal(t, models.StateUndeployed, created.StoredConfig.DesiredState)
	stored, err := db.GetConfig(created.StoredConfig.UUID)
	require.NoError(t, err)
	assert.Equal(t, models.StateUndeployed, stored.DesiredState)

	redeployed := deploy(created.StoredConfig.UUID, "")
	assert.True(t, redeployed.IsUpdate)
	assert.Equal(t, models.StateDeployed, redeployed.StoredConfig.DesiredState)
}
