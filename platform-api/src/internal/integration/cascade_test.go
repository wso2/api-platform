/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied. See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

//go:build integration

package integration

import (
	"testing"

	"github.com/google/uuid"
)

// graph holds the identifiers seeded into a single organization.
type graph struct {
	org, project, app          string
	apiArtifact, depArtifact   string
	plan, sub, gateway, deploy string
	apiKey                     string
	planLimit                  string
}

// seedOrgGraph inserts a representative object graph for one organization that
// touches every table whose foreign keys were changed for SQL Server
// (applications, subscriptions, deployments, deployment_status,
// publication_mappings) plus their parents.
func seedOrgGraph(t *testing.T, it *itDB) graph {
	t.Helper()
	g := graph{
		org: id(), project: id(), app: id(),
		apiArtifact: id(), depArtifact: id(),
		plan: id(), sub: id(), gateway: id(), deploy: id(),
		apiKey:    id(),
		planLimit: id(),
	}

	it.exec(t, `INSERT INTO organizations (uuid, handle, display_name, region) VALUES (?, ?, ?, ?)`,
		g.org, "h-"+g.org[:8], "it org", "us")
	it.exec(t, `INSERT INTO projects (uuid, handle, display_name, organization_uuid) VALUES (?, ?, ?, ?)`,
		g.project, "proj", "proj", g.org)
	it.exec(t, `INSERT INTO applications (uuid, handle, project_uuid, organization_uuid, display_name, type) VALUES (?, ?, ?, ?, ?, ?)`,
		g.app, "app-"+g.app[:8], g.project, g.org, "app", "standard")

	// REST API: an artifact + its rest_apis row (shared uuid).
	it.exec(t, `INSERT INTO artifacts (uuid, type, organization_uuid) VALUES (?, ?, ?)`,
		g.apiArtifact, "rest_api", g.org)
	it.exec(t, `INSERT INTO rest_apis (uuid, organization_uuid, handle, display_name, version, project_uuid, lifecycle_status, configuration) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		g.apiArtifact, g.org, "api-"+g.apiArtifact[:8], "api", "v1.0", g.project, "CREATED", []byte("{}"))

	it.exec(t, `INSERT INTO subscription_plans (uuid, handle, display_name, organization_uuid) VALUES (?, ?, ?, ?)`,
		g.plan, "plan-"+g.plan[:8], "Plan "+g.plan[:8], g.org)
	it.exec(t, `INSERT INTO subscription_plan_limits (uuid, subscription_plan_uuid, limit_type, limit_count, time_unit) VALUES (?, ?, ?, ?, ?)`,
		g.planLimit, g.plan, "REQUEST_COUNT", 100, "MINUTE")
	it.exec(t, `INSERT INTO subscriptions (uuid, artifact_uuid, subscriber_id, subscription_token, subscription_token_hash, subscription_plan_uuid, organization_uuid)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		g.sub, g.apiArtifact, "subscriber", "tok-"+g.sub[:8], "hash-"+g.sub[:8], g.plan, g.org)

	// Gateway + a deployment + its current status.
	it.exec(t, `INSERT INTO gateways (uuid, organization_uuid, handle, display_name, properties) VALUES (?, ?, ?, ?, ?)`,
		g.gateway, g.org, "gw-"+g.gateway[:8], "gw", []byte("{}"))
	it.exec(t, `INSERT INTO gateway_endpoints (gateway_uuid, host, protocol, port, context) VALUES (?, ?, ?, ?, ?)`,
		g.gateway, "localhost", "https", 443, "")
	it.exec(t, `INSERT INTO artifacts (uuid, type, organization_uuid) VALUES (?, ?, ?)`,
		g.depArtifact, "rest_api", g.org)
	it.exec(t, `INSERT INTO deployments (uuid, display_name, artifact_uuid, organization_uuid, gateway_uuid, content) VALUES (?, ?, ?, ?, ?, ?)`,
		g.deploy, "d", g.depArtifact, g.org, g.gateway, []byte("x"))
	it.exec(t, `INSERT INTO deployment_status (artifact_uuid, organization_uuid, gateway_uuid, deployment_uuid) VALUES (?, ?, ?, ?)`,
		g.depArtifact, g.org, g.gateway, g.deploy)

	// An API key on the deployment artifact + its application mapping.
	it.exec(t, `INSERT INTO api_keys (uuid, artifact_uuid, display_name, masked_api_key, api_key_hashes) VALUES (?, ?, ?, ?, ?)`,
		g.apiKey, g.depArtifact, "key", "ab12", []byte("{}"))
	it.exec(t, `INSERT INTO application_api_key_mappings (application_uuid, api_key_id) VALUES (?, ?)`, g.app, g.apiKey)
	it.exec(t, `INSERT INTO application_artifact_mappings (application_uuid, artifact_uuid) VALUES (?, ?)`, g.app, g.depArtifact)
	return g
}

