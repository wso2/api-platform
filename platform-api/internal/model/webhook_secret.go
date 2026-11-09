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
	// WebhookSecretMetadataTypePortal marks a secret managed for an API portal instance.
	WebhookSecretMetadataTypePortal = "PORTAL"
)

// WebhookSecretMetadata is the metadata of an org-level managed secret. The secret
// values live in WebhookSecret rows, one per rotation version.
type WebhookSecretMetadata struct {
	UUID           string    `db:"uuid"`
	OrganizationID string    `db:"organization_uuid"`
	Handle         string    `db:"handle"`
	Name           string    `db:"name"`
	Type           string    `db:"type"`
	CreatedBy      string    `db:"created_by"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedBy      string    `db:"updated_by"`
	UpdatedAt      time.Time `db:"updated_at"`
}

// WebhookSecret is one immutable, encrypted version of a WebhookSecretMetadata.
type WebhookSecret struct {
	UUID         string    `db:"uuid"`
	SecretID     string    `db:"secret_uuid"`
	Version      int       `db:"secret_version"`
	SecretEnc    []byte    `db:"secret_enc"`
	SecretHash   string    `db:"secret_hash"`
	MaskedSecret string    `db:"masked_secret"`
	CreatedBy    string    `db:"created_by"`
	CreatedAt    time.Time `db:"created_at"`
}
