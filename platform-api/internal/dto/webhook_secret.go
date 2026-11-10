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

// CreateWebhookSecretRequest is the request body for POST /webhook-secrets.
type CreateWebhookSecretRequest struct {
	Handle string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type,omitempty"`
}

// WebhookSecretVersion describes one rotation version. The value is never included.
type WebhookSecretVersion struct {
	Version      int       `json:"version"`
	MaskedSecret string    `json:"maskedSecret"`
	Hash         string    `json:"hash"`
	CreatedBy    string    `json:"createdBy,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

// WebhookSecretInfo is returned on GET (list and by id).
type WebhookSecretInfo struct {
	Handle    string                 `json:"id"`
	Name      string                 `json:"name"`
	Type      string                 `json:"type"`
	CreatedBy string                 `json:"createdBy,omitempty"`
	CreatedAt time.Time              `json:"createdAt"`
	Versions  []WebhookSecretVersion `json:"versions,omitempty"`
}

// WebhookSecretWithValue is returned once, on create and on rotate. Secret is
// the plaintext of the newly issued version and cannot be retrieved again.
type WebhookSecretWithValue struct {
	WebhookSecretInfo
	Secret string `json:"secret"`
}

// WebhookSecretListResponse wraps the paginated list of webhook secrets.
type WebhookSecretListResponse struct {
	Count      int                  `json:"count"`
	List       []*WebhookSecretInfo `json:"list"`
	Pagination Pagination           `json:"pagination"`
}
