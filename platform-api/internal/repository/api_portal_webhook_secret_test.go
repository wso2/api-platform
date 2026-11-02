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
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/vault"
)

// TestAPIPortalWebhookSecretRepo_EncryptionAtRest covers FR-007: the bytes
// written into api_portal_webhook_secrets.secret_enc MUST be ciphertext —
// specifically, the plaintext MUST NOT appear as a substring of the stored
// bytes — and vault.SecretVault.Decrypt MUST round-trip the ciphertext back
// to the original plaintext.
//
// The test exercises the repository against the real vault helper used in
// production so a regression that stops encrypting (plaintext passthrough)
// or that silently writes a known-cleartext envelope would fail the
// substring check.
func TestAPIPortalWebhookSecretRepo_EncryptionAtRest(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-webhook-encryption"
	const portalUUID = "portal-webhook-encryption"
	createTestAPIPortalOrg(t, db, orgUUID)
	portalRepo := NewAPIPortalRepo(db)
	if err := portalRepo.Create(newTestAPIPortal(portalUUID, orgUUID, "encryption-portal")); err != nil {
		t.Fatalf("Create portal: %v", err)
	}

	// A real vault with a deterministic 32-byte key — same shape every test
	// in the repo uses.
	v, err := vault.NewInHouseVault(bytes.Repeat([]byte("t"), 32))
	if err != nil {
		t.Fatalf("NewInHouseVault: %v", err)
	}
	ctx := context.Background()

	const plaintext = "9f3cfbbd7a1e4bbd8f3c5c6d8e4e5a3f9f3cfbbd7a1e4bbd8f3c5c6d8e4e5a3f" // 64-char hex, treated as plaintext here
	ciphertext, err := v.Encrypt(ctx, plaintext)
	if err != nil {
		t.Fatalf("vault Encrypt: %v", err)
	}

	repo := NewAPIPortalWebhookSecretRepo(db)
	if err := repo.Insert(portalUUID, ciphertext); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	got, err := repo.GetByPortalUUID(portalUUID)
	if err != nil {
		t.Fatalf("GetByPortalUUID: %v", err)
	}
	if got == nil {
		t.Fatal("GetByPortalUUID returned nil for just-inserted portal")
	}

	// FR-007: the stored bytes MUST NOT contain the plaintext as a substring.
	// A naive "store the hex string" regression would make this fail.
	if bytes.Contains(got.SecretEnc, []byte(plaintext)) {
		t.Fatal("secret_enc contains the plaintext as a substring — encryption at rest is broken")
	}

	// Vault round-trip MUST recover the original plaintext.
	roundTripped, err := v.Decrypt(ctx, got.SecretEnc)
	if err != nil {
		t.Fatalf("vault Decrypt: %v", err)
	}
	if roundTripped != plaintext {
		t.Fatalf("round-trip plaintext = %q, want %q", roundTripped, plaintext)
	}
}

// TestAPIPortalWebhookSecretRepo_CascadeOnPortalDelete covers FR-018: when a
// portal row is deleted, its api_portal_webhook_secrets row MUST be removed
// too (the FOREIGN KEY ... ON DELETE CASCADE edge in the schema).
//
// The repository has no explicit Delete method on the secret row — the
// FR-018 claim rests on the schema cascade, so this test deletes the portal
// and asserts the sibling row is gone.
func TestAPIPortalWebhookSecretRepo_CascadeOnPortalDelete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-webhook-cascade"
	const portalUUID = "portal-webhook-cascade"
	createTestAPIPortalOrg(t, db, orgUUID)
	portalRepo := NewAPIPortalRepo(db)
	if err := portalRepo.Create(newTestAPIPortal(portalUUID, orgUUID, "cascade-portal")); err != nil {
		t.Fatalf("Create portal: %v", err)
	}

	secretRepo := NewAPIPortalWebhookSecretRepo(db)
	if err := secretRepo.Insert(portalUUID, []byte("ciphertext-placeholder")); err != nil {
		t.Fatalf("Insert secret: %v", err)
	}
	if err := secretRepo.Rotate(portalUUID, []byte("ciphertext-rotated"), time.Now().UTC().Add(10*time.Minute)); err != nil {
		t.Fatalf("Rotate secret: %v", err)
	}

	// Sanity: the secret row exists before the portal delete.
	before, err := secretRepo.GetByPortalUUID(portalUUID)
	if err != nil || before == nil {
		t.Fatalf("sanity: secret row missing before portal delete (err=%v row=%v)", err, before)
	}

	if err := portalRepo.Delete(portalUUID, orgUUID); err != nil {
		t.Fatalf("Delete portal: %v", err)
	}

	after, err := secretRepo.GetByPortalUUID(portalUUID)
	if err != nil {
		t.Fatalf("GetByPortalUUID after cascade: %v", err)
	}
	if after != nil {
		t.Fatalf("FR-018: expected secret row to cascade-delete with the parent portal, got %+v", after)
	}
}

