/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package service

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/vault"
)

// webhookSecretPortalRepo serves one portal by handle (for the secret service)
// and by uuid (for the key registry), and records deletes.
type webhookSecretPortalRepo struct {
	repository.APIPortalRepository
	portal    *model.APIPortal
	getErr    error
	deleted   []string
	deleteErr error
}

func (f *webhookSecretPortalRepo) GetByHandleAndOrgID(string, string) (*model.APIPortal, error) {
	return f.portal, f.getErr
}

func (f *webhookSecretPortalRepo) GetByUUIDAny(string) (*model.APIPortal, error) {
	return f.portal, f.getErr
}

func (f *webhookSecretPortalRepo) Delete(id, _ string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, id)
	return nil
}

// memWebhookSecretRepo is an in-memory APIPortalWebhookSecretRepository. A
// duplicate Insert fails the way SQLite reports a primary-key violation, which
// repository.IsUniqueViolation recognises.
type memWebhookSecretRepo struct {
	rows map[string]*model.APIPortalWebhookSecret

	readErr   error
	insertErr error // returned by Insert instead of writing, when set
	rotateErr error
	// beforeInsert runs inside Insert before the row is written, so a test
	// can play the concurrent caller that wins the first insert.
	beforeInsert func()
	// dropAfterInsertConflict empties the store after a conflicting Insert, to
	// cover the "conflict but no row" path.
	dropAfterInsertConflict bool

	reads, inserts, rotates int
	lastValidUntil          time.Time
}

func newMemWebhookSecretRepo() *memWebhookSecretRepo {
	return &memWebhookSecretRepo{rows: map[string]*model.APIPortalWebhookSecret{}}
}

func (m *memWebhookSecretRepo) GetByPortalUUID(uuid string) (*model.APIPortalWebhookSecret, error) {
	m.reads++
	if m.readErr != nil {
		return nil, m.readErr
	}
	row, ok := m.rows[uuid]
	if !ok {
		return nil, nil
	}
	cp := *row
	return &cp, nil
}

func (m *memWebhookSecretRepo) Insert(uuid string, enc []byte) error {
	m.inserts++
	if m.insertErr != nil {
		return m.insertErr
	}
	if m.beforeInsert != nil {
		hook := m.beforeInsert
		m.beforeInsert = nil
		hook()
	}
	if _, ok := m.rows[uuid]; ok {
		if m.dropAfterInsertConflict {
			delete(m.rows, uuid)
		}
		return errors.New("UNIQUE constraint failed: api_portal_webhook_secrets.portal_uuid")
	}
	m.rows[uuid] = &model.APIPortalWebhookSecret{PortalUUID: uuid, SecretEnc: enc}
	return nil
}

func (m *memWebhookSecretRepo) Rotate(uuid string, enc []byte, validUntil time.Time) error {
	m.rotates++
	m.lastValidUntil = validUntil
	if m.rotateErr != nil {
		return m.rotateErr
	}
	row, ok := m.rows[uuid]
	if !ok {
		return errors.New("rotate: no row")
	}
	row.PreviousSecretEnc = row.SecretEnc
	row.PreviousValidUntil = sql.NullTime{Valid: true, Time: validUntil}
	row.SecretEnc = enc
	row.RotatedAt = sql.NullTime{Valid: true, Time: time.Now().UTC()}
	return nil
}

func (m *memWebhookSecretRepo) ClearExpiredPrevious(uuid string, now time.Time) error {
	if row, ok := m.rows[uuid]; ok && row.PreviousValidUntil.Valid && !now.Before(row.PreviousValidUntil.Time) {
		row.PreviousSecretEnc = nil
		row.PreviousValidUntil = sql.NullTime{}
	}
	return nil
}

const webhookSecretTestGrace = 10 * time.Minute

type webhookSecretTestEnv struct {
	svc      *APIPortalWebhookSecretService
	portals  *webhookSecretPortalRepo
	secrets  *memWebhookSecretRepo
	registry *PortalWebhookKeyRegistry
	vault    vault.SecretVault
}

func newWebhookSecretTestEnv(t *testing.T) *webhookSecretTestEnv {
	t.Helper()
	v, err := vault.NewInHouseVault(bytes.Repeat([]byte("t"), 32))
	if err != nil {
		t.Fatalf("vault: %v", err)
	}
	portals := &webhookSecretPortalRepo{portal: &model.APIPortal{ID: "p-1", Handle: "acme", OrganizationID: "o-1"}}
	secrets := newMemWebhookSecretRepo()
	registry := NewPortalWebhookKeyRegistry(portals, secrets, v, 5*time.Minute)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewAPIPortalWebhookSecretService(portals, secrets, v, webhookSecretTestGrace, registry, logger)
	return &webhookSecretTestEnv{svc: svc, portals: portals, secrets: secrets, registry: registry, vault: v}
}

