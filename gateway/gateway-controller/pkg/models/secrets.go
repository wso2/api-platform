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

// Secret represents a secret in the storage layer.
// Handle (metadata.name) is the sole identifier — no separate UUID is generated.
// Provider and KeyVersion are not stored as separate fields; they are encoded
// inside the Ciphertext envelope (enc:<provider>:v1:<key-version>:<base64>).
type Secret struct {
	Handle      string    // User-provided unique identifier (primary key)
	DisplayName string    // Human-readable display name (stored for list views)
	Description *string   // Optional human-readable description (may be nil)
	Value       string    // Plaintext secret data (in-memory only, never persisted)
	Ciphertext  []byte    // Encrypted secret with self-describing metadata (stored in database)
	CreatedAt   time.Time // Creation timestamp (UTC)
	UpdatedAt   time.Time // Last modification timestamp (UTC)
}

// SecretMeta contains non-sensitive metadata for list operations.
// Used by GetSecrets to provide rich list views without exposing secret material.
type SecretMeta struct {
	Handle      string    // User-provided unique identifier
	DisplayName string    // Human-readable display name
	CreatedAt   time.Time // Creation timestamp (UTC)
	UpdatedAt   time.Time // Last modification timestamp (UTC)
}
