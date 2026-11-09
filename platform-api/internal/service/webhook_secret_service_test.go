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
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

type fakeWebhookSecretRepo struct {
	repository.WebhookSecretRepository
	info     *model.WebhookSecretMetadata
	versions []*model.WebhookSecret
	last     int // like webhook_secrets_metadata.last_version: only grows, so a number is never reused
	// createConflict makes the next Create lose a race: another request creates the secret first.
	createConflict bool
	// lookupErr makes GetVersionByHash fail, as a database outage would.
	lookupErr error
}

func (f *fakeWebhookSecretRepo) Create(info *model.WebhookSecretMetadata, first *model.WebhookSecret) error {
	if f.createConflict {
		f.createConflict = false
		winner := *info
		winner.UUID = "info-1"
		f.info, f.versions, f.last = &winner, nil, 0
		return apperror.WebhookSecretExists.New()
	}
	info.UUID, first.Version = "info-1", 1
	f.info, f.versions, f.last = info, []*model.WebhookSecret{first}, 1
	return nil
}

func (f *fakeWebhookSecretRepo) GetMetadata(orgID, handle string) (*model.WebhookSecretMetadata, error) {
	if f.info == nil || f.info.OrganizationID != orgID || f.info.Handle != handle {
		return nil, apperror.WebhookSecretNotFound.New()
	}
	return f.info, nil
}

func (f *fakeWebhookSecretRepo) AddVersion(_ string, max int, w *model.WebhookSecret) error {
	if len(f.versions) >= max {
		return apperror.WebhookSecretVersionLimit.New()
	}
	f.last++
	w.Version = f.last
	f.versions = append([]*model.WebhookSecret{w}, f.versions...)
	return nil
}

func (f *fakeWebhookSecretRepo) DeleteVersion(_ string, version int, _ string) error {
	for i, v := range f.versions {
		if v.Version == version {
			f.versions = append(f.versions[:i], f.versions[i+1:]...)
			return nil
		}
	}
	return apperror.WebhookSecretNotFound.New()
}

func (f *fakeWebhookSecretRepo) Delete(orgID, handle string) error {
	if f.info == nil || f.info.OrganizationID != orgID || f.info.Handle != handle {
		return apperror.WebhookSecretNotFound.New()
	}
	f.info, f.versions = nil, nil
	return nil
}

func (f *fakeWebhookSecretRepo) GetVersionByHash(orgID, _, hash string) (*model.WebhookSecret, error) {
	if f.lookupErr != nil {
		return nil, f.lookupErr
	}
	if f.info == nil || f.info.OrganizationID != orgID {
		return nil, nil
	}
	for _, v := range f.versions {
		if v.SecretHash == hash {
			return v, nil
		}
	}
	return nil, nil
}

func (f *fakeWebhookSecretRepo) ListVersions(string) ([]*model.WebhookSecret, error) {
	return f.versions, nil
}

func newTestWebhookSecretService(repo repository.WebhookSecretRepository, max int) *WebhookSecretService {
	return NewWebhookSecretService(repo, &mockVault{}, newTestIdentityService(), max)
}