// storedSecret decrypts the current secret held for p-1.
func (e *webhookSecretTestEnv) storedSecret(t *testing.T) string {
	t.Helper()
	row := e.secrets.rows["p-1"]
	if row == nil {
		t.Fatal("no secret row stored for p-1")
	}
	plain, err := e.vault.Decrypt(context.Background(), row.SecretEnc)
	if err != nil {
		t.Fatalf("decrypt stored secret: %v", err)
	}
	return plain
}

func TestEnsureWebhookSecret_GeneratesOnceThenReturnsTheSame(t *testing.T) {
	e := newWebhookSecretTestEnv(t)
	ctx := context.Background()

	first, uuid, err := e.svc.EnsureWebhookSecret(ctx, "acme", "o-1")
	if err != nil {
		t.Fatalf("first Ensure: %v", err)
	}
	if uuid != "p-1" {
		t.Errorf("portalUUID = %q, want p-1", uuid)
	}
	if len(first) != 2*webhookSecretBytes {
		t.Errorf("secret length = %d, want %d hex chars", len(first), 2*webhookSecretBytes)
	}
	if got := e.storedSecret(t); got != first {
		t.Errorf("stored secret differs from the returned one")
	}
	if string(e.secrets.rows["p-1"].SecretEnc) == first {
		t.Error("secret stored in plaintext")
	}

	second, _, err := e.svc.EnsureWebhookSecret(ctx, "acme", "o-1")
	if err != nil {
		t.Fatalf("second Ensure: %v", err)
	}
	if second != first {
		t.Error("second Ensure returned a different secret; Ensure must be idempotent")
	}
	if e.secrets.inserts != 1 {
		t.Errorf("inserts = %d, want 1", e.secrets.inserts)
	}
}

func TestEnsureWebhookSecret_ConcurrentFirstInsertReturnsTheWinner(t *testing.T) {
	e := newWebhookSecretTestEnv(t)
	ctx := context.Background()
	winnerEnc, err := e.vault.Encrypt(ctx, "winner-secret")
	if err != nil {
		t.Fatal(err)
	}
	e.secrets.beforeInsert = func() {
		e.secrets.rows["p-1"] = &model.APIPortalWebhookSecret{PortalUUID: "p-1", SecretEnc: winnerEnc}
	}

	got, _, err := e.svc.EnsureWebhookSecret(ctx, "acme", "o-1")
	if err != nil {
		t.Fatalf("Ensure after a lost insert race: %v", err)
	}
	if got != "winner-secret" {
		t.Errorf("Ensure = %q, want the secret the concurrent caller stored", got)
	}
}

func TestEnsureWebhookSecret_Failures(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name    string
		setup   func(*webhookSecretTestEnv)
		wantErr func(error) bool
	}{
		{"unknown portal", func(e *webhookSecretTestEnv) { e.portals.portal = nil },
			func(err error) bool { return apperror.APIPortalNotFound.Is(err) }},
		{"portal lookup fails", func(e *webhookSecretTestEnv) { e.portals.getErr = boom },
			func(err error) bool { return errors.Is(err, boom) }},
		{"secret read fails", func(e *webhookSecretTestEnv) { e.secrets.readErr = boom },
			func(err error) bool { return errors.Is(err, boom) }},
		{"insert fails for another reason", func(e *webhookSecretTestEnv) { e.secrets.insertErr = boom },
			func(err error) bool { return errors.Is(err, boom) }},
		{"insert conflicts but the winning row is gone", func(e *webhookSecretTestEnv) {
			e.secrets.beforeInsert = func() { e.secrets.rows["p-1"] = &model.APIPortalWebhookSecret{PortalUUID: "p-1"} }
			e.secrets.dropAfterInsertConflict = true
		}, func(err error) bool { return err != nil && repository.IsUniqueViolation(err) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newWebhookSecretTestEnv(t)
			tc.setup(e)
			_, _, err := e.svc.EnsureWebhookSecret(context.Background(), "acme", "o-1")
			if !tc.wantErr(err) {
				t.Fatalf("Ensure error = %v", err)
			}
		})
	}
}

func TestRotateWebhookSecret_WithoutARowInserts(t *testing.T) {
	e := newWebhookSecretTestEnv(t)
	got, uuid, err := e.svc.RotateWebhookSecret(context.Background(), "acme", "o-1")
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if uuid != "p-1" || got == "" {
		t.Fatalf("Rotate = (%q, %q)", got, uuid)
	}
	if e.secrets.inserts != 1 || e.secrets.rotates != 0 {
		t.Errorf("inserts=%d rotates=%d, want 1/0", e.secrets.inserts, e.secrets.rotates)
	}
	if e.storedSecret(t) != got {
		t.Error("stored secret differs from the returned one")
	}
}

