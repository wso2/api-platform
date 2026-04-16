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

// WebSubReceiver is a multi-channel WebSub receiver.
// It owns the topic registry, subscription store, delivery engine,
// consumer manager, and the sync producer for subscription state.
type WebSubReceiver struct {
	hubHandler     *HubHandler
	webhookHandler *WebhookReceiverHandler
	deliverer      *Deliverer
	topics         *TopicRegistry
	store          subscription.SubscriptionStore
	consumerMgr    *ConsumerManager
	syncProducer   *subscription.SyncProducer
	brokerDriver   connectors.BrokerDriver
	channel        connectors.ChannelInfo
	opts           Options
}

// NewReceiver creates a WebSub receiver supporting multiple channels (topics).
// It registers hub and webhook-receiver handlers on the shared HTTP mux provided in cfg.
func NewReceiver(cfg connectors.ReceiverConfig, opts Options) (connectors.Receiver, error) {
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
	if len(opts.Brokers) > 0 && cfg.Channel.InternalSubTopic != "" {
		var err error
		syncProducer, err = subscription.NewSyncProducer(opts.Brokers, opts.RuntimeID, cfg.Channel.InternalSubTopic)
		if err != nil {
			slog.Warn("Failed to create sync producer, subscription sync disabled", "error", err)
		}
	}

	verificationTimeout := time.Duration(opts.VerificationTimeoutSeconds) * time.Second

	// Create HubHandler for subscribe/unsubscribe on {context}/{version}/hub.
	hubHandler := NewHubHandler(
		topics, store, verificationTimeout, opts.DefaultLeaseSeconds,
		cfg.Processor, cfg.BrokerDriver, cfg.Channel.Name,
		cfg.Channel.Channels, consumerMgr, syncProducer,
	)

	// Create WebhookReceiverHandler for ingress on {context}/{version}/webhook-receiver.
	webhookHandler := NewWebhookReceiverHandler(
		topics, cfg.Processor, cfg.BrokerDriver,
		cfg.Channel.Name, cfg.Channel.Channels,
	)

	// Register handlers on shared mux.
	basePath := cfg.Channel.Context + "/" + cfg.Channel.Version
	cfg.Mux.Handle(basePath+"/hub", hubHandler)
	cfg.Mux.Handle(basePath+"/webhook-receiver", webhookHandler)

	return &WebSubReceiver{
		hubHandler:     hubHandler,
		webhookHandler: webhookHandler,
		deliverer:      deliverer,
		topics:         topics,
		store:          store,
		consumerMgr:    consumerMgr,
		syncProducer:   syncProducer,
		brokerDriver:   cfg.BrokerDriver,
		channel:        cfg.Channel,
		opts:           opts,
	}, nil
}

// Start ensures Kafka topics exist and sets up the consumer manager context.
// The HTTP server is managed by the runtime.
func (e *WebSubReceiver) Start(ctx context.Context) error {
	// Collect all Kafka topics to ensure.
	var topicsToEnsure []string
	for _, kafkaTopic := range e.channel.Channels {
		topicsToEnsure = append(topicsToEnsure, kafkaTopic)
	}
	if e.channel.InternalSubTopic != "" {
		topicsToEnsure = append(topicsToEnsure, e.channel.InternalSubTopic)
	}

	if len(topicsToEnsure) > 0 {
		if err := e.brokerDriver.EnsureTopics(ctx, topicsToEnsure); err != nil {
			return fmt.Errorf("failed to ensure kafka topics: %w", err)
		}
	}

	e.consumerMgr.SetContext(ctx)

	// Ensure the subscription sync topic exists before producing or reconciling.
	if e.syncProducer != nil {
		if err := e.syncProducer.EnsureSyncTopic(ctx); err != nil {
			slog.Warn("Failed to ensure subscription sync topic", "error", err)
		}
	}

	// Reconcile subscriptions from the Kafka sync topic so that existing
	// subscriptions survive a binding update (remove + re-add).
	e.reconcileSubscriptions(ctx)

	slog.Info("WebSub receiver started",
		"api", e.channel.Name,
		"channels", len(e.channel.Channels),
		"context", e.channel.Context,
		"version", e.channel.Version,
	)
	return nil
}

// Stop stops all per-callback consumers and the sync producer.
func (e *WebSubReceiver) Stop(ctx context.Context) error {
	e.consumerMgr.StopAll(ctx)

	if e.syncProducer != nil {
		e.syncProducer.Close()
	}

	return nil
}

// reconcileSubscriptions replays subscriptions from the Kafka sync topic and
// restores any active subscriptions whose channel names belong to this receiver.
func (e *WebSubReceiver) reconcileSubscriptions(ctx context.Context) {
	if len(e.opts.Brokers) == 0 {
		return
	}

	reconciler := subscription.NewReconciler(e.opts.Brokers, e.store, e.opts.RuntimeID, e.channel.InternalSubTopic)

	// Build a set of channel names this receiver owns.
	ownedChannels := make(map[string]bool, len(e.channel.Channels))
	for channelName := range e.channel.Channels {
		ownedChannels[channelName] = true
	}

	// Callback: when a subscription is replayed, start its per-callback consumer
	// if the subscription's topic (channel name) belongs to this receiver.
	reconciler.SetCallback(func(sub *subscription.Subscription, isTombstone bool) {
		if isTombstone {
			return
		}
		if sub.State != subscription.StateActive {
			return
		}
		if !ownedChannels[sub.Topic] {
			return // subscription belongs to a different API
		}
		kafkaTopic, ok := e.channel.Channels[sub.Topic]
		if !ok {
			return
		}
		if err := e.consumerMgr.AddSubscription(sub.CallbackURL, sub.Secret, kafkaTopic); err != nil {
			slog.Error("Failed to restore consumer during reconciliation",
				"callback", sub.CallbackURL,
				"topic", sub.Topic,
				"error", err)
		}
	})

	if err := reconciler.Reconcile(ctx); err != nil {
		slog.Warn("Subscription reconciliation failed (non-fatal)", "api", e.channel.Name, "error", err)
	}
}
