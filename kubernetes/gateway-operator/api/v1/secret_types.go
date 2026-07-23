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
)

// SecretSpec mirrors the management-API SecretConfigData payload, with the
// sensitive Value field expressed as a SecretValueSource so operators can
// keep plaintext out of the CR by referencing a Kubernetes Secret.
type SecretSpec struct {
	// DisplayName is a human-readable secret name.
	// +kubebuilder:validation:Required
	DisplayName string `json:"displayName"`

	// Description is an optional description of the secret.
	// +optional
	Description *string `json:"description,omitempty"`

	// Value is the secret plaintext, supplied either inline or via valueFrom.
	// +kubebuilder:validation:Required
	Value SecretValueSource `json:"value"`
}

//+kubebuilder:object:root=true
//+kubebuilder:storageversion
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=managedsecrets,singular=managedsecret,shortName=msec

// ManagedSecret represents a platform secret stored by the gateway-controller.
//
// The CRD is named ManagedSecret (rather than Secret) so it does not collide
// with the built-in core/v1 Secret kind. The plural "managedsecrets" is used
// in URLs and RBAC to remain unambiguous.
type ManagedSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecretSpec     `json:"spec,omitempty"`
	Status ResourceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ManagedSecretList contains a list of ManagedSecret.
type ManagedSecretList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ManagedSecret `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ManagedSecret{}, &ManagedSecretList{})
}
