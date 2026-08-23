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

package migrationcore

// Per-entity deletes for the LIVE dual-write path (the batch backfill never calls
// these). They reproduce v2's cascade shape: deleting the parent row lets the v2
// FK ON DELETE CASCADE remove the type row and artifact_uuid/gateway_uuid children;
// tables without a cascading FK are deleted explicitly by their composite key.

// DeleteArtifact removes an artifact (any of the six types). The v2 FKs
// (rest_apis/llm_*/mcp/websub/webbroker .uuid→artifacts, and api_keys/deployments/
// deployment_status/subscriptions/mappings .artifact_uuid→artifacts, all ON DELETE
// CASCADE) remove the type row and every artifact-keyed child.
func DeleteArtifact(ex Execer, uuid string) error {
	return deleteWhere(ex, "artifacts", []string{"uuid"}, []any{uuid})
}

// DeleteOrganization removes an org; org-scoped rows cascade via their FKs.
func DeleteOrganization(ex Execer, uuid string) error {
	return deleteWhere(ex, "organizations", []string{"uuid"}, []any{uuid})
}

// DeleteProject removes a project (project-scoped children cascade).
func DeleteProject(ex Execer, uuid string) error {
	return deleteWhere(ex, "projects", []string{"uuid"}, []any{uuid})
}

// DeleteApplication removes an application (its mappings cascade).
func DeleteApplication(ex Execer, uuid string) error {
	return deleteWhere(ex, "applications", []string{"uuid"}, []any{uuid})
}

// DeleteLLMProviderTemplate removes a template (blocked by FK if providers reference it).
func DeleteLLMProviderTemplate(ex Execer, uuid string) error {
	return deleteWhere(ex, "llm_provider_templates", []string{"uuid"}, []any{uuid})
}

// DeleteSubscriptionPlan removes a plan; subscription_plan_limits cascade.
func DeleteSubscriptionPlan(ex Execer, uuid string) error {
	return deleteWhere(ex, "subscription_plans", []string{"uuid"}, []any{uuid})
}

// DeleteSubscription removes a single subscription.
func DeleteSubscription(ex Execer, uuid string) error {
	return deleteWhere(ex, "subscriptions", []string{"uuid"}, []any{uuid})
}

// DeleteGateway removes a gateway; gateway_endpoints and gateway_tokens cascade.
func DeleteGateway(ex Execer, uuid string) error {
	return deleteWhere(ex, "gateways", []string{"uuid"}, []any{uuid})
}

// DeleteGatewayToken removes a single gateway token.
func DeleteGatewayToken(ex Execer, uuid string) error {
	return deleteWhere(ex, "gateway_tokens", []string{"uuid"}, []any{uuid})
}

// DeleteGatewayCustomPolicy removes a policy; gateway_custom_policy_usages cascade.
func DeleteGatewayCustomPolicy(ex Execer, uuid string) error {
	return deleteWhere(ex, "gateway_custom_policies", []string{"uuid"}, []any{uuid})
}

// DeleteAPIKey removes an api key; application_api_key_mappings cascade.
func DeleteAPIKey(ex Execer, uuid string) error {
	return deleteWhere(ex, "api_keys", []string{"uuid"}, []any{uuid})
}

// DeleteDeployment removes a single deployment (deployment_status cascades).
func DeleteDeployment(ex Execer, uuid string) error {
	return deleteWhere(ex, "deployments", []string{"uuid"}, []any{uuid})
}

// ---- composite-key children (deleted directly when only the mapping changes) ----

// DeleteArtifactGatewayMapping removes one artifact↔gateway association.
func DeleteArtifactGatewayMapping(ex Execer, org, artifactUUID, gatewayUUID string) error {
	return deleteWhere(ex, "artifact_gateway_mappings",
		[]string{"organization_uuid", "artifact_uuid", "gateway_uuid"}, []any{org, artifactUUID, gatewayUUID})
}

// DeleteApplicationAPIKeyMapping removes one application↔api-key mapping.
func DeleteApplicationAPIKeyMapping(ex Execer, applicationUUID, apiKeyID string) error {
	return deleteWhere(ex, "application_api_key_mappings",
		[]string{"application_uuid", "api_key_id"}, []any{applicationUUID, apiKeyID})
}

// DeleteApplicationArtifactMapping removes one application↔artifact mapping.
func DeleteApplicationArtifactMapping(ex Execer, applicationUUID, artifactUUID string) error {
	return deleteWhere(ex, "application_artifact_mappings",
		[]string{"application_uuid", "artifact_uuid"}, []any{applicationUUID, artifactUUID})
}

// DeletePolicyUsage removes one policy-usage row.
func DeletePolicyUsage(ex Execer, policyUUID, artifactUUID string) error {
	return deleteWhere(ex, "gateway_custom_policy_usages",
		[]string{"policy_uuid", "artifact_uuid"}, []any{policyUUID, artifactUUID})
}

// DeleteDeploymentStatus removes the current-state row for an artifact+gateway.
func DeleteDeploymentStatus(ex Execer, org, artifactUUID, gatewayUUID string) error {
	return deleteWhere(ex, "deployment_status",
		[]string{"organization_uuid", "artifact_uuid", "gateway_uuid"}, []any{org, artifactUUID, gatewayUUID})
}
