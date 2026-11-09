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
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
	"github.com/wso2/api-platform/platform-api/internal/vault"
)

const (
	// DefaultWebhookSecretMaxVersions is the number of versions a webhook secret
	// keeps when no limit is configured.
	DefaultWebhookSecretMaxVersions = 3

	webhookSecretPrefix  = "whsec_"
	webhookSecretBytes   = 32
	webhookSecretMaxLen  = 40
	webhookSecretMaskLen = 4
)

// WebhookSecretService manages org-level secrets that are rotated through a
// bounded set of versions. Every retained version stays valid, so consumers can
// move to a new version before the old one is deleted.
type WebhookSecretService struct {
	repo        repository.WebhookSecretRepository
	vault       vault.SecretVault
	identity    *IdentityService
	maxVersions int
	readOnly    readOnlyMode // TEMP-READ-ONLY-MODE: remove with webhook_secret_readonly.go
}

func NewWebhookSecretService(repo repository.WebhookSecretRepository, v vault.SecretVault, identity *IdentityService, maxVersions int) *WebhookSecretService {
	// A limit of 1 could never rotate (the last version cannot be deleted to make room).
	if maxVersions < 2 {
		maxVersions = DefaultWebhookSecretMaxVersions
	}
	return &WebhookSecretService{repo: repo, vault: v, identity: identity, maxVersions: maxVersions}
}

// Create registers a new secret with its first version.
func (s *WebhookSecretService) Create(ctx context.Context, orgID, actor string, req *dto.CreateWebhookSecretRequest) (*dto.WebhookSecretWithValue, error) {
	if actor == "" {
		return nil, apperror.Unauthorized.New()
	}
	if err := utils.ValidateHandle(req.Handle); err != nil {
		return nil, err
	}
	if len(req.Handle) > webhookSecretMaxLen {
		return nil, apperror.ValidationFailed.New("The id must be at most 40 characters.")
	}
	if req.Name == "" || len(req.Name) > 255 {
		return nil, apperror.ValidationFailed.New("The name is required and must be at most 255 characters.")
	}
	secretType := req.Type
	if secretType == "" {
		secretType = model.WebhookSecretMetadataTypePortal
	} else if secretType != model.WebhookSecretMetadataTypePortal {
		return nil, apperror.ValidationFailed.New("The secret type must be PORTAL.")
	}

	// Creator names are resolved before the write: a lookup failure after it would hide the
	// plaintext, which is returned only once, behind an error for a secret that was stored.
	names, err := s.creatorNames(actor, actor)
	if err != nil {
		return nil, err
	}
	plaintext, version, err := s.newVersion(ctx, actor)
	if err != nil {
		return nil, err
	}
	info := &model.WebhookSecretMetadata{
		OrganizationID: orgID, Handle: req.Handle, Name: req.Name, Type: secretType,
		CreatedBy: actor, UpdatedBy: actor,
	}
	if err := s.repo.Create(info, version); err != nil {
		return nil, err
	}
	return s.withValue(info, version, plaintext, names), nil
}

// Rotate issues a new version. It fails with WebhookSecretVersionLimit when the
// secret already holds the maximum number of versions.
func (s *WebhookSecretService) Rotate(ctx context.Context, orgID, handle, actor string) (*dto.WebhookSecretWithValue, error) {
	if actor == "" {
		return nil, apperror.Unauthorized.New()
	}
	info, err := s.repo.GetMetadata(orgID, handle)
	if err != nil {
		return nil, err
	}
	names, err := s.creatorNames(info.CreatedBy, actor)
	if err != nil {
		return nil, err
	}
	plaintext, version, err := s.newVersion(ctx, actor)
	if err != nil {
		return nil, err
	}
	if err := s.repo.AddVersion(info.UUID, s.maxVersions, version); err != nil {
		return nil, err
	}
	return s.withValue(info, version, plaintext, names), nil
}

func (s *WebhookSecretService) Get(orgID, handle string) (*dto.WebhookSecretInfo, error) {
	info, err := s.repo.GetMetadata(orgID, handle)
	if err != nil {
		return nil, err
	}
	versions, err := s.repo.ListVersions(info.UUID)
	if err != nil {
		return nil, err
	}
	return s.toInfo(info, versions)
}

