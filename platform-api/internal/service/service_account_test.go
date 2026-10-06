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
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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
func (f *fakeSARepo) List(orgID, search string, limit, offset int) ([]*model.ServiceAccount, error) {
	term := strings.ToLower(strings.TrimSpace(search))
	var out []*model.ServiceAccount
	for _, e := range f.byUUID {
		if e.OrganizationID != orgID {
			continue
		}
		if term != "" && !strings.Contains(strings.ToLower(e.DisplayName+" "+e.Handle), term) {
			continue
		}
		c := *e
		out = append(out, &c)
	}
	// Sorted only to page deterministically; tests must not rely on the order.
	slices.SortFunc(out, func(a, b *model.ServiceAccount) int { return strings.Compare(a.Handle, b.Handle) })
	return out[min(offset, len(out)):min(offset+limit, len(out))], nil
}
func (f *fakeSARepo) Count(orgID, search string) (int, error) {
	all, _ := f.List(orgID, search, len(f.byUUID), 0)
	return len(all), nil
}

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

func newSAFixture(t *testing.T) *saFixture { return newSAFixtureMode(t, config.AuthzModeScope) }

func newSAFixtureMode(t *testing.T, mode string) *saFixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Server{}
	cfg.Auth.ServiceAccount = config.ServiceAccount{TokenTTL: 15 * time.Minute, Audience: "platform-api"}
	cfg.Auth.Authorization.Mode = mode
	signer := NewSATokenSigner(&ServiceAccountKeys{Issuer: "platform-api", PrivateKey: key, Current: &key.PublicKey}, cfg)

	f := &saFixture{repo: newFakeSARepo(), audit: &fakeAudit{}, logs: &bytes.Buffer{}, key: key}
	logger := slog.New(slog.NewTextHandler(f.logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	roles := map[string][]string{
		"ap_sa_operator": {"ap:gateway:read", "ap:rest_api:read"},
		"ap_sa_reader":   {"ap:rest_api:read"},
		"ap_operator":    {"ap:gateway:read"},
	}
	f.svc = NewServiceAccountService(f.repo, fakeOrgRepo{}, f.audit, newTestIdentityService(), roles, signer,
		15*time.Minute, mode, config.ClaimMappings{}, logger)
	return f
}

func createReq(handle string) *api.ServiceAccountCreateRequest {
	return &api.ServiceAccountCreateRequest{Id: handle, DisplayName: "CI", Description: ptr("deploys"), Roles: []string{"ap_sa_operator"}}
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
}

func TestServiceAccountCreate_Validation(t *testing.T) {
	f := newSAFixture(t)
	badRole := createReq("c-bot")
	badRole.Roles = []string{"ap_sa_nope"}
	noRole := createReq("e-bot")
	noRole.Roles = nil
	reserved := createReq("token")
	longDesc := createReq("g-bot")
	longDesc.Description = ptr(strings.Repeat("d", 1024))
	for name, req := range map[string]*api.ServiceAccountCreateRequest{
		"description too long": longDesc, "unknown role": badRole, "no role": noRole, "reserved id": reserved,
	} {
		if _, err := f.svc.Create("org-1", "admin", req); !apperror.ValidationFailed.Is(err) {
			t.Errorf("%s: want validation error, got %v", name, err)
		}
	}

	if _, err := f.svc.Create("org-1", "admin", createReq("ci-bot")); err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("n", 256)
	if _, err := f.svc.Update("org-1", "ci-bot", "admin", &api.ServiceAccountUpdateRequest{DisplayName: &long}); !apperror.ValidationFailed.Is(err) {
		t.Fatalf("overlong display name on update: got %v", err)
	}
}

// Only id, displayName and roles are required; a description may be empty.
func TestServiceAccountOptionalDescription(t *testing.T) {
	f := newSAFixture(t)
	req := createReq("ci-bot")
	req.Description = nil
	creds, err := f.svc.Create("org-1", "creator", req)
	if err != nil || creds.ServiceAccount.Description != "" {
		t.Fatalf("create without description: %+v, %v", creds, err)
	}
	got, err := f.svc.Update("org-1", "ci-bot", "editor", &api.ServiceAccountUpdateRequest{Description: ptr("deploys")})
	if err != nil || got.Description != "deploys" {
		t.Fatalf("set description: %+v, %v", got, err)
	}
}

func TestServiceAccountExchange(t *testing.T) {
	f := newSAFixture(t)
	creds, _ := f.svc.Create("org-1", "admin", createReq("ci-bot"))

	resp, err := f.svc.Exchange(ExchangeRequest{ClientID: creds.ClientId, ClientSecret: creds.ClientSecret,
		Scope: "ap:rest_api:read ap:gateway:read ap:rest_api:read", ClientIP: "10.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if *resp.Scope != "ap:rest_api:read ap:gateway:read" || resp.ExpiresIn != 900 || resp.TokenType != "Bearer" {
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
		"organization": "idp-org-1", "org_handle": "acme", "scope": "ap:rest_api:read ap:gateway:read",
		"sa_tv": float64(1),
	}
	for k, v := range want {
		if claims[k] != v {
			t.Errorf("claim %s = %v, want %v", k, claims[k], v)
		}
	}
	if _, ok := claims["roles"]; ok {
		t.Error("a scope-mode token must not carry roles")
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
		"unknown client":     {ClientID: unmatched, ClientSecret: creds.ClientSecret},
		"wrong secret":       {ClientID: creds.ClientId, ClientSecret: "apsa_" + strings.Repeat("f", 64)},
		"bad secret + scope": {ClientID: creds.ClientId, ClientSecret: "apsa_" + strings.Repeat("f", 64), Scope: "ap:nope:read"},
		"secret w/o prefix":  {ClientID: creds.ClientId, ClientSecret: strings.TrimPrefix(creds.ClientSecret, "apsa_")},
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
	roles := []string{"ap_sa_reader"}
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

	resp, err := f.svc.Exchange(ExchangeRequest{ClientID: creds.ClientId, ClientSecret: creds.ClientSecret, Scope: "ap:rest_api:read"})
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
	roles := []string{"ap_sa_operator", "ap_sa_reader", "ap_sa_reader"}
	resp, err := f.svc.Update("org-1", "ci-bot", "admin", &api.ServiceAccountUpdateRequest{Roles: &roles})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.repo.revoked) != 0 {
		t.Fatalf("adding a role wrote %d revocations", len(f.repo.revoked))
	}
	if !slices.Equal(resp.Roles, []string{"ap_sa_operator", "ap_sa_reader"}) {
		t.Fatalf("roles = %v, want deduplicated", resp.Roles)
	}
	// Any role in the mapping is accepted, not only service-account ones.
	mixed := []string{"ap_sa_operator", "ap_operator"}
	if _, err := f.svc.Update("org-1", "ci-bot", "admin", &api.ServiceAccountUpdateRequest{Roles: &mixed}); err != nil {
		t.Fatalf("a person's role on update: got %v", err)
	}
}

func TestServiceAccountExchange_ScopeMode(t *testing.T) {
	f := newSAFixture(t)
	creds, _ := f.svc.Create("org-1", "admin", createReq("ci-bot"))
	for name, scope := range map[string]string{
		"no scope":        "",
		"blank scope":     "   ",
		"ungranted scope": "ap:rest_api:read ap:rest_api:manage",
		"unknown scope":   "ap:nope:read",
	} {
		_, err := f.svc.Exchange(ExchangeRequest{ClientID: creds.ClientId, ClientSecret: creds.ClientSecret, Scope: scope})
		if !apperror.ServiceAccountInvalidScope.Is(err) {
			t.Errorf("%s: want 400 invalid scope, got %v", name, err)
		}
	}
	resp, err := f.svc.Exchange(ExchangeRequest{ClientID: creds.ClientId, ClientSecret: creds.ClientSecret, Scope: "ap:gateway:read"})
	if err != nil {
		t.Fatal(err)
	}
	claims := jwt.MapClaims{}
	if _, err := jwt.ParseWithClaims(resp.AccessToken, claims, func(*jwt.Token) (interface{}, error) { return &f.key.PublicKey, nil }); err != nil {
		t.Fatal(err)
	}
	if claims["scope"] != "ap:gateway:read" || f.svc.TokenScope(claims) != "ap:gateway:read" {
		t.Fatalf("token scope = %v, want exactly the request", claims["scope"])
	}
}

// In scope mode the request may draw on any of the account's roles.
func TestServiceAccountExchange_ScopeModeAcrossRoles(t *testing.T) {
	f := newSAFixture(t)
	req := createReq("ci-bot")
	req.Roles = []string{"ap_sa_reader", "ap_sa_extra"}
	f.svc.roleScopeMap["ap_sa_extra"] = []string{"ap:project:read"}
	creds, err := f.svc.Create("org-1", "admin", req)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := f.svc.Exchange(ExchangeRequest{ClientID: creds.ClientId, ClientSecret: creds.ClientSecret,
		Scope: "ap:project:read ap:rest_api:read"})
	if err != nil || *resp.Scope != "ap:project:read ap:rest_api:read" {
		t.Fatalf("scope across roles: %v, %v", resp, err)
	}
}

func TestServiceAccountExchange_RoleMode(t *testing.T) {
	f := newSAFixtureMode(t, config.AuthzModeRole)
	creds, _ := f.svc.Create("org-1", "admin", createReq("ci-bot"))
	for _, scope := range []string{"", "ap:nope:read"} {
		resp, err := f.svc.Exchange(ExchangeRequest{ClientID: creds.ClientId, ClientSecret: creds.ClientSecret, Scope: scope})
		if err != nil {
			t.Fatalf("scope %q: role mode must ignore it, got %v", scope, err)
		}
		if *resp.Scope != "ap:gateway:read ap:rest_api:read" {
			t.Fatalf("response scope = %q, want the roles' scopes", *resp.Scope)
		}
		claims := jwt.MapClaims{}
		if _, err := jwt.ParseWithClaims(resp.AccessToken, claims, func(*jwt.Token) (interface{}, error) { return &f.key.PublicKey, nil }); err != nil {
			t.Fatal(err)
		}
		if _, ok := claims["scope"]; ok {
			t.Fatal("a role-mode token must not carry scope")
		}
		if roles, _ := claims["roles"].([]any); len(roles) != 1 || roles[0] != "ap_sa_operator" {
			t.Fatalf("roles claim = %v", claims["roles"])
		}
		if got := f.svc.TokenScope(claims); got != "ap:gateway:read ap:rest_api:read" {
			t.Fatalf("TokenScope = %q", got)
		}
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
	if first.UUID == second.UUID || first.Subject("acme") == second.Subject("acme") {
		t.Fatal("re-created handle inherited the old identity")
	}
}

func TestServiceAccountListAndGet(t *testing.T) {
	f := newSAFixture(t)
	f.svc.identity = NewIdentityService(renamingIdentityRepo{})
	for _, h := range []string{"a-bot", "b-bot", "c-bot"} {
		if _, err := f.svc.Create("org-1", "admin", createReq(h)); err != nil {
			t.Fatal(err)
		}
	}
	page, err := f.svc.List("org-1", "", 2, 0)
	if err != nil || page.Count != 2 || page.Pagination.Total != 3 || page.Pagination.Limit != 2 {
		t.Fatalf("first page: %+v, %v", page, err)
	}
	if *page.List[0].CreatedBy != "resolved:admin" || *page.List[0].UpdatedBy != "resolved:admin" {
		t.Fatalf("audit fields not resolved: %q", *page.List[0].CreatedBy)
	}
	rest, _ := f.svc.List("org-1", "", 2, 2)
	seen := map[string]bool{}
	for _, sa := range append(page.List, rest.List...) {
		seen[sa.Id] = true
	}
	if rest.Count != 1 || len(seen) != 3 {
		t.Fatalf("pages overlap or miss an account: %v", seen)
	}
	if page, _ := f.svc.List("org-2", "", 10, 0); page.Count != 0 || page.Pagination.Total != 0 {
		t.Fatalf("another org sees accounts: %+v", page)
	}

	got, err := f.svc.Get("org-1", "b-bot")
	if err != nil || got.Id != "b-bot" || got.Roles[0] != "ap_sa_operator" || *got.CreatedBy != "resolved:admin" {
		t.Fatalf("get: %+v, %v", got, err)
	}
	if _, err := f.svc.Get("org-2", "b-bot"); !apperror.ServiceAccountNotFound.Is(err) {
		t.Fatalf("get from another org: %v", err)
	}
}

// renamingIdentityRepo resolves every UUID to a value unlike the input, so a
// missing resolve shows.
type renamingIdentityRepo struct{ passthroughIdentityRepo }

func (renamingIdentityRepo) GetSubByUUID(uuid string) (string, bool, error) {
	return "resolved:" + uuid, true, nil
}

func (renamingIdentityRepo) GetSubsByUUIDs(uuids []string) (map[string]string, error) {
	out := map[string]string{}
	for _, id := range uuids {
		out[id] = "resolved:" + id
	}
	return out, nil
}

// With a cache attached, a revoke on this replica applies before the next poll.
func TestServiceAccountRevokeAppliesToLocalCache(t *testing.T) {
	f := newSAFixture(t)
	cache := NewRevocationCache(&fakeLedger{}, quietLogger())
	if err := cache.Load(); err != nil {
		t.Fatal(err)
	}
	f.svc.SetRevocationCache(cache)
	f.svc.Create("org-1", "admin", createReq("ci-bot")) //nolint:errcheck
	sa, _ := f.repo.GetByHandle("org-1", "ci-bot")

	disabled := api.ServiceAccountUpdateRequestStatusDisabled
	if _, err := f.svc.Update("org-1", "ci-bot", "admin", &api.ServiceAccountUpdateRequest{Status: &disabled}); err != nil {
		t.Fatal(err)
	}
	if !cache.IsRevoked(sa.UUID, 1) || cache.IsRevoked(sa.UUID, 2) {
		t.Fatal("disable must raise the local watermark to version 2 at once")
	}
}

func writeKeyPair(t *testing.T, dir, name string) (privFile, pubFile string, key *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	privFile, pubFile = filepath.Join(dir, name+".key"), filepath.Join(dir, name+".pub")
	if err := os.WriteFile(privFile, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pubFile, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	return privFile, pubFile, key
}

func TestLoadServiceAccountKeys(t *testing.T) {
	dir := t.TempDir()
	sharedPriv, _, shared := writeKeyPair(t, dir, "shared")
	ownPriv, ownPub, own := writeKeyPair(t, dir, "own")
	_, retiredPub, retired := writeKeyPair(t, dir, "retired")
	missing := filepath.Join(dir, "missing.key")

	t.Run("shared auth.jwt key", func(t *testing.T) {
		cfg := &config.Server{}
		cfg.Auth.JWT = config.JWT{Issuer: "platform-api", PrivateKeyFile: sharedPriv}
		keys, err := LoadServiceAccountKeys(cfg)
		if err != nil || keys.OwnIssuer || keys.Issuer != "platform-api" || !keys.Current.Equal(&shared.PublicKey) {
			t.Fatalf("keys = %+v, %v", keys, err)
		}
		if pubs := keys.PublicKeys(); len(pubs) != 1 {
			t.Fatalf("public keys = %d, want 1", len(pubs))
		}
	})
	t.Run("no private key", func(t *testing.T) {
		if _, err := LoadServiceAccountKeys(&config.Server{}); !errors.Is(err, ErrNoServiceAccountSigningKey) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("unreadable shared key", func(t *testing.T) {
		cfg := &config.Server{}
		cfg.Auth.JWT = config.JWT{PrivateKeyFile: missing}
		if _, err := LoadServiceAccountKeys(cfg); !errors.Is(err, ErrNoServiceAccountSigningKey) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("own key with a retired key", func(t *testing.T) {
		cfg := &config.Server{}
		cfg.Auth.JWT = config.JWT{Issuer: "platform-api", PrivateKeyFile: sharedPriv}
		cfg.Auth.ServiceAccount.JWT = config.JWT{Issuer: "platform-api-sa", PrivateKeyFile: ownPriv, PublicKeyFile: ownPub}
		cfg.Auth.ServiceAccount.RetiredPublicKeyFiles = []string{retiredPub}
		keys, err := LoadServiceAccountKeys(cfg)
		if err != nil || !keys.OwnIssuer || keys.Issuer != "platform-api-sa" {
			t.Fatalf("keys = %+v, %v", keys, err)
		}
		pubs := keys.PublicKeys()
		if len(pubs) != 2 || !pubs[0].Equal(&own.PublicKey) || !pubs[1].Equal(&retired.PublicKey) {
			t.Fatal("PublicKeys must list the current key first, then the retired ones")
		}
	})
	t.Run("unreadable own key is not the soft error", func(t *testing.T) {
		cfg := &config.Server{}
		cfg.Auth.ServiceAccount.JWT = config.JWT{Issuer: "platform-api-sa", PrivateKeyFile: missing}
		if _, err := LoadServiceAccountKeys(cfg); err == nil || errors.Is(err, ErrNoServiceAccountSigningKey) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("unreadable retired key", func(t *testing.T) {
		cfg := &config.Server{}
		cfg.Auth.ServiceAccount.JWT = config.JWT{Issuer: "platform-api-sa", PrivateKeyFile: ownPriv}
		cfg.Auth.ServiceAccount.RetiredPublicKeyFiles = []string{missing}
		if _, err := LoadServiceAccountKeys(cfg); err == nil {
			t.Fatal("a missing retired key must fail startup")
		}
	})
}

// errLookupRepo fails every client ID lookup, as a database outage would.
type errLookupRepo struct{ *fakeSARepo }

func (errLookupRepo) GetByClientID(string) (*model.ServiceAccount, error) {
	return nil, errors.New("connection refused")
}

// A lookup error and a missing signing key look like any other failed exchange.
func TestServiceAccountExchange_InternalFailuresAreUniform(t *testing.T) {
	f := newSAFixture(t)
	creds, _ := f.svc.Create("org-1", "admin", createReq("ci-bot"))
	req := ExchangeRequest{ClientID: creds.ClientId, ClientSecret: creds.ClientSecret, Scope: "ap:rest_api:read"}

	noSigner := NewServiceAccountService(f.repo, fakeOrgRepo{}, f.audit, newTestIdentityService(),
		map[string][]string{"ap_sa_operator": {"ap:rest_api:read"}}, nil, time.Minute, config.AuthzModeScope, config.ClaimMappings{}, quietLogger())
	if _, err := noSigner.Exchange(req); !apperror.Unauthorized.Is(err) {
		t.Fatalf("no signing key: got %v", err)
	}
	lookupFails := NewServiceAccountService(errLookupRepo{f.repo}, fakeOrgRepo{}, f.audit, newTestIdentityService(),
		nil, nil, time.Minute, config.AuthzModeScope, config.ClaimMappings{}, quietLogger())
	if _, err := lookupFails.Exchange(req); !apperror.Unauthorized.Is(err) {
		t.Fatalf("lookup error: got %v", err)
	}
}

func TestServiceAccountCreateAndUpdate_MoreValidation(t *testing.T) {
	f := newSAFixture(t)
	if _, err := f.svc.Create("org-1", "admin", createReq("Not A Handle!")); !apperror.ValidationFailed.Is(err) {
		t.Fatal("invalid handle accepted")
	}
	if _, err := f.svc.Create("org-1", "admin", createReq("ci-bot")); err != nil {
		t.Fatal(err)
	}
	bogus := api.ServiceAccountUpdateRequestStatus("paused")
	if _, err := f.svc.Update("org-1", "ci-bot", "admin", &api.ServiceAccountUpdateRequest{Status: &bogus}); !apperror.ValidationFailed.Is(err) {
		t.Fatalf("unknown status: got %v", err)
	}
	desc := "  nightly deploys  "
	got, err := f.svc.Update("org-1", "ci-bot", "admin", &api.ServiceAccountUpdateRequest{Description: &desc})
	if err != nil || got.Description != "nightly deploys" {
		t.Fatalf("description update: %+v, %v", got, err)
	}
	if _, err := f.svc.Update("org-1", "no-such-bot", "admin", &api.ServiceAccountUpdateRequest{Description: &desc}); !apperror.ServiceAccountNotFound.Is(err) {
		t.Fatalf("update of a missing account: got %v", err)
	}
	if err := f.svc.Delete("org-1", "no-such-bot", "admin"); !apperror.ServiceAccountNotFound.Is(err) {
		t.Fatalf("delete of a missing account: got %v", err)
	}
	if _, err := f.svc.RegenerateSecret("org-1", "no-such-bot", "admin"); !apperror.ServiceAccountNotFound.Is(err) {
		t.Fatalf("regenerate of a missing account: got %v", err)
	}
}

// The roles column is VARCHAR(1023); a longer list is a 400, not a DB error.
func TestServiceAccountCreate_RolesTooLong(t *testing.T) {
	mapping := map[string][]string{}
	var roles []string
	for i := 0; i < 60; i++ {
		role := fmt.Sprintf("ap_sa_role_%010d", i)
		mapping[role] = []string{"ap:rest_api:read"}
		roles = append(roles, role)
	}
	f := newSAFixture(t)
	svc := NewServiceAccountService(f.repo, fakeOrgRepo{}, f.audit, newTestIdentityService(), mapping, nil,
		time.Minute, config.AuthzModeScope, config.ClaimMappings{}, quietLogger())
	req := createReq("ci-bot")
	req.Roles = roles
	if _, err := svc.Create("org-1", "admin", req); !apperror.ValidationFailed.Is(err) {
		t.Fatalf("roles over 1023 bytes: got %v", err)
	}
}

func TestToServiceAccountAPI_LastUsedIP(t *testing.T) {
	if got := toServiceAccountAPI(&model.ServiceAccount{LastUsedIP: "203.0.113.7"}); got.LastUsedIp == nil || *got.LastUsedIp != "203.0.113.7" {
		t.Fatalf("lastUsedIp = %v", got.LastUsedIp)
	}
	if got := toServiceAccountAPI(&model.ServiceAccount{}); got.LastUsedIp != nil {
		t.Fatal("an unused account must have no lastUsedIp")
	}
}

func TestServiceAccountRoles_AllRolesSorted(t *testing.T) {
	f := newSAFixture(t)
	resp := f.svc.Roles()
	var names []string
	for _, r := range resp.List {
		names = append(names, r.Name)
	}
	if want := []string{"ap_operator", "ap_sa_operator", "ap_sa_reader"}; !slices.Equal(names, want) {
		t.Fatalf("roles = %v, want %v", names, want)
	}
	if resp.Count != 3 || resp.Pagination.Total != 3 {
		t.Fatalf("count = %d, total = %d, want 3", resp.Count, resp.Pagination.Total)
	}
	if !slices.Equal(resp.List[1].Scopes, []string{"ap:gateway:read", "ap:rest_api:read"}) {
		t.Fatalf("ap_sa_operator scopes = %v", resp.List[1].Scopes)
	}
	resp.List[1].Scopes[0] = "changed"
	if f.svc.roleScopeMap["ap_sa_operator"][0] != "ap:gateway:read" {
		t.Fatal("Roles exposed the shared role map")
	}
}
