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
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/subscription"
)

// Handler implements the WebSub hub HTTP endpoint.
// It supports W3C-aligned subscribe/unsubscribe, as well as a publish
// endpoint for external parties to ingest data into the hub.
type Handler struct {
	topics        *TopicRegistry
	store         subscription.SubscriptionStore
	verifier      *Verifier
	processor     connectors.MessageProcessor
	endpoint      connectors.Endpoint
	channelName   string
	publicTopic   string
	internalTopic string
	defaultLease  int
}

// NewHandler creates a new WebSub hub handler.
func NewHandler(
	topics *TopicRegistry,
	store subscription.SubscriptionStore,
	verificationTimeout time.Duration,
	defaultLease int,
	processor connectors.MessageProcessor,
	endpoint connectors.Endpoint,
	channelName string,
	publicTopic string,
	internalTopic string,
) *Handler {
	return &Handler{
		topics:        topics,
		store:         store,
		verifier:      NewVerifier(store, verificationTimeout),
		processor:     processor,
		endpoint:      endpoint,
		channelName:   channelName,
		publicTopic:   publicTopic,
		internalTopic: internalTopic,
		defaultLease:  defaultLease,
	}
}

// ServeHTTP dispatches on hub.mode for form-encoded requests,
// or treats non-form POSTs as publish (content ingestion).
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ct := r.Header.Get("Content-Type")
	// If the content type is form-encoded, dispatch on hub.mode.
	if ct == "application/x-www-form-urlencoded" || ct == "" {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form data", http.StatusBadRequest)
			return
		}

		mode := r.FormValue("hub.mode")
		switch mode {
		case "subscribe":
			h.handleSubscribe(w, r)
		case "unsubscribe":
			h.handleUnsubscribe(w, r)
		case "publish":
			h.handlePublishForm(w, r)
		default:
			http.Error(w, fmt.Sprintf("unknown hub.mode: %s", mode), http.StatusBadRequest)
		}
		return
	}

	// Non-form POST = content distribution request (W3C §7.1 publish).
	h.handlePublishContent(w, r)
}

