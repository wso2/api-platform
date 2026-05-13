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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretValueSource carries a sensitive string that may be either provided
// inline or sourced from a Kubernetes Secret.
//
// Exactly one of Value or ValueFrom must be set; controllers reject CRs that
// set neither or both.
//
// +kubebuilder:validation:XValidation:rule="has(self.value) != has(self.valueFrom)",message="exactly one of value or valueFrom must be set"
type SecretValueSource struct {
	// Value is the inline plaintext value. Avoid for production use; prefer
	// ValueFrom so the secret is stored in a Kubernetes Secret.
	// +optional
	Value *string `json:"value,omitempty"`

	// ValueFrom selects a key from a Kubernetes Secret in the same namespace
	// as the owning CR.
	// +optional
	ValueFrom *corev1.SecretKeySelector `json:"valueFrom,omitempty"`
}

// ResourceStatus carries the controller-managed lifecycle fields shared by
// the new management-API CRDs.
//
// For UUID-keyed kinds (Subscription, SubscriptionPlan, Certificate) the
// gateway-controller assigns Id on first deploy; controllers persist it and
// use it to address the resource for subsequent update/delete calls.
type ResourceStatus struct {
	// Conditions represent the latest available observations of the
	// resource's state. The standard types are Accepted and Programmed.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Id is the gateway-issued identifier for UUID-keyed resources. When
	// set the controller addresses the resource via PUT/DELETE /<plural>/{id}.
	// +optional
	Id string `json:"id,omitempty"`

	// LastUpdateTime is the last time the status was updated.
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`
}

// Common condition types shared across the new management-API CRDs. They
// mirror the constants on RestApi so operators can reuse the same logic.
const (
	// ConditionAccepted indicates the CR passed validation and is accepted
	// for processing.
	ConditionAccepted = "Accepted"
	// ConditionProgrammed indicates the resource is successfully deployed
	// to the gateway.
	ConditionProgrammed = "Programmed"
)

// Accepted condition reasons.
const (
	ReasonAccepted             = "Accepted"
	ReasonInvalidConfiguration = "InvalidConfiguration"
	ReasonPending              = "Pending"
)

// Programmed condition reasons.
const (
	ReasonProgrammed        = "Programmed"
	ReasonProgrammedPending = "Pending"
	ReasonInvalid           = "Invalid"
	ReasonGatewayNotReady   = "GatewayNotReady"
	ReasonDeploymentFailed  = "DeploymentFailed"
	ReasonRetrying          = "Retrying"
)