// TestCascade_DeleteRestAPIRemovesSubscriptions verifies the kept
// api_uuid -> rest_apis CASCADE still removes subscriptions (the
// subscriptions.organization_uuid / artifacts edges are now NO ACTION on
// SQL Server, so cleanup must flow through rest_apis).
func TestCascade_DeleteRestAPIRemovesSubscriptions(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	g := seedOrgGraph(t, it)

	if got := it.count(t, "subscriptions", "uuid", g.sub); got != 1 {
		t.Fatalf("precondition: want 1 subscription, got %d", got)
	}
	// Mirrors APIRepo.DeleteAPI ordering: deployments are removed explicitly,
	// rest_apis + artifacts cascade the rest.
	it.exec(t, `DELETE FROM rest_apis WHERE uuid = ?`, g.apiArtifact)
	it.exec(t, `DELETE FROM artifacts WHERE uuid = ?`, g.apiArtifact)

	if got := it.count(t, "subscriptions", "uuid", g.sub); got != 0 {
		t.Fatalf("[%s] subscription not cascade-deleted after REST API delete: %d remain", it.driver, got)
	}
}

// TestCascade_DeleteGatewayRemovesDeployments verifies gateway deletion still
// removes its deployments and deployment_status (deployment_status now cascades
// only via deployment_uuid on SQL Server).
func TestCascade_DeleteGatewayRemovesDeployments(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	g := seedOrgGraph(t, it)

	if got := it.count(t, "deployment_status", "deployment_uuid", g.deploy); got != 1 {
		t.Fatalf("precondition: want 1 deployment_status, got %d", got)
	}
	it.exec(t, `DELETE FROM gateways WHERE uuid = ? AND organization_uuid = ?`, g.gateway, g.org)

	if got := it.count(t, "deployments", "uuid", g.deploy); got != 0 {
		t.Fatalf("[%s] deployment not removed after gateway delete: %d remain", it.driver, got)
	}
	if got := it.count(t, "deployment_status", "deployment_uuid", g.deploy); got != 0 {
		t.Fatalf("[%s] deployment_status not removed after gateway delete: %d remain", it.driver, got)
	}
}

// TestCascade_DeleteApplicationRemovesMappings verifies application deletion
// still cascade-removes its key and artifact mappings (these edges are
// unchanged, but the app's organization edge changed, so it is worth pinning).
func TestCascade_DeleteApplicationRemovesMappings(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	g := seedOrgGraph(t, it)

	it.exec(t, `DELETE FROM applications WHERE uuid = ? AND organization_uuid = ?`, g.app, g.org)
	if got := it.count(t, "application_api_key_mappings", "api_key_id", g.apiKey); got != 0 {
		t.Fatalf("[%s] application_api_key_mappings not removed after application delete: %d remain", it.driver, got)
	}
	if got := it.count(t, "application_artifact_mappings", "application_uuid", g.app); got != 0 {
		t.Fatalf("[%s] application_artifact_mappings not removed after application delete: %d remain", it.driver, got)
	}
}

// TestCascade_DeleteProjectRemovesApplications verifies the kept projects ->
// applications CASCADE (the org-direct application edge is now NO ACTION on
// SQL Server, so applications must be cleaned via the project edge). Project
// deletion is guarded against associated APIs in the service layer, so we
// remove the API artifacts first to mirror a legitimate project delete.
func TestCascade_DeleteProjectRemovesApplications(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	g := seedOrgGraph(t, it)

	// Remove API-side rows first (as the service guard requires no APIs).
	it.exec(t, `DELETE FROM artifacts WHERE uuid = ?`, g.apiArtifact)

	it.exec(t, `DELETE FROM projects WHERE uuid = ?`, g.project)
	if got := it.count(t, "applications", "uuid", g.app); got != 0 {
		t.Fatalf("[%s] application not removed after project delete: %d remain", it.driver, got)
	}
}

