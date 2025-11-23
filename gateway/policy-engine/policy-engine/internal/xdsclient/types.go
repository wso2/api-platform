package xdsclient

import (
	"time"
)

const (
	// PolicyChainTypeURL is the type URL for PolicyChainConfig resources
	// Uses WSO2 API Platform domain for custom resource types
	PolicyChainTypeURL = "api-platform.wso2.org/v1.PolicyChainConfig"

	// Default configuration values
	DefaultNodeID          = "policy-engine"
	DefaultCluster         = "policy-engine-cluster"
	DefaultConnectTimeout  = 10 * time.Second
	DefaultRequestTimeout  = 5 * time.Second
	DefaultMaxReconnectDelay = 60 * time.Second
	DefaultInitialReconnectDelay = 1 * time.Second
)

// ClientState represents the current state of the xDS client
type ClientState int

const (
	StateDisconnected ClientState = iota
	StateConnecting
	StateConnected
	StateReconnecting
	StateStopped
)

func (s ClientState) String() string {
	switch s {
	case StateDisconnected:
		return "Disconnected"
	case StateConnecting:
		return "Connecting"
	case StateConnected:
		return "Connected"
	case StateReconnecting:
		return "Reconnecting"
	case StateStopped:
		return "Stopped"
	default:
		return "Unknown"
	}
}
