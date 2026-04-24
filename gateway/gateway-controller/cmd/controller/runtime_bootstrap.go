package main

import (
	"fmt"
	"log/slog"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine/funcs"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

func hydrateStoredConfigsFromDatabaseOnStartup(
	configStore *storage.ConfigStore,
	db storage.Storage,
	routerConfig *config.RouterConfig,
	policyDefinitions map[string]models.PolicyDefinition,
	log *slog.Logger,
	skipInvalidDeployments bool,
) error {
	if configStore == nil {
		return nil
	}

	if err := hydrateConfigsByKindForStartup(
		configStore.GetAllByKind(string(api.MCPProxyConfigurationKindMcp)),
		"stored MCP proxy configuration",
		log,
		skipInvalidDeployments,
		utils.HydrateStoredMCPConfig,
	); err != nil {
		return err
	}

	if err := hydrateConfigsByKindForStartup(
		configStore.GetAllByKind(string(api.LLMProviderConfigurationKindLlmProvider)),
		"stored LLM provider configuration",
		log,
		skipInvalidDeployments,
		func(cfg *models.StoredConfig) error {
			return utils.HydrateLLMConfig(cfg, configStore, db, routerConfig, policyDefinitions)
		},
	); err != nil {
		return err
	}

	return hydrateConfigsByKindForStartup(
		configStore.GetAllByKind(string(api.LLMProxyConfigurationKindLlmProxy)),
		"stored LLM proxy configuration",
		log,
		skipInvalidDeployments,
		func(cfg *models.StoredConfig) error {
			return utils.HydrateLLMConfig(cfg, configStore, db, routerConfig, policyDefinitions)
		},
	)
}

func hydrateConfigsByKindForStartup(
	configs []*models.StoredConfig,
	description string,
	log *slog.Logger,
	skipInvalidDeployments bool,
	hydrate func(*models.StoredConfig) error,
) error {
	for _, storedCfg := range configs {
		if err := hydrate(storedCfg); err != nil {
			if log != nil {
				logFn := log.Error
				message := "Failed to hydrate " + description
				if skipInvalidDeployments {
					logFn = log.Warn
					message = "Skipping invalid " + description + " during startup"
				}
				logFn(message,
					slog.String("id", storedCfg.UUID),
					slog.String("handle", storedCfg.Handle),
					slog.Any("error", err))
			}
			if !skipInvalidDeployments {
				return fmt.Errorf("failed to hydrate %s: %w", description, err)
			}
		}
	}

	return nil
}

func loadRuntimeConfigsFromExistingAPIConfigurations(
	loadedConfigs []*models.StoredConfig,
	runtimeStore *storage.RuntimeConfigStore,
	secretResolver funcs.SecretResolver,
	transformer models.ConfigTransformer,
	log *slog.Logger,
	skipInvalidDeployments bool,
) (int, error) {
	if runtimeStore == nil || transformer == nil {
		return 0, nil
	}

	loadedCount := 0
	for _, apiConfig := range loadedConfigs {
		if apiConfig == nil || !supportsRuntimeBootstrapKind(apiConfig.Kind) {
			continue
		}

		if secretResolver != nil {
			if err := templateengine.RenderSpec(apiConfig, secretResolver, log); err != nil {
				if log != nil {
					if skipInvalidDeployments {
						log.Warn("Template rendering failed during startup load, skipping",
							slog.String("api_id", apiConfig.UUID),
							slog.Any("error", err),
						)
					} else {
						log.Error("Template rendering failed during startup load",
							slog.String("api_id", apiConfig.UUID),
							slog.Any("error", err),
						)
					}
				}
				if skipInvalidDeployments {
					continue
				}
				return loadedCount, fmt.Errorf("failed to render config for startup %s: %w", apiConfig.UUID, err)
			}
		}

		rdc, err := transformer.Transform(apiConfig)
		if err != nil {
			if log != nil {
				if skipInvalidDeployments {
					log.Warn("Failed to transform API config at startup",
						slog.String("api_id", apiConfig.UUID),
						slog.String("kind", apiConfig.Kind),
						slog.Any("error", err))
				} else {
					log.Error("Failed to transform API config at startup",
						slog.String("api_id", apiConfig.UUID),
						slog.String("kind", apiConfig.Kind),
						slog.Any("error", err))
				}
			}
			if skipInvalidDeployments {
				continue
			}
			return loadedCount, fmt.Errorf(
				"failed to transform startup config %s (%s): %w",
				apiConfig.UUID,
				apiConfig.Kind,
				err,
			)
		}

		runtimeStore.Set(storage.Key(apiConfig.Kind, apiConfig.Handle), rdc)
		loadedCount++
	}

	return loadedCount, nil
}

func supportsRuntimeBootstrapKind(kind string) bool {
	switch kind {
	case models.KindRestApi, models.KindMcp, models.KindLlmProvider, models.KindLlmProxy:
		return true
	default:
		return false
	}
}
