package dto

// SubscriptionType is a typed string for subscription policy types used by DTOs.
type SubscriptionType string

const (
	// SubscriptionTypeRequestCount indicates a request-count based policy.
	SubscriptionTypeRequestCount SubscriptionType = "requestCount"
)

// Visibility indicates API visibility in DevPortal.
type APIVisibility string

const (
	APIVisibilityPublic  APIVisibility = "PUBLIC"
	APIVisibilityPrivate APIVisibility = "PRIVATE"
)

// APIStatus contains API lifecycle/status values.
type APIStatus string

const (
	APIStatusPublished   APIStatus = "PUBLISHED"
	APIStatusUnpublished APIStatus = "CREATED"
)

// APIType contains API type identifiers.
type APIType string

const (
	APITypeMCP        APIType = "MCP"
	APITypeMCPOnly    APIType = "MCPSERVERSONLY"
	APITypeAPIProxies APIType = "APISONLY"
	APITypeDefault    APIType = "DEFAULT"
	APITypeWS         APIType = "WS"
)
