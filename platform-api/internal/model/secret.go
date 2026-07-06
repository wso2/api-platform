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

package model

import "time"

const (
	SecretTypeCertificate = "CERTIFICATE"
	SecretTypeGeneric     = "GENERIC"

	SecretProviderInHouse        = "IN_BUILT"
	SecretProviderAWSKMS         = "AWS_KMS"
	SecretProviderHashiCorpVault = "HASHICORP_VAULT"

	SecretStatusActive     = "ACTIVE"
	SecretStatusDeprecated = "DEPRECATED"

	// SecretScopeType* identify the kind of entity a secret is scoped to.
	SecretScopeTypeOrg      = "org"
	SecretScopeTypeProject  = "project"
	SecretScopeTypeArtifact = "artifact"
)

// Secret represents an encrypted secret stored in the platform.
type Secret struct {
	UUID           string        `db:"uuid"`
	OrganizationID string        `db:"organization_uuid"`
	Handle         string        `db:"handle"`
	DisplayName    string        `db:"name"`
	Description    string        `db:"description"`
	Ciphertext     []byte        `db:"ciphertext"`
	Hash           string        `db:"hash"`
	Type           string        `db:"type"`
	Provider       string        `db:"provider"`
	Status         string        `db:"status"`
	CreatedAt      time.Time     `db:"created_at"`
	CreatedBy      string        `db:"created_by"`
	UpdatedAt      time.Time     `db:"updated_at"`
	UpdatedBy      string        `db:"updated_by"`
	Scopes         []SecretScope `db:"-"`
}

// SecretScope links a secret to a scoped entity (org, project, artifact).
type SecretScope struct {
	SecretUUID string `db:"secret_uuid"`
	Scope      string `db:"scope"`
	ScopeValue string `db:"scope_value"`
}

// SecretReference identifies a resource that references a secret.
type SecretReference struct {
	Type   string `json:"type"`
	Handle string `json:"handle"`
	Name   string `json:"name"`
}
