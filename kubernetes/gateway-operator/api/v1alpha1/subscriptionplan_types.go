/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SubscriptionPlanSpec mirrors the management-API SubscriptionPlanCreateRequest
// payload. The gateway-controller assigns a UUID id on first POST; the
// controller persists it to .status.id and uses it for subsequent
// PUT/DELETE.
type SubscriptionPlanSpec struct {
	// PlanName is the human-readable plan name.
	// +kubebuilder:validation:Required
	PlanName string `json:"planName"`

	// BillingPlan is an optional billing plan identifier.
	// +optional
	BillingPlan *string `json:"billingPlan,omitempty"`

	// ExpiryTime is the optional plan expiry.
	// +optional
	ExpiryTime *metav1.Time `json:"expiryTime,omitempty"`

	// Status is the lifecycle state for this plan (active/inactive/etc).
	// Mirrors the management-API SubscriptionPlanCreateRequest.Status enum.
	// +optional
	// +kubebuilder:validation:Enum=ACTIVE;INACTIVE
	Status *string `json:"status,omitempty"`

	// StopOnQuotaReach controls whether traffic is blocked when the quota
	// is exhausted.
	// +optional
	StopOnQuotaReach *bool `json:"stopOnQuotaReach,omitempty"`

	// ThrottleLimitCount is the request count limit.
	// +optional
	// +kubebuilder:validation:Minimum=0
	ThrottleLimitCount *int64 `json:"throttleLimitCount,omitempty"`

	// ThrottleLimitUnit is the time unit for the throttle window.
	// +optional
	// +kubebuilder:validation:Enum=Day;Hour;Min;Month
	ThrottleLimitUnit *string `json:"throttleLimitUnit,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=subscriptionplans,singular=subscriptionplan,shortName=splan
//+kubebuilder:printcolumn:name="Id",type=string,JSONPath=`.status.id`
//+kubebuilder:printcolumn:name="Plan",type=string,JSONPath=`.spec.planName`

// SubscriptionPlan is the Schema for the subscriptionplans API.
type SubscriptionPlan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SubscriptionPlanSpec `json:"spec,omitempty"`
	Status ResourceStatus       `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SubscriptionPlanList contains a list of SubscriptionPlan.
type SubscriptionPlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SubscriptionPlan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SubscriptionPlan{}, &SubscriptionPlanList{})
}
