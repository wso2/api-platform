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

// APIPolicyTargetRef identifies the HTTPRoute this APIPolicy is associated with.
// Per-rule / per-resource application is not configured on APIPolicy: it is determined only by
// where the policy is referenced from HTTPRoute rules (e.g. rules[].filters ExtensionRef).
type APIPolicyTargetRef struct {
	// Group of the referent (e.g. gateway.networking.k8s.io).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Enum=gateway.networking.k8s.io
	Group string `json:"group"`
	// Kind of the referent. Use HTTPRoute for Gateway API integration.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Enum=HTTPRoute
	Kind string `json:"kind"`
	// Name of the referent HTTPRoute.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Namespace of the referent; defaults to the APIPolicy's namespace if unset.
	// +optional
	Namespace *string `json:"namespace,omitempty"`
}

// APIPolicySpec holds a shared targetRef and a list of policy instances (same logical shape as RestApi embedded policies).
type APIPolicySpec struct {
	// TargetRef selects the HTTPRoute this resource is associated with (required). It applies to all entries in policies.
	// +kubebuilder:validation:Required
	TargetRef APIPolicyTargetRef `json:"targetRef"`
	// Policies is the list of policy instances (name, version, optional params, executionCondition) applied together
	// for this APIPolicy object (API-level via label, or rule scope via ExtensionRef).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Policies []Policy `json:"policies"`
}

// APIPolicyStatus defines observed state.
type APIPolicyStatus struct {
	// Conditions represent the latest observations.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=apipolicies,singular=apipolicy,shortName=apol
//+kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.targetRef.name`
//+kubebuilder:printcolumn:name="First",type=string,JSONPath=`.spec.policies[0].name`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// APIPolicy is a namespaced policy definition for the Gateway API HTTPRoute integration only.
// Reference it from HTTPRoute rule filters (ExtensionRef); that placement determines which rule
// / match scope receives the policies. It does not apply to RestApi / APIGateway reconciliation.
// (Distinct from the embedded Policy type on RestApi.)
type APIPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   APIPolicySpec   `json:"spec"`
	Status APIPolicyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// APIPolicyList contains a list of APIPolicy.
type APIPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []APIPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&APIPolicy{}, &APIPolicyList{})
}
