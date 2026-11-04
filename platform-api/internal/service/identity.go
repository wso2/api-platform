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
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// IdentityService resolves the internal platform UUID for the actor behind a
// request, translating the identity-provider's resolved actor identifier
// (the OIDC "sub" claim, falling back to the configured claim / user_id via
// middleware.GetActorIdentityFromRequest) into our own stable UUID. This UUID
// is what gets stored in audit columns
// (created_by/updated_by/revoked_by/performed_by).
//
// The internal UUID never leaves the process. API responses resolve it to the
// user's display name via ResolveIdentityField(s) / DisplayNamesForUUIDs, falling
// back to the raw identity when no display name is stored; external/data-plane
// consumers (e.g. gateway events) resolve it to the raw identity via
// SubForUUID / SubsForUUIDs. Both fall back to constants.DeletedUser when the
// UUID has no mapping.
type IdentityService struct {
	repo repository.UserIdentityMappingRepository
}

// NewIdentityService creates a new IdentityService.
func NewIdentityService(repo repository.UserIdentityMappingRepository) *IdentityService {
	return &IdentityService{repo: repo}
}

// ToInternalUUID maps a raw resolved actor identity (or header-supplied actor
// id) to our internal platform UUID, creating the mapping on first use. An
// empty identity returns an empty string and creates no row — see
// InternalUserID for the anonymous-write path, which never fails.
func (s *IdentityService) ToInternalUUID(identity string) (string, error) {
	return s.repo.GetOrCreateUUID(identity)
}

// InternalUserID resolves the internal platform UUID for the actor making the
// given request, preferring the token's "sub" claim and falling back through
// the configured claim / user_id. When the request carries no resolvable
// identity, a fresh standalone UUID is minted so the write never fails; no
// mapping row is created for it, so it reads back as constants.DeletedUser
// (see SubForUUID) — it can never be confused with a real user.
//
// The request's display-name claim, when present, is stored on (or refreshes)
// the mapping row for display. It is optional: a token without it maps exactly
// as before, and a failure to save it is logged, never returned.
func (s *IdentityService) InternalUserID(r *http.Request) (string, error) {
	identity, ok := middleware.GetActorIdentityFromRequest(r)
	if !ok || identity == "" {
		return utils.GenerateUUID()
	}
	displayName, _ := middleware.GetDisplayNameFromRequest(r)
	return s.ToInternalUUIDWithDisplayName(identity, displayName)
}

// ToInternalUUIDWithDisplayName is ToInternalUUID that also stores (or
// refreshes) the user's display name on the mapping row. The name is optional:
// an empty one maps exactly as ToInternalUUID does, and a failure to save it is
// logged and not returned. The driver error is left out of the log because it
// can carry the rejected value.
func (s *IdentityService) ToInternalUUIDWithDisplayName(identity, displayName string) (string, error) {
	mapped, err := s.repo.GetOrCreateUUIDWithDisplayName(identity, displayName)
	if errors.Is(err, repository.ErrDisplayNameNotSaved) {
		slog.Default().Warn("Failed to save the user's display name; continuing without it", "userUUID", mapped)
		return mapped, nil
	}
	return mapped, err
}

// DisplayIdentity satisfies pdk.Identity: the identity a response shows for an
// internal user id, resolved as ResolveIdentityField does. An empty id is
// returned as is, the same as a field with no recorded user.
func (s *IdentityService) DisplayIdentity(internalUserID string) (string, error) {
	if internalUserID == "" {
		return "", nil
	}
	return s.DisplayNameForUUID(internalUserID)
}

// SubForUUID resolves an internal UUID stored in an audit column back to the
// raw identity that machine consumers match on (gateway events), or
// constants.DeletedUser if uuid has no mapping (an anonymous write, or a user
// whose mapping was removed). API responses use DisplayNameForUUID instead.
func (s *IdentityService) SubForUUID(uuid string) (string, error) {
	if uuid == "" {
		return constants.DeletedUser, nil
	}
	identity, found, err := s.repo.GetSubByUUID(uuid)
	if err != nil {
		return "", err
	}
	if !found || identity == "" {
		return constants.DeletedUser, nil
	}
	return identity, nil
}

