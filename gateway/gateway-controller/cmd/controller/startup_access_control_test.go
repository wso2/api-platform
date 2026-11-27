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

package main

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
)

func accessControlConfig(ac config.AccessControlConfig) *config.Config {
	cfg := &config.Config{}
	cfg.Controller.Server.NetworkSecurity.AccessControl = ac
	return cfg
}

func metadataUpstream(handle string) *models.StoredConfig {
	return &models.StoredConfig{UUID: handle, Kind: models.KindRestApi, Handle: handle, DisplayName: handle, Version: "v1",
		DesiredState: models.StateDeployed,
		Configuration: configWithTemplateSpec{Spec: map[string]any{
			"upstream": map[string]any{"main": map[string]any{"url": "http://169.254.169.254/latest"}},
		}}}
}

func TestInstallArtifactAccessControl(t *testing.T) {
	t.Cleanup(func() { templateengine.SetPostRenderCheck(nil) })
	render := func() error { return templateengine.RenderSpec(metadataUpstream("api"), nil, nil) }

	// Enabled: RenderSpec refuses the metadata address.
	require.NoError(t, installArtifactAccessControl(accessControlConfig(config.AccessControlConfig{Enabled: true, Mode: "deny"}), nil, newDiscardLogger()))
	assert.Error(t, render(), "an enabled policy refuses the metadata address")

	// Disabled: the check is cleared.
	require.NoError(t, installArtifactAccessControl(accessControlConfig(config.AccessControlConfig{Enabled: false}), nil, newDiscardLogger()))
	assert.NoError(t, render(), "a disabled policy refuses nothing")

	// Invalid: reported, so main can refuse to start.
	err := installArtifactAccessControl(accessControlConfig(config.AccessControlConfig{Enabled: true, Mode: "allow"}), nil, newDiscardLogger())
	assert.Error(t, err)
}

// loadStartupRuntimeConfigs runs before the first router snapshot: a refused
// artifact is undeployed and persisted there, and the rest load.
func TestLoadStartupRuntimeConfigs_UndeploysRefusedArtifactsBeforeTheFirstSnapshot(t *testing.T) {
	require.NoError(t, installArtifactAccessControl(accessControlConfig(config.AccessControlConfig{Enabled: true, Mode: "deny"}), nil, newDiscardLogger()))
	t.Cleanup(func() { templateengine.SetPostRenderCheck(nil) })

	refused := metadataUpstream("refused-api")
	valid := &models.StoredConfig{UUID: "valid-api", Kind: models.KindRestApi, Handle: "valid-api", DisplayName: "V", Version: "v1",
		DesiredState: models.StateDeployed,
		Configuration: configWithTemplateSpec{Spec: map[string]any{
			"upstream": map[string]any{"main": map[string]any{"url": "http://backend.example.com"}},
		}}}

	runtimeStore := storage.NewRuntimeConfigStore()
	var persisted []string
	err := loadStartupRuntimeConfigs([]*models.StoredConfig{refused, valid}, runtimeStore, &fakeSecretResolver{},
		&fakeRuntimeTransformer{}, nil, &config.RouterConfig{}, true, false,
		func(c *models.StoredConfig) error { persisted = append(persisted, c.UUID); return nil }, newDiscardLogger())

	require.NoError(t, err)
	assert.Equal(t, []string{"refused-api"}, persisted)
	assert.Equal(t, models.StateUndeployed, refused.DesiredState)
	_, ok := runtimeStore.Get(storage.Key(models.KindRestApi, "valid-api"))
	assert.True(t, ok, "positive control: the permitted artifact loads")

	// A nil persist callback and a nil logger are tolerated.
	again := metadataUpstream("refused-again")
	require.NoError(t, loadStartupRuntimeConfigs([]*models.StoredConfig{again}, storage.NewRuntimeConfigStore(),
		&fakeSecretResolver{}, &fakeRuntimeTransformer{}, nil, &config.RouterConfig{}, false, false, nil, nil))
	assert.Equal(t, models.StateUndeployed, again.DesiredState)
}

// jwksRouteTransformer gives every artifact a route on the gateway JWKS path.
type jwksRouteTransformer struct{}

func (jwksRouteTransformer) Transform(cfg *models.StoredConfig) (*models.RuntimeDeployConfig, error) {
	return &models.RuntimeDeployConfig{
		Metadata: models.Metadata{UUID: cfg.UUID, Kind: cfg.Kind, Handle: cfg.Handle},
		Routes:   map[string]*models.Route{"r": {Method: "GET", Path: constants.GatewayJWKSPath}},
	}, nil
}

type failingTransformer struct{}

func (failingTransformer) Transform(*models.StoredConfig) (*models.RuntimeDeployConfig, error) {
	return nil, errors.New("transform failed")
}

func TestLoadStartupRuntimeConfigs_ReportsJWKSConflictsAndLoadErrors(t *testing.T) {
	api := &models.StoredConfig{UUID: "jwks-api", Kind: models.KindRestApi, Handle: "jwks-api", DesiredState: models.StateDeployed}
	runtimeStore := storage.NewRuntimeConfigStore()
	require.NoError(t, loadStartupRuntimeConfigs([]*models.StoredConfig{api}, runtimeStore, nil,
		jwksRouteTransformer{}, nil, &config.RouterConfig{}, true, false, nil, newDiscardLogger()))
	_, ok := runtimeStore.Get(storage.Key(models.KindRestApi, "jwks-api"))
	assert.True(t, ok, "a route shadowed by the JWKS endpoint still loads; it is only reported")

	err := loadStartupRuntimeConfigs([]*models.StoredConfig{api}, storage.NewRuntimeConfigStore(), nil,
		failingTransformer{}, nil, &config.RouterConfig{}, false, false, nil, newDiscardLogger())
	assert.Error(t, err, "a load failure without skip_invalid_deployments_on_startup stops startup")
}
