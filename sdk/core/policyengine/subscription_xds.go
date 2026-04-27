package policyengine

import "time"

// SubscriptionData represents a single subscription entry for xDS transmission.
type SubscriptionData struct {
	// APIId identifies the API this subscription belongs to.
	APIId string `json:"apiId" yaml:"apiId"`

	// ApplicationId identifies the subscribed application (opaque id from STS/DevPortal).
	// Optional in the new design; may be empty when using token-based subscriptions.
	ApplicationId string `json:"applicationId,omitempty" yaml:"applicationId,omitempty"`

	// SubscriptionToken is the opaque token identifying this subscription.
	SubscriptionToken string `json:"subscriptionToken" yaml:"subscriptionToken"`

	// Status is the subscription status (e.g. ACTIVE, INACTIVE, REVOKED).
	Status string `json:"status" yaml:"status"`

	// ThrottleLimitCount is the number of requests allowed per throttle window (from plan).
	ThrottleLimitCount int `json:"throttleLimitCount,omitempty" yaml:"throttleLimitCount,omitempty"`

	// ThrottleLimitUnit is the throttle window duration unit (Min, Hour, Day, Month).
	ThrottleLimitUnit string `json:"throttleLimitUnit,omitempty" yaml:"throttleLimitUnit,omitempty"`

	// StopOnQuotaReach indicates whether to block requests when quota is exhausted.
	StopOnQuotaReach bool `json:"stopOnQuotaReach,omitempty" yaml:"stopOnQuotaReach,omitempty"`

	// PlanName is the name of the subscription plan
	PlanName string `json:"planName,omitempty" yaml:"planName,omitempty"`

	// BillingCustomerId is the billing customer identifier
	BillingCustomerId *string `json:"billingCustomerId,omitempty" yaml:"billingCustomerId,omitempty"`

	// BillingSubscriptionId is the billing subscription identifier
	BillingSubscriptionId *string `json:"billingSubscriptionId,omitempty" yaml:"billingSubscriptionId,omitempty"`
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
