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
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// Audit values for service-account lifecycle events.
const (
	auditResourceServiceAccount = "service_account"
	auditActionCreate           = "CREATE"
	auditActionUpdate           = "UPDATE"
	auditActionDelete           = "DELETE"
	auditActionDisable          = "DISABLE"
	auditActionEnable           = "ENABLE"
	auditActionRegenerateSecret = "REGENERATE_SECRET"
)

// dummySecretHash is compared on an unknown client ID, so a miss costs the
// same as a wrong secret.
var dummySecretHash = hashServiceAccountSecret(model.ServiceAccountSecretPrefix + strings.Repeat("0", 64))

// ServiceAccountService manages service accounts and exchanges their
// credentials for tokens.
type ServiceAccountService struct {
	repo         repository.ServiceAccountRepository
	orgRepo      repository.OrganizationRepository
	auditRepo    repository.AuditRepository
	identity     *IdentityService
	roleScopeMap map[string][]string
	signer       *SATokenSigner // nil when no signing key is configured
	tokenTTL     time.Duration
	revocations  *RevocationCache // optional; applies local revokes at once
	slogger      *slog.Logger
}

// SetRevocationCache lets this replica apply its own revokes without waiting
// for the next poll.
func (s *ServiceAccountService) SetRevocationCache(c *RevocationCache) { s.revocations = c }

// NewServiceAccountService creates a ServiceAccountService. signer may be nil,
// in which case every exchange fails with the uniform 401.
func NewServiceAccountService(repo repository.ServiceAccountRepository, orgRepo repository.OrganizationRepository,
	auditRepo repository.AuditRepository, identity *IdentityService, roleScopeMap map[string][]string,
	signer *SATokenSigner, tokenTTL time.Duration, slogger *slog.Logger) *ServiceAccountService {
	return &ServiceAccountService{
		repo: repo, orgRepo: orgRepo, auditRepo: auditRepo, identity: identity, roleScopeMap: roleScopeMap,
		signer: signer, tokenTTL: tokenTTL, slogger: slogger,
	}
}

// Create makes an account and returns its only copy of the secret.
func (s *ServiceAccountService) Create(orgID, actor string, req *api.ServiceAccountCreateRequest) (*api.ServiceAccountCredentials, error) {
	if err := utils.ValidateHandle(req.Id); err != nil {
		return nil, err
	}
	if req.Id == constants.ServiceAccountReservedHandle {
		return nil, apperror.ValidationFailed.New(fmt.Sprintf("The id %q is reserved.", req.Id))
	}
	displayName, owner, description := strings.TrimSpace(req.DisplayName), strings.TrimSpace(req.Owner), strings.TrimSpace(req.Description)
	if err := requireText(map[string]string{"displayName": displayName, "owner": owner, "description": description}); err != nil {
		return nil, err
	}
	roles, err := s.validateRoles(req.Roles)
	if err != nil {
		return nil, err
	}
	org, err := s.orgRepo.GetOrganizationByUUID(orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to load organization %s: %w", orgID, err)
	}
	if org == nil {
		return nil, fmt.Errorf("organization %s not found", orgID)
	}

	accountUUID, err := utils.GenerateUUID()
	if err != nil {
		return nil, err
	}
	secret, err := newServiceAccountSecret()
	if err != nil {
		return nil, err
	}
	suffix, err := utils.GenerateAPIKey()
	if err != nil {
		return nil, err
	}
	sa := &model.ServiceAccount{
		UUID:             accountUUID,
		OrganizationID:   orgID,
		Handle:           req.Id,
		DisplayName:      displayName,
		Owner:            owner,
		Description:      description,
		ClientID:         "sa_" + org.Handle + "_" + req.Id + "_" + suffix[:6],
		ClientSecretHash: hashServiceAccountSecret(secret),
		MaskedSecret:     maskServiceAccountSecret(secret),
		// The identity row shares the account's UUID; the startup reservation
		// check relies on that.
		IdentityUUID: accountUUID,
		Roles:        strings.Join(roles, " "),
		Status:       model.ServiceAccountStatusActive,
		TokenVersion: 1,
		CreatedBy:    actor,
		UpdatedBy:    actor,
	}
	if err := s.repo.Create(sa, sa.Subject(org.Handle)); err != nil {
		return nil, err
	}
	s.audit(auditActionCreate, sa, actor)
	s.slogger.Info("service account created", "accountUuid", sa.UUID, "orgUuid", orgID, "roles", sa.Roles)

	resp, err := s.toAPI(sa)
	if err != nil {
		return nil, err
	}
	return &api.ServiceAccountCredentials{ServiceAccount: *resp, ClientId: sa.ClientID, ClientSecret: secret}, nil
}

