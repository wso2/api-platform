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

package dto

import "time"

// CreateSecretRequest is the request body for POST /api/v1/secrets.
type CreateSecretRequest struct {
	Handle      string `json:"name" binding:"required"`
	DisplayName string `json:"displayName" binding:"required"`
	Description string `json:"description"`
	Value       string `json:"value" binding:"required"`
	Type        string `json:"type"`
}

// UpdateSecretRequest is the request body for PUT /api/v1/secrets/:id.
type UpdateSecretRequest struct {
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Value       string `json:"value" binding:"required"`
}

// SecretResponse is returned on POST and PUT — includes the plaintext value once.
type SecretResponse struct {
	ID          string    `json:"id"`
	Handle      string    `json:"name"`
	DisplayName string    `json:"displayName"`
	Description string    `json:"description,omitempty"`
	Value       string    `json:"value"`
	Type        string    `json:"type"`
	Provider    string    `json:"provider"`
	Status      string    `json:"status"`
	Hash        string    `json:"hash"`
	ValueScope  string    `json:"valueScope"`
	CreatedAt   time.Time `json:"createdAt"`
	CreatedBy   string    `json:"createdBy,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
	UpdatedBy   string    `json:"updatedBy,omitempty"`
}

// SecretSummary is returned on GET list and GET by ID — no value field.
type SecretSummary struct {
	ID          string    `json:"id"`
	Handle      string    `json:"name"`
	DisplayName string    `json:"displayName"`
	Description string    `json:"description,omitempty"`
	Type        string    `json:"type"`
	Provider    string    `json:"provider"`
	Status      string    `json:"status"`
	Hash        string    `json:"hash"`
	ValueScope  string    `json:"valueScope"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// SecretListResponse wraps the paginated list of secrets.
type SecretListResponse struct {
	List       []*SecretSummary `json:"list"`
	Pagination Pagination       `json:"pagination"`
}

// SecretSyncItem is returned by the internal GW sync endpoint.
// Value is only populated when the caller requests includeValues=true (startup bulk fetch).
type SecretSyncItem struct {
	ID          string    `json:"id"`
	Handle      string    `json:"name"`
	DisplayName string    `json:"displayName"`
	Type        string    `json:"type"`
	Provider    string    `json:"provider"`
	Status      string    `json:"status"`
	Hash        string    `json:"hash"`
	ValueScope  string    `json:"valueScope"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Value       *string   `json:"value,omitempty"`
}

// SecretSyncListResponse is the response body for GET /api/internal/v1/secrets.
type SecretSyncListResponse struct {
	List  []SecretSyncItem `json:"list"`
	Count int              `json:"count"`
}

// SecretDeleteConflictResponse is returned with 409 when a secret has active references.
type SecretDeleteConflictResponse struct {
	Error      string               `json:"error"`
	References []SecretReferenceDTO `json:"references"`
}

// SecretReferenceDTO identifies a resource that references a secret.
type SecretReferenceDTO struct {
	Type   string `json:"type"`
	Handle string `json:"handle"`
	Name   string `json:"name"`
}
