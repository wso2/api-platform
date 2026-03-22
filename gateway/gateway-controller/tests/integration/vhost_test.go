/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package integration

import (
	"fmt"
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
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

func TestVhostMaterializationOnDeploy(t *testing.T) {
	const mainDefault = "*.gw.example.com"
	const sandboxDefault = "*-sandbox.gw.example.com"

	makeYAML := func(name, vhostBlock string) []byte {
		yaml := fmt.Sprintf(`apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: %s
spec:
  displayName: Test API %s
  version: "1.0"
  context: /%s
  upstream:
    main:
      url: http://backend:8080
  operations:
    - method: GET
      path: /resource
`, name, name, name)
		if vhostBlock != "" {
			yaml += vhostBlock
		}
		return []byte(yaml)
	}

	setupService := func(t *testing.T, routerCfg *config.RouterConfig) (*utils.APIDeploymentService, storage.Storage) {
		t.Helper()
		db, _, cleanup := setupTestDB(t)
		t.Cleanup(cleanup)

		store := storage.NewConfigStore()
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		validator := config.NewAPIValidator()
		fullCfg := &config.Config{Router: *routerCfg}
		snapshotManager := xds.NewSnapshotManager(store, logger, routerCfg, db, fullCfg)
		svc := utils.NewAPIDeploymentService(store, db, snapshotManager, validator, routerCfg)
		return svc, db
	}

	routerCfg := &config.RouterConfig{
		VHosts: config.VHostsConfig{
			Main:    config.VHostEntry{Default: mainDefault},
			Sandbox: config.VHostEntry{Default: sandboxDefault},
		},
	}

	t.Run("RestApi without vhosts", func(t *testing.T) {
		svc, db := setupService(t, routerCfg)
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))

		result, err := svc.DeployAPIConfiguration(utils.APIDeploymentParams{
			Data:        makeYAML("no-vhosts", ""),
			ContentType: "application/yaml",
			Logger:      logger,
			Origin:      models.OriginGatewayAPI,
		})
		require.NoError(t, err)
		require.NotNil(t, result)

		// Read back from DB
		cfg, err := db.GetConfigByKindAndHandle(models.KindRestApi, "no-vhosts")
		require.NoError(t, err)

		srcCfg, ok := cfg.SourceConfiguration.(api.RestAPI)
		require.True(t, ok, "SourceConfiguration should be api.RestAPI")

		apiData := srcCfg.Spec

		require.NotNil(t, apiData.Vhosts, "vhosts should be populated")
		assert.Equal(t, mainDefault, apiData.Vhosts.Main)
		require.NotNil(t, apiData.Vhosts.Sandbox, "sandbox vhost should be populated")
		assert.Equal(t, sandboxDefault, *apiData.Vhosts.Sandbox)
	})

	t.Run("RestApi with sentinel vhosts", func(t *testing.T) {
		svc, db := setupService(t, routerCfg)
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))

		vhostBlock := fmt.Sprintf(`  vhosts:
    main: %q
    sandbox: %q
`, constants.VHostGatewayDefault, constants.VHostGatewayDefault)
		result, err := svc.DeployAPIConfiguration(utils.APIDeploymentParams{
			Data:        makeYAML("sentinel-vhosts", vhostBlock),
			ContentType: "application/yaml",
			Logger:      logger,
			Origin:      models.OriginGatewayAPI,
		})
		require.NoError(t, err)
		require.NotNil(t, result)

		cfg, err := db.GetConfigByKindAndHandle(models.KindRestApi, "sentinel-vhosts")
		require.NoError(t, err)

		srcCfg, ok := cfg.SourceConfiguration.(api.RestAPI)
		require.True(t, ok)

		apiData := srcCfg.Spec

		require.NotNil(t, apiData.Vhosts)
		assert.Equal(t, mainDefault, apiData.Vhosts.Main, "sentinel should resolve to router default")
		require.NotNil(t, apiData.Vhosts.Sandbox)
		assert.Equal(t, sandboxDefault, *apiData.Vhosts.Sandbox, "sentinel should resolve to sandbox default")
	})

	t.Run("RestApi with explicit vhosts", func(t *testing.T) {
		svc, db := setupService(t, routerCfg)
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))

		vhostBlock := `  vhosts:
    main: "custom.example.com"
    sandbox: "custom-sandbox.example.com"
`
		result, err := svc.DeployAPIConfiguration(utils.APIDeploymentParams{
			Data:        makeYAML("explicit-vhosts", vhostBlock),
			ContentType: "application/yaml",
			Logger:      logger,
			Origin:      models.OriginGatewayAPI,
		})
		require.NoError(t, err)
		require.NotNil(t, result)

		cfg, err := db.GetConfigByKindAndHandle(models.KindRestApi, "explicit-vhosts")
		require.NoError(t, err)

		srcCfg, ok := cfg.SourceConfiguration.(api.RestAPI)
		require.True(t, ok)

		apiData := srcCfg.Spec

		require.NotNil(t, apiData.Vhosts)
		assert.Equal(t, "custom.example.com", apiData.Vhosts.Main, "explicit vhost should be unchanged")
		require.NotNil(t, apiData.Vhosts.Sandbox)
		assert.Equal(t, "custom-sandbox.example.com", *apiData.Vhosts.Sandbox, "explicit sandbox vhost should be unchanged")
	})

	t.Run("RestApi without vhosts and no sandbox default", func(t *testing.T) {
		noSandboxCfg := &config.RouterConfig{
			VHosts: config.VHostsConfig{
				Main:    config.VHostEntry{Default: mainDefault},
				Sandbox: config.VHostEntry{Default: ""},
			},
		}
		svc, db := setupService(t, noSandboxCfg)
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))

		result, err := svc.DeployAPIConfiguration(utils.APIDeploymentParams{
			Data:        makeYAML("no-sandbox-default", ""),
			ContentType: "application/yaml",
			Logger:      logger,
			Origin:      models.OriginGatewayAPI,
		})
		require.NoError(t, err)
		require.NotNil(t, result)

		cfg, err := db.GetConfigByKindAndHandle(models.KindRestApi, "no-sandbox-default")
		require.NoError(t, err)

		srcCfg, ok := cfg.SourceConfiguration.(api.RestAPI)
		require.True(t, ok)

		apiData := srcCfg.Spec

		require.NotNil(t, apiData.Vhosts, "vhosts should be populated")
		assert.Equal(t, mainDefault, apiData.Vhosts.Main, "main vhost should be populated")
		assert.Nil(t, apiData.Vhosts.Sandbox, "sandbox should be nil when no sandbox default configured")
	})
}
