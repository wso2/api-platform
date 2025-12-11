/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// APIKeyStatus represents the status of an API key
type APIKeyStatus string

const (
	APIKeyStatusActive  APIKeyStatus = "active"
	APIKeyStatusRevoked APIKeyStatus = "revoked"
	APIKeyStatusExpired APIKeyStatus = "expired"
)

// APIKey represents an API key for an API
type APIKey struct {
	ID         string       `json:"id" db:"id"`
	APIKey     string       `json:"api_key" db:"api_key"`
	APIName    string       `json:"api_name" db:"api_name"`
	APIVersion string       `json:"api_version" db:"api_version"`
	Status     APIKeyStatus `json:"status" db:"status"`
	CreatedAt  time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at" db:"updated_at"`
	ExpiresAt  *time.Time   `json:"expires_at" db:"expires_at"`
}

// GenerateAPIKey creates a new API key with a random 32-byte value prefixed with "gw_"
func GenerateAPIKey(apiName, apiVersion string) (*APIKey, error) {
	// Generate UUID for the record ID
	id := uuid.New().String()

	// Generate 32 random bytes
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Encode to hex and prefix with "gw_"
	apiKey := "gw_" + hex.EncodeToString(randomBytes)

	now := time.Now()

	return &APIKey{
		ID:         id,
		APIKey:     apiKey,
		APIName:    apiName,
		APIVersion: apiVersion,
		Status:     APIKeyStatusActive,
		CreatedAt:  now,
		UpdatedAt:  now,
		ExpiresAt:  nil, // No expiration by default
	}, nil
}

// IsValid checks if the API key is valid (active and not expired)
func (ak *APIKey) IsValid() bool {
	if ak.Status != APIKeyStatusActive {
		return false
	}

	if ak.ExpiresAt != nil && time.Now().After(*ak.ExpiresAt) {
		return false
	}

	return true
}

// IsExpired checks if the API key has expired
func (ak *APIKey) IsExpired() bool {
	return ak.ExpiresAt != nil && time.Now().After(*ak.ExpiresAt)
}
