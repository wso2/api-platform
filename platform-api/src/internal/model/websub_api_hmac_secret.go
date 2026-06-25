/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
 *
 */

package model

import "time"

// WebSubAPIHmacSecret represents a platform-managed HMAC secret for a WebSub API.
// Plaintext is never persisted; only the AES-256-GCM encrypted form is stored.
type WebSubAPIHmacSecret struct {
	UUID            string    `json:"uuid"`
	ArtifactUUID    string    `json:"artifactUuid"`
	Name            string    `json:"name"`
	DisplayName     string    `json:"displayName"`
	EncryptedSecret string    `json:"-"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// WebSubAPIHmacSecretEvent is broadcast to gateways when a secret is created, regenerated, or deleted.
type WebSubAPIHmacSecretEvent struct {
	// ArtifactUUID is the UUID of the WebSub API artifact.
	ArtifactUUID string `json:"artifactUuid"`
	// SecretName is the slug name of the secret.
	SecretName string `json:"secretName"`
}
