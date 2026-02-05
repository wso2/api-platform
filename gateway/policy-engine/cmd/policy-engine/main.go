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
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"

	"github.com/wso2/api-platform/gateway/policy-engine/internal/admin"
	"github.com/wso2/api-platform/gateway/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/policy-engine/internal/kernel"
	"github.com/wso2/api-platform/gateway/policy-engine/internal/metrics"
	"github.com/wso2/api-platform/gateway/policy-engine/internal/pkg/cel"
	"github.com/wso2/api-platform/gateway/policy-engine/internal/registry"
	"github.com/wso2/api-platform/gateway/policy-engine/internal/tracing"
	"github.com/wso2/api-platform/gateway/policy-engine/internal/utils"
	"github.com/wso2/api-platform/gateway/policy-engine/internal/xdsclient"
)

// Version information (set via ldflags during build)
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

var (
	configFile       = flag.String("config", "", "Path to configuration file (required)")
	policyChainsFile = flag.String("policy-chains-file", "", "Path to policy chains file (enables file mode)")
	xdsServerAddr    = flag.String("xds-server", "", "xDS server address (e.g., localhost:18000)")
	xdsNodeID        = flag.String("xds-node-id", "", "xDS node identifier")
)

func main() {
	flag.Parse()

	// Validate that config file is provided
	if *configFile == "" {
		fmt.Fprintf(os.Stderr, "Error: -config flag is required\n")
		fmt.Fprintf(os.Stderr, "Usage: %s -config <path-to-config.toml>\n", os.Args[0])
		os.Exit(1)
	}

	// Load configuration from file
	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration from %s: %v\n", *configFile, err)
		os.Exit(1)
	}

	// Initialize metrics based on configuration
	// This must be done before any metrics are used to ensure no-op behavior when disabled
	metrics.SetEnabled(cfg.PolicyEngine.Metrics.Enabled)
	metrics.Init() // Initialize metrics immediately so they're available throughout the codebase

	// Apply flag overrides
	applyFlagOverrides(cfg)

	// Set up structured logging based on configuration
	logger := setupLogger(cfg)
	slog.SetDefault(logger)
	ctx := context.Background()

	// Log startup info based on listen mode
	if cfg.PolicyEngine.Server.ExtProcSocket != "" {
		slog.InfoContext(ctx, "Policy Engine starting",
			"version", Version,
			"git_commit", GitCommit,
			"build_date", BuildDate,
			"config_file", *configFile,
			"config_mode", cfg.PolicyEngine.ConfigMode.Mode,
			"extproc_socket", cfg.PolicyEngine.Server.ExtProcSocket)
	} else {
		slog.InfoContext(ctx, "Policy Engine starting",
			"version", Version,
			"git_commit", GitCommit,
			"build_date", BuildDate,
			"config_file", *configFile,
			"config_mode", cfg.PolicyEngine.ConfigMode.Mode,
			"extproc_port", cfg.PolicyEngine.Server.ExtProcPort)
	}

	// Initialize tracing (if enabled in config)
	tracingShutdown, err := tracing.InitTracer(cfg)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to initialize tracer", "error", err)
		os.Exit(1)
	}
	defer tracingShutdown()

	// Initialize core components
	k := kernel.NewKernel()
	reg := registry.GetRegistry()

	// Set config in registry for ${config} CEL resolution
	if err := reg.SetConfig(cfg.PolicyEngine.RawConfig); err != nil {
		slog.ErrorContext(ctx, "Failed to set config in registry", "error", err)
		os.Exit(1)
	}
	slog.InfoContext(ctx, "Config set in registry for ${config} CEL resolution")

	// Initialize CEL evaluator
	celEvaluator, err := cel.NewCELEvaluator()
	if err != nil {
		slog.ErrorContext(ctx, "Failed to create CEL evaluator", "error", err)
		os.Exit(1)
	}

	// Get tracer for chain executor - will be NoOp if tracing is disabled
	serviceName := cfg.PolicyEngine.TracingServiceName
	if serviceName == "" {
		serviceName = "policy-engine"
	}

	// Initialize chain executor
	chainExecutor := executor.NewChainExecutor(reg, celEvaluator, otel.Tracer(serviceName))

	// Policy registration happens automatically via Builder-generated plugin_registry.go
	slog.InfoContext(ctx, "Policies registered via Builder-generated code")

	// Initialize configuration source based on mode
	var xdsClient *xdsclient.Client
	switch cfg.PolicyEngine.ConfigMode.Mode {
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
		slog.ErrorContext(ctx, "Invalid config mode", "mode", cfg.PolicyEngine.ConfigMode.Mode)
		os.Exit(1)
	}

	// Create and start ext_proc gRPC server
	extprocServer := kernel.NewExternalProcessorServer(k, chainExecutor, cfg.TracingConfig, cfg.PolicyEngine.TracingServiceName)

	var lis net.Listener
	if cfg.PolicyEngine.Server.ExtProcSocket != "" {
		// UDS mode - cleanup stale socket file
		socketPath := cfg.PolicyEngine.Server.ExtProcSocket
		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			slog.WarnContext(ctx, "Failed to remove existing socket file", "path", socketPath, "error", err)
		}

		lis, err = net.Listen("unix", socketPath)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to listen on Unix socket", "path", socketPath, "error", err)
			os.Exit(1)
		}

		// Set socket permissions (readable/writable by owner and group)
		if err := os.Chmod(socketPath, 0660); err != nil {
			slog.WarnContext(ctx, "Failed to set socket permissions", "path", socketPath, "error", err)
		}

		slog.InfoContext(ctx, "Policy Engine listening on Unix socket", "path", socketPath)
	} else {
		// TCP mode
		lis, err = net.Listen("tcp", fmt.Sprintf(":%d", cfg.PolicyEngine.Server.ExtProcPort))
		if err != nil {
			slog.ErrorContext(ctx, "Failed to listen on port", "port", cfg.PolicyEngine.Server.ExtProcPort, "error", err)
			os.Exit(1)
		}

		slog.InfoContext(ctx, "Policy Engine listening on TCP port", "port", cfg.PolicyEngine.Server.ExtProcPort)
	}

	grpcServer := grpc.NewServer()
	extprocv3.RegisterExternalProcessorServer(grpcServer, extprocServer)

	// Start admin HTTP server if enabled
	var adminServer *admin.Server
	if cfg.PolicyEngine.Admin.Enabled {
		adminServer = admin.NewServer(&cfg.PolicyEngine.Admin, k, reg)
		go func() {
			if err := adminServer.Start(ctx); err != nil {
				slog.ErrorContext(ctx, "Admin server error", "error", err)
			}
		}()
	}

	// Start metrics HTTP server if enabled
	var metricsServer *metrics.Server
	if cfg.PolicyEngine.Metrics.Enabled {
		metricsServer = metrics.NewServer(&cfg.PolicyEngine.Metrics)
		go func() {
			if err := metricsServer.Start(ctx); err != nil {
				slog.ErrorContext(ctx, "Metrics server error", "error", err)
			}
		}()
		// Start periodic memory metrics updater
		metrics.StartMemoryMetricsUpdater(ctx, 15*time.Second)
	}

	// Start access log service server if enabled
	var alsServer *grpc.Server
	slog.DebugContext(ctx, "Policy engine ALS server config", "config", cfg.Analytics.AccessLogsServiceCfg)
	if cfg.Analytics.Enabled {
		// Start the access log service server
		slog.Info("Starting the ALS gRPC server...")
		alsServer = utils.StartAccessLogServiceServer(cfg)
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

	if metricsServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := metricsServer.Stop(shutdownCtx); err != nil {
			slog.ErrorContext(ctx, "Error stopping metrics server", "error", err)
		}
	}

	if xdsClient != nil {
		slog.InfoContext(ctx, "Stopping xDS client")
		xdsClient.Stop()
		xdsClient.Wait()
	}

	if alsServer != nil {
		slog.InfoContext(ctx, "Stopping ALS gRPC server")
		alsServer.GracefulStop()
	}

	grpcServer.GracefulStop()

	// Cleanup Unix socket if used
	if cfg.PolicyEngine.Server.ExtProcSocket != "" {
		if err := os.Remove(cfg.PolicyEngine.Server.ExtProcSocket); err != nil && !os.IsNotExist(err) {
			slog.WarnContext(ctx, "Failed to cleanup socket file on shutdown",
				"path", cfg.PolicyEngine.Server.ExtProcSocket, "error", err)
		}
	}

	slog.InfoContext(ctx, "Policy Engine shut down successfully")
}

