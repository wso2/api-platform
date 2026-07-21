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

// CertificateSpec mirrors the management-API CertificateUploadRequest, with
// the PEM bytes expressed as a SecretValueSource so production-grade
// certificates can be referenced from a Kubernetes Secret rather than
// inlined into the CR.
//
// The gateway-controller assigns a UUID id on first upload that the
// controller persists to .status.id; subsequent reconcile uses
// `/certificates/{status.id}` for PUT/DELETE.
type CertificateSpec struct {
	// DisplayName is a human-readable certificate name. The controller
	// passes it through to the management API as `name`.
	// +kubebuilder:validation:Required
	DisplayName string `json:"displayName"`

	// Certificate is the PEM-encoded X.509 certificate (single or bundle).
	// +kubebuilder:validation:Required
	Certificate SecretValueSource `json:"certificate"`
}

//+kubebuilder:object:root=true
//+kubebuilder:storageversion
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=certificates,singular=certificate,shortName=cert

// Certificate is the Schema for the certificates API.
type Certificate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CertificateSpec `json:"spec,omitempty"`
	Status ResourceStatus  `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CertificateList contains a list of Certificate.
type CertificateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Certificate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Certificate{}, &CertificateList{})
}
