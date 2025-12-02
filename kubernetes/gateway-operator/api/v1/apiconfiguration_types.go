/*
Copyright 2025.

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

// APIConditionReady is the canonical Ready condition type for APIConfiguration
const APIConditionReady = "Ready"

// APIPhase represents the lifecycle phase of an APIConfiguration
// +kubebuilder:validation:Enum=Pending;Deployed;Failed
type APIPhase string

const (
	// APIPhasePending indicates the controller is waiting to deploy the API
	APIPhasePending APIPhase = "Pending"
	// APIPhaseDeployed indicates the API has been deployed to all target gateways
	APIPhaseDeployed APIPhase = "Deployed"
	// APIPhaseFailed indicates the controller failed to deploy the API
	APIPhaseFailed APIPhase = "Failed"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// GatewayReference defines a reference to a GatewayConfiguration
type GatewayReference struct {
	// Name is the name of the GatewayConfiguration
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace is the namespace of the GatewayConfiguration
	// If empty, the API's namespace is used
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// APIConfigurationSpec defines the desired state of APIConfiguration
type APIConfigurationSpec struct {
	// GatewayRefs is a list of explicit references to GatewayConfigurations.
	// If not specified, the API will be selected by gateways based on their APISelector.
	// If specified, the API will be deployed only to the referenced gateways.
	// +optional
	GatewayRefs []GatewayReference `json:"gatewayRefs,omitempty"`

	// API contains the API configuration from the gateway controller
	// +kubebuilder:validation:Required
	APIConfiguration APIConfig `json:"apiConfiguration"`
}

// APIConfigurationStatus defines the observed state of APIConfiguration
type APIConfigurationStatus struct {
	// Conditions represent the latest available observations of the API's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Phase represents the current phase of the API (Pending, Deployed, Failed)
	// +optional
	Phase APIPhase `json:"phase,omitempty"`

	// DeployedGateways is a list of gateways where this API is deployed
	// +optional
	DeployedGateways []string `json:"deployedGateways,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed spec
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// LastUpdateTime is the last time the status was updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// APIConfiguration is the Schema for the apiconfigurations API
type APIConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   APIConfigurationSpec   `json:"spec,omitempty"`
	Status APIConfigurationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// APIConfigurationList contains a list of APIConfiguration
type APIConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []APIConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&APIConfiguration{}, &APIConfigurationList{})
}
