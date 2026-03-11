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

package model

import "time"

// APIKey represents a persisted API key record in the database.
type APIKey struct {
	UUID           string
	ArtifactUUID   string
	Name           string
	MaskedAPIKey   string
	APIKeyHashes   string // JSON string mapping algorithm to hash e.g. {"sha256": "<hashed_api_key>"}
	Status         string
	CreatedAt      time.Time
	CreatedBy      string
	UpdatedAt      time.Time
	ExpiresAt      *time.Time
	ProvisionedBy  *string // Identifier of the developer portal that provisioned this key; nil if not provided
	AllowedTargets string  // Comma-separated list of allowed LLM providers/proxies; defaults to 'ALL'
}
