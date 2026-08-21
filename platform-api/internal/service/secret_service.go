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
	"log/slog"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/vault"
)

type SecretInUseError struct {
	References []model.SecretReference
}

func (e *SecretInUseError) Error() string {
	return "secret is referenced by one or more resources"
}

type SecretService struct {
	repo          repository.SecretRepository
	vault         vault.SecretVault
	identity      *IdentityService
	gatewayRepo   repository.GatewayRepository
	gatewayEvents *GatewayEventsService
	slogger       *slog.Logger
}

func NewSecretService(repo repository.SecretRepository, v vault.SecretVault, identity *IdentityService) *SecretService {
	return &SecretService{repo: repo, vault: v, identity: identity, slogger: slog.Default()}
}

// WithGatewayBroadcast enables best-effort secret.updated/secret.deleted
// notifications to every gateway in the organization on rotate/delete (see
// broadcastSecretEvent). Optional: a SecretService built without this call simply
// skips broadcasting — kept as a post-construction setter rather than a constructor
// parameter so the many existing unit tests that build a bare SecretService don't
// all need updating for a purely additive, best-effort side channel.
func (s *SecretService) WithGatewayBroadcast(gatewayRepo repository.GatewayRepository, gatewayEvents *GatewayEventsService) *SecretService {
	s.gatewayRepo = gatewayRepo
	s.gatewayEvents = gatewayEvents
	return s
}

// toSecretResponse converts secret via secretToResponse and resolves its
// createdBy/updatedBy UUIDs to their raw external identity.
func (s *SecretService) toSecretResponse(secret *model.Secret) (*dto.SecretResponse, error) {
	resp := secretToResponse(secret)
	if resp == nil {
		return nil, nil
	}
	createdBy, err := s.identity.SubForUUID(resp.CreatedBy)
	if err != nil {
		return nil, err
	}
	resp.CreatedBy = createdBy
	updatedBy, err := s.identity.SubForUUID(resp.UpdatedBy)
	if err != nil {
		return nil, err
	}
	resp.UpdatedBy = updatedBy
	return resp, nil
}

// toSecretSummary converts secret via secretToSummary and resolves its
// createdBy UUID to its raw external identity.
func (s *SecretService) toSecretSummary(secret *model.Secret) (*dto.SecretSummary, error) {
	resp := secretToSummary(secret)
	if resp == nil {
		return nil, nil
	}
	createdBy, err := s.identity.SubForUUID(resp.CreatedBy)
	if err != nil {
		return nil, err
	}
	resp.CreatedBy = createdBy
	return resp, nil
}

func (s *SecretService) Create(orgID, createdBy string, req *dto.CreateSecretRequest) (*dto.SecretResponse, error) {
	if err := validateSecretHandle(req.Handle); err != nil {
		return nil, err
	}

	secretType := req.Type
	if secretType == "" {
		secretType = model.SecretTypeGeneric
	} else if secretType != model.SecretTypeGeneric && secretType != model.SecretTypeCertificate {
		return nil, apperror.ValidationFailed.New("The secret type must be one of GENERIC or CERTIFICATE.")
	}

	exists, err := s.repo.Exists(orgID, req.Handle)
	if err != nil {
		return nil, fmt.Errorf("failed to check secret existence: %w", err)
	}
	if exists {
		return nil, apperror.SecretExists.New()
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

	return s.toSecretResponse(secret)
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
		summary, err := s.toSecretSummary(sec)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}

	return &dto.SecretListResponse{
		Count: len(summaries),
		List:  summaries,
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
	return s.toSecretSummary(secret)
}

func (s *SecretService) Update(orgID, handle, updatedBy string, req *dto.UpdateSecretRequest) (*dto.SecretResponse, error) {
	existing, err := s.repo.GetByHandle(orgID, handle)
	if err != nil {
		return nil, err
	}

	if req.DisplayName != "" {
		existing.DisplayName = req.DisplayName
	}
	if req.Description != "" {
		existing.Description = req.Description
	}
	// Value is optional: a metadata-only edit (no value) must not touch the
	// ciphertext/hash or reactivate a deprecated secret — only an explicit
	// rotation is an intent to put the secret back into service.
	if req.Value != "" {
		ciphertext, err := s.vault.Encrypt(context.Background(), req.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt secret: %w", err)
		}
		existing.Ciphertext = ciphertext
		existing.Hash = hashSecret(s.vault.HashKey(), req.Value)
		existing.Status = model.SecretStatusActive
	}
	existing.UpdatedBy = updatedBy

	if err := s.repo.Update(existing); err != nil {
		return nil, fmt.Errorf("failed to update secret: %w", err)
	}

	// Build the response before broadcasting: toSecretResponse can still fail
	// (identity-mapping lookups), and the caller must not be told the rotation
	// failed after gateways have already been notified of it. If this errors,
	// the DB commit above still stands — a retry will simply broadcast then.
	resp, err := s.toSecretResponse(existing)
	if err != nil {
		return nil, err
	}

	s.broadcastSecretEvent(orgID, "updated", &model.SecretUpdatedEvent{
		Handle:      existing.Handle,
		DisplayName: existing.DisplayName,
		Hash:        existing.Hash,
		// existing.UpdatedAt was just set in-place by s.repo.Update above — see
		// model.SecretUpdatedEvent.Revision for why UnixNano() is a safe ordering token.
		Revision: existing.UpdatedAt.UnixNano(),
	})

	return resp, nil
}

// GetReferences returns the resources that currently reference handle, so a
// caller can show why a secret is in use before attempting a delete. Existence
// is checked first so an unknown handle reports SecretNotFound rather than an
// empty usages list indistinguishable from "not referenced".
func (s *SecretService) GetReferences(orgID, handle string) ([]dto.SecretReferenceDTO, error) {
	if _, err := s.repo.GetByHandle(orgID, handle); err != nil {
		return nil, err
	}

	refs, err := s.repo.FindRefs(orgID, handle)
	if err != nil {
		return nil, fmt.Errorf("failed to find secret references: %w", err)
	}

	result := make([]dto.SecretReferenceDTO, 0, len(refs))
	for _, ref := range refs {
		result = append(result, dto.SecretReferenceDTO{Type: ref.Type, Handle: ref.Handle, Name: ref.Name})
	}
	return result, nil
}

func (s *SecretService) Delete(orgID, handle, updatedBy string) error {
	// Captured before the delete so the broadcast Revision reflects the same
	// moment as the DB write.
	deletedAt := time.Now().UTC()

	refs, err := s.repo.FindRefsAndDelete(orgID, handle)
	if err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}
	if len(refs) > 0 {
		return &SecretInUseError{References: refs}
	}

	s.broadcastSecretEvent(orgID, "deleted", &model.SecretDeletedEvent{
		Handle:   handle,
		Revision: deletedAt.UnixNano(),
	})

	return nil
}

