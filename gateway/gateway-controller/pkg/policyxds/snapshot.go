/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package policyxds

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	// PolicyChainTypeURL is the custom type URL for policy chain configurations
	PolicyChainTypeURL = "api-platform.wso2.org/v1.PolicyChainConfig"

	// RouteConfigTypeURL is the custom type URL for route config (metadata + resolver)
	RouteConfigTypeURL = "api-platform.wso2.org/v1.RouteConfig"
)

// SnapshotManager manages xDS snapshots for policy and route configurations.
// It holds two LinearCaches: one for PolicyChainConfig and one for RouteConfig.
type SnapshotManager struct {
	policyCache  *cache.LinearCache
	routeCache   *cache.LinearCache
	runtimeStore *storage.RuntimeConfigStore
	logger       *slog.Logger
	nodeID       string
	mu           sync.RWMutex
	translator   *Translator
}

// NewSnapshotManager creates a new policy snapshot manager with LinearCaches for custom type URLs.
func NewSnapshotManager(logger *slog.Logger) *SnapshotManager {
	policyCache := cache.NewLinearCache(
		PolicyChainTypeURL,
		cache.WithLogger(slogAdapter{logger}),
	)
	routeCache := cache.NewLinearCache(
		RouteConfigTypeURL,
		cache.WithLogger(slogAdapter{logger}),
	)

	return &SnapshotManager{
		policyCache: policyCache,
		routeCache:  routeCache,
		logger:      logger,
		nodeID:      "policy-node",
		translator:  NewTranslator(logger),
	}
}

// SetRuntimeStore sets the RuntimeConfigStore for the new API path.
func (sm *SnapshotManager) SetRuntimeStore(store *storage.RuntimeConfigStore) {
	sm.runtimeStore = store
}

// GetPolicyCache returns the policy chain cache.
func (sm *SnapshotManager) GetPolicyCache() cache.Cache {
	return sm.policyCache
}

// GetRouteCache returns the route config cache.
func (sm *SnapshotManager) GetRouteCache() cache.Cache {
	return sm.routeCache
}

// GetCache returns the policy chain cache (backward compatible).
func (sm *SnapshotManager) GetCache() cache.Cache {
	return sm.policyCache
}

// UpdateSnapshot generates new xDS snapshots from all RuntimeDeployConfigs.
func (sm *SnapshotManager) UpdateSnapshot(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.runtimeStore == nil {
		sm.logger.Warn("RuntimeConfigStore not set, skipping snapshot update")
		return nil
	}

	rdcs := sm.runtimeStore.GetAll()

	sm.logger.Info("Updating policy snapshot",
		slog.Int("rdc_count", len(rdcs)),
		slog.String("node_id", sm.nodeID))

	// Translate RuntimeDeployConfigs to xDS resources (both types)
	resourcesMap, err := sm.translator.TranslateRuntimeConfigs(rdcs)
	if err != nil {
		sm.logger.Error("Failed to translate runtime configs", slog.Any("error", err))
		return fmt.Errorf("failed to translate runtime configs: %w", err)
	}

	// Update policy chain cache
	policyResources, _ := resourcesMap[PolicyChainTypeURL]
	policyById := make(map[string]types.Resource)
	for key, res := range policyResources {
		policyById[key] = res
	}
	sm.policyCache.SetResources(policyById)

	// Update route config cache
	routeResources, _ := resourcesMap[RouteConfigTypeURL]
	routeById := make(map[string]types.Resource)
	for key, res := range routeResources {
		routeById[key] = res
	}
	sm.routeCache.SetResources(routeById)

	version := sm.runtimeStore.IncrementResourceVersion()
	sm.logger.Info("Policy snapshot updated successfully",
		slog.Int64("version", version),
		slog.Int("policy_resources", len(policyById)),
		slog.Int("route_resources", len(routeById)))

	return nil
}

// Translator converts RuntimeDeployConfig to xDS resources.
type Translator struct {
	logger *slog.Logger
}

// NewTranslator creates a new policy translator.
func NewTranslator(logger *slog.Logger) *Translator {
	return &Translator{
		logger: logger,
	}
}

