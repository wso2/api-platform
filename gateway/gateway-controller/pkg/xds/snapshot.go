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

package xds

import (
	"context"
	"fmt"
	"time"

	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"go.uber.org/zap"
)

// StatusUpdateCallback is called after xDS snapshot update completes
type StatusUpdateCallback func(configID string, success bool, version int64, correlationID string)

// SnapshotManager manages xDS snapshots for Envoy
type SnapshotManager struct {
	cache            cache.SnapshotCache
	translator       *Translator
	store            *storage.ConfigStore
	logger           *zap.Logger
	nodeID           string // Node ID for Envoy (default: "router-node")
	statusCallback   StatusUpdateCallback
	sdsSecretManager *SDSSecretManager
}

// NewSnapshotManager creates a new snapshot manager
func NewSnapshotManager(store *storage.ConfigStore, logger *zap.Logger, routerConfig *config.RouterConfig, db storage.Storage, cfg *config.Config) *SnapshotManager {
	// Create a snapshot cache with a simple node ID hasher
	snapshotCache := cache.NewSnapshotCache(false, cache.IDHash{}, logger.Sugar())

	return &SnapshotManager{
		cache:            snapshotCache,
		translator:       NewTranslator(logger, routerConfig, db, cfg),
		store:            store,
		logger:           logger,
		nodeID:           "router-node",
		statusCallback:   nil,
		sdsSecretManager: nil,
	}
}

// SetSDSSecretManager sets the SDS secret manager
func (sm *SnapshotManager) SetSDSSecretManager(sdsSecretManager *SDSSecretManager) {
	sm.sdsSecretManager = sdsSecretManager
}

// SetStatusCallback sets the callback for status updates
func (sm *SnapshotManager) SetStatusCallback(callback StatusUpdateCallback) {
	sm.statusCallback = callback
}

// UpdateSnapshot generates a new xDS snapshot from all configurations and updates the cache
// The correlationID parameter is optional and used for request tracing in logs
func (sm *SnapshotManager) UpdateSnapshot(ctx context.Context, correlationID string) error {
	startTime := time.Now()
	trigger := "manual"
	if correlationID != "" {
		trigger = "api_update"
	}

	// Create a logger with correlation ID if provided
	log := sm.logger
	if correlationID != "" {
		log = sm.logger.With(zap.String("correlation_id", correlationID))
	}
	// Get all configurations from in-memory store
	configs := sm.store.GetAll()

	// Translate configurations to Envoy resources if this is not event gw
	//resources, err := sm.translator.TranslateConfigs(configs, correlationID)
	// If event gw,
	resources, err := sm.translator.TranslateConfigs(configs, correlationID)
	if err != nil {
		log.Error("Failed to translate configurations", zap.Error(err))
		metrics.SnapshotGenerationTotal.WithLabelValues("main", "error", trigger).Inc()
		metrics.TranslationErrorsTotal.WithLabelValues("translation_failed").Inc()
		// Mark all pending configs as failed
		if sm.statusCallback != nil {
			for _, cfg := range configs {
				sm.statusCallback(cfg.ID, false, 0, correlationID)
			}
		}
		return fmt.Errorf("failed to translate configurations: %w", err)
	}

	// Add SDS secrets if SDS secret manager is configured
	if sm.sdsSecretManager != nil {
		secret, err := sm.sdsSecretManager.GetSecret()
		if err != nil {
			log.Warn("Failed to get SDS secret, continuing without it", zap.Error(err))
		} else {
			resources[resource.SecretType] = []types.Resource{secret}
			log.Debug("Added SDS secret to snapshot", zap.String("secret_name", SecretNameUpstreamCA))
		}
	}

	// Increment snapshot version
	version := sm.store.IncrementSnapshotVersion()

	// Create new snapshot
	snapshot, err := cache.NewSnapshot(
		fmt.Sprintf("%d", version),
		resources,
	)
	if err != nil {
		log.Error("Failed to create snapshot", zap.Error(err))
		metrics.SnapshotGenerationTotal.WithLabelValues("main", "error", trigger).Inc()
		metrics.TranslationErrorsTotal.WithLabelValues("snapshot_create_failed").Inc()
		// Mark all pending configs as failed
		if sm.statusCallback != nil {
			for _, cfg := range configs {
				sm.statusCallback(cfg.ID, false, 0, correlationID)
			}
		}
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	// Validate snapshot consistency
	if err := snapshot.Consistent(); err != nil {
		log.Error("Snapshot is inconsistent", zap.Error(err))
		metrics.SnapshotGenerationTotal.WithLabelValues("main", "error", trigger).Inc()
		metrics.TranslationErrorsTotal.WithLabelValues("snapshot_inconsistent").Inc()
		// Mark all pending configs as failed
		if sm.statusCallback != nil {
			for _, cfg := range configs {
				sm.statusCallback(cfg.ID, false, 0, correlationID)
			}
		}
		return fmt.Errorf("snapshot is inconsistent: %w", err)
	}

	// Update cache with new snapshot
	if err := sm.cache.SetSnapshot(ctx, sm.nodeID, snapshot); err != nil {
		log.Error("Failed to set snapshot", zap.Error(err))
		metrics.SnapshotGenerationTotal.WithLabelValues("main", "error", trigger).Inc()
		metrics.TranslationErrorsTotal.WithLabelValues("cache_set_failed").Inc()
		// Mark all pending configs as failed
		if sm.statusCallback != nil {
			for _, cfg := range configs {
				sm.statusCallback(cfg.ID, false, 0, correlationID)
			}
		}
		return fmt.Errorf("failed to set snapshot: %w", err)
	}

	log.Info("Updated xDS snapshot",
		zap.Int64("version", version),
		zap.Int("num_configs", len(configs)),
		zap.Int("num_listeners", len(resources[resource.ListenerType])),
		zap.Int("num_routes", len(resources[resource.RouteType])),
		zap.Int("num_clusters", len(resources[resource.ClusterType])),
	)

	// Record successful snapshot generation metrics
	duration := time.Since(startTime)
	metrics.SnapshotGenerationTotal.WithLabelValues("main", "success", trigger).Inc()
	metrics.SnapshotGenerationDurationSeconds.WithLabelValues("main").Observe(duration.Seconds())

	// Record snapshot sizes
	metrics.SnapshotSize.WithLabelValues("listener").Set(float64(len(resources[resource.ListenerType])))
	metrics.SnapshotSize.WithLabelValues("route").Set(float64(len(resources[resource.RouteType])))
	metrics.SnapshotSize.WithLabelValues("cluster").Set(float64(len(resources[resource.ClusterType])))
	metrics.SnapshotSize.WithLabelValues("endpoint").Set(float64(len(resources[resource.EndpointType])))

	// Record routes per API metric (if there are APIs)
	if len(configs) > 0 && len(resources[resource.RouteType]) > 0 {
		routesPerAPI := float64(len(resources[resource.RouteType])) / float64(len(configs))
		metrics.RoutesPerAPI.Observe(routesPerAPI)
	}

	// Mark all successfully deployed configs
	if sm.statusCallback != nil {
		for _, cfg := range configs {
			sm.statusCallback(cfg.ID, true, version, correlationID)
		}
	}

	return nil
}

// GetCache returns the snapshot cache for use by xDS server
func (sm *SnapshotManager) GetCache() cache.SnapshotCache {
	return sm.cache
}

// GetTranslator returns the translator instance
func (sm *SnapshotManager) GetTranslator() *Translator {
	return sm.translator
}
