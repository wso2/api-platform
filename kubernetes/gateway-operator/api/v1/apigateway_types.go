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

	// Labels are custom labels propagated to all gateway-managed resources:
	// Deployment metadata, Pod labels, Service metadata, and ConfigMaps.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations are custom annotations propagated to all gateway-managed resources:
	// Deployment metadata, Pod annotations, Service metadata, and ConfigMaps.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// GatewaySpec defines the desired state of APIGateway
type GatewaySpec struct {
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

	// ConfigRef references a ConfigMap containing custom Helm values configuration.
	// The ConfigMap data should contain a key "values.yaml" with the Helm values content.
	// If not specified, the operator will use the default mounted configuration.
	// +optional
	ConfigRef *corev1.LocalObjectReference `json:"configRef,omitempty"`

	// AdminCredentials sets the gateway-controller's basic-auth admin user from a
	// Kubernetes Secret. The operator configures the gateway with this user and uses
	// the same credentials to call the gateway's management API; the password never
	// enters the Helm values or the gateway ConfigMap. The admin user is listed first,
	// followed by the other basic-auth users in the ConfigRef values (a ConfigRef user
	// with the same username is replaced). The operator's default admin/admin user is
	// not kept. When unset, the admin credentials come from the ConfigRef values, as
	// before.
	// +optional
	AdminCredentials *GatewayAdminCredentials `json:"adminCredentials,omitempty"`
}

// GatewayAdminCredentials references the gateway-controller admin user's password
// in a Kubernetes Secret.
// +kubebuilder:validation:XValidation:rule="has(self.passwordSecretRef.name) && size(self.passwordSecretRef.name) > 0",message="passwordSecretRef.name must name the Secret that holds the admin password",fieldPath=".passwordSecretRef.name"
// +kubebuilder:validation:XValidation:rule="!has(self.passwordSecretRef.name) || size(self.passwordSecretRef.name) == 0 || (size(self.passwordSecretRef.name) <= 253 && self.passwordSecretRef.name.matches('^[a-z0-9]([-a-z0-9]*[a-z0-9])?([.][a-z0-9]([-a-z0-9]*[a-z0-9])?)*$'))",message="passwordSecretRef.name must be a Secret name: lowercase letters, digits, '-' and '.', at most 253 characters",fieldPath=".passwordSecretRef.name"
// +kubebuilder:validation:XValidation:rule="size(self.passwordSecretRef.key) <= 253 && self.passwordSecretRef.key.matches('^[-._a-zA-Z0-9]+$') && self.passwordSecretRef.key != '.' && !self.passwordSecretRef.key.startsWith('..')",message="passwordSecretRef.key must be a Secret key name (letters, digits, '-', '_', '.'), at most 253 characters, not starting with '..'",fieldPath=".passwordSecretRef.key"
// +kubebuilder:validation:XValidation:rule="!has(self.source) || self.source != 'file' || !self.passwordSecretRef.key.contains('..')",message="passwordSecretRef.key must not contain '..' when source is file: the gateway-controller rejects such a file path",fieldPath=".passwordSecretRef.key"
// +kubebuilder:validation:XValidation:rule="!has(self.passwordSecretRef.optional) || !self.passwordSecretRef.optional",message="passwordSecretRef.optional must not be true: the admin password Secret is required",fieldPath=".passwordSecretRef.optional"
type GatewayAdminCredentials struct {
	// Username of the admin user. It cannot contain ':' (HTTP Basic authentication
	// splits on the first ':') or control characters.
	// +kubebuilder:default=admin
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[^:\x00-\x1f\x7f]+$`
	// +optional
	Username string `json:"username,omitempty"`

	// PasswordSecretRef selects the key holding the plaintext admin password in a
	// Secret in the APIGateway's namespace. The Secret and key are required, so
	// optional cannot be true.
	// +kubebuilder:validation:Required
	PasswordSecretRef corev1.SecretKeySelector `json:"passwordSecretRef"`

	// Source is how the gateway-controller receives the password: "file" (the
	// default) mounts the Secret key read-only and the controller reads it from the
	// file, so the value never enters the pod environment; "env" injects it as an
	// environment variable from the Secret.
	// +kubebuilder:validation:Enum=env;file
	// +kubebuilder:default=file
	// +optional
	Source string `json:"source,omitempty"`
}

// Admin password delivery modes for GatewayAdminCredentials.Source.
const (
	AdminCredentialsSourceEnv  = "env"
	AdminCredentialsSourceFile = "file"
)

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

// Condition Types for Gateway
const (
	// GatewayConditionAccepted indicates the CR passed validation and is accepted for processing
	GatewayConditionAccepted = "Accepted"
	// GatewayConditionProgrammed indicates the Gateway is successfully deployed/programmed
	GatewayConditionProgrammed = "Programmed"
	// GatewayConditionReady is the canonical Ready condition type for Gateway (retained for compatibility)
	GatewayConditionReady = "Ready"
)

// Accepted Condition Reasons
const (
	// GatewayAcceptedReasonAccepted indicates the CR passed validation
	GatewayAcceptedReasonAccepted = "Accepted"
	// GatewayAcceptedReasonInvalidConfiguration indicates the CR failed validation
	GatewayAcceptedReasonInvalidConfiguration = "InvalidConfiguration"
	// GatewayAcceptedReasonPending indicates validation is pending
	GatewayAcceptedReasonPending = "Pending"
)

// Programmed Condition Reasons
const (
	// GatewayProgrammedReasonProgrammed indicates successful deployment
	GatewayProgrammedReasonProgrammed = "Programmed"
	// GatewayProgrammedReasonPending indicates deployment is pending
	GatewayProgrammedReasonPending = "Pending"
	// GatewayProgrammedReasonInvalid indicates configuration is invalid for deployment
	GatewayProgrammedReasonInvalid = "Invalid"
	// GatewayProgrammedReasonDeploymentFailed indicates deployment failed (non-retryable)
	GatewayProgrammedReasonDeploymentFailed = "DeploymentFailed"
	// GatewayProgrammedReasonRetrying indicates deployment failed and is being retried
	GatewayProgrammedReasonRetrying = "Retrying"
)

// GatewayPhase represents the lifecycle phase of a Gateway
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

// GatewayStatus defines the observed state of APIGateway
type GatewayStatus struct {
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

	// ConfigHash represents the hash of the referenced configuration
	// used to detect changes in ConfigMap content
	// +optional
	ConfigHash string `json:"configHash,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:storageversion
//+kubebuilder:subresource:status

// APIGateway is the Schema for the apigateways API
type APIGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GatewaySpec   `json:"spec,omitempty"`
	Status GatewayStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// APIGatewayList contains a list of APIGateway
type APIGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []APIGateway `json:"items"`
}

func init() {
	SchemeBuilder.Register(&APIGateway{}, &APIGatewayList{})
}
