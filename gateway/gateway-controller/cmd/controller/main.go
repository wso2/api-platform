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
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/logger"
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

	// Initialize xDS snapshot manager with access log config
	snapshotManager := xds.NewSnapshotManager(configStore, log, cfg.Router.AccessLogs)

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
	apiServer := handlers.NewAPIServer(configStore, db, snapshotManager, log)

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

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", zap.Error(err))
	}

	xdsServer.Stop()

	log.Info("Gateway-Controller stopped")
}