func TestWebhookSecretService_CreateReturnsValueOnceAndStoresOnlyEncrypted(t *testing.T) {
	repo := &fakeWebhookSecretRepo{}
	svc := newTestWebhookSecretService(repo, 0)

	resp, err := svc.Create(context.Background(), "org-1", "user-1", &dto.CreateWebhookSecretRequest{Handle: "portal-a", Name: "Portal A"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !strings.HasPrefix(resp.Secret, "whsec_") || len(resp.Secret) != len("whsec_")+64 {
		t.Fatalf("unexpected secret format: %q", resp.Secret)
	}
	stored := repo.versions[0]
	if string(stored.SecretEnc) == resp.Secret || strings.Contains(stored.MaskedSecret, resp.Secret[6:len(resp.Secret)-4]) {
		t.Fatalf("plaintext must not be stored or fully echoed in the mask: %+v", stored)
	}
	if got := repo.versions[0].MaskedSecret; got != "whsec_****"+resp.Secret[len(resp.Secret)-4:] {
		t.Fatalf("unexpected mask %q", got)
	}

	got, err := svc.Get("org-1", "portal-a")
	if err != nil || got.Versions[0].MaskedSecret == "" {
		t.Fatalf("get: %v %+v", err, got)
	}
}

func TestWebhookSecretService_DefaultLimitIsThree(t *testing.T) {
	repo := &fakeWebhookSecretRepo{}
	svc := newTestWebhookSecretService(repo, 0)
	ctx := context.Background()
	if _, err := svc.Create(ctx, "org-1", "user-1", &dto.CreateWebhookSecretRequest{Handle: "portal-a", Name: "Portal A"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := svc.Rotate(ctx, "org-1", "portal-a", "user-1"); err != nil {
			t.Fatalf("rotate %d: %v", i, err)
		}
	}
	_, err := svc.Rotate(ctx, "org-1", "portal-a", "user-1")
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeWebhookSecretVersionLimit {
		t.Fatalf("fourth version: want WEBHOOK_SECRET_VERSION_LIMIT, got %v", err)
	}

	plain, err := svc.Plaintexts(ctx, "org-1", "portal-a")
	if err != nil || len(plain) != 3 {
		t.Fatalf("every retained version must decrypt: %v %v", plain, err)
	}
}

func TestWebhookSecretService_RejectsBadInputAndUnknownActor(t *testing.T) {
	svc := newTestWebhookSecretService(&fakeWebhookSecretRepo{}, 3)
	ctx := context.Background()
	cases := map[string]struct {
		actor string
		req   dto.CreateWebhookSecretRequest
	}{
		"empty actor fails closed": {"", dto.CreateWebhookSecretRequest{Handle: "portal-a", Name: "n"}},
		"invalid handle":           {"u", dto.CreateWebhookSecretRequest{Handle: "Bad Handle", Name: "n"}},
		"handle over 40 chars":     {"u", dto.CreateWebhookSecretRequest{Handle: strings.Repeat("a", 41), Name: "n"}},
		"missing name":             {"u", dto.CreateWebhookSecretRequest{Handle: "portal-a"}},
		"unknown type":             {"u", dto.CreateWebhookSecretRequest{Handle: "portal-a", Name: "n", Type: "OTHER"}},
	}
	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			if _, err := svc.Create(ctx, "org-1", c.actor, &c.req); err == nil {
				t.Fatal("want error")
			}
		})
	}
	if _, err := svc.Rotate(ctx, "org-1", "portal-a", ""); err == nil {
		t.Fatal("rotate with empty actor must fail")
	}
	if _, err := svc.Rotate(ctx, "org-2", "missing", "u"); err == nil {
		t.Fatal("rotate of unknown secret must fail")
	}
}

func TestWebhookSecretService_IssuePortalSecretStopsAtTheLimitAndKeepsEveryVersion(t *testing.T) {
	repo := &fakeWebhookSecretRepo{}
	svc := newTestWebhookSecretService(repo, 3)
	ctx := context.Background()

	var issued []string
	for i := 0; i < 3; i++ {
		secret, err := svc.IssuePortalSecret(ctx, "org-1", "my-portal")
		if err != nil {
			t.Fatalf("issue %d: %v", i, err)
		}
		if !strings.HasPrefix(secret, "whsec_") {
			t.Fatalf("issue %d: want a whsec_ value, got %q", i, secret)
		}
		issued = append(issued, secret)
	}
	if repo.info.Type != model.WebhookSecretMetadataTypePortal || repo.info.CreatedBy != "" {
		t.Fatalf("provisioned secret must be PORTAL type with no creator: %+v", repo.info)
	}

	// At the limit nothing is dropped: the oldest version may still be needed.
	_, err := svc.IssuePortalSecret(ctx, "org-1", "my-portal")
	if !isAppErrorCode(err, apperror.CodeWebhookSecretVersionLimit) {
		t.Fatalf("issue at the limit: want WEBHOOK_SECRET_VERSION_LIMIT, got %v", err)
	}
	plain, err := svc.Plaintexts(ctx, "org-1", "my-portal")
	if err != nil || len(plain) != 3 {
		t.Fatalf("every version must be kept: %v %v", plain, err)
	}

	// Once an operator frees a slot, issuing works again and the newest value is retained.
	if err := repo.DeleteVersion("info-1", repo.versions[len(repo.versions)-1].Version, ""); err != nil {
		t.Fatalf("delete oldest: %v", err)
	}
	fresh, err := svc.IssuePortalSecret(ctx, "org-1", "my-portal")
	if err != nil {
		t.Fatalf("issue after freeing a slot: %v", err)
	}
	issued = append(issued, fresh)
	plain, err = svc.Plaintexts(ctx, "org-1", "my-portal")
	if err != nil || len(plain) != 3 {
		t.Fatalf("plaintexts: %v %v", plain, err)
	}
	// The retained versions are the last three issued, newest first.
	for i, want := range []string{issued[3], issued[2], issued[1]} {
		if plain[i] != want {
			t.Fatalf("retained version %d: got %q, want %q", i, plain[i], want)
		}
	}
	// A version number is never reused, so a stale delete cannot remove a newer credential.
	if repo.versions[0].Version != 4 {
		t.Fatalf("newest version number: got %d, want 4", repo.versions[0].Version)
	}
}

