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

// SubscriptionSpec mirrors the management-API SubscriptionCreateRequest
// payload. The gateway-controller issues a UUID id which the controller
// persists to .status.id for subsequent PUT/DELETE.
type SubscriptionSpec struct {
	// ApiId is the API identifier (deployment id or handle).
	// +kubebuilder:validation:Required
	ApiId string `json:"apiId"`

	// SubscriptionToken is the opaque token used to invoke the API. The
	// token is sent as plaintext to the gateway which stores only its hash;
	// inline in the CR or supplied via valueFrom.
	// +kubebuilder:validation:Required
	SubscriptionToken SecretValueSource `json:"subscriptionToken"`

	// ApplicationId is an optional application identifier.
	// +optional
	ApplicationId *string `json:"applicationId,omitempty"`

	// SubscriptionPlanId is an optional plan UUID. When the plan is also
	// managed via SubscriptionPlan CR, set this to the SubscriptionPlan's
	// .status.id once it has been deployed.
	// +optional
	SubscriptionPlanId *string `json:"subscriptionPlanId,omitempty"`

	// BillingCustomerId is an optional billing customer identifier.
	// +optional
	BillingCustomerId *string `json:"billingCustomerId,omitempty"`

	// BillingSubscriptionId is an optional billing subscription identifier.
	// +optional
	BillingSubscriptionId *string `json:"billingSubscriptionId,omitempty"`

	// Status is the lifecycle state for this subscription. Mirrors the
	// management-API SubscriptionCreateRequest.Status enum.
	// +optional
	Status *string `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=subscriptions,singular=subscription,shortName=sub
//+kubebuilder:printcolumn:name="Id",type=string,JSONPath=`.status.id`
//+kubebuilder:printcolumn:name="ApiId",type=string,JSONPath=`.spec.apiId`

// Subscription is the Schema for the subscriptions API.
type Subscription struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SubscriptionSpec `json:"spec"`
	Status ResourceStatus   `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SubscriptionList contains a list of Subscription.
type SubscriptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Subscription `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Subscription{}, &SubscriptionList{})
}
