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
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/admin"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/kernel"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/pkg/cel"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/pythonbridge"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/resolver"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/tracing"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/utils"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/xdsclient"
)

// Version information (set via ldflags during build)
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// stringSliceFlag collects a repeatable string flag into a slice, preserving the
// order in which the flags were supplied on the command line.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string { return strings.Join(*s, ", ") }

func (s *stringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

var (
	configFiles      stringSliceFlag
	policyChainsFile = flag.String("policy-chains-file", "", "Path to policy chains file (enables file mode)")
	xdsServerAddr    = flag.String("xds-server", "", "xDS server address (e.g., localhost:18000)")
)

func init() {
	flag.Var(&configFiles, "config",
		"Path to a configuration file (required; repeatable, merged in order with last-wins precedence)")
}

type noOpXDSSyncStatusProvider struct{}

func (noOpXDSSyncStatusProvider) GetPolicyChainVersion() string {
	return ""
}

type alwaysHealthyProvider struct{}

func (alwaysHealthyProvider) IsHealthy() bool {
	return true
}

func main() {
	flag.Parse()

	// Validate that at least one config file is provided
	if len(configFiles) == 0 {
		fmt.Fprintf(os.Stderr, "Error: -config flag is required\n")
		fmt.Fprintf(os.Stderr, "Usage: %s -config <path-to-config.toml> [-config <overlay.toml> ...]\n", os.Args[0])
		os.Exit(1)
	}

	// Load configuration from the file(s), merged in order with last-wins precedence
	cfg, err := config.Load(configFiles...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration from %s: %v\n", strings.Join(configFiles, ", "), err)
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
	serverMode := cfg.PolicyEngine.Server.Mode
	if serverMode == "" {
		serverMode = "uds" // Default to UDS
	}
	if serverMode == "uds" {
		slog.InfoContext(ctx, "Policy Engine starting",
			"version", Version,
			"git_commit", GitCommit,
			"build_date", BuildDate,
			"config_files", []string(configFiles),
			"config_mode", cfg.PolicyEngine.ConfigMode.Mode,
			"server_mode", serverMode,
			"extproc_socket", constants.DefaultPolicyEngineSocketPath)
	} else {
		slog.InfoContext(ctx, "Policy Engine starting",
			"version", Version,
			"git_commit", GitCommit,
			"build_date", BuildDate,
			"config_files", []string(configFiles),
			"config_mode", cfg.PolicyEngine.ConfigMode.Mode,
			"server_mode", serverMode,
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

	// Freeze the operation-resolver registry before anything reads it: no resolver
	// can be registered once the kernel and the xDS client are running, so what the
	// runtime advertises to the control plane and what it can actually serve are the
	// same set for the whole process lifetime.
	resolvers := resolver.DefaultRegistry()
	slog.InfoContext(ctx, "Operation resolvers registered",
		"resolvers", resolvers.Names(), "protocol_version", resolver.ProtocolVersion)

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

	// Initialize Python executor bridge from configuration
	pythonbridge.Init(cfg.PolicyEngine.PythonExecutor)

	// Initialize configuration source based on mode
	var xdsClient *xdsclient.Client
	var xdsSyncStatusProvider admin.XDSSyncStatusProvider = noOpXDSSyncStatusProvider{}
	var healthProvider admin.HealthProvider = alwaysHealthyProvider{}
	switch cfg.PolicyEngine.ConfigMode.Mode {
	case "xds":
		if *xdsServerAddr == "" {
			slog.ErrorContext(ctx, "Error: -xds-server flag is required when config mode is 'xds'")
			os.Exit(1)
		}
		xdsClient, err = initializeXDSClient(ctx, cfg, *xdsServerAddr, k, reg, resolvers)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to initialize xDS client", "error", err)
			os.Exit(1)
		}
		xdsSyncStatusProvider = xdsClient
		healthProvider = xdsClient
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
	extprocServer := kernel.NewExternalProcessorServer(k, chainExecutor, cfg.TracingConfig, cfg.PolicyEngine.TracingServiceName, cfg.PolicyEngine.RequestBody.MaxDecompressedBytes, cfg.PolicyEngine.ResponseBody.MaxDecompressedBytes)

	// Create listener based on mode (same pattern as gateway-controller)
	var lis net.Listener
	switch serverMode {
	case "uds":
		// UDS mode (default) - use constant socket path
		socketPath := constants.DefaultPolicyEngineSocketPath
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
	case "tcp":
		// TCP mode
		lis, err = net.Listen("tcp", fmt.Sprintf(":%d", cfg.PolicyEngine.Server.ExtProcPort))
		if err != nil {
			slog.ErrorContext(ctx, "Failed to listen on port", "port", cfg.PolicyEngine.Server.ExtProcPort, "error", err)
			os.Exit(1)
		}

		slog.InfoContext(ctx, "Policy Engine listening on TCP port", "port", cfg.PolicyEngine.Server.ExtProcPort)
	}

	grpcServer := grpc.NewServer(extProcServerOptions(cfg)...)
	extprocv3.RegisterExternalProcessorServer(grpcServer, extprocServer)

	// Enable block/mutex profiling sampling when pprof is enabled. These are the
	// only profiles that need explicit rate setup; 0 leaves them disabled. Gated so
	// the sampling overhead is never paid unless pprof is deliberately turned on.
	if cfg.PolicyEngine.Admin.Pprof.Enabled {
		runtime.SetBlockProfileRate(cfg.PolicyEngine.Admin.Pprof.BlockProfileRate)
		runtime.SetMutexProfileFraction(cfg.PolicyEngine.Admin.Pprof.MutexProfileFraction)
	}

	// Start admin HTTP server if enabled
	var adminServer *admin.Server
	if cfg.PolicyEngine.Admin.Enabled {
		var pythonHealthChecker admin.PythonHealthChecker
		if pythonbridge.IsAvailable(cfg.PolicyEngine.PythonExecutor) {
			sm := pythonbridge.GetStreamManager()
			pythonHealthChecker = pythonbridge.NewPythonHealthAdapter(sm)
		}
		adminServer = admin.NewServer(&cfg.PolicyEngine.Admin, k, reg, xdsSyncStatusProvider, healthProvider, pythonHealthChecker)
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

	// Start the access log service server when the collector is enabled. The
	// collector is the shared transport that carries collected data to its
	// consumers (analytics, traffic logging).
	var alsServer *grpc.Server
	var alsAnalytics *analytics.Analytics
	slog.DebugContext(ctx, "Policy engine ALS server config", "config", cfg.Collector.Server)
	if cfg.IsCollectorEnabled() {
		// Start the access log service server
		slog.Info("Starting the ALS gRPC server...")
		alsServer, alsAnalytics = utils.StartAccessLogServiceServer(cfg)
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

	// Flush analytics publishers only after the ALS server has stopped, so the
	// flush cannot race newly arriving events. Publishers that buffer (the
	// traffic-log HTTP sink, Moesif) would otherwise lose their in-flight batch on
	// every restart, rolling update and scale-down.
	if alsAnalytics != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(),
			cfg.TrafficLogging.EffectiveShutdownTimeout())
		if err := alsAnalytics.Close(shutdownCtx); err != nil {
			slog.ErrorContext(ctx, "Error flushing analytics publishers", "error", err)
		}
		cancel()
	}

	grpcServer.GracefulStop()

	// Cleanup Unix socket if used (UDS mode)
	if serverMode == "uds" {
		if err := os.Remove(constants.DefaultPolicyEngineSocketPath); err != nil && !os.IsNotExist(err) {
			slog.WarnContext(ctx, "Failed to cleanup socket file on shutdown",
				"path", constants.DefaultPolicyEngineSocketPath, "error", err)
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

	// Tagging is per logger, never process-wide on stdout: the traffic-log
	// publisher writes bare JSON to os.Stdout on the same descriptor, and a
	// process-wide prefix would corrupt it.
	var handler slog.Handler
	if cfg.PolicyEngine.Logging.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
		return slog.New(handler).With(slog.String(logComponentField, logComponentValue))
	}
	handler = slog.NewTextHandler(newComponentPrefixWriter(os.Stdout, logComponentPrefix), opts)

	return slog.New(handler)
}

const (
	logComponentPrefix = "[pol] "
	logComponentField  = "component"
	logComponentValue  = "pol"
)

// componentPrefixWriter prepends a tag to every write. slog emits one complete
// record per Write, so this yields one tag per line. slog serializes writes, so
// no lock is needed here.
type componentPrefixWriter struct {
	w      io.Writer
	prefix []byte
}

func newComponentPrefixWriter(w io.Writer, prefix string) *componentPrefixWriter {
	return &componentPrefixWriter{w: w, prefix: []byte(prefix)}
}

// Write emits the tag and p in one underlying Write so another writer sharing
// the descriptor cannot interleave between them. The returned count excludes
// the tag, which is framing rather than bytes consumed from the caller.
func (c *componentPrefixWriter) Write(p []byte) (int, error) {
	buf := make([]byte, 0, len(c.prefix)+len(p))
	buf = append(buf, c.prefix...)
	buf = append(buf, p...)
	if _, err := c.w.Write(buf); err != nil {
		return 0, err
	}
	return len(p), nil
}

// initializeXDSClient initializes and starts the xDS client
func initializeXDSClient(ctx context.Context, cfg *config.Config, serverAddr string, k *kernel.Kernel, reg *registry.PolicyRegistry, resolvers resolver.ResolverRegistry) (*xdsclient.Client, error) {
	slog.InfoContext(ctx, "Initializing xDS client",
		"server", serverAddr)

	xdsConfig := &xdsclient.Config{
		ServerAddress:         serverAddr,
		ConnectTimeout:        cfg.PolicyEngine.XDS.ConnectTimeout,
		RequestTimeout:        cfg.PolicyEngine.XDS.RequestTimeout,
		InitialReconnectDelay: cfg.PolicyEngine.XDS.InitialReconnectDelay,
		MaxReconnectDelay:     cfg.PolicyEngine.XDS.MaxReconnectDelay,
		TLSEnabled:            cfg.PolicyEngine.XDS.TLS.Enabled,
		TLSCertPath:           cfg.PolicyEngine.XDS.TLS.CertPath,
		TLSKeyPath:            cfg.PolicyEngine.XDS.TLS.KeyPath,
		TLSCAPath:             cfg.PolicyEngine.XDS.TLS.CAPath,
	}

	client, err := xdsclient.NewClient(xdsConfig, k, reg, resolvers)
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

// extProcServerOptions bounds the ext_proc gRPC server explicitly, rather than taking
// gRPC's defaults: the receive default is 4 MiB whatever the body ceilings are configured
// to be, the send default is unbounded, and the concurrent-stream default is effectively
// unlimited. This is the hottest gRPC server in the data plane, so all three are set from
// validated configuration (see config.Config.Validate, which refuses to start when a
// message limit is below what the body ceilings require).
func extProcServerOptions(cfg *config.Config) []grpc.ServerOption {
	server := cfg.PolicyEngine.Server
	slog.Info("ext_proc gRPC server limits",
		"max_recv_msg_bytes", server.MaxRecvMsgBytes,
		"max_send_msg_bytes", server.MaxSendMsgBytes,
		"max_concurrent_streams", server.MaxConcurrentStreams)

	return []grpc.ServerOption{
		grpc.MaxRecvMsgSize(int(server.MaxRecvMsgBytes)),
		grpc.MaxSendMsgSize(int(server.MaxSendMsgBytes)),
		grpc.MaxConcurrentStreams(server.MaxConcurrentStreams),
	}
}
