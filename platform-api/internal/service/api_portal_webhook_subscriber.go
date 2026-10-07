/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

// Self-registration of platform-api as a webhook subscriber on each portal, so
// subscription + apikey events reach the receiver mounted at
// webhook.RoutePath. Shared by the OSS Create path (sync, fail-closed) and
// the cloud plugin's provisioning poller (async, best-effort).

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
)

// platformAPISubscriberID is the fixed handle every platform-api subscriber
// row carries on the portal. Making it a constant means idempotency across
// Create retries + multiple apip-platform-api replicas is automatic: the
// portal's uniqueness constraint on (org, handle) collapses duplicate POSTs
// into a 409 that the PUT branch below treats as "already registered".
const platformAPISubscriberID = "platform-api"

// webhookSubscribersPath is the portal's REST path for the subscribers
// collection. Lives under the portal's /api-portal/<org>-less mount because
// the shared-key auth path is scope-checked, not org-scoped: the
// synthesiseSharedKeyPrincipal fixed principal already carries the org
// resolved from the portal's own config.
const webhookSubscribersPath = "/api-portal/api/v0.9/webhook-subscribers"

// webhookEventPatterns are the event names platform-api subscribes to.
// Narrower than the portal's full event set because platform-api only acts
// on application + apikey + subscription lifecycle changes. The receiver's
// handler map (webhook/receiver.go:120) already wires application.* to
// handleApplicationCreated/Updated/Deleted; dropping application.* from this
// subscription list left those handlers unreachable, so every portal "new app"
// event sat with no delivery target and Platform API never learned about the
// application that owned the API keys it did receive.
var webhookEventPatterns = []string{"application.*", "apikey.*", "subscription.*"}

// webhookSubscriberBody is the portal's POST /webhook-subscribers request
// shape. Omits optional fields the portal defaults sensibly.
type webhookSubscriberBody struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"displayName"`
	TargetURL   string   `json:"targetUrl"`
	Secret      string   `json:"secret"`
	Events      []string `json:"events"`
	Enabled     bool     `json:"enabled"`
}

// EnsureWebhookSubscriberOnPortal registers (or re-registers via PUT on 409)
// a webhook subscriber row on the portal at handle. The subscriber id is
// fixed (platformAPISubscriberID) so repeat calls across apip-platform-api
// replicas or Create retries converge on the same row via the portal's
// uniqueness constraint. No-op (returns nil) when webhook delivery or the
// auto-seed flag is off - the method is safe to call unconditionally from
// callers that do not themselves check configuration.
func (s *APIPortalService) EnsureWebhookSubscriberOnPortal(ctx context.Context, handle, orgID string) error {
	if !s.webhookCfg.Enabled || !s.webhookCfg.AutoSeedSubscribers {
		return nil
	}
	if s.webhookCfg.ReceiverURL == "" {
		// Should never fire — config validator rejects AutoSeedSubscribers=true
		// without ReceiverURL at boot — but refuse here too so a mis-wired
		// test case fails loudly instead of seeding an unreachable URL.
		return fmt.Errorf("webhook.receiver_url is not configured")
	}
	portal, err := s.portalRepo.GetByHandleAndOrgID(strings.TrimSpace(handle), orgID)
	if err != nil {
		return fmt.Errorf("resolve portal %q: %w", handle, err)
	}
	if portal == nil {
		return apperror.APIPortalNotFound.New()
	}
	if portal.URL == "" {
		return fmt.Errorf("portal %q has no URL", handle)
	}
	authHeader, err := s.AuthHeaderForPortal(ctx, portal.Handle, orgID)
	if err != nil {
		return fmt.Errorf("build shared-key auth header for portal %q: %w", handle, err)
	}

	body := webhookSubscriberBody{
		ID:          platformAPISubscriberID,
		DisplayName: "Platform API",
		TargetURL:   s.webhookCfg.ReceiverURL,
		Secret:      s.webhookCfg.Secret,
		Events:      webhookEventPatterns,
		Enabled:     true,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal webhook subscriber body: %w", err)
	}

	subscribersURL := strings.TrimRight(portal.URL, "/") + webhookSubscribersPath

	// POST first; on 409 the subscriber already exists so switch to PUT with
	// the same body - keeps the row's targetUrl / secret / events in sync
	// with current config if the operator rotated any of them since the
	// original seed.
	status, respBody, err := s.doPortalRequest(ctx, http.MethodPost, subscribersURL, authHeader, raw)
	if err != nil {
		return fmt.Errorf("POST %s: %w", webhookSubscribersPath, err)
	}
	if status >= 200 && status < 300 {
		return nil
	}
	if status != http.StatusConflict {
		return fmt.Errorf("POST %s returned %d: %s", webhookSubscribersPath, status, truncateForLog(respBody))
	}

	putURL := subscribersURL + "/" + platformAPISubscriberID
	status, respBody, err = s.doPortalRequest(ctx, http.MethodPut, putURL, authHeader, raw)
	if err != nil {
		return fmt.Errorf("PUT %s/%s: %w", webhookSubscribersPath, platformAPISubscriberID, err)
	}
	if status >= 200 && status < 300 {
		return nil
	}
	return fmt.Errorf("PUT %s/%s returned %d: %s", webhookSubscribersPath, platformAPISubscriberID, status, truncateForLog(respBody))
}

// doPortalRequest is the shared primitive for the POST/PUT pair above.
// Returns (status, bounded-body, error). A transport error surfaces as an
// error with status=0; non-2xx responses are returned as-is for the caller
// to classify.
func (s *APIPortalService) doPortalRequest(ctx context.Context, method, url, authHeader string, body []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.webhookHTTPClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	// 32 KiB cap matches the webhook receiver's own error-body truncation;
	// a chatty 500 body shouldn't bloat our error.
	limited := io.LimitReader(resp.Body, 32*1024)
	respBody, _ := io.ReadAll(limited)
	return resp.StatusCode, respBody, nil
}

// truncateForLog trims the response body to a bounded prefix suitable for
// embedding in an error string. Keeps 512 bytes to balance debuggability
// with log-size discipline.
func truncateForLog(b []byte) string {
	const max = 512
	if len(b) > max {
		return string(b[:max]) + "...(truncated)"
	}
	return string(b)
}
