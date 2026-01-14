package utils

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

func TestDeployAPIConfigurationWebSubKindTopicRegistration(t *testing.T) {
	configStore := storage.NewConfigStore()
	db := &storage.SQLiteStorage{}
	snapshotManager := &xds.SnapshotManager{}
	validator := config.NewAPIValidator()
	service := NewAPIDeploymentService(configStore, db, snapshotManager, validator, nil)

	// Inline YAML config similar to websubhub.yaml
	yamlConfig := `kind: WebSubApi
apiVersion: gateway.api-platform.wso2.com/v1alpha1
metadata:
  name: testapi
spec:
  displayName: testapi
  context: /test
  version: v1
  vhosts:
    main: "*"
  channels:
    - path: /topic1
    - path: /topic2
`

	// Build a StoredAPIConfig from the YAML
	var apiCfg api.APIConfiguration
	parser := config.NewParser()
	if err := parser.Parse([]byte(yamlConfig), "application/yaml", &apiCfg); err != nil {
		t.Fatalf("failed to parse inline yaml: %v", err)
	}

	cfg := &models.StoredConfig{
		ID:              "test-config-1",
		Configuration:   apiCfg,
		Status:          models.StatusPending,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		DeployedAt:      nil,
		DeployedVersion: 0,
	}

	err := service.store.Add(cfg)
	if err != nil {
		t.Fatalf("failed to add config to store: %v", err)
	}
	assert.True(t, configStore.TopicManager.IsTopicExist(cfg.ID, "testapi_test_v1_topic1"))
	assert.True(t, configStore.TopicManager.IsTopicExist(cfg.ID, "testapi_test_v1_topic2"))
}

func TestDeployAPIConfigurationWebSubKindRevisionDeployment(t *testing.T) {
	configStore := storage.NewConfigStore()
	validator := config.NewAPIValidator()
	service := NewAPIDeploymentService(configStore, nil, nil, validator, nil)

	// Inline YAML config similar to websubhub.yaml
	yamlConfig := `kind: WebSubApi
apiVersion: gateway.api-platform.wso2.com/v1alpha1
metadata:
  name: testapi
spec:
  displayName: testapi
  context: /test
  version: v1
  vhosts:
    main: "*"
  channels:
    - path: /topic1
    - path: /topic2
`

	// Build a StoredAPIConfig from the YAML
	var apiCfg api.APIConfiguration
	parser := config.NewParser()
	if err := parser.Parse([]byte(yamlConfig), "application/yaml", &apiCfg); err != nil {
		t.Fatalf("failed to parse inline yaml: %v", err)
	}

	cfg := &models.StoredConfig{
		ID:              "test-config-1",
		Configuration:   apiCfg,
		Status:          models.StatusPending,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		DeployedAt:      nil,
		DeployedVersion: 0,
	}

	err := service.store.Add(cfg)
	if err != nil {
		t.Fatalf("failed to add config to store: %v", err)
	}
	assert.True(t, configStore.TopicManager.IsTopicExist(cfg.ID, "testapi_test_v1_topic1"))
	assert.True(t, configStore.TopicManager.IsTopicExist(cfg.ID, "testapi_test_v1_topic2"))

	// Second deployment with topic2 removed -> should deregister topic2
	yamlConfig2 := `kind: WebSubApi
apiVersion: gateway.api-platform.wso2.com/v1alpha1
metadata:
  name: testapi
spec:
  displayName: testapi
  context: /test
  version: v1
  vhosts:
    main: "*"
  channels:
    - path: /topic1
`

	if err := parser.Parse([]byte(yamlConfig2), "application/yaml", &apiCfg); err != nil {
		t.Fatalf("failed to parse inline yaml: %v", err)
	}

	cfg, err = service.store.GetByNameVersion("testapi", "v1")
	if err != nil {
		t.Fatalf("failed to get config from store: %v", err)
	}

	cfg.Configuration = apiCfg
	cfg.CreatedAt = time.Now()
	cfg.UpdatedAt = time.Now()
	cfg.DeployedVersion += 1
	cfg.Status = models.StatusPending
	cfg.DeployedAt = nil

	err = service.store.Update(cfg)
	if err != nil {
		t.Fatalf("failed to add config to store: %v", err)
	}
	assert.True(t, configStore.TopicManager.IsTopicExist(cfg.ID, "testapi_test_v1_topic1"))
	assert.False(t, configStore.TopicManager.IsTopicExist(cfg.ID, "testapi_test_v1_topic2"))
}