func (h *Handler) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	callback := r.FormValue("hub.callback")
	topic := r.FormValue("hub.topic")
	secret := r.FormValue("hub.secret")
	leaseStr := r.FormValue("hub.lease_seconds")

	if callback == "" {
		http.Error(w, "hub.callback is required", http.StatusBadRequest)
		return
	}
	if topic == "" {
		http.Error(w, "hub.topic is required", http.StatusBadRequest)
		return
	}

	if !h.topics.IsRegistered(topic) {
		http.Error(w, fmt.Sprintf("topic not registered: %s", topic), http.StatusNotFound)
		return
	}

	// Verify that the Kafka topic actually exists before accepting the subscription.
	exists, err := h.endpoint.TopicExists(r.Context(), h.internalTopic)
	if err != nil {
		slog.Error("Failed to check topic existence", "topic", h.internalTopic, "error", err)
		http.Error(w, "failed to verify topic existence", http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Error(w, fmt.Sprintf("topic does not exist in broker: %s", h.internalTopic), http.StatusNotFound)
		return
	}

	// Reject duplicate: same callback URL cannot subscribe to the same channel twice.
	if h.store.ExistsByCallback(callback) {
		http.Error(w, "callback URL is already subscribed to this channel", http.StatusConflict)
		return
	}

	leaseSeconds := h.defaultLease
	if leaseStr != "" {
		parsed, err := strconv.Atoi(leaseStr)
		if err != nil {
			http.Error(w, "invalid hub.lease_seconds", http.StatusBadRequest)
			return
		}
		leaseSeconds = parsed
	}

	sub := &subscription.Subscription{
		Topic:        topic,
		CallbackURL:  callback,
		Secret:       secret,
		LeaseSeconds: leaseSeconds,
		State:        subscription.StatePending,
		CreatedAt:    time.Now(),
	}

	if err := h.store.Add(sub); err != nil {
		slog.Error("Failed to add subscription", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Verify intent synchronously — only return 202 on success.
	if err := h.verifier.VerifySubscribe(r.Context(), sub); err != nil {
		slog.Error("Subscription verification failed", "topic", topic, "callback", callback, "error", err)
		// Remove the pending subscription on verification failure.
		if removeErr := h.store.Remove(topic, callback); removeErr != nil {
			slog.Error("Failed to remove failed subscription", "error", removeErr)
		}
		http.Error(w, "intent verification failed", http.StatusForbidden)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	callback := r.FormValue("hub.callback")
	topic := r.FormValue("hub.topic")

	if callback == "" {
		http.Error(w, "hub.callback is required", http.StatusBadRequest)
		return
	}
	if topic == "" {
		http.Error(w, "hub.topic is required", http.StatusBadRequest)
		return
	}

	// Verify intent synchronously — only return 202 on success.
	sub := &subscription.Subscription{
		Topic:       topic,
		CallbackURL: callback,
		State:       subscription.StateActive,
	}
	if err := h.verifier.VerifyUnsubscribe(r.Context(), sub); err != nil {
		slog.Error("Unsubscribe verification failed", "topic", topic, "callback", callback, "error", err)
		http.Error(w, "intent verification failed", http.StatusForbidden)
		return
	}

	// Verification succeeded, remove subscription.
	if err := h.store.Remove(topic, callback); err != nil {
		slog.Error("Failed to remove subscription", "error", err)
		http.Error(w, "subscription not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// handlePublishForm handles W3C hub.mode=publish form requests.
// The publisher specifies hub.topic and optionally hub.url / hub.content.
func (h *Handler) handlePublishForm(w http.ResponseWriter, r *http.Request) {
	topic := r.FormValue("hub.topic")
	if topic == "" {
		http.Error(w, "hub.topic is required", http.StatusBadRequest)
		return
	}

	if !h.topics.IsRegistered(topic) {
		http.Error(w, fmt.Sprintf("topic not registered: %s", topic), http.StatusNotFound)
		return
	}

	msg := &connectors.Message{
		Value:   []byte(r.FormValue("hub.content")),
		Headers: make(map[string][]string),
		Topic:   h.publicTopic,
	}

	// If hub.url is provided, the publisher is notifying that content
	// at that URL has been updated (W3C §7). Store as metadata.
	if hubURL := r.FormValue("hub.url"); hubURL != "" {
		msg.Headers["hub-url"] = []string{hubURL}
	}

	if err := h.publishToEndpoint(r.Context(), h.internalTopic, msg); err != nil {
		slog.Error("Publish failed", "topic", topic, "error", err)
		http.Error(w, "publish failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// handlePublishContent handles direct content POST (non-form).
// External parties POST raw content (JSON, XML, etc.) to the hub endpoint.
// The content is ingested into the broker via inbound policies.
func (h *Handler) handlePublishContent(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10 MB limit
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	headers := make(map[string][]string)
	if ct := r.Header.Get("Content-Type"); ct != "" {
		headers["content-type"] = []string{ct}
	}

	if h.internalTopic == "" {
		http.Error(w, "no topic registered for this channel", http.StatusNotFound)
		return
	}

	msg := &connectors.Message{
		Value:   body,
		Headers: headers,
		Topic:   h.publicTopic,
	}

	if err := h.publishToEndpoint(r.Context(), h.internalTopic, msg); err != nil {
		slog.Error("Content publish failed", "topic", h.internalTopic, "error", err)
		http.Error(w, "publish failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// publishToEndpoint applies inbound policies and publishes the message to the endpoint.
func (h *Handler) publishToEndpoint(ctx context.Context, topic string, msg *connectors.Message) error {
	processed, shortCircuited, err := h.processor.ProcessInbound(ctx, h.channelName, msg)
	if err != nil {
		return fmt.Errorf("inbound policy execution failed: %w", err)
	}
	if shortCircuited {
		slog.Info("Publish short-circuited by inbound policy", "topic", topic)
		return nil
	}

	if err := h.endpoint.Publish(ctx, topic, processed); err != nil {
		return fmt.Errorf("failed to publish to endpoint: %w", err)
	}

	slog.Debug("Published to endpoint", "topic", topic, "channel", h.channelName)
	return nil
}
