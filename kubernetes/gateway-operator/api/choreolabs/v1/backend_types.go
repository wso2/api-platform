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

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
)

// BackendEndpoint is one target of a Backend.
type BackendEndpoint struct {
	// URL is the endpoint's scheme, host and port; the path comes from basePath. All endpoints of
	// a Backend use the same scheme, and a Backend with tls uses https.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^https?://`
	URL string `json:"url"`

	// Weight is the endpoint's relative share of requests across the Backend's endpoints.
	// Omitted or 0 counts as 1.
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Weight *int `json:"weight,omitempty"`
}

// BackendTLS is how the gateway connects to the Backend's endpoints over TLS. It applies to this
// Backend only, never to the API that routes to it.
type BackendTLS struct {
	// Identity names a gateway identity (a certificate with usage: identity) to present.
	// +optional
	Identity string `json:"identity,omitempty"`

	// TrustedCAs names certificates (usage: upstream) to trust for this Backend in place of the
	// gateway-wide trust bundle.
	// +optional
	TrustedCAs []string `json:"trustedCAs,omitempty"`

	// VerifyHostName checks each endpoint's certificate against its host. Defaults to true.
	// +optional
	VerifyHostName *bool `json:"verifyHostName,omitempty"`
}

// BackendSpec mirrors the gateway management API's BackendConfiguration.
type BackendSpec struct {
	// DisplayName is a human-readable name.
	// +optional
	// +kubebuilder:validation:MaxLength=253
	DisplayName string `json:"displayName,omitempty"`

	// Description describes the Backend.
	// +optional
	// +kubebuilder:validation:MaxLength=1023
	Description string `json:"description,omitempty"`

	// BasePath replaces the API's base path for requests routed to this Backend: the gateway
	// strips the API context and prefixes this path. Must start with "/" and must not end with
	// "/"; omit for root.
	// +optional
	// +kubebuilder:validation:Pattern=`^/[a-zA-Z0-9\-._~!$&'()*+,;=:@%/]*[^/]$`
	BasePath *string `json:"basePath,omitempty"`

	// Endpoints are the Backend's targets, load-balanced by weight.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Endpoints []BackendEndpoint `json:"endpoints"`

	// Timeout is the connect timeout to the endpoints.
	// +optional
	Timeout *apiv1.UpstreamTimeout `json:"timeout,omitempty"`

	// TLS configures the connection to https endpoints.
	// +optional
	TLS *BackendTLS `json:"tls,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=backends,singular=backend
//+kubebuilder:validation:XValidation:rule="size(self.metadata.name) >= 3 && size(self.metadata.name) <= 40 && self.metadata.name.matches('^[a-z0-9]+(-[a-z0-9]+)*$')",message="metadata.name must be 3 to 40 lowercase letters and digits, with single hyphens between them"
//+kubebuilder:printcolumn:name="Base Path",type=string,JSONPath=`.spec.basePath`
//+kubebuilder:printcolumn:name="Programmed",type=string,JSONPath=`.status.conditions[?(@.type=="Programmed")].status`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Backend is a gateway-wide backend: defined once, any API on the selected gateways can route to
// it by name with the dynamic-endpoint policy, fixed or chosen per request. Its labels are what a
// dynamic-endpoint backendSelector matches, and they also select the gateways it is deployed to.
type Backend struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackendSpec          `json:"spec"`
	Status apiv1.ResourceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BackendList contains a list of Backend.
type BackendList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Backend `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Backend{}, &BackendList{})
}
