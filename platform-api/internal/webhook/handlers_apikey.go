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

package webhook

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
)

// looksLikeUniqueViolation reports whether err is a database unique-constraint violation, across the
// supported drivers (SQLite "UNIQUE constraint failed", PostgreSQL "duplicate key value violates
// unique constraint", SQL Server "Violation of UNIQUE KEY constraint"). Used only to make an
// already-injected key an idempotent success on a duplicate webhook delivery.
func looksLikeUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "unique constraint") ||
		strings.Contains(s, "duplicate key") ||
		strings.Contains(s, "unique key")
}

// API key event types.
const (
	EventAPIKeyGenerated          = "apikey.generated"
	EventAPIKeyRegenerated        = "apikey.regenerated"
	EventAPIKeyRevoked            = "apikey.revoked"
	EventAPIKeyApplicationUpdated = "apikey.application_updated"
)

// apiRef identifies the API an event targets. ref_id is the Platform API artifact handle (the
// Developer Portal's REFERENCE_ID); the API-key/subscription services resolve it to the API.
// type is the artifact kind (e.g. RestApi, LlmProvider) and scopes the handle lookup to exactly
// one artifact table, so every API resolution passes kind alongside the handle.
type apiRef struct {
	RefID   string `json:"ref_id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Type    string `json:"type"`
}

// kind returns the trimmed artifact kind carried by the event.
func (a *apiRef) kind() string {
	return strings.TrimSpace(a.Type)
}

// validate checks the API reference an event carries: ref_id must be present and type must be a
// recognised artifact kind, since it scopes every downstream API lookup to one artifact table.
func (a *apiRef) validate() error {
	if strings.TrimSpace(a.RefID) == "" {
		return fmt.Errorf("%w: data.api.ref_id is required", ErrInvalidEnvelope)
	}
	if a.kind() == "" {
		return fmt.Errorf("%w: data.api.type (kind) is required", ErrInvalidEnvelope)
	}
	if !constants.ValidArtifactKinds[a.kind()] {
		return fmt.Errorf("%w: invalid data.api.type (kind) %q", ErrInvalidEnvelope, a.kind())
	}
	return nil
}

// apiKeyData is the data payload for all apikey.* events. key carries the encrypted secret and is
// present only for generate/regenerate (events that carry a new secret, listed in
// encrypted_fields as "key"); revoke omits it.
//
// handle is the Platform API handle for the key: its stable identity (api_keys uniqueness is
// artifact + name), used to resolve the key on regenerate/revoke/application_updated and set as the
// key name on generate. display_name is the human-readable name shown for the key.
type apiKeyData struct {
	KeyID       string        `json:"key_id"`
	Handle      string        `json:"handle"`
	DisplayName string        `json:"display_name"`
	ExpiresAt   string        `json:"expires_at"`
	API         apiRef        `json:"api"`
	Key         *EncryptedKey `json:"key"`
}

func (d *apiKeyData) validate() error {
	return d.API.validate()
}

// handlePtr returns the key handle as a pointer, or nil when absent. When present it is the stable
// identity used by generate (as the key name) and regenerate/revoke/application_updated (to resolve
// the key), since api_keys uniqueness is artifact + name.
func (d *apiKeyData) handlePtr() *string {
	if strings.TrimSpace(d.Handle) == "" {
		return nil
	}
	v := strings.TrimSpace(d.Handle)
	return &v
}

// displayName returns the human-readable key name. It falls back to the handle when the event omits
// display_name, so the created key always has a display name.
func (d *apiKeyData) displayName() string {
	if v := strings.TrimSpace(d.DisplayName); v != "" {
		return v
	}
	return strings.TrimSpace(d.Handle)
}

func (d *apiKeyData) externalRefPtr() *string {
	if strings.TrimSpace(d.KeyID) == "" {
		return nil
	}
	v := strings.TrimSpace(d.KeyID)
	return &v
}

// parseExpiresAt converts the optional RFC3339 expires_at into a *time.Time.
func parseExpiresAt(s string) (*time.Time, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid expires_at %q: %v", ErrInvalidEnvelope, s, err)
	}
	return &t, nil
}

// handleAPIKeyGenerated decrypts the new key secret and injects it via the existing API-key service,
// which hashes it, persists it, and broadcasts to the gateways where the API is deployed.
//
// The key is identified by (api, handle), so handle is required and the operation is idempotent by
// that identity: a duplicate delivery whose key already exists is treated as success rather than
// failing on the (artifact_uuid, name) unique constraint.
func (r *Receiver) handleAPIKeyGenerated(ctx context.Context, env *Envelope) error {
	var d apiKeyData
	if err := env.decodeData(&d); err != nil {
		return err
	}
	if err := d.validate(); err != nil {
		return err
	}
	if d.handlePtr() == nil {
		return fmt.Errorf("%w: data.handle is required to identify the generated key", ErrInvalidEnvelope)
	}
	if d.Key == nil {
		return fmt.Errorf("%w: data.key (encrypted secret) is required for %s", ErrInvalidEnvelope, env.EventType)
	}

	plaintext, err := r.decryptor.Decrypt(d.Key)
	if err != nil {
		return err
	}

	expiresAt, err := parseExpiresAt(d.ExpiresAt)
	if err != nil {
		return err
	}

	req := &api.CreateAPIKeyRequest{
		// Id is the key's Platform API handle (its stable name); DisplayName is the human-readable name.
		Id:            d.handlePtr(),
		ApiKey:        plaintext,
		DisplayName:   d.displayName(),
		ExternalRefId: d.externalRefPtr(),
		ExpiresAt:     expiresAt,
	}
	// userID is empty: webhook events are system-originated, not tied to an interactive user.
	if err := r.apiKeys.CreateAPIKey(ctx, d.API.RefID, d.API.kind(), env.OrgID, "", req); err != nil {
		// Domain-level idempotency: a key already injected under this (api, handle) means a prior
		// delivery succeeded. The underlying Create surfaces a raw unique-constraint error, so match
		// on the constraint phrasing rather than a typed error.
		if looksLikeUniqueViolation(err) {
			return nil
		}
		return err
	}
	return nil
}

// handleAPIKeyRegenerated rotates an existing key. The key is identified by its handle within the API.
func (r *Receiver) handleAPIKeyRegenerated(ctx context.Context, env *Envelope) error {
	var d apiKeyData
	if err := env.decodeData(&d); err != nil {
		return err
	}
	if err := d.validate(); err != nil {
		return err
	}
	if d.handlePtr() == nil {
		return fmt.Errorf("%w: data.handle is required to identify the key to regenerate", ErrInvalidEnvelope)
	}
	if d.Key == nil {
		return fmt.Errorf("%w: data.key (encrypted secret) is required for %s", ErrInvalidEnvelope, env.EventType)
	}

	plaintext, err := r.decryptor.Decrypt(d.Key)
	if err != nil {
		return err
	}

	// Carry the Developer Portal's expiry through. The Developer Portal is authoritative for the
	// key's expiry, so a regeneration reflects data.expires_at (and clears it only when the DP
	// sends a non-expiring key). Omitting it here would reset every regenerated key to no-expiry.
	expiresAt, err := parseExpiresAt(d.ExpiresAt)
	if err != nil {
		return err
	}

	req := &api.UpdateAPIKeyRequest{
		ApiKey:        plaintext,
		ExternalRefId: d.externalRefPtr(),
		ExpiresAt:     expiresAt,
	}
	return r.apiKeys.UpdateAPIKey(ctx, d.API.RefID, d.API.kind(), env.OrgID, *d.handlePtr(), "", req)
}

// handleAPIKeyRevoked revokes an existing key, identified by its handle within the API.
func (r *Receiver) handleAPIKeyRevoked(ctx context.Context, env *Envelope) error {
	var d apiKeyData
	if err := env.decodeData(&d); err != nil {
		return err
	}
	if err := d.validate(); err != nil {
		return err
	}
	if d.handlePtr() == nil {
		return fmt.Errorf("%w: data.handle is required to identify the key to revoke", ErrInvalidEnvelope)
	}
	return r.apiKeys.RevokeAPIKey(ctx, d.API.RefID, d.API.kind(), env.OrgID, *d.handlePtr(), "")
}

// appRef is the optional application reference on apikey.application_updated. It is null when the
// key was dissociated from its application. handle is the Developer Portal application handle, which
// the Platform API stores as the application handle and uses to resolve the application; id (the
// Developer Portal's internal id) is received but not used for resolution.
type appRef struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Handle      string `json:"handle"`
}

// apiKeyApplicationData is the data payload for apikey.application_updated. The key is resolved the
// same way as the other apikey.* events — by (api.ref_id, handle) — so it embeds apiKeyData for that
// identity. application is the new owning application, or null to dissociate the key.
type apiKeyApplicationData struct {
	apiKeyData
	Application *appRef `json:"application"`
}

// handleAPIKeyApplicationUpdated reconciles a change to which application an API key belongs to.
// The key is resolved exactly like revoke — by (api.ref_id, handle); application is the new owner, or
// null to dissociate. SetAPIKeyApplication is idempotent, so a redelivery re-applies the same owner.
func (r *Receiver) handleAPIKeyApplicationUpdated(ctx context.Context, env *Envelope) error {
	var d apiKeyApplicationData
	if err := env.decodeData(&d); err != nil {
		return err
	}
	if err := d.validate(); err != nil {
		return err
	}
	if d.handlePtr() == nil {
		return fmt.Errorf("%w: data.handle is required to identify the key", ErrInvalidEnvelope)
	}
	appHandle := ""
	if d.Application != nil {
		appHandle = strings.TrimSpace(d.Application.Handle)
		if appHandle == "" {
			return fmt.Errorf("%w: data.application.handle is required when application is present (send null to dissociate)", ErrInvalidEnvelope)
		}
	}
	return r.apps.SetAPIKeyApplication(*d.handlePtr(), d.API.RefID, d.API.kind(), appHandle, env.OrgID, "")
}
