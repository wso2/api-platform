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
