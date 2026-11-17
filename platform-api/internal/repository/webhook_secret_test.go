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

package repository

import (
	"errors"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

func newWebhookSecretVersion(tag string) *model.WebhookSecret {
	return &model.WebhookSecret{
		SecretEnc: []byte("enc:" + tag), SecretHash: "hmac-sha256:" + tag,
		MaskedSecret: "whsec_****" + tag, CreatedBy: "user-1",
	}
}

func TestWebhookSecretRepo_RotationLimitAndVersionDelete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	if _, err := db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_at, updated_at)
		VALUES ('org-1', 'org-1', 'Org 1', 'us', 'idp-1', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	repo := NewWebhookSecretRepo(db)

	info := &model.WebhookSecretMetadata{OrganizationID: "org-1", Handle: "portal-a", Name: "Portal A", Type: model.WebhookSecretMetadataTypePortal, CreatedBy: "user-1", UpdatedBy: "user-1"}
	if err := repo.Create(info, newWebhookSecretVersion("v1")); err != nil {
		t.Fatalf("create: %v", err)
	}
	dup := &model.WebhookSecretMetadata{OrganizationID: "org-1", Handle: "portal-a", Name: "Dup", Type: model.WebhookSecretMetadataTypePortal}
	if err := repo.Create(dup, newWebhookSecretVersion("x")); !isAppErr(err, apperror.CodeWebhookSecretExists) {
		t.Fatalf("duplicate handle: want WEBHOOK_SECRET_EXISTS, got %v", err)
	}

	for _, tag := range []string{"v2", "v3"} {
		if err := repo.AddVersion(info.UUID, 3, newWebhookSecretVersion(tag)); err != nil {
			t.Fatalf("rotate %s: %v", tag, err)
		}
	}
	if err := repo.AddVersion(info.UUID, 3, newWebhookSecretVersion("v4")); !isAppErr(err, apperror.CodeWebhookSecretVersionLimit) {
		t.Fatalf("fourth rotation: want WEBHOOK_SECRET_VERSION_LIMIT, got %v", err)
	}
	versions, err := repo.ListVersions(info.UUID)
	if err != nil || len(versions) != 3 || versions[0].Version != 3 || versions[2].Version != 1 {
		t.Fatalf("versions after limit: %v %v", versions, err)
	}

	if err := repo.DeleteVersion(info.UUID, 2, "user-1"); err != nil {
		t.Fatalf("delete version 2: %v", err)
	}
	if err := repo.DeleteVersion(info.UUID, 2, "user-1"); !isAppErr(err, apperror.CodeWebhookSecretNotFound) {
		t.Fatalf("delete missing version: want WEBHOOK_SECRET_NOT_FOUND, got %v", err)
	}
	// A slot is free again; the new version continues the sequence, never reusing 2.
	if err := repo.AddVersion(info.UUID, 3, newWebhookSecretVersion("v4")); err != nil {
		t.Fatalf("rotate after delete: %v", err)
	}
	versions, _ = repo.ListVersions(info.UUID)
	if len(versions) != 3 || versions[0].Version != 4 {
		t.Fatalf("versions after re-rotation: %v", versions)
	}

	for _, v := range []int{1, 3} {
		if err := repo.DeleteVersion(info.UUID, v, "user-1"); err != nil {
			t.Fatalf("delete version %d: %v", v, err)
		}
	}
	if err := repo.DeleteVersion(info.UUID, 4, "user-1"); !isAppErr(err, apperror.CodeWebhookSecretLastVersion) {
		t.Fatalf("delete last version: want WEBHOOK_SECRET_LAST_VERSION, got %v", err)
	}
}

