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

package repository

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/database"

	"github.com/google/uuid"
)

// UserIdentityMappingRepo persists the internal-UUID <-> IdP-identity mapping.
type UserIdentityMappingRepo struct {
	db *database.DB
}

// NewUserIdentityMappingRepo creates a new UserIdentityMappingRepo.
func NewUserIdentityMappingRepo(db *database.DB) UserIdentityMappingRepository {
	return &UserIdentityMappingRepo{db: db}
}

// GetOrCreateUUID returns the internal platform UUID mapped to the given
// resolved actor identity, creating the mapping on first use. An empty
// identity returns an empty UUID without creating a row — callers that must
// never fail on a missing identity (e.g. anonymous writes) mint their own
// standalone UUID instead (see service.IdentityService.InternalUserID); this
// method never stores a row with an empty idp_id.
func (r *UserIdentityMappingRepo) GetOrCreateUUID(identity string) (string, error) {
	if identity == "" {
		return "", nil
	}

	if existing, err := r.getUUID(identity); err != nil {
		return "", err
	} else if existing != "" {
		return existing, nil
	}

	newUUID := uuid.New().String()
	query := `INSERT INTO user_idp_references (uuid, idp_id, created_at) VALUES (?, ?, ?)`
	_, err := r.db.Exec(r.db.Rebind(query), newUUID, identity, time.Now().UTC())
	if err != nil {
		if isUniqueViolation(err) {
			// Lost the race to another concurrent request inserting the same idp_id.
			return r.getUUID(identity)
		}
		return "", err
	}
	return newUUID, nil
}

func (r *UserIdentityMappingRepo) getUUID(identity string) (string, error) {
	query := `SELECT uuid FROM user_idp_references WHERE idp_id = ?`
	var mappedUUID string
	err := r.db.QueryRow(r.db.Rebind(query), identity).Scan(&mappedUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return mappedUUID, nil
}

// GetSubByUUID returns the resolved actor identity mapped to uuid, or
// found=false if uuid has no mapping (a "hanging" UUID — e.g. an anonymous
// write, or a mapping that was removed).
func (r *UserIdentityMappingRepo) GetSubByUUID(uuid string) (string, bool, error) {
	query := `SELECT idp_id FROM user_idp_references WHERE uuid = ?`
	var identity string
	err := r.db.QueryRow(r.db.Rebind(query), uuid).Scan(&identity)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return identity, true, nil
}

// GetSubsByUUIDs batch-resolves multiple UUIDs to their mapped identity in a
// single query, for list endpoints (avoids N+1). UUIDs with no mapping are
// absent from the returned map.
func (r *UserIdentityMappingRepo) GetSubsByUUIDs(uuids []string) (map[string]string, error) {
	result := make(map[string]string)
	unique := dedupeNonEmpty(uuids)
	if len(unique) == 0 {
		return result, nil
	}

	placeholders := make([]string, len(unique))
	args := make([]any, len(unique))
	for i, id := range unique {
		placeholders[i] = "?"
		args[i] = id
	}
	query := `SELECT uuid, idp_id FROM user_idp_references WHERE uuid IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var mappedUUID, identity string
		if err := rows.Scan(&mappedUUID, &identity); err != nil {
			return nil, err
		}
		result[mappedUUID] = identity
	}
	return result, rows.Err()
}

// dedupeNonEmpty returns the distinct, non-empty values of ids.
func dedupeNonEmpty(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	unique := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}

// isUniqueViolation detects DB unique-constraint violations. It delegates to the
// shared, dialect-aware IsUniqueViolation (SQLite, PostgreSQL and SQL Server).
func isUniqueViolation(err error) bool {
	return IsUniqueViolation(err)
}
