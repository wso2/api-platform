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
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
	"github.com/wso2/api-platform/platform-api/internal/vault"
)

// validateAPIPortalURL enforces input-time constraints on a caller-supplied
// portal URL:
//   - Empty is valid — the URL is populated later by the provisioner in the
//     cloud flow, and OSS may register a portal before the URL is known.
//   - Non-empty must parse as an absolute URL with a host, and use the https
//     scheme. This blocks stored SSRF via `file://`, `javascript:`, and any
//     plain-http URL that could be pointed at instance-metadata endpoints such
//     as http://169.254.169.254/.
//
// Deeper outbound-hardening (private-IP blocklist, DNS-rebinding checks,
// redirect controls) is intentionally NOT enforced here — it belongs in the
// shared outbound HTTP client the publisher will build later, so every
// outbound integration gets the same protection uniformly.
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

// validateAPIPortalSTSTokenURL runs the same base checks as validateAPIPortalURL
// on `authConfig.stsTokenUrl` — the target of the outbound `client_credentials`
// token request that carries clientSecret. Empty is rejected because the
// oauth2 grant needs an endpoint; a required-field check upstream also
// enforces this, but keeping it here means callers see a clear message.
//
// Host-based restrictions (loopback / private / link-local / metadata
// literals, DNS-based resolve-and-recheck) are intentionally NOT enforced
// here — a legitimate local / on-prem deployment can have its STS at
// https://localhost:9443 or a private-range address. Operator-aware egress
// controls are planned as a shared outbound HTTP client feature; the same
// deferral applies to `validateAPIPortalURL`.
func validateAPIPortalSTSTokenURL(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return apperror.ValidationFailed.New("The stsTokenUrl field is required.")
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return apperror.ValidationFailed.New("The stsTokenUrl field is not a valid URL.")
	}
	if !u.IsAbs() || u.Host == "" {
		return apperror.ValidationFailed.New("The stsTokenUrl field must be an absolute URL with a host.")
	}
	if u.Scheme != "https" {
		return apperror.ValidationFailed.New("The stsTokenUrl field must use the https scheme.")
	}
	return nil
}

// APIPortalService encapsulates business logic for the /api-portals resource.
// The handler layer translates OpenAPI-generated request/response DTOs into
// the service's own request structs so the service stays independent of the
// generated code.
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

// invalidateCachedAuthProvider is a no-op when the service was constructed
// without a registry (e.g. in unit tests that don't need outbound auth). Keeps
// call sites clean of nil checks.
func (s *APIPortalService) invalidateCachedAuthProvider(portalHandle string) {
	if s.authRegistry == nil {
		return
	}
	s.authRegistry.Invalidate(portalHandle)
}

// validateAPIPortalAuthConfig enforces per-authType constraints on the config
// map. For `local` the map must be empty; for `oauth2` all required keys must
// be present and non-empty strings, and no unknown keys are allowed.
func validateAPIPortalAuthConfig(authType string, cfg map[string]interface{}) error {
	switch authType {
	case constants.APIPortalAuthTypeLocal:
		if len(cfg) > 0 {
			return apperror.ValidationFailed.New(
				"authConfig must be empty when authType is local.")
		}
		return nil
	case constants.APIPortalAuthTypeOAuth2:
		for _, key := range constants.APIPortalOAuth2RequiredAuthConfigKeys {
			v, ok := cfg[key]
			if !ok {
				return apperror.ValidationFailed.New(
					fmt.Sprintf("authConfig field %q is required for authType %q.", key, authType))
			}
			s, isString := v.(string)
			if !isString || strings.TrimSpace(s) == "" {
				return apperror.ValidationFailed.New(
					fmt.Sprintf("authConfig field %q must be a non-empty string.", key))
			}
		}
		allowed := map[string]bool{
			constants.APIPortalAuthConfigKeySTSTokenURL:  true,
			constants.APIPortalAuthConfigKeyClientID:     true,
			constants.APIPortalAuthConfigKeyClientSecret: true,
		}
		for k := range cfg {
			if !allowed[k] {
				return apperror.ValidationFailed.New(
					fmt.Sprintf("authConfig field %q is not supported for authType %q.", k, authType))
			}
		}
		// stsTokenUrl is the target of the outbound client_credentials
		// request that carries clientSecret, so it gets a stricter shape
		// check than a generic string. Ciphertext (already-encrypted, from
		// a merge path) has never occupied this key — clientSecret is the
		// only encrypted field — so the value here is a plaintext URL and
		// the parse-and-check is safe. See validateAPIPortalSTSTokenURL.
		if raw, ok := cfg[constants.APIPortalAuthConfigKeySTSTokenURL].(string); ok {
			if err := validateAPIPortalSTSTokenURL(raw); err != nil {
				return err
			}
		}
		return nil
	}
	return apperror.ValidationFailed.New(
		fmt.Sprintf("The authType %q is not supported.", authType))
}

