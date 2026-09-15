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
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
	"github.com/wso2/api-platform/platform-api/internal/vault"
)

// validateAPIPortalURL requires https + absolute URL with host to block stored SSRF vectors (file://, javascript:, plain-http metadata endpoints).
func validateAPIPortalURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return "", apperror.ValidationFailed.New("The url field is not a valid URL.")
	}
	if !u.IsAbs() || u.Host == "" {
		return "", apperror.ValidationFailed.New("The url field must be an absolute URL with a host.")
	}
	if u.Scheme != "https" {
		return "", apperror.ValidationFailed.New("The url field must use the https scheme.")
	}
	return u.String(), nil
}

// sharedKeyPattern matches the 64-hex-char shape produced by `openssl rand -hex 32`, which the portal middleware sha256s for verification.
var sharedKeyPattern = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

// validateAndEncryptSharedKey returns AES-GCM ciphertext of the raw key; plaintext is discarded on return.
func validateAndEncryptSharedKey(v vault.SecretVault, raw string) ([]byte, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, apperror.ValidationFailed.New("The sharedKey field is required.")
	}
	if !sharedKeyPattern.MatchString(trimmed) {
		return nil, apperror.ValidationFailed.New(
			"The sharedKey field must be a 64-character hex string (32 bytes of entropy, matches `openssl rand -hex 32`).")
	}
	ciphertext, err := v.Encrypt(context.Background(), trimmed)
	if err != nil {
		return nil, err
	}
	return ciphertext, nil
}

// APIPortalService encapsulates business logic for the /api-portals resource.
type APIPortalService struct {
	portalRepo   repository.APIPortalRepository
	orgRepo      repository.OrganizationRepository
	auditRepo    repository.AuditRepository
	vault        vault.SecretVault
	authRegistry *APIPortalAuthRegistry
	identity     *IdentityService
	slogger      *slog.Logger
}

// NewAPIPortalService constructs an APIPortalService.
func NewAPIPortalService(
	portalRepo repository.APIPortalRepository,
	orgRepo repository.OrganizationRepository,
	auditRepo repository.AuditRepository,
	secretVault vault.SecretVault,
	authRegistry *APIPortalAuthRegistry,
	identity *IdentityService,
	slogger *slog.Logger,
) *APIPortalService {
	return &APIPortalService{
		portalRepo:   portalRepo,
		orgRepo:      orgRepo,
		auditRepo:    auditRepo,
		vault:        secretVault,
		authRegistry: authRegistry,
		identity:     identity,
		slogger:      slogger,
	}
}

// invalidateCachedAuthProvider drops the cached provider; no-op when registry is nil (tests without outbound auth).
func (s *APIPortalService) invalidateCachedAuthProvider(portalHandle, orgID string) {
	if s.authRegistry == nil {
		return
	}
	s.authRegistry.Invalidate(portalHandle, orgID)
}

// AuthHeaderForPortal returns the "SharedKey <raw>" Authorization header for outbound calls to the portal's admin API.
func (s *APIPortalService) AuthHeaderForPortal(ctx context.Context, portalHandle, orgID string) (string, error) {
	if s.authRegistry == nil {
		return "", fmt.Errorf("shared-key AuthProvider registry is not initialised")
	}
	provider, err := s.authRegistry.Get(portalHandle, orgID)
	if err != nil {
		return "", err
	}
	return provider.AuthorizationHeader(ctx)
}

// PaginationInfo is the {total, offset, limit} triplet used to build the list-response envelope.
type PaginationInfo struct {
	Total  int
	Offset int
	Limit  int
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// CreateAPIPortal validates the request, enforces handle uniqueness, encrypts the shared key, and inserts a row scoped to orgID.
func (s *APIPortalService) CreateAPIPortal(req *api.CreateApiPortalRequest, orgID, createdBy string) (*api.ApiPortalResponse, error) {
	if req == nil {
		return nil, apperror.ValidationFailed.New("The request body is required.")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, apperror.ValidationFailed.New("The name field is required.")
	}
	if err := utils.ValidateHandle(strings.TrimSpace(req.Handle)); err != nil {
		return nil, err
	}
	portalURL, err := validateAPIPortalURL(req.Url)
	if err != nil {
		return nil, err
	}
	if portalURL == "" {
		return nil, apperror.ValidationFailed.New("The url field is required.")
	}
	encryptedKey, err := validateAndEncryptSharedKey(s.vault, derefStr(req.SharedKey))
	if err != nil {
		return nil, err
	}

	org, err := s.orgRepo.GetOrganizationByUUID(orgID)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, apperror.OrganizationNotFound.New()
	}

	exists, err := s.portalRepo.Exists(strings.TrimSpace(req.Handle), orgID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperror.APIPortalExists.New()
	}

	actor := strings.TrimSpace(createdBy)
	portal := &model.APIPortal{
		ID:              uuid.New().String(),
		OrganizationID:  orgID,
		Handle:          strings.TrimSpace(req.Handle),
		Name:            name,
		Description:     strings.TrimSpace(derefStr(req.Description)),
		URL:             portalURL,
		Status:          constants.APIPortalStatusActive,
		InternalAuthKey: encryptedKey,
		Metadata:        derefAPIPortalMetadata(req.Metadata),
		CreatedBy:       actor,
		UpdatedBy:       actor,
	}

	if err := s.portalRepo.Create(portal); err != nil {
		if repository.IsUniqueViolation(err) {
			// Concurrent create won the race between Exists and INSERT.
			return nil, apperror.APIPortalExists.New()
		}
		return nil, err
	}
	_ = s.auditRepo.Record("CREATE", portal.ID, "api_portal", orgID, actor)
	return ModelToAPIPortalResponse(portal), nil
}

