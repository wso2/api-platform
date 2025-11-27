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
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	"go.uber.org/zap"
)

// TestDeployRESTAPIConfiguration tests deploying REST API configurations
func TestDeployRESTAPIConfiguration(t *testing.T) {
	// Setup test environment
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-deploy-rest.db")

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	db, err := storage.NewSQLiteStorage(dbPath, logger)
	require.NoError(t, err)
	defer db.Close()

	store := storage.NewConfigStore()
	accessLogConfig := config.AccessLogsConfig{Enabled: false}
	snapshotMgr := xds.NewSnapshotManager(store, logger, accessLogConfig)

	deploymentService := utils.NewAPIDeploymentService(store, db, snapshotMgr)

	// Create REST API YAML configuration
	restYAML := `
version: api-platform.wso2.com/v1
kind: http/rest
data:
  name: TestRESTAPI
  apiType: http/rest
  version: v1.0
  context: /testrest
  upstream:
    - url: http://backend.example.com
  operations:
    - method: GET
      path: /users
    - method: POST
      path: /users
`

	params := utils.APIDeploymentParams{
		Data:          []byte(restYAML),
		ContentType:   "application/yaml",
		APIID:         "",
		CorrelationID: "test-correlation-rest-1",
		Logger:        logger,
	}

	// Test: Deploy new REST API
	t.Run("Deploy New REST API", func(t *testing.T) {
		result, err := deploymentService.DeployAPIConfiguration(params)
		assert.NoError(t, err, "Deployment should succeed")
		assert.NotNil(t, result)
		assert.NotNil(t, result.StoredConfig)
		assert.False(t, result.IsUpdate, "Should be a new creation, not an update")
		assert.Equal(t, "TestRESTAPI", result.StoredConfig.GetAPIName())
		assert.Equal(t, "v1.0", result.StoredConfig.GetAPIVersion())
		assert.Equal(t, "/testrest", result.StoredConfig.GetContext())

		// Verify it was stored in memory
		retrieved, err := store.GetByNameVersion("TestRESTAPI", "v1.0")
		assert.NoError(t, err)
		assert.Equal(t, result.StoredConfig.ID, retrieved.ID)

		// Verify it was stored in database
		dbConfig, err := db.GetConfig(result.StoredConfig.ID)
		assert.NoError(t, err)
		assert.Equal(t, "TestRESTAPI", dbConfig.GetAPIName())
	})

	// Test: Deploy duplicate REST API (should update)
	t.Run("Deploy Duplicate REST API", func(t *testing.T) {
		// Sleep briefly to ensure timestamps differ
		time.Sleep(10 * time.Millisecond)

		result, err := deploymentService.DeployAPIConfiguration(params)
		assert.Error(t, err, "Duplicate deployment should not succeed via deployment")
		assert.Nil(t, result)
	})
}

// TestDeployAsyncAPIConfiguration tests deploying async (WebSub) API configurations
func TestDeployAsyncAPIConfiguration(t *testing.T) {
	// Setup test environment
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-deploy-async.db")

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	db, err := storage.NewSQLiteStorage(dbPath, logger)
	require.NoError(t, err)
	defer db.Close()

	store := storage.NewConfigStore()
	accessLogConfig := config.AccessLogsConfig{Enabled: false}
	snapshotMgr := xds.NewSnapshotManager(store, logger, accessLogConfig)

	deploymentService := utils.NewAPIDeploymentService(store, db, snapshotMgr)

	// Create async (WebSub) API YAML configuration
	asyncYAML := `
version: api-platform.wso2.com/v1
kind: async/websub
data:
  name: TestWebSubAPI
  apiType: async/websub
  version: v1.0
  context: /testwebsub
  servers:
    - url: http://host.docker.internal:9098/hub
      protocol: websub
  channels:
    - path: /notifications
    - path: /alerts
`

	params := utils.APIDeploymentParams{
		Data:          []byte(asyncYAML),
		ContentType:   "application/yaml",
		APIID:         "",
		CorrelationID: "test-correlation-async-1",
		Logger:        logger,
	}

	// Test: Deploy new async API
	t.Run("Deploy New Async API", func(t *testing.T) {
		result, err := deploymentService.DeployAPIConfiguration(params)
		assert.NoError(t, err, "Async deployment should succeed")
		assert.NotNil(t, result)
		assert.NotNil(t, result.StoredConfig)
		assert.False(t, result.IsUpdate, "Should be a new creation")
		assert.Equal(t, "TestWebSubAPI", result.StoredConfig.GetAPIName())
		assert.Equal(t, "v1.0", result.StoredConfig.GetAPIVersion())
		assert.Equal(t, "/testwebsub", result.StoredConfig.GetContext())

		// Verify configuration kind
		assert.Equal(t, api.APIConfigurationKindAsyncwebsub, result.StoredConfig.Configuration.Kind)

		// Verify it was stored
		retrieved, err := store.GetByNameVersion("TestWebSubAPI", "v1.0")
		assert.NoError(t, err)
		assert.Equal(t, result.StoredConfig.ID, retrieved.ID)

		// Verify async-specific data
		asyncData, err := result.StoredConfig.Configuration.Data.AsWebhookAPIData()
		assert.NoError(t, err)
		assert.Equal(t, 2, len(asyncData.Channels), "Should have 2 channels")
		assert.Equal(t, 1, len(asyncData.Servers), "Should have 1 server")
		assert.Equal(t, api.Websub, asyncData.Servers[0].Protocol)
	})

	// Test: Deploy duplicate async API (should update)
	t.Run("Deploy Duplicate Async API", func(t *testing.T) {
		time.Sleep(10 * time.Millisecond)

		result, err := deploymentService.DeployAPIConfiguration(params)
		assert.Error(t, err, "Duplicate async deployment should not succeed via deployment")
		assert.Nil(t, result)
	})
}

