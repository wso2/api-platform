/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package subscriptionxds

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	cache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/logger"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	policyenginev1 "github.com/wso2/api-platform/sdk/gateway/policyengine/v1"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	// SubscriptionStateTypeURL is the custom type URL for subscription state configurations.
	SubscriptionStateTypeURL = "api-platform.wso2.org/v1.SubscriptionState"
)

// SnapshotManager manages xDS snapshots for subscription state.
type SnapshotManager struct {
	cache  *cache.LinearCache
	store  storage.Storage
	logger *slog.Logger
	mu     sync.RWMutex
	// version is incremented on each successful snapshot update.
	version int64
}

// NewSnapshotManager creates a new subscription snapshot manager backed by the given storage.
func NewSnapshotManager(store storage.Storage, log *slog.Logger) *SnapshotManager {
	if log == nil {
		log = slog.Default()
	}

	linearCache := cache.NewLinearCache(
		SubscriptionStateTypeURL,
		cache.WithLogger(logger.NewXDSLogger(log)),
	)

	return &SnapshotManager{
		cache:   linearCache,
		store:   store,
		logger:  log,
		version: 0,
	}
}

// GetCache returns the underlying cache as the generic Cache interface.
func (sm *SnapshotManager) GetCache() cache.Cache {
	return sm.cache
}

// UpdateSnapshot generates a new xDS snapshot from all ACTIVE subscriptions for this gateway.
func (sm *SnapshotManager) UpdateSnapshot(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.store == nil {
		// In memory-only mode there is no persistent storage; skip snapshot generation gracefully.
		return nil
	}

	// Storage is already scoped by gatewayId internally; ListSubscriptionsByAPI requires an API id.
	// Instead of scanning by API, we load all configs and for each API we fetch its ACTIVE subscriptions.
	configs, err := sm.store.GetAllConfigs()
	if err != nil {
		return fmt.Errorf("failed to load configs for subscription snapshot: %w", err)
	}

	var subs []policyenginev1.SubscriptionData
	for _, cfg := range configs {
		if cfg == nil {
			continue
		}
		// Only REST APIs participate in subscription validation.
		if cfg.Configuration.Kind != "RestApi" {
			continue
		}
		apiID := cfg.ID
		status := string(models.SubscriptionStatusActive)
		list, err := sm.store.ListSubscriptionsByAPI(apiID, "", nil, &status)
		if err != nil {
			sm.logger.Warn("Failed to list subscriptions for API when building snapshot",
				slog.String("api_id", apiID),
				slog.Any("error", err))
			return fmt.Errorf("failed to list subscriptions for API %s when building snapshot: %w", apiID, err)
		}
		for _, s := range list {
			if s == nil {
				continue
			}
			subs = append(subs, policyenginev1.SubscriptionData{
				APIId:         s.APIID,
				ApplicationId: s.ApplicationID,
				Status:        string(s.Status),
			})
		}
	}

	state := &policyenginev1.SubscriptionStateResource{
		Subscriptions: subs,
		Version:       sm.version + 1,
		Timestamp:     time.Now(),
	}

	resource, err := createSubscriptionStateResource(state)
	if err != nil {
		return fmt.Errorf("failed to create subscription state resource: %w", err)
	}

	resourcesByID := map[string]types.Resource{
		"subscription-state": resource,
	}
	sm.cache.SetResources(resourcesByID)
	sm.version++

	sm.logger.Info("Subscription snapshot updated successfully",
		slog.Int("subscription_count", len(subs)),
		slog.Int64("version", sm.version))

	return nil
}

// createSubscriptionStateResource converts the state to an xDS resource.
func createSubscriptionStateResource(state *policyenginev1.SubscriptionStateResource) (types.Resource, error) {
	jsonBytes, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal subscription state: %w", err)
	}

	st := &structpb.Struct{}
	if err := st.UnmarshalJSON(jsonBytes); err != nil {
		return nil, fmt.Errorf("failed to convert subscription state to struct: %w", err)
	}

	anyMsg, err := anypb.New(st)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap subscription state in Any: %w", err)
	}
	anyMsg.TypeUrl = SubscriptionStateTypeURL
	return anyMsg, nil
}