func TestRotateWebhookSecret_KeepsThePreviousForTheGraceWindow(t *testing.T) {
	e := newWebhookSecretTestEnv(t)
	ctx := context.Background()
	old, _, err := e.svc.EnsureWebhookSecret(ctx, "acme", "o-1")
	if err != nil {
		t.Fatal(err)
	}

	before := time.Now().UTC()
	rotated, _, err := e.svc.RotateWebhookSecret(ctx, "acme", "o-1")
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if rotated == old {
		t.Fatal("Rotate returned the old secret")
	}
	if e.storedSecret(t) != rotated {
		t.Error("current secret is not the rotated one")
	}
	prev, err := e.vault.Decrypt(ctx, e.secrets.rows["p-1"].PreviousSecretEnc)
	if err != nil || prev != old {
		t.Errorf("previous secret = %q (%v), want the old one", prev, err)
	}
	if d := e.secrets.lastValidUntil.Sub(before); d < webhookSecretTestGrace || d > webhookSecretTestGrace+time.Minute {
		t.Errorf("previous valid for %s after rotation, want about %s", d, webhookSecretTestGrace)
	}
}

func TestRotateWebhookSecret_InvalidatesCachedKeys(t *testing.T) {
	e := newWebhookSecretTestEnv(t)
	ctx := context.Background()
	if _, _, err := e.svc.EnsureWebhookSecret(ctx, "acme", "o-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.registry.Get(ctx, "p-1"); err != nil {
		t.Fatalf("warm the cache: %v", err)
	}
	if _, _, err := e.svc.RotateWebhookSecret(ctx, "acme", "o-1"); err != nil {
		t.Fatal(err)
	}
	readsAfterRotate := e.secrets.reads
	keys, err := e.registry.Get(ctx, "p-1")
	if err != nil {
		t.Fatalf("Get after rotate: %v", err)
	}
	if e.secrets.reads == readsAfterRotate {
		t.Error("Get after a rotation served the cached keys; the rotation must invalidate them")
	}
	if keys.Previous == nil {
		t.Error("keys after rotation carry no previous verifier for the grace window")
	}
}

func TestRotateWebhookSecret_ConcurrentFirstInsertRotatesOverIt(t *testing.T) {
	e := newWebhookSecretTestEnv(t)
	ctx := context.Background()
	winnerEnc, err := e.vault.Encrypt(ctx, "winner-secret")
	if err != nil {
		t.Fatal(err)
	}
	e.secrets.beforeInsert = func() {
		e.secrets.rows["p-1"] = &model.APIPortalWebhookSecret{PortalUUID: "p-1", SecretEnc: winnerEnc}
	}

	got, _, err := e.svc.RotateWebhookSecret(ctx, "acme", "o-1")
	if err != nil {
		t.Fatalf("Rotate after a lost insert race: %v", err)
	}
	if e.secrets.rotates != 1 {
		t.Errorf("rotates = %d, want 1 (rotate over the row that won)", e.secrets.rotates)
	}
	if e.storedSecret(t) != got {
		t.Error("current secret is not the rotated one")
	}
}

func TestRotateWebhookSecret_Failures(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name  string
		setup func(*webhookSecretTestEnv)
		check func(error) bool
	}{
		{"unknown portal", func(e *webhookSecretTestEnv) { e.portals.portal = nil },
			func(err error) bool { return apperror.APIPortalNotFound.Is(err) }},
		{"portal lookup fails", func(e *webhookSecretTestEnv) { e.portals.getErr = boom },
			func(err error) bool { return errors.Is(err, boom) }},
		{"secret read fails", func(e *webhookSecretTestEnv) { e.secrets.readErr = boom },
			func(err error) bool { return errors.Is(err, boom) }},
		{"insert fails for another reason", func(e *webhookSecretTestEnv) { e.secrets.insertErr = boom },
			func(err error) bool { return errors.Is(err, boom) }},
		{"rotate fails", func(e *webhookSecretTestEnv) {
			e.secrets.rows["p-1"] = &model.APIPortalWebhookSecret{PortalUUID: "p-1", SecretEnc: []byte("x")}
			e.secrets.rotateErr = boom
		}, func(err error) bool { return errors.Is(err, boom) }},
		{"rotate over a lost insert race fails", func(e *webhookSecretTestEnv) {
			e.secrets.beforeInsert = func() { e.secrets.rows["p-1"] = &model.APIPortalWebhookSecret{PortalUUID: "p-1"} }
			e.secrets.rotateErr = boom
		}, func(err error) bool { return errors.Is(err, boom) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newWebhookSecretTestEnv(t)
			tc.setup(e)
			_, _, err := e.svc.RotateWebhookSecret(context.Background(), "acme", "o-1")
			if !tc.check(err) {
				t.Fatalf("Rotate error = %v", err)
			}
		})
	}
}