// TestUndeployRESTAPIConfiguration tests undeploying REST API configurations
func TestUndeployRESTAPIConfiguration(t *testing.T) {
	// Setup test environment
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-undeploy-rest.db")

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	db, err := storage.NewSQLiteStorage(dbPath, logger)
	require.NoError(t, err)
	defer db.Close()

	store := storage.NewConfigStore()
	accessLogConfig := config.AccessLogsConfig{Enabled: false}
	snapshotMgr := xds.NewSnapshotManager(store, logger, accessLogConfig)

	deploymentService := utils.NewAPIDeploymentService(store, db, snapshotMgr)

	// First, deploy a REST API
	restYAML := `
version: api-platform.wso2.com/v1
kind: http/rest
data:
  name: UndeployTestAPI
  apiType: http/rest
  version: v1.0
  context: /undeploytest
  upstream:
    - url: http://backend.example.com
  operations:
    - method: GET
      path: /test
`

	deployParams := utils.APIDeploymentParams{
		Data:          []byte(restYAML),
		ContentType:   "application/yaml",
		APIID:         "",
		CorrelationID: "test-deploy-for-undeploy",
		Logger:        logger,
	}

	deployResult, err := deploymentService.DeployAPIConfiguration(deployParams)
	require.NoError(t, err, "Initial deployment must succeed")
	require.NotNil(t, deployResult)

	apiID := deployResult.StoredConfig.ID

	// Test: Undeploy REST API
	t.Run("Undeploy REST API", func(t *testing.T) {
		undeployParams := utils.APIDeploymentParams{
			CorrelationID: "test-undeploy-rest-1",
			Logger:        logger,
		}

		result, err := deploymentService.UndeployAPIConfiguration("UndeployTestAPI", "v1.0", undeployParams)
		assert.NoError(t, err, "Undeploy should succeed")
		assert.NotNil(t, result)
		assert.True(t, result.IsUpdate, "Should be marked as update operation")
		assert.Nil(t, result.StoredConfig, "StoredConfig should be nil after undeploy")

		// Verify it was removed from memory store
		_, err = store.Get(apiID)
		assert.Error(t, err, "Config should not exist in memory after undeploy")
		assert.ErrorIs(t, err, storage.ErrNotFound)

		// Verify it was removed from database
		_, err = db.GetConfig(apiID)
		assert.Error(t, err, "Config should not exist in DB after undeploy")
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})

	// Test: Undeploy non-existent API
	t.Run("Undeploy Non-Existent API", func(t *testing.T) {
		undeployParams := utils.APIDeploymentParams{
			CorrelationID: "test-undeploy-nonexistent",
			Logger:        logger,
		}

		_, err := deploymentService.UndeployAPIConfiguration("NonExistentAPI", "v1.0", undeployParams)
		assert.Error(t, err, "Undeploying non-existent API should fail")
	})
}