// SubsForUUIDs batch-resolves multiple UUIDs (e.g. across a list response) to
// their raw identity in a single lookup, avoiding N+1 queries. Every non-empty
// input UUID is present in the result, resolving to constants.DeletedUser if
// it has no mapping.
func (s *IdentityService) SubsForUUIDs(uuids []string) (map[string]string, error) {
	resolved, err := s.repo.GetSubsByUUIDs(uuids)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(uuids))
	for _, id := range uuids {
		if id == "" {
			continue
		}
		if identity, ok := resolved[id]; ok && identity != "" {
			result[id] = identity
		} else {
			result[id] = constants.DeletedUser
		}
	}
	return result, nil
}

// DisplayNamesForUUIDs batch-resolves internal UUIDs to what an API response
// shows for each: the stored display name, else the raw identity (so rows
// written before display names were captured, or by users whose token never
// carried the claim, render exactly as before), else constants.DeletedUser
// when the UUID has no mapping. Every non-empty input UUID is present in the
// result.
func (s *IdentityService) DisplayNamesForUUIDs(uuids []string) (map[string]string, error) {
	identities, err := s.repo.GetIdentitiesByUUIDs(uuids)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(uuids))
	for _, id := range uuids {
		if id == "" {
			continue
		}
		result[id] = displayNameOf(identities[id])
	}
	return result, nil
}

// DisplayNameForUUID is the single-UUID form of DisplayNamesForUUIDs; an empty
// UUID resolves to constants.DeletedUser, as SubForUUID does.
func (s *IdentityService) DisplayNameForUUID(uuid string) (string, error) {
	if uuid == "" {
		return constants.DeletedUser, nil
	}
	names, err := s.DisplayNamesForUUIDs([]string{uuid})
	if err != nil {
		return "", err
	}
	return names[uuid], nil
}

func displayNameOf(identity repository.UserIdentity) string {
	if name := strings.TrimSpace(identity.DisplayName); name != "" {
		return name
	}
	if identity.IdpID != "" {
		return identity.IdpID
	}
	return constants.DeletedUser
}

// ResolveIdentityField replaces the UUID held in *field (a generated API
// response's createdBy/updatedBy/revokedBy/performedBy pointer, which holds
// our internal UUID immediately after a model→API mapper runs) with the user's
// display name (see DisplayNamesForUUIDs), in place. No-op if field is nil or
// points to nil/empty.
//
// This is the single place a detail response's audit-identity field is
// unwrapped before being returned to the caller — see ResolveIdentityFields
// for the batch/list equivalent.
func (s *IdentityService) ResolveIdentityField(field **string) error {
	if field == nil || *field == nil || **field == "" {
		return nil
	}
	resolved, err := s.DisplayNameForUUID(**field)
	if err != nil {
		return err
	}
	*field = &resolved
	return nil
}

// ResolveIdentityFields batch-resolves multiple createdBy/updatedBy-style
// pointer fields (e.g. across every record in a list response) in a single
// lookup, avoiding N+1 queries. Each entry is a pointer to a *string field
// (e.g. &resp.CreatedBy); nil entries, and fields that are nil or empty, are
// skipped.
func (s *IdentityService) ResolveIdentityFields(fields []**string) error {
	uuids := make([]string, 0, len(fields))
	for _, f := range fields {
		if f != nil && *f != nil && **f != "" {
			uuids = append(uuids, **f)
		}
	}
	resolved, err := s.DisplayNamesForUUIDs(uuids)
	if err != nil {
		return err
	}
	for _, f := range fields {
		if f == nil || *f == nil || **f == "" {
			continue
		}
		if identity, ok := resolved[**f]; ok {
			v := identity
			*f = &v
		}
	}
	return nil
}
