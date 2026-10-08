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

// TEMP-READ-ONLY-MODE: this whole file is part of the temporary organization-scoped
// read-only mode used while Bijira migrates from Platform API v1 to v2. Delete it
// when the mode is removed.

package webhook

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

const (
	readOnlyTestOrgA = "11111111-1111-1111-1111-111111111111"
	readOnlyTestOrgB = "22222222-2222-2222-2222-222222222222"
)

// fakeOrgResolver maps API Portal organization handles to platform organizations.
type fakeOrgResolver map[string]*model.Organization

func (f fakeOrgResolver) GetOrganizationByHandle(handle string) (*model.Organization, error) {
	return f[handle], nil
}

// newReadOnlyTestReceiver builds a receiver with nil services: the read-only check
// runs before dispatch, and the writable case below uses an event type no handler
// is registered for, so no service is ever reached.
func newReadOnlyTestReceiver(t *testing.T) (*Receiver, config.Webhook) {
	t.Helper()
	cfg := config.Webhook{
		Enabled:            true,
		Secret:             "read-only-test-secret",
		SignatureTolerance: 5 * time.Minute,
		MaxBodySize:        1 << 20,
		SignatureHeader:    "X-Api-Portal-Signature",
	}
	orgs := fakeOrgResolver{
		"org-a": &model.Organization{ID: readOnlyTestOrgA, Handle: "org-a"},
		"org-b": &model.Organization{ID: readOnlyTestOrgB, Handle: "org-b"},
	}
	r, err := NewReceiver(cfg, nil, nil, nil, orgs, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewReceiver: %v", err)
	}
	r.SetReadOnly(&config.ReadOnly{Enabled: true, WritableOrganizations: []string{readOnlyTestOrgB}})
	return r, cfg
}

func signedEventRequest(cfg config.Webhook, orgHandle string) *http.Request {
	body := []byte(`{"event_id":"evt-1","event_type":"readonly.test.event","org":{"ref_id":"` + orgHandle + `"},"data":{}}`)
	req := httptest.NewRequest(http.MethodPost, RoutePath, bytes.NewReader(body))
	req.Header.Set(cfg.SignatureHeader, sign(cfg.Secret, time.Now().Unix(), body))
	return req
}

func TestReceiver_ReadOnlyRejectsEventsForFrozenOrganization(t *testing.T) {
	r, cfg := newReadOnlyTestReceiver(t)

	rec := httptest.NewRecorder()
	if err := r.ReceiveEvent(rec, signedEventRequest(cfg, "org-a")); err != nil {
		t.Fatalf("a read-only rejection is written directly, not returned; got error %v", err)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (body: %s)", rec.Code, rec.Body.String())
	}
	var body apperror.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	if body.Code != apperror.CodeCommonOrganizationReadOnly {
		t.Errorf("body.code = %q, want %q", body.Code, apperror.CodeCommonOrganizationReadOnly)
	}
}

func TestReceiver_ReadOnlyLetsWritableOrganizationReachDispatch(t *testing.T) {
	r, cfg := newReadOnlyTestReceiver(t)

	rec := httptest.NewRecorder()
	err := r.ReceiveEvent(rec, signedEventRequest(cfg, "org-b"))
	// The event type is deliberately unknown: reaching the "unsupported event
	// type" validation error proves the request got past the read-only check.
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeCommonValidationFailed {
		t.Fatalf("expected the unsupported-event validation error after passing the read-only check, got %v", err)
	}
	if rec.Code == http.StatusServiceUnavailable {
		t.Fatal("writable organization must not be rejected with 503")
	}
}
