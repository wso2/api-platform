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
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"

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

// maxDisplayNameUnits is the width of user_idp_references.display_name in
// UTF-16 code units, the unit SQL Server's NVARCHAR(255) counts. Capping in
// these units also fits Postgres VARCHAR(255), which counts characters.
const maxDisplayNameUnits = 255

// identityLookupBatchSize bounds the UUIDs bound into one IN (...) query,
// well under SQL Server's limit of 2,100 parameters per statement.
const identityLookupBatchSize = 500

// ErrDisplayNameNotSaved wraps a failure to record a display name on a row that
// was otherwise found or created. The UUID returned alongside it is valid, so
// callers can log it and carry on: the display name is for display only.
var ErrDisplayNameNotSaved = errors.New("display name not saved")

// UserIdentity is one user_idp_references row as seen by readers.
// DisplayName is empty when the row has none.
type UserIdentity struct {
	IdpID       string
	DisplayName string
}

// GetOrCreateUUIDWithDisplayName is GetOrCreateUUID that also records the
// user's display name. The row is found or created exactly as GetOrCreateUUID
// does, without the name, so a name the database rejects can never stop the
// user being mapped; the name is then saved by a separate update, made only
// when displayName is non-empty and differs from the stored value. An empty
// value (the claim was absent from the token) never clears a stored name.
func (r *UserIdentityMappingRepo) GetOrCreateUUIDWithDisplayName(identity, displayName string) (string, error) {
	if identity == "" {
		return "", nil
	}
	displayName = normalizeDisplayName(displayName)

	existing, stored, err := r.getUUIDAndDisplayName(identity)
	if err != nil {
		return "", err
	}
	if existing == "" {
		if existing, err = r.GetOrCreateUUID(identity); err != nil || existing == "" {
			return existing, err
		}
		stored = sql.NullString{}
	}
	return existing, r.refreshDisplayName(existing, stored, displayName)
}

// refreshDisplayName updates the stored display name when the incoming one is
// non-empty and different. Failures are wrapped in ErrDisplayNameNotSaved.
func (r *UserIdentityMappingRepo) refreshDisplayName(mappedUUID string, stored sql.NullString, displayName string) error {
	if displayName == "" || (stored.Valid && stored.String == displayName) {
		return nil
	}
	query := `UPDATE user_idp_references SET display_name = ? WHERE uuid = ?`
	if _, err := r.db.Exec(r.db.Rebind(query), displayName, mappedUUID); err != nil {
		return fmt.Errorf("%w: %w", ErrDisplayNameNotSaved, err)
	}
	return nil
}

func (r *UserIdentityMappingRepo) getUUIDAndDisplayName(identity string) (string, sql.NullString, error) {
	query := `SELECT uuid, display_name FROM user_idp_references WHERE idp_id = ?`
	var mappedUUID string
	var displayName sql.NullString
	err := r.db.QueryRow(r.db.Rebind(query), identity).Scan(&mappedUUID, &displayName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", sql.NullString{}, nil
		}
		return "", sql.NullString{}, err
	}
	return mappedUUID, displayName, nil
}

// GetIdentitiesByUUIDs batch-resolves multiple UUIDs to their idp_id and
// display name in a single query. UUIDs with no mapping are absent from the
// returned map.
func (r *UserIdentityMappingRepo) GetIdentitiesByUUIDs(uuids []string) (map[string]UserIdentity, error) {
	result := make(map[string]UserIdentity)
	unique := dedupeNonEmpty(uuids)
	for start := 0; start < len(unique); start += identityLookupBatchSize {
		end := min(start+identityLookupBatchSize, len(unique))
		if err := r.getIdentitiesInto(result, unique[start:end]); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (r *UserIdentityMappingRepo) getIdentitiesInto(result map[string]UserIdentity, uuids []string) error {
	placeholders := make([]string, len(uuids))
	args := make([]any, len(uuids))
	for i, id := range uuids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := `SELECT uuid, idp_id, display_name FROM user_idp_references WHERE uuid IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var mappedUUID, identity string
		var displayName sql.NullString
		if err := rows.Scan(&mappedUUID, &identity, &displayName); err != nil {
			return err
		}
		result[mappedUUID] = UserIdentity{IdpID: identity, DisplayName: displayName.String}
	}
	return rows.Err()
}

// normalizeDisplayName trims a token-supplied display name, drops invalid
// UTF-8 and control characters (Postgres rejects NUL in text), and caps it to
// maxDisplayNameUnits UTF-16 code units without splitting a character, so an
// overlong claim fits the column on every engine.
func normalizeDisplayName(name string) string {
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.ToValidUTF8(name, ""))
	name = strings.TrimSpace(name)

	units := 0
	for i, r := range name {
		units += utf16.RuneLen(r)
		if units > maxDisplayNameUnits {
			return strings.TrimSpace(name[:i])
		}
	}
	return name
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
