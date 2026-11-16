package main

import (
	"fmt"
	"log/slog"
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine/funcs"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/transform"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/upstreamguard"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

func hydrateStoredConfigsFromDatabaseOnStartup(
	configStore *storage.ConfigStore,
	db storage.Storage,
	routerConfig *config.RouterConfig,
	policyDefinitions map[string]models.PolicyDefinition,
	policyVersionResolver utils.PolicyVersionResolver,
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
		func(cfg *models.StoredConfig) error {
			return utils.HydrateStoredMCPConfig(cfg, policyVersionResolver)
		},
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

// loadStartupRuntimeConfigs loads the stored artifacts into runtimeStore at
// startup and logs what the loaded set implies. It runs before the initial
// router snapshot: loading renders each artifact, and one the gateway
// configuration refuses (an environment variable outside policy_engine.env_access,
// or a URL network access control does not permit) is undeployed and handed to
// onRefused to persist, so the first snapshot never routes it.
func loadStartupRuntimeConfigs(
	loadedAPIs []*models.StoredConfig,
	runtimeStore *storage.RuntimeConfigStore,
	secretResolver funcs.SecretResolver,
	transformer models.ConfigTransformer,
	routerConfig *config.RouterConfig,
	gatewayJWKSEnabled bool,
	skipInvalidDeployments bool,
	onRefused func(*models.StoredConfig) error,
	log *slog.Logger,
) error {
	refused := 0
	loadedCount, err := loadRuntimeConfigsFromExistingAPIConfigurations(
		loadedAPIs, runtimeStore, secretResolver, transformer, log, skipInvalidDeployments,
		func(c *models.StoredConfig) error {
			refused++
			if onRefused == nil {
				return nil
			}
			return onRefused(c)
		},
	)
	if err != nil {
		return err
	}
	if log == nil {
		return nil
	}
	log.Info("Loaded runtime configs from API configurations",
		slog.Int("total_apis", len(loadedAPIs)),
		slog.Int("configs_loaded", loadedCount),
		slog.Int("undeployed_as_refused", refused))
	warnMtlsAPIsWithoutDedicatedHostname(loadedAPIs, routerConfig, log)
	if gatewayJWKSEnabled && runtimeStore != nil {
		if conflicts := transform.GatewayJWKSConflicts(runtimeStore.GetAll()); len(conflicts) > 0 {
			log.Warn("Deployed routes use the gateway JWKS path and are shadowed by the gateway JWKS endpoint; "+
				"move them to another path, or set [signing] jwks_endpoint_enabled = false to keep serving them",
				slog.String("path", constants.GatewayJWKSPath),
				slog.Any("routes", conflicts))
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
	onEnvRefused func(*models.StoredConfig) error,
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
				// A stored artifact the gateway configuration now refuses — it reads
				// an environment variable outside the env access allow list, or names
				// a URL network access control does not permit — is undeployed rather
				// than failing startup or being left with a route but no policy chain,
				// whatever skipInvalidDeployments says. Other artifacts keep loading.
				if upstreamguard.IsArtifactRefusal(err) {
					utils.UndeployRefusedAtStartup(apiConfig, onEnvRefused, upstreamguard.RefusalReason(err), log)
					continue
				}
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
	case models.KindRestApi, models.KindMcp, models.KindLlmProvider, models.KindLlmProxy, models.KindAgent, models.KindGraphQLApi:
		return true
	default:
		return false
	}
}

// warnMtlsAPIsWithoutDedicatedHostname logs one warning per stored mtls-auth
// API that router.downstream_tls.mtls_requires_dedicated_hostname would refuse
// to deploy. Such an API keeps being served. Pass rendered configurations.
// With client_certificate_request all_connections the requirement has no
// effect, and one warning says so instead.
func warnMtlsAPIsWithoutDedicatedHostname(configs []*models.StoredConfig, routerConfig *config.RouterConfig, log *slog.Logger) {
	if log == nil || routerConfig == nil || !routerConfig.DownstreamTLS.MtlsRequiresDedicatedHostname {
		return
	}
	if routerConfig.DownstreamTLS.AsksAllConnections() {
		log.Warn("router.downstream_tls.mtls_requires_dedicated_hostname has no effect while " +
			"router.downstream_tls.client_certificate_request is all_connections")
		return
	}
	if !routerConfig.HTTPSEnabled {
		return
	}
	for _, cfg := range configs {
		if cfg == nil || cfg.Kind != models.KindRestApi {
			continue
		}
		restCfg, ok := cfg.Configuration.(api.RestAPI)
		if !ok {
			continue
		}
		found := config.UndedicatedHostnames(restCfg, routerConfig.VHosts)
		if len(found) == 0 {
			continue
		}
		settings := make([]string, 0, len(found))
		for _, h := range found {
			settings = append(settings, h.Setting)
		}
		log.Warn(fmt.Sprintf("mtls-auth API %s has no dedicated hostname; the HTTPS listener asks every connection "+
			"for a client certificate; update it to set %s", cfg.Handle, strings.Join(settings, " and ")),
			slog.String("handle", cfg.Handle),
			slog.String("id", cfg.UUID))
	}
}

// installArtifactAccessControl installs controller.server.network_security.access_control
// as RenderSpec's post-render check, or clears the check when it is disabled. It
// returns an error for an invalid configuration, which cfg.Validate has normally
// rejected already.
func installArtifactAccessControl(cfg *config.Config, policyDefinitions map[string]models.PolicyDefinition, log *slog.Logger) error {
	policy, enabled, err := cfg.ArtifactAccessControlPolicy()
	if err != nil {
		return err
	}
	if !enabled {
		templateengine.SetPostRenderCheck(nil)
		log.Info("Network access control for artifact URLs is disabled")
		return nil
	}
	guard := upstreamguard.New(policy, policyDefinitions, log)
	templateengine.SetPostRenderCheck(guard.CheckArtifact)
	ac := cfg.Controller.Server.NetworkSecurity.AccessControl
	log.Info("Network access control for artifact URLs is enabled",
		slog.String("mode", ac.Mode),
		slog.Int("hostname_patterns", len(ac.Hostnames)),
		slog.Bool("deny_private_networks", ac.DenyPrivateNetworks))
	return nil
}