func newWebhookSecretTestPortal(t *testing.T, orgUUID, portalUUID string) (APIPortalRepository, APIPortalWebhookSecretRepository, func()) {
	t.Helper()
	db, cleanup := setupTestDB(t)
	createTestAPIPortalOrg(t, db, orgUUID)
	portalRepo := NewAPIPortalRepo(db)
	if err := portalRepo.Create(newTestAPIPortal(portalUUID, orgUUID, portalUUID+"-handle")); err != nil {
		cleanup()
		t.Fatalf("Create portal: %v", err)
	}
	return portalRepo, NewAPIPortalWebhookSecretRepo(db), cleanup
}

func TestAPIPortalRepo_GetByUUIDAny(t *testing.T) {
	portalRepo, _, cleanup := newWebhookSecretTestPortal(t, "org-any", "portal-any")
	defer cleanup()

	got, err := portalRepo.GetByUUIDAny("portal-any")
	if err != nil {
		t.Fatalf("GetByUUIDAny: %v", err)
	}
	if got == nil || got.ID != "portal-any" || got.OrganizationID != "org-any" {
		t.Fatalf("GetByUUIDAny = %+v, want portal-any in org-any", got)
	}

	missing, err := portalRepo.GetByUUIDAny("no-such-portal")
	if err != nil {
		t.Fatalf("GetByUUIDAny(missing): %v", err)
	}
	if missing != nil {
		t.Fatalf("GetByUUIDAny(missing) = %+v, want nil", missing)
	}
}

func TestAPIPortalWebhookSecretRepo_MissingRowIsNil(t *testing.T) {
	_, repo, cleanup := newWebhookSecretTestPortal(t, "org-missing", "portal-missing")
	defer cleanup()

	got, err := repo.GetByPortalUUID("portal-missing")
	if err != nil {
		t.Fatalf("GetByPortalUUID: %v", err)
	}
	if got != nil {
		t.Fatalf("GetByPortalUUID = %+v, want nil before any secret is stored", got)
	}
}

// TestAPIPortalWebhookSecretRepo_DuplicateInsertIsUniqueViolation pins what the
// service relies on to settle two concurrent first calls: the losing Insert
// fails with an error IsUniqueViolation recognises.
func TestAPIPortalWebhookSecretRepo_DuplicateInsertIsUniqueViolation(t *testing.T) {
	_, repo, cleanup := newWebhookSecretTestPortal(t, "org-dup", "portal-dup")
	defer cleanup()

	if err := repo.Insert("portal-dup", []byte("first")); err != nil {
		t.Fatalf("first Insert: %v", err)
	}
	err := repo.Insert("portal-dup", []byte("second"))
	if err == nil {
		t.Fatal("second Insert succeeded, want a primary-key violation")
	}
	if !IsUniqueViolation(err) {
		t.Fatalf("IsUniqueViolation(%v) = false, want true", err)
	}
}

func TestAPIPortalWebhookSecretRepo_RotateAndClearExpiredPrevious(t *testing.T) {
	_, repo, cleanup := newWebhookSecretTestPortal(t, "org-rot", "portal-rot")
	defer cleanup()

	if err := repo.Insert("portal-rot", []byte("old")); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	validUntil := time.Now().UTC().Add(10 * time.Minute).Truncate(time.Second)
	if err := repo.Rotate("portal-rot", []byte("new"), validUntil); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	row, err := repo.GetByPortalUUID("portal-rot")
	if err != nil || row == nil {
		t.Fatalf("GetByPortalUUID after Rotate: row=%v err=%v", row, err)
	}
	if string(row.SecretEnc) != "new" || string(row.PreviousSecretEnc) != "old" {
		t.Fatalf("after Rotate current=%q previous=%q, want new/old", row.SecretEnc, row.PreviousSecretEnc)
	}
	if !row.PreviousValidUntil.Valid || !row.PreviousValidUntil.Time.Equal(validUntil) {
		t.Fatalf("PreviousValidUntil = %+v, want %v", row.PreviousValidUntil, validUntil)
	}
	if !row.RotatedAt.Valid {
		t.Fatal("RotatedAt not set by Rotate")
	}

	// Inside the grace window the previous slot stays.
	if err := repo.ClearExpiredPrevious("portal-rot", validUntil.Add(-time.Minute)); err != nil {
		t.Fatalf("ClearExpiredPrevious (inside grace): %v", err)
	}
	if row, _ = repo.GetByPortalUUID("portal-rot"); len(row.PreviousSecretEnc) == 0 {
		t.Fatal("previous secret cleared inside its grace window")
	}

	// At or past the deadline it is cleared; the current secret is untouched.
	if err := repo.ClearExpiredPrevious("portal-rot", validUntil.Add(time.Second)); err != nil {
		t.Fatalf("ClearExpiredPrevious (after grace): %v", err)
	}
	row, _ = repo.GetByPortalUUID("portal-rot")
	if len(row.PreviousSecretEnc) != 0 || row.PreviousValidUntil.Valid {
		t.Fatalf("previous slot after grace = %q / %+v, want cleared", row.PreviousSecretEnc, row.PreviousValidUntil)
	}
	if string(row.SecretEnc) != "new" {
		t.Fatalf("current secret = %q after cleanup, want new", row.SecretEnc)
	}
}
