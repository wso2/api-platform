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

package featureusage

// This file is the only Go code to change when a Platform API feature is added
// to the taxonomy (taxonomy/api-platform-taxonomy.yaml): add one row per REST
// API operation that serves it, or list it in notBound with the reason it is
// not reported from here. The tests in bindings_test.go check every row
// against the taxonomy and resources/openapi.yaml. See the
// add-feature-usage-tracking skill (.agents/skills) for the procedure.
//
// Every call to a bound operation is reported, successful or failed; outcome
// comes from the response status and is never listed here.

// bindings maps REST API operations (OpenAPI operationIds) to taxonomy
// features. Each operation appears at most once.
var bindings = []Binding{
	// Manage APIs
	{Op: "CreateRESTAPI", Feature: "apip.manage-apis.rest-api-create"},
	{Op: "ImportOpenAPI", Feature: "apip.manage-apis.rest-api-create"},
	{Op: "CreateGraphQLAPI", Feature: "apip.manage-apis.graphql-api-create", From: Sources{
		"schema_source": FormJSON("metadata", "/schemaSource").Default("introspection"),
	}},
	{Op: "UpdateRESTAPI", Feature: "apip.manage-apis.api-update", Set: Dims{"api_type": "rest-api"}},
	{Op: "UpdateGraphQLAPI", Feature: "apip.manage-apis.api-update", Set: Dims{"api_type": "graphql-api"}},
	{Op: "UpdateRESTAPISpec", Feature: "apip.manage-apis.api-definition-update"},
	{Op: "DeleteRESTAPI", Feature: "apip.manage-apis.api-delete", Set: Dims{"api_type": "rest-api"}},
	{Op: "DeleteGraphQLAPI", Feature: "apip.manage-apis.api-delete", Set: Dims{"api_type": "graphql-api"}},
	{Op: "ValidateOpenAPISpec", Feature: "apip.manage-apis.api-contract-validate",
		Set:  Dims{"contract_type": "openapi"},
		From: Sources{"contract_source": Present("file", "url")}},
	{Op: "ValidateGraphQLSchema", Feature: "apip.manage-apis.api-contract-validate",
		Set:  Dims{"contract_type": "graphql-schema"},
		From: Sources{"contract_source": FormJSON("metadata", "/schemaSource").Default("introspection")}},
	{Op: "CreateAPIDocument", Feature: "apip.manage-apis.api-document-create", From: Sources{
		"api_type":      Path("apiType").Map(apiTypes),
		"document_type": Form("type").Map(documentTypes),
	}},
	{Op: "UpdateAPIDocument", Feature: "apip.manage-apis.api-document-update", From: Sources{
		"api_type":      Path("apiType").Map(apiTypes),
		"document_type": Form("type").Map(documentTypes),
	}},
	{Op: "DeleteAPIDocument", Feature: "apip.manage-apis.api-document-delete", From: Sources{
		"api_type": Path("apiType").Map(apiTypes),
	}},
	{Op: "UpsertAPIThumbnail", Feature: "apip.manage-apis.api-thumbnail-upload", From: Sources{
		"api_type": Path("apiType").Map(apiTypes),
	}},
	{Op: "DeleteAPIThumbnail", Feature: "apip.manage-apis.api-thumbnail-remove", From: Sources{
		"api_type": Path("apiType").Map(apiTypes),
	}},
	{Op: "InvokeRESTAPIOnGateway", Feature: "apip.manage-apis.api-test-invoke"},

	// Gateways
	{Op: "CreateGateway", Feature: "apip.gateway.gateway-register", From: Sources{
		"gateway_type": JSON("/functionalityType").Default("regular"),
		"is_critical":  JSON("/isCritical").Default(false),
	}},
	{Op: "UpdateGateway", Feature: "apip.gateway.gateway-update", From: Sources{
		"gateway_type": JSON("/functionalityType"),
	}},
	{Op: "rotateGatewayToken", Feature: "apip.gateway.gateway-token-regenerate"},

	// Security
	{Op: "CreateAPIKey", Feature: "apip.security.api-key-create", Set: Dims{"artifact_kind": "rest-api"}},
	{Op: "CreateGraphQLAPIKey", Feature: "apip.security.api-key-create", Set: Dims{"artifact_kind": "graphql-api"}},
	{Op: "RevokeAPIKey", Feature: "apip.security.api-key-revoke", Set: Dims{"artifact_kind": "rest-api"}},
	{Op: "RevokeGraphQLAPIKey", Feature: "apip.security.api-key-revoke", Set: Dims{"artifact_kind": "graphql-api"}},
	{Op: "createServiceAccount", Feature: "apip.security.service-account-create"},
	{Op: "updateServiceAccount", Feature: "apip.security.service-account-update", From: Sources{
		"status": JSON("/status"),
	}},
	{Op: "deleteServiceAccount", Feature: "apip.security.service-account-delete"},
	{Op: "regenerateServiceAccountSecret", Feature: "apip.security.service-account-secret-regenerate"},

	// Lifecycle
	{Op: "DeployAPI", Feature: "apip.lifecycle.api-deploy", Set: Dims{"artifact_kind": "rest-api"}},
	{Op: "DeployGraphQLAPI", Feature: "apip.lifecycle.api-deploy", Set: Dims{"artifact_kind": "graphql-api"}},
	{Op: "UndeployDeployment", Feature: "apip.lifecycle.deployment-undeploy", Set: Dims{"artifact_kind": "rest-api"}},
	{Op: "UndeployGraphQLAPIDeployment", Feature: "apip.lifecycle.deployment-undeploy", Set: Dims{"artifact_kind": "graphql-api"}},
	{Op: "RestoreDeployment", Feature: "apip.lifecycle.deployment-restore", Set: Dims{"artifact_kind": "rest-api"}},
	{Op: "RestoreGraphQLAPIDeployment", Feature: "apip.lifecycle.deployment-restore", Set: Dims{"artifact_kind": "graphql-api"}},
	{Op: "DeleteDeployment", Feature: "apip.lifecycle.deployment-delete", Set: Dims{"artifact_kind": "rest-api"}},
	{Op: "DeleteGraphQLAPIDeployment", Feature: "apip.lifecycle.deployment-delete", Set: Dims{"artifact_kind": "graphql-api"}},
	{Op: "publishRestApiToApiPortal", Feature: "apip.lifecycle.api-publish", Set: Dims{"api_type": "rest-api"}},
	{Op: "publishGraphQLApiToApiPortal", Feature: "apip.lifecycle.api-publish", Set: Dims{"api_type": "graphql-api"}},
	{Op: "publishMcpProxyToApiPortal", Feature: "apip.lifecycle.api-publish", Set: Dims{"api_type": "mcp-proxy"}},
	{Op: "unpublishRestApiFromApiPortal", Feature: "apip.lifecycle.api-unpublish", Set: Dims{"api_type": "rest-api"}},
	{Op: "unpublishGraphQLApiFromApiPortal", Feature: "apip.lifecycle.api-unpublish", Set: Dims{"api_type": "graphql-api"}},
	{Op: "unpublishMcpProxyFromApiPortal", Feature: "apip.lifecycle.api-unpublish", Set: Dims{"api_type": "mcp-proxy"}},
	{Op: "deprecateRestApiOnApiPortal", Feature: "apip.lifecycle.api-deprecate", Set: Dims{"api_type": "rest-api"}},
	{Op: "deprecateGraphQLApiOnApiPortal", Feature: "apip.lifecycle.api-deprecate", Set: Dims{"api_type": "graphql-api"}},
	{Op: "deprecateMcpProxyOnApiPortal", Feature: "apip.lifecycle.api-deprecate", Set: Dims{"api_type": "mcp-proxy"}},
	{Op: "saveApiPublicationDraft", Feature: "apip.lifecycle.publication-draft-save",
		Set: Dims{"draft_part": "metadata"}, From: Sources{"api_type": Path("apiType").Map(portalAPITypes)}},
	{Op: "saveApiPublicationDraftDefinition", Feature: "apip.lifecycle.publication-draft-save",
		Set: Dims{"draft_part": "definition"}, From: Sources{"api_type": Path("apiType").Map(portalAPITypes)}},
	{Op: "saveApiPublicationDraftThumbnail", Feature: "apip.lifecycle.publication-draft-save",
		Set: Dims{"draft_part": "thumbnail"}, From: Sources{"api_type": Path("apiType").Map(portalAPITypes)}},
	{Op: "saveApiPublicationDraftLandingPage", Feature: "apip.lifecycle.publication-draft-save",
		Set: Dims{"draft_part": "landing-page"}, From: Sources{"api_type": Path("apiType").Map(portalAPITypes)}},

	// Monetization
	{Op: "CreateSubscriptionPlan", Feature: "apip.monetization.subscription-plan-create"},
	{Op: "UpdateSubscriptionPlan", Feature: "apip.monetization.subscription-plan-update", From: Sources{
		"status": JSON("/status").Map(map[string]string{"ACTIVE": "active", "INACTIVE": "inactive"}),
	}},
	{Op: "DeleteSubscriptionPlan", Feature: "apip.monetization.subscription-plan-delete"},
	{Op: "CreateMeter", Feature: "apip.monetization.meter-create"},
	{Op: "UpdateMeter", Feature: "apip.monetization.meter-update"},
	{Op: "DeleteMeter", Feature: "apip.monetization.meter-delete"},

	// Administration
	{Op: "CreateProject", Feature: "apip.admin.project-create"},
	{Op: "DeleteProject", Feature: "apip.admin.project-delete"},
}

