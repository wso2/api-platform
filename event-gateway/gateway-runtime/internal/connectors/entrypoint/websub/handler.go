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
	"net/http"
	"strconv"
	"time"

	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/hub"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/subscription"
)

// Handler implements the WebSub hub HTTP endpoint for POST /hub.
type Handler struct {
	topics       *TopicRegistry
	store        subscription.SubscriptionStore
	hub          *hub.Hub
	verifier     *Verifier
	defaultLease int
}

// NewHandler creates a new WebSub hub handler.
func NewHandler(topics *TopicRegistry, store subscription.SubscriptionStore, h *hub.Hub, verificationTimeout time.Duration, defaultLease int) *Handler {
	return &Handler{
		topics:       topics,
		store:        store,
		hub:          h,
		verifier:     NewVerifier(store, verificationTimeout),
		defaultLease: defaultLease,
	}
}

// ServeHTTP dispatches on hub.mode form parameter.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	case "register":
		h.handleRegister(w, r)
	case "deregister":
		h.handleDeregister(w, r)
	case "subscribe":
		h.handleSubscribe(w, r)
	case "unsubscribe":
		h.handleUnsubscribe(w, r)
	default:
		http.Error(w, fmt.Sprintf("unknown hub.mode: %s", mode), http.StatusBadRequest)
	}
}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	topic := r.FormValue("hub.topic")
	if topic == "" {
		http.Error(w, "hub.topic is required", http.StatusBadRequest)
		return
	}

	h.topics.Register(topic)
	slog.Info("Topic registered", "topic", topic)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "topic registered: %s", topic)
}

func (h *Handler) handleDeregister(w http.ResponseWriter, r *http.Request) {
	topic := r.FormValue("hub.topic")
	if topic == "" {
		http.Error(w, "hub.topic is required", http.StatusBadRequest)
		return
	}

	h.topics.Deregister(topic)
	slog.Info("Topic deregistered", "topic", topic)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "topic deregistered: %s", topic)
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

	// Trigger async verification
	go func() {
		ctx := context.Background()
		if err := h.verifier.VerifySubscribe(ctx, sub); err != nil {
			slog.Error("Subscription verification failed", "topic", topic, "callback", callback, "error", err)
		}
	}()

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

	// Trigger async verification for unsubscribe
	go func() {
		ctx := context.Background()
		sub := &subscription.Subscription{
			Topic:       topic,
			CallbackURL: callback,
			State:       subscription.StateActive,
		}
		if err := h.verifier.VerifyUnsubscribe(ctx, sub); err != nil {
			slog.Error("Unsubscribe verification failed", "topic", topic, "callback", callback, "error", err)
		} else {
			// Verification succeeded, remove subscription
			if err := h.store.Remove(topic, callback); err != nil {
				slog.Error("Failed to remove subscription", "error", err)
			}
		}
	}()

	w.WriteHeader(http.StatusAccepted)
}