func TestWebhookSecretService_IssuePortalSecretJoinsAConcurrentCreate(t *testing.T) {
	repo := &fakeWebhookSecretRepo{createConflict: true}
	svc := newTestWebhookSecretService(repo, 3)
	secret, err := svc.IssuePortalSecret(context.Background(), "org-1", "my-portal")
	if err != nil {
		t.Fatalf("losing the create race must fall through to adding a version: %v", err)
	}
	if !strings.HasPrefix(secret, "whsec_") || len(repo.versions) != 1 || repo.versions[0].Version != 1 {
		t.Fatalf("want the one version added to the winner's secret, got %q %+v", secret, repo.versions)
	}
}

type frozenOrgs struct{ frozen map[string]bool }

func (f frozenOrgs) IsReadOnlyOrg(org string) bool { return f.frozen[org] }

func TestWebhookSecretService_PortalSecretChangesAreRefusedForAFrozenOrganization(t *testing.T) {
	repo := &fakeWebhookSecretRepo{}
	svc := newTestWebhookSecretService(repo, 3)
	ctx := context.Background()
	if _, err := svc.IssuePortalSecret(ctx, "org-1", "my-portal"); err != nil {
		t.Fatalf("issue while writable: %v", err)
	}
	svc.readOnly = frozenOrgs{frozen: map[string]bool{"org-1": true}}

	if _, err := svc.IssuePortalSecret(ctx, "org-1", "my-portal"); !isAppErrorCode(err, apperror.CodeCommonOrganizationReadOnly) {
		t.Fatalf("issue for a frozen org: want ORGANIZATION_READ_ONLY, got %v", err)
	}
	if err := svc.DeletePortalSecret(ctx, "org-1", "my-portal"); !isAppErrorCode(err, apperror.CodeCommonOrganizationReadOnly) {
		t.Fatalf("delete for a frozen org: want ORGANIZATION_READ_ONLY, got %v", err)
	}
	if repo.info == nil || len(repo.versions) != 1 {
		t.Fatalf("a refused change must leave the secret untouched: %+v %d", repo.info, len(repo.versions))
	}
	// Another organization is unaffected.
	if _, err := svc.IssuePortalSecret(ctx, "org-2", "other-portal"); err != nil {
		t.Fatalf("a writable org must still issue: %v", err)
	}
}

func TestWebhookSecretService_PortalSecretsSkipsOnlyTheVersionThatCannotBeRead(t *testing.T) {
	repo := &fakeWebhookSecretRepo{}
	svc := NewWebhookSecretService(repo, &mockVault{decryptFn: func(ct []byte) (string, error) {
		if string(ct) == "enc:broken" {
			return "", errors.New("corrupt ciphertext")
		}
		return strings.TrimPrefix(string(ct), "enc:"), nil
	}}, newTestIdentityService(), 3)
	repo.info = &model.WebhookSecretMetadata{UUID: "info-1", OrganizationID: "org-1", Handle: "p", Type: model.WebhookSecretMetadataTypePortal}
	repo.versions = []*model.WebhookSecret{{Version: 3, SecretEnc: []byte("enc:good-new")}, {Version: 2, SecretEnc: []byte("enc:broken")}, {Version: 1, SecretEnc: []byte("enc:good-old")}}
	svc.repo = repoWithOrgVersions{fakeWebhookSecretRepo: repo}

	got, err := svc.PortalSecrets(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("one unreadable version must not fail the list: %v", err)
	}
	if len(got) != 2 || got[0] != "good-new" || got[1] != "good-old" {
		t.Fatalf("want the two readable versions, newest first, got %v", got)
	}
}

// repoWithOrgVersions serves the organization-wide listing from the fake's versions.
type repoWithOrgVersions struct{ *fakeWebhookSecretRepo }

func (r repoWithOrgVersions) ListVersionsForOrg(string, string) ([]*model.WebhookSecret, error) {
	return r.versions, nil
}

func TestWebhookSecretService_DeletePortalSecretIsIdempotent(t *testing.T) {
	repo := &fakeWebhookSecretRepo{}
	svc := newTestWebhookSecretService(repo, 3)
	ctx := context.Background()
	if err := svc.DeletePortalSecret(ctx, "org-1", "my-portal"); err != nil {
		t.Fatalf("delete of absent secret must succeed: %v", err)
	}
	if _, err := svc.IssuePortalSecret(ctx, "org-1", "my-portal"); err != nil {
		t.Fatalf("issue: %v", err)
	}
	if err := svc.DeletePortalSecret(ctx, "org-1", "my-portal"); err != nil || repo.info != nil {
		t.Fatalf("delete: %v (info %v)", err, repo.info)
	}
}

