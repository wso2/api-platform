package utils

import (
	"fmt"
	"net/http"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	"github.com/wso2/api-platform/httpkit/httpclient"
)

const testGatewayID = "test-gateway"

// testHTTPClient builds a plain outbound *http.Client for tests via httpkit's
// secure-by-default builder. Production code injects one single shared client built in
// cmd/controller/main.go; tests build their own throwaway instance since there is no shared
// process-level client to inject.
func testHTTPClient() *http.Client {
	client, err := httpclient.New(httpclient.DefaultConfig())
	if err != nil {
		panic(fmt.Sprintf("test HTTP client: unreachable construction error for a fixed default config: %v", err))
	}
	return client
}

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
		testHTTPClient(),
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
		nil,
	)
}