// applyFlagOverrides applies command-line flag overrides to the configuration
func applyFlagOverrides(cfg *config.Config) {
	// If policy-chains-file is provided, switch to file mode
	if *policyChainsFile != "" {
		cfg.PolicyEngine.ConfigMode.Mode = "file"
		cfg.PolicyEngine.FileConfig.Path = *policyChainsFile
		cfg.PolicyEngine.XDS.Enabled = false
	}

	// Override xDS server address if provided
	if *xdsServerAddr != "" {
		cfg.PolicyEngine.XDS.ServerAddress = *xdsServerAddr
	}

	// Override xDS node ID if provided
	if *xdsNodeID != "" {
		cfg.PolicyEngine.XDS.NodeID = *xdsNodeID
	}
}

// setupLogger creates a logger based on configuration
func setupLogger(cfg *config.Config) *slog.Logger {
	var level slog.Level
	switch cfg.PolicyEngine.Logging.Level {
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
	if cfg.PolicyEngine.Logging.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

// initializeXDSClient initializes and starts the xDS client
func initializeXDSClient(ctx context.Context, cfg *config.Config, k *kernel.Kernel, reg *registry.PolicyRegistry) (*xdsclient.Client, error) {
	slog.InfoContext(ctx, "Initializing xDS client",
		"server", cfg.PolicyEngine.XDS.ServerAddress,
		"node_id", cfg.PolicyEngine.XDS.NodeID,
		"cluster", cfg.PolicyEngine.XDS.Cluster)

	xdsConfig := &xdsclient.Config{
		ServerAddress:         cfg.PolicyEngine.XDS.ServerAddress,
		NodeID:                cfg.PolicyEngine.XDS.NodeID,
		Cluster:               cfg.PolicyEngine.XDS.Cluster,
		ConnectTimeout:        cfg.PolicyEngine.XDS.ConnectTimeout,
		RequestTimeout:        cfg.PolicyEngine.XDS.RequestTimeout,
		InitialReconnectDelay: cfg.PolicyEngine.XDS.InitialReconnectDelay,
		MaxReconnectDelay:     cfg.PolicyEngine.XDS.MaxReconnectDelay,
		TLSEnabled:            cfg.PolicyEngine.XDS.TLS.Enabled,
		TLSCertPath:           cfg.PolicyEngine.XDS.TLS.CertPath,
		TLSKeyPath:            cfg.PolicyEngine.XDS.TLS.KeyPath,
		TLSCAPath:             cfg.PolicyEngine.XDS.TLS.CAPath,
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
	slog.InfoContext(ctx, "Loading file-based configuration", "path", cfg.PolicyEngine.FileConfig.Path)

	configLoader := kernel.NewConfigLoader(k, reg)
	if err := configLoader.LoadFromFile(cfg.PolicyEngine.FileConfig.Path); err != nil {
		return fmt.Errorf("failed to load configuration from file: %w", err)
	}

	return nil
}
