package xds

import (
	"context"
	"fmt"

	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
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
}

// NewSnapshotManager creates a new snapshot manager
func NewSnapshotManager(store *storage.ConfigStore, logger *zap.Logger, accessLogConfig config.AccessLogsConfig) *SnapshotManager {
	// Create a snapshot cache with a simple node ID hasher
	snapshotCache := cache.NewSnapshotCache(false, cache.IDHash{}, logger.Sugar())

	return &SnapshotManager{
		cache:          snapshotCache,
		translator:     NewTranslator(logger, accessLogConfig),
		store:          store,
		logger:         logger,
		nodeID:         "router-node",
		statusCallback: nil,
	}
}

// SetStatusCallback sets the callback for status updates
func (sm *SnapshotManager) SetStatusCallback(callback StatusUpdateCallback) {
	sm.statusCallback = callback
}

// UpdateSnapshot generates a new xDS snapshot from all configurations and updates the cache
// The correlationID parameter is optional and used for request tracing in logs
func (sm *SnapshotManager) UpdateSnapshot(ctx context.Context, correlationID string) error {
	// Create a logger with correlation ID if provided
	log := sm.logger
	if correlationID != "" {
		log = sm.logger.With(zap.String("correlation_id", correlationID))
	}
	// Get all configurations from in-memory store
	configs := sm.store.GetAll()

	// Translate configurations to Envoy resources
	resources, err := sm.translator.TranslateConfigs(configs, correlationID)
	if err != nil {
		log.Error("Failed to translate configurations", zap.Error(err))
		// Mark all pending configs as failed
		if sm.statusCallback != nil {
			for _, cfg := range configs {
				sm.statusCallback(cfg.ID, false, 0, correlationID)
			}
		}
		return fmt.Errorf("failed to translate configurations: %w", err)
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
