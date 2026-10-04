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

package service

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// fakeSARepo is an in-memory ServiceAccountRepository.
type fakeSARepo struct {
	byUUID  map[string]*model.ServiceAccount
	revoked []*model.ServiceAccountRevocation
}

func newFakeSARepo() *fakeSARepo { return &fakeSARepo{byUUID: map[string]*model.ServiceAccount{}} }

func (f *fakeSARepo) Create(sa *model.ServiceAccount, _ string) error {
	for _, e := range f.byUUID {
		if e.OrganizationID == sa.OrganizationID && e.Handle == sa.Handle {
			return apperror.ServiceAccountExists.New()
		}
	}
	c := *sa
	f.byUUID[sa.UUID] = &c
	return nil
}
func (f *fakeSARepo) find(match func(*model.ServiceAccount) bool) (*model.ServiceAccount, error) {
	for _, e := range f.byUUID {
		if match(e) {
			c := *e
			return &c, nil
		}
	}
	return nil, apperror.ServiceAccountNotFound.New()
}
func (f *fakeSARepo) GetByHandle(orgID, handle string) (*model.ServiceAccount, error) {
	return f.find(func(e *model.ServiceAccount) bool { return e.OrganizationID == orgID && e.Handle == handle })
}
func (f *fakeSARepo) GetByClientID(id string) (*model.ServiceAccount, error) {
	return f.find(func(e *model.ServiceAccount) bool { return e.ClientID == id })
}
func (f *fakeSARepo) List(string, int, int) ([]*model.ServiceAccount, error) { return nil, nil }
func (f *fakeSARepo) Count(string) (int, error)                              { return 0, nil }

// check mirrors the real repository's conditional writes.
func (f *fakeSARepo) check(uuid string, prevVersion int64, prevStatus string) error {
	cur, ok := f.byUUID[uuid]
	if !ok {
		return apperror.ServiceAccountNotFound.New()
	}
	if cur.TokenVersion != prevVersion || (prevStatus != "" && cur.Status != prevStatus) {
		return apperror.Conflict.New()
	}
	return nil
}
func (f *fakeSARepo) Update(sa *model.ServiceAccount, prevVersion int64, prevStatus string, rev *model.ServiceAccountRevocation) error {
	if err := f.check(sa.UUID, prevVersion, prevStatus); err != nil {
		return err
	}
	c := *sa
	f.byUUID[sa.UUID] = &c
	f.note(rev)
	return nil
}
func (f *fakeSARepo) UpdateSecret(sa *model.ServiceAccount, prevVersion int64, rev *model.ServiceAccountRevocation) error {
	return f.Update(sa, prevVersion, "", rev)
}
func (f *fakeSARepo) Delete(_, uuid string, prevVersion int64, rev *model.ServiceAccountRevocation) error {
	if err := f.check(uuid, prevVersion, ""); err != nil {
		return err
	}
	delete(f.byUUID, uuid)
	f.note(rev)
	return nil
}
func (f *fakeSARepo) TouchLastUsed(string, time.Time, string) error { return nil }
func (f *fakeSARepo) ForeignReservedIdentities() ([]string, error)  { return nil, nil }
func (f *fakeSARepo) note(rev *model.ServiceAccountRevocation) {
	if rev != nil {
		f.revoked = append(f.revoked, rev)
	}
}

type fakeOrgRepo struct {
	repository.OrganizationRepository
}

func (fakeOrgRepo) GetOrganizationByUUID(id string) (*model.Organization, error) {
	return &model.Organization{ID: id, Handle: "acme", Name: "Acme", IdpOrganizationRefUUID: "idp-" + id}, nil
}

type fakeAudit struct {
	actions []string
	err     error
}

func (a *fakeAudit) Record(action, _, _, _, _ string) error {
	a.actions = append(a.actions, action)
	return a.err
}

type saFixture struct {
	svc   *ServiceAccountService
	repo  *fakeSARepo
	audit *fakeAudit
	logs  *bytes.Buffer
	key   *rsa.PrivateKey
}

