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
	"database/sql"
	"errors"
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
	portalRepo     repository.APIPortalRepository
	orgRepo        repository.OrganizationRepository
	auditRepo      repository.AuditRepository
	vault          vault.SecretVault
	authRegistry   *APIPortalAuthRegistry
	webhookSecrets *APIPortalWebhookSecretService
	identity       *IdentityService
	slogger        *slog.Logger
}

// NewAPIPortalService constructs an APIPortalService. webhookSecrets may be
// nil in tests that do not exercise EnsureWebhookSecret / RotateWebhookSecret;
// calling those methods with no backing service returns a configuration error.
func NewAPIPortalService(
	portalRepo repository.APIPortalRepository,
	orgRepo repository.OrganizationRepository,
	auditRepo repository.AuditRepository,
	secretVault vault.SecretVault,
	authRegistry *APIPortalAuthRegistry,
	webhookSecrets *APIPortalWebhookSecretService,
	identity *IdentityService,
	slogger *slog.Logger,
) *APIPortalService {
	return &APIPortalService{
		portalRepo:     portalRepo,
		orgRepo:        orgRepo,
		auditRepo:      auditRepo,
		vault:          secretVault,
		authRegistry:   authRegistry,
		webhookSecrets: webhookSecrets,
		identity:       identity,
		slogger:        slogger,
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

// EnsureWebhookSecret delegates to the per-portal webhook secret service: it
// returns the portal's current signing secret, generating one on first call.
// Shape-satisfies pdk.APIPortals.EnsureWebhookSecret so the plugin's Seed can
// obtain the secret for the subscriber row in one PDK call.
func (s *APIPortalService) EnsureWebhookSecret(ctx context.Context, handle, orgID string) (string, string, error) {
	if s.webhookSecrets == nil {
		return "", "", fmt.Errorf("per-portal webhook secret service is not initialised")
	}
	return s.webhookSecrets.EnsureWebhookSecret(ctx, handle, orgID)
}

// RotateWebhookSecret delegates to the per-portal webhook secret service.
// Shape-satisfies pdk.APIPortals.RotateWebhookSecret so the plugin's rotation
// change can obtain a fresh secret before PUT-ing the subscriber row.
func (s *APIPortalService) RotateWebhookSecret(ctx context.Context, handle, orgID string) (string, string, error) {
	if s.webhookSecrets == nil {
		return "", "", fmt.Errorf("per-portal webhook secret service is not initialised")
	}
	return s.webhookSecrets.RotateWebhookSecret(ctx, handle, orgID)
}

// withWebhookSecretFlag populates the webhookSecretConfigured bool on a built
// ApiPortalResponse by asking the per-portal webhook secret service whether a
// secret row exists. Silent fallback to false when the service is nil (tests)
// or the lookup fails; the field itself is informational and must not break a
// successful portal read.
func (s *APIPortalService) withWebhookSecretFlag(resp *api.ApiPortalResponse, portalUUID string) *api.ApiPortalResponse {
	if resp == nil {
		return nil
	}
	configured := false
	if s.webhookSecrets != nil {
		has, err := s.webhookSecrets.HasSecret(portalUUID)
		if err != nil {
			s.slogger.Warn("Failed to probe webhook secret configuration",
				"portalUUID", portalUUID, "error", err)
		} else {
			configured = has
		}
	}
	resp.WebhookSecretConfigured = &configured
	return resp
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
// The row is created with status "active" - the OSS-native lifecycle has no
// intermediate provisioning state. Cloud-plugin callers that need to insert
// with a different initial status should use CreateAPIPortalWithStatus.
func (s *APIPortalService) CreateAPIPortal(req *api.CreateApiPortalRequest, orgID, createdBy string) (*api.ApiPortalResponse, error) {
	return s.CreateAPIPortalWithStatus(req, orgID, createdBy, constants.APIPortalStatusActive)
}

// CreateAPIPortalWithStatus is the create path for callers that own the
// portal's provisioning lifecycle (currently only the cloud plugin's managed
// portals feature). Behaves identically to CreateAPIPortal but writes the
// provided status instead of hardcoding active. Not exposed via REST; only
// Go-embedding callers can reach it. The status value is validated against
// the platform-api constants set.
func (s *APIPortalService) CreateAPIPortalWithStatus(req *api.CreateApiPortalRequest, orgID, createdBy, status string) (*api.ApiPortalResponse, error) {
	if req == nil {
		return nil, apperror.ValidationFailed.New("The request body is required.")
	}
	if err := validateAPIPortalStatus(status); err != nil {
		return nil, err
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
		Status:          status,
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
	return s.withWebhookSecretFlag(ModelToAPIPortalResponse(portal), portal.ID), nil
}

// validateAPIPortalStatus rejects any value not in the platform-api-defined
// constants set so a caller cannot inject unknown state.
func validateAPIPortalStatus(status string) error {
	switch status {
	case constants.APIPortalStatusPending,
		constants.APIPortalStatusActive,
		constants.APIPortalStatusFailed:
		return nil
	default:
		return apperror.ValidationFailed.New(
			fmt.Sprintf("Invalid API Portal status %q; expected one of %q, %q, %q.",
				status,
				constants.APIPortalStatusPending,
				constants.APIPortalStatusActive,
				constants.APIPortalStatusFailed))
	}
}

// UpdateAPIPortalStatus flips only the provisioning status column, scoped by
// handle within orgID. Used by the cloud plugin's provisioning poller to
// transition pending -> active on success or pending -> failed on timeout.
// Not exposed via REST.
//
// Target status is restricted to the terminal values (active, failed) so the
// docstring's pending -> terminal contract is enforced. Writing pending here
// would rewrite an already-pending row in place, keep it eligible for further
// polling, and record a misleading state-transition audit event. Initial
// pending assignment lives on the Create path (CreateAPIPortalWithStatus).
func (s *APIPortalService) UpdateAPIPortalStatus(handle, orgID, updatedBy, status string) error {
	if err := validateAPIPortalStatus(status); err != nil {
		return err
	}
	if status == constants.APIPortalStatusPending {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("API Portal status transitions cannot target %q; only %q or %q are permitted here.",
				constants.APIPortalStatusPending,
				constants.APIPortalStatusActive,
				constants.APIPortalStatusFailed))
	}
	portal, err := s.portalRepo.GetByHandleAndOrgID(strings.TrimSpace(handle), orgID)
	if err != nil {
		return err
	}
	if portal == nil {
		return apperror.APIPortalNotFound.New()
	}
	// repository.ErrAPIPortalNotPending is a normal outcome for concurrent pollers
	// racing the state machine (see repository docs); it flows through unchanged
	// so plugin callers can detect it via errors.Is and drop their write silently.
	// A transition no user made keeps the row's updated_by; the audit names
	// the user the row kept.
	actor, err := s.portalRepo.UpdateStatus(portal.ID, orgID, strings.TrimSpace(updatedBy), status)
	if err != nil {
		return err
	}
	_ = s.auditRepo.Record("UPDATE_STATUS", portal.ID, "api_portal", orgID, actor)
	return nil
}

// ReopenAPIPortal moves an active or failed portal back to pending, for a
// change that takes it out of service until the change completes; the change's
// owner later moves it on with UpdateAPIPortalStatus. A portal that is already
// pending returns ErrAPIPortalPending (unwrapped, for errors.Is).
func (s *APIPortalService) ReopenAPIPortal(handle, orgID, updatedBy string) error {
	portal, err := s.portalRepo.GetByHandleAndOrgID(strings.TrimSpace(handle), orgID)
	if err != nil {
		return err
	}
	if portal == nil {
		return apperror.APIPortalNotFound.New()
	}
	actor, err := s.portalRepo.Reopen(portal.ID, orgID, strings.TrimSpace(updatedBy))
	if err != nil {
		return err
	}
	_ = s.auditRepo.Record("UPDATE_STATUS", portal.ID, "api_portal", orgID, actor)
	return nil
}

// GetAPIPortalStatus returns just the provisioning status column for a portal.
// Cheaper than GetAPIPortal when the caller (typically the cloud plugin's Get
// projection) only needs the status. Missing portal surfaces as an
// APIPortalNotFound error; empty status is impossible because the column has
// a schema default and a NOT NULL constraint.
func (s *APIPortalService) GetAPIPortalStatus(handle, orgID string) (string, error) {
	status, err := s.portalRepo.GetStatusByHandle(strings.TrimSpace(handle), orgID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", apperror.APIPortalNotFound.New()
		}
		return "", err
	}
	return status, nil
}

// ListAPIPortalStatuses returns handle -> status for every portal in the org.
// Used by the cloud plugin's List projection to stamp status per row in one
// DB round-trip. Empty result when the org has no portals.
func (s *APIPortalService) ListAPIPortalStatuses(orgID string) (map[string]string, error) {
	return s.portalRepo.ListStatusesByOrg(orgID)
}

// ListAPIPortalLoginEnvironments returns handle -> loginEnvironment for
// portals in the org whose metadata blob carries the key. Plugin-facing
// companion to ListAPIPortalStatuses that keeps the cloud-plugin-specific
// loginEnvironment field off ApiPortalListItem (the REST list projection
// stays lightweight and OSS-neutral). Portals whose metadata does not carry
// the key are omitted from the map; empty org returns an empty map.
func (s *APIPortalService) ListAPIPortalLoginEnvironments(orgID string) (map[string]string, error) {
	return s.portalRepo.ListLoginEnvironmentsByOrg(orgID)
}

// ListAPIPortalsByStatus returns the identity (org, handle, url, status) of
// every portal across every org whose status matches. Cross-org by design:
// the cloud plugin's provisioning poller has no org list at startup and needs
// to re-track every pending portal to survive a crash mid-provisioning.
func (s *APIPortalService) ListAPIPortalsByStatus(status string) ([]api.APIPortalIdentity, error) {
	if err := validateAPIPortalStatus(status); err != nil {
		return nil, err
	}
	portals, err := s.portalRepo.ListByStatus(status)
	if err != nil {
		return nil, err
	}
	out := make([]api.APIPortalIdentity, 0, len(portals))
	for _, p := range portals {
		if p == nil {
			continue
		}
		out = append(out, api.APIPortalIdentity{
			OrgID:  p.OrganizationID,
			Handle: p.Handle,
			URL:    p.URL,
			Status: p.Status,
		})
	}
	return out, nil
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
	return s.withWebhookSecretFlag(ModelToAPIPortalResponse(portal), portal.ID), nil
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
	// A pending portal's URL is owned by its provisioner until it is active.
	if req.Url != nil && portal.IsPending() {
		requested, err := validateAPIPortalURL(*req.Url)
		if err != nil {
			return nil, err
		}
		if requested != portal.URL {
			return nil, apperror.ValidationFailed.New("The url field cannot be changed while the API Portal is pending.")
		}
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
	return s.withWebhookSecretFlag(ModelToAPIPortalResponse(portal), portal.ID), nil
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