// GetAPIPortal returns a single API Portal identified by its handle within orgID.
func (s *APIPortalService) GetAPIPortal(handle, orgID string) (*api.ApiPortalResponse, error) {
	portal, err := s.portalRepo.GetByHandleAndOrgID(strings.TrimSpace(handle), orgID)
	if err != nil {
		return nil, err
	}
	if portal == nil {
		return nil, apperror.APIPortalNotFound.New()
	}
	return ModelToAPIPortalResponse(portal), nil
}

// ListAPIPortals returns a page of API Portals in the organization; Limit/Offset are normalized here.
func (s *APIPortalService) ListAPIPortals(orgID string, limit, offset int, sortBy, sortOrder, search string) (*api.ApiPortalListResponse, error) {
	org, err := s.orgRepo.GetOrganizationByUUID(orgID)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, apperror.OrganizationNotFound.New()
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	total, err := s.portalRepo.Count(orgID, search)
	if err != nil {
		return nil, err
	}
	opts := repository.ListOptions{
		Limit:     limit,
		Offset:    offset,
		SortBy:    sortBy,
		SortOrder: sortOrder,
		Search:    search,
	}
	page, err := s.portalRepo.ListPaginated(orgID, opts)
	if err != nil {
		return nil, err
	}
	return buildAPIPortalListResponse(page, PaginationInfo{Total: total, Offset: offset, Limit: limit}), nil
}

// UpdateAPIPortal applies whitelisted mutations from req; nil pointer fields mean "not sent" and are left unchanged. A non-nil SharedKey re-encrypts and rotates the stored value.
func (s *APIPortalService) UpdateAPIPortal(handle string, req *api.UpdateApiPortalRequest, orgID, updatedBy string) (*api.ApiPortalResponse, error) {
	if req == nil {
		return nil, apperror.ValidationFailed.New("The request body is required.")
	}
	portal, err := s.portalRepo.GetByHandleAndOrgID(strings.TrimSpace(handle), orgID)
	if err != nil {
		return nil, err
	}
	if portal == nil {
		return nil, apperror.APIPortalNotFound.New()
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, apperror.ValidationFailed.New("The name field cannot be empty.")
		}
		portal.Name = name
	}
	if req.Description != nil {
		portal.Description = strings.TrimSpace(*req.Description)
	}
	if req.Url != nil {
		portalURL, err := validateAPIPortalURL(*req.Url)
		if err != nil {
			return nil, err
		}
		if portalURL == "" {
			return nil, apperror.ValidationFailed.New("The url field cannot be empty.")
		}
		portal.URL = portalURL
	}
	if req.SharedKey != nil {
		encryptedKey, err := validateAndEncryptSharedKey(s.vault, *req.SharedKey)
		if err != nil {
			return nil, err
		}
		portal.InternalAuthKey = encryptedKey
	}
	if req.Metadata != nil {
		// Supplied metadata map fully replaces stored (no per-key merge).
		portal.Metadata = derefAPIPortalMetadata(req.Metadata)
	}
	portal.UpdatedBy = strings.TrimSpace(updatedBy)

	if err := s.portalRepo.Update(portal); err != nil {
		return nil, err
	}
	_ = s.auditRepo.Record("UPDATE", portal.ID, "api_portal", orgID, portal.UpdatedBy)
	// Config may have changed; drop cached AuthProvider so next call rebuilds.
	s.invalidateCachedAuthProvider(portal.Handle, portal.OrganizationID)
	return ModelToAPIPortalResponse(portal), nil
}

// DeleteAPIPortal removes the API Portal identified by its handle within orgID.
func (s *APIPortalService) DeleteAPIPortal(handle, orgID, actor string) error {
	portal, err := s.portalRepo.GetByHandleAndOrgID(strings.TrimSpace(handle), orgID)
	if err != nil {
		return err
	}
	if portal == nil {
		return apperror.APIPortalNotFound.New()
	}
	if err := s.portalRepo.Delete(portal.ID, orgID); err != nil {
		return err
	}
	_ = s.auditRepo.Record("DELETE", portal.ID, "api_portal", orgID, strings.TrimSpace(actor))
	s.invalidateCachedAuthProvider(portal.Handle, portal.OrganizationID)
	return nil
}
