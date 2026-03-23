package main

import (
	"log/slog"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/transform"
)

// loadPoliciesFromAPIs transforms existing API configurations into RuntimeDeployConfigs
// and bulk-loads them into the runtime store. It writes directly to the store without
// triggering per-API snapshot updates; callers are responsible for triggering a snapshot
// after this returns.
func loadPoliciesFromAPIs(
	log *slog.Logger,
	cfg *config.Config,
	configStore *storage.ConfigStore,
	policyManager *policyxds.PolicyManager,
	policyDefinitions map[string]models.PolicyDefinition,
) {
	log.Info("Loading runtime deploy configs from stored API configurations")
	loadedAPIs := configStore.GetAll()
	loadedCount := 0
	runtimeStore := policyManager.GetRuntimeStore()

	for _, apiConfig := range loadedAPIs {
		transformer, ok := transform.Get(apiConfig.Kind)
		if !ok {
			log.Error("No transformer registered for API kind — skipping API during startup",
				slog.String("kind", apiConfig.Kind),
				slog.String("api_id", apiConfig.UUID),
				slog.String("name", apiConfig.Handle))
			continue
		}
		rdc, err := transformer.Transform(apiConfig)
		if err != nil {
			log.Warn("Failed to transform API config",
				slog.String("api_id", apiConfig.UUID),
				slog.String("kind", apiConfig.Kind),
				slog.Any("error", err))
			continue
		}
		key := storage.Key(apiConfig.Kind, apiConfig.Handle)
		runtimeStore.Set(key, rdc)
		loadedCount++
	}

	log.Info("Loaded runtime deploy configs from API configurations",
		slog.Int("total_apis", len(loadedAPIs)),
		slog.Int("configs_loaded", loadedCount))
}