func (s *WebhookSecretService) List(orgID string, limit, offset int) (*dto.WebhookSecretListResponse, error) {
	infos, total, err := s.repo.ListMetadata(orgID, limit, offset)
	if err != nil {
		return nil, err
	}
	list := make([]*dto.WebhookSecretInfo, 0, len(infos))
	for _, info := range infos {
		item, err := s.toInfo(info, nil)
		if err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return &dto.WebhookSecretListResponse{
		Count: len(list), List: list,
		Pagination: dto.Pagination{Total: total, Offset: offset, Limit: limit},
	}, nil
}

// DeleteVersion frees a version slot. The last remaining version is refused.
func (s *WebhookSecretService) DeleteVersion(orgID, handle, actor string, version int) error {
	if actor == "" {
		return apperror.Unauthorized.New()
	}
	info, err := s.repo.GetMetadata(orgID, handle)
	if err != nil {
		return err
	}
	return s.repo.DeleteVersion(info.UUID, version, actor)
}

// Delete removes the secret and all of its versions.
func (s *WebhookSecretService) Delete(orgID, handle string) error {
	return s.repo.Delete(orgID, handle)
}

// Plaintexts decrypts every retained version of a secret, newest first, for
// callers that verify a signature against any valid version.
func (s *WebhookSecretService) Plaintexts(ctx context.Context, orgID, handle string) ([]string, error) {
	info, err := s.repo.GetMetadata(orgID, handle)
	if err != nil {
		return nil, err
	}
	versions, err := s.repo.ListVersions(info.UUID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(versions))
	for _, v := range versions {
		p, err := s.vault.Decrypt(ctx, v.SecretEnc)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt webhook secret version: %w", err)
		}
		out = append(out, p)
	}
	return out, nil
}

// IssuePortalSecret returns the plaintext of a newly issued version of an API
// portal's webhook secret, creating the secret on first use. It is the
// provisioning path (no user acts), so versions carry no creator. At the version
// limit it fails with WebhookSecretVersionLimit and leaves every version in place:
// whether the oldest one is still needed (a delivery sealed with it may be pending) is
// not known here, so an operator frees a slot with DeleteVersion.
func (s *WebhookSecretService) IssuePortalSecret(ctx context.Context, orgID, portalHandle string) (string, error) {
	if err := s.rejectIfReadOnly(orgID); err != nil { // TEMP-READ-ONLY-MODE: remove with webhook_secret_readonly.go
		return "", err
	}
	handle := portalSecretHandle(portalHandle)
	info, err := s.repo.GetMetadata(orgID, handle)
	if err != nil {
		if !isAppErrorCode(err, apperror.CodeWebhookSecretNotFound) {
			return "", err
		}
		plaintext, version, err := s.newVersion(ctx, "")
		if err != nil {
			return "", err
		}
		info = &model.WebhookSecretMetadata{OrganizationID: orgID, Handle: handle, Name: "Portal " + portalHandle, Type: model.WebhookSecretMetadataTypePortal}
		createErr := s.repo.Create(info, version)
		if createErr == nil {
			return plaintext, nil
		}
		if !isAppErrorCode(createErr, apperror.CodeWebhookSecretExists) {
			return "", createErr
		}
		// A concurrent request created the secret first; add a version to it instead.
		if info, err = s.repo.GetMetadata(orgID, handle); err != nil {
			return "", err
		}
	}

	plaintext, version, err := s.newVersion(ctx, "")
	if err != nil {
		return "", err
	}
	if err := s.repo.AddVersion(info.UUID, s.maxVersions, version); err != nil {
		return "", err
	}
	return plaintext, nil
}

// DeletePortalSecret removes an API portal's webhook secret. Absent is not an error.
func (s *WebhookSecretService) DeletePortalSecret(_ context.Context, orgID, portalHandle string) error {
	if err := s.rejectIfReadOnly(orgID); err != nil { // TEMP-READ-ONLY-MODE: remove with webhook_secret_readonly.go
		return err
	}
	err := s.repo.Delete(orgID, portalSecretHandle(portalHandle))
	if err != nil && !isAppErrorCode(err, apperror.CodeWebhookSecretNotFound) {
		return err
	}
	return nil
}

// portalSecretHandle derives the secret id for a portal. The id column is 40
// characters, so a longer portal handle is cut and suffixed with a hash of the
// full handle to stay unique.
func portalSecretHandle(portalHandle string) string {
	if len(portalHandle) <= webhookSecretMaxLen {
		return portalHandle
	}
	sum := sha256.Sum256([]byte(portalHandle))
	return strings.TrimRight(portalHandle[:webhookSecretMaxLen-9], "-") + "-" + hex.EncodeToString(sum[:4])
}

func isAppErrorCode(err error, code string) bool {
	var appErr *apperror.Error
	return errors.As(err, &appErr) && appErr.Code == code
}

// PortalSecretByHash decrypts the version of an organization's PORTAL secret whose hash is the
// one the portal sent with a delivery. It returns "" when no retained version has that hash
// (never issued, or already deleted), so the caller rejects the delivery rather than guessing.
//
// The plaintext is not cached: it lives only for the verification of one delivery, and a deleted
// version stops resolving on every instance at once.
func (s *WebhookSecretService) PortalSecretByHash(ctx context.Context, orgID, hash string) (string, error) {
	version, err := s.repo.GetVersionByHash(orgID, model.WebhookSecretMetadataTypePortal, hash)
	if err != nil || version == nil {
		return "", err
	}
	plaintext, err := s.vault.Decrypt(ctx, version.SecretEnc)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt webhook secret version: %w", err)
	}
	return plaintext, nil
}