func (s *ServiceAccountService) List(orgID string, limit, offset int) (*api.ServiceAccountListResponse, error) {
	accounts, err := s.repo.List(orgID, limit, offset)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.Count(orgID)
	if err != nil {
		return nil, err
	}
	list := make([]api.ServiceAccount, 0, len(accounts))
	for _, sa := range accounts {
		list = append(list, toServiceAccountAPI(sa))
	}
	fields := make([]**string, 0, 2*len(list))
	for i := range list {
		fields = append(fields, &list[i].CreatedBy, &list[i].UpdatedBy)
	}
	if err := s.identity.ResolveIdentityFields(fields); err != nil {
		return nil, err
	}
	return &api.ServiceAccountListResponse{
		Count:      len(list),
		List:       list,
		Pagination: api.Pagination{Total: total, Offset: offset, Limit: limit},
	}, nil
}

func (s *ServiceAccountService) Get(orgID, handle string) (*api.ServiceAccount, error) {
	sa, err := s.repo.GetByHandle(orgID, handle)
	if err != nil {
		return nil, err
	}
	return s.toAPI(sa)
}

// Update changes metadata, roles or status. Disabling or removing a role bumps
// the token version and writes a watermark in the same transaction, so tokens
// already issued stop working. Adding a role revokes nothing: old tokens just
// lack the new scopes until they are re-exchanged.
func (s *ServiceAccountService) Update(orgID, handle, actor string, req *api.ServiceAccountUpdateRequest) (*api.ServiceAccount, error) {
	sa, err := s.repo.GetByHandle(orgID, handle)
	if err != nil {
		return nil, err
	}
	before := *sa

	if req.DisplayName != nil {
		sa.DisplayName = strings.TrimSpace(*req.DisplayName)
	}
	if req.Owner != nil {
		sa.Owner = strings.TrimSpace(*req.Owner)
	}
	if req.Description != nil {
		sa.Description = strings.TrimSpace(*req.Description)
	}
	if err := requireText(map[string]string{"displayName": sa.DisplayName, "owner": sa.Owner, "description": sa.Description}); err != nil {
		return nil, err
	}
	if req.Roles != nil {
		roles, err := s.validateRoles(*req.Roles)
		if err != nil {
			return nil, err
		}
		sa.Roles = strings.Join(roles, " ")
	}
	if req.Status != nil {
		switch st := string(*req.Status); st {
		case model.ServiceAccountStatusActive, model.ServiceAccountStatusDisabled:
			sa.Status = st
		default:
			return nil, apperror.ValidationFailed.New("status must be active or disabled")
		}
	}
	sa.UpdatedBy = actor

	var rev *model.ServiceAccountRevocation
	disabling := before.Status != model.ServiceAccountStatusDisabled && sa.Status == model.ServiceAccountStatusDisabled
	if disabling || rolesRemoved(before.RoleList(), sa.RoleList()) {
		sa.TokenVersion = before.TokenVersion + 1
		rev = s.revocation(sa, sa.TokenVersion, actor)
	}
	if err := s.repo.Update(sa, before.TokenVersion, before.Status, rev); err != nil {
		return nil, err
	}
	s.remember(rev)

	if sa.DisplayName != before.DisplayName || sa.Owner != before.Owner ||
		sa.Description != before.Description || sa.Roles != before.Roles {
		s.audit(auditActionUpdate, sa, actor)
		// The audit table has no detail column, so the role change is logged here.
		s.slogger.Info("service account updated", "accountUuid", sa.UUID, "orgUuid", orgID,
			"rolesBefore", before.Roles, "rolesAfter", sa.Roles)
	}
	if sa.Status != before.Status {
		action := auditActionEnable
		if disabling {
			action = auditActionDisable
		}
		s.audit(action, sa, actor)
		s.slogger.Info("service account status changed", "accountUuid", sa.UUID, "orgUuid", orgID, "status", sa.Status)
	}
	return s.toAPI(sa)
}

// Delete removes the account. Its watermark survives the row.
func (s *ServiceAccountService) Delete(orgID, handle, actor string) error {
	sa, err := s.repo.GetByHandle(orgID, handle)
	if err != nil {
		return err
	}
	rev := s.revocation(sa, sa.TokenVersion+1, actor)
	if err := s.repo.Delete(orgID, sa.UUID, sa.TokenVersion, rev); err != nil {
		return err
	}
	s.remember(rev)
	s.audit(auditActionDelete, sa, actor)
	s.slogger.Info("service account deleted", "accountUuid", sa.UUID, "orgUuid", orgID)
	return nil
}

