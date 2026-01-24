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

// SnapshotManager manages xDS snapshots for policy configurations
type SnapshotManager struct {
	cache      *cache.LinearCache // Use LinearCache directly for custom type URLs
	store      *storage.PolicyStore
	logger     *slog.Logger
	nodeID     string
	mu         sync.RWMutex
	translator *Translator
}

// NewSnapshotManager creates a new policy snapshot manager with LinearCache for custom type URLs
func NewSnapshotManager(store *storage.PolicyStore, logger *slog.Logger) *SnapshotManager {
	// Create a LinearCache for custom PolicyChainConfig type URL
	// LinearCache is designed for single custom resource types in ADS
	linearCache := cache.NewLinearCache(
		PolicyChainTypeURL,
		cache.WithLogger(slogAdapter{logger}),
	)

	return &SnapshotManager{
		cache:      linearCache,
		store:      store,
		logger:     logger,
		nodeID:     "policy-node",
		translator: NewTranslator(logger),
	}
}

// GetCache returns the underlying cache as the generic Cache interface
func (sm *SnapshotManager) GetCache() cache.Cache {
	return sm.cache
}

// UpdateSnapshot generates a new xDS snapshot from all policy configurations
func (sm *SnapshotManager) UpdateSnapshot(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Get all policy configurations from store
	policies := sm.store.GetAll()

	sm.logger.Info("Updating policy snapshot",
		slog.Int("policy_count", len(policies)),
		slog.String("node_id", sm.nodeID))

	// Translate policies to xDS resources
	resourcesMap, err := sm.translator.TranslatePolicies(policies)
	if err != nil {
		sm.logger.Error("Failed to translate policies", slog.Any("error", err))
		return fmt.Errorf("failed to translate policies: %w", err)
	}

	// Get the policy resources from the map
	policyResources, ok := resourcesMap[PolicyChainTypeURL]
	if !ok {
		sm.logger.Warn("No policy resources found after translation")
		policyResources = []types.Resource{} // Empty resources
	}

	// Increment resource version
	version := sm.store.IncrementResourceVersion()
	versionStr := fmt.Sprintf("%d", version)

	// For LinearCache, we need to update resources directly
	// Convert []types.Resource to map[string]types.Resource (keyed by policy ID)
	resourcesById := make(map[string]types.Resource)
	for i, res := range policyResources {
		// Use index-based key since policy resources don't have inherent names
		resourcesById[fmt.Sprintf("policy-%d", i)] = res
	}

	// Update the linear cache with new resources
	// SetResources replaces all resources in the cache
	sm.cache.SetResources(resourcesById)

	sm.logger.Info("Policy snapshot updated successfully",
		slog.String("version", versionStr),
		slog.Int("policy_count", len(policies)))

	return nil
}

// Translator converts policy configurations to xDS resources
type Translator struct {
	logger *slog.Logger
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

const (
	// PolicyChainTypeURL is the custom type URL for policy chain configurations
	PolicyChainTypeURL = "api-platform.wso2.org/v1.PolicyChainConfig"
)

// NewTranslator creates a new policy translator
func NewTranslator(logger *slog.Logger) *Translator {
	return &Translator{
		logger: logger,
	}
}

// TranslatePolicies translates policy configurations to xDS resources
// Uses ADS with custom type URL for policy distribution
func (t *Translator) TranslatePolicies(policies []*models.StoredPolicyConfig) (map[string][]types.Resource, error) {
	resources := make(map[string][]types.Resource)

	// For policy data, we use custom PolicyChainConfig type
	var policyResources []types.Resource

	for _, policy := range policies {
		// Convert policy to a custom resource
		policyResource, err := t.createPolicyResource(policy)
		if err != nil {
			t.logger.Error("Failed to create policy resource",
				slog.String("id", policy.ID),
				slog.Any("error", err))
			continue
		}

		policyResources = append(policyResources, policyResource)

		t.logger.Debug("Processing policy for xDS",
			slog.String("id", policy.ID),
			slog.String("api_name", policy.APIName()),
			slog.String("version", policy.APIVersion()),
			slog.Int("route_count", len(policy.Configuration.Routes)))
	}

	// Store policy resources with custom type URL
	resources[PolicyChainTypeURL] = policyResources

	t.logger.Info("Translated policies to xDS resources",
		slog.Int("total_policies", len(policies)),
		slog.Int("policy_resources", len(policyResources)))

	return resources, nil
}

// createPolicyResource creates a custom PolicyChainConfig resource from a policy configuration
func (t *Translator) createPolicyResource(policy *models.StoredPolicyConfig) (types.Resource, error) {
	// Use JSON marshaling to properly handle all field types including pointers
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal policy to JSON: %w", err)
	}

	// Convert JSON to map[string]interface{}
	var policyMap map[string]interface{}
	if err := json.Unmarshal(policyJSON, &policyMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal policy JSON: %w", err)
	}

	// Create struct from map
	policyStruct, err := structpb.NewStruct(policyMap)
	if err != nil {
		return nil, fmt.Errorf("failed to create policy struct: %w", err)
	}

	// Wrap in google.protobuf.Any with custom type URL
	anyMsg, err := anypb.New(policyStruct)
	if err != nil {
		return nil, fmt.Errorf("failed to create Any message: %w", err)
	}

	// Override the type URL to our custom type
	anyMsg.TypeUrl = PolicyChainTypeURL

	return anyMsg, nil
}
