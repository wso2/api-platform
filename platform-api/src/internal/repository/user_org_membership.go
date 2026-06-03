/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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
	"platform-api/src/internal/database"
	"platform-api/src/internal/model"
)

type UserOrgMembershipRepo struct {
	db *database.DB
}

func NewUserOrgMembershipRepo(db *database.DB) UserOrgMembershipRepository {
	return &UserOrgMembershipRepo{db: db}
}

// CreateMembership inserts a user-org membership record, ignoring duplicates.
func (r *UserOrgMembershipRepo) CreateMembership(userID, orgUUID, role string) error {
	var raw string
	if r.db.Driver() == "postgres" {
		raw = `INSERT INTO user_org_memberships (user_id, organization_uuid, role)
			VALUES (?, ?, ?) ON CONFLICT DO NOTHING`
	} else {
		raw = `INSERT OR IGNORE INTO user_org_memberships (user_id, organization_uuid, role)
			VALUES (?, ?, ?)`
	}
	_, err := r.db.Exec(r.db.Rebind(raw), userID, orgUUID, role)
	return err
}

// GetOrganizationsByUserID returns all organizations the user is a member of.
func (r *UserOrgMembershipRepo) GetOrganizationsByUserID(userID string) ([]*model.Organization, error) {
	query := r.db.Rebind(`
		SELECT o.uuid, o.handle, o.name, o.region, o.created_at, o.updated_at
		FROM organizations o
		JOIN user_org_memberships m ON o.uuid = m.organization_uuid
		WHERE m.user_id = ?
		ORDER BY m.created_at ASC
	`)
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orgs []*model.Organization
	for rows.Next() {
		org := &model.Organization{}
		if err := rows.Scan(&org.ID, &org.Handle, &org.Name, &org.Region, &org.CreatedAt, &org.UpdatedAt); err != nil {
			return nil, err
		}
		orgs = append(orgs, org)
	}
	return orgs, rows.Err()
}

// HasMembership returns true if the user has a membership record for the given org.
func (r *UserOrgMembershipRepo) HasMembership(userID, orgUUID string) (bool, error) {
	query := r.db.Rebind(`
		SELECT COUNT(1) FROM user_org_memberships
		WHERE user_id = ? AND organization_uuid = ?
	`)
	var count int
	err := r.db.QueryRow(query, userID, orgUUID).Scan(&count)
	return count > 0, err
}