func TestWebhookSecretRepo_OrgScopeAndCascade(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	for _, id := range []string{"org-1", "org-2"} {
		if _, err := db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_at, updated_at)
			VALUES (?, ?, ?, 'us', ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, id, id, id, "idp-"+id); err != nil {
			t.Fatalf("seed org: %v", err)
		}
	}
	repo := NewWebhookSecretRepo(db)
	info := &model.WebhookSecretMetadata{OrganizationID: "org-1", Handle: "portal-a", Name: "Portal A", Type: model.WebhookSecretMetadataTypePortal}
	if err := repo.Create(info, newWebhookSecretVersion("v1")); err != nil {
		t.Fatalf("create: %v", err)
	}

	if vs, err := repo.ListVersionsForOrg("org-1", model.WebhookSecretMetadataTypePortal); err != nil || len(vs) != 1 {
		t.Fatalf("org-1 portal versions: %v %v", vs, err)
	}
	if vs, err := repo.ListVersionsForOrg("org-2", model.WebhookSecretMetadataTypePortal); err != nil || len(vs) != 0 {
		t.Fatalf("org-2 must see no portal versions: %v %v", vs, err)
	}

	byHash, err := repo.GetVersionByHash("org-1", model.WebhookSecretMetadataTypePortal, "hmac-sha256:v1")
	if err != nil || byHash == nil || string(byHash.SecretEnc) != "enc:v1" {
		t.Fatalf("lookup by hash in the owning org: %v %v", byHash, err)
	}
	if other, err := repo.GetVersionByHash("org-2", model.WebhookSecretMetadataTypePortal, "hmac-sha256:v1"); err != nil || other != nil {
		t.Fatalf("the same hash must not resolve in another org: %v %v", other, err)
	}
	if none, err := repo.GetVersionByHash("org-1", model.WebhookSecretMetadataTypePortal, "no-such-hash"); err != nil || none != nil {
		t.Fatalf("an unknown hash must return nothing, not an error: %v %v", none, err)
	}

	if _, err := repo.GetMetadata("org-2", "portal-a"); !isAppErr(err, apperror.CodeWebhookSecretNotFound) {
		t.Fatalf("cross-org get: want not found, got %v", err)
	}
	if err := repo.Delete("org-2", "portal-a"); !isAppErr(err, apperror.CodeWebhookSecretNotFound) {
		t.Fatalf("cross-org delete: want not found, got %v", err)
	}
	if err := repo.Delete("org-1", "portal-a"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM webhook_secrets`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("versions must cascade on delete: count=%d err=%v", n, err)
	}
}

func isAppErr(err error, code string) bool {
	var appErr *apperror.Error
	return errors.As(err, &appErr) && appErr.Code == code
}

func TestWebhookSecretRepo_ConcurrentRotationStaysWithinLimit(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	if _, err := db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_at, updated_at)
		VALUES ('org-1', 'org-1', 'Org 1', 'us', 'idp-1', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	repo := NewWebhookSecretRepo(db)
	info := &model.WebhookSecretMetadata{OrganizationID: "org-1", Handle: "portal-a", Name: "Portal A", Type: model.WebhookSecretMetadataTypePortal}
	if err := repo.Create(info, newWebhookSecretVersion("v1")); err != nil {
		t.Fatalf("create: %v", err)
	}

	const attempts = 8
	results := make(chan error, attempts)
	for i := 0; i < attempts; i++ {
		go func() { results <- repo.AddVersion(info.UUID, 3, newWebhookSecretVersion("c")) }()
	}
	succeeded := 0
	for i := 0; i < attempts; i++ {
		if err := <-results; err == nil {
			succeeded++
		} else if !isAppErr(err, apperror.CodeWebhookSecretVersionLimit) {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if succeeded != 2 {
		t.Fatalf("want exactly 2 successful rotations on top of v1, got %d", succeeded)
	}
}

// Deleting the newest version must not hand its number out again: a delete that still names it
// would otherwise remove the credential issued after it.
func TestWebhookSecretRepo_VersionNumbersAreNeverReused(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	if _, err := db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_at, updated_at)
		VALUES ('org-1', 'org-1', 'Org 1', 'us', 'idp-1', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	repo := NewWebhookSecretRepo(db)
	info := &model.WebhookSecretMetadata{OrganizationID: "org-1", Handle: "portal-a", Name: "Portal A", Type: model.WebhookSecretMetadataTypePortal}
	if err := repo.Create(info, newWebhookSecretVersion("v1")); err != nil {
		t.Fatalf("create: %v", err)
	}
	for _, tag := range []string{"v2", "v3"} {
		if err := repo.AddVersion(info.UUID, 3, newWebhookSecretVersion(tag)); err != nil {
			t.Fatalf("rotate %s: %v", tag, err)
		}
	}
	if err := repo.DeleteVersion(info.UUID, 3, "u"); err != nil {
		t.Fatalf("delete the newest version: %v", err)
	}
	replacement := newWebhookSecretVersion("v4")
	if err := repo.AddVersion(info.UUID, 3, replacement); err != nil {
		t.Fatalf("rotate after deleting the newest: %v", err)
	}
	if replacement.Version != 4 {
		t.Fatalf("the replacement must get a new number, got %d (3 was already issued)", replacement.Version)
	}
	// A late delete aimed at version 3 finds nothing and cannot touch the new credential.
	if err := repo.DeleteVersion(info.UUID, 3, "u"); !isAppErr(err, apperror.CodeWebhookSecretNotFound) {
		t.Fatalf("stale delete: want WEBHOOK_SECRET_NOT_FOUND, got %v", err)
	}
	versions, err := repo.ListVersions(info.UUID)
	if err != nil || len(versions) != 3 || versions[0].Version != 4 {
		t.Fatalf("versions: %v %v", versions, err)
	}
}
