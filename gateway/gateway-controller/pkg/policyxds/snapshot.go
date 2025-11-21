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
	"sync"

	runtimev3 "github.com/envoyproxy/go-control-plane/envoy/service/runtime/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"
)

// SnapshotManager manages xDS snapshots for policy configurations
type SnapshotManager struct {
	cache      cache.SnapshotCache
	store      *storage.PolicyStore
	logger     *zap.Logger
	nodeID     string
	mu         sync.RWMutex
	translator *Translator
}

// NewSnapshotManager creates a new policy snapshot manager
func NewSnapshotManager(store *storage.PolicyStore, logger *zap.Logger) *SnapshotManager {
	// Create a snapshot cache with a simple node ID hasher
	snapshotCache := cache.NewSnapshotCache(false, cache.IDHash{}, logger.Sugar())

	return &SnapshotManager{
		cache:      snapshotCache,
		store:      store,
		logger:     logger,
		nodeID:     "policy-node",
		translator: NewTranslator(logger),
	}
}

// GetCache returns the underlying snapshot cache
func (sm *SnapshotManager) GetCache() cache.SnapshotCache {
	return sm.cache
}

// UpdateSnapshot generates a new xDS snapshot from all policy configurations
func (sm *SnapshotManager) UpdateSnapshot(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Get all policy configurations from store
	policies := sm.store.GetAll()

	sm.logger.Info("Updating policy snapshot",
		zap.Int("policy_count", len(policies)),
		zap.String("node_id", sm.nodeID))

	// Translate policies to xDS resources
	resources, err := sm.translator.TranslatePolicies(policies)
	if err != nil {
		sm.logger.Error("Failed to translate policies", zap.Error(err))
		return fmt.Errorf("failed to translate policies: %w", err)
	}

	// Increment resource version
	version := sm.store.IncrementResourceVersion()

	// Create new snapshot
	snapshot, err := cache.NewSnapshot(
		fmt.Sprintf("%d", version),
		resources,
	)
	if err != nil {
		sm.logger.Error("Failed to create snapshot", zap.Error(err))
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	// Validate snapshot consistency
	if err := snapshot.Consistent(); err != nil {
		sm.logger.Error("Snapshot is inconsistent", zap.Error(err))
		return fmt.Errorf("snapshot is inconsistent: %w", err)
	}

	// Set snapshot for the node
	if err := sm.cache.SetSnapshot(ctx, sm.nodeID, snapshot); err != nil {
		sm.logger.Error("Failed to set snapshot", zap.Error(err))
		return fmt.Errorf("failed to set snapshot: %w", err)
	}

	sm.logger.Info("Policy snapshot updated successfully",
		zap.String("version", fmt.Sprintf("%d", version)),
		zap.Int("policy_count", len(policies)))

	return nil
}

// Translator converts policy configurations to xDS resources
type Translator struct {
	logger *zap.Logger
}

// NewTranslator creates a new policy translator
func NewTranslator(logger *zap.Logger) *Translator {
	return &Translator{
		logger: logger,
	}
}

// TranslatePolicies translates policy configurations to xDS resources
// For now, we'll use runtime resources (RTDS - Runtime Discovery Service)
// which is perfect for dynamic configuration like policies
func (t *Translator) TranslatePolicies(policies []*models.StoredPolicyConfig) (map[resource.Type][]types.Resource, error) {
	resources := make(map[resource.Type][]types.Resource)

	// For policy data, we use the Runtime resource type
	// This allows Envoy to receive arbitrary configuration at runtime
	var runtimeResources []types.Resource

	for _, policy := range policies {
		// Convert policy to a runtime resource
		runtimeResource, err := t.createRuntimeResource(policy)
		if err != nil {
			t.logger.Error("Failed to create runtime resource for policy",
				zap.String("id", policy.ID),
				zap.Error(err))
			continue
		}

		runtimeResources = append(runtimeResources, runtimeResource)

		t.logger.Debug("Processing policy for xDS",
			zap.String("id", policy.ID),
			zap.String("api_name", policy.GetAPIName()),
			zap.String("version", policy.GetAPIVersion()),
			zap.Int("route_count", len(policy.Configuration.Routes)))
	}

	// Store runtime resources
	resources[resource.RuntimeType] = runtimeResources

	t.logger.Info("Translated policies to xDS resources",
		zap.Int("total_policies", len(policies)),
		zap.Int("runtime_resources", len(runtimeResources)))

	return resources, nil
}

// createRuntimeResource creates an Envoy Runtime resource from a policy configuration
func (t *Translator) createRuntimeResource(policy *models.StoredPolicyConfig) (types.Resource, error) {
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

	runtime := &runtimev3.Runtime{
		Name:  policy.ID,
		Layer: policyStruct,
	}

	return runtime, nil
}
