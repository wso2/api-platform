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
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

const (
	testAPIHandle = "petstore"
	testAPIUUID   = "api-uuid-1"
	testOrgID     = "org-1"
	testUserID    = "user-1"
)

// generatedKeyRegex matches the key material utils.GenerateAPIKey produces:
// 32 crypto/rand bytes hex-encoded, i.e. exactly 64 lowercase hex characters.
var generatedKeyRegex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// stubCreateArtifactRepo resolves any handle/kind to one fixed API.
type stubCreateArtifactRepo struct {
	repository.ArtifactRepository
	meta *model.APIMetadata
}

func (r *stubCreateArtifactRepo) GetAPIMetadataByHandleAndKind(handle, kind, orgUUID string) (*model.APIMetadata, error) {
	return r.meta, nil
}

// stubCreateAPIRepo reports no gateway associations, which is a valid state: the key is
// still persisted and any gateway associated later picks it up via the deploy-time backfill.
type stubCreateAPIRepo struct {
	repository.APIRepository
}

func (r *stubCreateAPIRepo) GetAPIGatewaysWithDetails(apiUUID, orgUUID string) ([]*model.APIGatewayWithDetails, error) {
	return nil, nil
}

// recordingAPIKeyRepo captures what Create persisted and lets a test pre-seed names so the
// collision-retry path in resolveUniqueKeyName can be exercised.
type recordingAPIKeyRepo struct {
	repository.APIKeyRepository
	existing map[string]*model.APIKey
	created  []*model.APIKey
}

func (r *recordingAPIKeyRepo) Create(key *model.APIKey) error {
	r.created = append(r.created, key)
	return nil
}

func (r *recordingAPIKeyRepo) GetByArtifactAndName(artifactUUID, name string) (*model.APIKey, error) {
	return r.existing[name], nil
}

type stubCreateAuditRepo struct {
	repository.AuditRepository
}

func (stubCreateAuditRepo) Record(action, resourceUUID, resourceType, orgUUID, performedBy string) error {
	return nil
}

// newCreateAPIKeyTestService wires an APIKeyService against in-memory stubs and returns it
// alongside the key repo, so a test can assert on what was actually persisted.
func newCreateAPIKeyTestService(t *testing.T, existing map[string]*model.APIKey) (*APIKeyService, *recordingAPIKeyRepo) {
	t.Helper()

	if existing == nil {
		existing = map[string]*model.APIKey{}
	}
	keyRepo := &recordingAPIKeyRepo{existing: existing}

	svc := NewAPIKeyService(
		&stubCreateAPIRepo{},
		&stubCreateArtifactRepo{meta: &model.APIMetadata{
			ID:             testAPIUUID,
			Handle:         testAPIHandle,
			Kind:           constants.RestApi,
			OrganizationID: testOrgID,
		}},
		keyRepo,
		NewGatewayEventsService(&capturingEventHub{}, newTestIdentityService(), newTestLogger()),
		stubCreateAuditRepo{},
		nil, // defaults to [sha256]
		newTestLogger(),
	)
	return svc, keyRepo
}

// sha256Hex is an independent hash implementation, so the assertions below verify the stored
// digest rather than re-deriving it through the same helper the service used.
func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// assertPersistedKeyMaterial checks the single persisted record hashes and masks plainKey.
func assertPersistedKeyMaterial(t *testing.T, keyRepo *recordingAPIKeyRepo, plainKey string) *model.APIKey {
	t.Helper()

	if len(keyRepo.created) != 1 {
		t.Fatalf("expected exactly 1 persisted API key, got %d", len(keyRepo.created))
	}
	dbKey := keyRepo.created[0]

	wantHash := sha256Hex(plainKey)
	if !strings.Contains(dbKey.APIKeyHashes, wantHash) {
		t.Errorf("persisted hashes %q do not contain the sha256 of the key material", dbKey.APIKeyHashes)
	}
	if wantMasked := maskAPIKey(plainKey); dbKey.MaskedAPIKey != wantMasked {
		t.Errorf("persisted masked key = %q, want %q", dbKey.MaskedAPIKey, wantMasked)
	}
	if strings.Contains(dbKey.APIKeyHashes, plainKey) || dbKey.MaskedAPIKey == plainKey {
		t.Error("plaintext key material leaked into a persisted field")
	}
	if dbKey.Status != constants.APIKeyStatusActive {
		t.Errorf("persisted status = %q, want %q", dbKey.Status, constants.APIKeyStatusActive)
	}
	return dbKey
}

// TestCreateAPIKey_GeneratesKeyWhenOmitted is the regression for
// https://github.com/wso2/api-platform/issues/3252: omitting apiKey used to be rejected with
// "API key value is required". The server now mints one with the same primitive the LLM
// proxy/provider key paths use, persists only its hash, and returns the plaintext once.
func TestCreateAPIKey_GeneratesKeyWhenOmitted(t *testing.T) {
	for _, tc := range []struct {
		name     string
		supplied string
	}{
		{name: "absent", supplied: ""},
		{name: "whitespace only", supplied: "   "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, keyRepo := newCreateAPIKeyTestService(t, nil)

			resp, err := svc.CreateAPIKey(context.Background(), testAPIHandle, constants.RestApi, testOrgID, testUserID,
				&api.CreateAPIKeyRequest{DisplayName: "Production Key", ApiKey: tc.supplied})
			if err != nil {
				t.Fatalf("CreateAPIKey returned an unexpected error: %v", err)
			}

			if resp.ApiKey == nil {
				t.Fatal("response omitted apiKey, but the server generated the key — it is unrecoverable after this response")
			}
			if !generatedKeyRegex.MatchString(*resp.ApiKey) {
				t.Errorf("generated key %q is not 64 lowercase hex characters", *resp.ApiKey)
			}
			assertPersistedKeyMaterial(t, keyRepo, *resp.ApiKey)
		})
	}
}

