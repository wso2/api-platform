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

package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wso2/api-platform/common/authenticators"
	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/adminserver"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/handlers"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/apikeyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/controllerext"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/controlplane"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption/aesgcm"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/eventlistener"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/immutable"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/lazyresourcexds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/logger"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/secrets"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/restapi"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/transform"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/version"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

// API base paths for the gateway-controller HTTP surfaces.
// These must stay in sync with the `servers.url` values in the OpenAPI specs
// (api/management-openapi.yaml and api/admin-openapi.yaml).
const (
	ManagementAPIBasePath = "/api/management/v0.9"
)

// ToBackendConfig converts a top-level config into a storage backend config.
func ToBackendConfig(cfg *config.Config) storage.BackendConfig {
	pg := cfg.Controller.Storage.Postgres
	return storage.BackendConfig{
		Type:       cfg.Controller.Storage.Type,
		SQLitePath: cfg.Controller.Storage.SQLite.Path,
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
		GatewayID: cfg.Controller.Server.GatewayID,
	}
}

// DeprecatedManagementPathMiddleware returns a Gin middleware that marks
// responses served on the legacy unprefixed management API paths as
// deprecated, following RFC 8594.
func DeprecatedManagementPathMiddleware(newBasePath string) api.MiddlewareFunc {
	return func(c *gin.Context) {
		successor := newBasePath + c.Request.URL.Path
		c.Writer.Header().Set("Deprecation", "true")
		c.Writer.Header().Set("Link", fmt.Sprintf("<%s>; rel=\"successor-version\"", successor))
		c.Writer.Header().Set("Warning",
			fmt.Sprintf("299 - \"Deprecated API: migrate to %s prefix\"", newBasePath))
		c.Next()
	}
}

