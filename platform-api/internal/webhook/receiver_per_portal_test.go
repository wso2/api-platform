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
	perPortalTestPortalUUID = "55555555-5555-5555-5555-555555555555"
	perPortalTestPortalOrg  = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	perPortalTestOtherOrg   = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	perPortalTestOrgHandle  = "portal-org"
	perPortalTestOtherOrgHd = "other-org"
	perPortalTestTolerance  = 5 * time.Minute
	perPortalTestCurrent    = "current-portal-secret-current-portal-secret-00"
	perPortalTestPrevious   = "previous-portal-secret-previous-portal-secret-0"
)

// fakeKeyProvider is a minimal PortalKeyProvider for receiver tests. If
// errOnGet is set it is returned; otherwise the provider returns `keys` for
// any requested portal uuid.
type fakeKeyProvider struct {
	keys     *PortalWebhookKeys
	errOnGet error
}

func (f fakeKeyProvider) Get(_ context.Context, _ string) (*PortalWebhookKeys, error) {
	if f.errOnGet != nil {
		return nil, f.errOnGet
	}
	return f.keys, nil
}

// perPortalReceiverCfg mirrors the production defaults with the per-portal
// path mounted and legacy off; individual tests can flip LegacySharedSecret.
func perPortalReceiverCfg() config.Webhook {
	return config.Webhook{
		Enabled:            true,
		LegacySharedSecret: false,
		RotationGrace:      10 * time.Minute,
		SignatureTolerance: perPortalTestTolerance,
		MaxBodySize:        1 << 20,
		SignatureHeader:    "X-Api-Portal-Signature",
	}
}

// buildPortalKeys constructs a PortalWebhookKeys with the specified secret
// strings and optional previous-slot grace window.
func buildPortalKeys(t *testing.T, currentSecret, previousSecret string, previousValidUntil time.Time) *PortalWebhookKeys {
	t.Helper()
	currentDec, err := NewDecryptor(currentSecret)
	if err != nil {
		t.Fatalf("NewDecryptor(current): %v", err)
	}
	keys := &PortalWebhookKeys{
		PortalUUID:       perPortalTestPortalUUID,
		OrganizationUUID: perPortalTestPortalOrg,
		Current:          NewVerifier(currentSecret, perPortalTestTolerance),
		CurrentDecryptor: currentDec,
	}
	if previousSecret != "" {
		prevDec, err := NewDecryptor(previousSecret)
		if err != nil {
			t.Fatalf("NewDecryptor(previous): %v", err)
		}
		keys.Previous = NewVerifier(previousSecret, perPortalTestTolerance)
		keys.PreviousDecryptor = prevDec
		keys.PreviousValidUntil = previousValidUntil
	}
	return keys
}

// newPerPortalReceiver builds a receiver whose per-portal path uses the given
// PortalKeyProvider. apiKeys/subs/apps are nil: these tests all stop at the
// "unsupported event type" dispatch step, which runs before any downstream
// service is called.
func newPerPortalReceiver(t *testing.T, cfg config.Webhook, prov PortalKeyProvider) *Receiver {
	t.Helper()
	orgs := fakeOrgResolver{
		perPortalTestOrgHandle:  &model.Organization{ID: perPortalTestPortalOrg, Handle: perPortalTestOrgHandle},
		perPortalTestOtherOrgHd: &model.Organization{ID: perPortalTestOtherOrg, Handle: perPortalTestOtherOrgHd},
	}
	r, err := NewReceiver(cfg, prov, nil, nil, nil, orgs, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewReceiver: %v", err)
	}
	return r
}

// signedPerPortalRequest builds a request to the per-portal path with the body
// signed by `secret`. r.PathValue("portalUUID") is populated via SetPathValue
// since the test calls the handler directly rather than going through the
// mux.
func signedPerPortalRequest(cfg config.Webhook, secret, orgHandle string) *http.Request {
	body := []byte(`{"event_id":"evt-1","event_type":"per_portal.test.event","org":{"ref_id":"` + orgHandle + `"},"data":{}}`)
	path := strings.Replace(PerPortalRoutePath, "{portalUUID}", perPortalTestPortalUUID, 1)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.SetPathValue("portalUUID", perPortalTestPortalUUID)
	req.Header.Set(cfg.SignatureHeader, sign(secret, time.Now().Unix(), body))
	return req
}

func TestReceiver_PerPortalHappyPathReachesDispatch(t *testing.T) {
	cfg := perPortalReceiverCfg()
	keys := buildPortalKeys(t, perPortalTestCurrent, "", time.Time{})
	r := newPerPortalReceiver(t, cfg, fakeKeyProvider{keys: keys})

	rec := httptest.NewRecorder()
	err := r.ReceivePerPortalEvent(rec, signedPerPortalRequest(cfg, perPortalTestCurrent, perPortalTestOrgHandle))
	// Deliberately unknown event type: reaching the "unsupported event type"
	// validation error proves verify + org-binding + dispatch lookup all passed.
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeCommonValidationFailed {
		t.Fatalf("expected validation error at dispatch, got %v (status rec: %d)", err, rec.Code)
	}
}

