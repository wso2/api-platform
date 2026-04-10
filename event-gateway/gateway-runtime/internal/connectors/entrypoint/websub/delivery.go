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
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/subscription"
)

// DeliveryConfig holds configuration for the delivery engine.
type DeliveryConfig struct {
	MaxRetries     int
	InitialDelayMs int
	MaxDelayMs     int
	Concurrency    int
}

// Deliverer delivers events to subscriber callback URLs.
type Deliverer struct {
	store     subscription.SubscriptionStore
	processor connectors.MessageProcessor
	config    DeliveryConfig
	client    *http.Client
	sem       chan struct{}
	wg        sync.WaitGroup
}

// NewDeliverer creates a new Deliverer.
func NewDeliverer(store subscription.SubscriptionStore, processor connectors.MessageProcessor, config DeliveryConfig) *Deliverer {
	return &Deliverer{
		store:     store,
		processor: processor,
		config:    config,
		client:    &http.Client{Timeout: 30 * time.Second},
		sem:       make(chan struct{}, config.Concurrency),
	}
}

// DeliverToSubscribers delivers a message to all active subscribers for the public topic
// associated with this channel, even when events are consumed from a different broker topic.
// It applies outbound policies before delivery and handles retries.
func (d *Deliverer) DeliverToSubscribers(ctx context.Context, bindingName, publicTopic string, msg *connectors.Message) error {
	msg.Topic = publicTopic

	subs := d.store.GetByTopic(publicTopic)
	if len(subs) == 0 {
		return nil
	}

	// Apply outbound policies
	processed, shortCircuited, err := d.processor.ProcessOutbound(ctx, bindingName, msg)
	if err != nil {
		return fmt.Errorf("outbound policy execution failed: %w", err)
	}
	if shortCircuited {
		slog.Info("Delivery short-circuited by outbound policy", "topic", msg.Topic)
		return nil
	}

	for _, sub := range subs {
		if sub.State != subscription.StateActive {
			continue
		}
		if sub.LeaseSeconds > 0 && !sub.ExpiresAt.IsZero() && time.Now().After(sub.ExpiresAt) {
			continue
		}

		d.wg.Add(1)
		d.sem <- struct{}{} // acquire semaphore
		go func(s *subscription.Subscription) {
			defer d.wg.Done()
			defer func() { <-d.sem }() // release semaphore

			if err := d.deliverWithRetry(ctx, s, processed); err != nil {
				slog.Error("Delivery failed after retries, marking subscription inactive",
					"topic", s.Topic,
					"callback", s.CallbackURL,
					"error", err,
				)
				if updateErr := d.store.UpdateState(s.ID, subscription.StateInactive); updateErr != nil {
					slog.Error("Failed to mark subscription inactive", "error", updateErr)
				}
			}
		}(sub)
	}

	return nil
}

// Wait waits for all in-flight deliveries to complete.
func (d *Deliverer) Wait() {
	d.wg.Wait()
}

func (d *Deliverer) deliverWithRetry(ctx context.Context, sub *subscription.Subscription, msg *connectors.Message) error {
	delay := time.Duration(d.config.InitialDelayMs) * time.Millisecond
	maxDelay := time.Duration(d.config.MaxDelayMs) * time.Millisecond

	var lastErr error
	for attempt := 0; attempt <= d.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
			}
		}

		if err := d.deliver(ctx, sub, msg); err != nil {
			lastErr = err
			slog.Warn("Delivery attempt failed",
				"topic", sub.Topic,
				"callback", sub.CallbackURL,
				"attempt", attempt+1,
				"error", err,
			)
			continue
		}
		return nil
	}
	return lastErr
}

func (d *Deliverer) deliver(ctx context.Context, sub *subscription.Subscription, msg *connectors.Message) error {
	body := msg.Value
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.CallbackURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create delivery request: %w", err)
	}

	// Set content type from message headers
	if ct, ok := msg.Headers["content-type"]; ok && len(ct) > 0 {
		req.Header.Set("Content-Type", ct[0])
	} else {
		req.Header.Set("Content-Type", "application/octet-stream")
	}

	// Add HMAC signature if secret is set
	if sub.Secret != "" {
		mac := hmac.New(sha256.New, []byte(sub.Secret))
		mac.Write(body)
		signature := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Hub-Signature", "sha256="+signature)
	}

	// Add Link headers per W3C spec
	req.Header.Add("Link", fmt.Sprintf("<%s>; rel=\"self\"", sub.Topic))

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("delivery request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("delivery failed: subscriber returned status %d", resp.StatusCode)
	}

	return nil
}
