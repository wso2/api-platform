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

// ApiKeyParentRef identifies the parent resource that owns this API key.
// Supported parents map to nested management-API paths:
//
//   - RestApi     -> /rest-apis/{name}/api-keys/...
//   - LlmProvider -> /llm-providers/{name}/api-keys/...
//   - LlmProxy    -> /llm-proxies/{name}/api-keys/...
type ApiKeyParentRef struct {
	// Kind selects the parent resource kind.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=RestApi;LlmProvider;LlmProxy
	Kind string `json:"kind"`

	// Name is the parent resource handle (metadata.name).
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

// ApiKeyExpiry describes an expiration duration for the API key.
type ApiKeyExpiry struct {
	// Duration is the magnitude of the expiry duration (must be positive).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	Duration int64 `json:"duration"`

	// Unit is the time unit of the expiry duration.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=seconds;minutes;hours;days;weeks;months
	Unit string `json:"unit"`
}

// ApiKeySpec mirrors the management-API APIKeyCreationRequest payload, with
// the optional inline ApiKey expressed as a SecretValueSource for external-
// key injection (so the value need not be inlined in the CR).
// +kubebuilder:validation:XValidation:rule="!has(self.expiresAt) || !has(self.expiresIn)",message="expiresAt and expiresIn are mutually exclusive"
type ApiKeySpec struct {
	// ParentRef identifies the resource the key is issued under.
	// +kubebuilder:validation:Required
	ParentRef ApiKeyParentRef `json:"parentRef"`

	// DisplayName is a human-readable label for the key.
	// +optional
	DisplayName *string `json:"displayName,omitempty"`

	// ApiKey is an optional pre-generated key value (>= 36 chars). When
	// supplied the gateway hashes the value rather than generating its own.
	// +optional
	ApiKey *SecretValueSource `json:"apiKey,omitempty"`

	// MaskedApiKey is an optional masked rendering of an externally
	// generated key, used purely for display by portals.
	// +optional
	MaskedApiKey *string `json:"maskedApiKey,omitempty"`

	// ExpiresIn is an optional duration after which the key expires.
	// Mutually exclusive with expiresAt: the ApiKey spec CEL rule
	// "!has(self.expiresAt) || !has(self.expiresIn)" rejects manifests that set both.
	// +optional
	ExpiresIn *ApiKeyExpiry `json:"expiresIn,omitempty"`

	// ExpiresAt is an optional absolute expiry time.
	// Mutually exclusive with expiresIn: the ApiKey spec CEL rule
	// "!has(self.expiresAt) || !has(self.expiresIn)" rejects manifests that set both.
	// +optional
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`

	// Issuer optionally constrains regeneration to a single portal.
	// +optional
	Issuer *string `json:"issuer,omitempty"`

	// ExternalRefId is an optional reference id for tracing.
	// +optional
	ExternalRefId *string `json:"externalRefId,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=apikeys,singular=apikey,shortName=apik

// ApiKey is the Schema for the apikeys API.
//
// The key handle in the management API is the CR's metadata.name. The
// nested REST path is built from spec.parentRef.kind + spec.parentRef.name.
type ApiKey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApiKeySpec     `json:"spec,omitempty"`
	Status ResourceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ApiKeyList contains a list of ApiKey.
type ApiKeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ApiKey `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ApiKey{}, &ApiKeyList{})
}
