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

// SecretUpdatedEvent is broadcast to every gateway in the organization when a secret's
// value is rotated. Hash is the HMAC-SHA256 change-detection digest (see hashSecret) —
// safe to broadcast, since it never permits recovering the plaintext value. The
// plaintext itself is deliberately never part of this payload: the EventHub persists
// events to the shared DB, so gateways instead pull the fresh value over the
// authenticated internal secret-value endpoint once they receive this notification.
//
// Revision is UnixNano() of the secret's updated_at at the moment this event was
// raised (set by SecretService.Update/Delete from the same value just committed to
// the DB). It is not a true per-row incrementing counter — that would need a schema
// migration across three DB engines — but time.Now() on a single writer process is
// monotonically non-decreasing across successive calls, which is all a gateway needs
// to detect an event that arrived out of order relative to one it already applied for
// the same handle. See Client.secretRevisionCache in gateway-controller.
type SecretUpdatedEvent struct {
	Handle      string `json:"handle"`
	DisplayName string `json:"name"`
	Hash        string `json:"hash"`
	Revision    int64  `json:"revision"`
}

// SecretDeletedEvent is broadcast to every gateway in the organization when a
// secret is permanently deleted. Deletion only succeeds once no artifact — current
// config or any deployed snapshot, on any gateway — references the handle, so every
// gateway can safely evict its local copy on receipt.
//
// Revision — see SecretUpdatedEvent.Revision. A gateway must keep comparing against
// it even after evicting the secret locally, so a deletion event that arrives late
// (after the same handle has already been reused by a subsequent create) is not
// applied and does not evict the newly created secret.
type SecretDeletedEvent struct {
	Handle   string `json:"handle"`
	Revision int64  `json:"revision"`
}
