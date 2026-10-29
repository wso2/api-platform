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

const (
	AuditActionCreate   = "create"
	AuditActionUpdate   = "update"
	AuditActionDelete   = "delete"
	AuditActionDeploy   = "deploy"
	AuditActionUndeploy = "undeploy"
	AuditActionRestore  = "restore"
	AuditActionRevoke   = "revoke"
)

const (
	AuditOutcomeSucceeded = "succeeded"
	AuditOutcomeFailed    = "failed"
	AuditOutcomeDenied    = "denied"
)

const (
	AuditResourceTypeRestAPI              = "RestApi"
	AuditResourceTypeApplication          = "Application"
	AuditResourceTypeLLMProvider          = "LlmProvider"
	AuditResourceTypeLLMAPIKey            = "LlmApiKey"
	AuditResourceTypeSubscription         = "Subscription"
	AuditResourceTypeSubscriptionPlan     = "SubscriptionPlan"
	AuditResourceTypeMCP                  = "Mcp"
	AuditResourceTypeAPIKey               = "ApiKey"
	AuditResourceTypeDeployment           = "Deployment"
	AuditResourceTypeGateway              = "Gateway"
	AuditResourceTypeCustomPolicy         = "CustomPolicy"
	AuditResourceTypeProject              = "Project"
	AuditResourceTypeOrganization         = "Organization"
	AuditResourceTypeAPIPortal            = "ApiPortal"
	AuditResourceTypeAPIDefinition        = "ApiDefinition"
	AuditResourceTypeLLMProviderTemplate  = "LlmProviderTemplate"
	AuditResourceTypeLLMProxy             = "LlmProxy"
	AuditResourceTypeWebSubAPI            = "WebSubApi"
	AuditResourceTypeWebBrokerAPI         = "WebBrokerApi"
	AuditResourceTypeSecret               = "Secret"
	AuditResourceTypeGraphQLAPI           = "GraphQLApi"
	AuditResourceTypeAgentProxy           = "AgentProxy"
	AuditResourceTypeAPIDocument          = "ApiDocument"
	AuditResourceTypeMeter                = "Meter"
)

// AuditLog is the DB-row shape for one audit_log entry — persisted by
// repository.AuditLogRepo. The service-layer DTO (service.AuditEvent) is what
// callers build; DBSink converts it to AuditLog on insert.
type AuditLog struct {
	UUID             string    `db:"uuid"`
	OrganizationUUID string    `db:"organization_uuid"`
	CorrelationID    string    `db:"correlation_id"`
	Action           string    `db:"action"`
	ResourceType     string    `db:"resource_type"`
	ResourceUUID     string    `db:"resource_uuid"`
	ResourceName     string    `db:"resource_name"`
	PerformedBy      string    `db:"performed_by"`
	PerformedAt      time.Time `db:"performed_at"`
	Outcome          string    `db:"outcome"`
	EventData        string    `db:"event_data"` // JSON string; empty means NULL in DB
}
