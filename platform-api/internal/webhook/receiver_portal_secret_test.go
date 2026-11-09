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
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// fakePortalSecrets holds the retained portal secrets per organization UUID and resolves them the
// way the service does: by hash, within one organization.
type fakePortalSecrets map[string][]string

func (f fakePortalSecrets) PortalSecretByHash(_ context.Context, orgID, hash string) (string, error) {
	for _, s := range f[orgID] {
		if SecretHash(s) == hash {
			return s, nil
		}
	}
	return "", nil
}

func (f fakePortalSecrets) PortalSecrets(_ context.Context, orgID string) ([]string, error) {
	return f[orgID], nil
}

func newPortalSecretReceiver(t *testing.T) (*Receiver, config.Webhook) {
	t.Helper()
	cfg := config.Webhook{
		Enabled: true, Secret: "global-secret", SignatureTolerance: 5 * time.Minute,
		MaxBodySize: 1 << 20, SignatureHeader: "X-Api-Portal-Signature", LegacySharedSecret: true,
	}
	orgs := fakeOrgResolver{"org-a": &model.Organization{ID: readOnlyTestOrgA, Handle: "org-a"}}
	r, err := NewReceiver(cfg, nil, nil, nil, nil, nil, orgs, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewReceiver: %v", err)
	}
	r.SetPortalSecrets(fakePortalSecrets{
		readOnlyTestOrgA: {"whsec_newest", "whsec_older"},
		readOnlyTestOrgB: {"whsec_other_org"},
	})
	return r, cfg
}

// deliver posts an org-a event signed with signWith. secretHash is the value the portal puts in
// the envelope ("" omits it). The event type has no handler, so a request that authenticates ends
// in a validation error while one that does not ends in Unauthorized.
func deliver(t *testing.T, r *Receiver, cfg config.Webhook, signWith, secretHash string) error {
	t.Helper()
	signature, body := signedDelivery(signWith, secretHash)
	req := httptest.NewRequest(http.MethodPost, RoutePath, bytes.NewReader(body))
	req.Header.Set(cfg.SignatureHeader, signature)
	return r.ReceiveEvent(httptest.NewRecorder(), req)
}

// signedDelivery returns the signature header value and body of the org-a event deliver posts.
func signedDelivery(signWith, secretHash string) (string, []byte) {
	hashField := ""
	if secretHash != "" {
		hashField = `"secret_hash":"` + secretHash + `",`
	}
	body := []byte(`{"event_id":"evt-1","event_type":"portal.secret.test",` + hashField + `"org":{"ref_id":"org-a"},"data":{}}`)
	return sign(signWith, time.Now().Unix(), body), body
}

