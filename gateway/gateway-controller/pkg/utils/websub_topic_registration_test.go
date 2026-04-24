package utils

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

func TestDeployAPIConfigurationWebSubKindTopicRegistration(t *testing.T) {
	configStore := storage.NewConfigStore()
	var db storage.Storage
	snapshotManager := &xds.SnapshotManager{}
	validator := config.NewAPIValidator()
	service := newTestAPIDeploymentService(configStore, db, snapshotManager, validator, nil)

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
    - name: /topic1
      method: SUB
    - name: /topic2
      method: SUB
`

	// Build a StoredAPIConfig from the YAML
	var apiCfg api.WebSubAPI
	parser := config.NewParser()
	if err := parser.Parse([]byte(yamlConfig), "application/yaml", &apiCfg); err != nil {
		t.Fatalf("failed to parse inline yaml: %v", err)
	}

	cfg := &models.StoredConfig{
		UUID:          "0000-test-config-1-0000-000000000000",
		Kind:          string(api.WebSubAPIKindWebSubApi),
		Handle:        "testapi",
		DisplayName:   "testapi",
		Version:       "v1",
		Configuration: apiCfg,
		DesiredState:  models.StateDeployed,
		Origin:        models.OriginGatewayAPI,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		DeployedAt:    nil,
	}

	err := service.store.Add(cfg)
	if err != nil {
		t.Fatalf("failed to add config to store: %v", err)
	}
	t.Logf("topics after add: %v", configStore.TopicManager.GetAllForConfig())
	assert.True(t, configStore.TopicManager.IsTopicExist(cfg.UUID, "test_topic1"))
	assert.True(t, configStore.TopicManager.IsTopicExist(cfg.UUID, "test_topic2"))
}

func TestDeployAPIConfigurationWebSubKindRevisionDeployment(t *testing.T) {
	configStore := storage.NewConfigStore()
	validator := config.NewAPIValidator()
	service := newTestAPIDeploymentService(configStore, nil, nil, validator, nil)

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
    - name: /topic1
      method: SUB
    - name: /topic2
      method: SUB
`

	// Build a StoredAPIConfig from the YAML
	var apiCfg api.WebSubAPI
	parser := config.NewParser()
	if err := parser.Parse([]byte(yamlConfig), "application/yaml", &apiCfg); err != nil {
		t.Fatalf("failed to parse inline yaml: %v", err)
	}

	cfg := &models.StoredConfig{
		UUID:          "0000-test-config-1-0000-000000000000",
		Kind:          string(api.WebSubAPIKindWebSubApi),
		Handle:        "testapi",
		DisplayName:   "testapi",
		Version:       "v1",
		Configuration: apiCfg,
		DesiredState:  models.StateDeployed,
		Origin:        models.OriginGatewayAPI,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		DeployedAt:    nil,
	}

	err := service.store.Add(cfg)
	if err != nil {
		t.Fatalf("failed to add config to store: %v", err)
	}
	assert.True(t, configStore.TopicManager.IsTopicExist(cfg.UUID, "test_topic1"))
	assert.True(t, configStore.TopicManager.IsTopicExist(cfg.UUID, "test_topic2"))

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
    - name: /topic1
      method: SUB
`

	if err := parser.Parse([]byte(yamlConfig2), "application/yaml", &apiCfg); err != nil {
		t.Fatalf("failed to parse inline yaml: %v", err)
	}

	cfg, err = service.store.GetByKindNameAndVersion(models.KindWebSubApi, "testapi", "v1")
	if err != nil || cfg == nil {
		t.Fatalf("failed to get config from store: %v", err)
	}

	cfg.Configuration = apiCfg
	cfg.CreatedAt = time.Now()
	cfg.UpdatedAt = time.Now()
	cfg.DesiredState = models.StateDeployed
	cfg.DeployedAt = nil

	err = service.store.Update(cfg)
	if err != nil {
		t.Fatalf("failed to add config to store: %v", err)
	}
	assert.True(t, configStore.TopicManager.IsTopicExist(cfg.UUID, "test_topic1"))
	assert.False(t, configStore.TopicManager.IsTopicExist(cfg.UUID, "test_topic2"))
}

func TestTopicRegistrationForConcurrentAPIConfigs(t *testing.T) {
	configStore := storage.NewConfigStore()
	validator := config.NewAPIValidator()
	service := newTestAPIDeploymentService(configStore, nil, nil, validator, nil)

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
    - name: /topic1
      method: SUB
    - name: /topic2
      method: SUB`

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
    - name: /topic3
      method: SUB
    - name: /topic4
      method: SUB`

	var apiCfgA, apiCfgB api.WebSubAPI
	parser := config.NewParser()
	if err := parser.Parse([]byte(yamlA), "application/yaml", &apiCfgA); err != nil {
		t.Fatalf("failed to parse yamlA: %v", err)
	}
	if err := parser.Parse([]byte(yamlB), "application/yaml", &apiCfgB); err != nil {
		t.Fatalf("failed to parse yamlB: %v", err)
	}

	cfgA := &models.StoredConfig{
		UUID:          "0000-cfg-a-0000-000000000000",
		Kind:          string(api.WebSubAPIKindWebSubApi),
		Handle:        "testapiA",
		DisplayName:   "testapiA",
		Version:       "v1",
		Configuration: apiCfgA,
		DesiredState:  models.StateDeployed,
		Origin:        models.OriginGatewayAPI,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		DeployedAt:    nil,
	}

	cfgB := &models.StoredConfig{
		UUID:          "0000-cfg-b-0000-000000000000",
		Kind:          string(api.WebSubAPIKindWebSubApi),
		Handle:        "testapiB",
		DisplayName:   "testapiB",
		Version:       "v1",
		Configuration: apiCfgB,
		DesiredState:  models.StateDeployed,
		Origin:        models.OriginGatewayAPI,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		DeployedAt:    nil,
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
	assert.True(t, configStore.TopicManager.IsTopicExist(cfgA.UUID, "testA_topic1"))
	assert.True(t, configStore.TopicManager.IsTopicExist(cfgA.UUID, "testA_topic2"))

	// Verify topics for cfgB
	assert.True(t, configStore.TopicManager.IsTopicExist(cfgB.UUID, "testB_topic3"))
	assert.True(t, configStore.TopicManager.IsTopicExist(cfgB.UUID, "testB_topic4"))
}

func TestTopicDeregistrationOnConfigDeletion(t *testing.T) {
	configStore := storage.NewConfigStore()
	validator := config.NewAPIValidator()
	service := newTestAPIDeploymentService(configStore, nil, nil, validator, nil)

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
    - name: /topic1
      method: SUB
    - name: /topic2
      method: SUB`

	// Build a StoredAPIConfig from the YAML
	var apiCfg api.WebSubAPI
	parser := config.NewParser()
	if err := parser.Parse([]byte(yamlConfig), "application/yaml", &apiCfg); err != nil {
		t.Fatalf("failed to parse inline yaml: %v", err)
	}

	cfg := &models.StoredConfig{
		UUID:          "0000-test-config-1-0000-000000000000",
		Kind:          string(api.WebSubAPIKindWebSubApi),
		Handle:        "testapi",
		DisplayName:   "testapi",
		Version:       "v1",
		Configuration: apiCfg,
		DesiredState:  models.StateDeployed,
		Origin:        models.OriginGatewayAPI,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		DeployedAt:    nil,
	}

	err := service.store.Add(cfg)
	if err != nil {
		t.Fatalf("failed to add config to store: %v", err)
	}
	assert.True(t, configStore.TopicManager.IsTopicExist(cfg.UUID, "test_topic1"))
	assert.True(t, configStore.TopicManager.IsTopicExist(cfg.UUID, "test_topic2"))

	err = service.store.Delete(cfg.UUID)
	if err != nil {
		t.Fatalf("failed to delete config from store: %v", err)
	}

	assert.False(t, configStore.TopicManager.IsTopicExist(cfg.UUID, "test_topic1"))
	assert.False(t, configStore.TopicManager.IsTopicExist(cfg.UUID, "test_topic2"))
}
