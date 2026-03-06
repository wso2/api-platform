package policyenginev1

import "time"

// SubscriptionData represents a single subscription entry for xDS transmission.
type SubscriptionData struct {
	// APIId identifies the API this subscription belongs to.
	APIId string `json:"apiId" yaml:"apiId"`

	// ApplicationId identifies the subscribed application (opaque id from STS/DevPortal).
	ApplicationId string `json:"applicationId" yaml:"applicationId"`

	// Status is the subscription status (e.g. ACTIVE, INACTIVE, REVOKED).
	Status string `json:"status" yaml:"status"`
}

// SubscriptionStateResource represents the complete state of subscriptions
// that is sent from gateway-controller to the policy-engine via xDS.
type SubscriptionStateResource struct {
	// Subscriptions is the list of all subscriptions known to the gateway.
	Subscriptions []SubscriptionData `json:"subscriptions" yaml:"subscriptions"`

	// Version is a monotonically increasing version for this snapshot.
	Version int64 `json:"version" yaml:"version"`

	// Timestamp records when this snapshot was generated.
	Timestamp time.Time `json:"timestamp" yaml:"timestamp"`
}