// encryptAPIPortalAuthConfigSecrets walks the sensitive-key list and encrypts
// each key's value in place. Values are base64-encoded ciphertext strings
// after this returns. Empty / nil values are removed rather than encrypted so
// we never store an encrypted empty string.
func encryptAPIPortalAuthConfigSecrets(v vault.SecretVault, cfg map[string]interface{}) error {
	if cfg == nil {
		return nil
	}
	for _, key := range constants.APIPortalAuthConfigSensitiveKeys {
		raw, ok := cfg[key]
		if !ok {
			continue
		}
		if raw == nil {
			delete(cfg, key)
			continue
		}
		plaintext, isString := raw.(string)
		if !isString {
			return apperror.ValidationFailed.New(
				fmt.Sprintf("authConfig field %q must be a string.", key))
		}
		if plaintext == "" {
			delete(cfg, key)
			continue
		}
		ciphertext, err := v.Encrypt(context.Background(), plaintext)
		if err != nil {
			return fmt.Errorf("failed to encrypt authConfig field %q: %w", key, err)
		}
		cfg[key] = base64.StdEncoding.EncodeToString(ciphertext)
	}
	return nil
}

// mergeAPIPortalAuthConfig returns existing + incoming, with incoming keys
// overwriting existing ones. Used on Update so a caller can rotate a single
// field (e.g. only stsTokenUrl) without having to re-send fields they don't
// want to change — including clientSecret, which they can't fetch back.
func mergeAPIPortalAuthConfig(existing, incoming map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{}, len(existing)+len(incoming))
	for k, v := range existing {
		merged[k] = v
	}
	for k, v := range incoming {
		merged[k] = v
	}
	return merged
}

// copyStringMap returns a shallow copy so the service can encrypt/mutate its
// own working set without touching the caller's map (which lives in the
// generated request DTO the handler translated).
func copyStringMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// CreateAPIPortalRequest is the service-layer input for creating an API Portal.
// Fields mirror the OpenAPI CreateApiPortalRequest but stay independent of the
// generated types.
type CreateAPIPortalRequest struct {
	Handle      string
	Name        string
	Description string
	URL         string
	AuthType    string
	AuthConfig  map[string]interface{}
	Metadata    map[string]interface{}
}

// UpdateAPIPortalRequest carries mutable fields for a partial update. Pointer
// fields distinguish "not sent" (nil) from "sent as empty" (non-nil, empty).
// Only whitelisted fields are respected here; Handle, ID, OrganizationID,
// CreatedAt, CreatedBy are ignored per the design's immutability rules.
//
// AuthConfig on update uses merge semantics: supplied keys overwrite existing
// keys, missing keys retain their stored values. This lets a caller rotate a
// single field without re-supplying clientSecret (which they can't fetch back
// after it's been stored encrypted).
//
// Metadata on update uses replace semantics: if supplied (non-nil), it fully
// replaces the stored metadata. Callers that want a partial-update on metadata
// should GET, modify, PUT the whole thing.
type UpdateAPIPortalRequest struct {
	Name        *string
	Description *string
	URL         *string
	AuthType    *string
	AuthConfig  map[string]interface{} // when nil, existing preserved; when non-nil, merged in
	Metadata    map[string]interface{} // when nil, existing preserved; when non-nil, replaces
}

// APIPortalListOptions bundles the pagination inputs for List.
type APIPortalListOptions struct {
	repository.ListOptions
}

// APIPortalListResponse is the service-layer list result. The handler wraps
// this in the OpenAPI-generated envelope.
type APIPortalListResponse struct {
	Count      int
	List       []*model.APIPortal
	Pagination PaginationInfo
}

// PaginationInfo is the {total, offset, limit} triplet returned in list responses.
type PaginationInfo struct {
	Total  int
	Offset int
	Limit  int
}

// CreateAPIPortal validates the request, enforces uniqueness of the handle,
// and inserts a new row scoped to orgID.
func (s *APIPortalService) CreateAPIPortal(req *CreateAPIPortalRequest, orgID, createdBy string) (*model.APIPortal, error) {
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
	authType := strings.TrimSpace(req.AuthType)
	if !constants.ValidAPIPortalAuthTypes[authType] {
		return nil, apperror.ValidationFailed.New(
			fmt.Sprintf("The authType %q is not supported.", authType))
	}
	portalURL, err := validateAPIPortalURL(req.URL)
	if err != nil {
		return nil, err
	}
	if portalURL == "" {
		return nil, apperror.ValidationFailed.New("The url field is required.")
	}
	// Copy the incoming authConfig so we don't mutate the caller's map when we
	// encrypt secret fields in place.
	authConfig := copyStringMap(req.AuthConfig)
	if err := validateAPIPortalAuthConfig(authType, authConfig); err != nil {
		return nil, err
	}
	if err := encryptAPIPortalAuthConfigSecrets(s.vault, authConfig); err != nil {
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
		ID:             uuid.New().String(),
		OrganizationID: orgID,
		Handle:         strings.TrimSpace(req.Handle),
		Name:           name,
		Description:    strings.TrimSpace(req.Description),
		URL:            portalURL,
		Status:         constants.APIPortalStatusActive,
		AuthType:       authType,
		AuthConfig:     authConfig,
		Metadata:       req.Metadata,
		CreatedBy:      actor,
		UpdatedBy:      actor,
	}

	if err := s.portalRepo.Create(portal); err != nil {
		if repository.IsUniqueViolation(err) {
			// A concurrent create won the race between Exists and INSERT.
			return nil, apperror.APIPortalExists.New()
		}
		return nil, err
	}
	_ = s.auditRepo.Record("CREATE", portal.ID, "api_portal", orgID, actor)
	return portal, nil
}

