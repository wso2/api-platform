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
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/vault"
)


type SecretInUseError struct {
	References []model.SecretReference
}

func (e *SecretInUseError) Error() string {
	return constants.ErrSecretInUse.Error()
}

type SecretService struct {
	repo  repository.SecretRepository
	vault vault.SecretVault
}

func NewSecretService(repo repository.SecretRepository, v vault.SecretVault) *SecretService {
	return &SecretService{repo: repo, vault: v}
}

func (s *SecretService) Create(orgID, createdBy string, req *dto.CreateSecretRequest) (*dto.SecretResponse, error) {
	secretType := req.Type
	if secretType == "" {
		secretType = model.SecretTypeGeneric
	} else if secretType != model.SecretTypeGeneric && secretType != model.SecretTypeCertificate {
		return nil, constants.ErrInvalidSecretType
	}

	exists, err := s.repo.Exists(orgID, req.Handle)
	if err != nil {
		return nil, fmt.Errorf("failed to check secret existence: %w", err)
	}
	if exists {
		return nil, constants.ErrSecretAlreadyExists
	}

	ciphertext, err := s.vault.Encrypt(context.Background(), req.Value)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt secret: %w", err)
	}

	secret := &model.Secret{
		OrganizationID: orgID,
		Handle:         req.Handle,
		DisplayName:    req.DisplayName,
		Description:    req.Description,
		Ciphertext:     ciphertext,
		Hash:           hashSecret(s.vault.HashKey(), req.Value),
		Type:           secretType,
		Provider:       s.vault.ProviderName(),
		Status:         model.SecretStatusActive,
		CreatedBy:      createdBy,
		UpdatedBy:      createdBy,
		Scopes: []model.SecretScope{
			{Scope: model.SecretScopeTypeOrg, ScopeValue: orgID},
		},
	}

	if err := s.repo.Create(secret); err != nil {
		return nil, fmt.Errorf("failed to persist secret: %w", err)
	}

	return secretToResponse(secret), nil
}

func (s *SecretService) List(orgID string, limit, offset int, updatedAfter *time.Time) (*dto.SecretListResponse, error) {
	secrets, err := s.repo.List(orgID, limit, offset, updatedAfter)
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	total, err := s.repo.Count(orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to count secrets: %w", err)
	}

	summaries := make([]*dto.SecretSummary, 0, len(secrets))
	for _, sec := range secrets {
		summaries = append(summaries, secretToSummary(sec))
	}

	return &dto.SecretListResponse{
		List: summaries,
		Pagination: dto.Pagination{
			Total:  total,
			Limit:  limit,
			Offset: offset,
		},
	}, nil
}

func (s *SecretService) Get(orgID, handle string) (*dto.SecretSummary, error) {
	secret, err := s.repo.GetByHandle(orgID, handle)
	if err != nil {
		return nil, err
	}
	return secretToSummary(secret), nil
}

func (s *SecretService) Update(orgID, handle, updatedBy string, req *dto.UpdateSecretRequest) (*dto.SecretResponse, error) {
	existing, err := s.repo.GetByHandle(orgID, handle)
	if err != nil {
		return nil, err
	}

	ciphertext, err := s.vault.Encrypt(context.Background(), req.Value)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt secret: %w", err)
	}

	if req.DisplayName != "" {
		existing.DisplayName = req.DisplayName
	}
	if req.Description != "" {
		existing.Description = req.Description
	}
	existing.Ciphertext = ciphertext
	existing.Hash = hashSecret(s.vault.HashKey(), req.Value)
	existing.UpdatedBy = updatedBy
	// Rotation is an explicit intent to put the secret back into service.
	existing.Status = model.SecretStatusActive

	if err := s.repo.Update(existing); err != nil {
		return nil, fmt.Errorf("failed to update secret: %w", err)
	}

	return secretToResponse(existing), nil
}

func (s *SecretService) Delete(orgID, handle, updatedBy string) error {
	refs, err := s.repo.FindRefsAndSoftDelete(orgID, handle, updatedBy)
	if err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}
	if len(refs) > 0 {
		return &SecretInUseError{References: refs}
	}
	return nil
}

// ValidateSecretRefs checks that every {{ secret "handle" }} placeholder in configText
// resolves to an active org-scoped secret.
func (s *SecretService) ValidateSecretRefs(orgID, configText string) error {
	matches := constants.SecretPlaceholderRe.FindAllStringSubmatch(configText, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{})
	var missing []string

	for _, m := range matches {
		handle := m[1]
		if _, already := seen[handle]; already {
			continue
		}
		seen[handle] = struct{}{}

		found, err := s.repo.Exists(orgID, handle)
		if err != nil {
			return fmt.Errorf("failed to check existence of secret %q: %w", handle, err)
		}
		if !found {
			missing = append(missing, handle)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("%w: %v", constants.ErrSecretRefMissing, missing)
	}
	return nil
}

// Decrypt returns the plaintext value of a secret — intended for internal GW use only.
func (s *SecretService) Decrypt(orgID, handle string) (string, error) {
	secret, err := s.repo.GetByHandle(orgID, handle)
	if err != nil {
		return "", err
	}
	if secret.Status == model.SecretStatusDeprecated {
		return "", errors.New("secret is deprecated")
	}
	return s.vault.Decrypt(context.Background(), secret.Ciphertext)
}

// DecryptCiphertext decrypts an already-fetched ciphertext blob directly, without a
// database round-trip. Used in the bulk includeValues=true loop where the caller
// already holds the model.Secret rows.
func (s *SecretService) DecryptCiphertext(ciphertext []byte) (string, error) {
	return s.vault.Decrypt(context.Background(), ciphertext)
}

// hashSecret returns a keyed HMAC-SHA256 digest of plaintext, prefixed with "hmac-sha256:".
// Using HMAC instead of bare SHA-256 prevents offline dictionary attacks against the hash
// values returned in list/get/sync responses.
func hashSecret(key []byte, plaintext string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(plaintext))
	return fmt.Sprintf("hmac-sha256:%x", mac.Sum(nil))
}

func secretToResponse(s *model.Secret) *dto.SecretResponse {
	return &dto.SecretResponse{
		Handle:      s.Handle,
		DisplayName: s.DisplayName,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

func secretToSummary(s *model.Secret) *dto.SecretSummary {
	return &dto.SecretSummary{
		Handle:      s.Handle,
		DisplayName: s.DisplayName,
		Description: s.Description,
		Type:        s.Type,
		Provider:    s.Provider,
		Status:      s.Status,
		Hash:        s.Hash,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}
