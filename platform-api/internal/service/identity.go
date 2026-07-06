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
	"net/http"

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
// The internal UUID never leaves the process: API responses and
// external/data-plane consumers (e.g. gateway events) must resolve it back to
// the raw identity via SubForUUID / SubsForUUIDs before it is emitted —
// falling back to constants.DeletedUser when the UUID has no mapping.
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
func (s *IdentityService) InternalUserID(r *http.Request) (string, error) {
	identity, ok := middleware.GetActorIdentityFromRequest(r)
	if !ok || identity == "" {
		return utils.GenerateUUID()
	}
	return s.ToInternalUUID(identity)
}

// SubForUUID resolves an internal UUID stored in an audit column back to the
// raw identity that should be shown to external consumers (API responses,
// gateway events), or constants.DeletedUser if uuid has no mapping (an
// anonymous write, or a user whose mapping was removed).
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

// ResolveIdentityField replaces the UUID held in *field (a generated API
// response's createdBy/updatedBy/revokedBy/performedBy pointer, which holds
// our internal UUID immediately after a model→API mapper runs) with the raw
// external identity, in place. No-op if field is nil or points to nil/empty.
//
// This is the single place a detail response's audit-identity field is
// unwrapped before being returned to the caller — see ResolveIdentityFields
// for the batch/list equivalent.
func (s *IdentityService) ResolveIdentityField(field **string) error {
	if field == nil || *field == nil || **field == "" {
		return nil
	}
	resolved, err := s.SubForUUID(**field)
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
	resolved, err := s.SubsForUUIDs(uuids)
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
