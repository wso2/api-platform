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

// APIKeyCreatedEvent represents the payload for "apikey.created" event type.
// This event is sent when an external API key is registered to hybrid gateways.
type APIKeyCreatedEvent struct {
	// ApiId identifies the API this key belongs to
	ApiId string `json:"apiId"`

	// Name is the unique name of the API key
	Name string `json:"name,omitempty"`

	// DisplayName is the display name of the API key
	DisplayName string `json:"displayName,omitempty"`

	// ApiKey is the plain API key value (hashing happens in the gateway)
	ApiKey string `json:"apiKey"`

	// ExternalRefId is an optional reference ID for tracing purposes
	ExternalRefId *string `json:"externalRefId,omitempty"`

	// Operations specifies which API operations this key can access (default: "*")
	Operations string `json:"operations"`

	// ExpiresAt is the optional expiration time in ISO 8601 format
	ExpiresAt *string `json:"expiresAt,omitempty"`
}

// APIKeyRevokedEvent represents the payload for "apikey.revoked" event type.
// This event is sent when an API key is revoked from hybrid gateways.
type APIKeyRevokedEvent struct {
	// ApiId identifies the API this key belongs to
	ApiId string `json:"apiId"`

	// KeyName is the unique name of the API key that was revoked
	KeyName string `json:"keyName"`
}

// APIKeyUpdatedEvent represents the payload for "apikey.updated" event type.
// This event is sent when an API key is updated/regenerated on hybrid gateways.
type APIKeyUpdatedEvent struct {
	// ApiId identifies the API this key belongs to
	ApiId string `json:"apiId"`

	// KeyName is the unique name of the API key being updated
	KeyName string `json:"keyName"`

	// ApiKey is the new plain API key value (hashing happens in the gateway)
	ApiKey string `json:"apiKey"`

	// ExpiresAt is the optional new expiration time in ISO 8601 format
	ExpiresAt *string `json:"expiresAt,omitempty"`
}
