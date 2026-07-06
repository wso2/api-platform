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
	"time"

	"github.com/wso2/api-platform/platform-api/internal/database"
)

// UserOrganizationMappingRepo persists user<->organization membership rows.
type UserOrganizationMappingRepo struct {
	db *database.DB
}

// NewUserOrganizationMappingRepo creates a new UserOrganizationMappingRepo.
func NewUserOrganizationMappingRepo(db *database.DB) UserOrganizationMappingRepository {
	return &UserOrganizationMappingRepo{db: db}
}

// AddMembership records that userUUID has onboarded to orgUUID. Idempotent: a
// duplicate (userUUID, orgUUID) pair is treated as success, not an error.
func (r *UserOrganizationMappingRepo) AddMembership(userUUID, orgUUID string) error {
	query := `INSERT INTO user_organization_mappings (user_uuid, org_uuid, created_at) VALUES (?, ?, ?)`
	_, err := r.db.Exec(r.db.Rebind(query), userUUID, orgUUID, time.Now())
	if err != nil {
		if isUniqueViolation(err) {
			return nil
		}
		return err
	}
	return nil
}

// DeleteByUser removes all membership rows for userUUID, within tx. Callers
// must run this before deleting the referenced user_idp_references row (no
// ON DELETE CASCADE on that FK by design).
func (r *UserOrganizationMappingRepo) DeleteByUser(tx *sql.Tx, userUUID string) error {
	_, err := tx.Exec(r.db.Rebind(`DELETE FROM user_organization_mappings WHERE user_uuid = ?`), userUUID)
	return err
}

// DeleteByOrg removes all membership rows for orgUUID, within tx. Callers
// must run this before deleting the referenced organizations row (no
// ON DELETE CASCADE on that FK by design).
func (r *UserOrganizationMappingRepo) DeleteByOrg(tx *sql.Tx, orgUUID string) error {
	_, err := tx.Exec(r.db.Rebind(`DELETE FROM user_organization_mappings WHERE org_uuid = ?`), orgUUID)
	return err
}
