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

// Command controller is the event-gateway-controller binary: gateway-controller
// (core) plus WebSub/WebBroker management-API support. It imports
// gateway-controller as a library (the abstraction layer) and supplies the
// concrete implementations for the extension points core exposes:
// xds.EventGatewayXDSHooks, policyxds.EventChannelTranslator,
// restapi.SetWebSubTopicDeregistrar, storage.SetWebSubTopicUpdater,
// eventlistener.WebhookSecretEventHandler, and policyxds.WebhookSecretCacheProvider
// / controlplane.WebhookSecretSnapshotRefresher.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/wso2/api-platform/common/authenticators"
	"github.com/wso2/api-platform/common/eventhub"
	commonmodels "github.com/wso2/api-platform/common/models"
	"github.com/wso2/api-platform/common/webhooksecret"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/adminserver"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/handlers"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/apikeyxds"
	coreconfig "github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/controlplane"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption/aesgcm"
	coreeventlistener "github.com/wso2/api-platform/gateway/gateway-controller/pkg/eventlistener"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/immutable"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/lazyresourcexds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/logger"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/secrets"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/restapi"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/subscriptionxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/transform"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	coreversion "github.com/wso2/api-platform/gateway/gateway-controller/pkg/version"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	gohttpkit "github.com/wso2/go-httpkit/middleware"

	eventgateway "github.com/wso2/api-platform/event-gateway/gateway-controller/pkg/api/eventgateway"
	eventgatewayconfig "github.com/wso2/api-platform/event-gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/event-gateway/gateway-controller/pkg/eventlistener"
	"github.com/wso2/api-platform/event-gateway/gateway-controller/pkg/handler"
	"github.com/wso2/api-platform/event-gateway/gateway-controller/pkg/hubtopic"
	"github.com/wso2/api-platform/event-gateway/gateway-controller/pkg/kindsupport"
	"github.com/wso2/api-platform/event-gateway/gateway-controller/pkg/policyhooks"
	"github.com/wso2/api-platform/event-gateway/gateway-controller/pkg/translator"
	"github.com/wso2/api-platform/event-gateway/gateway-controller/pkg/webhooksecretservice"
	"github.com/wso2/api-platform/event-gateway/gateway-controller/pkg/webhooksecretxds"
)

const (
	managementAPIBasePath = "/api/management/v1"
)

func toBackendConfig(cfg *coreconfig.Config) storage.BackendConfig {
	pg := cfg.Controller.Storage.EffectivePostgresConfig()
	ms := cfg.Controller.Storage.EffectiveSQLServerConfig()
	return storage.BackendConfig{
		Type:       cfg.Controller.Storage.Type,
		SQLitePath: cfg.Controller.Storage.EffectiveSQLitePath(),
		Postgres: storage.PostgresConnectionConfig{
			DSN:             pg.DSN,
			Host:            pg.Host,
			Port:            pg.Port,
			Database:        pg.Database,
			User:            pg.User,
			Password:        pg.Password,
			SSLMode:         pg.SSLMode,
			ConnectTimeout:  pg.ConnectTimeout,
			MaxOpenConns:    pg.MaxOpenConns,
			MaxIdleConns:    pg.MaxIdleConns,
			ConnMaxLifetime: pg.ConnMaxLifetime,
			ConnMaxIdleTime: pg.ConnMaxIdleTime,
			ApplicationName: pg.ApplicationName,
		},
		SQLServer: storage.SQLServerConnectionConfig{
			DSN:                    ms.DSN,
			Host:                   ms.Host,
			Port:                   ms.Port,
			Database:               ms.Database,
			User:                   ms.User,
			Password:               ms.Password,
			Encrypt:                cfg.Controller.Storage.SQLServerEncrypt(),
			TrustServerCertificate: cfg.Controller.Storage.SQLServerTrustServerCertificate(),
			ConnectTimeout:         ms.ConnectTimeout,
			MaxOpenConns:           ms.MaxOpenConns,
			MaxIdleConns:           ms.MaxIdleConns,
			ConnMaxLifetime:        ms.ConnMaxLifetime,
			ConnMaxIdleTime:        ms.ConnMaxIdleTime,
			ApplicationName:        ms.ApplicationName,
		},
		GatewayID: cfg.Controller.Server.GatewayID,
	}
}

