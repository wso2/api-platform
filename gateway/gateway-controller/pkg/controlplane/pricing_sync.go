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

package controlplane

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// The pricing catalog is the model price file the policy engine's llm-cost policy reads
// (policy_configurations.llm_cost_v1.pricing_file). The controller reads the same file and
// posts it, unchanged, to the control plane after every (re)connect, as it posts the policy
// manifest: POST /api/internal/v1/gateways/{gatewayId}/pricing. Nothing is pushed on a
// timer or when the file changes: the policy engine reads the file only at start, so a
// changed file reaches the control plane when the gateway restarts and connects again.
// It is pushed only when cost tracking or budgets are on, and a problem with it never
// delays the connection or the other pushes, nor stops the gateway.
const (
	// defaultPricingFile is where the gateway runtime mounts the catalog, and what the
	// policy engine uses when its config names no file.
	defaultPricingFile = "/etc/policy-engine/llm-pricing/model_prices.json"
	// maxPricingBytes is the largest file sent: the control plane refuses more (413).
	maxPricingBytes = 2 << 20
	// pricingPushTimeout bounds one attempt.
	pricingPushTimeout = 30 * time.Second
)

// pricingEnabled reports whether this gateway tracks costs or enforces budgets, the two
// things that use prices.
func (c *Client) pricingEnabled() bool {
	cfg := c.systemConfig
	return cfg != nil && (cfg.AICostManagement.Budgets.Enabled || cfg.AICostManagement.Tracking.Enabled)
}

// pricingFile is the catalog's path: the one the llm-cost policy is configured with, else
// the default mount.
func (c *Client) pricingFile() string {
	if c.systemConfig != nil {
		if table, ok := c.systemConfig.PolicyConfigurations["llm_cost_v1"].(map[string]interface{}); ok {
			if path, ok := table["pricing_file"].(string); ok && path != "" {
				return path
			}
		}
	}
	return defaultPricingFile
}

// readPricingFile returns the catalog's bytes, as they are, when the file is a JSON object
// the control plane will take.
func readPricingFile(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	switch {
	case len(raw) > maxPricingBytes:
		return nil, fmt.Errorf("the file is larger than %d bytes", maxPricingBytes)
	case !json.Valid(raw) || !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")):
		return nil, errors.New("the file is not a JSON object")
	}
	return raw, nil
}

// pushPricingOnConnect posts the catalog to the control plane right after a connect.
// A missing or unreadable file, and a refusal by the control plane, are logged once at
// warn and are never an error for the gateway; a lost connection or a 5xx is retried as
// the other pushes are (controller.controlplane.retry_config).
func (c *Client) pushPricingOnConnect(gatewayID string) {
	if c.IsOnPrem() || !c.pricingEnabled() {
		return
	}
	path := c.pricingFile()
	body, err := readPricingFile(path)
	if err != nil {
		if c.pricingWarned.CompareAndSwap(false, true) {
			c.logger.Warn("The pricing catalog is not pushed to the control plane",
				slog.String("file", path), slog.Any("error", err))
		}
		return
	}
	c.pricingWarned.Store(false)

	err = c.retryPush("pricing catalog push", func() error {
		status, detail, err := c.postPricing(gatewayID, body)
		switch {
		case err != nil:
			return err // a lost connection: retry
		case status >= 200 && status < 300:
			return nil
		case status == http.StatusTooManyRequests || status >= 500:
			return fmt.Errorf("pricing catalog push returned %d: %s", status, detail) // retry
		default:
			// The control plane refused it (an old control plane has no such route, or
			// the file is not accepted): retrying won't change that, and prices are
			// advisory, so it is logged once and the gateway carries on.
			if c.pricingRejected.CompareAndSwap(false, true) {
				c.logger.Warn("The control plane did not accept the pricing catalog",
					slog.Int("status", status), slog.String("detail", detail))
			}
			return nil
		}
	}, slog.String("gateway_id", gatewayID))
	if err != nil {
		return // already logged by retryPush; the next connect pushes it again
	}
	c.logger.Debug("Pushed the pricing catalog to the control plane",
		slog.String("gateway_id", gatewayID), slog.Int("bytes", len(body)))
}

// postPricing sends one attempt. It returns the response status and the start of its
// body, or an error when no response came.
func (c *Client) postPricing(gatewayID string, body []byte) (int, string, error) {
	url := c.getRestAPIBaseURL() + "/gateways/" + gatewayID + "/pricing"

	ctx, cancel := context.WithTimeout(c.ctx, pricingPushTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, "", fmt.Errorf("failed to create pricing request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", c.config.Token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("failed to send the pricing catalog: %w", err)
	}
	defer resp.Body.Close()
	detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return resp.StatusCode, string(bytes.TrimSpace(detail)), nil
}
