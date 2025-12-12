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

package k8sutil

import (
	corev1 "k8s.io/api/core/v1"
)

// GatewayManifestTemplateData holds all the data needed to render the gateway manifest template
type GatewayManifestTemplateData struct {
	// GatewayName is the name of the gateway - used to prefix all resource names
	GatewayName string

	// Replicas is the number of replicas for gateway-controller and router deployments
	Replicas int32

	// GatewayImage is the container image for the gateway-controller
	GatewayImage string

	// RouterImage is the container image for the router (Envoy)
	RouterImage string

	// ControlPlaneHost is the host and port of the control plane (e.g., "host.docker.internal:9243")
	ControlPlaneHost string

	// ControlPlaneTokenSecret is the secret reference for control plane authentication token
	ControlPlaneTokenSecret *SecretReference

	// LogLevel is the logging level (debug, info, warn, error)
	LogLevel string

	// StorageType is the type of storage backend (sqlite, postgres, memory)
	StorageType string

	// StorageSQLitePath is the path to the SQLite database file
	StorageSQLitePath string

	// Resources defines resource requests and limits for containers
	Resources *ResourceRequirements

	// NodeSelector is a map of node selector labels
	NodeSelector map[string]string

	// Tolerations is a list of tolerations
	Tolerations []corev1.Toleration

	// Affinity defines pod affinity/anti-affinity rules
	Affinity *corev1.Affinity
}

// SecretReference holds the secret name and key for referencing secrets
type SecretReference struct {
	Name string
	Key  string
}

// ResourceRequirements defines resource requests and limits
type ResourceRequirements struct {
	Requests *ResourceList
	Limits   *ResourceList
}

// ResourceList defines CPU and memory resources
type ResourceList struct {
	CPU    string
	Memory string
}

// NewGatewayManifestTemplateData creates a new GatewayManifestTemplateData with default values
func NewGatewayManifestTemplateData(gatewayName string) *GatewayManifestTemplateData {
	return &GatewayManifestTemplateData{
		GatewayName:       gatewayName,
		Replicas:          1,
		GatewayImage:      "wso2/gateway-controller:latest",
		RouterImage:       "wso2/gateway-router:latest",
		ControlPlaneHost:  "host.docker.internal:9243",
		LogLevel:          "info",
		StorageType:       "sqlite",
		StorageSQLitePath: "/app/data/gateway.db",
	}
}
