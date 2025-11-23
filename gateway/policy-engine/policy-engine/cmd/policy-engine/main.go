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

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"google.golang.org/grpc"

	"github.com/policy-engine/policy-engine/internal/executor"
	"github.com/policy-engine/policy-engine/internal/kernel"
	"github.com/policy-engine/policy-engine/internal/pkg/cel"
	"github.com/policy-engine/policy-engine/internal/registry"
)

// T091: Command-line flags implementation
var (
	extprocPort = flag.Int("extproc-port", 9001, "Port for ext_proc gRPC server")
	xdsPort     = flag.Int("xds-port", 9002, "Port for xDS policy discovery server (future)")
	configFile  = flag.String("config-file", "configs/policy-chains.yaml", "Path to policy chain configuration file")
)

// T090: Main entry point with gRPC server initialization
func main() {
	flag.Parse()

	// Set up structured logging with slog
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)
	ctx := context.Background()

	slog.InfoContext(ctx, "Policy Engine starting")
	slog.InfoContext(ctx, "Configuration", "extproc_port", *extprocPort, "config_file", *configFile)

	// Initialize components
	// T093: Wire Kernel and Core components
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
	// The generated code in plugin_registry.go imports all compiled policies and registers
	// them with full metadata (name, version, phase support, body requirements) at init time.
	// No runtime policy.yaml reading is needed - all metadata is embedded in the binary.
	slog.InfoContext(ctx, "Policies registered via Builder-generated code")

	// Load policy chain configuration from file
	configLoader := kernel.NewConfigLoader(k, reg)
	if err := configLoader.LoadFromFile(*configFile); err != nil {
		slog.ErrorContext(ctx, "Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Create ext_proc gRPC server
	extprocServer := kernel.NewExternalProcessorServer(k, chainExecutor)

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *extprocPort))
	if err != nil {
		slog.ErrorContext(ctx, "Failed to listen on port", "port", *extprocPort, "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()

	// Register ext_proc service
	extprocv3.RegisterExternalProcessorServer(grpcServer, extprocServer)

	slog.InfoContext(ctx, "Policy Engine listening", "port", *extprocPort)

	// T092: Graceful shutdown handling
	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			slog.ErrorContext(ctx, "Failed to serve", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	sig := <-sigChan
	slog.InfoContext(ctx, "Received signal, shutting down gracefully", "signal", sig)

	// Graceful shutdown
	grpcServer.GracefulStop()
	slog.InfoContext(ctx, "Policy Engine shut down successfully")
}
