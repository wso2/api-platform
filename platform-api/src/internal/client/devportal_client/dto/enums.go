/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

// SubscriptionType is a typed string for subscription policy types used by DTOs.
type SubscriptionType string

const (
	// SubscriptionTypeRequestCount indicates a request-count based policy.
	SubscriptionTypeRequestCount SubscriptionType = "requestCount"
)

// Visibility indicates API visibility in DevPortal.
type APIVisibility string

const (
	APIVisibilityPublic  APIVisibility = "PUBLIC"
	APIVisibilityPrivate APIVisibility = "PRIVATE"
)

// APIStatus contains API lifecycle/status values.
type APIStatus string

const (
	APIStatusPublished   APIStatus = "PUBLISHED"
	APIStatusUnpublished APIStatus = "CREATED"
)

// APIType contains API type identifiers.
type APIType string

const (
	APITypeMCP        APIType = "MCP"
	APITypeMCPOnly    APIType = "MCPSERVERSONLY"
	APITypeAPIProxies APIType = "APISONLY"
	APITypeDefault    APIType = "DEFAULT"
	APITypeWS         APIType = "WS"
)
