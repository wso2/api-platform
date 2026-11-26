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

package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

const (
	uniformTestOrgID     = "33333333-3333-3333-3333-333333333333"
	uniformTestFailEvent = "uniform.test.fail"
	uniformTestPanic     = "uniform.test.panic"
)

// uniformOrgs resolves "org-known" and "org-store-down", fails the lookup for "org-broken", and
// knows no other handle.
type uniformOrgs struct{}

func (uniformOrgs) GetOrganizationByHandle(handle string) (*model.Organization, error) {
	switch handle {
	case "org-known", "org-store-down":
		return &model.Organization{ID: uniformTestOrgID, Handle: handle}, nil
	case "org-broken":
		return nil, errors.New("organization store unavailable")
	}
	return nil, nil
}

func newUniformTestMux(t *testing.T) (*http.ServeMux, config.Webhook) {
	t.Helper()
	cfg := config.Webhook{
		Enabled:            true,
		Secret:             "uniform-test-secret",
		SignatureTolerance: 5 * time.Minute,
		MaxBodySize:        1 << 12,
		SignatureHeader:    "X-Api-Portal-Signature",
	}
	r, err := NewReceiver(cfg, nil, nil, nil, nil, nil, uniformOrgs{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewReceiver: %v", err)
	}
	r.SetPortalSecrets(failingPortalSecrets{})
	r.handlers[uniformTestFailEvent] = func(context.Context, *Envelope) error {
		return errors.New("subscription store unavailable")
	}
	r.handlers[uniformTestPanic] = func(context.Context, *Envelope) error {
		panic("handler bug")
	}
	mux := http.NewServeMux()
	r.RegisterRoutes(mux)
	return mux, cfg
}

func uniformEventBody(eventID, eventType, orgHandle, secretHash string) []byte {
	return []byte(`{"event_id":"` + eventID + `","event_type":"` + eventType + `","org":{"ref_id":"` + orgHandle +
		`"},"secret_hash":"` + secretHash + `","data":{}}`)
}

// TestReceiver_EveryFailedDeliveryGetsTheSameResponse pins that the route answers a failure at any
// stage with the same status and body, so a caller cannot tell which check it failed or why.
func TestReceiver_EveryFailedDeliveryGetsTheSameResponse(t *testing.T) {
	mux, cfg := newUniformTestMux(t)
	globalHash := SecretHash(cfg.Secret)
	signed := func(body []byte) string { return sign(cfg.Secret, time.Now().Unix(), body) }

	cases := []struct {
		name   string
		body   []byte
		header func(body []byte) string
	}{
		{"no signature", uniformEventBody("e1", EventAPIKeyRevoked, "org-known", globalHash),
			func([]byte) string { return "" }},
		{"wrong signature", uniformEventBody("e1", EventAPIKeyRevoked, "org-known", globalHash),
			func(b []byte) string { return sign("not-the-secret", time.Now().Unix(), b) }},
		{"expired signature", uniformEventBody("e1", EventAPIKeyRevoked, "org-known", globalHash),
			func(b []byte) string { return sign(cfg.Secret, time.Now().Add(-time.Hour).Unix(), b) }},
		{"unknown secret hash", uniformEventBody("e1", EventAPIKeyRevoked, "org-unknown", strings.Repeat("0", 64)), signed},
		{"secret store outage", uniformEventBody("e1", EventAPIKeyRevoked, "org-store-down", strings.Repeat("0", 64)), signed},
		{"hash-less delivery", uniformEventBody("e1", EventAPIKeyRevoked, "org-known", ""), signed},
		{"unreadable body", []byte(`not json`), signed},
		{"body too large", bytes.Repeat([]byte("x"), int(cfg.MaxBodySize)+1), signed},
		{"invalid envelope", uniformEventBody("", EventAPIKeyRevoked, "org-known", globalHash), signed},
		{"unknown organization", uniformEventBody("e1", EventAPIKeyRevoked, "org-unknown", globalHash), signed},
		{"organization store outage", uniformEventBody("e1", EventAPIKeyRevoked, "org-broken", globalHash), signed},
		{"unsupported event type", uniformEventBody("e1", "no.such.event", "org-known", globalHash), signed},
		{"invalid event data", uniformEventBody("e1", EventAPIKeyRevoked, "org-known", globalHash), signed},
		{"internal handler failure", uniformEventBody("e1", uniformTestFailEvent, "org-known", globalHash), signed},
		{"handler panic", uniformEventBody("e1", uniformTestPanic, "org-known", globalHash), signed},
	}

	var wantBody string
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, RoutePath, bytes.NewReader(tc.body))
			if h := tc.header(tc.body); h != "" {
				req.Header.Set(cfg.SignatureHeader, h)
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401 (body: %s)", rec.Code, rec.Body.String())
			}
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body %q: %v", rec.Body.String(), err)
			}
			if body["code"] != apperror.CodeCommonUnauthorized {
				t.Errorf("code = %v, want %s", body["code"], apperror.CodeCommonUnauthorized)
			}
			if _, ok := body["trackingId"]; ok {
				t.Errorf("a per-response tracking id would make responses distinguishable: %s", rec.Body.String())
			}
			if wantBody == "" {
				wantBody = rec.Body.String()
			} else if got := rec.Body.String(); got != wantBody {
				t.Errorf("body = %s, want the same body as every other failure: %s", got, wantBody)
			}
		})
	}
}