func TestPortalSecretHandle_FitsColumnAndStaysUnique(t *testing.T) {
	if got := portalSecretHandle("short-portal"); got != "short-portal" {
		t.Fatalf("short handles pass through, got %q", got)
	}
	long1 := strings.Repeat("a", 60) + "-one"
	long2 := strings.Repeat("a", 60) + "-two"
	h1, h2 := portalSecretHandle(long1), portalSecretHandle(long2)
	if len(h1) > 40 || len(h2) > 40 || h1 == h2 {
		t.Fatalf("long handles must fit 40 chars and differ: %q %q", h1, h2)
	}
}

func TestWebhookSecretService_StoresThePortalCompatibleHashAndResolvesByIt(t *testing.T) {
	repo := &fakeWebhookSecretRepo{}
	svc := newTestWebhookSecretService(repo, 3)
	ctx := context.Background()

	first, err := svc.Create(ctx, "org-1", "user-1", &dto.CreateWebhookSecretRequest{Handle: "portal-a", Name: "Portal A"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	second, err := svc.Rotate(ctx, "org-1", "portal-a", "user-1")
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}

	// The stored (and API-visible) hash is exactly what the portal sends as secret_hash.
	if got := first.Versions[0].Hash; got != utils.WebhookSecretHash(first.Secret) {
		t.Fatalf("stored hash %q must equal the portal's secret_hash %q", got, utils.WebhookSecretHash(first.Secret))
	}

	for _, want := range []string{first.Secret, second.Secret} {
		got, err := svc.PortalSecretByHash(ctx, "org-1", utils.WebhookSecretHash(want))
		if err != nil || got != want {
			t.Fatalf("lookup by hash: got %q, %v; want the secret it names", got, err)
		}
	}
	if got, err := svc.PortalSecretByHash(ctx, "org-1", utils.WebhookSecretHash("whsec_unknown")); err != nil || got != "" {
		t.Fatalf("an unknown hash must resolve to nothing, got %q, %v", got, err)
	}
	if got, _ := svc.PortalSecretByHash(ctx, "org-2", utils.WebhookSecretHash(first.Secret)); got != "" {
		t.Fatal("a hash must not resolve in another organization")
	}
}

// portalSecretFixture holds one PORTAL secret and counts how often a delivery decrypts it.
func portalSecretFixture(t *testing.T) (*WebhookSecretService, *fakeWebhookSecretRepo, *int) {
	t.Helper()
	repo := &fakeWebhookSecretRepo{}
	decrypts := 0
	svc := NewWebhookSecretService(repo, &mockVault{decryptFn: func(ct []byte) (string, error) {
		decrypts++
		return strings.TrimPrefix(string(ct), "enc:"), nil
	}}, newTestIdentityService(), 3)
	repo.info = &model.WebhookSecretMetadata{UUID: "info-1", OrganizationID: "org-1", Handle: "p", Type: model.WebhookSecretMetadataTypePortal}
	repo.versions = []*model.WebhookSecret{{Version: 1, SecretEnc: []byte("enc:whsec_a"), SecretHash: "h1"}}
	return svc, repo, &decrypts
}

// No plaintext is kept between deliveries: each one reads the stored version and decrypts it, so a
// version deleted anywhere, including on another instance, stops resolving at once.
func TestWebhookSecretService_PortalSecretByHashKeepsNoPlaintextBetweenDeliveries(t *testing.T) {
	svc, repo, decrypts := portalSecretFixture(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if got, err := svc.PortalSecretByHash(ctx, "org-1", "h1"); err != nil || got != "whsec_a" {
			t.Fatalf("lookup %d: %q, %v", i, got, err)
		}
	}
	if *decrypts != 3 {
		t.Fatalf("every delivery must decrypt the stored version, decrypted %d times", *decrypts)
	}

	repo.versions = nil // deleted by another instance
	if got, err := svc.PortalSecretByHash(ctx, "org-1", "h1"); err != nil || got != "" {
		t.Fatalf("a deleted secret must stop resolving at once, got %q, %v", got, err)
	}
}

func TestWebhookSecretService_PortalSecretByHashReturnsAStoreFailure(t *testing.T) {
	svc, repo, _ := portalSecretFixture(t)
	repo.lookupErr = errors.New("db down")
	if got, err := svc.PortalSecretByHash(context.Background(), "org-1", "h1"); err == nil || got != "" {
		t.Fatalf("a store failure must be an error, not a secret: %q, %v", got, err)
	}
}
