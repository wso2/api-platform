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
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/admin"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/binding"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/config"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors/endpoint/kafka"
	wsconn "github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors/entrypoint/websocket"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors/entrypoint/websub"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/hub"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/subscription"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/pkg/engine"
)

func main() {
	configPath := flag.String("config", "configs/config.toml", "Path to TOML config file")
	channelsPath := flag.String("channels", "configs/channels.yaml", "Path to channels YAML file")
	flag.Parse()

	// Load configuration
	cfg, rawConfig, err := config.Load(*configPath)
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	if cfg.RuntimeID == "" {
		cfg.RuntimeID = uuid.New().String()
	}

	// Start admin server first
	adminServer := admin.NewServer(cfg.Server.AdminPort)
	adminServer.Start()

	// Create policy engine
	eng, err := engine.New(rawConfig)
	if err != nil {
		slog.Error("Failed to create policy engine", "error", err)
		os.Exit(1)
	}

	// Register policies (via plugins.go blank imports which call init())
	registerPolicies(eng)

	// Load policy chains if chains file is specified
	if cfg.PolicyEngine.ChainsFile != "" {
		if err := eng.LoadChainsFromFile(cfg.PolicyEngine.ChainsFile); err != nil {
			slog.Error("Failed to load policy chains", "error", err)
			os.Exit(1)
		}
	}

	// Create hub
	h := hub.NewHub(eng)

	// Load channel bindings
	if err := binding.LoadChannels(*channelsPath, eng, h); err != nil {
		slog.Error("Failed to load channel bindings", "error", err)
		os.Exit(1)
	}

	// Create subscription store
	subStore := subscription.NewInMemoryStore(cfg.RuntimeID)

	// Create topic registry and register topics from bindings
	topicRegistry := websub.NewTopicRegistry()
	for _, b := range h.AllBindings() {
		if b.Mode == "websub" {
			topicRegistry.Register(b.EndpointTopic)
		}
	}

	// Create Kafka publisher
	kafkaPublisher, err := kafka.NewPublisher(cfg.Kafka.Brokers)
	if err != nil {
		slog.Error("Failed to create Kafka publisher", "error", err)
		os.Exit(1)
	}
	defer kafkaPublisher.Close()

	// Create WebSub handler
	verificationTimeout := time.Duration(cfg.WebSub.VerificationTimeoutSeconds) * time.Second
	websubHandler := websub.NewHandler(topicRegistry, subStore, h, verificationTimeout, cfg.WebSub.DefaultLeaseSeconds)

	// Create WebSub deliverer
	deliverer := websub.NewDeliverer(subStore, h, websub.DeliveryConfig{
		MaxRetries:     cfg.WebSub.DeliveryMaxRetries,
		InitialDelayMs: cfg.WebSub.DeliveryInitialDelayMs,
		MaxDelayMs:     cfg.WebSub.DeliveryMaxDelayMs,
		Concurrency:    cfg.WebSub.DeliveryConcurrency,
	})

	// Create WebSocket server
	wsConfig := wsconn.DefaultServerConfig()
	wsServer := wsconn.NewServer(wsConfig, func(ctx context.Context, msg *connectors.Message) error {
		// WebSocket inbound: find binding by topic/channel and process through hub
		b := h.GetBindingByTopic(msg.Topic)
		if b == nil {
			return fmt.Errorf("no binding for channel: %s", msg.Topic)
		}

		processed, shortCircuited, err := h.ProcessInbound(ctx, b.Name, msg)
		if err != nil {
			return err
		}
		if shortCircuited {
			return nil
		}

		return kafkaPublisher.Publish(ctx, b.EndpointTopic, processed)
	})

	// Start Kafka consumers for topics with WebSub bindings
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var kafkaConsumers []*kafka.Consumer
	websubTopics := collectWebSubTopics(h)
	if len(websubTopics) > 0 {
		groupID := cfg.Kafka.ConsumerGroupPrefix + "-websub"
		consumer, err := kafka.NewConsumer(cfg.Kafka.Brokers, groupID, websubTopics, func(ctx context.Context, msg *connectors.Message) error {
			b := h.GetBindingByTopic(msg.Topic)
			if b == nil {
				return nil
			}
			return deliverer.DeliverToSubscribers(ctx, b.Name, msg)
		})
		if err != nil {
			slog.Error("Failed to create Kafka consumer for WebSub", "error", err)
			os.Exit(1)
		}
		consumer.Start(ctx)
		kafkaConsumers = append(kafkaConsumers, consumer)
	}

	// Start Kafka consumers for WebSocket bindings
	wsTopics := collectProtocolMediationTopics(h)
	if len(wsTopics) > 0 {
		groupID := cfg.Kafka.ConsumerGroupPrefix + "-ws"
		consumer, err := kafka.NewConsumer(cfg.Kafka.Brokers, groupID, wsTopics, func(ctx context.Context, msg *connectors.Message) error {
			b := h.GetBindingByTopic(msg.Topic)
			if b == nil {
				return nil
			}

			processed, shortCircuited, err := h.ProcessOutbound(ctx, b.Name, msg)
			if err != nil {
				return err
			}
			if shortCircuited {
				return nil
			}

			wsServer.Broadcast(b.Name, processed.Value)
			return nil
		})
		if err != nil {
			slog.Error("Failed to create Kafka consumer for WebSocket", "error", err)
			os.Exit(1)
		}
		consumer.Start(ctx)
		kafkaConsumers = append(kafkaConsumers, consumer)
	}

	// Start HTTP servers
	websubMux := http.NewServeMux()
	websubMux.Handle("/hub", websubHandler)
	websubServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.WebSubPort),
		Handler: websubMux,
	}

	wsMux := http.NewServeMux()
	wsMux.HandleFunc("/", wsServer.HandleUpgrade)
	wsHTTPServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.WebSocketPort),
		Handler: wsMux,
	}

	go func() {
		slog.Info("WebSub server starting", "port", cfg.Server.WebSubPort)
		if err := websubServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("WebSub server error", "error", err)
		}
	}()

	go func() {
		slog.Info("WebSocket server starting", "port", cfg.Server.WebSocketPort)
		if err := wsHTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("WebSocket server error", "error", err)
		}
	}()

	// Mark as ready
	adminServer.SetReady(true)
	slog.Info("Event gateway is ready",
		"runtime_id", cfg.RuntimeID,
		"websub_port", cfg.Server.WebSubPort,
		"websocket_port", cfg.Server.WebSocketPort,
	)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	<-sigChan

	slog.Info("Shutting down event gateway...")
	adminServer.SetReady(false)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	cancel() // Cancel main context

	// Stop consumers
	for _, c := range kafkaConsumers {
		c.Stop()
	}

	// Wait for pending deliveries
	deliverer.Wait()

	// Shutdown HTTP servers
	websubServer.Shutdown(shutdownCtx)
	wsHTTPServer.Shutdown(shutdownCtx)

	// Close WebSocket connections
	wsServer.CloseAll()

	// Stop admin server
	adminServer.Stop(shutdownCtx)

	slog.Info("Event gateway shutdown complete")
}

func collectWebSubTopics(h *hub.Hub) []string {
	var topics []string
	for _, b := range h.AllBindings() {
		if b.Mode == "websub" && b.EndpointTopic != "" {
			topics = append(topics, b.EndpointTopic)
		}
	}
	return topics
}

func collectProtocolMediationTopics(h *hub.Hub) []string {
	var topics []string
	for _, b := range h.AllBindings() {
		if b.Mode == "protocol-mediation" && b.EndpointTopic != "" {
			topics = append(topics, b.EndpointTopic)
		}
	}
	return topics
}
