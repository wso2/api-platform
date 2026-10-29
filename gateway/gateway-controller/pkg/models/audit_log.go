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

package models

import "time"

// AuditLog is a row in the audit_log table.
type AuditLog struct {
	UUID          string
	GatewayID     string
	CorrelationID string
	Action        string
	ResourceType  string
	ResourceUUID  string
	ResourceName  string
	PerformedBy   string
	PerformedAt   time.Time
	Outcome       string
	EventData     string // raw JSON; empty string stored as NULL
}

const (
	AuditActionCreate = "create"
	AuditActionUpdate = "update"
	AuditActionDelete = "delete"
	AuditActionRevoke = "revoke"
)

const (
	AuditOutcomeSucceeded = "succeeded"
	AuditOutcomeFailed    = "failed"
	AuditOutcomeDenied    = "denied"
)

const (
	AuditResourceTypeRestAPI             = "RestApi"
	AuditResourceTypeGraphQLAPI          = "GraphQLApi"
	AuditResourceTypeLLMProxy            = "LlmProxy"
	AuditResourceTypeLLMProvider         = "LlmProvider"
	AuditResourceTypeLLMProviderTemplate = "LlmProviderTemplate"
	AuditResourceTypeMCPProxy            = "McpProxy"
	AuditResourceTypeAgent               = "Agent"
	AuditResourceTypeAPIKey              = "ApiKey"
	AuditResourceTypeSubscriptionPlan    = "SubscriptionPlan"
	AuditResourceTypeSubscription        = "Subscription"
	AuditResourceTypeCertificate         = "Certificate"
	AuditResourceTypeSecret              = "Secret"
)
