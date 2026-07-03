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

// Package webhook implements the Platform API control-plane webhook receiver.
//
// It is the receiver-side counterpart to the Developer Portal producer described in
// docs-local/devportal-webhook-integration.md. The Developer Portal commits a domain
// change, writes it to a transactional outbox, and delivers signed webhooks (at-least-once)
// to this endpoint. Platform API verifies the request, decrypts any sensitive fields,
// reconciles its own state by reusing the existing API-key / subscription services, and
// lets the existing EventHub -> WebSocket sync propagate the change to gateway replicas.
package webhook

import (
	"encoding/json"
	"fmt"
)

// CurrentSchemaVersion is assumed when an event omits schema_version. The producer lists
// schema_version as forward-compat work, so a missing value must be tolerated (not rejected).
const CurrentSchemaVersion = "1.0"

// EncryptedKey is the hybrid-encrypted (AES-256-GCM + RSA-OAEP) representation of a sensitive
// value (an API key secret, etc.). wrappedKey is the AES content key wrapped with the receiver's
// RSA public key; iv/tag/ciphertext are the AES-GCM parts. See Decryptor.
type EncryptedKey struct {
	WrappedKey string `json:"wrappedKey"`
	IV         string `json:"iv"`
	Tag        string `json:"tag"`
	Ciphertext string `json:"ciphertext"`
}

// Empty reports whether the encrypted field is unset.
func (e *EncryptedKey) Empty() bool {
	return e == nil || (e.WrappedKey == "" && e.Ciphertext == "")
}

// orgRef identifies the organization an event targets. ref_id is the control-plane org
// reference (the Platform API organization UUID); the Developer Portal falls back to its own
// org UUID when the org has not been linked to the control plane.
type orgRef struct {
	RefID string `json:"ref_id"`
}

// Envelope is the common webhook event envelope. It mirrors the Developer Portal DP_EVENT
// outbox row. Data carries the event-type-specific payload and is decoded by each handler.
type Envelope struct {
	EventID    string `json:"event_id"`
	EventType  string `json:"event_type"`
	OccurredAt string `json:"occurred_at"`
	Org        orgRef `json:"org"`
	// OrgID is the legacy flat org identifier, superseded by Org.RefID. DecodeEnvelope copies
	// Org.RefID into it when present, so downstream handlers can keep reading env.OrgID.
	OrgID         string `json:"org_id"`
	GatewayType   string `json:"gateway_type"`
	AggregateType string `json:"aggregate_type"`
	AggregateID   string `json:"aggregate_id"`
	SchemaVersion string `json:"schema_version"`
	// EncryptedFields lists which fields inside Data carry an encrypted envelope. Always present
	// in the new contract — empty when no fields are encrypted.
	EncryptedFields []string        `json:"encrypted_fields"`
	Data            json.RawMessage `json:"data"`
}

// IsEncrypted reports whether the named Data field carries an encrypted envelope, per the
// envelope's encrypted_fields list.
func (e *Envelope) IsEncrypted(field string) bool {
	for _, f := range e.EncryptedFields {
		if f == field {
			return true
		}
	}
	return false
}

// DecodeEnvelope parses the raw request body into an Envelope. A decode failure maps to 400.
func DecodeEnvelope(body []byte) (*Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidEnvelope, err)
	}
	// Prefer the nested org.ref_id; fall back to the legacy flat org_id. Downstream handlers
	// read env.OrgID, so normalize onto it.
	if env.Org.RefID != "" {
		env.OrgID = env.Org.RefID
	}
	return &env, nil
}

// Validate checks that the mandatory envelope fields are present. schema_version is optional;
// when absent it is treated as CurrentSchemaVersion rather than rejected.
func (e *Envelope) Validate() error {
	if e.EventID == "" {
		return fmt.Errorf("%w: event_id is required", ErrInvalidEnvelope)
	}
	if e.EventType == "" {
		return fmt.Errorf("%w: event_type is required", ErrInvalidEnvelope)
	}
	if e.OrgID == "" {
		return fmt.Errorf("%w: org.ref_id is required", ErrInvalidEnvelope)
	}
	if e.SchemaVersion == "" {
		e.SchemaVersion = CurrentSchemaVersion
	}
	return nil
}

// decodeData unmarshals the event-specific data payload into out.
func (e *Envelope) decodeData(out interface{}) error {
	if len(e.Data) == 0 {
		return fmt.Errorf("%w: data is required for event %s", ErrInvalidEnvelope, e.EventType)
	}
	if err := json.Unmarshal(e.Data, out); err != nil {
		return fmt.Errorf("%w: invalid data for event %s: %v", ErrInvalidEnvelope, e.EventType, err)
	}
	return nil
}
