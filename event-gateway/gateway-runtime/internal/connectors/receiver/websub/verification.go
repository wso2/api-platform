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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/subscription"
)

// Verifier performs W3C WebSub §5.3 intent verification for subscribe/unsubscribe.
type Verifier struct {
	store   subscription.SubscriptionStore
	timeout time.Duration
	client  *http.Client
}

// NewVerifier creates a new Verifier.
func NewVerifier(store subscription.SubscriptionStore, timeout time.Duration) *Verifier {
	return &Verifier{
		store:   store,
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// VerifySubscribe performs intent verification for a subscribe request per W3C WebSub §5.3.
func (v *Verifier) VerifySubscribe(ctx context.Context, sub *subscription.Subscription) error {
	challenge, err := generateChallenge()
	if err != nil {
		return fmt.Errorf("failed to generate challenge: %w", err)
	}

	params := url.Values{
		"hub.mode":          {"subscribe"},
		"hub.topic":         {sub.Topic},
		"hub.challenge":     {challenge},
		"hub.lease_seconds": {strconv.Itoa(sub.LeaseSeconds)},
	}

	verifyURL := appendQueryParams(sub.CallbackURL, params)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, verifyURL, nil)
	if err != nil {
		if updateErr := v.store.UpdateState(sub.ID, subscription.StateInactive); updateErr != nil {
			slog.Error("Failed to update subscription state", "error", updateErr)
		}
		return fmt.Errorf("failed to create verification request: %w", err)
	}

	resp, err := v.client.Do(req)
	if err != nil {
		if updateErr := v.store.UpdateState(sub.ID, subscription.StateInactive); updateErr != nil {
			slog.Error("Failed to update subscription state", "error", updateErr)
		}
		return fmt.Errorf("verification request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if updateErr := v.store.UpdateState(sub.ID, subscription.StateInactive); updateErr != nil {
			slog.Error("Failed to update subscription state", "error", updateErr)
		}
		return fmt.Errorf("verification failed: subscriber returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if updateErr := v.store.UpdateState(sub.ID, subscription.StateInactive); updateErr != nil {
			slog.Error("Failed to update subscription state", "error", updateErr)
		}
		return fmt.Errorf("failed to read verification response: %w", err)
	}

	if string(body) != challenge {
		if updateErr := v.store.UpdateState(sub.ID, subscription.StateInactive); updateErr != nil {
			slog.Error("Failed to update subscription state", "error", updateErr)
		}
		return fmt.Errorf("verification failed: challenge mismatch (expected %q, got %q)", challenge, string(body))
	}

	// Verification succeeded — activate subscription
	var expiresAt time.Time
	if sub.LeaseSeconds > 0 {
		expiresAt = time.Now().Add(time.Duration(sub.LeaseSeconds) * time.Second)
	}
	sub.ExpiresAt = expiresAt

	if err := v.store.UpdateState(sub.ID, subscription.StateActive); err != nil {
		return fmt.Errorf("failed to activate subscription: %w", err)
	}

	slog.Info("Subscription verified and activated",
		"topic", sub.Topic,
		"callback", sub.CallbackURL,
		"lease_seconds", sub.LeaseSeconds,
	)

	return nil
}

// VerifyUnsubscribe performs intent verification for an unsubscribe request.
func (v *Verifier) VerifyUnsubscribe(ctx context.Context, sub *subscription.Subscription) error {
	challenge, err := generateChallenge()
	if err != nil {
		return fmt.Errorf("failed to generate challenge: %w", err)
	}

	params := url.Values{
		"hub.mode":      {"unsubscribe"},
		"hub.topic":     {sub.Topic},
		"hub.challenge": {challenge},
	}

	verifyURL := appendQueryParams(sub.CallbackURL, params)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, verifyURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create verification request: %w", err)
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("verification request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("verification failed: subscriber returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read verification response: %w", err)
	}

	if string(body) != challenge {
		return fmt.Errorf("verification failed: challenge mismatch")
	}

	slog.Info("Unsubscribe verified", "topic", sub.Topic, "callback", sub.CallbackURL)
	return nil
}

func generateChallenge() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func appendQueryParams(baseURL string, params url.Values) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return baseURL + "?" + params.Encode()
	}
	existing := u.Query()
	for k, vs := range params {
		for _, v := range vs {
			existing.Add(k, v)
		}
	}
	u.RawQuery = existing.Encode()
	return u.String()
}