func TestReceiver_PerPortalCrossOrgReturns403(t *testing.T) {
	cfg := perPortalReceiverCfg()
	keys := buildPortalKeys(t, perPortalTestCurrent, "", time.Time{})
	r := newPerPortalReceiver(t, cfg, fakeKeyProvider{keys: keys})

	rec := httptest.NewRecorder()
	err := r.ReceivePerPortalEvent(rec, signedPerPortalRequest(cfg, perPortalTestCurrent, perPortalTestOtherOrgHd))
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected apperror, got %v", err)
	}
	if appErr.Code != apperror.CodeCommonForbidden {
		t.Fatalf("expected forbidden code, got %q", appErr.Code)
	}
}

func TestReceiver_PerPortalUnresolvedOrgHandleReturns403(t *testing.T) {
	// FR-004: a payload org.ref_id handle that does not resolve to any known
	// organisation MUST be treated as a mismatch — the per-portal receiver
	// returns the same 403 as an explicit cross-org binding failure, not a 404
	// resource-not-found that would be an existence oracle for organisation
	// handles. Legacy ReceiveEvent keeps the 404 passthrough; this behaviour
	// is specific to the per-portal path.
	cfg := perPortalReceiverCfg()
	keys := buildPortalKeys(t, perPortalTestCurrent, "", time.Time{})
	r := newPerPortalReceiver(t, cfg, fakeKeyProvider{keys: keys})

	rec := httptest.NewRecorder()
	// "ghost-org" is deliberately not in fakeOrgResolver (which only knows
	// perPortalTestOrgHandle and perPortalTestOtherOrgHd), so resolveOrgUUID
	// returns apperror.OrganizationNotFound and the receiver converts it to
	// Forbidden.
	err := r.ReceivePerPortalEvent(rec, signedPerPortalRequest(cfg, perPortalTestCurrent, "ghost-org"))
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected apperror, got %v", err)
	}
	if appErr.Code != apperror.CodeCommonForbidden {
		t.Fatalf("expected forbidden code (same 403 as cross-org mismatch, not a 404 org-not-found), got %q", appErr.Code)
	}
}

func TestReceiver_PerPortalUnknownPortalReturns401(t *testing.T) {
	cfg := perPortalReceiverCfg()
	// Provider returns an APIPortalNotFound error — the registry maps unknown
	// portal uuids and missing-secret portals both to this.
	prov := fakeKeyProvider{errOnGet: apperror.APIPortalNotFound.New()}
	r := newPerPortalReceiver(t, cfg, prov)

	rec := httptest.NewRecorder()
	err := r.ReceivePerPortalEvent(rec, signedPerPortalRequest(cfg, perPortalTestCurrent, perPortalTestOrgHandle))
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected apperror, got %v", err)
	}
	if appErr.Code != apperror.CodeCommonUnauthorized {
		t.Fatalf("expected generic unauthorized (no portal-existence oracle), got %q", appErr.Code)
	}
}

func TestReceiver_PerPortalInvalidEnvelopeReturns400(t *testing.T) {
	// FR-021: a syntactically invalid body or a body missing the mandatory
	// top-level org.ref_id MUST be rejected with ValidationFailed (HTTP 400)
	// AFTER the HMAC verifies — the envelope schema is a public contract so
	// surfacing shape errors here does not leak tenant state. The order matters:
	// HMAC verification runs first (step 5 in receiver-route.md §Request flow),
	// then envelope decode / Validate (step 6).
	cfg := perPortalReceiverCfg()
	keys := buildPortalKeys(t, perPortalTestCurrent, "", time.Time{})
	r := newPerPortalReceiver(t, cfg, fakeKeyProvider{keys: keys})

	buildReq := func(body []byte) *http.Request {
		path := strings.Replace(PerPortalRoutePath, "{portalUUID}", perPortalTestPortalUUID, 1)
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
		req.SetPathValue("portalUUID", perPortalTestPortalUUID)
		req.Header.Set(cfg.SignatureHeader, sign(perPortalTestCurrent, time.Now().Unix(), body))
		return req
	}

	cases := []struct {
		name string
		body []byte
	}{
		{"syntactically invalid JSON", []byte(`{not-json`)},
		{"missing org.ref_id", []byte(`{"event_id":"evt-1","event_type":"per_portal.test.event","data":{}}`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			err := r.ReceivePerPortalEvent(rec, buildReq(tc.body))
			var appErr *apperror.Error
			if !errors.As(err, &appErr) {
				t.Fatalf("expected apperror, got %v", err)
			}
			if appErr.Code != apperror.CodeCommonValidationFailed {
				t.Fatalf("expected validation-failed (400 for envelope-shape failure, tenant-neutral), got %q", appErr.Code)
			}
		})
	}
}

