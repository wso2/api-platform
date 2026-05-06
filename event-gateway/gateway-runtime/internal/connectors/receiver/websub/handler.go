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

// HubHandler implements the WebSub hub handler for subscribe/unsubscribe.
// Registered at {context}/{version}/hub.
type HubHandler struct {
	topics       *TopicRegistry
	store        subscription.SubscriptionStore
	verifier     *Verifier
	processor    connectors.MessageProcessor
	brokerDriver connectors.BrokerDriver
	bindingName  string
	channels     map[string]string // channel-name → Kafka topic
	consumerMgr  *ConsumerManager
	syncProducer *subscription.SyncProducer
	defaultLease int
}

// NewHubHandler creates a new hub handler for subscribe/unsubscribe.
func NewHubHandler(
	topics *TopicRegistry,
	store subscription.SubscriptionStore,
	verificationTimeout time.Duration,
	defaultLease int,
	processor connectors.MessageProcessor,
	brokerDriver connectors.BrokerDriver,
	bindingName string,
	channels map[string]string,
	consumerMgr *ConsumerManager,
	syncProducer *subscription.SyncProducer,
) *HubHandler {
	return &HubHandler{
		topics:       topics,
		store:        store,
		verifier:     NewVerifier(store, verificationTimeout),
		processor:    processor,
		brokerDriver: brokerDriver,
		bindingName:  bindingName,
		channels:     channels,
		consumerMgr:  consumerMgr,
		syncProducer: syncProducer,
		defaultLease: defaultLease,
	}
}

// ServeHTTP dispatches on hub.mode for form-encoded subscribe/unsubscribe requests.
func (h *HubHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

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
	default:
		http.Error(w, fmt.Sprintf("unknown hub.mode: %s", mode), http.StatusBadRequest)
	}
}

