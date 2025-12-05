package utils

import (
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
	//store := &storage.ConfigStore{TopicManager: topicManager}
	db := &storage.SQLiteStorage{}
	snapshotManager := &xds.SnapshotManager{}
	validator := config.NewAPIValidator()
	service := NewAPIDeploymentService(configStore, db, snapshotManager, validator)

	// Inline YAML config similar to websubhub.yaml
	yamlConfig := `kind: async/websub
version: api-platform.wso2.com/v1
spec:
  apiType: async/websub
  name: testapi
  context: /test
  version: v1
  servers:
    - url: "http://host.docker.internal:9098"
      protocol: websub
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

	cfg := &models.StoredAPIConfig{
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
	//store := &storage.ConfigStore{TopicManager: topicManager}
	db := &storage.SQLiteStorage{}
	snapshotManager := &xds.SnapshotManager{}
	validator := config.NewAPIValidator()
	service := NewAPIDeploymentService(configStore, db, snapshotManager, validator)

	// Inline YAML config similar to websubhub.yaml
	yamlConfig := `kind: async/websub
version: api-platform.wso2.com/v1
spec:
  apiType: async/websub
  name: testapi
  context: /test
  version: v1
  servers:
    - url: "http://host.docker.internal:9098"
      protocol: websub
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

	cfg := &models.StoredAPIConfig{
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
	yamlConfig2 := `kind: async/websub
version: api-platform.wso2.com/v1
spec:
  apiType: async/websub
  name: testapi
  context: /test
  version: v1
  servers:
    - url: "http://host.docker.internal:9098"
      protocol: websub
  channels:
    - path: /topic1
`

	if err := parser.Parse([]byte(yamlConfig2), "application/yaml", &apiCfg); err != nil {
		t.Fatalf("failed to parse inline yaml: %v", err)
	}

	// cfg = &models.StoredAPIConfig{
	// 	ID:              "test-config-1",
	// 	Configuration:   apiCfg,
	// 	Status:          models.StatusPending,
	// 	CreatedAt:       time.Now(),
	// 	UpdatedAt:       time.Now(),
	// 	DeployedAt:      nil,
	// 	DeployedVersion: 0,
	// }

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