// CleanupOrphanedSecrets permanently deletes any of the given handles that are no
// longer referenced by anything, once the resource that used to hold one of these
// references (an LLM provider/proxy or MCP proxy) has already been deleted. Best-effort:
// a handle still referenced by some other resource, already gone, or any other error is
// logged and skipped rather than failing the caller's delete — the resource is already
// durably deleted by the time this runs.
func (s *SecretService) CleanupOrphanedSecrets(orgID string, handles []string, deletedBy string) {
	for _, handle := range handles {
		if err := s.Delete(orgID, handle, deletedBy); err != nil {
			var inUseErr *SecretInUseError
			if errors.As(err, &inUseErr) || apperror.SecretNotFound.Is(err) {
				continue // still referenced elsewhere, or already gone — expected, not a failure
			}
			s.slogger.Warn("failed to clean up orphaned secret after resource deletion",
				"handle", handle, "orgId", orgID, "err", err)
		}
	}
}

// broadcastSecretEvent sends a secret.* event to every gateway in the organization.
// Best-effort: a load or delivery failure is logged and swallowed rather than failing
// the rotate/delete call — the change is already durably committed, and a gateway that
// misses the push still catches up via its own poll-based incremental sync (see
// docs/specs/secrets-management.md §6.5-6.7). Also a safe no-op when WithGatewayBroadcast
// was never called (e.g. in unit tests).
func (s *SecretService) broadcastSecretEvent(orgUUID, action string, payload interface{}) {
	if s.gatewayEvents == nil || s.gatewayRepo == nil {
		return
	}
	gateways, err := s.gatewayRepo.GetByOrganizationID(orgUUID)
	if err != nil {
		s.slogger.Warn("Failed to load gateways for secret broadcast",
			"orgId", orgUUID, "action", action, "error", err)
		return
	}
	for _, gw := range gateways {
		if gw == nil || gw.ID == "" {
			continue
		}
		var broadcastErr error
		switch action {
		case "updated":
			broadcastErr = s.gatewayEvents.BroadcastSecretUpdatedEvent(gw.ID, payload.(*model.SecretUpdatedEvent))
		case "deleted":
			broadcastErr = s.gatewayEvents.BroadcastSecretDeletedEvent(gw.ID, payload.(*model.SecretDeletedEvent))
		}
		if broadcastErr != nil {
			s.slogger.Warn("Failed to broadcast secret event",
				"gatewayId", gw.ID, "action", action, "error", broadcastErr)
		}
	}
}

