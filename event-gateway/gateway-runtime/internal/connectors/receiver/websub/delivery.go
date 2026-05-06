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
	"strings"
	"time"

	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
)

// DeliveryConfig holds configuration for the delivery engine.
type DeliveryConfig struct {
	MaxRetries     int
	InitialDelayMs int
	MaxDelayMs     int
	Concurrency    int
}

// Deliverer delivers events to a single subscriber callback URL.
type Deliverer struct {
	config DeliveryConfig
	client *http.Client
}

// NewDeliverer creates a new Deliverer.
func NewDeliverer(config DeliveryConfig) *Deliverer {
	return &Deliverer{
		config: config,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Deliver delivers a message to a single callback URL with retry and HMAC.
func (d *Deliverer) Deliver(ctx context.Context, callbackURL, secret string, msg *connectors.Message) error {
	return d.deliverWithRetry(ctx, callbackURL, secret, msg)
}

func (d *Deliverer) deliverWithRetry(ctx context.Context, callbackURL, secret string, msg *connectors.Message) error {
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

		if err := d.doDeliver(ctx, callbackURL, secret, msg); err != nil {
			lastErr = err
			slog.Warn("Delivery attempt failed",
				"callback", callbackURL,
				"attempt", attempt+1,
				"error", err,
			)
			continue
		}
		return nil
	}
	return lastErr
}

func (d *Deliverer) doDeliver(ctx context.Context, callbackURL, secret string, msg *connectors.Message) error {
	body := msg.Value
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, callbackURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create delivery request: %w", err)
	}

	// Forward application-level headers only. Skip RFC 2616 hop-by-hop headers
	// and any internal/gateway-specific headers to prevent information leakage
	// and transport mis-configuration in the downstream subscriber request.
	for k, vals := range msg.Headers {
		if isForwardableHeader(k) {
			for _, v := range vals {
				req.Header.Add(k, v)
			}
		}
	}

	// Ensure a content-type is set if not already present via message headers.
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/octet-stream")
	}

	// Add HMAC signature if secret is set.
	if secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		signature := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Hub-Signature", "sha256="+signature)
	}

	// Add Link headers per W3C spec.
	if topic := msg.Topic; topic != "" {
		req.Header.Add("Link", fmt.Sprintf("<%s>; rel=\"self\"", topic))
	}

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

// hopByHopHeaders contains the RFC 2616 hop-by-hop headers that must never be
// forwarded to a downstream subscriber. Keys are lower-cased for comparison.
var hopByHopHeaders = map[string]bool{
	"connection":        true,
	"keep-alive":        true,
	"te":                true,
	"trailer":           true,
	"transfer-encoding": true,
	"upgrade":           true,
	"host":              true,
}

// internalHeaderPrefixes lists lower-cased prefixes used by the gateway for
// internal metadata that must not be leaked to external subscribers.
var internalHeaderPrefixes = []string{
	"proxy-",
	"x-internal-",
}

// isForwardableHeader reports whether header k should be included in the
// outgoing subscriber delivery request. It returns false for RFC hop-by-hop
// headers and for any gateway-internal header prefix.
func isForwardableHeader(k string) bool {
	lower := strings.ToLower(k)
	if hopByHopHeaders[lower] {
		return false
	}
	for _, prefix := range internalHeaderPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return false
		}
	}
	return true
}
