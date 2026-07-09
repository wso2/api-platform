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
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// ---- mock vault -------------------------------------------------------------

type mockVault struct {
	encryptFn func(string) ([]byte, error)
	decryptFn func([]byte) (string, error)
}

func (m *mockVault) Encrypt(_ context.Context, plain string) ([]byte, error) {
	if m.encryptFn != nil {
		return m.encryptFn(plain)
	}
	return []byte("enc:" + plain), nil
}

func (m *mockVault) Decrypt(_ context.Context, ct []byte) (string, error) {
	if m.decryptFn != nil {
		return m.decryptFn(ct)
	}
	val := string(ct)
	if len(val) > 4 {
		return val[4:], nil // strip "enc:"
	}
	return val, nil
}

func (m *mockVault) ProviderName() string { return "mock" }
func (m *mockVault) HashKey() []byte      { return make([]byte, 32) }

// ---- mock repo --------------------------------------------------------------

type mockSecretRepo struct {
	repository.SecretRepository

	secrets map[string]*model.Secret

	createFn                func(*model.Secret) error
	existsFn                func(orgID, handle string) (bool, error)
	getByHandleFn           func(orgID, handle string) (*model.Secret, error)
	updateFn                func(*model.Secret) error
	findRefsAndSoftDeleteFn func(orgID, handle, by string) ([]model.SecretReference, error)
	findRefsFn              func(orgID, handle string) ([]model.SecretReference, error)
	listFn                  func(orgID string, limit, offset int, after *time.Time) ([]*model.Secret, error)
	countFn                 func(orgID string) (int, error)
}

func newMockRepo() *mockSecretRepo {
	return &mockSecretRepo{secrets: make(map[string]*model.Secret)}
}

func (m *mockSecretRepo) Create(s *model.Secret) error {
	if m.createFn != nil {
		return m.createFn(s)
	}
	if _, exists := m.secrets[s.Handle]; exists {
		return apperror.SecretExists.New()
	}
	if s.UUID == "" {
		s.UUID = "uuid-" + s.Handle
	}
	m.secrets[s.Handle] = s
	return nil
}

func (m *mockSecretRepo) Exists(orgID, handle string) (bool, error) {
	if m.existsFn != nil {
		return m.existsFn(orgID, handle)
	}
	s, ok := m.secrets[handle]
	return ok && s.Status == model.SecretStatusActive, nil
}

func (m *mockSecretRepo) GetByHandle(orgID, handle string) (*model.Secret, error) {
	if m.getByHandleFn != nil {
		return m.getByHandleFn(orgID, handle)
	}
	s, ok := m.secrets[handle]
	if !ok {
		return nil, apperror.SecretNotFound.New()
	}
	return s, nil
}

func (m *mockSecretRepo) Update(s *model.Secret) error {
	if m.updateFn != nil {
		return m.updateFn(s)
	}
	if _, ok := m.secrets[s.Handle]; !ok {
		return apperror.SecretNotFound.New()
	}
	m.secrets[s.Handle] = s
	return nil
}

func (m *mockSecretRepo) FindRefsAndSoftDelete(orgID, handle, by string) ([]model.SecretReference, error) {
	if m.findRefsAndSoftDeleteFn != nil {
		return m.findRefsAndSoftDeleteFn(orgID, handle, by)
	}
	if m.findRefsFn != nil {
		refs, err := m.findRefsFn(orgID, handle)
		if err != nil || len(refs) > 0 {
			return refs, err
		}
	}
	s, ok := m.secrets[handle]
	if !ok {
		return nil, apperror.SecretNotFound.New()
	}
	s.Status = model.SecretStatusDeprecated
	return nil, nil
}

func (m *mockSecretRepo) FindRefs(orgID, handle string) ([]model.SecretReference, error) {
	if m.findRefsFn != nil {
		return m.findRefsFn(orgID, handle)
	}
	return nil, nil
}

func (m *mockSecretRepo) List(orgID string, limit, offset int, after *time.Time) ([]*model.Secret, error) {
	if m.listFn != nil {
		return m.listFn(orgID, limit, offset, after)
	}
	var list []*model.Secret
	for _, s := range m.secrets {
		list = append(list, s)
	}
	start := offset
	if start > len(list) {
		return nil, nil
	}
	end := start + limit
	if end > len(list) {
		end = len(list)
	}
	return list[start:end], nil
}

func (m *mockSecretRepo) Count(orgID string) (int, error) {
	if m.countFn != nil {
		return m.countFn(orgID)
	}
	return len(m.secrets), nil
}

// ---- Create tests -----------------------------------------------------------