func TestTopicRegistrationForConcurrentAPIConfigs(t *testing.T) {
	configStore := storage.NewConfigStore()
	validator := config.NewAPIValidator()
	service := NewAPIDeploymentService(configStore, nil, nil, validator, nil)

	// Two different API YAMLs
	yamlA := `kind: WebSubApi
apiVersion: gateway.api-platform.wso2.com/v1alpha1
metadata:
  name: testapiA
spec:
  displayName: testapiA
  context: /testA
  version: v1
  vhosts:
    main: "*"
  channels:
    - path: /topic1
    - path: /topic2
`

	yamlB := `kind: WebSubApi
apiVersion: gateway.api-platform.wso2.com/v1alpha1
metadata:
  name: testapiB
spec:
  displayName: testapiB
  context: /testB
  version: v1
  vhosts:
    main: "*"
  channels:
    - path: /topic3
    - path: /topic4
`

	var apiCfgA, apiCfgB api.APIConfiguration
	parser := config.NewParser()
	if err := parser.Parse([]byte(yamlA), "application/yaml", &apiCfgA); err != nil {
		t.Fatalf("failed to parse yamlA: %v", err)
	}
	if err := parser.Parse([]byte(yamlB), "application/yaml", &apiCfgB); err != nil {
		t.Fatalf("failed to parse yamlB: %v", err)
	}

	cfgA := &models.StoredConfig{
		ID:              "cfg-a",
		Configuration:   apiCfgA,
		Status:          models.StatusPending,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		DeployedAt:      nil,
		DeployedVersion: 0,
	}

	cfgB := &models.StoredConfig{
		ID:              "cfg-b",
		Configuration:   apiCfgB,
		Status:          models.StatusPending,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		DeployedAt:      nil,
		DeployedVersion: 0,
	}

	var wg sync.WaitGroup
	wg.Add(2)

	var errA, errB error

	go func() {
		defer wg.Done()
		if err := service.store.Add(cfgA); err != nil {
			errA = err
		}
	}()

	go func() {
		defer wg.Done()
		if err := service.store.Add(cfgB); err != nil {
			errB = err
		}
	}()

	wg.Wait()

	if errA != nil {
		t.Fatalf("failed to add cfgA: %v", errA)
	}
	if errB != nil {
		t.Fatalf("failed to add cfgB: %v", errB)
	}

	// Verify topics for cfgA
	assert.True(t, configStore.TopicManager.IsTopicExist(cfgA.ID, "testapiA_testA_v1_topic1"))
	assert.True(t, configStore.TopicManager.IsTopicExist(cfgA.ID, "testapiA_testA_v1_topic2"))

	// Verify topics for cfgB
	assert.True(t, configStore.TopicManager.IsTopicExist(cfgB.ID, "testapiB_testB_v1_topic3"))
	assert.True(t, configStore.TopicManager.IsTopicExist(cfgB.ID, "testapiB_testB_v1_topic4"))
}

func TestTopicDeregistrationOnConfigDeletion(t *testing.T) {
	configStore := storage.NewConfigStore()
	validator := config.NewAPIValidator()
	service := NewAPIDeploymentService(configStore, nil, nil, validator, nil)

	// Inline YAML config similar to websubhub.yaml
	yamlConfig := `kind: WebSubApi
apiVersion: gateway.api-platform.wso2.com/v1alpha1
metadata:
  name: testapi
spec:
  displayName: testapi
  context: /test
  version: v1
  vhosts:
    main: "*"
  channels:
    - path: /topic1
    - path: /topic2
`

	// Build a StoredAPIConfig from the YAML
	var apiCfg api.APIConfiguration
	parser := config.NewParser()
	if err := parser.Parse([]byte(yamlConfig), "application/yaml", &apiCfg); err != nil {
		t.Fatalf("failed to parse inline yaml: %v", err)
	}

	cfg := &models.StoredConfig{
		ID:              "test-config-1",
		Configuration:   apiCfg,
		Status:          models.StatusPending,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		DeployedAt:      nil,
		DeployedVersion: 0,
	}

	err := service.store.Add(cfg)
	if err != nil {
		t.Fatalf("failed to add config to store: %v", err)
	}
	assert.True(t, configStore.TopicManager.IsTopicExist(cfg.ID, "testapi_test_v1_topic1"))
	assert.True(t, configStore.TopicManager.IsTopicExist(cfg.ID, "testapi_test_v1_topic2"))

	err = service.store.Delete(cfg.ID)
	if err != nil {
		t.Fatalf("failed to delete config from store: %v", err)
	}

	assert.False(t, configStore.TopicManager.IsTopicExist(cfg.ID, "testapi_test_v1_topic1"))
	assert.False(t, configStore.TopicManager.IsTopicExist(cfg.ID, "testapi_test_v1_topic2"))
}