// RegenerateSecret replaces the secret with no overlap and revokes live tokens.
func (s *ServiceAccountService) RegenerateSecret(orgID, handle, actor string) (*api.ServiceAccountCredentials, error) {
	sa, err := s.repo.GetByHandle(orgID, handle)
	if err != nil {
		return nil, err
	}
	secret, err := newServiceAccountSecret()
	if err != nil {
		return nil, err
	}
	sa.ClientSecretHash = hashServiceAccountSecret(secret)
	sa.MaskedSecret = maskServiceAccountSecret(secret)
	sa.SecretRegeneratedBy = actor
	sa.UpdatedBy = actor
	prevVersion := sa.TokenVersion
	sa.TokenVersion++
	rev := s.revocation(sa, sa.TokenVersion, actor)
	if err := s.repo.UpdateSecret(sa, prevVersion, rev); err != nil {
		return nil, err
	}
	s.remember(rev)
	s.audit(auditActionRegenerateSecret, sa, actor)
	s.slogger.Info("service account secret regenerated", "accountUuid", sa.UUID, "orgUuid", orgID)

	resp, err := s.toAPI(sa)
	if err != nil {
		return nil, err
	}
	return &api.ServiceAccountCredentials{ServiceAccount: *resp, ClientId: sa.ClientID, ClientSecret: secret}, nil
}

// ExchangeRequest is what the token endpoint passes in. ClientIP and UserAgent
// are for logging and last-used tracking only.
type ExchangeRequest struct {
	ClientID     string
	ClientSecret string
	ClientIP     string
	UserAgent    string
}

// Exchange swaps client credentials for a token. Every failure is the same
// 401; the cause is logged, never returned.
func (s *ServiceAccountService) Exchange(req ExchangeRequest) (*api.ServiceAccountTokenResponse, error) {
	fail := func(cause string, sa *model.ServiceAccount) error {
		// An unmatched client ID is not logged: callers paste secrets into it.
		attrs := []any{"clientIp", req.ClientIP, "userAgent", req.UserAgent, "cause", cause}
		if sa != nil {
			attrs = append(attrs, "accountUuid", sa.UUID, "orgUuid", sa.OrganizationID)
		}
		s.slogger.Warn("service account token exchange failed", attrs...)
		return apperror.Unauthorized.New()
	}

	sa, err := s.repo.GetByClientID(req.ClientID)
	if err != nil {
		subtle.ConstantTimeCompare([]byte(dummySecretHash), []byte(hashServiceAccountSecret(req.ClientSecret)))
		if apperror.ServiceAccountNotFound.Is(err) {
			return nil, fail("unknown client", nil)
		}
		return nil, fail("lookup error: "+err.Error(), nil)
	}
	if subtle.ConstantTimeCompare([]byte(sa.ClientSecretHash), []byte(hashServiceAccountSecret(req.ClientSecret))) != 1 {
		return nil, fail("secret mismatch", sa)
	}
	if sa.Status != model.ServiceAccountStatusActive {
		return nil, fail("account disabled", sa)
	}
	if s.signer == nil {
		return nil, fail("no signing key configured", sa)
	}
	org, err := s.orgRepo.GetOrganizationByUUID(sa.OrganizationID)
	if err != nil || org == nil {
		return nil, fail("organization not found", sa)
	}

	// Roles are re-read here, so a mapping change takes effect within one TTL.
	// The token carries sa.TokenVersion, read in the same row as the secret
	// hash and status just checked, so it can never outrank the revoke that
	// replaced them.
	scope := expandRoles(sa.RoleList(), s.roleScopeMap)
	tok, err := s.signer.Sign(sa, org, scope)
	if err != nil {
		return nil, apperror.Internal.Wrap(err).WithLogMessage("failed to sign service-account token")
	}

	now := time.Now()
	if err := s.repo.TouchLastUsed(sa.UUID, now, req.ClientIP); err != nil {
		s.slogger.Warn("failed to record service account use", "accountUuid", sa.UUID, "error", err)
	}
	s.slogger.Info("service account token issued",
		"accountUuid", sa.UUID, "orgUuid", sa.OrganizationID, "clientIp", req.ClientIP,
		"userAgent", req.UserAgent, "scope", scope, "jti", tok.JTI, "exp", tok.ExpiresAt.Unix())

	return &api.ServiceAccountTokenResponse{
		AccessToken: tok.Token,
		TokenType:   "Bearer",
		ExpiresIn:   int(s.signer.TTL().Seconds()),
		Scope:       &scope,
	}, nil
}

// revocation builds the watermark rejecting sa's tokens below minVersion.
// expires_at is two TTLs out: one covers every token minted before the revoke,
// the other absorbs a minting replica whose clock ran ahead (a token's exp is
// set by that replica's clock).
func (s *ServiceAccountService) revocation(sa *model.ServiceAccount, minVersion int64, actor string) *model.ServiceAccountRevocation {
	now := time.Now().UTC()
	return &model.ServiceAccountRevocation{
		AccountUUID:     sa.UUID,
		OrganizationID:  sa.OrganizationID,
		MinTokenVersion: minVersion,
		ExpiresAt:       now.Add(2 * s.tokenTTL),
		RevokedBy:       actor,
	}
}