func TestPortalWebhookKeyRegistry_UnknownPortalOrSecretIsNotFound(t *testing.T) {
	e := newWebhookSecretTestEnv(t)
	ctx := context.Background()

	if _, err := e.registry.Get(ctx, "p-1"); !apperror.APIPortalNotFound.Is(err) {
		t.Errorf("Get with no secret row: err = %v, want APIPortalNotFound", err)
	}
	e.portals.portal = nil
	if _, err := e.registry.Get(ctx, "p-1"); !apperror.APIPortalNotFound.Is(err) {
		t.Errorf("Get for an unknown portal: err = %v, want APIPortalNotFound", err)
	}
}

func TestPortalWebhookKeyRegistry_InfrastructureErrorsAreNotNotFound(t *testing.T) {
	boom := errors.New("boom")
	e := newWebhookSecretTestEnv(t)
	ctx := context.Background()

	e.portals.getErr = boom
	if _, err := e.registry.Get(ctx, "p-1"); !errors.Is(err, boom) || apperror.APIPortalNotFound.Is(err) {
		t.Errorf("portal lookup failure: err = %v, want boom and not APIPortalNotFound", err)
	}
	e.portals.getErr = nil
	e.secrets.readErr = boom
	if _, err := e.registry.Get(ctx, "p-1"); !errors.Is(err, boom) {
		t.Errorf("secret read failure: err = %v, want boom", err)
	}
	e.secrets.readErr = nil
	e.secrets.rows["p-1"] = &model.APIPortalWebhookSecret{PortalUUID: "p-1", SecretEnc: []byte("not ciphertext")}
	if _, err := e.registry.Get(ctx, "p-1"); err == nil || apperror.APIPortalNotFound.Is(err) {
		t.Errorf("undecryptable secret: err = %v, want a decrypt error", err)
	}
}

func TestPortalWebhookKeyRegistry_NilRegistryIsAnError(t *testing.T) {
	var reg *PortalWebhookKeyRegistry
	if _, err := reg.Get(context.Background(), "p-1"); err == nil {
		t.Fatal("nil registry Get returned no error")
	}
}

func TestPortalWebhookKeyRegistry_PreviousInsideGraceIsLoaded(t *testing.T) {
	e := newWebhookSecretTestEnv(t)
	ctx := context.Background()
	if _, _, err := e.svc.EnsureWebhookSecret(ctx, "acme", "o-1"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.svc.RotateWebhookSecret(ctx, "acme", "o-1"); err != nil {
		t.Fatal(err)
	}
	keys, err := e.registry.Get(ctx, "p-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if keys.OrganizationUUID != "o-1" || keys.Current == nil || keys.CurrentDecryptor == nil {
		t.Fatalf("keys = %+v, want current verifier/decryptor for o-1", keys)
	}
	if keys.Previous == nil || keys.PreviousDecryptor == nil || keys.PreviousValidUntil.IsZero() {
		t.Errorf("previous slot not loaded inside its grace window: %+v", keys)
	}
}

func TestDeleteAPIPortal_DropsCachedWebhookKeys(t *testing.T) {
	e := newWebhookSecretTestEnv(t)
	ctx := context.Background()
	if _, _, err := e.svc.EnsureWebhookSecret(ctx, "acme", "o-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.registry.Get(ctx, "p-1"); err != nil {
		t.Fatalf("warm the cache: %v", err)
	}

	portalSvc := NewAPIPortalService(e.portals, nil, noopWebhookAudit{}, e.vault, nil, e.svc, nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := portalSvc.DeleteAPIPortal("acme", "o-1", "alice"); err != nil {
		t.Fatalf("DeleteAPIPortal: %v", err)
	}
	if len(e.portals.deleted) != 1 {
		t.Fatalf("portal not deleted: %v", e.portals.deleted)
	}

	// The FK cascade removes the secret row with the portal.
	delete(e.secrets.rows, "p-1")
	e.portals.portal = nil
	if _, err := e.registry.Get(ctx, "p-1"); !apperror.APIPortalNotFound.Is(err) {
		t.Errorf("Get right after delete: err = %v, want APIPortalNotFound (cached keys must be dropped)", err)
	}
}

type noopWebhookAudit struct{ repository.AuditRepository }

func (noopWebhookAudit) Record(string, string, string, string, string) error { return nil }
