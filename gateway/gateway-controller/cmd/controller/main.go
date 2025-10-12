package main

import (
	"context"
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
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/logger"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	log, err := logger.NewLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("Starting Gateway-Controller")

	// Get configuration from environment
	dbPath := getEnv("DB_PATH", "/data/gateway-controller.db")
	apiPort := getEnv("API_PORT", "9090")
	xdsPort := getEnvInt("XDS_PORT", 18000)

	// Initialize bbolt database
	log.Info("Initializing database", zap.String("path", dbPath))
	db, err := storage.NewBBoltStorage(dbPath)
	if err != nil {
		log.Fatal("Failed to initialize database", zap.Error(err))
	}
	defer db.Close()

	// Initialize in-memory config store
	configStore := storage.NewConfigStore()

	// Load configurations from database on startup
	log.Info("Loading configurations from database")
	if err := storage.LoadFromDatabase(db.GetDB(), configStore); err != nil {
		log.Fatal("Failed to load configurations from database", zap.Error(err))
	}
	log.Info("Loaded configurations", zap.Int("count", len(configStore.GetAll())))

	// Initialize xDS snapshot manager
	snapshotManager := xds.NewSnapshotManager(configStore, log)

	// Generate initial xDS snapshot
	log.Info("Generating initial xDS snapshot")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := snapshotManager.UpdateSnapshot(ctx); err != nil {
		log.Warn("Failed to generate initial xDS snapshot", zap.Error(err))
	}
	cancel()

	// Start xDS gRPC server
	log.Info("Starting xDS server", zap.Int("port", xdsPort))
	xdsServer := xds.NewServer(snapshotManager, xdsPort, log)
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
	router.Use(middleware.ErrorHandlingMiddleware(log))
	router.Use(middleware.LoggingMiddleware(log))
	router.Use(gin.Recovery())

	// Initialize API server
	apiServer := handlers.NewAPIServer(configStore, db, snapshotManager, log)

	// Register API routes
	api.RegisterHandlers(router, apiServer)

	// Start REST API server
	log.Info("Starting REST API server", zap.String("port", apiPort))

	// Setup graceful shutdown
	srv := &http.Server{
		Addr:    ":" + apiPort,
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
	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", zap.Error(err))
	}

	xdsServer.Stop()

	log.Info("Gateway-Controller stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		fmt.Sscanf(value, "%d", &intValue)
		return intValue
	}
	return defaultValue
}
