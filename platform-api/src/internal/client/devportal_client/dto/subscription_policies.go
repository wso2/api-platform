package dto

// SubscriptionPolicy represents a subscription policy in DevPortal.
type SubscriptionPolicy struct {
	PolicyName   string           `json:"policyName"`
	DisplayName  string           `json:"displayName,omitempty"`
	Description  string           `json:"description,omitempty"`
	BillingPlan  string           `json:"billingPlan,omitempty"`
	Type         SubscriptionType `json:"type,omitempty"`
	RequestCount string           `json:"requestCount,omitempty"`
}
