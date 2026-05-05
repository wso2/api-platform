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

// Package apikey provides shared types and helpers for decoding API key state
// received over xDS (AggregatedDiscoveryService) from the gateway controller.
//
// Both the gateway policy-engine and the event-gateway runtime depend on this
// package so that the proto-unwrapping and store-update logic is defined once.
package apikey

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	commonapikey "github.com/wso2/api-platform/common/apikey"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

// APIKeyStateResource represents the complete API key snapshot distributed over xDS.
type APIKeyStateResource struct {
	APIKeys   []APIKeyData `json:"apiKeys"`
	Version   int64        `json:"version"`
	Timestamp int64        `json:"timestamp"`
}

// APIKeyData represents a single API key entry in the xDS state snapshot.
type APIKeyData struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	APIKey          string     `json:"apiKey"`
	APIId           string     `json:"apiId"`
	ApplicationID   string     `json:"applicationId,omitempty"`
	ApplicationName string     `json:"applicationName,omitempty"`
	Operations      string     `json:"operations"`
	Status          string     `json:"status"`
	CreatedAt       time.Time  `json:"createdAt"`
	CreatedBy       string     `json:"createdBy"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	ExpiresAt       *time.Time `json:"expiresAt"`
	Source          string     `json:"source"`
	Issuer          *string    `json:"issuer,omitempty"`
	AllowedTargets  string     `json:"allowedTargets"`
}

// DecodeAPIKeyStateResource decodes an APIKeyState resource from its xDS wire format.
//
// The xDS server double-wraps resources:
//
//	outer Any (resource.Value) → inner Any → google.protobuf.Struct → JSON → APIKeyStateResource
func DecodeAPIKeyStateResource(resource *anypb.Any) (*APIKeyStateResource, error) {
	if resource == nil {
		return nil, fmt.Errorf("resource is nil")
	}

	// Unwrap outer Any → inner Any
	innerAny := &anypb.Any{}
	if err := proto.Unmarshal(resource.Value, innerAny); err != nil {
		return nil, fmt.Errorf("failed to unmarshal inner Any: %w", err)
	}

	// Unwrap inner Any → Struct
	s := &structpb.Struct{}
	if err := proto.Unmarshal(innerAny.Value, s); err != nil {
		return nil, fmt.Errorf("failed to unmarshal api keys struct from inner Any: %w", err)
	}

	// Struct → JSON bytes → APIKeyStateResource
	data, err := json.Marshal(s.AsMap())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Struct to JSON: %w", err)
	}

	var state APIKeyStateResource
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal APIKeyStateResource: %w", err)
	}

	return &state, nil
}

// ApplyToStore replaces the contents of store with the API keys from state.
// It builds an intermediate snapshot and calls store.ReplaceAll in one shot
// so that the store is never partially updated.
func ApplyToStore(ctx interface{ Done() <-chan struct{} }, state *APIKeyStateResource, store *commonapikey.APIkeyStore) error {
	snapshot := make(map[string]map[string]*commonapikey.APIKey, len(state.APIKeys))

	for _, d := range state.APIKeys {
		ak := &commonapikey.APIKey{
			ID:              d.ID,
			Name:            d.Name,
			APIKey:          d.APIKey,
			APIId:           d.APIId,
			ApplicationID:   d.ApplicationID,
			ApplicationName: d.ApplicationName,
			Operations:      d.Operations,
			Status:          commonapikey.APIKeyStatus(d.Status),
			CreatedAt:       d.CreatedAt,
			CreatedBy:       d.CreatedBy,
			UpdatedAt:       d.UpdatedAt,
			ExpiresAt:       d.ExpiresAt,
			Source:          d.Source,
			Issuer:          d.Issuer,
		}

		if err := addToSnapshot(snapshot, d.APIId, ak); err != nil {
			return fmt.Errorf("failed to build snapshot for API key %q (api %q): %w", d.ID, d.APIId, err)
		}
	}

	if err := store.ReplaceAll(snapshot); err != nil {
		return fmt.Errorf("failed to replace API key store: %w", err)
	}

	return nil
}

// addToSnapshot inserts an API key into the snapshot map, deduplicating by name.
func addToSnapshot(snapshot map[string]map[string]*commonapikey.APIKey, apiID string, ak *commonapikey.APIKey) error {
	ak.APIKey = strings.TrimSpace(ak.APIKey)
	if ak.APIKey == "" {
		return fmt.Errorf("%w: API key hash is empty for key %q", commonapikey.ErrInvalidInput, ak.ID)
	}

	if snapshot[apiID] == nil {
		snapshot[apiID] = make(map[string]*commonapikey.APIKey)
	}

	// Remove any existing entry with the same name (dedup by name).
	for hash, existing := range snapshot[apiID] {
		if existing.Name == ak.Name {
			delete(snapshot[apiID], hash)
			break
		}
	}

	if _, exists := snapshot[apiID][ak.APIKey]; exists {
		return commonapikey.ErrConflict
	}

	snapshot[apiID][ak.APIKey] = ak
	return nil
}
