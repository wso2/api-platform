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

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/common/webhooksecret"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/adminserver"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/apikeyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption/aesgcm"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/eventlistener"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/lazyresourcexds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/secrets"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/subscriptionxds"

	"github.com/wso2/api-platform/common/authenticators"
	commonmodels "github.com/wso2/api-platform/common/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/handlers"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/controlplane"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/immutable"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/logger"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/restapi"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/transform"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/version"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	gohttpkit "github.com/wso2/go-httpkit/middleware"
)

// API base paths for the gateway-controller HTTP surfaces.
// These must stay in sync with the `servers.url` values in the OpenAPI specs
// (api/management-openapi.yaml and api/admin-openapi.yaml).
const (
	managementAPIBasePath = "/api/management/v1"
	adminAPIBasePath      = "/api/admin/v1"
)

func toBackendConfig(cfg *config.Config) storage.BackendConfig {
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

// stringSliceFlag collects a repeatable string flag into a slice, preserving the
// order in which the flags were supplied on the command line.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string { return strings.Join(*s, ", ") }

func (s *stringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	// Parse command-line flags. -config is repeatable: files are merged in the
	// order given with last-wins precedence (a key set in a later file overrides
	// the same key from an earlier file).
	var configPaths stringSliceFlag
	flag.Var(&configPaths, "config",
		"Path to a configuration file (required; repeatable, merged in order with last-wins precedence)")
	flag.Parse()

	// Validate that at least one config file is provided
	if len(configPaths) == 0 {
		fmt.Fprintf(os.Stderr, "Error: -config flag is required\n")
		fmt.Fprintf(os.Stderr, "Usage: %s -config <path-to-config.toml> [-config <overlay.toml> ...]\n", os.Args[0])
		os.Exit(1)
	}

	// Load configuration
	cfg, err := config.LoadConfig(configPaths...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration from %s: %v\n", strings.Join(configPaths, ", "), err)
		os.Exit(1)
	}

	// Initialize metrics based on configuration
	// This must be done before any metrics are used to ensure no-op behavior when disabled
	metrics.SetEnabled(cfg.Controller.Metrics.Enabled)
	metrics.Init() // Initialize metrics immediately so they're available throughout the codebase

	// Initialize logger with config
	log := logger.NewLogger(logger.Config{
		Level:  cfg.Controller.Logging.Level,
		Format: cfg.Controller.Logging.Format,
	})

	log.Info("Starting Gateway-Controller",
		slog.String("version", version.Version),
		slog.String("git_commit", version.GitCommit),
		slog.String("build_date", version.BuildDate),
		slog.Any("config_files", []string(configPaths)),
		slog.String("storage_type", cfg.Controller.Storage.Type),
		slog.Bool("access_logs_enabled", cfg.Router.AccessLogs.Enabled),
		slog.String("control_plane_host", cfg.Controller.ControlPlane.Host),
		slog.Bool("control_plane_token_configured", cfg.Controller.ControlPlane.Token != ""),
		slog.Bool("skip_invalid_deployments_on_startup", cfg.Controller.Server.SkipInvalidDeploymentsOnStartup),
	)

	if !cfg.Controller.Auth.Basic.Enabled && !cfg.Controller.Auth.IDP.Enabled {
		log.Warn("No authentication configured: both basic auth and IDP are disabled. Gateway Controller API will allow all requests without authentication")
	}

	// In immutable mode, delete any stale SQLite files before opening the DB to
	// guarantee a fresh, reproducible state on every boot.
	if cfg.ImmutableGateway.Enabled {
		log.Info("Immutable gateway mode enabled — removing existing SQLite files for fresh start",
			slog.String("path", cfg.Controller.Storage.EffectiveSQLitePath()))
		if err := immutable.ResetSQLiteFiles(cfg.Controller.Storage.EffectiveSQLitePath(), log); err != nil {
			log.Error("Failed to reset SQLite files for immutable mode", slog.Any("error", err))
			os.Exit(1)
		}
	}

	// Initialize storage based on type
	var db storage.Storage
	db, err = storage.NewStorage(toBackendConfig(cfg), log)
	if err != nil {
		if strings.EqualFold(cfg.Controller.Storage.Type, "sqlite") && errors.Is(err, storage.ErrDatabaseLocked) {
			log.Error("Database is locked by another process",
				slog.String("database_path", cfg.Controller.Storage.EffectiveSQLitePath()),
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
	// Create separate storage connection for EventHub (avoids SQLite lock contention)
	ehBackendCfg := toBackendConfig(cfg)
	// Apply the EventHub-specific pool sizing to whichever SQL backend is in use
	// (a separate, smaller pool for the poller). Keep this in sync for every
	// connection-pooled backend so the override isn't silently dropped.
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

		// Initialize encryption providers
		var providers []encryption.EncryptionProvider
		for _, providerConfig := range cfg.Controller.Encryption.Providers {
			switch providerConfig.Type {
			case "aesgcm":
				// Convert config keys to AES-GCM key configs
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

		// Create provider manager
		encryptionProviderManager, err = encryption.NewProviderManager(providers, log)
		if err != nil {
			log.Error("Failed to initialize provider manager", slog.Any("error", err))
			os.Exit(1)
		}
		// Create secrets service
		secretsService = secrets.NewSecretsService(db, encryptionProviderManager, log)
	}
	log.Info("Loaded encryption providers")

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
			def.ManagedBy = "organization"
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
		// Use the same cache and node ID as the main xDS to ensure Envoy can fetch secrets
		sdsSecretManager = xds.NewSDSSecretManager(
			translator.GetCertStore(),
			snapshotManager.GetCache(),
			"router-node", // Same node ID as main xDS
			log,
		)
		// Update SDS secrets with current certificates
		if err := sdsSecretManager.UpdateSecrets(); err != nil {
			log.Warn("Failed to initialize SDS secrets", slog.Any("error", err))
		} else {
			log.Info("SDS secret manager initialized successfully")
			// Set the SDS secret manager in snapshot manager so secrets are included in snapshots
			snapshotManager.SetSDSSecretManager(sdsSecretManager)
		}
	}

	// Generate initial xDS snapshot
	log.Info("Generating initial xDS snapshot")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := snapshotManager.UpdateSnapshot(ctx, ""); err != nil {
		log.Warn("Failed to generate initial xDS snapshot", slog.Any("error", err))
	}
	cancel()

	// Create channels to detect when router and policy engine first connect
	routerConnected := make(chan struct{})
	policyEngineConnected := make(chan struct{})

	// Start xDS gRPC server with SDS support
	var xdsServerOpts []xds.ServerOption
	if cfg.Controller.Server.XDSTLS.Enabled {
		xdsTLSConfig, err := config.BuildXDSServerTLSConfig(cfg.Controller.Server.XDSTLS)
		if err != nil {
			log.Error("invalid server.xds_tls config, refusing to start main xDS server in plaintext", slog.Any("error", err))
			os.Exit(1)
		}
		xdsServerOpts = append(xdsServerOpts, xds.WithMTLS(xdsTLSConfig, cfg.Controller.Server.XDSTLS.AllowedClientIdentities))
	}
	xdsServer := xds.NewServer(snapshotManager, sdsSecretManager, cfg.Controller.Server.XDSPort, log, routerConnected, xdsServerOpts...)
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
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := apiKeySnapshotManager.UpdateSnapshot(ctx); err != nil {
			log.Warn("Failed to generate initial API key snapshot", slog.Any("error", err))
		} else {
			log.Info("Initial API key snapshot generated successfully")
		}
		cancel()
	}

	// Initialize policy xDS server
	log.Info("Initializing Policy xDS server", slog.Int("port", cfg.Controller.PolicyServer.Port))

	// Initialize policy snapshot manager and runtime config store
	policySnapshotManager := policyxds.NewSnapshotManager(log)
	runtimeStore := storage.NewRuntimeConfigStore()
	policySnapshotManager.SetRuntimeStore(runtimeStore)
	policySnapshotManager.SetConfigStore(configStore)

	// Initialize subscription snapshot manager (driven by DB storage)
	subscriptionSnapshotManager := subscriptionxds.NewSnapshotManager(db, log)

	// Initialize policy manager
	policyManager := policyxds.NewPolicyManager(policySnapshotManager, log)
	policyManager.SetRuntimeStore(runtimeStore)

	// Build transformer registry for StoredConfig → RuntimeDeployConfig conversion
	policyVersionResolver := utils.NewLoadedPolicyVersionResolver(policyDefinitions)
	restTransformer := transform.NewRestAPITransformer(&cfg.Router, cfg, policyDefinitions)
	llmTransformer := transform.NewLLMTransformer(configStore, db, &cfg.Router, cfg, policyDefinitions, policyVersionResolver)
	transformerRegistry := transform.NewRegistry(restTransformer, llmTransformer)
	policyManager.SetTransformers(transformerRegistry)

	// Wire the same transformer into the Envoy xDS translator so Envoy routes are built from the
	// RuntimeDeployConfig (RDC) path — identical to how the policy engine's RouteConfig/PolicyChain
	// resources are keyed. Without this the Envoy translator falls back to the legacy per-operation
	// path, which (a) does not render header matchers and (b) names routes "method|path|vhost"
	// (3 segments), while the policy resources are keyed "method|path|vhost|<header-hash>". The
	// policy engine resolves the chain by the Envoy route name, so the mismatch makes every
	// header-matched route fail with 500 ("policy chain not found"). WebSubApi is intentionally
	// excluded so it keeps using the async-specific legacy translation path.
	translator.SetTransformers(map[string]models.ConfigTransformer{
		"RestApi":     transformerRegistry,
		"Mcp":         transformerRegistry,
		"LlmProvider": transformerRegistry,
		"LlmProxy":    transformerRegistry,
	})

	// Load runtime configs from existing API configurations on startup.
	// We write directly to runtimeStore to avoid triggering N separate snapshot updates;
	// the single UpdateSnapshot call below covers all of them.
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
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	if err := policySnapshotManager.UpdateSnapshot(ctx); err != nil {
		log.Warn("Failed to generate initial policy xDS snapshot", slog.Any("error", err))
	}
	cancel()

	// Generate initial subscription snapshot
	log.Info("Generating initial subscription xDS snapshot")
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	if err := subscriptionSnapshotManager.UpdateSnapshot(ctx); err != nil {
		log.Warn("Failed to generate initial subscription xDS snapshot", slog.Any("error", err))
	}
	cancel()

	// Start policy xDS server in a separate goroutine
	serverOpts := []policyxds.ServerOption{
		policyxds.WithOnFirstConnect(policyEngineConnected),
	}
	if cfg.Controller.PolicyServer.TLS.Enabled {
		policyXDSTLSConfig, err := config.BuildXDSServerTLSConfig(cfg.Controller.PolicyServer.TLS)
		if err != nil {
			log.Error("invalid policy_server.tls config, refusing to start policy xDS server in plaintext", slog.Any("error", err))
			os.Exit(1)
		}
		serverOpts = append(serverOpts, policyxds.WithMTLS(policyXDSTLSConfig, cfg.Controller.PolicyServer.TLS.AllowedClientIdentities))
	}
	policyXDSServer := policyxds.NewServer(policySnapshotManager, apiKeySnapshotManager, lazyResourceSnapshotManager, subscriptionSnapshotManager, nil, cfg.Controller.PolicyServer.Port, log, serverOpts...)
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

	// Initialize and start control plane client with dependencies for API creation and API key management
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
		subscriptionSnapshotManager,
		eventHubInstance,
		secretsService,
		webhooksecret.GetStoreInstance(),
		nil,
	)
	if err := cpClient.Start(); err != nil {
		log.Error("Failed to start control plane client", slog.Any("error", err))
		// Don't fail startup - gateway can run in degraded mode without control plane
	}

	// Wire the DP->CP push into the LLM/MCP deployment services so artifacts created through
	// the service layer (notably the immutable-gateway loader, which bypasses the REST
	// handlers) are pushed to the control plane, matching the REST API service behavior. The
	// control plane client's own internal services are intentionally left unwired so
	// CP-originated applies are never pushed back.
	llmSvc.SetControlPlanePusher(cpClient, cfg.Controller.ControlPlane.DeploymentSyncEnabled)
	mcpSvc.SetControlPlanePusher(cpClient, cfg.Controller.ControlPlane.DeploymentSyncEnabled)

	restAPIService := restapi.NewRestAPIService(
		configStore, db, snapshotManager, policyManager,
		apiSvc, apiKeyXDSManager,
		cpClient, &cfg.Router, cfg,
		&http.Client{Timeout: 10 * time.Second}, config.NewParser(), validator, log,
		eventHubInstance, secretsService,
	)
	igw := immutable.NewImmutableGW(cfg.ImmutableGateway, restAPIService, llmSvc, mcpSvc)

	authConfig := generateAuthConfig(cfg)
	if cfg.Controller.Auth.IDP.Enabled && len(cfg.Controller.Auth.IDP.Audience) == 0 {
		log.Warn("IDP auth is enabled but no auth.idp.audience is configured - token audience ('aud') will NOT be " +
			"validated; set auth.idp.audience to the expected audience(s) to restrict this")
	}
	authMiddleWare, err := authenticators.AuthMiddleware(authConfig, log)
	if err != nil {
		log.Error("Failed to create auth middleware", slog.Any("error", err))
		os.Exit(1)
	}
	// Per-route middlewares: auth runs first, then authz (needs r.Pattern set by mux).
	// The generated wrapper applies middlewares via `handler = mw(handler)`, so the
	// last entry in the slice is outermost and executes first. authMiddleWare must be
	// last so it runs before authz can inspect the auth context it populates.
	perRouteMiddlewares := []api.MiddlewareFunc{
		authenticators.AuthorizationMiddleware(authConfig, log),
		authMiddleWare,
	}

	// Initialize EventListener for multi-replica sync (consumes EventHub events)
	var evtListener *eventlistener.EventListener
	evtListener = eventlistener.NewEventListener(
		eventHubInstance,
		configStore,
		db,
		snapshotManager,
		subscriptionSnapshotManager,
		apiKeyXDSManager,
		lazyResourceXDSManager,
		policyManager,
		&cfg.Router,
		log,
		cfg,
		policyDefinitions,
		secretsService,
	)
	if err := evtListener.Start(); err != nil {
		log.Error("Failed to start event listener", slog.Any("error", err))
		os.Exit(1)
	}
	log.Info("EventListener started for multi-replica sync")

	// Initialize API server with the configured validator and API key manager
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
		subscriptionSnapshotManager,
		secretsService,
		restAPIService,
	)

	// Load immutable gateway artifacts from the filesystem (no-op when immutable mode is disabled).
	if err := igw.LoadArtifacts(log); err != nil {
		log.Error("Failed to load immutable gateway artifacts", slog.Any("error", err))
		os.Exit(1)
	}

	// Ensure initial lazy resource snapshot includes default templates loaded from files.
	// At this point, the API server initialization has already persisted/published OOB templates.
	if lazyResourceStore.Count() > 0 {
		log.Info("Generating initial lazy resource snapshot for policy engine (including templates)",
			slog.Int("lazy_resource_count", lazyResourceStore.Count()))
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := lazyResourceSnapshotManager.UpdateSnapshot(ctx); err != nil {
			log.Warn("Failed to generate initial lazy resource snapshot", slog.Any("error", err))
		} else {
			log.Info("Initial lazy resource snapshot generated successfully")
		}
		cancel()
	}

	// Register API routes into a shared mux. Both registrations target the
	// same mux — HandlerWithOptions registers routes on the BaseRouter in-place.
	mux := http.NewServeMux()

	// Versioned routes under managementAPIBasePath.
	api.HandlerWithOptions(apiServer, api.StdHTTPServerOptions{
		BaseURL:     managementAPIBasePath,
		BaseRouter:  mux,
		Middlewares: append(perRouteMiddlewares, igw.Middleware()),
	})

	// Legacy unprefixed routes — deprecated; responses include RFC 8594 headers.
	// Remove once all known clients have switched to the versioned base path.
	api.HandlerWithOptions(apiServer, api.StdHTTPServerOptions{
		BaseURL:    "",
		BaseRouter: mux,
		Middlewares: append(perRouteMiddlewares,
			igw.Middleware(),
			deprecatedManagementPathMiddleware(managementAPIBasePath),
		),
	})

	// Outer middleware wraps the entire mux. CorrelationID must be outermost
	// so every downstream middleware gets a correlation-aware logger.
	outerMiddlewares := []func(http.Handler) http.Handler{
		middleware.CorrelationIDMiddleware(log),
		middleware.ErrorHandlingMiddleware(log),
		middleware.LoggingMiddleware(log),
	}
	if cfg.Controller.Metrics.Enabled {
		outerMiddlewares = append(outerMiddlewares, middleware.MetricsMiddleware())
	}
	handler := gohttpkit.Chain(outerMiddlewares...)(mux)

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
		// Gate the admin/debug server behind the same authentication middleware as
		// the management API, plus a deny-by-default admin-role authorization check.
		// The health probe stays public; every other admin
		// endpoint (config_dump, xds_sync_status, pprof) requires an authenticated
		// caller from controller.auth.basic who also holds the "admin" role.
		// Authentication runs first (outermost) to populate the auth context, then
		// authorization consumes it — the same ordering the management API uses.
		adminAuthz := authenticators.AuthorizationMiddleware(
			commonmodels.AuthConfig{ResourceRoles: adminResourceRoles()}, log)
		adminProtect := func(next http.Handler) http.Handler {
			return authMiddleWare(adminAuthz(next))
		}
		controllerAdminServer = adminserver.NewServer(&cfg.Controller.AdminServer, apiServer, adminProtect, log)
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

		// Set build info metric
		metrics.Info.WithLabelValues(version.Version, cfg.Controller.Storage.Type, version.BuildDate).Set(1)

		metricsServer = metrics.NewServer(&cfg.Controller.Metrics, log)
		if err := metricsServer.Start(); err != nil {
			log.Error("Metrics server failed", slog.Any("error", err))
			os.Exit(1)
		}

		// Start memory metrics updater with cancellable context
		var metricsCtx context.Context
		metricsCtx, metricsCtxCancel = context.WithCancel(context.Background())
		metrics.StartMemoryMetricsUpdater(metricsCtx, 15*time.Second)
	}

	// Start REST API server
	log.Info("Starting REST API server", slog.Int("port", cfg.Controller.Server.APIPort))

	// Setup graceful shutdown
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Controller.Server.APIPort),
		Handler:           handler,
		ReadHeaderTimeout: 30 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Failed to start REST API server", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	// Optional TLS listener for the REST API, additive to the plaintext one
	// above — never instead of it. A misconfigured/missing certificate here
	// disables just this listener rather than exiting the process, since the
	// plaintext listener remains the required one.
	var tlsSrv *http.Server
	if cfg.Controller.Server.TLS.Enabled {
		tlsConfig, err := buildRESTAPITLSConfig(&cfg.Controller.Server.TLS)
		if err != nil {
			log.Error("invalid server.tls config, REST API TLS listener disabled", slog.Any("error", err))
		} else {
			tlsSrv = &http.Server{
				Addr:              fmt.Sprintf(":%d", cfg.Controller.Server.TLS.Port),
				Handler:           handler,
				ReadHeaderTimeout: 30 * time.Second,
				TLSConfig:         tlsConfig,
			}
			go func() {
				log.Info("Starting REST API TLS server", slog.Int("port", cfg.Controller.Server.TLS.Port))
				if err := tlsSrv.ListenAndServeTLS(cfg.Controller.Server.TLS.CertPath, cfg.Controller.Server.TLS.KeyPath); err != nil && err != http.ErrServerClosed {
					log.Error("REST API TLS server error", slog.Any("error", err))
				}
			}()
		}
	}

	log.Info("Gateway Controller started successfully")

	// Print banner when both router and policy engine have sent their first ACK,
	// confirming they are fully initialized. One second delay lets Docker's log
	// buffers drain so the banner is not interleaved with Envoy's startup output.
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

	// Graceful shutdown with timeout
	ctx, cancel = context.WithTimeout(context.Background(), cfg.Controller.Server.ShutdownTimeout)
	defer cancel()

	// Stop event listener and EventHub first
	if evtListener != nil {
		evtListener.Stop()
	}
	if err := eventHubInstance.Close(); err != nil {
		log.Warn("Failed to close EventHub cleanly", slog.Any("error", err))
	}
	if eventHubStorage != nil {
		if err := eventHubStorage.Close(); err != nil {
			log.Warn("Failed to close EventHub storage cleanly", slog.Any("error", err))
		}
	}

	// Stop control plane client
	cpClient.Stop()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", slog.Any("error", err))
	}

	if tlsSrv != nil {
		if err := tlsSrv.Shutdown(ctx); err != nil {
			log.Error("REST API TLS server forced to shutdown", slog.Any("error", err))
		}
	}

	xdsServer.Stop()

	// Stop policy xDS server if it was started
	if policyXDSServer != nil {
		policyXDSServer.Stop()
	}

	// Stop metrics server if it was started
	if metricsServer != nil {
		// Cancel memory metrics updater context
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

	log.Info("Gateway-Controller stopped")
}