// extractSecretHandle returns the handle embedded in a {{ secret "handle" }}
// placeholder, or "" if value is empty, plaintext, or otherwise not a placeholder.
func extractSecretHandle(value string) string {
	m := constants.SecretPlaceholderRe.FindStringSubmatch(value)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// mainUpstreamAuthValue nil-safely reads upstream.main.auth.value from an
// UpstreamConfig, returning "" when any part of the chain is nil.
func mainUpstreamAuthValue(cfg *model.UpstreamConfig) string {
	if cfg == nil || cfg.Main == nil || cfg.Main.Auth == nil {
		return ""
	}
	return cfg.Main.Auth.Value
}

// sandboxUpstreamAuthValue nil-safely reads upstream.sandbox.auth.value from
// an UpstreamConfig, returning "" when any part of the chain is nil.
func sandboxUpstreamAuthValue(cfg *model.UpstreamConfig) string {
	if cfg == nil || cfg.Sandbox == nil || cfg.Sandbox.Auth == nil {
		return ""
	}
	return cfg.Sandbox.Auth.Value
}

// upstreamAuthValue nil-safely reads .Value from an UpstreamAuth.
func upstreamAuthValue(auth *model.UpstreamAuth) string {
	if auth == nil {
		return ""
	}
	return auth.Value
}

// cleanupRotatedSecret best-effort deletes the secret previously referenced by
// oldValue when it has been rotated to a different handle in newValue. Both
// values are expected to be {{ secret "handle" }} placeholders (or empty/
// plaintext, in which case nothing is deleted). Must be called only after the
// resource's own config has been persisted with newValue, so the old handle
// is no longer referenced by this resource by the time the in-use check runs.
func (s *SecretService) cleanupRotatedSecret(orgUUID, oldValue, newValue, updatedBy string, logger *slog.Logger) {
	oldHandle := extractSecretHandle(oldValue)
	if oldHandle == "" {
		return
	}
	if oldHandle == extractSecretHandle(newValue) {
		return
	}
	if err := s.Delete(orgUUID, oldHandle, updatedBy); err != nil && logger != nil {
		logger.Warn("could not delete rotated-out secret", "handle", oldHandle, "err", err)
	}
}

// ValidateSecretRefs checks that every {{ secret "handle" }} placeholder in configText
// resolves to an active org-scoped secret. Missing and deprecated handles are reported
// separately (§5.6: a deprecated secret exists but cannot be referenced by new/updated
// resources) so the caller isn't told a real, existing-but-retired handle "does not exist".
func (s *SecretService) ValidateSecretRefs(orgID, configText string) error {
	matches := constants.SecretPlaceholderRe.FindAllStringSubmatch(configText, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{})
	var missing []string
	var deprecated []string

	for _, m := range matches {
		handle := m[1]
		if _, already := seen[handle]; already {
			continue
		}
		seen[handle] = struct{}{}

		secret, err := s.repo.GetByHandle(orgID, handle)
		if err != nil {
			if apperror.SecretNotFound.Is(err) {
				missing = append(missing, handle)
				continue
			}
			return fmt.Errorf("failed to check existence of secret %q: %w", handle, err)
		}
		if secret.Status == model.SecretStatusDeprecated {
			deprecated = append(deprecated, handle)
		}
	}

	if len(missing) == 0 && len(deprecated) == 0 {
		return nil
	}

	var parts []string
	if len(missing) > 0 {
		parts = append(parts, fmt.Sprintf("do not exist: %s", strings.Join(missing, ", ")))
	}
	if len(deprecated) > 0 {
		parts = append(parts, fmt.Sprintf("are deprecated and cannot be referenced by new or updated resources: %s", strings.Join(deprecated, ", ")))
	}
	return apperror.ValidationFailed.New(fmt.Sprintf(
		"The following referenced secrets %s.", strings.Join(parts, "; ")))
}

// validateSecretHandle enforces the same handle shape the AI Workspace UI generates
// (constants.SecretHandlePattern) and the DB column's length limit
// (constants.SecretHandleMaxLength), so a non-UI caller cannot create a secret the
// UI itself could never produce — see constants.SecretHandlePattern's doc comment
// for why this matters (unreachable-via-router handles, placeholder-regex interference).
func validateSecretHandle(handle string) error {
	if handle == "" {
		return apperror.ValidationFailed.New("A secret handle is required.")
	}
	if len(handle) > constants.SecretHandleMaxLength {
		return apperror.ValidationFailed.New(fmt.Sprintf(
			"Secret handle must not exceed %d characters.", constants.SecretHandleMaxLength))
	}
	if !constants.SecretHandlePattern.MatchString(handle) {
		return apperror.ValidationFailed.New(
			"Secret handle may only contain lowercase letters, numbers, and single hyphens (no leading, trailing, or doubled hyphens).")
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
		CreatedBy:   s.CreatedBy,
		UpdatedBy:   s.UpdatedBy,
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
		CreatedBy:   s.CreatedBy,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}
