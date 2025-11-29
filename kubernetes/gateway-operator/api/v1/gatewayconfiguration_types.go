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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// APISelectorScope defines the scope of API selection for a gateway
// +kubebuilder:validation:Enum=Cluster;Namespaced;LabelSelector
type APISelectorScope string

const (
	// ClusterScope means the gateway accepts APIs from any namespace
	ClusterScope APISelectorScope = "Cluster"

	// NamespacedScope means the gateway only accepts APIs from specific namespaces
	NamespacedScope APISelectorScope = "Namespaced"

	// LabelSelectorScope means the gateway accepts APIs matching specific labels
	LabelSelectorScope APISelectorScope = "LabelSelector"
)

// APISelector defines how a gateway selects which APIs to route
type APISelector struct {
	// Scope determines the API selection strategy
	// +kubebuilder:validation:Required
	// +kubebuilder:default=Cluster
	Scope APISelectorScope `json:"scope"`

	// Namespaces is a list of namespaces from which APIs are selected.
	// Only used when Scope is "Namespaced".
	// If empty with Namespaced scope, only APIs in the gateway's namespace are selected.
	// +optional
	Namespaces []string `json:"namespaces,omitempty"`

	// MatchLabels is a map of {key,value} pairs for label-based selection.
	// Only used when Scope is "LabelSelector".
	// An API must have all specified labels to be selected.
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`

	// MatchExpressions is a list of label selector requirements for label-based selection.
	// Only used when Scope is "LabelSelector".
	// +optional
	MatchExpressions []metav1.LabelSelectorRequirement `json:"matchExpressions,omitempty"`
}

// GatewayInfrastructure defines the infrastructure configuration for the gateway
type GatewayInfrastructure struct {
	// Replicas is the number of gateway instances to deploy
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Resources defines the compute resources for gateway pods
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Image is the container image for the gateway
	// +optional
	Image string `json:"image,omitempty"`

	// RouterImage is the container image for the router/proxy
	// +optional
	RouterImage string `json:"routerImage,omitempty"`

	// NodeSelector is a selector for node assignment
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations for pod assignment
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Affinity for pod assignment
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
}

// GatewayConfigurationSpec defines the desired state of GatewayConfiguration
type GatewayConfigurationSpec struct {
	// GatewayClassName is an optional identifier for grouping gateways
	// This can be used to categorize gateways (e.g., "production", "development")
	// +optional
	GatewayClassName string `json:"gatewayClassName,omitempty"`

	// APISelector defines how this gateway selects which APIs to route
	// +kubebuilder:validation:Required
	APISelector APISelector `json:"apiSelector"`

	// Infrastructure defines the deployment configuration for the gateway
	// +optional
	Infrastructure *GatewayInfrastructure `json:"infrastructure,omitempty"`

	// ControlPlane defines the control plane connection settings
	// +optional
	ControlPlane *GatewayControlPlane `json:"controlPlane,omitempty"`

	// Storage defines the storage configuration for the gateway
	// +optional
	Storage *GatewayStorage `json:"storage,omitempty"`
}

// GatewayControlPlane defines control plane connection settings
type GatewayControlPlane struct {
	// Host is the control plane host address
	// +optional
	Host string `json:"host,omitempty"`

	// TokenSecretRef references a secret containing the authentication token
	// +optional
	TokenSecretRef *corev1.SecretKeySelector `json:"tokenSecretRef,omitempty"`

	// TLS settings for control plane connection
	// +optional
	TLS *GatewayTLSConfig `json:"tls,omitempty"`
}

// GatewayTLSConfig defines TLS configuration
type GatewayTLSConfig struct {
	// Enabled indicates whether TLS is enabled
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// CertSecretRef references a secret containing TLS certificates
	// +optional
	CertSecretRef *corev1.SecretKeySelector `json:"certSecretRef,omitempty"`
}

// GatewayStorage defines storage configuration
type GatewayStorage struct {
	// Type is the storage backend type (sqlite, postgres, mysql, etc.)
	// +kubebuilder:validation:Enum=sqlite;postgres;mysql
	// +kubebuilder:default=sqlite
	// +optional
	Type string `json:"type,omitempty"`

	// SQLitePath is the path for SQLite database (used when Type is sqlite)
	// +optional
	SQLitePath string `json:"sqlitePath,omitempty"`

	// ConnectionSecretRef references a secret containing database connection details
	// (used for postgres, mysql, etc.)
	// +optional
	ConnectionSecretRef *corev1.SecretKeySelector `json:"connectionSecretRef,omitempty"`
}

// GatewayConditionReady is the canonical Ready condition type for GatewayConfiguration
const GatewayConditionReady = "Ready"

// GatewayPhase represents the lifecycle phase of a GatewayConfiguration
// +kubebuilder:validation:Enum=Reconciling;Ready;Failed;Deleting
type GatewayPhase string

const (
	// GatewayPhaseReconciling indicates the controller is reconciling resources
	GatewayPhaseReconciling GatewayPhase = "Reconciling"
	// GatewayPhaseReady indicates the gateway is fully reconciled
	GatewayPhaseReady GatewayPhase = "Ready"
	// GatewayPhaseFailed indicates the gateway failed to reconcile
	GatewayPhaseFailed GatewayPhase = "Failed"
	// GatewayPhaseDeleting indicates the gateway is being deleted and cleanup is running
	GatewayPhaseDeleting GatewayPhase = "Deleting"
)

// GatewayConfigurationStatus defines the observed state of GatewayConfiguration
type GatewayConfigurationStatus struct {
	// Conditions represent the latest available observations of the gateway's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Phase represents the current phase of the gateway (Pending, Ready, Failed)
	// +optional
	Phase GatewayPhase `json:"phase,omitempty"`

	// SelectedAPIs is the count of APIs currently selected by this gateway
	// +optional
	SelectedAPIs int `json:"selectedAPIs,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed spec
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// AppliedGeneration tracks the latest spec generation that was successfully applied to the cluster
	// +optional
	AppliedGeneration int64 `json:"appliedGeneration,omitempty"`

	// LastUpdateTime is the last time the status was updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// GatewayConfiguration is the Schema for the gatewayconfigurations API
type GatewayConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GatewayConfigurationSpec   `json:"spec,omitempty"`
	Status GatewayConfigurationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GatewayConfigurationList contains a list of GatewayConfiguration
type GatewayConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GatewayConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GatewayConfiguration{}, &GatewayConfigurationList{})
}