func generateAuthConfig(config *config.Config) commonmodels.AuthConfig {
	// prefixed builds a resource key of the form "<METHOD> <managementAPIBasePath><path>"
	// matching the actual routes registered via RegisterHandlersWithOptions(BaseURL=managementAPIBasePath).
	prefixed := func(methodAndPath string) string {
		idx := strings.Index(methodAndPath, " ")
		if idx < 0 {
			return methodAndPath
		}
		return methodAndPath[:idx+1] + managementAPIBasePath + methodAndPath[idx+1:]
	}

	relativeRoles := map[string][]string{
		"POST /rest-apis":        {"admin", "developer"},
		"GET /rest-apis":         {"admin", "developer"},
		"GET /rest-apis/{id}":    {"admin", "developer"},
		"PUT /rest-apis/{id}":    {"admin", "developer"},
		"DELETE /rest-apis/{id}": {"admin", "developer"},

		"GET /certificates":         {"admin", "developer"},
		"POST /certificates":        {"admin"},
		"DELETE /certificates/{id}": {"admin"},
		"POST /certificates/reload": {"admin"},

		"GET /policies": {"admin", "developer"},

		"POST /mcp-proxies":        {"admin", "developer"},
		"GET /mcp-proxies":         {"admin", "developer"},
		"GET /mcp-proxies/{id}":    {"admin", "developer"},
		"PUT /mcp-proxies/{id}":    {"admin", "developer"},
		"DELETE /mcp-proxies/{id}": {"admin", "developer"},

		"POST /llm-provider-templates":        {"admin"},
		"GET /llm-provider-templates":         {"admin"},
		"GET /llm-provider-templates/{id}":    {"admin"},
		"PUT /llm-provider-templates/{id}":    {"admin"},
		"DELETE /llm-provider-templates/{id}": {"admin"},

		"POST /llm-providers":        {"admin"},
		"GET /llm-providers":         {"admin", "developer"},
		"GET /llm-providers/{id}":    {"admin", "developer"},
		"PUT /llm-providers/{id}":    {"admin"},
		"DELETE /llm-providers/{id}": {"admin"},

		"POST /llm-proxies":        {"admin", "developer"},
		"GET /llm-proxies":         {"admin", "developer"},
		"GET /llm-proxies/{id}":    {"admin", "developer"},
		"PUT /llm-proxies/{id}":    {"admin", "developer"},
		"DELETE /llm-proxies/{id}": {"admin", "developer"},

		"POST /rest-apis/{id}/api-keys":                         {"admin", "consumer"},
		"GET /rest-apis/{id}/api-keys":                          {"admin", "consumer"},
		"PUT /rest-apis/{id}/api-keys/{apiKeyName}":             {"admin", "consumer"},
		"POST /rest-apis/{id}/api-keys/{apiKeyName}/regenerate": {"admin", "consumer"},
		"DELETE /rest-apis/{id}/api-keys/{apiKeyName}":          {"admin", "consumer"},

		"POST /llm-providers/{id}/api-keys":                         {"admin", "consumer"},
		"GET /llm-providers/{id}/api-keys":                          {"admin", "consumer"},
		"PUT /llm-providers/{id}/api-keys/{apiKeyName}":             {"admin", "consumer"},
		"POST /llm-providers/{id}/api-keys/{apiKeyName}/regenerate": {"admin", "consumer"},
		"DELETE /llm-providers/{id}/api-keys/{apiKeyName}":          {"admin", "consumer"},

		"POST /llm-proxies/{id}/api-keys":                         {"admin", "consumer"},
		"GET /llm-proxies/{id}/api-keys":                          {"admin", "consumer"},
		"PUT /llm-proxies/{id}/api-keys/{apiKeyName}":             {"admin", "consumer"},
		"POST /llm-proxies/{id}/api-keys/{apiKeyName}/regenerate": {"admin", "consumer"},
		"DELETE /llm-proxies/{id}/api-keys/{apiKeyName}":          {"admin", "consumer"},

		// Root-level subscription endpoints
		"POST /subscriptions":                    {"admin"},
		"GET /subscriptions":                     {"admin"},
		"GET /subscriptions/{subscriptionId}":    {"admin"},
		"PUT /subscriptions/{subscriptionId}":    {"admin"},
		"DELETE /subscriptions/{subscriptionId}": {"admin"},

		// Subscription plan endpoints
		"POST /subscription-plans":            {"admin"},
		"GET /subscription-plans":             {"admin"},
		"GET /subscription-plans/{planId}":    {"admin"},
		"PUT /subscription-plans/{planId}":    {"admin"},
		"DELETE /subscription-plans/{planId}": {"admin"},

		"POST /secrets":        {"admin"},
		"GET /secrets":         {"admin"},
		"GET /secrets/{id}":    {"admin"},
		"PUT /secrets/{id}":    {"admin"},
		"DELETE /secrets/{id}": {"admin"},
	}

	// Populate both the versioned and legacy (unprefixed) keys so the auth
	// middleware matches either route form. The legacy form is deprecated and
	// will be removed in a future release.
	DefaultResourceRoles := make(map[string][]string, len(relativeRoles)*2)
	for methodAndPath, roles := range relativeRoles {
		DefaultResourceRoles[prefixed(methodAndPath)] = roles
		DefaultResourceRoles[methodAndPath] = roles
	}
	basicAuth := commonmodels.BasicAuth{Enabled: false}
	idpAuth := commonmodels.IDPConfig{Enabled: false}
	if config.Controller.Auth.Basic.Enabled {
		users := make([]commonmodels.User, len(config.Controller.Auth.Basic.Users))
		for i, authUser := range config.Controller.Auth.Basic.Users {
			users[i] = commonmodels.User{
				Username:       authUser.Username,
				Password:       authUser.Password,
				PasswordHashed: authUser.PasswordHashed,
				Roles:          authUser.Roles,
			}
		}
		basicAuth = commonmodels.BasicAuth{Enabled: true, Users: users}
	}
	if config.Controller.Auth.IDP.Enabled {
		idpAuth = commonmodels.IDPConfig{Enabled: true, IssuerURL: config.Controller.Auth.IDP.Issuer,
			JWKSUrl:           config.Controller.Auth.IDP.JWKSURL,
			ScopeClaim:        config.Controller.Auth.IDP.RolesClaim,
			PermissionMapping: &config.Controller.Auth.IDP.RoleMapping,
		}
		if len(config.Controller.Auth.IDP.Audience) > 0 {
			idpAuth.Audience = &config.Controller.Auth.IDP.Audience
		}
	}
	authConfig := commonmodels.AuthConfig{BasicAuth: &basicAuth,
		JWTConfig:     &idpAuth,
		ResourceRoles: DefaultResourceRoles,
	}
	return authConfig
}

