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
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/vault"
)

type fakeKeyRegistryPortalRepo struct {
	repository.APIPortalRepository
	portal *model.APIPortal
}

func (f *fakeKeyRegistryPortalRepo) GetByUUIDAny(string) (*model.APIPortal, error) {
	return f.portal, nil
}

type fakeKeyRegistrySecretRepo struct {
	row       *model.APIPortalWebhookSecret
	reads     int
	clears    int
	clearedAt time.Time
}

func (f *fakeKeyRegistrySecretRepo) GetByPortalUUID(string) (*model.APIPortalWebhookSecret, error) {
	f.reads++
	return f.row, nil
}

func (f *fakeKeyRegistrySecretRepo) Insert(string, []byte) error { return nil }

func (f *fakeKeyRegistrySecretRepo) Rotate(string, []byte, time.Time) error { return nil }

func (f *fakeKeyRegistrySecretRepo) ClearExpiredPrevious(_ string, now time.Time) error {
	f.clears++
	f.clearedAt = now
	// Mimic the SQL cleanup: strip the previous slot from the in-memory row so
	// a subsequent reload doesn't rebuild a verifier for the expired secret.
	if f.row != nil {
		f.row.PreviousSecretEnc = nil
		f.row.PreviousValidUntil = sql.NullTime{}
	}
	return nil
}

func newKeyRegistryTest(t *testing.T) (*PortalWebhookKeyRegistry, *fakeKeyRegistrySecretRepo, vault.SecretVault, *time.Time) {
	t.Helper()
	v, err := vault.NewInHouseVault(bytes.Repeat([]byte("t"), 32))
	if err != nil {
		t.Fatalf("vault: %v", err)
	}
	currentEnc, err := v.Encrypt(context.Background(), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatalf("encrypt current: %v", err)
	}
	portalRepo := &fakeKeyRegistryPortalRepo{portal: &model.APIPortal{ID: "p-1", OrganizationID: "o-1"}}
	secretRepo := &fakeKeyRegistrySecretRepo{row: &model.APIPortalWebhookSecret{PortalUUID: "p-1", SecretEnc: currentEnc}}

	clock := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	reg := NewPortalWebhookKeyRegistry(portalRepo, secretRepo, v, 5*time.Minute)
	reg.now = func() time.Time { return clock }
	return reg, secretRepo, v, &clock
}

// TestPortalWebhookKeyRegistry_CachesWithinTTL asserts a second Get within the
// TTL window reuses the cached entry without re-reading the DB.
func TestPortalWebhookKeyRegistry_CachesWithinTTL(t *testing.T) {
	reg, secretRepo, _, clock := newKeyRegistryTest(t)
	ctx := context.Background()

	if _, err := reg.Get(ctx, "p-1"); err != nil {
		t.Fatalf("first Get: %v", err)
	}
	readsAfterFirst := secretRepo.reads

	*clock = clock.Add(30 * time.Second) // within cacheEntryTTL (60s)
	if _, err := reg.Get(ctx, "p-1"); err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if secretRepo.reads != readsAfterFirst {
		t.Errorf("expected cache hit within TTL; reads went %d -> %d", readsAfterFirst, secretRepo.reads)
	}
}

// TestPortalWebhookKeyRegistry_RefreshesAfterTTL asserts an entry older than
// cacheEntryTTL is treated as a miss and reloaded from the repository — this
// is the mechanism that lets other replicas see a rotation performed on this
// one without any cross-replica coordination.
func TestPortalWebhookKeyRegistry_RefreshesAfterTTL(t *testing.T) {
	reg, secretRepo, _, clock := newKeyRegistryTest(t)
	ctx := context.Background()

	if _, err := reg.Get(ctx, "p-1"); err != nil {
		t.Fatalf("first Get: %v", err)
	}
	readsAfterFirst := secretRepo.reads

	*clock = clock.Add(cacheEntryTTL + time.Second)
	if _, err := reg.Get(ctx, "p-1"); err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if secretRepo.reads != readsAfterFirst+1 {
		t.Errorf("expected reload after TTL; reads went %d -> %d (want %d)", readsAfterFirst, secretRepo.reads, readsAfterFirst+1)
	}
}

// TestPortalWebhookKeyRegistry_ClearsExpiredPreviousOnLoad asserts the lazy
// cleanup wired into Get: when the previous-secret grace deadline has passed,
// the row's previous slot is cleared and no previous verifier is built. This
// is what keeps the `previous_secret_enc` column from carrying ciphertext for
// a rotated-out secret indefinitely.
func TestPortalWebhookKeyRegistry_ClearsExpiredPreviousOnLoad(t *testing.T) {
	reg, secretRepo, v, clock := newKeyRegistryTest(t)
	ctx := context.Background()

	previousEnc, err := v.Encrypt(ctx, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err != nil {
		t.Fatalf("encrypt previous: %v", err)
	}
	secretRepo.row.PreviousSecretEnc = previousEnc
	secretRepo.row.PreviousValidUntil = sql.NullTime{Valid: true, Time: clock.Add(-time.Minute)} // already expired

	keys, err := reg.Get(ctx, "p-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if keys.Previous != nil || keys.PreviousDecryptor != nil {
		t.Errorf("expected no previous verifier for an expired grace slot; got Previous=%v, PreviousDecryptor=%v", keys.Previous, keys.PreviousDecryptor)
	}
	if secretRepo.clears != 1 {
		t.Errorf("expected one ClearExpiredPrevious call, got %d", secretRepo.clears)
	}
	if !secretRepo.clearedAt.Equal(*clock) {
		t.Errorf("ClearExpiredPrevious called with wrong 'now': got %s, want %s", secretRepo.clearedAt, *clock)
	}
}
