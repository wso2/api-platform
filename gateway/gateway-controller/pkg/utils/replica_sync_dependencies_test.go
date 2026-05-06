package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

func TestRequireReplicaSyncWiring(t *testing.T) {
	t.Run("panics without event hub", func(t *testing.T) {
		assert.PanicsWithValue(t, "component requires non-nil EventHub", func() {
			requireReplicaSyncWiring("component", nil, "gateway-1")
		})
	})

	t.Run("panics without gateway ID", func(t *testing.T) {
		assert.PanicsWithValue(t, "component requires non-empty gateway ID", func() {
			requireReplicaSyncWiring("component", newReplicaSyncTestEventHub(), "   ")
		})
	})

	t.Run("returns trimmed gateway ID", func(t *testing.T) {
		assert.Equal(t, "gateway-1", requireReplicaSyncWiring("component", newReplicaSyncTestEventHub(), " gateway-1 "))
	})
}

func TestConstructorReplicaSyncWiring(t *testing.T) {
	store := storage.NewConfigStore()
	db := newTestMockDB()
	apiKeyConfig := &config.APIKeyConfig{}

	t.Run("api deployment stores constructor wiring", func(t *testing.T) {
		service := NewAPIDeploymentService(store, db, nil, nil, nil, newReplicaSyncTestEventHub(), " gateway-1 ", nil)
		require.NotNil(t, service.eventHub)
		assert.Equal(t, "gateway-1", service.gatewayID)
	})

	t.Run("api key stores constructor wiring", func(t *testing.T) {
		service := NewAPIKeyService(store, db, nil, apiKeyConfig, newReplicaSyncTestEventHub(), " gateway-2 ")
		require.NotNil(t, service.eventHub)
		assert.Equal(t, "gateway-2", service.gatewayID)
	})

	t.Run("mcp deployment stores constructor wiring", func(t *testing.T) {
		service := NewMCPDeploymentService(store, db, nil, nil, nil, newReplicaSyncTestEventHub(), " gateway-3 ", nil)
		require.NotNil(t, service.eventHub)
		assert.Equal(t, "gateway-3", service.gatewayID)
	})

	t.Run("subscription resource stores constructor wiring", func(t *testing.T) {
		service := NewSubscriptionResourceService(db, nil, newReplicaSyncTestEventHub(), " gateway-4 ")
		require.NotNil(t, service.eventHub)
		assert.Equal(t, "gateway-4", service.gatewayID)
	})
}