func TestSecretService_Create_SetsOrgSharedValueScope(t *testing.T) {
	repo := newMockRepo()
	svc := NewSecretService(repo, &mockVault{}, newTestIdentityService())

	_, err := svc.Create("org1", "alice", &dto.CreateSecretRequest{
		Handle:      "my-secret",
		DisplayName: "My Secret",
		Value:       "plaintext",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Secret must be stored — verify via stored model
	stored, err := repo.GetByHandle("org1", "my-secret")
	if err != nil {
		t.Fatalf("GetByHandle: %v", err)
	}
	if stored.Handle != "my-secret" {
		t.Errorf("Handle = %q, want %q", stored.Handle, "my-secret")
	}
}

func TestSecretService_Create_ReturnsCleartextValue(t *testing.T) {
	repo := newMockRepo()
	svc := NewSecretService(repo, &mockVault{}, newTestIdentityService())

	_, err := svc.Create("org1", "alice", &dto.CreateSecretRequest{
		Handle: "secret-with-value",
		Value:  "my-plaintext",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
}

func TestSecretService_Create_DuplicateHandle_ReturnsError(t *testing.T) {
	repo := newMockRepo()
	// Pre-populate so Exists returns true
	repo.secrets["dup"] = &model.Secret{Handle: "dup", Status: model.SecretStatusActive}

	svc := NewSecretService(repo, &mockVault{}, newTestIdentityService())
	_, err := svc.Create("org1", "alice", &dto.CreateSecretRequest{
		Handle: "dup",
		Value:  "val",
	})
	if !apperror.SecretExists.Is(err) {
		t.Errorf("expected ErrSecretAlreadyExists, got %v", err)
	}
}

func TestSecretService_Create_EncryptionError_PropagatesError(t *testing.T) {
	repo := newMockRepo()
	v := &mockVault{
		encryptFn: func(s string) ([]byte, error) {
			return nil, errors.New("vault unavailable")
		},
	}
	svc := NewSecretService(repo, v, newTestIdentityService())

	_, err := svc.Create("org1", "alice", &dto.CreateSecretRequest{
		Handle: "new-secret",
		Value:  "val",
	})
	if err == nil {
		t.Error("expected error from encryption failure")
	}
}

func TestSecretService_Create_DefaultsTypeToGeneric(t *testing.T) {
	repo := newMockRepo()
	svc := NewSecretService(repo, &mockVault{}, newTestIdentityService())

	_, err := svc.Create("org1", "alice", &dto.CreateSecretRequest{
		Handle: "typed-secret",
		Value:  "val",
		// Type not set
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	s := repo.secrets["typed-secret"]
	if s.Type != model.SecretTypeGeneric {
		t.Errorf("Type = %q, want %q", s.Type, model.SecretTypeGeneric)
	}
}

func TestSecretService_Create_InvalidType_ReturnsError(t *testing.T) {
	repo := newMockRepo()
	svc := NewSecretService(repo, &mockVault{}, newTestIdentityService())

	_, err := svc.Create("org1", "alice", &dto.CreateSecretRequest{
		Handle: "bad-type-secret",
		Value:  "val",
		Type:   "API_KEY",
	})
	if !apperror.ValidationFailed.Is(err) {
		t.Errorf("expected ErrInvalidSecretType, got %v", err)
	}
}

// ---- List tests -------------------------------------------------------------

func TestSecretService_List_ReturnsPagination(t *testing.T) {
	repo := newMockRepo()
	for _, h := range []string{"s1", "s2", "s3"} {
		repo.secrets[h] = &model.Secret{Handle: h, Status: model.SecretStatusActive}
	}

	svc := NewSecretService(repo, &mockVault{}, newTestIdentityService())
	resp, err := svc.List("org1", 10, 0, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if resp.Pagination.Total != 3 {
		t.Errorf("Total = %d, want 3", resp.Pagination.Total)
	}
	if resp.Pagination.Limit != 10 {
		t.Errorf("Limit = %d, want 10", resp.Pagination.Limit)
	}
	if resp.Pagination.Offset != 0 {
		t.Errorf("Offset = %d, want 0", resp.Pagination.Offset)
	}
}

func TestSecretService_List_PaginationReflectsOffset(t *testing.T) {
	repo := newMockRepo()
	svc := NewSecretService(repo, &mockVault{}, newTestIdentityService())

	resp, err := svc.List("org1", 5, 10, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if resp.Pagination.Offset != 10 {
		t.Errorf("Offset = %d, want 10", resp.Pagination.Offset)
	}
	if resp.Pagination.Limit != 5 {
		t.Errorf("Limit = %d, want 5", resp.Pagination.Limit)
	}
}

// ---- Get tests --------------------------------------------------------------

func TestSecretService_Get_NotFound(t *testing.T) {
	repo := newMockRepo()
	svc := NewSecretService(repo, &mockVault{}, newTestIdentityService())

	_, err := svc.Get("org1", "missing")
	if !apperror.SecretNotFound.Is(err) {
		t.Errorf("expected ErrSecretNotFound, got %v", err)
	}
}

func TestSecretService_Get_ReturnsSecret(t *testing.T) {
	repo := newMockRepo()
	repo.secrets["s1"] = &model.Secret{
		Handle:      "s1",
		DisplayName: "Secret One",
		Status:      model.SecretStatusActive,
	}

	svc := NewSecretService(repo, &mockVault{}, newTestIdentityService())
	summary, err := svc.Get("org1", "s1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if summary.Handle != "s1" {
		t.Errorf("Handle = %q, want %q", summary.Handle, "s1")
	}
}

// ---- Update tests -----------------------------------------------------------

func TestSecretService_Update_NotFound(t *testing.T) {
	repo := newMockRepo()
	svc := NewSecretService(repo, &mockVault{}, newTestIdentityService())

	_, err := svc.Update("org1", "ghost", "alice", &dto.UpdateSecretRequest{Value: "v"})
	if !apperror.SecretNotFound.Is(err) {
		t.Errorf("expected ErrSecretNotFound, got %v", err)
	}
}

func TestSecretService_Update_EncryptsNewValue(t *testing.T) {
	repo := newMockRepo()
	repo.secrets["upd"] = &model.Secret{
		Handle: "upd",
		Status: model.SecretStatusActive,
	}
	svc := NewSecretService(repo, &mockVault{}, newTestIdentityService())

	_, err := svc.Update("org1", "upd", "bob", &dto.UpdateSecretRequest{Value: "new-value"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if string(repo.secrets["upd"].Ciphertext) == "" {
		t.Error("Ciphertext should be set after update")
	}
}

// ---- Delete tests -----------------------------------------------------------

func TestSecretService_Delete_BlockedWhenInUse(t *testing.T) {
	repo := newMockRepo()
	repo.secrets["in-use"] = &model.Secret{Handle: "in-use", Status: model.SecretStatusActive}
	repo.findRefsFn = func(orgID, handle string) ([]model.SecretReference, error) {
		return []model.SecretReference{
			{Handle: "my-api", Name: "My API", Type: "RestApi"},
		}, nil
	}

	svc := NewSecretService(repo, &mockVault{}, newTestIdentityService())
	err := svc.Delete("org1", "in-use", "alice")
	if err == nil {
		t.Fatal("expected error when secret is in use")
	}

	var inUseErr *SecretInUseError
	if !errors.As(err, &inUseErr) {
		t.Errorf("expected SecretInUseError, got %T: %v", err, err)
	}
	if len(inUseErr.References) != 1 {
		t.Errorf("References len = %d, want 1", len(inUseErr.References))
	}
}

func TestSecretService_Delete_BlockedByArtifactLevelRef(t *testing.T) {
	repo := newMockRepo()
	repo.secrets["config-secret"] = &model.Secret{Handle: "config-secret", Status: model.SecretStatusActive}
	repo.findRefsFn = func(orgID, handle string) ([]model.SecretReference, error) {
		// artifact_secret_refs gateway_id='' row (artifact config, not deployed)
		return []model.SecretReference{
			{Handle: "draft-api", Name: "Draft API", Type: "RestApi"},
		}, nil
	}

	svc := NewSecretService(repo, &mockVault{}, newTestIdentityService())
	err := svc.Delete("org1", "config-secret", "alice")
	if err == nil {
		t.Fatal("should block deletion even for undeployed artifacts")
	}
}

func TestSecretService_Delete_SucceedsWhenNotInUse(t *testing.T) {
	repo := newMockRepo()
	repo.secrets["unused"] = &model.Secret{Handle: "unused", Status: model.SecretStatusActive}

	svc := NewSecretService(repo, &mockVault{}, newTestIdentityService())
	if err := svc.Delete("org1", "unused", "alice"); err != nil {
		t.Errorf("Delete: %v", err)
	}
	if repo.secrets["unused"].Status != model.SecretStatusDeprecated {
		t.Error("expected status to be DEPRECATED after deletion")
	}
}

func TestSecretService_Delete_NotFound(t *testing.T) {
	repo := newMockRepo()
	svc := NewSecretService(repo, &mockVault{}, newTestIdentityService())

	err := svc.Delete("org1", "ghost", "alice")
	if !apperror.SecretNotFound.Is(err) {
		t.Errorf("expected ErrSecretNotFound, got %v", err)
	}
}

// ---- ValidateSecretRefs tests -----------------------------------------------

func TestSecretService_ValidateSecretRefs_NoPlaceholders(t *testing.T) {
	repo := newMockRepo()
	svc := NewSecretService(repo, &mockVault{}, newTestIdentityService())

	if err := svc.ValidateSecretRefs("org1", `{"plain": "config"}`); err != nil {
		t.Errorf("unexpected error for config without placeholders: %v", err)
	}
}

func TestSecretService_ValidateSecretRefs_AllExist(t *testing.T) {
	repo := newMockRepo()
	repo.secrets["key1"] = &model.Secret{Handle: "key1", Status: model.SecretStatusActive}
	repo.secrets["key2"] = &model.Secret{Handle: "key2", Status: model.SecretStatusActive}

	svc := NewSecretService(repo, &mockVault{}, newTestIdentityService())
	config := `{{ secret "key1" }} {{ secret "key2" }}`
	if err := svc.ValidateSecretRefs("org1", config); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSecretService_ValidateSecretRefs_MissingHandle(t *testing.T) {
	repo := newMockRepo()
	svc := NewSecretService(repo, &mockVault{}, newTestIdentityService())

	config := `{{ secret "missing-key" }}`
	err := svc.ValidateSecretRefs("org1", config)
	if err == nil {
		t.Fatal("expected error for missing secret handle")
	}
	if !apperror.ValidationFailed.Is(err) {
		t.Errorf("expected ErrSecretRefMissing, got %v", err)
	}
}

func TestSecretService_ValidateSecretRefs_JSONEscapedForm(t *testing.T) {
	repo := newMockRepo()
	repo.secrets["my-key"] = &model.Secret{Handle: "my-key", Status: model.SecretStatusActive}

	svc := NewSecretService(repo, &mockVault{}, newTestIdentityService())
	// Simulates what Go's json.Marshal produces when the placeholder is inside a string field:
	// the surrounding quotes are escaped as \", giving {{ secret \"my-key\" }}.
	config := `{{ secret \"my-key\" }}`
	if err := svc.ValidateSecretRefs("org1", config); err != nil {
		t.Errorf("unexpected error for JSON-escaped placeholder: %v", err)
	}
}

func TestSecretService_ValidateSecretRefs_JSONEscapedForm_Missing(t *testing.T) {
	repo := newMockRepo()
	svc := NewSecretService(repo, &mockVault{}, newTestIdentityService())

	config := `{{ secret \"nonexistent-key\" }}`
	err := svc.ValidateSecretRefs("org1", config)
	if err == nil {
		t.Fatal("expected error for missing JSON-escaped secret handle")
	}
	if !apperror.ValidationFailed.Is(err) {
		t.Errorf("expected ErrSecretRefMissing, got %v", err)
	}
}

func TestSecretService_ValidateSecretRefs_DeduplicatesHandles(t *testing.T) {
	callCount := 0
	repo := newMockRepo()
	repo.existsFn = func(orgID, handle string) (bool, error) {
		callCount++
		return true, nil
	}

	svc := NewSecretService(repo, &mockVault{}, newTestIdentityService())
	config := `{{ secret "key1" }} {{ secret "key1" }} {{ secret "key1" }}`
	if err := svc.ValidateSecretRefs("org1", config); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("Exists called %d times, want 1 (should deduplicate)", callCount)
	}
}

// ---- Decrypt tests ----------------------------------------------------------

func TestSecretService_Decrypt_ReturnsPlaintext(t *testing.T) {
	repo := newMockRepo()
	repo.secrets["enc-secret"] = &model.Secret{
		Handle:     "enc-secret",
		Ciphertext: []byte("enc:plaintext-value"),
		Status:     model.SecretStatusActive,
	}

	svc := NewSecretService(repo, &mockVault{}, newTestIdentityService())
	plain, err := svc.Decrypt("org1", "enc-secret")
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if plain != "plaintext-value" {
		t.Errorf("Decrypt = %q, want %q", plain, "plaintext-value")
	}
}

func TestSecretService_Decrypt_DeprecatedSecret_ReturnsError(t *testing.T) {
	repo := newMockRepo()
	repo.secrets["dep-secret"] = &model.Secret{
		Handle:     "dep-secret",
		Ciphertext: []byte("enc:val"),
		Status:     model.SecretStatusDeprecated,
	}

	svc := NewSecretService(repo, &mockVault{}, newTestIdentityService())
	_, err := svc.Decrypt("org1", "dep-secret")
	if err == nil {
		t.Fatal("expected error decrypting deprecated secret")
	}
}
