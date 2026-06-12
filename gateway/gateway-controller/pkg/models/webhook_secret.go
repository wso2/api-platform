/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package models

import "time"

// WebhookSecret represents an HMAC shared secret scoped to a WebSub API.
// The ciphertext is stored AES-256-GCM encrypted; the plaintext is never persisted
// and is returned to the caller only once (on create or regenerate).
type WebhookSecret struct {
	// UUID is the unique identifier for this secret (UUIDv4).
	UUID string

	// GatewayID is the gateway this secret belongs to.
	GatewayID string

	// ArtifactUUID is the UUID of the parent WebSub API artifact.
	ArtifactUUID string

	// Name is the URL-safe slug for this secret (immutable after create).
	// Used as the human-readable reference in policy params: {{ secret "name" }}
	Name string

	// DisplayName is the human-readable label shown in the API listing.
	DisplayName string

	// Ciphertext contains the AES-256-GCM encrypted secret value.
	// Never returned to API clients; decrypted in-process only.
	Ciphertext []byte

	// Status is "active" or "revoked".
	Status string

	// CreatedAt is the timestamp when the secret was first generated.
	CreatedAt time.Time

	// UpdatedAt is the timestamp of the most recent regeneration or status change.
	UpdatedAt time.Time
}