// TestCreateAPIKey_UsesSuppliedKeyAndNeverEchoesIt covers the inject path external platforms
// use. The supplied value must be what gets hashed, and must NOT come back in the response —
// echoing a caller's secret adds a disclosure surface for no benefit.
func TestCreateAPIKey_UsesSuppliedKeyAndNeverEchoesIt(t *testing.T) {
	const injected = "sk_example_1234567890abcdef"

	svc, keyRepo := newCreateAPIKeyTestService(t, nil)

	resp, err := svc.CreateAPIKey(context.Background(), testAPIHandle, constants.RestApi, testOrgID, testUserID,
		&api.CreateAPIKeyRequest{DisplayName: "Injected Key", ApiKey: injected})
	if err != nil {
		t.Fatalf("CreateAPIKey returned an unexpected error: %v", err)
	}

	if resp.ApiKey != nil {
		t.Errorf("response echoed a caller-supplied key (%q); it must be returned only when the server generated it", *resp.ApiKey)
	}
	assertPersistedKeyMaterial(t, keyRepo, injected)
}

// TestCreateAPIKey_DerivesNameWhenIdAndDisplayNameOmitted covers the fallback branch in
// resolveUniqueKeyName. The handlers used to pre-derive the name via utils.GenerateHandle,
// which errors on an empty source — so a body carrying neither id nor displayName returned
// 400 even though nothing enforces displayName at runtime. Naming now belongs to the service.
func TestCreateAPIKey_DerivesNameWhenIdAndDisplayNameOmitted(t *testing.T) {
	svc, keyRepo := newCreateAPIKeyTestService(t, nil)

	resp, err := svc.CreateAPIKey(context.Background(), testAPIHandle, constants.RestApi, testOrgID, testUserID,
		&api.CreateAPIKeyRequest{})
	if err != nil {
		t.Fatalf("CreateAPIKey returned an unexpected error for a body with neither id nor displayName: %v", err)
	}

	if resp.KeyId == nil {
		t.Fatal("response omitted keyId")
	}
	if want := regexp.MustCompile(`^` + testAPIHandle + `-key-[0-9a-f]{8}$`); !want.MatchString(*resp.KeyId) {
		t.Errorf("derived key name = %q, want the <handle>-key-<8 hex> fallback shape", *resp.KeyId)
	}

	dbKey := assertPersistedKeyMaterial(t, keyRepo, *resp.ApiKey)
	if dbKey.DisplayName != *resp.KeyId {
		t.Errorf("persisted displayName = %q, want it to default to the derived name %q", dbKey.DisplayName, *resp.KeyId)
	}
}

// TestCreateAPIKey_KeyIdReflectsCollisionSuffix pins the response to the name actually
// persisted. resolveUniqueKeyName appends a random suffix on collision, so a caller that
// echoed back its own requested id would be pointed at a key that does not exist.
func TestCreateAPIKey_KeyIdReflectsCollisionSuffix(t *testing.T) {
	const requested = "production-key"

	svc, keyRepo := newCreateAPIKeyTestService(t, map[string]*model.APIKey{
		requested: {UUID: "existing", ArtifactUUID: testAPIUUID, Name: requested},
	})

	id := requested
	resp, err := svc.CreateAPIKey(context.Background(), testAPIHandle, constants.RestApi, testOrgID, testUserID,
		&api.CreateAPIKeyRequest{Id: &id, DisplayName: "Production Key"})
	if err != nil {
		t.Fatalf("CreateAPIKey returned an unexpected error: %v", err)
	}

	if resp.KeyId == nil {
		t.Fatal("response omitted keyId")
	}
	if *resp.KeyId == requested {
		t.Fatalf("keyId = %q, but that name was already taken — expected a suffixed name", requested)
	}
	if want := regexp.MustCompile(`^` + requested + `-[0-9a-f]{4}$`); !want.MatchString(*resp.KeyId) {
		t.Errorf("keyId = %q, want %q plus a 4-hex collision suffix", *resp.KeyId, requested)
	}

	dbKey := assertPersistedKeyMaterial(t, keyRepo, *resp.ApiKey)
	if dbKey.Name != *resp.KeyId {
		t.Errorf("keyId %q does not match the persisted name %q", *resp.KeyId, dbKey.Name)
	}
}

// TestCreateAPIKey_RejectsPastExpiryBeforeMintingKey asserts an already-expired request is
// turned away, and that nothing was persisted — an invalid request must not cost key
// generation or hashing work, which is why resolveExpiresAt runs first.
func TestCreateAPIKey_RejectsPastExpiryBeforeMintingKey(t *testing.T) {
	svc, keyRepo := newCreateAPIKeyTestService(t, nil)

	past := time.Now().Add(-time.Hour)
	resp, err := svc.CreateAPIKey(context.Background(), testAPIHandle, constants.RestApi, testOrgID, testUserID,
		&api.CreateAPIKeyRequest{DisplayName: "Expired Key", ExpiresAt: &past})
	if err == nil {
		t.Fatal("CreateAPIKey accepted an expiration in the past")
	}
	if resp != nil {
		t.Errorf("expected a nil response alongside the error, got %+v", resp)
	}
	if len(keyRepo.created) != 0 {
		t.Errorf("persisted %d key(s) despite the invalid expiration", len(keyRepo.created))
	}
}
