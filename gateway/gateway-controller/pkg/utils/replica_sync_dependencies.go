package utils

import (
	"strings"

	"github.com/wso2/api-platform/common/eventhub"
)

func requireReplicaSyncWiring(component string, eventHub eventhub.EventHub, gatewayID string) string {
	if eventHub == nil {
		panic(component + " requires non-nil EventHub")
	}

	trimmedGatewayID := strings.TrimSpace(gatewayID)
	if trimmedGatewayID == "" {
		panic(component + " requires non-empty gateway ID")
	}

	return trimmedGatewayID
}