func TestReceiver_PerPortalBadSignatureReturns401(t *testing.T) {
	cfg := perPortalReceiverCfg()
	keys := buildPortalKeys(t, perPortalTestCurrent, "", time.Time{})
	r := newPerPortalReceiver(t, cfg, fakeKeyProvider{keys: keys})

	// Sign with a different secret — current verifier should refuse, no previous.
	rec := httptest.NewRecorder()
	err := r.ReceivePerPortalEvent(rec, signedPerPortalRequest(cfg, "wrong-secret", perPortalTestOrgHandle))
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeCommonUnauthorized {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}

func TestReceiver_PerPortalPreviousSecretAcceptedWithinGrace(t *testing.T) {
	cfg := perPortalReceiverCfg()
	keys := buildPortalKeys(t, perPortalTestCurrent, perPortalTestPrevious, time.Now().Add(5*time.Minute))
	r := newPerPortalReceiver(t, cfg, fakeKeyProvider{keys: keys})

	// Sign with the previous secret — current verifier rejects, previous
	// verifier accepts because the grace is still live. Flow proceeds to
	// dispatch and lands on "unsupported event type".
	rec := httptest.NewRecorder()
	err := r.ReceivePerPortalEvent(rec, signedPerPortalRequest(cfg, perPortalTestPrevious, perPortalTestOrgHandle))
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeCommonValidationFailed {
		t.Fatalf("expected to reach dispatch via previous-secret grace, got %v", err)
	}
}

func TestReceiver_PerPortalPreviousSecretRejectedAfterGrace(t *testing.T) {
	cfg := perPortalReceiverCfg()
	keys := buildPortalKeys(t, perPortalTestCurrent, perPortalTestPrevious, time.Now().Add(-time.Minute))
	r := newPerPortalReceiver(t, cfg, fakeKeyProvider{keys: keys})

	rec := httptest.NewRecorder()
	err := r.ReceivePerPortalEvent(rec, signedPerPortalRequest(cfg, perPortalTestPrevious, perPortalTestOrgHandle))
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeCommonUnauthorized {
		t.Fatalf("expected unauthorized once grace expires, got %v", err)
	}
}

func TestReceiver_PerPortalRouteNotMountedWhenRegistryNil(t *testing.T) {
	cfg := perPortalReceiverCfg()
	cfg.LegacySharedSecret = true
	cfg.Secret = "legacy-only"
	// Without a keyRegistry, RegisterRoutes should mount only the legacy path.
	r, err := NewReceiver(cfg, nil, nil, nil, nil, fakeOrgResolver{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewReceiver: %v", err)
	}
	mux := http.NewServeMux()
	r.RegisterRoutes(mux)

	path := strings.Replace(PerPortalRoutePath, "{portalUUID}", perPortalTestPortalUUID, 1)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte("{}")))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("per-portal path should not be mounted without a keyRegistry; got status %d", rec.Code)
	}
}

func TestReceiver_LegacyRouteAcceptsValidSignatureWhenFlagOn(t *testing.T) {
	// FR-014: when LegacySharedSecret=true AND Secret!="", the legacy
	// /webhook/events route stays mounted and verifies signatures with the
	// configured global secret. This is the "no deployment breaks on upgrade"
	// guarantee for existing portals still signing with the shared secret.
	// Reaching the "unsupported event type" validation error (dispatcher does
	// not know "legacy.coexistence.test") proves the request passed HMAC
	// verification, envelope decode, and org resolution on the legacy route.
	const legacySecret = "legacy-coexistence-test-secret"
	cfg := perPortalReceiverCfg()
	cfg.LegacySharedSecret = true
	cfg.Secret = legacySecret
	cfg.Enabled = true
	orgs := fakeOrgResolver{
		perPortalTestOrgHandle: &model.Organization{ID: perPortalTestPortalOrg, Handle: perPortalTestOrgHandle},
	}
	r, err := NewReceiver(cfg, nil, nil, nil, nil, orgs, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewReceiver: %v", err)
	}

	body := []byte(`{"event_id":"evt-1","event_type":"legacy.coexistence.test","org":{"ref_id":"` + perPortalTestOrgHandle + `"},"data":{}}`)
	req := httptest.NewRequest(http.MethodPost, RoutePath, bytes.NewReader(body))
	req.Header.Set(cfg.SignatureHeader, sign(legacySecret, time.Now().Unix(), body))
	rec := httptest.NewRecorder()

	errExp := r.ReceiveEvent(rec, req)
	var appErr *apperror.Error
	if !errors.As(errExp, &appErr) || appErr.Code != apperror.CodeCommonValidationFailed {
		t.Fatalf("expected to reach dispatch (unsupported event type) on legacy route, got %v", errExp)
	}
}

func TestReceiver_LegacyRouteNotMountedWhenFlagOff(t *testing.T) {
	cfg := perPortalReceiverCfg() // LegacySharedSecret=false, Secret=""
	keys := buildPortalKeys(t, perPortalTestCurrent, "", time.Time{})
	r := newPerPortalReceiver(t, cfg, fakeKeyProvider{keys: keys})

	mux := http.NewServeMux()
	r.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, RoutePath, bytes.NewReader([]byte("{}")))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("legacy path should be mounted only when LegacySharedSecret && Secret!=\"\"; got status %d", rec.Code)
	}
}
