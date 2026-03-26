package utils

import (
	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/resolver"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

const testGatewayID = "test-gateway"

func newReplicaSyncTestEventHub() eventhub.EventHub {
	return &mockLLMEventHub{}
}

func newReplicaSyncTestDB(db storage.Storage) storage.Storage {
	if db != nil {
		return db
	}
	return newTestMockDB()
}

func newTestAPIDeploymentService(
	store *storage.ConfigStore,
	db storage.Storage,
	snapshotManager *xds.SnapshotManager,
	validator config.Validator,
	routerConfig *config.RouterConfig,
	policyResolver *resolver.PolicyResolver,
) *APIDeploymentService {
	service := NewAPIDeploymentService(store, newReplicaSyncTestDB(db), snapshotManager, validator, routerConfig, policyResolver)
	service.SetEventHub(newReplicaSyncTestEventHub(), testGatewayID)
	return service
}

func newTestAPIKeyService(
	store *storage.ConfigStore,
	db storage.Storage,
	xdsManager XDSManager,
	apiKeyConfig *config.APIKeyConfig,
) *APIKeyService {
	service := NewAPIKeyService(store, newReplicaSyncTestDB(db), xdsManager, apiKeyConfig)
	service.SetEventHub(newReplicaSyncTestEventHub(), testGatewayID)
	return service
}

func newTestMCPDeploymentService(
	store *storage.ConfigStore,
	db storage.Storage,
	snapshotManager *xds.SnapshotManager,
	policyManager *policyxds.PolicyManager,
	policyValidator *config.PolicyValidator,
) *MCPDeploymentService {
	service := NewMCPDeploymentService(store, newReplicaSyncTestDB(db), snapshotManager, policyManager, policyValidator)
	service.SetEventHub(newReplicaSyncTestEventHub(), testGatewayID)
	return service
}