// GetAPIPortal returns a single API Portal identified by its handle (wire ID) within orgID.
func (s *APIPortalService) GetAPIPortal(handle, orgID string) (*model.APIPortal, error) {
	portal, err := s.portalRepo.GetByHandleAndOrgID(strings.TrimSpace(handle), orgID)
	if err != nil {
		return nil, err
	}
	if portal == nil {
		return nil, apperror.APIPortalNotFound.New()
	}
	return portal, nil
}

// ListAPIPortals returns a page of API Portals in the organization, honoring
// the requested pagination + filter options. Limit/Offset are normalized here.
func (s *APIPortalService) ListAPIPortals(orgID string, opts APIPortalListOptions) (*APIPortalListResponse, error) {
	org, err := s.orgRepo.GetOrganizationByUUID(orgID)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, apperror.OrganizationNotFound.New()
	}
	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	if opts.Limit > 100 {
		opts.Limit = 100
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	total, err := s.portalRepo.Count(orgID, opts.Search)
	if err != nil {
		return nil, err
	}
	page, err := s.portalRepo.ListPaginated(orgID, opts.ListOptions)
	if err != nil {
		return nil, err
	}
	return &APIPortalListResponse{
		Count:      len(page),
		List:       page,
		Pagination: PaginationInfo{Total: total, Offset: opts.Offset, Limit: opts.Limit},
	}, nil
}

// UpdateAPIPortal loads the row, applies only the whitelisted mutations from req,
// persists the change, and returns the updated row.
func (s *APIPortalService) UpdateAPIPortal(handle string, req *UpdateAPIPortalRequest, orgID, updatedBy string) (*model.APIPortal, error) {
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
	if req.URL != nil {
		portalURL, err := validateAPIPortalURL(*req.URL)
		if err != nil {
			return nil, err
		}
		if portalURL == "" {
			return nil, apperror.ValidationFailed.New("The url field cannot be empty.")
		}
		portal.URL = portalURL
	}
	if req.AuthType != nil {
		at := strings.TrimSpace(*req.AuthType)
		if !constants.ValidAPIPortalAuthTypes[at] {
			return nil, apperror.ValidationFailed.New(
				fmt.Sprintf("The authType %q is not supported.", at))
		}
		portal.AuthType = at
	}
	if req.AuthConfig != nil {
		// Merge into the stored authConfig — supplied keys overwrite existing,
		// missing keys are retained. Encrypt any newly supplied sensitive
		// fields before persistence; existing encrypted values pass through
		// untouched because their key isn't in the incoming map.
		incoming := copyStringMap(req.AuthConfig)
		if err := encryptAPIPortalAuthConfigSecrets(s.vault, incoming); err != nil {
			return nil, err
		}
		portal.AuthConfig = mergeAPIPortalAuthConfig(portal.AuthConfig, incoming)
	}
	if req.Metadata != nil {
		// Metadata is opaque pass-through; supplied map fully replaces stored.
		portal.Metadata = copyStringMap(req.Metadata)
	}
	// authType owns the shape of authConfig. When the effective type is `local`,
	// authConfig keys carried over from a previous `oauth2` configuration are
	// dropped rather than left to fail a validation the caller cannot satisfy
	// (they can't send authConfig=null on the wire to clear it while nil-vs-
	// absent are the same shape in JSON).
	if portal.AuthType == constants.APIPortalAuthTypeLocal {
		portal.AuthConfig = nil
	}
	// Re-validate authConfig against the effective authType after all mutations.
	if err := validateAPIPortalAuthConfig(portal.AuthType, portal.AuthConfig); err != nil {
		return nil, err
	}
	portal.UpdatedBy = strings.TrimSpace(updatedBy)

	if err := s.portalRepo.Update(portal); err != nil {
		return nil, err
	}
	_ = s.auditRepo.Record("UPDATE", portal.ID, "api_portal", orgID, portal.UpdatedBy)
	// Config may have changed; drop any cached AuthProvider so the next
	// outbound call rebuilds from the new stored values.
	s.invalidateCachedAuthProvider(portal.Handle)
	return portal, nil
}

// DeleteAPIPortal removes the API Portal identified by its handle, org-scoped.
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
	s.invalidateCachedAuthProvider(portal.Handle)
	return nil
}