// TestUndeployAsyncAPIConfiguration tests undeploying async API configurations
func TestUndeployAsyncAPIConfiguration(t *testing.T) {
	// Setup test environment
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-undeploy-async.db")

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	db, err := storage.NewSQLiteStorage(dbPath, logger)
	require.NoError(t, err)
	defer db.Close()

	store := storage.NewConfigStore()
	accessLogConfig := config.AccessLogsConfig{Enabled: false}
	snapshotMgr := xds.NewSnapshotManager(store, logger, accessLogConfig)

	deploymentService := utils.NewAPIDeploymentService(store, db, snapshotMgr)

	// First, deploy an async API
	asyncYAML := `
version: api-platform.wso2.com/v1
kind: async/websub
data:
  name: UndeployWebSubAPI
  apiType: async/websub
  version: v1.0
  context: /undeploywebsub
  servers:
    - url: http://host.docker.internal:9098/hub
      protocol: websub
  channels:
    - path: /events/test
`

	deployParams := utils.APIDeploymentParams{
		Data:          []byte(asyncYAML),
		ContentType:   "application/yaml",
		APIID:         "",
		CorrelationID: "test-deploy-async-for-undeploy",
		Logger:        logger,
	}

	deployResult, err := deploymentService.DeployAPIConfiguration(deployParams)
	require.NoError(t, err, "Initial async deployment must succeed")
	require.NotNil(t, deployResult)

	apiID := deployResult.StoredConfig.ID

	// Test: Undeploy async API
	t.Run("Undeploy Async API", func(t *testing.T) {
		undeployParams := utils.APIDeploymentParams{
			CorrelationID: "test-undeploy-async-1",
			Logger:        logger,
		}

		result, err := deploymentService.UndeployAPIConfiguration("UndeployWebSubAPI", "v1.0", undeployParams)
		assert.NoError(t, err, "Async undeploy should succeed")
		assert.NotNil(t, result)
		assert.True(t, result.IsUpdate)
		assert.Nil(t, result.StoredConfig)

		// Verify removal from memory
		_, err = store.Get(apiID)
		assert.Error(t, err)
		assert.ErrorIs(t, err, storage.ErrNotFound)

		// Verify removal from database
		_, err = db.GetConfig(apiID)
		assert.Error(t, err)
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})
}

// TestDeployAPIConfiguration_ValidationErrors tests validation error handling
func TestDeployAPIConfiguration_ValidationErrors(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-validation.db")

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	db, err := storage.NewSQLiteStorage(dbPath, logger)
	require.NoError(t, err)
	defer db.Close()

	store := storage.NewConfigStore()
	accessLogConfig := config.AccessLogsConfig{Enabled: false}
	snapshotMgr := xds.NewSnapshotManager(store, logger, accessLogConfig)
	deploymentService := utils.NewAPIDeploymentService(store, db, snapshotMgr)

	// Test: Invalid REST API (missing required fields)
	t.Run("Invalid REST API - Missing Name", func(t *testing.T) {
		invalidYAML := `
version: api-platform.wso2.com/v1
kind: http/rest
data:
  apiType: http/rest
  version: v1.0
  context: /invalid
  upstream:
    - url: http://backend.example.com
  operations:
    - method: GET
      path: /test
`

		params := utils.APIDeploymentParams{
			Data:          []byte(invalidYAML),
			ContentType:   "application/yaml",
			CorrelationID: "test-invalid-1",
			Logger:        logger,
		}

		_, err := deploymentService.DeployAPIConfiguration(params)
		assert.Error(t, err, "Deployment with missing name should fail")
	})

	// Test: Invalid async API (missing required fields)
	t.Run("Invalid Async API - Missing Servers", func(t *testing.T) {
		invalidAsyncYAML := `
version: api-platform.wso2.com/v1
kind: async/websub
data:
  name: InvalidAsync
  apiType: async/websub
  version: v1.0
  context: /invalidasync
  channels:
    - path: /events/test
`

		params := utils.APIDeploymentParams{
			Data:          []byte(invalidAsyncYAML),
			ContentType:   "application/yaml",
			CorrelationID: "test-invalid-async-1",
			Logger:        logger,
		}

		_, err := deploymentService.DeployAPIConfiguration(params)
		assert.Error(t, err, "Deployment with missing servers should fail")
	})
}

// TestDeployAPIConfiguration_ParseErrors tests YAML parsing error handling
func TestDeployAPIConfiguration_ParseErrors(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-parse-errors.db")

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	db, err := storage.NewSQLiteStorage(dbPath, logger)
	require.NoError(t, err)
	defer db.Close()

	store := storage.NewConfigStore()
	accessLogConfig := config.AccessLogsConfig{Enabled: false}
	snapshotMgr := xds.NewSnapshotManager(store, logger, accessLogConfig)
	deploymentService := utils.NewAPIDeploymentService(store, db, snapshotMgr)

	// Test: Malformed YAML
	t.Run("Malformed YAML", func(t *testing.T) {
		malformedYAML := `
version: api-platform.wso2.com/v1
kind: http/rest
data:
  name: Test
    invalid indentation
`

		params := utils.APIDeploymentParams{
			Data:          []byte(malformedYAML),
			ContentType:   "application/yaml",
			CorrelationID: "test-malformed-1",
			Logger:        logger,
		}

		_, err := deploymentService.DeployAPIConfiguration(params)
		assert.Error(t, err, "Malformed YAML should fail to parse")
	})

	// Test: Invalid JSON
	t.Run("Invalid JSON", func(t *testing.T) {
		invalidJSON := `{"version": "api-platform.wso2.com/v1", "kind": "http/rest", "data": {invalid}}`

		params := utils.APIDeploymentParams{
			Data:          []byte(invalidJSON),
			ContentType:   "application/json",
			CorrelationID: "test-invalid-json-1",
			Logger:        logger,
		}

		_, err := deploymentService.DeployAPIConfiguration(params)
		assert.Error(t, err, "Invalid JSON should fail to parse")
	})
}
