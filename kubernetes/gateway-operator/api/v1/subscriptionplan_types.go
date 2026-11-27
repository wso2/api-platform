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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// SubscriptionPlanCondition is one condition of a subscription plan. The
// gateway-controller checks the params and the per-type rules (one condition
// of each built-in type, fixed ids, 1-10 limit windows).
// +kubebuilder:validation:XValidation:rule="self.type != 'custom' || has(self.id)",message="a custom condition needs an id"
type SubscriptionPlanCondition struct {
	// Id names the condition. Built-in types have fixed ids (requests,
	// tool-calls, tokens) and may omit it; custom conditions need one.
	// +optional
	// +kubebuilder:validation:MaxLength=63
	Id string `json:"id,omitempty"`

	// Type is the condition type.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=request-count;tool-call-count;token-count;custom
	Type string `json:"type"`

	// Params holds the condition's settings. Built-in types take
	// {"limits": [{"count", "timeAmount", "timeUnit"}], "onExceed"} (token-count
	// also takes "tokenType"); custom conditions take free-form key-values.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	Params *runtime.RawExtension `json:"params"`
}

// SubscriptionPlanSpec mirrors the management-API SubscriptionPlanCreateRequest
// payload. The gateway-controller assigns a UUID id on first POST; the
// controller persists it to .status.id and uses it for subsequent
// PUT/DELETE.
// +kubebuilder:validation:XValidation:rule="!has(self.conditions) || (!has(self.throttleLimitCount) && !has(self.throttleLimitUnit))",message="set conditions or throttleLimitCount/throttleLimitUnit, not both"
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

	// Conditions are the plan's limits and custom conditions. When set, the
	// gateway derives the legacy throttle fields from the request-count
	// condition, so leave throttleLimitCount/throttleLimitUnit unset.
	// +optional
	// +kubebuilder:validation:MaxItems=20
	// +listType=atomic
	Conditions []SubscriptionPlanCondition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:storageversion
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