func newSAFixture(t *testing.T) *saFixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Server{}
	cfg.Auth.ServiceAccount = config.ServiceAccount{TokenTTL: 15 * time.Minute, Audience: "platform-api"}
	signer := NewSATokenSigner(&ServiceAccountKeys{Issuer: "platform-api", PrivateKey: key, Current: &key.PublicKey}, cfg)

	f := &saFixture{repo: newFakeSARepo(), audit: &fakeAudit{}, logs: &bytes.Buffer{}, key: key}
	logger := slog.New(slog.NewTextHandler(f.logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	roles := map[string][]string{"ap_operator": {"ap:gateway:read", "ap:rest_api:read"}, "ap_reader": {"ap:rest_api:read"}}
	f.svc = NewServiceAccountService(f.repo, fakeOrgRepo{}, f.audit, newTestIdentityService(), roles, signer,
		15*time.Minute, logger)
	return f
}

func createReq(handle string) *api.ServiceAccountCreateRequest {
	return &api.ServiceAccountCreateRequest{Id: handle, DisplayName: "CI", Owner: "team", Description: "deploys", Roles: []string{"ap_operator"}}
}

func TestServiceAccountCreate_SecretFormat(t *testing.T) {
	f := newSAFixture(t)
	creds, err := f.svc.Create("org-1", "admin", createReq("ci-bot"))
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^apsa_[0-9a-f]{64}$`).MatchString(creds.ClientSecret) || len(creds.ClientSecret) != 69 {
		t.Fatalf("secret format: %d chars", len(creds.ClientSecret))
	}
	if !regexp.MustCompile(`^sa_acme_ci-bot_[0-9a-f]{6}$`).MatchString(creds.ClientId) {
		t.Fatalf("client ID format: %q", creds.ClientId)
	}
	stored, _ := f.repo.GetByHandle("org-1", "ci-bot")
	if stored.ClientSecretHash != hashServiceAccountSecret(creds.ClientSecret) || strings.Contains(stored.ClientSecretHash, "apsa_") {
		t.Fatal("secret must be stored only as its SHA-256")
	}
	if stored.MaskedSecret != "***"+creds.ClientSecret[64:] || *creds.ServiceAccount.MaskedSecret != stored.MaskedSecret {
		t.Fatalf("masked form: %q", stored.MaskedSecret)
	}
	if stored.IdentityUUID != stored.UUID {
		t.Fatal("identity UUID must be the account UUID")
	}
}

func TestServiceAccountCreate_Validation(t *testing.T) {
	f := newSAFixture(t)
	blankOwner := createReq("a-bot")
	blankOwner.Owner = "  "
	blankDesc := createReq("b-bot")
	blankDesc.Description = ""
	badRole := createReq("c-bot")
	badRole.Roles = []string{"ap_nope"}
	reserved := createReq("token")
	for name, req := range map[string]*api.ServiceAccountCreateRequest{
		"blank owner": blankOwner, "blank description": blankDesc, "unknown role": badRole, "reserved id": reserved,
	} {
		if _, err := f.svc.Create("org-1", "admin", req); !apperror.ValidationFailed.Is(err) {
			t.Errorf("%s: want validation error, got %v", name, err)
		}
	}

	if _, err := f.svc.Create("org-1", "admin", createReq("ci-bot")); err != nil {
		t.Fatal(err)
	}
	blank := "   "
	if _, err := f.svc.Update("org-1", "ci-bot", "admin", &api.ServiceAccountUpdateRequest{Owner: &blank}); !apperror.ValidationFailed.Is(err) {
		t.Fatalf("blanking owner on update: got %v", err)
	}
}

func TestServiceAccountExchange(t *testing.T) {
	f := newSAFixture(t)
	creds, _ := f.svc.Create("org-1", "admin", createReq("ci-bot"))

	resp, err := f.svc.Exchange(ExchangeRequest{ClientID: creds.ClientId, ClientSecret: creds.ClientSecret, ClientIP: "10.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if *resp.Scope != "ap:gateway:read ap:rest_api:read" || resp.ExpiresIn != 900 || resp.TokenType != "Bearer" {
		t.Fatalf("response: %+v", resp)
	}

	claims := jwt.MapClaims{}
	tok, err := jwt.ParseWithClaims(resp.AccessToken, claims, func(*jwt.Token) (interface{}, error) { return &f.key.PublicKey, nil })
	if err != nil {
		t.Fatal(err)
	}
	sa, _ := f.repo.GetByHandle("org-1", "ci-bot")
	want := map[string]interface{}{
		"sub": "sa:acme:ci-bot:" + sa.UUID, "iss": "platform-api", "aud": "platform-api", "azp": creds.ClientId,
		"organization": "idp-org-1", "org_handle": "acme", "scope": "ap:gateway:read ap:rest_api:read",
		"sa_tv": float64(1),
	}
	for k, v := range want {
		if claims[k] != v {
			t.Errorf("claim %s = %v, want %v", k, claims[k], v)
		}
	}
	if jti, _ := claims["jti"].(string); len(jti) != 36 {
		t.Errorf("jti = %q", jti)
	}
	if tok.Header["kid"] == nil {
		t.Error("kid header missing")
	}
	if !strings.Contains(f.logs.String(), "service account token issued") || !strings.Contains(f.logs.String(), "10.0.0.1") {
		t.Error("a successful exchange must be logged with the client IP")
	}
}

func TestServiceAccountExchange_UniformFailure(t *testing.T) {
	f := newSAFixture(t)
	creds, _ := f.svc.Create("org-1", "admin", createReq("ci-bot"))
	unmatched := "sa_pasted_" + creds.ClientSecret

	cases := map[string]ExchangeRequest{
		"unknown client":    {ClientID: unmatched, ClientSecret: creds.ClientSecret},
		"wrong secret":      {ClientID: creds.ClientId, ClientSecret: "apsa_" + strings.Repeat("f", 64)},
		"secret w/o prefix": {ClientID: creds.ClientId, ClientSecret: strings.TrimPrefix(creds.ClientSecret, "apsa_")},
	}
	var first *apperror.Error
	for name, req := range cases {
		_, err := f.svc.Exchange(req)
		var appErr *apperror.Error
		if !errors.As(err, &appErr) || !apperror.Unauthorized.Is(err) {
			t.Fatalf("%s: want the uniform 401, got %v", name, err)
		}
		if first == nil {
			first = appErr
		} else if appErr.Message != first.Message || appErr.Code != first.Code {
			t.Fatalf("%s: 401 differs from another cause", name)
		}
	}

	disabled := api.ServiceAccountUpdateRequestStatusDisabled
	if _, err := f.svc.Update("org-1", "ci-bot", "admin", &api.ServiceAccountUpdateRequest{Status: &disabled}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Exchange(ExchangeRequest{ClientID: creds.ClientId, ClientSecret: creds.ClientSecret}); !apperror.Unauthorized.Is(err) {
		t.Fatalf("disabled account: got %v", err)
	}

	logs := f.logs.String()
	if strings.Contains(logs, unmatched) || strings.Contains(logs, creds.ClientSecret) {
		t.Fatal("a secret or an unmatched client ID reached the log")
	}
}

func TestServiceAccountLifecycle_AuditAndRevocation(t *testing.T) {
	f := newSAFixture(t)
	f.svc.Create("org-1", "admin", createReq("ci-bot")) //nolint:errcheck

	disabled, active := api.ServiceAccountUpdateRequestStatusDisabled, api.ServiceAccountUpdateRequestStatusActive
	roles := []string{"ap_reader"}
	steps := []func() error{
		func() error {
			_, err := f.svc.Update("org-1", "ci-bot", "admin", &api.ServiceAccountUpdateRequest{Roles: &roles})
			return err
		},
		func() error {
			_, err := f.svc.Update("org-1", "ci-bot", "admin", &api.ServiceAccountUpdateRequest{Status: &disabled})
			return err
		},
		func() error {
			_, err := f.svc.Update("org-1", "ci-bot", "admin", &api.ServiceAccountUpdateRequest{Status: &active})
			return err
		},
		func() error { _, err := f.svc.RegenerateSecret("org-1", "ci-bot", "admin"); return err },
		func() error { return f.svc.Delete("org-1", "ci-bot", "admin") },
	}
	for i, step := range steps {
		if err := step(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
	want := "CREATE UPDATE DISABLE ENABLE REGENERATE_SECRET DELETE"
	if got := strings.Join(f.audit.actions, " "); got != want {
		t.Fatalf("audit actions = %q, want %q", got, want)
	}
	// Removing a role, disabling, regenerating and deleting revoke; re-enabling
	// does not. Each revoke raises the watermark to the account's new version.
	var got []int64
	for _, rev := range f.repo.revoked {
		got = append(got, rev.MinTokenVersion)
	}
	if want := []int64{2, 3, 4, 5}; !slices.Equal(got, want) {
		t.Fatalf("revocation watermarks = %v, want %v", got, want)
	}
	if lifetime := time.Until(f.repo.revoked[0].ExpiresAt); lifetime < 29*time.Minute || lifetime > 30*time.Minute {
		t.Fatalf("ledger row lifetime = %v, want two token TTLs", lifetime)
	}
}

// A token minted right after a revoke, with the new credentials, carries the
// new version and so clears the watermark: there is no lockout window.
func TestServiceAccountRegenerate_NewTokenPassesWatermark(t *testing.T) {
	f := newSAFixture(t)
	f.svc.Create("org-1", "admin", createReq("ci-bot")) //nolint:errcheck
	creds, err := f.svc.RegenerateSecret("org-1", "ci-bot", "admin")
	if err != nil {
		t.Fatal(err)
	}
	cache := NewRevocationCache(&fakeLedger{}, quietLogger())
	cache.Load() //nolint:errcheck
	cache.Remember(f.repo.revoked[0])

	resp, err := f.svc.Exchange(ExchangeRequest{ClientID: creds.ClientId, ClientSecret: creds.ClientSecret})
	if err != nil {
		t.Fatal(err)
	}
	claims := jwt.MapClaims{}
	if _, err := jwt.ParseWithClaims(resp.AccessToken, claims, func(*jwt.Token) (interface{}, error) { return &f.key.PublicKey, nil }); err != nil {
		t.Fatal(err)
	}
	sa, _ := f.repo.GetByHandle("org-1", "ci-bot")
	version := int64(claims["sa_tv"].(float64))
	if cache.IsRevoked(sa.UUID, version) {
		t.Fatalf("fresh token (version %d) rejected by its own revoke", version)
	}
	if !cache.IsRevoked(sa.UUID, version-1) {
		t.Fatal("a token from the old secret must be revoked")
	}
}

func TestServiceAccountUpdate_AddingRoleDoesNotRevoke(t *testing.T) {
	f := newSAFixture(t)
	f.svc.Create("org-1", "admin", createReq("ci-bot")) //nolint:errcheck
	roles := []string{"ap_operator", "ap_reader"}
	if _, err := f.svc.Update("org-1", "ci-bot", "admin", &api.ServiceAccountUpdateRequest{Roles: &roles}); err != nil {
		t.Fatal(err)
	}
	if len(f.repo.revoked) != 0 {
		t.Fatalf("adding a role wrote %d revocations", len(f.repo.revoked))
	}
}

func TestServiceAccountAuditFailureIsLogged(t *testing.T) {
	f := newSAFixture(t)
	f.audit.err = errors.New("audit table unavailable")
	if _, err := f.svc.Create("org-1", "admin", createReq("ci-bot")); err != nil {
		t.Fatalf("an audit failure must not fail the write: %v", err)
	}
	if !strings.Contains(f.logs.String(), "level=ERROR") || !strings.Contains(f.logs.String(), "failed to write service account audit row") {
		t.Fatal("a failed audit write must be logged at ERROR")
	}
}

// A re-created handle is a new account with a new identity, never the old one.
func TestServiceAccountRecreateGetsNewIdentity(t *testing.T) {
	f := newSAFixture(t)
	f.svc.Create("org-1", "admin", createReq("ci-bot")) //nolint:errcheck
	first, _ := f.repo.GetByHandle("org-1", "ci-bot")
	if err := f.svc.Delete("org-1", "ci-bot", "admin"); err != nil {
		t.Fatal(err)
	}
	f.svc.Create("org-1", "admin", createReq("ci-bot")) //nolint:errcheck
	second, _ := f.repo.GetByHandle("org-1", "ci-bot")
	if first.IdentityUUID == second.IdentityUUID || first.Subject("acme") == second.Subject("acme") {
		t.Fatal("re-created handle inherited the old identity")
	}
}

func TestAccountUUIDFromSubject(t *testing.T) {
	sa := &model.ServiceAccount{UUID: "0198a1b2-0000-7000-8000-000000000001", Handle: "ci-bot"}
	if got, ok := model.AccountUUIDFromSubject(sa.Subject("acme")); !ok || got != sa.UUID {
		t.Fatalf("round trip: %q, %v", got, ok)
	}
	for _, sub := range []string{"alice", "sa:", "sa:acme:ci-bot:", "user:sa:x"} {
		if _, ok := model.AccountUUIDFromSubject(sub); ok {
			t.Errorf("AccountUUIDFromSubject(%q) must fail", sub)
		}
	}
}
