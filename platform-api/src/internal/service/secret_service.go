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
	"errors"
	"fmt"
	"regexp"
	"time"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/vault"
)

var secretPlaceholderRegex = regexp.MustCompile(`\{\{\s*secret\s+"([^"]+)"\s*\}\}`)

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
		Hash:           hashSecret(req.Value),
		Type:           secretType,
		Provider:       s.vault.ProviderName(),
		Status:         model.SecretStatusActive,
		ValueScope:     model.SecretDefaultValueScope,
		CreatedBy:      createdBy,
		UpdatedBy:      createdBy,
	}

	if err := s.repo.Create(secret); err != nil {
		return nil, fmt.Errorf("failed to persist secret: %w", err)
	}

	resp := secretToResponse(secret)
	resp.Value = req.Value
	return resp, nil
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
	existing.Hash = hashSecret(req.Value)
	existing.UpdatedBy = updatedBy

	if err := s.repo.Update(existing); err != nil {
		return nil, fmt.Errorf("failed to update secret: %w", err)
	}

	resp := secretToResponse(existing)
	resp.Value = req.Value
	return resp, nil
}

func (s *SecretService) Delete(orgID, handle, updatedBy string) error {
	llmRefs, err := s.repo.FindLLMProviderRefs(orgID, handle)
	if err != nil {
		return fmt.Errorf("failed to scan LLM provider references: %w", err)
	}

	apiRefs, err := s.repo.FindAPIRefs(orgID, handle)
	if err != nil {
		return fmt.Errorf("failed to scan API references: %w", err)
	}

	allRefs := append(llmRefs, apiRefs...)
	if len(allRefs) > 0 {
		return &SecretInUseError{References: allRefs}
	}

	return s.repo.SoftDelete(orgID, handle, updatedBy)
}

// ValidateSecretRefs checks that every {{ secret "handle" }} placeholder in configText
// resolves to an active org-scoped secret.
func (s *SecretService) ValidateSecretRefs(orgID, configText string) error {
	matches := secretPlaceholderRegex.FindAllStringSubmatch(configText, -1)
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

		found, _ := s.repo.Exists(orgID, handle)
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

func hashSecret(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return fmt.Sprintf("sha256:%x", sum)
}

func secretToResponse(s *model.Secret) *dto.SecretResponse {
	return &dto.SecretResponse{
		ID:          s.UUID,
		Handle:      s.Handle,
		DisplayName: s.DisplayName,
		Description: s.Description,
		Type:        s.Type,
		Provider:    s.Provider,
		Status:      s.Status,
		Hash:        s.Hash,
		ValueScope: s.ValueScope,
		CreatedAt:   s.CreatedAt,
		CreatedBy:   s.CreatedBy,
		UpdatedAt:   s.UpdatedAt,
		UpdatedBy:   s.UpdatedBy,
	}
}

func secretToSummary(s *model.Secret) *dto.SecretSummary {
	return &dto.SecretSummary{
		ID:          s.UUID,
		Handle:      s.Handle,
		DisplayName: s.DisplayName,
		Description: s.Description,
		Type:        s.Type,
		Provider:    s.Provider,
		Status:      s.Status,
		Hash:        s.Hash,
		ValueScope: s.ValueScope,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}