func (s *ServiceAccountService) remember(rev *model.ServiceAccountRevocation) {
	if rev != nil && s.revocations != nil {
		s.revocations.Remember(rev)
	}
}

// audit records a lifecycle event. A failure is logged, never swallowed: a
// missing row for a privilege change must be visible.
func (s *ServiceAccountService) audit(action string, sa *model.ServiceAccount, actor string) {
	if err := s.auditRepo.Record(action, sa.UUID, auditResourceServiceAccount, sa.OrganizationID, actor); err != nil {
		s.slogger.Error("failed to write service account audit row",
			"action", action, "accountUuid", sa.UUID, "orgUuid", sa.OrganizationID, "error", err)
	}
}

// validateRoles rejects a role missing from the mapping file: the account
// would authenticate and then 403 on everything.
func (s *ServiceAccountService) validateRoles(roles []string) ([]string, error) {
	if len(roles) == 0 {
		return nil, apperror.ValidationFailed.New("roles must name at least one role")
	}
	out := make([]string, 0, len(roles))
	for _, role := range roles {
		role = strings.TrimSpace(role)
		if _, ok := s.roleScopeMap[role]; !ok {
			return nil, apperror.ValidationFailed.New(fmt.Sprintf("role %q is not defined in the role-to-scope mapping", role))
		}
		if !slices.Contains(out, role) {
			out = append(out, role)
		}
	}
	return out, nil
}

func (s *ServiceAccountService) toAPI(sa *model.ServiceAccount) (*api.ServiceAccount, error) {
	resp := toServiceAccountAPI(sa)
	if err := s.identity.ResolveIdentityFields([]**string{&resp.CreatedBy, &resp.UpdatedBy}); err != nil {
		return nil, err
	}
	return &resp, nil
}

func toServiceAccountAPI(sa *model.ServiceAccount) api.ServiceAccount {
	clientID, masked := sa.ClientID, sa.MaskedSecret
	createdAt, updatedAt := sa.CreatedAt, sa.UpdatedAt
	createdBy, updatedBy := sa.CreatedBy, sa.UpdatedBy
	resp := api.ServiceAccount{
		Id:                  sa.Handle,
		DisplayName:         sa.DisplayName,
		Owner:               sa.Owner,
		Description:         sa.Description,
		ClientId:            &clientID,
		MaskedSecret:        &masked,
		Roles:               sa.RoleList(),
		Status:              api.ServiceAccountStatus(sa.Status),
		LastUsedAt:          sa.LastUsedAt,
		SecretRegeneratedAt: sa.SecretRegeneratedAt,
		CreatedAt:           &createdAt,
		UpdatedAt:           &updatedAt,
		CreatedBy:           &createdBy,
		UpdatedBy:           &updatedBy,
	}
	if sa.LastUsedIP != "" {
		ip := sa.LastUsedIP
		resp.LastUsedIp = &ip
	}
	return resp
}

// rolesRemoved reports whether after lacks any role before had.
func rolesRemoved(before, after []string) bool {
	for _, role := range before {
		if !slices.Contains(after, role) {
			return true
		}
	}
	return false
}

// expandRoles is the union of the scopes the roles grant, deduplicated, in
// the same order the login endpoint produces.
func expandRoles(roles []string, roleScopeMap map[string][]string) string {
	seen := map[string]struct{}{}
	var scopes []string
	for _, role := range roles {
		for _, sc := range roleScopeMap[role] {
			if _, dup := seen[sc]; !dup {
				seen[sc] = struct{}{}
				scopes = append(scopes, sc)
			}
		}
	}
	return strings.Join(scopes, " ")
}

func requireText(fields map[string]string) error {
	for _, name := range []string{"displayName", "owner", "description"} {
		if v, ok := fields[name]; ok && v == "" {
			return apperror.ValidationFailed.New(name + " must not be blank")
		}
	}
	return nil
}

// newServiceAccountSecret is apsa_ plus 64 hex chars from crypto/rand.
func newServiceAccountSecret() (string, error) {
	raw, err := utils.GenerateAPIKey()
	if err != nil {
		return "", err
	}
	return model.ServiceAccountSecretPrefix + raw, nil
}

// hashServiceAccountSecret is SHA-256 of the whole secret, prefix included. No
// work factor: the secret is 256 random bits, not a password.
func hashServiceAccountSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func maskServiceAccountSecret(secret string) string {
	if len(secret) <= 5 {
		return "***"
	}
	return "***" + secret[len(secret)-5:]
}