// Run wires all gateway-controller components and starts the server.
// It blocks until a SIGINT/SIGTERM is received, then performs graceful shutdown.
// The ext argument is used to inject extension-specific components (e.g. event-gateway).
// Pass controllerext.NoOpExtension{} for the base controller.
func Run(configPath string, ext controllerext.ControllerExtension) {
	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration from %s: %v\n", configPath, err)
		os.Exit(1)
	}

	// Initialize metrics based on configuration
	metrics.SetEnabled(cfg.Controller.Metrics.Enabled)
	metrics.Init()

	// Initialize logger with config
	log := logger.NewLogger(logger.Config{
		Level:  cfg.Controller.Logging.Level,
		Format: cfg.Controller.Logging.Format,
	})

	log.Info("Starting Gateway-Controller",
		slog.String("version", version.Version),
		slog.String("git_commit", version.GitCommit),
		slog.String("build_date", version.BuildDate),
		slog.String("config_file", configPath),
		slog.String("storage_type", cfg.Controller.Storage.Type),
		slog.Bool("access_logs_enabled", cfg.Router.AccessLogs.Enabled),
		slog.String("control_plane_host", cfg.Controller.ControlPlane.Host),
		slog.Bool("control_plane_token_configured", cfg.Controller.ControlPlane.Token != ""),
		slog.Bool("skip_invalid_deployments_on_startup", cfg.Controller.Server.SkipInvalidDeploymentsOnStartup),
		slog.String("extension", ext.Name()),
	)

	if !cfg.Controller.Auth.Basic.Enabled && !cfg.Controller.Auth.IDP.Enabled {
		log.Warn("No authentication configured: both basic auth and IDP are disabled. Gateway Controller API will allow all requests without authentication")
	}

	// In immutable mode, delete any stale SQLite files before opening the DB to
	// guarantee a fresh, reproducible state on every boot.
	if cfg.ImmutableGateway.Enabled {
		log.Info("Immutable gateway mode enabled — removing existing SQLite files for fresh start",
			slog.String("path", cfg.Controller.Storage.SQLite.Path))
		if err := immutable.ResetSQLiteFiles(cfg.Controller.Storage.SQLite.Path, log); err != nil {
			log.Error("Failed to reset SQLite files for immutable mode", slog.Any("error", err))
			os.Exit(1)
		}
	}

	// Initialize storage; apply base schema then any extension-specific schema SQL.
	backendCfg := ToBackendConfig(cfg)
	var db storage.Storage
	db, err = storage.NewStorage(backendCfg, log, ext.AdditionalSchemaSQL(backendCfg.Type)...)
	if err != nil {
		if strings.EqualFold(cfg.Controller.Storage.Type, "sqlite") && errors.Is(err, storage.ErrDatabaseLocked) {
			log.Error("Database is locked by another process",
				slog.String("database_path", cfg.Controller.Storage.SQLite.Path),
				slog.String("troubleshooting", "Check if another gateway-controller instance is running or remove stale WAL files"))
			os.Exit(1)
		}
		log.Error("Failed to initialize database storage",
			slog.String("type", cfg.Controller.Storage.Type),
			slog.Any("error", err))
		os.Exit(1)
	}
	defer db.Close()

	// Initialize EventHub for multi-replica sync (requires persistent storage)
	var eventHubInstance eventhub.EventHub
	var eventHubStorage storage.Storage
	ehBackendCfg := ToBackendConfig(cfg)
	ehBackendCfg.Postgres.MaxOpenConns = cfg.Controller.EventHub.Database.MaxOpenConns
	ehBackendCfg.Postgres.MaxIdleConns = cfg.Controller.EventHub.Database.MaxIdleConns
	ehBackendCfg.Postgres.ConnMaxLifetime = cfg.Controller.EventHub.Database.ConnMaxLifetime
	ehBackendCfg.Postgres.ConnMaxIdleTime = cfg.Controller.EventHub.Database.ConnMaxIdleTime
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
	log.Info("EventHub initialized for multi-replica sync",
		slog.String("gateway_id", gatewayID))

	// Initialize in-memory config store
	configStore := storage.NewConfigStore()

	// Initialize in-memory API key store for xDS
	apiKeyStore := storage.NewAPIKeyStore(log)
	apiKeySnapshotManager := apikeyxds.NewAPIKeySnapshotManager(apiKeyStore, log)
	apiKeyXDSManager := apikeyxds.NewAPIKeyStateManager(apiKeyStore, apiKeySnapshotManager, log)

	// Initialize in-memory lazy resource store and components for xDS
	lazyResourceStore := storage.NewLazyResourceStore(log)
	lazyResourceSnapshotManager := lazyresourcexds.NewLazyResourceSnapshotManager(lazyResourceStore, log)
	lazyResourceXDSManager := lazyresourcexds.NewLazyResourceStateManager(lazyResourceStore, lazyResourceSnapshotManager, log)

	// Build ExtensionDeps for extension hooks (used throughout startup)
	deps := controllerext.ExtensionDeps{
		DB:        db,
		ConfigStore: configStore,
		Log:       log,
		Config:    cfg,
		EventHub:  eventHubInstance,
		GatewayID: gatewayID,
	}

	// Initialize the extension's xDS managers immediately after storage is ready.
	// EncryptionProviderManager is not yet available at this point; LoadOnStartup
	// (called after encryption init) handles any decryption-dependent work.
	initCtx, initCancel := context.WithTimeout(context.Background(), 30*time.Second)
	extXDS, err := ext.InitXDS(initCtx, deps)
	initCancel()
	if err != nil {
		log.Error("Extension InitXDS failed", slog.String("extension", ext.Name()), slog.Any("error", err))
		os.Exit(1)
	}

	// Initialize encryption providers for secret management
	var encryptionProviderManager *encryption.ProviderManager
	var secretsService *secrets.SecretService

	// Load configurations from database on startup
	log.Info("Loading configurations from database")
	if err := storage.LoadFromDatabase(db, configStore); err != nil {
		log.Error("Failed to load configurations from database", slog.Any("error", err))
		os.Exit(1)
	}
	if err := storage.LoadLLMProviderTemplatesFromDatabase(db, configStore); err != nil {
		log.Error("Failed to load llm provider template configurations from database", slog.Any("error", err))
		os.Exit(1)
	}
	log.Info("Loaded configurations", slog.Int("count", len(configStore.GetAll())))

	// Load API keys from database into both in-memory stores
	log.Info("Loading API keys from database")
	if err := storage.LoadAPIKeysFromDatabase(db, configStore, apiKeyStore); err != nil {
		log.Error("Failed to load API keys from database", slog.Any("error", err))
		os.Exit(1)
	}
	log.Info("Loaded API keys", slog.Int("count", apiKeyXDSManager.GetAPIKeyCount()))

	log.Info("Loading encryption providers")
	if len(cfg.Controller.Encryption.Providers) > 0 {
		log.Info("Initializing encryption providers", slog.Int("provider_count", len(cfg.Controller.Encryption.Providers)))

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
	}
	log.Info("Loaded encryption providers")

	// Update deps with now-available encryption provider
	deps.EncryptionProviderManager = encryptionProviderManager

	// Extension startup hydration: load secrets, seed in-memory state, etc.
	loadCtx, loadCancel := context.WithTimeout(context.Background(), 60*time.Second)
	if err := ext.LoadOnStartup(loadCtx, deps, extXDS); err != nil {
		log.Error("Extension LoadOnStartup failed", slog.String("extension", ext.Name()), slog.Any("error", err))
		os.Exit(1)
	}
	loadCancel()

	// Load policy definitions from files before any startup hydration or policy derivation.
	policyLoader := utils.NewPolicyLoader(log)
	policyDir := cfg.Controller.Policies.DefinitionsPath
	log.Info("Loading policy definitions from directory", slog.String("directory", policyDir))
	policyDefinitions, err := policyLoader.LoadPoliciesFromDirectory(policyDir)
	if err != nil {
		log.Error("Failed to load policy definitions", slog.Any("error", err))
		os.Exit(1)
	}
	log.Info("Policy definitions loaded", slog.Int("count", len(policyDefinitions)))

	// Detect custom policies from build-manifest.yaml.
	localPolicies, err := policyLoader.GetCustomPolicyNames(cfg.Controller.Policies.BuildManifestPath)
	if err != nil {
		log.Warn("Could not read build-manifest.yaml, Custom policies will not be marked in the gateway manifest",
			slog.String("path", cfg.Controller.Policies.BuildManifestPath),
			slog.Any("error", err))
	}
	for key, def := range policyDefinitions {
		def.ManagedBy = "wso2"
		if localPolicies[def.Name+"|"+def.Version] {
			def.ManagedBy = "customer"
		}
		policyDefinitions[key] = def
	}

	// MCP proxies and LLM artifacts are stored in source form and need to be
	// rehydrated into their derived RestAPI representations before startup
	// snapshot and policy work.
	if err := hydrateStoredConfigsFromDatabaseOnStartup(
		configStore,
		db,
		&cfg.Router,
		policyDefinitions,
		log,
		cfg.Controller.Server.SkipInvalidDeploymentsOnStartup,
	); err != nil {
		log.Error("Failed to hydrate stored configurations required for startup", slog.Any("error", err))
		os.Exit(1)
	}

	// Initialize xDS snapshot manager with router config
	snapshotManager := xds.NewSnapshotManager(configStore, log, &cfg.Router, db, cfg)

	// Initialize SDS secret manager if custom certificates are configured
	var sdsSecretManager *xds.SDSSecretManager
	translator := snapshotManager.GetTranslator()
	if translator != nil && translator.GetCertStore() != nil {
		sdsSecretManager = xds.NewSDSSecretManager(
			translator.GetCertStore(),
			snapshotManager.GetCache(),
			"router-node",
			log,
		)
		if err := sdsSecretManager.UpdateSecrets(); err != nil {
			log.Warn("Failed to initialize SDS secrets", slog.Any("error", err))
		} else {
			log.Info("SDS secret manager initialized successfully")
			snapshotManager.SetSDSSecretManager(sdsSecretManager)
		}
	}

	// Generate initial xDS snapshot
	log.Info("Generating initial xDS snapshot")
	snapCtx, snapCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := snapshotManager.UpdateSnapshot(snapCtx, ""); err != nil {
		log.Warn("Failed to generate initial xDS snapshot", slog.Any("error", err))
	}
	snapCancel()

	// Create channels to detect when router and policy engine first connect
	routerConnected := make(chan struct{})
	policyEngineConnected := make(chan struct{})

	// Start xDS gRPC server with SDS support
	xdsServer := xds.NewServer(snapshotManager, sdsSecretManager, cfg.Controller.Server.XDSPort, log, routerConnected)
	go func() {
		if err := xdsServer.Start(); err != nil {
			log.Error("xDS server failed", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	// Generate initial API key snapshot if API keys were loaded from database
	if apiKeyXDSManager.GetAPIKeyCount() > 0 {
		log.Info("Generating initial API key snapshot for policy engine",
			slog.Int("api_key_count", apiKeyXDSManager.GetAPIKeyCount()))
		akCtx, akCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := apiKeySnapshotManager.UpdateSnapshot(akCtx); err != nil {
			log.Warn("Failed to generate initial API key snapshot", slog.Any("error", err))
		} else {
			log.Info("Initial API key snapshot generated successfully")
		}
		akCancel()
	}

	// Initialize policy xDS server
	log.Info("Initializing Policy xDS server", slog.Int("port", cfg.Controller.PolicyServer.Port))

	// Initialize policy snapshot manager and runtime config store
	policySnapshotManager := policyxds.NewSnapshotManager(log)
	runtimeStore := storage.NewRuntimeConfigStore()
	policySnapshotManager.SetRuntimeStore(runtimeStore)
	policySnapshotManager.SetConfigStore(configStore)

	// Initialize policy manager
	policyManager := policyxds.NewPolicyManager(policySnapshotManager, log)
	policyManager.SetRuntimeStore(runtimeStore)

	// Build transformer registry for StoredConfig → RuntimeDeployConfig conversion
	policyVersionResolver := utils.NewLoadedPolicyVersionResolver(policyDefinitions)
	restTransformer := transform.NewRestAPITransformer(&cfg.Router, cfg, policyDefinitions)
	llmTransformer := transform.NewLLMTransformer(configStore, db, &cfg.Router, cfg, policyDefinitions, policyVersionResolver)
	transformerRegistry := transform.NewRegistry(restTransformer, llmTransformer)
	policyManager.SetTransformers(transformerRegistry)

	// Load runtime configs from existing API configurations on startup.
	log.Info("Loading runtime configs from existing API configurations")
	loadedAPIs := configStore.GetAll()
	loadedCount, err := loadRuntimeConfigsFromExistingAPIConfigurations(
		loadedAPIs,
		runtimeStore,
		secretsService,
		transformerRegistry,
		log,
		cfg.Controller.Server.SkipInvalidDeploymentsOnStartup,
	)
	if err != nil {
		log.Error("Failed to load runtime configs from API configurations", slog.Any("error", err))
		os.Exit(1)
	}
	log.Info("Loaded runtime configs from API configurations",
		slog.Int("total_apis", len(loadedAPIs)),
		slog.Int("configs_loaded", loadedCount))

	// Generate initial policy snapshot
	log.Info("Generating initial policy xDS snapshot")
	polCtx, polCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := policySnapshotManager.UpdateSnapshot(polCtx); err != nil {
		log.Warn("Failed to generate initial policy xDS snapshot", slog.Any("error", err))
	}
	polCancel()

	// Build policyxds server options: TLS + first-connect notification + extension caches.
	serverOpts := []policyxds.ServerOption{
		policyxds.WithOnFirstConnect(policyEngineConnected),
	}
	if cfg.Controller.PolicyServer.TLS.Enabled {
		serverOpts = append(serverOpts, policyxds.WithTLS(
			cfg.Controller.PolicyServer.TLS.CertFile,
			cfg.Controller.PolicyServer.TLS.KeyFile,
		))
	}
	for _, nc := range extXDS.ExtraCaches {
		serverOpts = append(serverOpts, policyxds.WithExtraCache(nc.Name, nc.Cache))
	}

	policyXDSServer := policyxds.NewServer(policySnapshotManager, apiKeySnapshotManager, lazyResourceSnapshotManager, cfg.Controller.PolicyServer.Port, log, serverOpts...)
	go func() {
		if err := policyXDSServer.Start(); err != nil {
			log.Error("Policy xDS server failed", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	// Load llm provider templates from files
	templateLoader := utils.NewLLMTemplateLoader(log)
	templateDir := cfg.Controller.LLM.TemplateDefinitionsPath
	log.Info("Loading llm provider templates from directory", slog.String("directory", templateDir))
	templateDefinitions, err := templateLoader.LoadTemplatesFromDirectory(templateDir)
	if err != nil {
		log.Error("Failed to load llm provider templates", slog.Any("error", err))
		os.Exit(1)
	}
	log.Info("Default llm provider templates loaded", slog.Int("count", len(templateDefinitions)))

	// Create validator with policy validation support
	validator := config.NewAPIValidator()
	policyValidator := config.NewPolicyValidator(policyDefinitions)
	validator.SetPolicyValidator(policyValidator)

	apiSvc := utils.NewAPIDeploymentService(configStore, db, snapshotManager, validator, &cfg.Router, eventHubInstance, gatewayID, secretsService)
	mcpSvc := utils.NewMCPDeploymentService(configStore, db, snapshotManager, policyManager, policyValidator, eventHubInstance, gatewayID, secretsService)
	llmSvc := utils.NewLLMDeploymentService(configStore, db, snapshotManager, lazyResourceXDSManager, templateDefinitions,
		apiSvc, &cfg.Router, policyVersionResolver, policyValidator)

	// Wire event-gateway components (nil when NoOpExtension is used).
	wiring := extXDS.EventGatewayWiring

	// Initialize and start control plane client
	cpClient := controlplane.NewClient(
		cfg.Controller.ControlPlane,
		log, configStore,
		db, snapshotManager,
		validator,
		&cfg.Router,
		apiKeyXDSManager, apiKeyStore,
		&cfg.APIKey,
		policyManager,
		cfg, policyDefinitions,
		lazyResourceXDSManager,
		templateDefinitions,
		wiring.SubscriptionSnapshotUpdater,
		eventHubInstance,
		secretsService,
		wiring.WebhookSecretStore,
		wiring.WebhookSecretSnapshotManager,
	)
	if err := cpClient.Start(); err != nil {
		log.Error("Failed to start control plane client", slog.Any("error", err))
	}

	restAPIService := restapi.NewRestAPIService(
		configStore, db, snapshotManager, policyManager,
		apiSvc, apiKeyXDSManager,
		cpClient, &cfg.Router, cfg,
		&http.Client{Timeout: 10 * time.Second}, config.NewParser(), validator, log,
		eventHubInstance, secretsService,
	)
	igw := immutable.NewImmutableGW(cfg.ImmutableGateway, restAPIService, llmSvc, mcpSvc)

	// Initialize Gin router
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()

	// IMPORTANT: CorrelationIDMiddleware must be registered first.
	router.Use(middleware.CorrelationIDMiddleware(log))
	router.Use(middleware.ErrorHandlingMiddleware(log))
	router.Use(middleware.LoggingMiddleware(log))
	if cfg.Controller.Metrics.Enabled {
		router.Use(middleware.MetricsMiddleware())
	}
	authConfig := GenerateAuthConfig(cfg, ManagementAPIBasePath, ext.AdditionalResourceRoles())
	authMiddleWare, err := authenticators.AuthMiddleware(authConfig, log)
	if err != nil {
		log.Error("Failed to create auth middleware", slog.Any("error", err))
		os.Exit(1)
	}
	router.Use(authMiddleWare)
	router.Use(authenticators.AuthorizationMiddleware(authConfig, log))
	router.Use(gin.Recovery())

	// Initialize EventListener for multi-replica sync.
	evtListener := eventlistener.NewEventListener(
		eventHubInstance,
		configStore,
		db,
		snapshotManager,
		apiKeyXDSManager,
		lazyResourceXDSManager,
		policyManager,
		&cfg.Router,
		log,
		cfg,
		policyDefinitions,
		secretsService,
		eventlistener.WithExtraProcessors(ext.ExtraEventProcessors(deps, extXDS)...),
	)
	if err := evtListener.Start(); err != nil {
		log.Error("Failed to start event listener", slog.Any("error", err))
		os.Exit(1)
	}
	log.Info("EventListener started for multi-replica sync")

	// Initialize API server.
	apiServer := handlers.NewAPIServer(
		configStore,
		db,
		snapshotManager,
		policyManager,
		lazyResourceXDSManager,
		log,
		cpClient,
		policyDefinitions,
		templateDefinitions,
		validator,
		apiKeyXDSManager,
		cfg,
		eventHubInstance,
		wiring.SubscriptionSnapshotUpdater,
		secretsService,
		restAPIService,
		wiring.WebhookSecretService,
	)

	// Load immutable gateway artifacts from the filesystem.
	if err := igw.LoadArtifacts(log); err != nil {
		log.Error("Failed to load immutable gateway artifacts", slog.Any("error", err))
		os.Exit(1)
	}

	// Ensure initial lazy resource snapshot includes default templates.
	if lazyResourceStore.Count() > 0 {
		log.Info("Generating initial lazy resource snapshot for policy engine (including templates)",
			slog.Int("lazy_resource_count", lazyResourceStore.Count()))
		lzCtx, lzCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := lazyResourceSnapshotManager.UpdateSnapshot(lzCtx); err != nil {
			log.Warn("Failed to generate initial lazy resource snapshot", slog.Any("error", err))
		} else {
			log.Info("Initial lazy resource snapshot generated successfully")
		}
		lzCancel()
	}

	// Register immutable gateway middleware.
	router.Use(igw.Middleware())

	// Register base management API routes (versioned + legacy deprecated paths).
	api.RegisterHandlersWithOptions(router, apiServer, api.GinServerOptions{
		BaseURL: ManagementAPIBasePath,
	})
	api.RegisterHandlersWithOptions(router, apiServer, api.GinServerOptions{
		Middlewares: []api.MiddlewareFunc{
			DeprecatedManagementPathMiddleware(ManagementAPIBasePath),
		},
	})

	// Mount extension-owned routes (e.g. event-gateway REST endpoints).
	if err := ext.RegisterRoutes(router, deps, extXDS); err != nil {
		log.Error("Extension RegisterRoutes failed", slog.String("extension", ext.Name()), slog.Any("error", err))
		os.Exit(1)
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

	// Start metrics server if enabled
	var metricsServer *metrics.Server
	var metricsCtxCancel context.CancelFunc
	if cfg.Controller.Metrics.Enabled {
		log.Info("Starting metrics server", slog.Int("port", cfg.Controller.Metrics.Port))
		metrics.Info.WithLabelValues(version.Version, cfg.Controller.Storage.Type, version.BuildDate).Set(1)
		metricsServer = metrics.NewServer(&cfg.Controller.Metrics, log)
		if err := metricsServer.Start(); err != nil {
			log.Error("Metrics server failed", slog.Any("error", err))
			os.Exit(1)
		}
		var metricsCtx context.Context
		metricsCtx, metricsCtxCancel = context.WithCancel(context.Background())
		metrics.StartMemoryMetricsUpdater(metricsCtx, 15*time.Second)
	}

	// Start REST API server
	log.Info("Starting REST API server", slog.Int("port", cfg.Controller.Server.APIPort))
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Controller.Server.APIPort),
		Handler:           router,
		ReadHeaderTimeout: 30 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Failed to start REST API server", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	log.Info("Gateway Controller started successfully")

	// Print banner when both router and policy engine have sent their first ACK.
	go func() {
		<-routerConnected
		<-policyEngineConnected
		time.Sleep(1 * time.Second)
		fmt.Print("\n\n" +
			"========================================================================\n" +
			"\n" +
			"\n" +
			"                   API Platform Gateway Started\n" +
			"\n" +
			"\n" +
			"========================================================================\n" +
			"\n\n")
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Gateway-Controller")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Controller.Server.ShutdownTimeout)
	defer shutdownCancel()

	// Stop extension first so it can flush its state
	ext.Shutdown(shutdownCtx)

	// Stop event listener and EventHub first
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

	if err := srv.Shutdown(shutdownCtx); err != nil {
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
		if err := metricsServer.Stop(shutdownCtx); err != nil {
			log.Error("Failed to stop metrics server", slog.Any("error", err))
		}
	}

	if controllerAdminServer != nil {
		if err := controllerAdminServer.Stop(shutdownCtx); err != nil {
			log.Error("Failed to stop controller admin server", slog.Any("error", err))
		}
	}

	log.Info("Gateway-Controller stopped")
}
