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

package registry

import (
	"fmt"
	"sync"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GatewayInfo holds information about a deployed gateway
type GatewayInfo struct {
	Name             string
	Namespace        string
	GatewayClassName string
	APISelector      *apiv1.APISelector
	ServiceName      string // Name of the gateway controller service
	ServicePort      int32  // Port of the gateway controller API
	ControlPlaneHost string
}

// GatewayRegistry maintains an in-memory registry of deployed gateways
type GatewayRegistry struct {
	mu       sync.RWMutex
	gateways map[string]*GatewayInfo // key: namespace/name
}

var (
	instance *GatewayRegistry
	once     sync.Once
)

// GetGatewayRegistry returns the singleton gateway registry instance
func GetGatewayRegistry() *GatewayRegistry {
	once.Do(func() {
		instance = &GatewayRegistry{
			gateways: make(map[string]*GatewayInfo),
		}
	})
	return instance
}

// Register adds or updates a gateway in the registry
func (r *GatewayRegistry) Register(gateway *GatewayInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := gateway.Namespace + "/" + gateway.Name
	r.gateways[key] = gateway
}

// Unregister removes a gateway from the registry
func (r *GatewayRegistry) Unregister(namespace, name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := namespace + "/" + name
	delete(r.gateways, key)
}

// Get retrieves a gateway by namespace and name
func (r *GatewayRegistry) Get(namespace, name string) (*GatewayInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := namespace + "/" + name
	gateway, exists := r.gateways[key]
	return gateway, exists
}

// FindMatchingGateways finds all gateways that match the given API's namespace and labels
func (r *GatewayRegistry) FindMatchingGateways(apiNamespace string, apiLabels map[string]string) []*GatewayInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matching []*GatewayInfo
	if len(r.gateways) == 0 {
		fmt.Println("No gateways registered in the registry")
		return matching
	}
	for _, gateway := range r.gateways {
		fmt.Printf("Checking gateway: %s/%s with selector: %+v\n", gateway.Namespace, gateway.Name, gateway.APISelector)
		if r.gatewayMatchesAPI(gateway, apiNamespace, apiLabels) {
			matching = append(matching, gateway)
		}
	}

	return matching
}

// gatewayMatchesAPI checks if a gateway should handle the given API
func (r *GatewayRegistry) gatewayMatchesAPI(gateway *GatewayInfo, apiNamespace string, apiLabels map[string]string) bool {
	if gateway.APISelector == nil {
		return false
	}

	switch gateway.APISelector.Scope {
	case apiv1.ClusterScope:
		// Cluster-scoped gateways accept APIs from any namespace
		return true

	case apiv1.NamespacedScope:
		// Check if API namespace is in the allowed list
		if gateway.APISelector.Namespaces == nil {
			return false
		}
		for _, ns := range gateway.APISelector.Namespaces {
			if ns == apiNamespace {
				return true
			}
		}
		return false

	case apiv1.LabelSelectorScope:
		// Check if API labels match the gateway's label selector
		return r.labelsMatch(gateway.APISelector, apiLabels)

	default:
		return false
	}
}

// labelsMatch checks if the API labels match the gateway's label selector
func (r *GatewayRegistry) labelsMatch(selector *apiv1.APISelector, apiLabels map[string]string) bool {
	// Check matchLabels
	if selector.MatchLabels != nil {
		for key, value := range selector.MatchLabels {
			if apiLabels[key] != value {
				return false
			}
		}
	}

	// Check matchExpressions
	if selector.MatchExpressions != nil {
		for _, expr := range selector.MatchExpressions {
			if !r.expressionMatches(expr, apiLabels) {
				return false
			}
		}
	}

	return true
}

// expressionMatches checks if a label selector expression matches the API labels
func (r *GatewayRegistry) expressionMatches(expr metav1.LabelSelectorRequirement, apiLabels map[string]string) bool {
	labelValue, exists := apiLabels[expr.Key]

	switch expr.Operator {
	case "In":
		if !exists {
			return false
		}
		for _, v := range expr.Values {
			if v == labelValue {
				return true
			}
		}
		return false

	case "NotIn":
		if !exists {
			return true
		}
		for _, v := range expr.Values {
			if v == labelValue {
				return false
			}
		}
		return true

	case "Exists":
		return exists

	case "DoesNotExist":
		return !exists

	default:
		return false
	}
}

// ListAll returns all registered gateways
func (r *GatewayRegistry) ListAll() []*GatewayInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	gateways := make([]*GatewayInfo, 0, len(r.gateways))
	for _, gateway := range r.gateways {
		gateways = append(gateways, gateway)
	}
	return gateways
}

// GetGatewayServiceEndpoint returns the HTTP endpoint for the gateway controller API
func (g *GatewayInfo) GetGatewayServiceEndpoint() string {
	// In Kubernetes, services can be accessed via: <service-name>.<namespace>.svc.cluster.local:<port>
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", g.ServiceName, g.Namespace, g.ServicePort)
}
