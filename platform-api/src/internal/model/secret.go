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
	SecretTypeAPIKey      = "API_KEY"
	SecretTypeCertificate = "CERTIFICATE"
	SecretTypePrivateKey  = "PRIVATE_KEY"
	SecretTypeGeneric     = "GENERIC"

	SecretProviderInHouse        = "IN_BUILT"
	SecretProviderAWSKMS         = "AWS_KMS"
	SecretProviderHashiCorpVault = "HASHICORP_VAULT"

	SecretStatusActive     = "ACTIVE"
	SecretStatusDeprecated = "DEPRECATED"

	// ValueScope controls where and how a secret value is supplied.
	// Only ORG_SHARED is implemented in v1; the others are reserved.
	SecretValueScopeOrgShared         = "ORG_SHARED"
	SecretValueScopeProjectShared     = "PROJECT_SHARED"
	SecretValueScopeOrgEnvironment    = "ORG_ENVIRONMENT"
	SecretValueScopeProjectEnvironment = "PROJECT_ENVIRONMENT"

	SecretDefaultValueScope = SecretValueScopeOrgShared
)

// Secret represents an encrypted secret stored in the platform.
type Secret struct {
	UUID           string    `db:"uuid"`
	OrganizationID string    `db:"organization_id"`
	Handle         string    `db:"handle"`
	ProjectID      *string   `db:"project_id"`
	DisplayName    string    `db:"display_name"`
	Description    string    `db:"description"`
	Ciphertext     []byte    `db:"ciphertext"`
	Hash           string    `db:"hash"`
	Type           string    `db:"type"`
	Provider       string    `db:"provider"`
	Status         string    `db:"status"`
	ValueScope     string    `db:"value_scope"`
	CreatedAt      time.Time `db:"created_at"`
	CreatedBy      string    `db:"created_by"`
	UpdatedAt      time.Time `db:"updated_at"`
	UpdatedBy      string    `db:"updated_by"`
}

// SecretReference identifies a resource that references a secret.
type SecretReference struct {
	Type   string `json:"type"`
	Handle string `json:"handle"`
	Name   string `json:"name"`
}