// TranslateRuntimeConfigs translates RuntimeDeployConfigs to xDS resources.
// Returns two maps: PolicyChainTypeURL → keyed resources, RouteConfigTypeURL → keyed resources.
func (t *Translator) TranslateRuntimeConfigs(rdcs []*models.RuntimeDeployConfig) (map[string]map[string]types.Resource, error) {
	policyResources := make(map[string]types.Resource)
	routeResources := make(map[string]types.Resource)

	for _, rdc := range rdcs {
		// Build policy chain resources (one per chain)
		for routeKey, chain := range rdc.PolicyChains {
			if len(chain.Policies) == 0 {
				continue
			}
			resource, err := t.createPolicyChainResource(routeKey, chain, rdc.Metadata)
			if err != nil {
				t.logger.Error("Failed to create policy chain resource",
					slog.String("route_key", routeKey),
					slog.Any("error", err))
				continue
			}
			policyResources[routeKey] = resource
		}

		// Build route config resources (one per route)
		for routeKey, route := range rdc.Routes {
			// Find upstream base path from the route's cluster
			upstreamBasePath := "/"
			if uc, ok := rdc.UpstreamClusters[route.Upstream.ClusterKey]; ok {
				upstreamBasePath = uc.BasePath
			}

			// Build upstream definition paths
			upstreamDefPaths := make(map[string]string)
			for clusterKey, uc := range rdc.UpstreamClusters {
				upstreamDefPaths[clusterKey] = uc.BasePath
			}

			resource, err := t.createRouteConfigResource(routeKey, rdc, upstreamBasePath, upstreamDefPaths)
			if err != nil {
				t.logger.Error("Failed to create route config resource",
					slog.String("route_key", routeKey),
					slog.Any("error", err))
				continue
			}
			routeResources[routeKey] = resource
		}
	}

	result := map[string]map[string]types.Resource{
		PolicyChainTypeURL: policyResources,
		RouteConfigTypeURL: routeResources,
	}

	t.logger.Info("Translated runtime configs to xDS resources",
		slog.Int("total_rdcs", len(rdcs)),
		slog.Int("policy_resources", len(policyResources)),
		slog.Int("route_resources", len(routeResources)))

	return result, nil
}

// createPolicyChainResource creates a PolicyChainConfig xDS resource.
func (t *Translator) createPolicyChainResource(routeKey string, chain *models.PolicyChain, metadata models.Metadata) (types.Resource, error) {
	// Build the policy chain data
	policies := make([]map[string]interface{}, 0, len(chain.Policies))
	for _, p := range chain.Policies {
		pol := map[string]interface{}{
			"name":       p.Name,
			"version":    p.Version,
			"enabled":    true,
			"parameters": p.Params,
		}
		if p.ExecutionCondition != nil {
			pol["executionCondition"] = *p.ExecutionCondition
		}
		policies = append(policies, pol)
	}

	data := map[string]interface{}{
		"configuration": map[string]interface{}{
			"routes": []map[string]interface{}{
				{
					"route_key": routeKey,
					"policies":  policies,
				},
			},
			"metadata": map[string]interface{}{
				"api_name": metadata.DisplayName,
				"version":  metadata.Version,
				"context":  "",
			},
		},
	}

	return toAnyResource(data, PolicyChainTypeURL)
}

// createRouteConfigResource creates a RouteConfig xDS resource.
func (t *Translator) createRouteConfigResource(
	routeKey string,
	rdc *models.RuntimeDeployConfig,
	upstreamBasePath string,
	upstreamDefPaths map[string]string,
) (types.Resource, error) {
	route := rdc.Routes[routeKey]

	metadataMap := map[string]interface{}{
		"kind":         rdc.Metadata.Kind,
		"handle":       rdc.Metadata.Handle,
		"name":         rdc.Metadata.Name,
		"version":      rdc.Metadata.Version,
		"display_name": rdc.Metadata.DisplayName,
		"project_id":   rdc.Metadata.ProjectID,
		"api_context":  rdc.Context,
		"vhost":        route.Vhost,
		"path":         route.OperationPath,
	}
	if rdc.Metadata.LLM != nil {
		metadataMap["template_handle"] = rdc.Metadata.LLM.TemplateHandle
		metadataMap["provider_name"] = rdc.Metadata.LLM.ProviderName
	}

	data := map[string]interface{}{
		"route_key":                 routeKey,
		"metadata":                  metadataMap,
		"resolver_name":             rdc.PolicyChainResolver,
		"upstream_base_path":        upstreamBasePath,
		"upstream_definition_paths": upstreamDefPaths,
	}

	// Add default upstream cluster info
	if route.Upstream.UseClusterHeader && route.Upstream.DefaultCluster != "" {
		data["default_upstream_cluster"] = route.Upstream.DefaultCluster
	}

	return toAnyResource(data, RouteConfigTypeURL)
}

// toAnyResource converts a map to an anypb.Any resource with the given type URL.
func toAnyResource(data map[string]interface{}, typeURL string) (types.Resource, error) {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal to JSON: %w", err)
	}

	var dataMap map[string]interface{}
	if err := json.Unmarshal(dataJSON, &dataMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	dataStruct, err := structpb.NewStruct(dataMap)
	if err != nil {
		return nil, fmt.Errorf("failed to create struct: %w", err)
	}

	anyMsg, err := anypb.New(dataStruct)
	if err != nil {
		return nil, fmt.Errorf("failed to create Any message: %w", err)
	}

	anyMsg.TypeUrl = typeURL
	return anyMsg, nil
}

// slogAdapter adapts slog.Logger to the go-control-plane Logger interface
type slogAdapter struct {
	logger *slog.Logger
}

func (a slogAdapter) Debugf(format string, args ...interface{}) {
	a.logger.Debug(fmt.Sprintf(format, args...))
}

func (a slogAdapter) Infof(format string, args ...interface{}) {
	a.logger.Info(fmt.Sprintf(format, args...))
}

func (a slogAdapter) Warnf(format string, args ...interface{}) {
	a.logger.Warn(fmt.Sprintf(format, args...))
}

func (a slogAdapter) Errorf(format string, args ...interface{}) {
	a.logger.Error(fmt.Sprintf(format, args...))
}
