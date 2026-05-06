/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *......
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
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"
	"github.com/wso2/api-platform/common/apikey"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/config"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/runtime"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/xdsclient"
)

func main() {
	configPath := flag.String("config", "configs/config.toml", "Path to TOML config file")
	channelsPath := flag.String("channels", "configs/channels.yaml", "Path to channels YAML file")
	flag.Parse()

	cfg, rawConfig, err := config.Load(*configPath)
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}
	slog.SetDefault(setupLogger(cfg))

	if cfg.RuntimeID == "" {
		cfg.RuntimeID = uuid.New().String()
	}

	// Build connector registry — all connector types are registered here.
	registry := connectors.NewRegistry()
	registerConnectors(registry, cfg)

	// Create the runtime — owns engine, hub, admin.
	rt, err := runtime.New(cfg, rawConfig, registry)
	if err != nil {
		slog.Error("Failed to create runtime", "error", err)
		os.Exit(1)
	}

	// Register compiled-in policies with the engine.
	registerPolicies(rt.Engine())

	// Decide startup mode: xDS control plane or static channels.yaml.
	if cfg.ControlPlane.Enabled {
		slog.Info("Control plane mode enabled, starting xDS client",
			"xds_address", cfg.ControlPlane.XDSAddress)

		handler := xdsclient.NewHandler(rt, xdsclient.KafkaConfig{
			Brokers: cfg.Kafka.Brokers,
		})
		eventConfigClient := xdsclient.NewClient(
			cfg.ControlPlane.XDSAddress,
			cfg.ControlPlane.NodeID,
			xdsclient.EventChannelConfigTypeURL,
			handler.HandleResources,
		)
		apiKeyHandler := xdsclient.NewAPIKeyStateHandler(apikey.GetAPIkeyStoreInstance())
		apiKeyClient := xdsclient.NewClient(
			cfg.ControlPlane.XDSAddress,
			cfg.ControlPlane.NodeID,
			xdsclient.APIKeyStateTypeURL,
			apiKeyHandler.HandleResources,
		)

		eventConfigClient.Start()
		apiKeyClient.Start()
		defer eventConfigClient.Stop()
		defer apiKeyClient.Stop()
	} else {
		// Static mode: parse channel bindings from YAML.
		if err := rt.LoadChannels(*channelsPath); err != nil {
			slog.Error("Failed to load channels", "error", err)
			os.Exit(1)
		}
	}

	// Run until SIGTERM/SIGINT.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	if err := rt.Run(ctx); err != nil {
		slog.Error("Runtime error", "error", err)
		os.Exit(1)
	}
}

func setupLogger(cfg *config.Config) *slog.Logger {
	var level slog.Level
	switch cfg.Logging.Level {
	case "debug":
		level = slog.LevelDebug
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