// PortalSecrets decrypts every retained version of every PORTAL secret in an organization,
// newest first. Used only for deliveries from a portal that predates secret_hash.
func (s *WebhookSecretService) PortalSecrets(ctx context.Context, orgID string) ([]string, error) {
	versions, err := s.repo.ListVersionsForOrg(orgID, model.WebhookSecretMetadataTypePortal)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(versions))
	for _, v := range versions {
		p, err := s.vault.Decrypt(ctx, v.SecretEnc)
		if err != nil {
			continue // one unreadable version must not hide the others from a verifier
		}
		out = append(out, p)
	}
	return out, nil
}

// newVersion generates a secret value and its encrypted, hashed, masked row.
func (s *WebhookSecretService) newVersion(ctx context.Context, actor string) (string, *model.WebhookSecret, error) {
	plaintext, err := generateWebhookSecretValue()
	if err != nil {
		return "", nil, err
	}
	enc, err := s.vault.Encrypt(ctx, plaintext)
	if err != nil {
		return "", nil, fmt.Errorf("failed to encrypt webhook secret: %w", err)
	}
	return plaintext, &model.WebhookSecret{
		SecretEnc:    enc,
		SecretHash:   utils.WebhookSecretHash(plaintext),
		MaskedSecret: maskWebhookSecret(plaintext),
		CreatedBy:    actor,
	}, nil
}

// creatorNames are the display names shown for a secret and for its newly issued version.
type creatorNames struct{ secret, version string }

// creatorNames resolves both names; callers do it before writing, so building the response of
// a write that has happened cannot fail.
func (s *WebhookSecretService) creatorNames(secretCreator, versionCreator string) (creatorNames, error) {
	secret, err := s.displayCreator(secretCreator)
	if err != nil {
		return creatorNames{}, err
	}
	version, err := s.displayCreator(versionCreator)
	if err != nil {
		return creatorNames{}, err
	}
	return creatorNames{secret: secret, version: version}, nil
}

func (s *WebhookSecretService) withValue(info *model.WebhookSecretMetadata, v *model.WebhookSecret, plaintext string, names creatorNames) *dto.WebhookSecretWithValue {
	return &dto.WebhookSecretWithValue{
		WebhookSecretInfo: dto.WebhookSecretInfo{
			Handle: info.Handle, Name: info.Name, Type: info.Type,
			CreatedBy: names.secret, CreatedAt: info.CreatedAt,
			Versions: []dto.WebhookSecretVersion{{
				Version: v.Version, MaskedSecret: v.MaskedSecret, Hash: v.SecretHash,
				CreatedBy: names.version, CreatedAt: v.CreatedAt,
			}},
		},
		Secret: plaintext,
	}
}

func (s *WebhookSecretService) toInfo(info *model.WebhookSecretMetadata, versions []*model.WebhookSecret) (*dto.WebhookSecretInfo, error) {
	createdBy, err := s.displayCreator(info.CreatedBy)
	if err != nil {
		return nil, err
	}
	out := &dto.WebhookSecretInfo{
		Handle: info.Handle, Name: info.Name, Type: info.Type,
		CreatedBy: createdBy, CreatedAt: info.CreatedAt,
	}
	for _, v := range versions {
		vBy, err := s.displayCreator(v.CreatedBy)
		if err != nil {
			return nil, err
		}
		out.Versions = append(out.Versions, dto.WebhookSecretVersion{
			Version: v.Version, MaskedSecret: v.MaskedSecret, Hash: v.SecretHash,
			CreatedBy: vBy, CreatedAt: v.CreatedAt,
		})
	}
	return out, nil
}

// displayCreator resolves a creator for display; rows written by the platform itself
// (provisioning) have none and stay empty rather than showing as a deleted user.
func (s *WebhookSecretService) displayCreator(createdBy string) (string, error) {
	if createdBy == "" {
		return "", nil
	}
	return s.identity.DisplayNameForUUID(createdBy)
}

// generateWebhookSecretValue returns "whsec_" followed by 32 random bytes in hex.
func generateWebhookSecretValue() (string, error) {
	b := make([]byte, webhookSecretBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate webhook secret: %w", err)
	}
	return webhookSecretPrefix + hex.EncodeToString(b), nil
}

// maskWebhookSecret keeps the prefix and the last characters only.
func maskWebhookSecret(secret string) string {
	if len(secret) <= len(webhookSecretPrefix)+webhookSecretMaskLen {
		return webhookSecretPrefix + "****"
	}
	return webhookSecretPrefix + "****" + secret[len(secret)-webhookSecretMaskLen:]
}
