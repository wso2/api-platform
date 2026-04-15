/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package websub

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/subscription"
)

// Options holds WebSub-specific configuration passed at registration time.
type Options struct {
	Port                       int
	VerificationTimeoutSeconds int
	DefaultLeaseSeconds        int
	DeliveryMaxRetries         int
	DeliveryInitialDelayMs     int
	DeliveryMaxDelayMs         int
	DeliveryConcurrency        int
	RuntimeID                  string
	ConsumerGroupPrefix        string
	Brokers                    []string
}

// WebSubEntrypoint is a multi-channel WebSub entrypoint.
// It owns the topic registry, subscription store, delivery engine,
// consumer manager, and the sync producer for subscription state.
type WebSubEntrypoint struct {
	hubHandler     *HubHandler
	webhookHandler *WebhookReceiverHandler
	deliverer      *Deliverer
	topics         *TopicRegistry
	store          subscription.SubscriptionStore
	consumerMgr    *ConsumerManager
	syncProducer   *subscription.SyncProducer
	endpoint       connectors.Endpoint
	channel        connectors.ChannelInfo
	opts           Options
}

// NewEntrypoint creates a WebSub entrypoint supporting multiple channels (topics).
// It registers hub and webhook-receiver handlers on the shared HTTP mux provided in cfg.
func NewEntrypoint(cfg connectors.EntrypointConfig, opts Options) (connectors.Entrypoint, error) {
	store := subscription.NewInMemoryStore(opts.RuntimeID)
	topics := NewTopicRegistry()

	// Register all channel names in the topic registry.
	if len(cfg.Channel.Channels) > 0 {
		for channelName := range cfg.Channel.Channels {
			topics.Register(channelName)
		}
	} else {
		// Fallback for legacy single-topic mode.
		topics.Register(cfg.Channel.PublicTopic)
	}

	deliverer := NewDeliverer(DeliveryConfig{
		MaxRetries:     opts.DeliveryMaxRetries,
		InitialDelayMs: opts.DeliveryInitialDelayMs,
		MaxDelayMs:     opts.DeliveryMaxDelayMs,
		Concurrency:    opts.DeliveryConcurrency,
	})

	// Create consumer manager for per-callback consumers.
	consumerMgr := NewConsumerManager(
		opts.Brokers,
		opts.ConsumerGroupPrefix,
		cfg.Processor,
		cfg.Channel.Name,
		deliverer,
	)

	// Create sync producer for subscription state.
	var syncProducer *subscription.SyncProducer
	if len(opts.Brokers) > 0 {
		var err error
		syncProducer, err = subscription.NewSyncProducer(opts.Brokers, opts.RuntimeID)
		if err != nil {
			slog.Warn("Failed to create sync producer, subscription sync disabled", "error", err)
		}
	}

	verificationTimeout := time.Duration(opts.VerificationTimeoutSeconds) * time.Second

	// Create HubHandler for subscribe/unsubscribe on {context}/{version}/hub.
	hubHandler := NewHubHandler(
		topics, store, verificationTimeout, opts.DefaultLeaseSeconds,
		cfg.Processor, cfg.Endpoint, cfg.Channel.Name,
		cfg.Channel.Channels, consumerMgr, syncProducer,
	)

	// Create WebhookReceiverHandler for ingress on {context}/{version}/webhook-receiver.
	webhookHandler := NewWebhookReceiverHandler(
		topics, cfg.Processor, cfg.Endpoint,
		cfg.Channel.Name, cfg.Channel.Channels,
	)

	// Register handlers on shared mux.
	basePath := cfg.Channel.Context + "/" + cfg.Channel.Version
	cfg.Mux.Handle(basePath+"/hub", hubHandler)
	cfg.Mux.Handle(basePath+"/webhook-receiver", webhookHandler)

	return &WebSubEntrypoint{
		hubHandler:     hubHandler,
		webhookHandler: webhookHandler,
		deliverer:      deliverer,
		topics:         topics,
		store:          store,
		consumerMgr:    consumerMgr,
		syncProducer:   syncProducer,
		endpoint:       cfg.Endpoint,
		channel:        cfg.Channel,
		opts:           opts,
	}, nil
}

// Start ensures Kafka topics exist and sets up the consumer manager context.
// The HTTP server is managed by the runtime.
func (e *WebSubEntrypoint) Start(ctx context.Context) error {
	// Collect all Kafka topics to ensure.
	var topicsToEnsure []string
	for _, kafkaTopic := range e.channel.Channels {
		topicsToEnsure = append(topicsToEnsure, kafkaTopic)
	}
	if e.channel.InternalSubTopic != "" {
		topicsToEnsure = append(topicsToEnsure, e.channel.InternalSubTopic)
	}

	if len(topicsToEnsure) > 0 {
		if err := e.endpoint.EnsureTopics(ctx, topicsToEnsure); err != nil {
			return fmt.Errorf("failed to ensure kafka topics: %w", err)
		}
	}

	e.consumerMgr.SetContext(ctx)

	slog.Info("WebSub entrypoint started",
		"api", e.channel.Name,
		"channels", len(e.channel.Channels),
		"context", e.channel.Context,
		"version", e.channel.Version,
	)
	return nil
}

// Stop stops all per-callback consumers and the sync producer.
func (e *WebSubEntrypoint) Stop(ctx context.Context) error {
	e.consumerMgr.StopAll(ctx)

	if e.syncProducer != nil {
		e.syncProducer.Close()
	}

	return nil
}
