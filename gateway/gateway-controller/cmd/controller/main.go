package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/apikeyxds"

	"github.com/gin-gonic/gin"
	"github.com/wso2/api-platform/common/authenticators"
	commonmodels "github.com/wso2/api-platform/common/models"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/handlers"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/controlplane"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/logger"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	policyenginev1 "github.com/wso2/api-platform/sdk/gateway/policyengine/v1"
	"go.uber.org/zap"
)

// Version information (set via ldflags during build)
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "config/config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger with config
	log, err := logger.NewLogger(logger.Config{
		Level:  cfg.GatewayController.Logging.Level,
		Format: cfg.GatewayController.Logging.Format,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("Starting Gateway-Controller",
		zap.String("version", Version),
		zap.String("git_commit", GitCommit),
		zap.String("build_date", BuildDate),
		zap.String("config_file", *configPath),
		zap.String("storage_type", cfg.GatewayController.Storage.Type),
		zap.Bool("access_logs_enabled", cfg.GatewayController.Router.AccessLogs.Enabled),
		zap.String("control_plane_host", cfg.GatewayController.ControlPlane.Host),
		zap.Bool("control_plane_token_configured", cfg.GatewayController.ControlPlane.Token != ""),
	)

	if !cfg.GatewayController.Auth.Basic.Enabled && !cfg.GatewayController.Auth.IDP.Enabled {
		log.Warn("No authentication configured: both basic auth and IDP are disabled. Gateway Controller API will allow all requests without authentication")
	}

	// Initialize storage based on type
	var db storage.Storage
	if cfg.IsPersistentMode() {
		switch cfg.GatewayController.Storage.Type {
		case "sqlite":
			log.Info("Initializing SQLite storage", zap.String("path", cfg.GatewayController.Storage.SQLite.Path))
			db, err = storage.NewSQLiteStorage(cfg.GatewayController.Storage.SQLite.Path, log)
			if err != nil {
				// Check for database locked error and provide clear guidance
				if err.Error() == "database is locked" || err.Error() == "failed to open database: database is locked" {
					log.Fatal("Database is locked by another process",
						zap.String("database_path", cfg.GatewayController.Storage.SQLite.Path),
						zap.String("troubleshooting", "Check if another gateway-controller instance is running or remove stale WAL files"))
				}
				log.Fatal("Failed to initialize SQLite database", zap.Error(err))
			}
			defer db.Close()
		case "postgres":
			log.Fatal("PostgreSQL storage not yet implemented")
		default:
			log.Fatal("Unknown storage type", zap.String("type", cfg.GatewayController.Storage.Type))
		}
	} else {
		log.Info("Running in memory-only mode (no persistent storage)")
	}

	// Initialize in-memory config store
	configStore := storage.NewConfigStore()

	// Initialize in-memory API key store for xDS
	apiKeyStore := storage.NewAPIKeyStore(log)

	// Load configurations from database on startup (if persistent mode)
	if cfg.IsPersistentMode() && db != nil {
		log.Info("Loading configurations from database")
		if err := storage.LoadFromDatabase(db, configStore); err != nil {
			log.Fatal("Failed to load configurations from database", zap.Error(err))
		}
		if err := storage.LoadLLMProviderTemplatesFromDatabase(db, configStore); err != nil {
			log.Fatal("Failed to load llm provider template configurations from database", zap.Error(err))
		}
		log.Info("Loaded configurations", zap.Int("count", len(configStore.GetAll())))

		// Load API keys from database into both in-memory stores
		log.Info("Loading API keys from database")
		if err := storage.LoadAPIKeysFromDatabase(db, configStore, apiKeyStore); err != nil {
			log.Fatal("Failed to load API keys from database", zap.Error(err))
		}
		log.Info("Loaded API keys", zap.Int("count", apiKeyStore.Count()))
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
			log.Warn("Failed to initialize SDS secrets", zap.Error(err))
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
		log.Warn("Failed to generate initial xDS snapshot", zap.Error(err))
	}
	cancel()

	// Start xDS gRPC server with SDS support
	xdsServer := xds.NewServer(snapshotManager, sdsSecretManager, cfg.GatewayController.Server.XDSPort, log)
	go func() {
		if err := xdsServer.Start(); err != nil {
			log.Fatal("xDS server failed", zap.Error(err))
		}
	}()

	apiKeySnapshotManager := apikeyxds.NewAPIKeySnapshotManager(apiKeyStore, log)
	apiKeyXDSManager := apikeyxds.NewAPIKeyStateManager(apiKeyStore, apiKeySnapshotManager, log)

	// Generate initial API key snapshot if API keys were loaded from database
	if cfg.IsPersistentMode() && apiKeyStore.Count() > 0 {
		log.Info("Generating initial API key snapshot for policy engine", zap.Int("api_key_count", apiKeyStore.Count()))
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := apiKeySnapshotManager.UpdateSnapshot(ctx); err != nil {
			log.Warn("Failed to generate initial API key snapshot", zap.Error(err))
		} else {
			log.Info("Initial API key snapshot generated successfully")
		}
		cancel()
	}

	// Initialize policy store and start policy xDS server if enabled
	var policyXDSServer *policyxds.Server
	var policyManager *policyxds.PolicyManager
	if cfg.GatewayController.PolicyServer.Enabled {
		log.Info("Initializing Policy xDS server", zap.Int("port", cfg.GatewayController.PolicyServer.Port))

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
					storedPolicy := derivePolicyFromAPIConfig(apiConfig, &cfg.GatewayController.Router)
					if storedPolicy != nil {
						if err := policyStore.Set(storedPolicy); err != nil {
							log.Warn("Failed to load policy from API",
								zap.String("api_id", apiConfig.ID),
								zap.Error(err))
						} else {
							derivedCount++
						}
					}
				}
			}
			log.Info("Loaded policies from API configurations",
				zap.Int("total_apis", len(loadedAPIs)),
				zap.Int("policies_derived", derivedCount))
		}

		// Generate initial policy snapshot
		log.Info("Generating initial policy xDS snapshot")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := policySnapshotManager.UpdateSnapshot(ctx); err != nil {
			log.Warn("Failed to generate initial policy xDS snapshot", zap.Error(err))
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
		policyXDSServer = policyxds.NewServer(policySnapshotManager, apiKeySnapshotManager, cfg.GatewayController.PolicyServer.Port, log, serverOpts...)
		go func() {
			if err := policyXDSServer.Start(); err != nil {
				log.Fatal("Policy xDS server failed", zap.Error(err))
			}
		}()
	} else {
		log.Info("Policy xDS server is disabled")
	}

	// Load policy definitions from files (must be done before creating validator)
	policyLoader := utils.NewPolicyLoader(log)
	policyDir := cfg.GatewayController.Policies.DefinitionsPath
	log.Info("Loading policy definitions from directory", zap.String("directory", policyDir))
	policyDefinitions, err := policyLoader.LoadPoliciesFromDirectory(policyDir)
	if err != nil {
		log.Fatal("Failed to load policy definitions", zap.Error(err))
	}
	log.Info("Policy definitions loaded", zap.Int("count", len(policyDefinitions)))

	// Load llm provider templates from files
	templateLoader := utils.NewLLMTemplateLoader(log)
	templateDir := cfg.GatewayController.LLM.TemplateDefinitionsPath
	log.Info("Loading llm provider templates from directory", zap.String("directory", templateDir))
	templateDefinitions, err := templateLoader.LoadTemplatesFromDirectory(templateDir)
	if err != nil {
		log.Fatal("Failed to load llm provider templates", zap.Error(err))
	}
	log.Info("Default llm provider templates loaded", zap.Int("count", len(templateDefinitions)))

	// Create validator with policy validation support
	validator := config.NewAPIValidator()
	policyValidator := config.NewPolicyValidator(policyDefinitions)
	validator.SetPolicyValidator(policyValidator)

	// Initialize and start control plane client with dependencies for API creation
	cpClient := controlplane.NewClient(cfg.GatewayController.ControlPlane, log, configStore, db, snapshotManager, validator, &cfg.GatewayController.Router)
	if err := cpClient.Start(); err != nil {
		log.Error("Failed to start control plane client", zap.Error(err))
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
	authConfig := generateAuthConfig(cfg)
	authMiddleWare, err := authenticators.AuthMiddleware(authConfig, log)
	if err != nil {
		log.Fatal("Failed to create auth middleware", zap.Error(err))
	}
	router.Use(authMiddleWare)
	router.Use(authenticators.AuthorizationMiddleware(authConfig, log))
	router.Use(gin.Recovery())

	// Initialize API server with the configured validator and API key manager
	apiServer := handlers.NewAPIServer(configStore, db, snapshotManager, policyManager, log, cpClient,
		policyDefinitions, templateDefinitions, validator, &cfg.GatewayController.Router, apiKeyXDSManager)

	// Register API routes (includes certificate management endpoints from OpenAPI spec)
	api.RegisterHandlers(router, apiServer)

	// Start REST API server
	log.Info("Starting REST API server", zap.Int("port", cfg.GatewayController.Server.APIPort))

	// Setup graceful shutdown
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.GatewayController.Server.APIPort),
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start REST API server", zap.Error(err))
		}
	}()

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
		log.Error("Server forced to shutdown", zap.Error(err))
	}

	xdsServer.Stop()

	// Stop policy xDS server if it was started
	if policyXDSServer != nil {
		policyXDSServer.Stop()
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

		"POST /apis/:id/generate-api-key":                {"admin", "consumer"},
		"GET /apis/:id/api-keys":                         {"admin", "consumer"},
		"POST /apis/:id/api-keys/:apiKeyName/regenerate": {"admin", "consumer"},
		"POST /apis/:id/revoke-api-key":                  {"admin", "consumer"},

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

// derivePolicyFromAPIConfig derives a policy configuration from an API configuration
// This is a simplified version of the buildStoredPolicyFromAPI function from handlers
func derivePolicyFromAPIConfig(cfg *models.StoredConfig, routerConfig *config.RouterConfig) *models.StoredPolicyConfig {
	apiCfg := &cfg.Configuration
	apiData, err := apiCfg.Spec.AsAPIConfigData()
	if err != nil {
		return nil
	}

	// Collect API-level policies
	apiPolicies := make(map[string]policyenginev1.PolicyInstance)
	if apiData.Policies != nil {
		for _, p := range *apiData.Policies {
			apiPolicies[p.Name] = convertAPIPolicyToModel(p, policy.LevelAPI)
		}
	}

	// Build routes with merged policies
	routes := make([]policyenginev1.PolicyChain, 0)
	for _, op := range apiData.Operations {
		var finalPolicies []policyenginev1.PolicyInstance

		if op.Policies != nil && len(*op.Policies) > 0 {
			// Operation has policies
			finalPolicies = make([]policyenginev1.PolicyInstance, 0, len(*op.Policies))
			addedNames := make(map[string]struct{})

			for _, opPolicy := range *op.Policies {
				finalPolicies = append(finalPolicies, convertAPIPolicyToModel(opPolicy, policy.LevelRoute))
				addedNames[opPolicy.Name] = struct{}{}
			}

			// Add remaining API-level policies
			if apiData.Policies != nil {
				for _, apiPolicy := range *apiData.Policies {
					if _, exists := addedNames[apiPolicy.Name]; !exists {
						finalPolicies = append(finalPolicies, apiPolicies[apiPolicy.Name])
					}
				}
			}
		} else {
			// No operation policies: use API-level policies
			if apiData.Policies != nil {
				finalPolicies = make([]policyenginev1.PolicyInstance, 0, len(*apiData.Policies))
				for _, p := range *apiData.Policies {
					finalPolicies = append(finalPolicies, apiPolicies[p.Name])
				}
			}
		}

		// Determine effective vhosts (fallback to global router defaults when not provided)
		effectiveMainVHost := routerConfig.VHosts.Main.Default
		effectiveSandboxVHost := routerConfig.VHosts.Sandbox.Default
		if apiData.Vhosts != nil {
			if strings.TrimSpace(apiData.Vhosts.Main) != "" {
				effectiveMainVHost = apiData.Vhosts.Main
			}
			if apiData.Vhosts.Sandbox != nil && strings.TrimSpace(*apiData.Vhosts.Sandbox) != "" {
				effectiveSandboxVHost = *apiData.Vhosts.Sandbox
			}
		}

		vhosts := []string{effectiveMainVHost}
		if apiData.Upstream.Sandbox != nil && apiData.Upstream.Sandbox.Url != nil &&
			strings.TrimSpace(*apiData.Upstream.Sandbox.Url) != "" {
			vhosts = append(vhosts, effectiveSandboxVHost)
		}

		for _, vhost := range vhosts {
			routes = append(routes, policyenginev1.PolicyChain{
				RouteKey: xds.GenerateRouteName(string(op.Method), apiData.Context, apiData.Version, op.Path, vhost),
				Policies: finalPolicies,
			})
		}
	}

	// If there are no policies at all, return nil
	policyCount := 0
	for _, r := range routes {
		policyCount += len(r.Policies)
	}
	if policyCount == 0 {
		return nil
	}

	now := time.Now().Unix()
	return &models.StoredPolicyConfig{
		ID: cfg.ID + "-policies",
		Configuration: policyenginev1.Configuration{
			Routes: routes,
			Metadata: policyenginev1.Metadata{
				CreatedAt:       now,
				UpdatedAt:       now,
				ResourceVersion: 0,
				APIName:         apiData.DisplayName,
				Version:         apiData.Version,
				Context:         apiData.Context,
			},
		},
		Version: 0,
	}
}

// convertAPIPolicyToModel converts generated api.Policy to policyenginev1.PolicyInstance
func convertAPIPolicyToModel(p api.Policy, attachedTo policy.Level) policyenginev1.PolicyInstance {
	paramsMap := make(map[string]interface{})
	if p.Params != nil {
		for k, v := range *p.Params {
			paramsMap[k] = v
		}
	}

	// Add attachedTo metadata to parameters
	if attachedTo != "" {
		paramsMap["attachedTo"] = string(attachedTo)
	}

	return policyenginev1.PolicyInstance{
		Name:               p.Name,
		Version:            p.Version,
		Enabled:            true, // Default to enabled
		ExecutionCondition: p.ExecutionCondition,
		Parameters:         paramsMap,
	}
}