// adminResourceRoles maps every non-public admin-server route to the roles allowed
// to reach it, following the same shape as generateAuthConfig: relative
// "METHOD /path" keys expanded into both the versioned (base-path-prefixed) and
// legacy (unprefixed) forms that AuthorizationMiddleware matches against r.Pattern.
// AuthorizationMiddleware denies by default, so any admin route absent from this map
// is rejected with 403 — a route added later without an entry fails closed rather
// than becoming world-readable. The health probe is intentionally omitted: it is
// exempt from auth entirely and never reaches authorization.
func adminResourceRoles() map[string][]string {
	const adminRole = "admin"

	// prefixed builds a resource key of the form "<METHOD> <adminAPIBasePath><path>"
	// matching the versioned routes registered via HandlerWithOptions(BaseURL=adminAPIBasePath),
	// mirroring the prefixed helper in generateAuthConfig.
	prefixed := func(methodAndPath string) string {
		idx := strings.Index(methodAndPath, " ")
		if idx < 0 {
			return methodAndPath
		}
		return methodAndPath[:idx+1] + adminAPIBasePath + methodAndPath[idx+1:]
	}

	relativeRoles := map[string][]string{
		"GET /config_dump":     {adminRole},
		"GET /xds_sync_status": {adminRole},
	}

	// pprof endpoints are registered directly on the mux (not via a BaseURL), so
	// their r.Pattern carries no method and no base-path prefix — map them as-is.
	// Only registered when admin_server.pprof is enabled, but mapping them
	// unconditionally is harmless and keeps them admin-gated when it is.
	pprofRoles := map[string][]string{
		"/debug/pprof/":        {adminRole},
		"/debug/pprof/cmdline": {adminRole},
		"/debug/pprof/profile": {adminRole},
		"/debug/pprof/symbol":  {adminRole},
		"/debug/pprof/trace":   {adminRole},
	}

	// Populate both the versioned and legacy (unprefixed) keys for each relative
	// route so the authz middleware matches either form, exactly as generateAuthConfig does.
	resourceRoles := make(map[string][]string, len(relativeRoles)*2+len(pprofRoles))
	for methodAndPath, roles := range relativeRoles {
		resourceRoles[prefixed(methodAndPath)] = roles
		resourceRoles[methodAndPath] = roles
	}
	for pattern, roles := range pprofRoles {
		resourceRoles[pattern] = roles
	}

	return resourceRoles
}

// deprecatedManagementPathMiddleware marks responses served on the legacy
// unprefixed management API paths as deprecated, following RFC 8594. Adds:
//   - `Deprecation: true`
//   - `Link: <newBasePath+path>; rel="successor-version"`
//   - `Warning: 299 - "Deprecated API: use <newBasePath> prefix"`
//
// Attached only to the second (legacy) registration; versioned-path requests
// bypass it.
func deprecatedManagementPathMiddleware(newBasePath string) api.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			successor := newBasePath + r.URL.Path
			w.Header().Set("Deprecation", "true")
			w.Header().Set("Link", fmt.Sprintf("<%s>; rel=\"successor-version\"", successor))
			w.Header().Set("Warning",
				fmt.Sprintf("299 - \"Deprecated API: migrate to %s prefix\"", newBasePath))
			next.ServeHTTP(w, r)
		})
	}
}
