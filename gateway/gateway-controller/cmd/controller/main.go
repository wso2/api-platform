package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/handlers"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/controlplane"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/logger"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	"go.uber.org/zap"
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
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("Starting Gateway-Controller",
		zap.String("config_file", *configPath),
		zap.String("storage_type", cfg.Storage.Type),
		zap.Bool("access_logs_enabled", cfg.Router.AccessLogs.Enabled),
		zap.String("control_plane_host", cfg.ControlPlane.Host),
		zap.Bool("control_plane_token_configured", cfg.ControlPlane.Token != ""),
	)

	// Initialize storage based on type
	var db storage.Storage
	if cfg.IsPersistentMode() {
		switch cfg.Storage.Type {
		case "sqlite":
			log.Info("Initializing SQLite storage", zap.String("path", cfg.Storage.SQLite.Path))
			db, err = storage.NewSQLiteStorage(cfg.Storage.SQLite.Path, log)
			if err != nil {
				// Check for database locked error and provide clear guidance
				if err.Error() == "database is locked" || err.Error() == "failed to open database: database is locked" {
					log.Fatal("Database is locked by another process",
						zap.String("database_path", cfg.Storage.SQLite.Path),
						zap.String("troubleshooting", "Check if another gateway-controller instance is running or remove stale WAL files"))
				}
				log.Fatal("Failed to initialize SQLite database", zap.Error(err))
			}
			defer db.Close()
		case "postgres":
			log.Fatal("PostgreSQL storage not yet implemented")
		default:
			log.Fatal("Unknown storage type", zap.String("type", cfg.Storage.Type))
		}
	} else {
		log.Info("Running in memory-only mode (no persistent storage)")
	}

	// Initialize in-memory config store
	configStore := storage.NewConfigStore()

	// Load configurations from database on startup (if persistent mode)
	if cfg.IsPersistentMode() && db != nil {
		log.Info("Loading configurations from database")
		if err := storage.LoadFromDatabase(db, configStore); err != nil {
			log.Fatal("Failed to load configurations from database", zap.Error(err))
		}
		log.Info("Loaded configurations", zap.Int("count", len(configStore.GetAll())))
	}

	// Initialize xDS snapshot manager with router config
	snapshotManager := xds.NewSnapshotManager(configStore, log, &cfg.Router)

	// Generate initial xDS snapshot
	log.Info("Generating initial xDS snapshot")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := snapshotManager.UpdateSnapshot(ctx, ""); err != nil {
		log.Warn("Failed to generate initial xDS snapshot", zap.Error(err))
	}
	cancel()

	// Start xDS gRPC server
	xdsServer := xds.NewServer(snapshotManager, cfg.Server.XDSPort, log)
	go func() {
		if err := xdsServer.Start(); err != nil {
			log.Fatal("xDS server failed", zap.Error(err))
		}
	}()

	// Initialize policy store and start policy xDS server if enabled
	var policyXDSServer *policyxds.Server
	var policyManager *policyxds.PolicyManager
	if cfg.PolicyServer.Enabled {
		log.Info("Initializing Policy xDS server", zap.Int("port", cfg.PolicyServer.Port))

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
				storedPolicy := derivePolicyFromAPIConfig(apiConfig)
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
		if cfg.PolicyServer.TLS.Enabled {
			serverOpts = append(serverOpts, policyxds.WithTLS(
				cfg.PolicyServer.TLS.CertFile,
				cfg.PolicyServer.TLS.KeyFile,
			))
		}
		policyXDSServer = policyxds.NewServer(policySnapshotManager, cfg.PolicyServer.Port, log, serverOpts...)
		go func() {
			if err := policyXDSServer.Start(); err != nil {
				log.Fatal("Policy xDS server failed", zap.Error(err))
			}
		}()
	} else {
		log.Info("Policy xDS server is disabled")
	}

	// Initialize and start control plane client with dependencies for API creation
	cpClient := controlplane.NewClient(cfg.ControlPlane, log, configStore, db, snapshotManager)
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
	router.Use(gin.Recovery())

	// Initialize API server
	apiServer := handlers.NewAPIServer(configStore, db, snapshotManager, policyManager, log, cpClient)

	// Register API routes
	api.RegisterHandlers(router, apiServer)

	// Start REST API server
	log.Info("Starting REST API server", zap.Int("port", cfg.Server.APIPort))

	// Setup graceful shutdown
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.APIPort),
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
	ctx, cancel = context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
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

// derivePolicyFromAPIConfig derives a policy configuration from an API configuration
// This is a simplified version of the buildStoredPolicyFromAPI function from handlers
func derivePolicyFromAPIConfig(cfg *models.StoredAPIConfig) *models.StoredPolicyConfig {
	apiCfg := &cfg.Configuration
	apiData := apiCfg.Data

	// Collect API-level policies
	apiPolicies := make(map[string]models.Policy)
	if apiData.Policies != nil {
		for _, p := range *apiData.Policies {
			apiPolicies[p.Name] = convertAPIPolicyToModel(p)
		}
	}

	// Build routes with merged policies
	routes := make([]models.RoutePolicy, 0)
	for _, op := range apiData.Operations {
		var finalPolicies []models.Policy

		if op.Policies != nil && len(*op.Policies) > 0 {
			// Operation has policies
			finalPolicies = make([]models.Policy, 0, len(*op.Policies))
			addedNames := make(map[string]struct{})

			for _, opPolicy := range *op.Policies {
				finalPolicies = append(finalPolicies, convertAPIPolicyToModel(opPolicy))
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
				finalPolicies = make([]models.Policy, 0, len(*apiData.Policies))
				for _, p := range *apiData.Policies {
					finalPolicies = append(finalPolicies, apiPolicies[p.Name])
				}
			}
		}

		// Construct route key
		routeKey := fmt.Sprintf("%s|%s|%s%s", op.Method, apiData.Version, apiData.Context, op.Path)
		routes = append(routes, models.RoutePolicy{
			RouteKey: routeKey,
			Policies: finalPolicies,
		})
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
		Configuration: models.PolicyConfiguration{
			Routes: routes,
			Metadata: models.Metadata{
				CreatedAt:       now,
				UpdatedAt:       now,
				ResourceVersion: 0,
				APIName:         apiData.Name,
				Version:         apiData.Version,
				Context:         apiData.Context,
			},
		},
		Version: 0,
	}
}

// convertAPIPolicyToModel converts generated api.Policy to models.Policy
func convertAPIPolicyToModel(p api.Policy) models.Policy {
	paramsMap := make(map[string]interface{})
	if p.Params != nil {
		for k, v := range *p.Params {
			paramsMap[k] = v
		}
	}
	return models.Policy{
		Name:               p.Name,
		Version:            p.Version,
		ExecutionCondition: p.ExecutionCondition,
		Params:             paramsMap,
	}
}
