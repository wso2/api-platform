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
	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"
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
// Storage I/O is performed without holding the manager lock; the lock is only held when
// updating the cache to avoid blocking other readers during the full storage sweep.
func (sm *SnapshotManager) UpdateSnapshot(ctx context.Context) error {
	// Perform all storage I/O without holding the lock.
	configs, err := sm.store.GetAllConfigs()
	if err != nil {
		return fmt.Errorf("failed to load configs for subscription snapshot: %w", err)
	}

	plans, planErr := sm.store.ListSubscriptionPlans("")
	if planErr != nil {
		sm.logger.Warn("Failed to list subscription plans for snapshot, aborting update",
			slog.Any("error", planErr))
		return fmt.Errorf("failed to list subscription plans for snapshot: %w", planErr)
	}
	planMap := make(map[string]*models.SubscriptionPlan)
	for _, p := range plans {
		if p != nil {
			planMap[p.ID] = p
		}
	}

	allActiveSubs, err := sm.store.ListActiveSubscriptions()
	if err != nil {
		return fmt.Errorf("failed to list active subscriptions for snapshot: %w", err)
	}

	// Build apiID -> subscriptions map from bulk result.
	subsByAPI := make(map[string][]*models.Subscription)
	for _, s := range allActiveSubs {
		if s == nil {
			continue
		}
		subsByAPI[s.APIID] = append(subsByAPI[s.APIID], s)
	}

	// Build set of API IDs that exist in configs (RestApi kind only).
	apiIDs := make(map[string]bool)
	for _, cfg := range configs {
		if cfg != nil && cfg.Kind == "RestApi" {
			apiIDs[cfg.UUID] = true
		}
	}

	// Assemble SubscriptionData only for APIs that exist in configs.
	var subs []policyenginev1.SubscriptionData
	for apiID, list := range subsByAPI {
		if !apiIDs[apiID] {
			continue
		}
		for _, s := range list {
			if s == nil {
				continue
			}
			entry := policyenginev1.SubscriptionData{
				APIId:             s.APIID,
				SubscriptionToken: s.SubscriptionTokenHash,
				Status:            string(s.Status),
			}
			if s.ApplicationID != nil {
				entry.ApplicationId = *s.ApplicationID
			}
			if s.SubscriptionPlanID != nil {
				plan, ok := planMap[*s.SubscriptionPlanID]
				if !ok || plan == nil {
					sm.logger.Warn("Subscription references missing plan, aborting snapshot update",
						slog.String("subscription_id", s.ID),
						slog.String("plan_id", *s.SubscriptionPlanID))
					return fmt.Errorf("subscription %s references plan %s which is missing", s.ID, *s.SubscriptionPlanID)
				}
				if plan.ThrottleLimitCount != nil {
					entry.ThrottleLimitCount = *plan.ThrottleLimitCount
				}
				// nil ThrottleLimitCount means unlimited (0); policy skips rate limit when ThrottleLimitCount <= 0
				if plan.ThrottleLimitUnit != nil {
					entry.ThrottleLimitUnit = *plan.ThrottleLimitUnit
				}
				entry.StopOnQuotaReach = plan.StopOnQuotaReach
				entry.PlanName = plan.PlanName
			}
			if s.BillingCustomerID != nil {
				entry.BillingCustomerId = s.BillingCustomerID
			}
			if s.BillingSubscriptionID != nil {
				entry.BillingSubscriptionId = s.BillingSubscriptionID
			}
			subs = append(subs, entry)
		}
	}

	state := &policyenginev1.SubscriptionStateResource{
		Subscriptions: subs,
		Timestamp:     time.Now(),
	}

	// Hold lock only for cache update; version is computed inside lock to avoid races.
	sm.mu.Lock()
	defer sm.mu.Unlock()

	state.Version = sm.version + 1
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
