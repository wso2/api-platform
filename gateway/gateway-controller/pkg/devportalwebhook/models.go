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

package devportalwebhook

import (
	"encoding/json"
	"time"
)

// EventEnvelope is the common outer envelope for every devportal webhook POST.
type EventEnvelope struct {
	EventID     string          `json:"event_id"`
	EventType   string          `json:"event_type"`
	OccurredAt  time.Time       `json:"occurred_at"`
	OrgID       string          `json:"org_id"`
	GatewayType string          `json:"gateway_type"`
	Data        json.RawMessage `json:"data"`
}

// APIRef is the API identification block present in every event.
type APIRef struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	// RefID is the control-plane reference ID; empty for gateway-native APIs.
	RefID string `json:"ref_id"`
}

// SubRef is the subscription identification block used in key and subscription events.
type SubRef struct {
	RefID    string `json:"ref_id"`
	PlanName string `json:"plan_name"`
}

// EncryptedKey carries the hybrid-encrypted API key secret.
// Encryption: RSA-OAEP (SHA-256) wraps an AES-256-GCM data key.
type EncryptedKey struct {
	WrappedKey string `json:"wrappedKey"` // base64 RSA-OAEP wrapped AES key
	IV         string `json:"iv"`         // base64 12-byte GCM IV
	Tag        string `json:"tag"`        // base64 16-byte GCM auth tag
	Ciphertext string `json:"ciphertext"` // base64 AES-256-GCM ciphertext
}

// APIKeyEventData is the data payload for apikey.generated and apikey.regenerated.
type APIKeyEventData struct {
	KeyID        string       `json:"key_id"`
	Name         string       `json:"name"`
	ExpiresAt    *time.Time   `json:"expires_at"`
	API          APIRef       `json:"api"`
	Subscription *SubRef      `json:"subscription"`
	EncryptedKey EncryptedKey `json:"encrypted_key"`
}

// APIKeyRevokedData is the data payload for apikey.revoked.
type APIKeyRevokedData struct {
	KeyID string `json:"key_id"`
	Name  string `json:"name"`
	API   APIRef `json:"api"`
}

// SubscriptionCreatedData is the data payload for subscription.created.
type SubscriptionCreatedData struct {
	Subscription SubRef `json:"subscription"`
	API          APIRef `json:"api"`
}

// SubscriptionDeletedData is the data payload for subscription.deleted.
type SubscriptionDeletedData struct {
	Subscription SubRef `json:"subscription"`
}
