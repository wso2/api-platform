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
	"time"

	"platform-api/src/internal/database"
	"platform-api/src/internal/model"
)

// CustomPolicyRepo implements CustomPolicyRepository
type CustomPolicyRepo struct {
	db *database.DB
}

// NewCustomPolicyRepo creates a new CustomPolicyRepo
func NewCustomPolicyRepo(db *database.DB) CustomPolicyRepository {
	return &CustomPolicyRepo{db: db}
}

// InsertCustomPolicy inserts or updates a custom policy by (organization_uuid, name, version).
func (r *CustomPolicyRepo) InsertCustomPolicy(policy *model.CustomPolicy) error {
	now := time.Now()
	query := `
		INSERT INTO gateway_custom_policies (uuid, organization_uuid, name, version, description, policy_definition, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(organization_uuid, name, version) DO UPDATE SET
			description      = excluded.description,
			policy_definition = excluded.policy_definition,
			updated_at       = excluded.updated_at
	`
	_, err := r.db.Exec(r.db.Rebind(query),
		policy.UUID, policy.OrganizationUUID, policy.Name, policy.Version,
		policy.Description, policy.PolicyDefinition, now, now,
	)
	return err
}

// GetCustomPolicyByNameAndVersion retrieves a custom policy by org, name, and version.
func (r *CustomPolicyRepo) GetCustomPolicyByNameAndVersion(orgUUID, name, version string) (*model.CustomPolicy, error) {
	query := `
		SELECT uuid, organization_uuid, name, version, description, policy_definition, created_at, updated_at
		FROM gateway_custom_policies
		WHERE organization_uuid = ? AND name = ? AND version = ?
	`
	p := &model.CustomPolicy{}
	err := r.db.QueryRow(r.db.Rebind(query), orgUUID, name, version).Scan(
		&p.UUID, &p.OrganizationUUID, &p.Name, &p.Version,
		&p.Description, &p.PolicyDefinition, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

// ListCustomPolicyByOrganization retrieves all custom policies for an organization.
func (r *CustomPolicyRepo) ListCustomPolicyByOrganization(orgUUID string) ([]*model.CustomPolicy, error) {
	query := `
		SELECT uuid, organization_uuid, name, version, description, policy_definition, created_at, updated_at
		FROM gateway_custom_policies
		WHERE organization_uuid = ?
		ORDER BY name, version
	`
	rows, err := r.db.Query(r.db.Rebind(query), orgUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []*model.CustomPolicy
	for rows.Next() {
		p := &model.CustomPolicy{}
		if err := rows.Scan(
			&p.UUID, &p.OrganizationUUID, &p.Name, &p.Version,
			&p.Description, &p.PolicyDefinition, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		policies = append(policies, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return policies, nil
}

// UpdateCustomPolicy updates an existing policy's version and definition identified by (org, name, oldVersion).
func (r *CustomPolicyRepo) UpdateCustomPolicy(policy *model.CustomPolicy, oldVersion string) error {
	now := time.Now()
	query := `
		UPDATE gateway_custom_policies
		SET version = ?, description = ?, policy_definition = ?, updated_at = ?
		WHERE organization_uuid = ? AND name = ? AND version = ?
	`
	_, err := r.db.Exec(r.db.Rebind(query),
		policy.Version, policy.Description, policy.PolicyDefinition, now,
		policy.OrganizationUUID, policy.Name, oldVersion,
	)
	return err
}

// GetCustomPolicyByUUID retrieves a custom policy by its UUID, scoped to an organization.
func (r *CustomPolicyRepo) GetCustomPolicyByUUID(orgUUID, policyUUID string) (*model.CustomPolicy, error) {
	query := `
		SELECT uuid, organization_uuid, name, version, description, policy_definition, created_at, updated_at
		FROM gateway_custom_policies
		WHERE organization_uuid = ? AND uuid = ?
	`
	p := &model.CustomPolicy{}
	err := r.db.QueryRow(r.db.Rebind(query), orgUUID, policyUUID).Scan(
		&p.UUID, &p.OrganizationUUID, &p.Name, &p.Version,
		&p.Description, &p.PolicyDefinition, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

// GetCustomPoliciesByName retrieves all versions of a custom policy for a given org and name.
func (r *CustomPolicyRepo) GetCustomPoliciesByName(orgUUID, name string) ([]*model.CustomPolicy, error) {
	query := `
		SELECT uuid, organization_uuid, name, version, description, policy_definition, created_at, updated_at
		FROM gateway_custom_policies
		WHERE organization_uuid = ? AND name = ?
		ORDER BY version
	`
	rows, err := r.db.Query(r.db.Rebind(query), orgUUID, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []*model.CustomPolicy
	for rows.Next() {
		p := &model.CustomPolicy{}
		if err := rows.Scan(
			&p.UUID, &p.OrganizationUUID, &p.Name, &p.Version,
			&p.Description, &p.PolicyDefinition, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		policies = append(policies, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return policies, nil
}

// DeleteCustomPolicy removes a custom policy by org, name, and version.
func (r *CustomPolicyRepo) DeleteCustomPolicy(orgUUID, name, version string) error {
	query := `DELETE FROM gateway_custom_policies WHERE organization_uuid = ? AND name = ? AND version = ?`
	_, err := r.db.Exec(r.db.Rebind(query), orgUUID, name, version)
	return err
}

// CountCustomPolicyUsages returns the number of APIs that reference this custom policy via the join table.
func (r *CustomPolicyRepo) CountCustomPolicyUsages(policyUUID string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM gateway_custom_policy_usages WHERE policy_uuid = ?`
	err := r.db.QueryRow(r.db.Rebind(query), policyUUID).Scan(&count)
	return count, err
}
