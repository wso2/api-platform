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
}

// WebSubEntrypoint is a single-channel WebSub entrypoint.
// It owns the topic registry, subscription store, delivery engine,
// and an endpoint consumer for event delivery.
type WebSubEntrypoint struct {
	handler   *Handler
	deliverer *Deliverer
	topics    *TopicRegistry
	store     subscription.SubscriptionStore
	consumer  connectors.Entrypoint
	endpoint  connectors.Endpoint
	channel   connectors.ChannelInfo
	opts      Options
}

// NewEntrypoint creates a WebSub entrypoint for a single channel.
// It registers its hub handler on the shared HTTP mux provided in cfg.
func NewEntrypoint(cfg connectors.EntrypointConfig, opts Options) (connectors.Entrypoint, error) {
	store := subscription.NewInMemoryStore(opts.RuntimeID)
	topics := NewTopicRegistry()
	topics.Register(cfg.Channel.PublicTopic)

	verificationTimeout := time.Duration(opts.VerificationTimeoutSeconds) * time.Second
	handler := NewHandler(topics, store, verificationTimeout, opts.DefaultLeaseSeconds,
		cfg.Processor, cfg.Endpoint, cfg.Channel.Name, cfg.Channel.PublicTopic, cfg.Channel.EndpointTopic)
	deliverer := NewDeliverer(store, cfg.Processor, DeliveryConfig{
		MaxRetries:     opts.DeliveryMaxRetries,
		InitialDelayMs: opts.DeliveryInitialDelayMs,
		MaxDelayMs:     opts.DeliveryMaxDelayMs,
		Concurrency:    opts.DeliveryConcurrency,
	})

	// Register handler on shared mux under the channel's context path.
	hubPath := cfg.Channel.Context + "/_hub"
	cfg.Mux.Handle(hubPath, handler)

	return &WebSubEntrypoint{
		handler:   handler,
		deliverer: deliverer,
		topics:    topics,
		store:     store,
		endpoint:  cfg.Endpoint,
		channel:   cfg.Channel,
		opts:      opts,
	}, nil
}

// Start creates an endpoint consumer for event delivery.
// The HTTP server is managed by the runtime.
func (e *WebSubEntrypoint) Start(ctx context.Context) error {
	if e.channel.EndpointTopic == "" {
		return nil
	}

	groupID := e.opts.ConsumerGroupPrefix + "-websub-" + e.channel.Name
	consumer, err := e.endpoint.Subscribe(groupID, []string{e.channel.EndpointTopic},
		func(ctx context.Context, msg *connectors.Message) error {
			return e.deliverer.DeliverToSubscribers(ctx, e.channel.Name, e.channel.PublicTopic, msg)
		})
	if err != nil {
		return fmt.Errorf("failed to create websub consumer: %w", err)
	}
	if err := consumer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start websub consumer: %w", err)
	}
	e.consumer = consumer

	slog.Info("WebSub entrypoint started",
		"channel", e.channel.Name,
		"topic", e.channel.PublicTopic,
		"endpoint_topic", e.channel.EndpointTopic,
	)
	return nil
}

// Stop drains in-flight deliveries and stops the consumer.
func (e *WebSubEntrypoint) Stop(ctx context.Context) error {
	if e.consumer != nil {
		if err := e.consumer.Stop(ctx); err != nil {
			slog.Error("Failed to stop websub consumer", "error", err)
		}
	}
	e.deliverer.Wait()
	return nil
}
