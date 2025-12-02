package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"google.golang.org/grpc"

	"github.com/policy-engine/policy-engine/internal/admin"
	"github.com/policy-engine/policy-engine/internal/config"
	"github.com/policy-engine/policy-engine/internal/executor"
	"github.com/policy-engine/policy-engine/internal/kernel"
	"github.com/policy-engine/policy-engine/internal/pkg/cel"
	"github.com/policy-engine/policy-engine/internal/registry"
	"github.com/policy-engine/policy-engine/internal/xdsclient"
)

var (
	configFile       = flag.String("config", "configs/config.yaml", "Path to configuration file")
	policyChainsFile = flag.String("policy-chains-file", "", "Path to policy chains file (enables file mode)")
	xdsServerAddr    = flag.String("xds-server", "", "xDS server address (e.g., localhost:18000)")
	xdsNodeID        = flag.String("xds-node-id", "", "xDS node identifier")
)

func main() {
	flag.Parse()

	// Load configuration from file
	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Apply flag overrides
	applyFlagOverrides(cfg)

	// Set up structured logging based on configuration
	logger := setupLogger(cfg)
	slog.SetDefault(logger)
	ctx := context.Background()

	slog.InfoContext(ctx, "Policy Engine starting",
		"config_file", *configFile,
		"config_mode", cfg.ConfigMode.Mode,
		"extproc_port", cfg.Server.ExtProcPort)

	// Initialize core components
	k := kernel.NewKernel()
	reg := registry.GetRegistry()

	// Initialize CEL evaluator
	celEvaluator, err := cel.NewCELEvaluator()
	if err != nil {
		slog.ErrorContext(ctx, "Failed to create CEL evaluator", "error", err)
		os.Exit(1)
	}

	// Initialize chain executor
	chainExecutor := executor.NewChainExecutor(reg, celEvaluator)

	// Policy registration happens automatically via Builder-generated plugin_registry.go
	slog.InfoContext(ctx, "Policies registered via Builder-generated code")

	// Initialize configuration source based on mode
	var xdsClient *xdsclient.Client
	switch cfg.ConfigMode.Mode {
	case "xds":
		xdsClient, err = initializeXDSClient(ctx, cfg, k, reg)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to initialize xDS client", "error", err)
			os.Exit(1)
		}
		defer xdsClient.Stop()
		slog.InfoContext(ctx, "xDS client started successfully")

	case "file":
		if err := initializeFileConfig(ctx, cfg, k, reg); err != nil {
			slog.ErrorContext(ctx, "Failed to load file configuration", "error", err)
			os.Exit(1)
		}
		slog.InfoContext(ctx, "File configuration loaded successfully")

	default:
		slog.ErrorContext(ctx, "Invalid config mode", "mode", cfg.ConfigMode.Mode)
		os.Exit(1)
	}

	// Create and start ext_proc gRPC server
	extprocServer := kernel.NewExternalProcessorServer(k, chainExecutor)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.ExtProcPort))
	if err != nil {
		slog.ErrorContext(ctx, "Failed to listen on port",
			"port", cfg.Server.ExtProcPort,
			"error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	extprocv3.RegisterExternalProcessorServer(grpcServer, extprocServer)

	slog.InfoContext(ctx, "Policy Engine listening", "port", cfg.Server.ExtProcPort)

	// Start admin HTTP server if enabled
	var adminServer *admin.Server
	if cfg.Admin.Enabled {
		adminServer = admin.NewServer(&cfg.Admin, k, reg)
		go func() {
			if err := adminServer.Start(ctx); err != nil {
				slog.ErrorContext(ctx, "Admin server error", "error", err)
			}
		}()
	}

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	serverErrCh := make(chan error, 1)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			serverErrCh <- err
		}
	}()

	// Wait for shutdown signal or server error
	select {
	case sig := <-sigChan:
		slog.InfoContext(ctx, "Received signal, shutting down gracefully", "signal", sig)
	case err := <-serverErrCh:
		slog.ErrorContext(ctx, "Server error", "error", err)
	}

	// Graceful shutdown
	if adminServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := adminServer.Stop(shutdownCtx); err != nil {
			slog.ErrorContext(ctx, "Error stopping admin server", "error", err)
		}
	}

	if xdsClient != nil {
		slog.InfoContext(ctx, "Stopping xDS client")
		xdsClient.Stop()
		xdsClient.Wait()
	}

	grpcServer.GracefulStop()
	slog.InfoContext(ctx, "Policy Engine shut down successfully")
}

// applyFlagOverrides applies command-line flag overrides to the configuration
func applyFlagOverrides(cfg *config.Config) {
	// If policy-chains-file is provided, switch to file mode
	if *policyChainsFile != "" {
		cfg.ConfigMode.Mode = "file"
		cfg.FileConfig.Path = *policyChainsFile
		cfg.XDS.Enabled = false
	}

	// Override xDS server address if provided
	if *xdsServerAddr != "" {
		cfg.XDS.ServerAddress = *xdsServerAddr
	}

	// Override xDS node ID if provided
	if *xdsNodeID != "" {
		cfg.XDS.NodeID = *xdsNodeID
	}
}

// setupLogger creates a logger based on configuration
func setupLogger(cfg *config.Config) *slog.Logger {
	var level slog.Level
	switch cfg.Logging.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if cfg.Logging.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

// initializeXDSClient initializes and starts the xDS client
func initializeXDSClient(ctx context.Context, cfg *config.Config, k *kernel.Kernel, reg *registry.PolicyRegistry) (*xdsclient.Client, error) {
	slog.InfoContext(ctx, "Initializing xDS client",
		"server", cfg.XDS.ServerAddress,
		"node_id", cfg.XDS.NodeID,
		"cluster", cfg.XDS.Cluster)

	xdsConfig := &xdsclient.Config{
		ServerAddress:         cfg.XDS.ServerAddress,
		NodeID:                cfg.XDS.NodeID,
		Cluster:               cfg.XDS.Cluster,
		ConnectTimeout:        cfg.XDS.ConnectTimeout,
		RequestTimeout:        cfg.XDS.RequestTimeout,
		InitialReconnectDelay: cfg.XDS.InitialReconnectDelay,
		MaxReconnectDelay:     cfg.XDS.MaxReconnectDelay,
		TLSEnabled:            cfg.XDS.TLS.Enabled,
		TLSCertPath:           cfg.XDS.TLS.CertPath,
		TLSKeyPath:            cfg.XDS.TLS.KeyPath,
		TLSCAPath:             cfg.XDS.TLS.CAPath,
	}

	client, err := xdsclient.NewClient(xdsConfig, k, reg)
	if err != nil {
		return nil, fmt.Errorf("failed to create xDS client: %w", err)
	}

	if err := client.Start(); err != nil {
		return nil, fmt.Errorf("failed to start xDS client: %w", err)
	}

	return client, nil
}

// initializeFileConfig loads policy chains from a static YAML file
func initializeFileConfig(ctx context.Context, cfg *config.Config, k *kernel.Kernel, reg *registry.PolicyRegistry) error {
	slog.InfoContext(ctx, "Loading file-based configuration", "path", cfg.FileConfig.Path)

	configLoader := kernel.NewConfigLoader(k, reg)
	if err := configLoader.LoadFromFile(cfg.FileConfig.Path); err != nil {
		return fmt.Errorf("failed to load configuration from file: %w", err)
	}

	return nil
}
