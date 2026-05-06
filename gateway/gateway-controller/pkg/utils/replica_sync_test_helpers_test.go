package utils

import (
	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
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
) *APIDeploymentService {
	return newTestAPIDeploymentServiceWithHub(
		store,
		db,
		snapshotManager,
		validator,
		routerConfig,
		newReplicaSyncTestEventHub(),
		testGatewayID,
	)
}

func newTestAPIDeploymentServiceWithHub(
	store *storage.ConfigStore,
	db storage.Storage,
	snapshotManager *xds.SnapshotManager,
	validator config.Validator,
	routerConfig *config.RouterConfig,
	hub eventhub.EventHub,
	gatewayID string,
) *APIDeploymentService {
	return NewAPIDeploymentService(
		store,
		newReplicaSyncTestDB(db),
		snapshotManager,
		validator,
		routerConfig,
		hub,
		gatewayID,
		nil,
	)
}

func newTestAPIKeyService(
	store *storage.ConfigStore,
	db storage.Storage,
	xdsManager XDSManager,
	apiKeyConfig *config.APIKeyConfig,
) *APIKeyService {
	return NewAPIKeyService(store, newReplicaSyncTestDB(db), xdsManager, apiKeyConfig, newReplicaSyncTestEventHub(), testGatewayID)
}

func newTestMCPDeploymentService(
	store *storage.ConfigStore,
	db storage.Storage,
	snapshotManager *xds.SnapshotManager,
	policyManager *policyxds.PolicyManager,
	policyValidator *config.PolicyValidator,
) *MCPDeploymentService {
	return newTestMCPDeploymentServiceWithHub(
		store,
		db,
		snapshotManager,
		policyManager,
		policyValidator,
		newReplicaSyncTestEventHub(),
		testGatewayID,
	)
}

func newTestMCPDeploymentServiceWithHub(
	store *storage.ConfigStore,
	db storage.Storage,
	snapshotManager *xds.SnapshotManager,
	policyManager *policyxds.PolicyManager,
	policyValidator *config.PolicyValidator,
	hub eventhub.EventHub,
	gatewayID string,
) *MCPDeploymentService {
	return NewMCPDeploymentService(
		store,
		newReplicaSyncTestDB(db),
		snapshotManager,
		policyManager,
		policyValidator,
		hub,
		gatewayID,
		nil,
	)
}