// notBound lists features the taxonomy marks "Emitted by Platform API." that
// are deliberately not reported from a REST API binding, with the reason.
var notBound = map[string]string{
	"apip.security.service-account-token-issue": "POST /service-accounts/token is unauthenticated machine " +
		"traffic: it carries no organization in context, so the call cannot be attributed",
}

// apiTypes maps the {apiType} path values the /apis/{apiType}/{apiId} routes
// accept (the URL form and the Go constant form) to the taxonomy's api_type.
var apiTypes = map[string]string{
	"rest-api":    "rest-api",
	"RestApi":     "rest-api",
	"graphql-api": "graphql-api",
	"GraphQLApi":  "graphql-api",
}

// portalAPITypes is apiTypes plus MCP proxies, which can be published to an
// API Portal.
var portalAPITypes = map[string]string{
	"rest-api":    "rest-api",
	"RestApi":     "rest-api",
	"graphql-api": "graphql-api",
	"GraphQLApi":  "graphql-api",
	"mcp-proxy":   "mcp-proxy",
	"MCPProxy":    "mcp-proxy",
	"Mcp":         "mcp-proxy",
}

// documentTypes maps the API document types (matched regardless of case, as
// the API matches them) to the taxonomy's document_type values.
var documentTypes = map[string]string{
	"HowTo":        "how-to",
	"Samples":      "samples",
	"SupportForum": "support-forum",
	"PublicForum":  "public-forum",
	"Other":        "other",
}