func (h *HubHandler) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	// Enforce subscribe policies before processing.
	subMsg := httpRequestToMessage(r)
	_, shortCircuited, err := h.processor.ProcessSubscribe(r.Context(), h.bindingName, subMsg)
	if err != nil {
		slog.Error("Subscribe policy execution failed", "error", err)
		http.Error(w, "policy execution failed", http.StatusInternalServerError)
		return
	}
	if shortCircuited {
		writePolicyResponse(w, nil, http.StatusForbidden, "forbidden by policy")
		return
	}

	callback := r.FormValue("hub.callback")
	topic := r.FormValue("hub.topic") // topic = channel name
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

	// Validate channel name is registered.
	if !h.topics.IsRegistered(topic) {
		http.Error(w, fmt.Sprintf("topic not registered: %s", topic), http.StatusNotFound)
		return
	}

	// Resolve the Kafka topic for this channel.
	kafkaTopic, ok := h.channels[topic]
	if !ok {
		http.Error(w, fmt.Sprintf("no kafka topic for channel: %s", topic), http.StatusNotFound)
		return
	}

	// Verify that the Kafka topic actually exists.
	exists, err := h.brokerDriver.TopicExists(r.Context(), kafkaTopic)
	if err != nil {
		slog.Error("Failed to check topic existence", "topic", kafkaTopic, "error", err)
		http.Error(w, "failed to verify topic existence", http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Error(w, fmt.Sprintf("topic does not exist in broker: %s", kafkaTopic), http.StatusNotFound)
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

	// Verify intent synchronously.
	if err := h.verifier.VerifySubscribe(r.Context(), sub); err != nil {
		slog.Error("Subscription verification failed", "topic", topic, "callback", callback, "error", err)
		if removeErr := h.store.Remove(topic, callback); removeErr != nil {
			slog.Error("Failed to remove failed subscription", "error", removeErr)
		}
		http.Error(w, "intent verification failed", http.StatusForbidden)
		return
	}

	// Create/update per-callback consumer.
	if err := h.consumerMgr.AddSubscription(callback, secret, kafkaTopic); err != nil {
		slog.Error("Failed to create consumer for subscription", "callback", callback, "error", err)
		// Don't fail the subscription — consumer can be recreated on reconciliation.
	}

	// Publish subscription state to sync topic.
	if h.syncProducer != nil {
		if err := h.syncProducer.PublishSubscription(r.Context(), sub); err != nil {
			slog.Error("Failed to sync subscription", "error", err)
		}
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *HubHandler) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	// Enforce subscribe policies before processing.
	subMsg := httpRequestToMessage(r)
	_, shortCircuited, err := h.processor.ProcessSubscribe(r.Context(), h.bindingName, subMsg)
	if err != nil {
		slog.Error("Unsubscribe policy execution failed", "error", err)
		http.Error(w, "policy execution failed", http.StatusInternalServerError)
		return
	}
	if shortCircuited {
		writePolicyResponse(w, nil, http.StatusForbidden, "forbidden by policy")
		return
	}

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

	// Verify intent synchronously.
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

	// Stop/update per-callback consumer.
	kafkaTopic := h.channels[topic]
	if kafkaTopic != "" {
		if err := h.consumerMgr.RemoveSubscription(callback, kafkaTopic); err != nil {
			slog.Error("Failed to update consumer on unsubscribe", "callback", callback, "error", err)
		}
	}

	// Remove subscription from store.
	if err := h.store.Remove(topic, callback); err != nil {
		slog.Error("Failed to remove subscription", "error", err)
		http.Error(w, "subscription not found", http.StatusNotFound)
		return
	}

	// Publish tombstone to sync topic.
	if h.syncProducer != nil {
		if err := h.syncProducer.PublishTombstone(r.Context(), topic, callback); err != nil {
			slog.Error("Failed to sync unsubscription", "error", err)
		}
	}

	w.WriteHeader(http.StatusAccepted)
}

// WebhookReceiverHandler handles ingress (content distribution) on
// {context}/{version}/webhook-receiver?topic=X.
type WebhookReceiverHandler struct {
	topics       *TopicRegistry
	processor    connectors.MessageProcessor
	brokerDriver connectors.BrokerDriver
	bindingName  string
	channels     map[string]string // channel-name → Kafka topic
}

// NewWebhookReceiverHandler creates a new webhook receiver handler.
func NewWebhookReceiverHandler(
	topics *TopicRegistry,
	processor connectors.MessageProcessor,
	brokerDriver connectors.BrokerDriver,
	bindingName string,
	channels map[string]string,
) *WebhookReceiverHandler {
	return &WebhookReceiverHandler{
		topics:       topics,
		processor:    processor,
		brokerDriver: brokerDriver,
		bindingName:  bindingName,
		channels:     channels,
	}
}

// ServeHTTP handles POST requests to the webhook receiver.
// Query param "topic" identifies which channel the event is for.
func (h *WebhookReceiverHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	channelName := r.URL.Query().Get("topic")
	if channelName == "" {
		http.Error(w, "query parameter 'topic' is required", http.StatusBadRequest)
		return
	}

	if !h.topics.IsRegistered(channelName) {
		http.Error(w, fmt.Sprintf("channel not registered: %s", channelName), http.StatusNotFound)
		return
	}

	kafkaTopic, ok := h.channels[channelName]
	if !ok {
		http.Error(w, fmt.Sprintf("no kafka topic for channel: %s", channelName), http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10 MB limit
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	headers := make(map[string][]string, len(r.Header))
	for k, v := range r.Header {
		headers[k] = append([]string(nil), v...)
	}

	msg := &connectors.Message{
		Value:    body,
		Headers:  headers,
		Topic:    channelName,
		Metadata: buildMessageMetadata(r),
	}

	// Enforce inbound policies before publishing.
	processed, shortCircuited, err := h.processor.ProcessInbound(r.Context(), h.bindingName, msg)
	if err != nil {
		slog.Error("Inbound policy execution failed", "channel", channelName, "error", err)
		http.Error(w, "policy execution failed", http.StatusInternalServerError)
		return
	}
	if shortCircuited {
		slog.Info("Inbound request rejected by policy", "channel", channelName, "binding", h.bindingName)
		writePolicyResponse(w, processed, http.StatusForbidden, "forbidden by policy")
		return
	}

	if err := h.publishToBrokerDriver(r.Context(), kafkaTopic, processed); err != nil {
		slog.Error("Webhook receiver publish failed", "channel", channelName, "topic", kafkaTopic, "error", err)
		http.Error(w, "publish failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// writePolicyResponse writes the HTTP response from a short-circuited policy execution.
// If msg is non-nil and carries a status code in Metadata["status_code"], that status is used
// along with any headers and body from the message. Otherwise the fallback status and body are used.
func writePolicyResponse(w http.ResponseWriter, msg *connectors.Message, fallbackStatus int, fallbackBody string) {
	if msg != nil {
		statusCode := fallbackStatus
		if sc, ok := msg.Metadata["status_code"]; ok {
			if code, ok := sc.(int); ok && code > 0 {
				statusCode = code
			}
		}
		for k, vals := range msg.Headers {
			for _, v := range vals {
				w.Header().Add(k, v)
			}
		}
		if len(msg.Value) > 0 {
			w.WriteHeader(statusCode)
			_, _ = w.Write(msg.Value)
			return
		}
		w.WriteHeader(statusCode)
		return
	}
	http.Error(w, fallbackBody, fallbackStatus)
}

func (h *WebhookReceiverHandler) publishToBrokerDriver(ctx context.Context, kafkaTopic string, msg *connectors.Message) error {
	if err := h.brokerDriver.Publish(ctx, kafkaTopic, msg); err != nil {
		return fmt.Errorf("failed to publish to broker-driver: %w", err)
	}

	slog.Debug("Published to broker-driver", "topic", kafkaTopic, "binding", h.bindingName)
	return nil
}

// httpRequestToMessage builds a Message from an HTTP request for subscribe policy enforcement.
// Only the request headers are relevant; the form body has already been parsed.
func httpRequestToMessage(r *http.Request) *connectors.Message {
	headers := make(map[string][]string, len(r.Header))
	for k, v := range r.Header {
		headers[k] = append([]string(nil), v...)
	}
	return &connectors.Message{
		Headers:  headers,
		Topic:    r.FormValue("hub.topic"),
		Metadata: buildMessageMetadata(r),
	}
}

func buildMessageMetadata(r *http.Request) map[string]interface{} {
	if r == nil || r.URL == nil {
		return nil
	}

	return map[string]interface{}{
		"request_path":   r.URL.RequestURI(),
		"request_method": r.Method,
	}
}