// TestCascade_DeleteWebSubAPIRemovesHmacSecrets verifies that deleting the
// artifacts row that backs a WebSub API cascade-removes both the websub_apis row
// and all associated websub_api_hmac_secrets rows (two independent CASCADE edges
// from artifacts). This catches regressions if either FK is changed to NO ACTION.
func TestCascade_DeleteWebSubAPIRemovesHmacSecrets(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()

	orgUUID := id()
	projectUUID := id()
	artifactUUID := id()

	it.exec(t, `INSERT INTO organizations (uuid, handle, display_name, region) VALUES (?, ?, ?, ?)`,
		orgUUID, "wsc-"+orgUUID[:8], "cascade org", "us")
	it.exec(t, `INSERT INTO projects (uuid, handle, display_name, organization_uuid) VALUES (?, ?, ?, ?)`,
		projectUUID, "cascade-proj", "cascade-proj", orgUUID)
	it.exec(t, `INSERT INTO artifacts (uuid, type, organization_uuid) VALUES (?, ?, ?)`,
		artifactUUID, "WebSubApi", orgUUID)
	it.exec(t, `INSERT INTO websub_apis (uuid, organization_uuid, handle, display_name, version, project_uuid, lifecycle_status, configuration) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		artifactUUID, orgUUID, "ws-api-"+artifactUUID[:8], "ws-api", "v1.0", projectUUID, "CREATED", []byte("{}"))

	secret1 := id()
	secret2 := id()
	it.exec(t, `INSERT INTO websub_api_hmac_secrets (uuid, artifact_uuid, handle, encrypted_secret, status) VALUES (?, ?, ?, ?, ?)`,
		secret1, artifactUUID, "github-secret", []byte("enc1"), "active")
	it.exec(t, `INSERT INTO websub_api_hmac_secrets (uuid, artifact_uuid, handle, encrypted_secret, status) VALUES (?, ?, ?, ?, ?)`,
		secret2, artifactUUID, "gitlab-secret", []byte("enc2"), "active")

	if got := it.count(t, "websub_api_hmac_secrets", "artifact_uuid", artifactUUID); got != 2 {
		t.Fatalf("precondition: want 2 hmac secrets, got %d", got)
	}
	if got := it.count(t, "websub_apis", "uuid", artifactUUID); got != 1 {
		t.Fatalf("precondition: want 1 websub_api row, got %d", got)
	}

	it.exec(t, `DELETE FROM artifacts WHERE uuid = ?`, artifactUUID)

	if got := it.count(t, "websub_api_hmac_secrets", "artifact_uuid", artifactUUID); got != 0 {
		t.Fatalf("[%s] hmac secrets not cascade-deleted after artifact delete: %d remain", it.driver, got)
	}
	if got := it.count(t, "websub_apis", "uuid", artifactUUID); got != 0 {
		t.Fatalf("[%s] websub_api not cascade-deleted after artifact delete: %d remain", it.driver, got)
	}
}

// TestCascade_DeleteSubscriptionPlanRemovesLimits verifies that deleting a
// subscription plan cascade-removes its subscription_plan_limits rows. On SQL
// Server the limit's organization/composite edges are NO ACTION (to avoid the
// multiple-cascade-paths restriction), so cleanup must flow through the
// subscription_plan_uuid -> subscription_plans CASCADE edge. The subscription is
// removed first because subscriptions.subscription_plan_uuid blocks plan deletion.
func TestCascade_DeleteSubscriptionPlanRemovesLimits(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	g := seedOrgGraph(t, it)

	if got := it.count(t, "subscription_plan_limits", "uuid", g.planLimit); got != 1 {
		t.Fatalf("precondition: want 1 subscription_plan_limit, got %d", got)
	}
	it.exec(t, `DELETE FROM subscriptions WHERE uuid = ?`, g.sub)
	it.exec(t, `DELETE FROM subscription_plans WHERE uuid = ? AND organization_uuid = ?`, g.plan, g.org)

	if got := it.count(t, "subscription_plan_limits", "uuid", g.planLimit); got != 0 {
		t.Fatalf("[%s] subscription_plan_limit not removed after plan delete: %d remain", it.driver, got)
	}
}

func id() string { return uuid.NewString() }