func errorCode(err error) string {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

func authenticated(err error) bool { return errorCode(err) != apperror.CodeCommonUnauthorized }

func TestReceiver_SelectsTheSecretNamedByOrgAndSecretHash(t *testing.T) {
	r, cfg := newPortalSecretReceiver(t)

	for name, secret := range map[string]string{
		"newest portal version": "whsec_newest",
		"older portal version":  "whsec_older",
		"global secret":         "global-secret",
	} {
		if err := deliver(t, r, cfg, secret, SecretHash(secret)); !authenticated(err) {
			t.Errorf("%s: must authenticate, got %v", name, err)
		}
	}
}

func TestReceiver_RejectsADeliveryWhoseSecretHashNamesNoHeldSecret(t *testing.T) {
	r, cfg := newPortalSecretReceiver(t)

	cases := map[string]struct{ signWith, hash string }{
		"unknown secret":                   {"whsec_deleted", SecretHash("whsec_deleted")},
		"another org's secret":             {"whsec_other_org", SecretHash("whsec_other_org")},
		"hash of a held secret, wrong key": {"whsec_forged", SecretHash("whsec_newest")},
		"garbage hash":                     {"whsec_newest", "not-a-hash"},
		// The hash names the older version but the signature is by the newer one: the named
		// secret is the only one checked, so another valid secret cannot stand in for it.
		"signed by a different held secret": {"whsec_newest", SecretHash("whsec_older")},
		"global secret key, other hash":     {"global-secret", SecretHash("whsec_older")},
	}
	for name, c := range cases {
		if err := deliver(t, r, cfg, c.signWith, c.hash); authenticated(err) {
			t.Errorf("%s: must be unauthorized, got %v", name, err)
		}
	}
}

func TestReceiver_WithoutSecretHashFallsBackToTheOrgsSecretsThenGlobal(t *testing.T) {
	r, cfg := newPortalSecretReceiver(t)

	for name, secret := range map[string]string{
		"portal secret": "whsec_older",
		"global secret": "global-secret",
	} {
		if err := deliver(t, r, cfg, secret, ""); !authenticated(err) {
			t.Errorf("%s without a hash must still authenticate, got %v", name, err)
		}
	}
	if err := deliver(t, r, cfg, "whsec_other_org", ""); authenticated(err) {
		t.Error("another org's secret must not authenticate without a hash either")
	}
}

func TestReceiver_WithoutSecretHashIsRejectedWhenLegacyIsDisabled(t *testing.T) {
	r, cfg := newPortalSecretReceiver(t)
	r.cfg.LegacySharedSecret = false

	for name, secret := range map[string]string{"portal secret": "whsec_older", "global secret": "global-secret"} {
		if got := errorCode(deliver(t, r, cfg, secret, "")); got != apperror.CodeCommonUnauthorized {
			t.Errorf("%s without a hash must be rejected when legacy is off, got %q", name, got)
		}
	}
	if err := deliver(t, r, cfg, "whsec_older", SecretHash("whsec_older")); !authenticated(err) {
		t.Errorf("a delivery with secret_hash is unaffected by the setting, got %v", err)
	}
}

func TestReceiver_DecryptorFollowsTheSecretThatVerified(t *testing.T) {
	r, _ := newPortalSecretReceiver(t)

	body := []byte(`{"event_id":"e","event_type":"x","secret_hash":"` + SecretHash("whsec_older") + `","org":{"ref_id":"org-a"},"data":{}}`)
	header := sign("whsec_older", time.Now().Unix(), body)
	d, err := r.authenticate(context.Background(), header, body)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	want, _ := NewDecryptor("whsec_older")
	if d == nil || !bytes.Equal(d.key, want.key) {
		t.Fatal("decryptor must be derived from the secret the hash selected")
	}
}

// failingPortalSecrets answers every lookup with a store error.
type failingPortalSecrets struct{ fakePortalSecrets }

func (failingPortalSecrets) PortalSecretByHash(context.Context, string, string) (string, error) {
	return "", errors.New("connection refused")
}

// A store outage is the platform's failure, not the portal's, so it is classified (and logged) as
// a server error rather than a bad signature. The route still answers it like every other failed
// delivery (TestReceiver_EveryFailedDeliveryGetsTheSameResponse).
func TestReceiver_ASecretLookupFailureIsAServerErrorNotAnAuthenticationFailure(t *testing.T) {
	r, cfg := newPortalSecretReceiver(t)
	r.SetPortalSecrets(failingPortalSecrets{})

	err := deliver(t, r, cfg, "whsec_newest", SecretHash("whsec_newest"))
	if got := errorCode(err); got != apperror.CodeCommonInternalError {
		t.Fatalf("lookup failure: want %s, got %q (%v)", apperror.CodeCommonInternalError, got, err)
	}
	// The shared secret never needs the store, so it still authenticates during an outage.
	if err := deliver(t, r, cfg, "global-secret", SecretHash("global-secret")); !authenticated(err) {
		t.Fatalf("global secret must still authenticate: %v", err)
	}
	// An unknown hash is still just an invalid signature.
	r.SetPortalSecrets(fakePortalSecrets{})
	if got := errorCode(deliver(t, r, cfg, "whsec_x", SecretHash("whsec_x"))); got != apperror.CodeCommonUnauthorized {
		t.Fatalf("unknown hash: want %s, got %q", apperror.CodeCommonUnauthorized, got)
	}
}

// Without a secret_hash every retained portal secret is a candidate; the count is bounded so one
// unauthenticated request cannot cost an HMAC per secret of a large organization.
func TestReceiver_BoundsTheSecretsTriedForADeliveryWithoutASecretHash(t *testing.T) {
	r, cfg := newPortalSecretReceiver(t)
	many := make([]string, 0, maxPortalSecretCandidates+5)
	for i := 0; i < maxPortalSecretCandidates+5; i++ {
		many = append(many, "whsec_filler_"+string(rune('a'+i%26))+string(rune('a'+i/26)))
	}
	many = append(many, "whsec_beyond_the_bound")
	r.SetPortalSecrets(fakePortalSecrets{readOnlyTestOrgA: many})

	if got := errorCode(deliver(t, r, cfg, "whsec_beyond_the_bound", "")); got != apperror.CodeCommonUnauthorized {
		t.Fatalf("a secret past the candidate bound must not authenticate, got %q", got)
	}
	if !authenticated(deliver(t, r, cfg, many[0], "")) {
		t.Fatal("a secret within the bound must authenticate")
	}
}

type failingOrgResolver struct{}

func (failingOrgResolver) GetOrganizationByHandle(string) (*model.Organization, error) {
	return nil, errors.New("db down")
}

func TestReceiver_AnOrganizationLookupFailureIsAServerErrorNotAnAuthenticationFailure(t *testing.T) {
	r, cfg := newPortalSecretReceiver(t)
	r.orgs = failingOrgResolver{}

	for name, secret := range map[string]string{"known secret": "whsec_newest", "unknown secret": "whsec_x"} {
		err := deliver(t, r, cfg, secret, SecretHash(secret))
		if got := errorCode(err); got != apperror.CodeCommonInternalError {
			t.Errorf("%s: an org lookup outage must be %s, got %q (%v)", name, apperror.CodeCommonInternalError, got, err)
		}
	}
	// The shared secret never needs the org lookup. ReceiveEvent resolves the org after
	// authenticating, so check authentication itself rather than the delivery's outcome.
	signature, body := signedDelivery("global-secret", SecretHash("global-secret"))
	if _, err := r.authenticate(context.Background(), signature, body); err != nil {
		t.Errorf("global secret must authenticate without the org lookup: %v", err)
	}
}
