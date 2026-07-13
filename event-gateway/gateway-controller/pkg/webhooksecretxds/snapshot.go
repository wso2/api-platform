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

// Package webhooksecretxds manages xDS snapshots for WebSub HMAC webhook secrets.
// It serialises the in-memory WebhookSecretStore into an xDS resource and pushes
// it to connected event-gateway instances via a LinearCache.
package webhooksecretxds

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/wso2/api-platform/common/webhooksecret"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/logger"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

// WebhookSecretStateTypeURL is the xDS type URL for webhook secret state
// snapshots — re-exported from core's policyxds package, which is the single
// source of truth so core's CombinedCache dispatch and this producer never
// drift apart.
const WebhookSecretStateTypeURL = policyxds.WebhookSecretStateTypeURL

// SnapshotManager maintains an xDS LinearCache for webhook secret state and
// rebuilds it whenever the in-memory WebhookSecretStore changes.
type SnapshotManager struct {
	cache  *cache.LinearCache
	store  *webhooksecret.WebhookSecretStore
	logger *slog.Logger
	mu     sync.Mutex
}

// NewSnapshotManager creates a new webhook secret snapshot manager.
func NewSnapshotManager(store *webhooksecret.WebhookSecretStore, log *slog.Logger) *SnapshotManager {
	linearCache := cache.NewLinearCache(
		WebhookSecretStateTypeURL,
		cache.WithLogger(logger.NewXDSLogger(log)),
	)
	return &SnapshotManager{
		cache:  linearCache,
		store:  store,
		logger: log,
	}
}

// GetCache returns the underlying LinearCache as a generic Cache interface.
func (sm *SnapshotManager) GetCache() cache.Cache {
	return sm.cache
}

// RefreshSnapshot serialises the current store contents and pushes them to
// the LinearCache so all connected event-gateway instances receive an update.
func (sm *SnapshotManager) RefreshSnapshot() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	all := sm.store.GetAll()

	sm.logger.Info("Refreshing webhook secret xDS snapshot",
		slog.Int("api_count", len(all)))

	resource, err := buildResource(all)
	if err != nil {
		return fmt.Errorf("webhooksecretxds: failed to build resource: %w", err)
	}

	sm.cache.SetResources(map[string]types.Resource{
		"webhook-secret-state": resource,
	})

	sm.logger.Info("Webhook secret xDS snapshot updated",
		slog.Int("api_count", len(all)))
	return nil
}

// buildResource converts a map[apiId]map[name]plaintext to an anypb.Any resource.
// The serialisation mirrors the APIKeyState approach:
//
//	Go map → JSON → structpb.Struct → anypb.Any (with custom TypeURL)
func buildResource(secrets map[string]map[string]string) (types.Resource, error) {
	payload := map[string]any{
		"secrets": secrets,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	s := &structpb.Struct{}
	if err := s.UnmarshalJSON(jsonBytes); err != nil {
		return nil, fmt.Errorf("structpb unmarshal: %w", err)
	}

	structBytes, err := proto.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("proto marshal struct: %w", err)
	}

	return &anypb.Any{
		TypeUrl: WebhookSecretStateTypeURL,
		Value:   structBytes,
	}, nil
}