func main() {
	configPath := flag.String("config", "", "Path to configuration file (required)")
	flag.Parse()

	if *configPath == "" {
		fmt.Fprintf(os.Stderr, "Error: -config flag is required\n")
		fmt.Fprintf(os.Stderr, "Usage: %s -config <path-to-config.toml>\n", os.Args[0])
		os.Exit(1)
	}

	// Register WebSubApi/WebBrokerApi kind support into core's registries
	// before any config is loaded/deployed/validated.
	kindsupport.Register()

	cfg, err := coreconfig.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration from %s: %v\n", *configPath, err)
		os.Exit(1)
	}

	// This module's own EventGatewayConfig section, loaded from the same file.
	eventGatewayCfg, err := eventgatewayconfig.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load event_gateway configuration from %s: %v\n", *configPath, err)
		os.Exit(1)
	}
	if eventGatewayCfg.Enabled {
		if err := eventGatewayCfg.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "Invalid event_gateway configuration: %v\n", err)
			os.Exit(1)
		}
	}

	metrics.SetEnabled(cfg.Controller.Metrics.Enabled)
	metrics.Init()

	log := logger.NewLogger(logger.Config{
		Level:  cfg.Controller.Logging.Level,
		Format: cfg.Controller.Logging.Format,
	})

	log.Info("Starting Event-Gateway-Controller",
		slog.String("version", coreversion.Version),
		slog.String("git_commit", coreversion.GitCommit),
		slog.String("build_date", coreversion.BuildDate),
		slog.String("config_file", *configPath),
		slog.String("storage_type", cfg.Controller.Storage.Type),
		slog.Bool("event_gateway_enabled", eventGatewayCfg.Enabled),
	)

	if !cfg.Controller.Auth.Basic.Enabled && !cfg.Controller.Auth.IDP.Enabled {
		log.Warn("No authentication configured: both basic auth and IDP are disabled. Event-Gateway-Controller API will allow all requests without authentication")
	}

	if cfg.ImmutableGateway.Enabled {
		if err := immutable.ResetSQLiteFiles(cfg.Controller.Storage.EffectiveSQLitePath(), log); err != nil {
			log.Error("Failed to reset SQLite files for immutable mode", slog.Any("error", err))
			os.Exit(1)
		}
	}

	var db storage.Storage
	db, err = storage.NewStorage(toBackendConfig(cfg), log)
	if err != nil {
		if strings.EqualFold(cfg.Controller.Storage.Type, "sqlite") && errors.Is(err, storage.ErrDatabaseLocked) {
			log.Error("Database is locked by another process", slog.Any("error", err))
			os.Exit(1)
		}
		log.Error("Failed to initialize database storage", slog.Any("error", err))
		os.Exit(1)
	}
	defer db.Close()

	var eventHubInstance eventhub.EventHub
	var eventHubStorage storage.Storage
	ehBackendCfg := toBackendConfig(cfg)
	ehBackendCfg.Postgres.MaxOpenConns = cfg.Controller.EventHub.Database.MaxOpenConns
	ehBackendCfg.Postgres.MaxIdleConns = cfg.Controller.EventHub.Database.MaxIdleConns
	ehBackendCfg.Postgres.ConnMaxLifetime = cfg.Controller.EventHub.Database.ConnMaxLifetime
	ehBackendCfg.Postgres.ConnMaxIdleTime = cfg.Controller.EventHub.Database.ConnMaxIdleTime
	ehBackendCfg.SQLServer.MaxOpenConns = cfg.Controller.EventHub.Database.MaxOpenConns
	ehBackendCfg.SQLServer.MaxIdleConns = cfg.Controller.EventHub.Database.MaxIdleConns
	ehBackendCfg.SQLServer.ConnMaxLifetime = cfg.Controller.EventHub.Database.ConnMaxLifetime
	ehBackendCfg.SQLServer.ConnMaxIdleTime = cfg.Controller.EventHub.Database.ConnMaxIdleTime
	eventHubStorage, err = storage.NewStorage(ehBackendCfg, log)
	if err != nil {
		log.Error("Failed to initialize EventHub storage", slog.Any("error", err))
		os.Exit(1)
	}
	eventHubDB := eventHubStorage.GetDB()

	gatewayID := strings.TrimSpace(cfg.Controller.Server.GatewayID)
	if eventHubDB == nil {
		log.Error("EventHub storage returned nil database handle")
		os.Exit(1)
	}
	if gatewayID == "" {
		log.Error("EventHub requires non-empty gateway ID")
		os.Exit(1)
	}
	eventHubInstance = eventhub.New(eventHubDB, log, eventhub.Config{
		PollInterval:    cfg.Controller.EventHub.PollInterval,
		CleanupInterval: cfg.Controller.EventHub.CleanupInterval,
		RetentionPeriod: cfg.Controller.EventHub.RetentionPeriod,
	})
	if err := eventHubInstance.Initialize(); err != nil {
		log.Error("Failed to initialize EventHub", slog.Any("error", err))
		os.Exit(1)
	}
	if err := eventHubInstance.RegisterGateway(gatewayID); err != nil {
		log.Error("Failed to register gateway with EventHub", slog.Any("error", err))
		os.Exit(1)
	}

	configStore := storage.NewConfigStore()

	// Sync WebSub topics into the shared TopicManager whenever a WebSubApi
	// config is stored (core's storage.ConfigStore.Add/Update calls this hook).
	configStore.SetWebSubTopicUpdater(kindsupport.UpdateTopicManager)

	apiKeyStore := storage.NewAPIKeyStore(log)
	apiKeySnapshotManager := apikeyxds.NewAPIKeySnapshotManager(apiKeyStore, log)
	apiKeyXDSManager := apikeyxds.NewAPIKeyStateManager(apiKeyStore, apiKeySnapshotManager, log)

	lazyResourceStore := storage.NewLazyResourceStore(log)
	lazyResourceSnapshotManager := lazyresourcexds.NewLazyResourceSnapshotManager(lazyResourceStore, log)
	lazyResourceXDSManager := lazyresourcexds.NewLazyResourceStateManager(lazyResourceStore, lazyResourceSnapshotManager, log)

	var encryptionProviderManager *encryption.ProviderManager
	var secretsService *secrets.SecretService

	if err := storage.LoadFromDatabase(db, configStore); err != nil {
		log.Error("Failed to load configurations from database", slog.Any("error", err))
		os.Exit(1)
	}
	if err := storage.LoadLLMProviderTemplatesFromDatabase(db, configStore); err != nil {
		log.Error("Failed to load llm provider template configurations from database", slog.Any("error", err))
		os.Exit(1)
	}

	if err := storage.LoadAPIKeysFromDatabase(db, configStore, apiKeyStore); err != nil {
		log.Error("Failed to load API keys from database", slog.Any("error", err))
		os.Exit(1)
	}

	var webhookSecretStore *webhooksecret.WebhookSecretStore
	var webhookSecretSnapshotManager *webhooksecretxds.SnapshotManager
	var webhookSecretService *webhooksecretservice.WebhookSecretService

	if len(cfg.Controller.Encryption.Providers) > 0 {
		var providers []encryption.EncryptionProvider
		for _, providerConfig := range cfg.Controller.Encryption.Providers {
			switch providerConfig.Type {
			case "aesgcm":
				var keyConfigs []aesgcm.KeyConfig
				for _, keyConf := range providerConfig.Keys {
					keyConfigs = append(keyConfigs, aesgcm.KeyConfig{
						Version:  keyConf.Version,
						FilePath: keyConf.FilePath,
					})
				}
				provider, err := aesgcm.NewAESGCMProvider(keyConfigs, log)
				if err != nil {
					log.Error("Failed to initialize AES-GCM provider", slog.Any("error", err))
					os.Exit(1)
				}
				providers = append(providers, provider)
			default:
				log.Error("Unsupported encryption provider type", slog.String("type", providerConfig.Type))
				os.Exit(1)
			}
		}

		encryptionProviderManager, err = encryption.NewProviderManager(providers, log)
		if err != nil {
			log.Error("Failed to initialize provider manager", slog.Any("error", err))
			os.Exit(1)
		}
		secretsService = secrets.NewSecretsService(db, encryptionProviderManager, log)

		// Webhook-secret (HMAC) infra — this binary owns it entirely; core only
		// knows about it through the WebhookSecretEventHandler /
		// WebhookSecretCacheProvider / WebhookSecretSnapshotRefresher interfaces.
		webhookSecretStore = webhooksecret.GetStoreInstance()
		webhookSecretSnapshotManager = webhooksecretxds.NewSnapshotManager(webhookSecretStore, log)
		webhookSecretService = webhooksecretservice.NewWebhookSecretService(db, encryptionProviderManager, webhookSecretStore, eventHubInstance, gatewayID, log)

		log.Info("Loading webhook secrets from database")
		if err := storage.LoadWebhookSecretsFromDatabase(db, encryptionProviderManager, webhookSecretStore); err != nil {
			log.Error("Failed to load webhook secrets from database", slog.Any("error", err))
			os.Exit(1)
		}
		if err := webhookSecretSnapshotManager.RefreshSnapshot(); err != nil {
			log.Warn("Failed to generate initial webhook secret xDS snapshot", slog.Any("error", err))
		}
	}

	policyLoader := utils.NewPolicyLoader(log)
	policyDir := cfg.Controller.Policies.DefinitionsPath
	policyDefinitions, err := policyLoader.LoadPoliciesFromDirectory(policyDir)
	if err != nil {
		log.Error("Failed to load policy definitions", slog.Any("error", err))
		os.Exit(1)
	}

	localPolicies, err := policyLoader.GetCustomPolicyNames(cfg.Controller.Policies.BuildManifestPath)
	if err != nil {
		log.Warn("Could not read build-manifest.yaml, Custom policies will not be marked in the gateway manifest", slog.Any("error", err))
	}
	for key, def := range policyDefinitions {
		def.ManagedBy = "wso2"
		if localPolicies[def.Name+"|"+def.Version] {
			def.ManagedBy = "organization"
		}
		policyDefinitions[key] = def
	}

	if err := hydrateStoredConfigsFromDatabaseOnStartup(
		configStore, db, &cfg.Router, policyDefinitions, log,
		cfg.Controller.Server.SkipInvalidDeploymentsOnStartup,
	); err != nil {
		log.Error("Failed to hydrate stored configurations required for startup", slog.Any("error", err))
		os.Exit(1)
	}

	snapshotManager := xds.NewSnapshotManager(configStore, log, &cfg.Router, db, cfg)

	// Wire the WebSub xDS translation hooks into the Envoy translator.
	snapshotManager.GetTranslator().SetEventGatewayXDSHooks(translator.New(eventGatewayCfg))

	var sdsSecretManager *xds.SDSSecretManager
	xdsTranslator := snapshotManager.GetTranslator()
	if xdsTranslator != nil && xdsTranslator.GetCertStore() != nil {
		sdsSecretManager = xds.NewSDSSecretManager(xdsTranslator.GetCertStore(), snapshotManager.GetCache(), "router-node", log)
		if err := sdsSecretManager.UpdateSecrets(); err != nil {
			log.Warn("Failed to initialize SDS secrets", slog.Any("error", err))
		} else {
			snapshotManager.SetSDSSecretManager(sdsSecretManager)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := snapshotManager.UpdateSnapshot(ctx, ""); err != nil {
		log.Warn("Failed to generate initial xDS snapshot", slog.Any("error", err))
	}
	cancel()

	routerConnected := make(chan struct{})
	policyEngineConnected := make(chan struct{})

	xdsServer := xds.NewServer(snapshotManager, sdsSecretManager, cfg.Controller.Server.XDSPort, log, routerConnected)
	go func() {
		if err := xdsServer.Start(); err != nil {
			log.Error("xDS server failed", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	if apiKeyXDSManager.GetAPIKeyCount() > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := apiKeySnapshotManager.UpdateSnapshot(ctx); err != nil {
			log.Warn("Failed to generate initial API key snapshot", slog.Any("error", err))
		}
		cancel()
	}

	policySnapshotManager := policyxds.NewSnapshotManager(log)
	runtimeStore := storage.NewRuntimeConfigStore()
	policySnapshotManager.SetRuntimeStore(runtimeStore)
	policySnapshotManager.SetConfigStore(configStore)

	// Wire the WebSub/WebBroker event-channel translation hooks.
	policySnapshotManager.GetTranslator().SetEventChannelHooks(policyhooks.New(log))

	subscriptionSnapshotManager := subscriptionxds.NewSnapshotManager(db, log)

	policyManager := policyxds.NewPolicyManager(policySnapshotManager, log)
	policyManager.SetRuntimeStore(runtimeStore)

	policyVersionResolver := utils.NewLoadedPolicyVersionResolver(policyDefinitions)
	restTransformer := transform.NewRestAPITransformer(&cfg.Router, cfg, policyDefinitions)
	llmTransformer := transform.NewLLMTransformer(configStore, db, &cfg.Router, cfg, policyDefinitions, policyVersionResolver)
	transformerRegistry := transform.NewRegistry(restTransformer, llmTransformer)
	policyManager.SetTransformers(transformerRegistry)

	xdsTranslator.SetTransformers(map[string]models.ConfigTransformer{
		"RestApi":     transformerRegistry,
		"Mcp":         transformerRegistry,
		"LlmProvider": transformerRegistry,
		"LlmProxy":    transformerRegistry,
	})

	loadedAPIs := configStore.GetAll()
	if _, err := loadRuntimeConfigsFromExistingAPIConfigurations(loadedAPIs, runtimeStore, secretsService, transformerRegistry, log, cfg.Controller.Server.SkipInvalidDeploymentsOnStartup); err != nil {
		log.Error("Failed to load runtime configs from API configurations", slog.Any("error", err))
		os.Exit(1)
	}

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	if err := policySnapshotManager.UpdateSnapshot(ctx); err != nil {
		log.Warn("Failed to generate initial policy xDS snapshot", slog.Any("error", err))
	}
	cancel()

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	if err := subscriptionSnapshotManager.UpdateSnapshot(ctx); err != nil {
		log.Warn("Failed to generate initial subscription xDS snapshot", slog.Any("error", err))
	}
	cancel()

	serverOpts := []policyxds.ServerOption{policyxds.WithOnFirstConnect(policyEngineConnected)}
	if cfg.Controller.PolicyServer.TLS.Enabled {
		serverOpts = append(serverOpts, policyxds.WithTLS(cfg.Controller.PolicyServer.TLS.CertFile, cfg.Controller.PolicyServer.TLS.KeyFile))
	}
	policyXDSServer := policyxds.NewServer(policySnapshotManager, apiKeySnapshotManager, lazyResourceSnapshotManager, subscriptionSnapshotManager, webhookSecretSnapshotManager, cfg.Controller.PolicyServer.Port, log, serverOpts...)
	go func() {
		if err := policyXDSServer.Start(); err != nil {
			log.Error("Policy xDS server failed", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	templateLoader := utils.NewLLMTemplateLoader(log)
	templateDefinitions, err := templateLoader.LoadTemplatesFromDirectory(cfg.Controller.LLM.TemplateDefinitionsPath)
	if err != nil {
		log.Error("Failed to load llm provider templates", slog.Any("error", err))
		os.Exit(1)
	}

	validator := coreconfig.NewAPIValidator()
	policyValidator := coreconfig.NewPolicyValidator(policyDefinitions)
	validator.SetPolicyValidator(policyValidator)

	apiSvc := utils.NewAPIDeploymentService(configStore, db, snapshotManager, validator, &cfg.Router, eventHubInstance, gatewayID, secretsService)
	mcpSvc := utils.NewMCPDeploymentService(configStore, db, snapshotManager, policyManager, policyValidator, eventHubInstance, gatewayID, secretsService)
	llmSvc := utils.NewLLMDeploymentService(configStore, db, snapshotManager, lazyResourceXDSManager, templateDefinitions, apiSvc, &cfg.Router, policyVersionResolver, policyValidator)

	cpClient := controlplane.NewClient(
		cfg.Controller.ControlPlane, log, configStore, db, snapshotManager, validator, &cfg.Router,
		apiKeyXDSManager, apiKeyStore, &cfg.APIKey, policyManager, cfg, policyDefinitions,
		lazyResourceXDSManager, templateDefinitions, subscriptionSnapshotManager, eventHubInstance,
		secretsService, webhookSecretStore, webhookSecretSnapshotManager,
	)
	if err := cpClient.Start(); err != nil {
		log.Error("Failed to start control plane client", slog.Any("error", err))
	}

	llmSvc.SetControlPlanePusher(cpClient, cfg.Controller.ControlPlane.DeploymentSyncEnabled)
	mcpSvc.SetControlPlanePusher(cpClient, cfg.Controller.ControlPlane.DeploymentSyncEnabled)

	httpClient := &http.Client{Timeout: 10 * time.Second}

	restAPIService := restapi.NewRestAPIService(
		configStore, db, snapshotManager, policyManager, apiSvc, apiKeyXDSManager,
		cpClient, &cfg.Router, cfg, httpClient, coreconfig.NewParser(), validator, log,
		eventHubInstance, secretsService,
	)
	// Deregister WebSub hub topics on delete.
	restAPIService.SetWebSubTopicDeregistrar(hubtopic.New(apiSvc, httpClient, eventGatewayCfg).Deregister)

	igw := immutable.NewImmutableGW(cfg.ImmutableGateway, restAPIService, llmSvc, mcpSvc)

	authConfig := generateAuthConfig(cfg)
	authMiddleWare, err := authenticators.AuthMiddleware(authConfig, log)
	if err != nil {
		log.Error("Failed to create auth middleware", slog.Any("error", err))
		os.Exit(1)
	}
	perRouteMiddlewares := []api.MiddlewareFunc{
		authenticators.AuthorizationMiddleware(authConfig, log),
		authMiddleWare,
	}
	eventGatewayPerRouteMiddlewares := []eventgateway.MiddlewareFunc{
		authenticators.AuthorizationMiddleware(authConfig, log),
		authMiddleWare,
	}

	// Event listener — multi-replica sync, with this binary's own webhook-secret handler wired in.
	evtListener := coreeventlistener.NewEventListener(
		eventHubInstance, configStore, db, snapshotManager, subscriptionSnapshotManager,
		apiKeyXDSManager, lazyResourceXDSManager, policyManager, &cfg.Router, log, cfg,
		policyDefinitions, secretsService,
	)
	if webhookSecretService != nil {
		evtListener.SetWebhookSecretHandler(eventlistener.NewWebhookSecretHandler(db, encryptionProviderManager, webhookSecretStore, webhookSecretSnapshotManager, log))
	}
	if err := evtListener.Start(); err != nil {
		log.Error("Failed to start event listener", slog.Any("error", err))
		os.Exit(1)
	}

	apiServer := handlers.NewAPIServer(
		configStore, db, snapshotManager, policyManager, lazyResourceXDSManager, log, cpClient,
		policyDefinitions, templateDefinitions, validator, apiKeyXDSManager, cfg, eventHubInstance,
		subscriptionSnapshotManager, secretsService, restAPIService,
	)

	eventGatewayHandler := handler.NewWebSubServer(handler.Deps{
		Store:                configStore,
		DB:                   db,
		DeploymentService:    apiSvc,
		APIKeyService:        utils.NewAPIKeyService(configStore, db, apiKeyXDSManager, &cfg.APIKey, eventHubInstance, gatewayID),
		ControlPlaneClient:   cpClient,
		SystemConfig:         cfg,
		EventGatewayConfig:   eventGatewayCfg,
		HTTPClient:           httpClient,
		Logger:               log,
		WebhookSecretService: webhookSecretService,
		DeploymentSearcher:   apiServer,
		EventHub:             eventHubInstance,
		GatewayID:            gatewayID,
	})

	if err := igw.LoadArtifacts(log); err != nil {
		log.Error("Failed to load immutable gateway artifacts", slog.Any("error", err))
		os.Exit(1)
	}

	if lazyResourceStore.Count() > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := lazyResourceSnapshotManager.UpdateSnapshot(ctx); err != nil {
			log.Warn("Failed to generate initial lazy resource snapshot", slog.Any("error", err))
		}
		cancel()
	}

	mux := http.NewServeMux()

	// Core management routes (versioned + legacy), identical to gateway-controller.
	api.HandlerWithOptions(apiServer, api.StdHTTPServerOptions{
		BaseURL:     managementAPIBasePath,
		BaseRouter:  mux,
		Middlewares: append(perRouteMiddlewares, igw.Middleware()),
	})
	api.HandlerWithOptions(apiServer, api.StdHTTPServerOptions{
		BaseURL:     "",
		BaseRouter:  mux,
		Middlewares: append(perRouteMiddlewares, igw.Middleware(), deprecatedManagementPathMiddleware(managementAPIBasePath)),
	})

	// WebSub/WebBroker routes (versioned + legacy), from this module's own spec.
	eventgateway.HandlerWithOptions(eventGatewayHandler, eventgateway.StdHTTPServerOptions{
		BaseURL:     managementAPIBasePath,
		BaseRouter:  mux,
		Middlewares: eventGatewayPerRouteMiddlewares,
	})
	eventgateway.HandlerWithOptions(eventGatewayHandler, eventgateway.StdHTTPServerOptions{
		BaseURL:     "",
		BaseRouter:  mux,
		Middlewares: append(eventGatewayPerRouteMiddlewares, deprecatedManagementPathMiddleware(managementAPIBasePath)),
	})

	outerMiddlewares := []func(http.Handler) http.Handler{
		middleware.CorrelationIDMiddleware(log),
		middleware.ErrorHandlingMiddleware(log),
		middleware.LoggingMiddleware(log),
	}
	if cfg.Controller.Metrics.Enabled {
		outerMiddlewares = append(outerMiddlewares, middleware.MetricsMiddleware())
	}
	httpHandler := gohttpkit.Chain(outerMiddlewares...)(mux)

	// Enable block/mutex profiling sampling when pprof is enabled. These are the
	// only profiles that need explicit rate setup; 0 leaves them disabled. Gated so
	// the sampling overhead is never paid unless pprof is deliberately turned on.
	if cfg.Controller.AdminServer.Pprof.Enabled {
		runtime.SetBlockProfileRate(cfg.Controller.AdminServer.Pprof.BlockProfileRate)
		runtime.SetMutexProfileFraction(cfg.Controller.AdminServer.Pprof.MutexProfileFraction)
	}

	// Start controller admin server for debug endpoints if enabled.
	var controllerAdminServer *adminserver.Server
	if cfg.Controller.AdminServer.Enabled {
		controllerAdminServer = adminserver.NewServer(&cfg.Controller.AdminServer, apiServer, log)
		go func() {
			if err := controllerAdminServer.Start(); err != nil {
				log.Error("Controller admin server failed", slog.Any("error", err))
				os.Exit(1)
			}
		}()
	}

	var metricsServer *metrics.Server
	var metricsCtxCancel context.CancelFunc
	if cfg.Controller.Metrics.Enabled {
		metrics.Info.WithLabelValues(coreversion.Version, cfg.Controller.Storage.Type, coreversion.BuildDate).Set(1)
		metricsServer = metrics.NewServer(&cfg.Controller.Metrics, log)
		if err := metricsServer.Start(); err != nil {
			log.Error("Metrics server failed", slog.Any("error", err))
			os.Exit(1)
		}
		var metricsCtx context.Context
		metricsCtx, metricsCtxCancel = context.WithCancel(context.Background())
		metrics.StartMemoryMetricsUpdater(metricsCtx, 15*time.Second)
	}

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Controller.Server.APIPort),
		Handler:           httpHandler,
		ReadHeaderTimeout: 30 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Failed to start REST API server", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	log.Info("Event-Gateway-Controller started successfully")

	go func() {
		<-routerConnected
		<-policyEngineConnected
		time.Sleep(1 * time.Second)
		fmt.Print("\n\n" +
			"========================================================================\n" +
			"\n" +
			"                 API Platform Event-Gateway-Controller Started\n" +
			"\n" +
			"========================================================================\n" +
			"\n\n")
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Event-Gateway-Controller")

	ctx, cancel = context.WithTimeout(context.Background(), cfg.Controller.Server.ShutdownTimeout)
	defer cancel()

	evtListener.Stop()
	if err := eventHubInstance.Close(); err != nil {
		log.Warn("Failed to close EventHub cleanly", slog.Any("error", err))
	}
	if eventHubStorage != nil {
		if err := eventHubStorage.Close(); err != nil {
			log.Warn("Failed to close EventHub storage cleanly", slog.Any("error", err))
		}
	}

	cpClient.Stop()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", slog.Any("error", err))
	}

	xdsServer.Stop()
	if policyXDSServer != nil {
		policyXDSServer.Stop()
	}

	if metricsServer != nil {
		if metricsCtxCancel != nil {
			metricsCtxCancel()
		}
		if err := metricsServer.Stop(ctx); err != nil {
			log.Error("Failed to stop metrics server", slog.Any("error", err))
		}
	}

	if controllerAdminServer != nil {
		if err := controllerAdminServer.Stop(ctx); err != nil {
			log.Error("Failed to stop controller admin server", slog.Any("error", err))
		}
	}

	log.Info("Event-Gateway-Controller stopped")
}

func generateAuthConfig(cfg *coreconfig.Config) commonmodels.AuthConfig {
	prefixed := func(methodAndPath string) string {
		idx := strings.Index(methodAndPath, " ")
		if idx < 0 {
			return methodAndPath
		}
		return methodAndPath[:idx+1] + managementAPIBasePath + methodAndPath[idx+1:]
	}

	relativeRoles := map[string][]string{
		"POST /websub-apis":        {"admin", "developer"},
		"GET /websub-apis":         {"admin", "developer"},
		"GET /websub-apis/{id}":    {"admin", "developer"},
		"PUT /websub-apis/{id}":    {"admin", "developer"},
		"DELETE /websub-apis/{id}": {"admin", "developer"},

		"POST /webbroker-apis":        {"admin", "developer"},
		"GET /webbroker-apis":         {"admin", "developer"},
		"GET /webbroker-apis/{id}":    {"admin", "developer"},
		"DELETE /webbroker-apis/{id}": {"admin", "developer"},

		"POST /websub-apis/{id}/api-keys":                         {"admin", "consumer"},
		"GET /websub-apis/{id}/api-keys":                          {"admin", "consumer"},
		"PUT /websub-apis/{id}/api-keys/{apiKeyName}":             {"admin", "consumer"},
		"POST /websub-apis/{id}/api-keys/{apiKeyName}/regenerate": {"admin", "consumer"},
		"DELETE /websub-apis/{id}/api-keys/{apiKeyName}":          {"admin", "consumer"},

		"POST /websub-apis/{id}/secrets":                         {"admin", "developer"},
		"GET /websub-apis/{id}/secrets":                          {"admin", "developer"},
		"DELETE /websub-apis/{id}/secrets/{secretName}":          {"admin", "developer"},
		"POST /websub-apis/{id}/secrets/{secretName}/regenerate": {"admin", "developer"},

		"POST /webbroker-apis/{id}/api-keys":                         {"admin", "consumer"},
		"GET /webbroker-apis/{id}/api-keys":                          {"admin", "consumer"},
		"PUT /webbroker-apis/{id}/api-keys/{apiKeyName}":             {"admin", "consumer"},
		"POST /webbroker-apis/{id}/api-keys/{apiKeyName}/regenerate": {"admin", "consumer"},
		"DELETE /webbroker-apis/{id}/api-keys/{apiKeyName}":          {"admin", "consumer"},
	}

	DefaultResourceRoles := make(map[string][]string, len(relativeRoles)*2)
	for methodAndPath, roles := range relativeRoles {
		DefaultResourceRoles[prefixed(methodAndPath)] = roles
		DefaultResourceRoles[methodAndPath] = roles
	}
	basicAuth := commonmodels.BasicAuth{Enabled: false}
	idpAuth := commonmodels.IDPConfig{Enabled: false}
	if cfg.Controller.Auth.Basic.Enabled {
		users := make([]commonmodels.User, len(cfg.Controller.Auth.Basic.Users))
		for i, authUser := range cfg.Controller.Auth.Basic.Users {
			users[i] = commonmodels.User{
				Username:       authUser.Username,
				Password:       authUser.Password,
				PasswordHashed: authUser.PasswordHashed,
				Roles:          authUser.Roles,
			}
		}
		basicAuth = commonmodels.BasicAuth{Enabled: true, Users: users}
	}
	if cfg.Controller.Auth.IDP.Enabled {
		idpAuth = commonmodels.IDPConfig{
			Enabled:           true,
			IssuerURL:         cfg.Controller.Auth.IDP.Issuer,
			JWKSUrl:           cfg.Controller.Auth.IDP.JWKSURL,
			ScopeClaim:        cfg.Controller.Auth.IDP.RolesClaim,
			PermissionMapping: &cfg.Controller.Auth.IDP.RoleMapping,
		}
	}
	return commonmodels.AuthConfig{
		BasicAuth:     &basicAuth,
		JWTConfig:     &idpAuth,
		ResourceRoles: DefaultResourceRoles,
	}
}

// deprecatedManagementPathMiddleware marks responses served on the legacy
// unprefixed management API paths as deprecated (RFC 8594). Mirrors core's
// gateway-controller/cmd/controller/main.go implementation exactly.
func deprecatedManagementPathMiddleware(newBasePath string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			successor := newBasePath + r.URL.Path
			w.Header().Set("Deprecation", "true")
			w.Header().Set("Link", fmt.Sprintf("<%s>; rel=\"successor-version\"", successor))
			w.Header().Set("Warning", fmt.Sprintf("299 - \"Deprecated API: migrate to %s prefix\"", newBasePath))
			next.ServeHTTP(w, r)
		})
	}
}
