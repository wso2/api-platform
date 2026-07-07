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
// Accepts multipart/form-data to support file-based secret values in future.
type CreateSecretRequest struct {
	Handle      string `form:"id"          binding:"required"`
	DisplayName string `form:"displayName" binding:"required"`
	Description string `form:"description"`
	Value       string `form:"value"       binding:"required"`
	Type        string `form:"type"`
}

// UpdateSecretRequest is the request body for PUT /api/v1/secrets/:id.
// Accepts multipart/form-data to support file-based secret values in future.
type UpdateSecretRequest struct {
	DisplayName string `form:"displayName"`
	Description string `form:"description"`
	Value       string `form:"value" binding:"required"`
}

// SecretResponse is returned on POST and PUT.
type SecretResponse struct {
	Handle      string    `json:"id"`
	DisplayName string    `json:"displayName"`
	CreatedBy   string    `json:"createdBy,omitempty"`
	UpdatedBy   string    `json:"updatedBy,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// SecretSummary is returned on GET list and GET by ID — no value field.
type SecretSummary struct {
	Handle      string    `json:"id"`
	DisplayName string    `json:"displayName"`
	Description string    `json:"description,omitempty"`
	Type        string    `json:"type"`
	Provider    string    `json:"provider"`
	Status      string    `json:"status"`
	Hash        string    `json:"hash"`
	CreatedBy   string    `json:"createdBy,omitempty"`
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
	ID          string    `json:"uuid"`
	Handle      string    `json:"handle"`
	DisplayName string    `json:"name"`
	Type        string    `json:"type"`
	Provider    string    `json:"provider"`
	Status      string    `json:"status"`
	Hash        string    `json:"hash"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Value       *string   `json:"value,omitempty"`
}

// SecretSyncListResponse is the response body for GET /api/internal/v1/secrets.
type SecretSyncListResponse struct {
	List  []SecretSyncItem `json:"list"`
	Count int              `json:"count"`
}

// SecretReferenceDTO identifies a resource that references a secret. Carried
// in the standard error response's `details.references` when a delete is
// blocked by SECRET_IN_USE (see utils.ErrorResponse.Details).
type SecretReferenceDTO struct {
	Type   string `json:"type"`
	Handle string `json:"handle"`
	Name   string `json:"name"`
}

// SecretInUseDetails is the `details` payload for a SECRET_IN_USE error.
type SecretInUseDetails struct {
	References []SecretReferenceDTO `json:"references"`
}
