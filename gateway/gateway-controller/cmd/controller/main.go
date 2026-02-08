package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/apikeyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/lazyresourcexds"

	"github.com/gin-gonic/gin"
	"github.com/wso2/api-platform/common/authenticators"
	commonmodels "github.com/wso2/api-platform/common/models"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/handlers"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/controlplane"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/logger"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	policybuilder "github.com/wso2/api-platform/gateway/gateway-controller/pkg/policy"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

// Version information (set via ldflags during build)
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "", "Path to configuration file (required)")
	flag.Parse()

	// Validate that config file is provided
	if *configPath == "" {
		fmt.Fprintf(os.Stderr, "Error: -config flag is required\n")
		fmt.Fprintf(os.Stderr, "Usage: %s -config <path-to-config.toml>\n", os.Args[0])
		os.Exit(1)
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration from %s: %v\n", *configPath, err)
		os.Exit(1)
	}

	// Initialize metrics based on configuration
	// This must be done before any metrics are used to ensure no-op behavior when disabled
	metrics.SetEnabled(cfg.GatewayController.Metrics.Enabled)
	metrics.Init() // Initialize metrics immediately so they're available throughout the codebase

	// Initialize logger with config
	log := logger.NewLogger(logger.Config{
		Level:  cfg.GatewayController.Logging.Level,
		Format: cfg.GatewayController.Logging.Format,
	})

	log.Info("Starting Gateway-Controller",
		slog.String("version", Version),
		slog.String("git_commit", GitCommit),
		slog.String("build_date", BuildDate),
		slog.String("config_file", *configPath),
		slog.String("storage_type", cfg.GatewayController.Storage.Type),
		slog.Bool("access_logs_enabled", cfg.GatewayController.Router.AccessLogs.Enabled),
		slog.String("control_plane_host", cfg.GatewayController.ControlPlane.Host),
		slog.Bool("control_plane_token_configured", cfg.GatewayController.ControlPlane.Token != ""),
	)

	if !cfg.GatewayController.Auth.Basic.Enabled && !cfg.GatewayController.Auth.IDP.Enabled {
		log.Warn("No authentication configured: both basic auth and IDP are disabled. Gateway Controller API will allow all requests without authentication")
	}

	// Initialize storage based on type
	var db storage.Storage
	if cfg.IsPersistentMode() {
		switch cfg.GatewayController.Storage.Type {
		case "sqlite":
			log.Info("Initializing SQLite storage", slog.String("path", cfg.GatewayController.Storage.SQLite.Path))
			db, err = storage.NewSQLiteStorage(cfg.GatewayController.Storage.SQLite.Path, log)
			if err != nil {
				// Check for database locked error and provide clear guidance
				if err.Error() == "database is locked" || err.Error() == "failed to open database: database is locked" {
					log.Error("Database is locked by another process",
						slog.String("database_path", cfg.GatewayController.Storage.SQLite.Path),
						slog.String("troubleshooting", "Check if another gateway-controller instance is running or remove stale WAL files"))
					os.Exit(1)
				}
				log.Error("Failed to initialize SQLite database", slog.Any("error", err))
				os.Exit(1)
			}
			defer db.Close()
		case "postgres":
			log.Error("PostgreSQL storage not yet implemented")
			os.Exit(1)
		default:
			log.Error("Unknown storage type", slog.String("type", cfg.GatewayController.Storage.Type))
			os.Exit(1)
		}
	} else {
		log.Info("Running in memory-only mode (no persistent storage)")
	}

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

	// Load configurations from database on startup (if persistent mode)
	if cfg.IsPersistentMode() && db != nil {
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
	}

	// Initialize xDS snapshot manager with router config
	snapshotManager := xds.NewSnapshotManager(configStore, log, &cfg.GatewayController.Router, db, cfg)

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

	// Start xDS gRPC server with SDS support
	xdsServer := xds.NewServer(snapshotManager, sdsSecretManager, cfg.GatewayController.Server.XDSPort, log)
	go func() {
		if err := xdsServer.Start(); err != nil {
			log.Error("xDS server failed", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	// Generate initial API key snapshot if API keys were loaded from database
	if cfg.IsPersistentMode() && apiKeyXDSManager.GetAPIKeyCount() > 0 {
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

	// Load policy definitions from files (must be before policy derivation and validator)
	policyLoader := utils.NewPolicyLoader(log)
	policyDir := cfg.GatewayController.Policies.DefinitionsPath
	log.Info("Loading policy definitions from directory", slog.String("directory", policyDir))
	policyDefinitions, err := policyLoader.LoadPoliciesFromDirectory(policyDir)
	if err != nil {
		log.Error("Failed to load policy definitions", slog.Any("error", err))
		os.Exit(1)
	}
	log.Info("Policy definitions loaded", slog.Int("count", len(policyDefinitions)))

	// Initialize policy store and start policy xDS server if enabled
	var policyXDSServer *policyxds.Server
	var policyManager *policyxds.PolicyManager
	if cfg.GatewayController.PolicyServer.Enabled {
		log.Info("Initializing Policy xDS server", slog.Int("port", cfg.GatewayController.PolicyServer.Port))

		// Initialize policy store
		policyStore := storage.NewPolicyStore()

		// Initialize policy snapshot manager
		policySnapshotManager := policyxds.NewSnapshotManager(policyStore, log)
		// Initialize policy manager (used to derive policies from API configurations)
		policyManager = policyxds.NewPolicyManager(policyStore, policySnapshotManager, log)

		// Load policies from existing API configurations on startup
		if cfg.IsPersistentMode() {
			log.Info("Deriving policies from loaded API configurations")
			loadedAPIs := configStore.GetAll()
			derivedCount := 0
			for _, apiConfig := range loadedAPIs {
				// Derive policy configuration from API
				if apiConfig.Configuration.Kind == api.RestApi {
					storedPolicy := policybuilder.DerivePolicyFromAPIConfig(apiConfig, &cfg.GatewayController.Router, cfg, policyDefinitions)
					if storedPolicy != nil {
						if err := policyStore.Set(storedPolicy); err != nil {
							log.Warn("Failed to load policy from API",
								slog.String("api_id", apiConfig.ID),
								slog.Any("error", err))
						} else {
							derivedCount++
						}
					}
				}
			}
			log.Info("Loaded policies from API configurations",
				slog.Int("total_apis", len(loadedAPIs)),
				slog.Int("policies_derived", derivedCount))
		}

		// Generate initial policy snapshot
		log.Info("Generating initial policy xDS snapshot")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := policySnapshotManager.UpdateSnapshot(ctx); err != nil {
			log.Warn("Failed to generate initial policy xDS snapshot", slog.Any("error", err))
		}
		cancel()

		// Start policy xDS server in a separate goroutine
		var serverOpts []policyxds.ServerOption
		if cfg.GatewayController.PolicyServer.TLS.Enabled {
			serverOpts = append(serverOpts, policyxds.WithTLS(
				cfg.GatewayController.PolicyServer.TLS.CertFile,
				cfg.GatewayController.PolicyServer.TLS.KeyFile,
			))
		}
		policyXDSServer = policyxds.NewServer(policySnapshotManager, apiKeySnapshotManager, lazyResourceSnapshotManager, cfg.GatewayController.PolicyServer.Port, log, serverOpts...)
		go func() {
			if err := policyXDSServer.Start(); err != nil {
				log.Error("Policy xDS server failed", slog.Any("error", err))
				os.Exit(1)
			}
		}()
	} else {
		log.Info("Policy xDS server is disabled")
	}

	// Load llm provider templates from files
	templateLoader := utils.NewLLMTemplateLoader(log)
	templateDir := cfg.GatewayController.LLM.TemplateDefinitionsPath
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

	// Initialize and start control plane client with dependencies for API creation and API key management
	cpClient := controlplane.NewClient(cfg.GatewayController.ControlPlane, log, configStore, db, snapshotManager, validator, &cfg.GatewayController.Router, apiKeyXDSManager, &cfg.GatewayController.APIKey, policyManager, cfg, policyDefinitions)
	if err := cpClient.Start(); err != nil {
		log.Error("Failed to start control plane client", slog.Any("error", err))
		// Don't fail startup - gateway can run in degraded mode without control plane
	}

	// Initialize Gin router
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()

	// Add middleware
	// IMPORTANT: CorrelationIDMiddleware must be registered first to ensure
	// correlation ID is available in context for subsequent middleware and handlers
	router.Use(middleware.CorrelationIDMiddleware(log))
	router.Use(middleware.ErrorHandlingMiddleware(log))
	router.Use(middleware.LoggingMiddleware(log))
	// Add metrics middleware if metrics are enabled
	if cfg.GatewayController.Metrics.Enabled {
		router.Use(middleware.MetricsMiddleware())
	}
	authConfig := generateAuthConfig(cfg)
	authMiddleWare, err := authenticators.AuthMiddleware(authConfig, log)
	if err != nil {
		log.Error("Failed to create auth middleware", slog.Any("error", err))
		os.Exit(1)
	}
	router.Use(authMiddleWare)
	router.Use(authenticators.AuthorizationMiddleware(authConfig, log))
	router.Use(gin.Recovery())

	// Initialize API server with the configured validator and API key manager
	apiServer := handlers.NewAPIServer(configStore, db, snapshotManager, policyManager, lazyResourceXDSManager, log, cpClient,
		policyDefinitions, templateDefinitions, validator, apiKeyXDSManager, cfg)

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

	// Register API routes (includes certificate management endpoints from OpenAPI spec)
	api.RegisterHandlers(router, apiServer)

	// Start metrics server if enabled
	var metricsServer *metrics.Server
	var metricsCtxCancel context.CancelFunc
	if cfg.GatewayController.Metrics.Enabled {
		log.Info("Starting metrics server", slog.Int("port", cfg.GatewayController.Metrics.Port))

		// Set build info metric
		metrics.Info.WithLabelValues(Version, cfg.GatewayController.Storage.Type, BuildDate).Set(1)

		metricsServer = metrics.NewServer(&cfg.GatewayController.Metrics, log)
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
	log.Info("Starting REST API server", slog.Int("port", cfg.GatewayController.Server.APIPort))

	// Setup graceful shutdown
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.GatewayController.Server.APIPort),
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Failed to start REST API server", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	log.Info("Gateway Controller started successfully")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Gateway-Controller")

	// Graceful shutdown with timeout
	ctx, cancel = context.WithTimeout(context.Background(), cfg.GatewayController.Server.ShutdownTimeout)
	defer cancel()

	// Stop control plane client first
	cpClient.Stop()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", slog.Any("error", err))
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

	log.Info("Gateway-Controller stopped")
}

func generateAuthConfig(config *config.Config) commonmodels.AuthConfig {
	var DefaultResourceRoles = map[string][]string{
		"POST /apis":       {"admin", "developer"},
		"GET /apis":        {"admin", "developer"},
		"GET /apis/:id":    {"admin", "developer"},
		"PUT /apis/:id":    {"admin", "developer"},
		"DELETE /apis/:id": {"admin", "developer"},

		"GET /certificates":         {"admin", "developer"},
		"POST /certificates":        {"admin", "developer"},
		"DELETE /certificates/:id":  {"admin"},
		"POST /certificates/reload": {"admin"},

		"GET /policies": {"admin", "developer"},

		"POST /mcp-proxies":       {"admin", "developer"},
		"GET /mcp-proxies":        {"admin", "developer"},
		"GET /mcp-proxies/:id":    {"admin", "developer"},
		"PUT /mcp-proxies/:id":    {"admin", "developer"},
		"DELETE /mcp-proxies/:id": {"admin", "developer"},

		"POST /llm-provider-templates":       {"admin"},
		"GET /llm-provider-templates":        {"admin"},
		"GET /llm-provider-templates/:id":    {"admin"},
		"PUT /llm-provider-templates/:id":    {"admin"},
		"DELETE /llm-provider-templates/:id": {"admin"},

		"POST /llm-providers":       {"admin"},
		"GET /llm-providers":        {"admin", "developer"},
		"GET /llm-providers/:id":    {"admin", "developer"},
		"PUT /llm-providers/:id":    {"admin"},
		"DELETE /llm-providers/:id": {"admin"},

		"POST /llm-proxies":       {"admin", "developer"},
		"GET /llm-proxies":        {"admin", "developer"},
		"GET /llm-proxies/:id":    {"admin", "developer"},
		"PUT /llm-proxies/:id":    {"admin", "developer"},
		"DELETE /llm-proxies/:id": {"admin", "developer"},

		"POST /apis/:id/api-keys":                        {"admin", "consumer"},
		"GET /apis/:id/api-keys":                         {"admin", "consumer"},
		"PUT /apis/:id/api-keys/:apiKeyName":             {"admin", "consumer"},
		"POST /apis/:id/api-keys/:apiKeyName/regenerate": {"admin", "consumer"},
		"DELETE /apis/:id/api-keys/:apiKeyName":          {"admin", "consumer"},

		"GET /config_dump": {"admin"},
	}
	basicAuth := commonmodels.BasicAuth{Enabled: false}
	idpAuth := commonmodels.IDPConfig{Enabled: false}
	if config.GatewayController.Auth.Basic.Enabled {
		users := make([]commonmodels.User, len(config.GatewayController.Auth.Basic.Users))
		for i, authUser := range config.GatewayController.Auth.Basic.Users {
			users[i] = commonmodels.User{
				Username:       authUser.Username,
				Password:       authUser.Password,
				PasswordHashed: authUser.PasswordHashed,
				Roles:          authUser.Roles,
			}
		}
		basicAuth = commonmodels.BasicAuth{Enabled: true, Users: users}
	}
	if config.GatewayController.Auth.IDP.Enabled {
		idpAuth = commonmodels.IDPConfig{Enabled: true, IssuerURL: config.GatewayController.Auth.IDP.Issuer,
			JWKSUrl:           config.GatewayController.Auth.IDP.JWKSURL,
			ScopeClaim:        config.GatewayController.Auth.IDP.RolesClaim,
			PermissionMapping: &config.GatewayController.Auth.IDP.RoleMapping,
		}
	}
	authConfig := commonmodels.AuthConfig{BasicAuth: &basicAuth,
		JWTConfig:     &idpAuth,
		ResourceRoles: DefaultResourceRoles,
		SkipPaths:     []string{"/health"},
	}
	return authConfig
}
