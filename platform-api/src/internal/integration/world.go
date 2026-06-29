//go:build integration

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

package integration

import (
	"fmt"

	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
)

// world holds the state of a single scenario: the database under test plus the
// identifiers and intermediate results threaded between its Given/When/Then
// steps. A fresh world (and a fresh database) is created per scenario so the
// scenarios stay independent.
type world struct {
	it *itDB

	// Parent graph seeded through the real repositories.
	orgID  string
	projID string

	// Raw object graph (cascade scenarios), seeded via direct INSERTs.
	g graph

	// REST API CRUD scenario.
	createdAPIIDs []string
	firstAPI      *model.API

	// Gateway / token scenario.
	gatewayIDs     []string
	firstGatewayID string
	gwToken        *model.GatewayToken
	tokenHash      string

	// API key scenario.
	artifactUUID string

	// Pagination scenarios.
	orgTotal  int
	projN     int
	seenCount int

	// Subscription plan scenario.
	throttle     int
	throttleUnit string
	listedPlanID string

	// Application lookup scenario.
	appUUID   string
	appHandle string

	// LLM control-plane scenarios.
	templateUUID   string
	templateHandle string
	providerUUID   string
	providerHandle string
	proxyHandle    string

	// MCP control-plane scenario.
	mcpHandle string

	// WebBroker API scenario.
	webbrokerHandles []string

	// Secret scenario.
	secretHandles     []string
	firstSecretCipher []byte
	firstSecretHash   string
	rotatedCipher     []byte
	rotatedHash       string

	// Deployment scenario.
	depArtifactID  string
	depGatewayID   string
	depIDs         []string
	lastDepContent []byte
}

// graph holds the identifiers seeded into a single organization for the cascade
// scenarios. It touches every table whose foreign keys were changed for SQL
// Server (applications, subscriptions, deployments, deployment_status,
// publication_mappings) plus their parents.
type graph struct {
	org, project, app          string
	apiArtifact, depArtifact   string
	plan, sub, gateway, deploy string
	apiKey                     string
	planLimit                  string
}

// openDB opens a fresh database for the scenario. Mirrors the "Given a clean
// platform-api database" background step.
func (w *world) openDB() error {
	it, err := openITDB()
	if err != nil {
		return err
	}
	w.it = it
	return nil
}

func (w *world) close() {
	if w.it != nil {
		w.it.cleanup()
		w.it = nil
	}
}

// seedOrgProject creates one organization and one project through the real
// repositories. It is the lightweight parent graph for the CRUD scenarios
// (REST APIs, gateways and API keys all hang off these).
func (w *world) seedOrgProject(prefix string) error {
	orgRepo := repository.NewOrganizationRepo(w.it.db)
	projectRepo := repository.NewProjectRepo(w.it.db)

	org := &model.Organization{ID: id(), Handle: prefix + "-" + id()[:8], Name: prefix + " org", Region: "us"}
	if err := orgRepo.CreateOrganization(org); err != nil {
		return fmt.Errorf("[%s] create org failed: %w", w.it.driver, err)
	}
	projID := id()
	pName := prefix + "-proj-" + projID[:6]
	proj := &model.Project{ID: projID, Handle: pName, Name: pName, OrganizationID: org.ID, Description: "p"}
	if err := projectRepo.CreateProject(proj); err != nil {
		return fmt.Errorf("[%s] create project failed: %w", w.it.driver, err)
	}
	w.orgID, w.projID = org.ID, projID
	return nil
}

// seedOrgGraph inserts a representative object graph for one organization that
// touches every table whose foreign keys were changed for SQL Server plus their
// parents, via direct INSERTs (so the cascade behavior, not the repository code,
// is what is under test).
func (w *world) seedOrgGraph() error {
	g := graph{
		org: id(), project: id(), app: id(),
		apiArtifact: id(), depArtifact: id(),
		plan: id(), sub: id(), gateway: id(), deploy: id(),
		apiKey:    id(),
		planLimit: id(),
	}

	stmts := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO organizations (uuid, handle, name, region) VALUES (?, ?, ?, ?)`,
			[]any{g.org, "h-" + g.org[:8], "it org", "us"}},
		{`INSERT INTO projects (uuid, handle, name, organization_uuid) VALUES (?, ?, ?, ?)`,
			[]any{g.project, "proj", "proj", g.org}},
		{`INSERT INTO applications (uuid, handle, project_uuid, organization_uuid, name, type) VALUES (?, ?, ?, ?, ?, ?)`,
			[]any{g.app, "app-" + g.app[:8], g.project, g.org, "app", "standard"}},
		// REST API: an artifact + its rest_apis row (shared uuid).
		{`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES (?, ?, ?)`,
			[]any{g.apiArtifact, "rest_api", g.org}},
		{`INSERT INTO rest_apis (uuid, organization_uuid, handle, name, version, project_uuid, lifecycle_status, configuration) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			[]any{g.apiArtifact, g.org, "api-" + g.apiArtifact[:8], "api", "v1.0", g.project, "CREATED", []byte("{}")}},
		{`INSERT INTO subscription_plans (uuid, handle, name, organization_uuid) VALUES (?, ?, ?, ?)`,
			[]any{g.plan, "plan-" + g.plan[:8], "Plan " + g.plan[:8], g.org}},
		{`INSERT INTO subscription_plan_limits (uuid, subscription_plan_uuid, limit_type, limit_count, time_unit) VALUES (?, ?, ?, ?, ?)`,
			[]any{g.planLimit, g.plan, "REQUEST_COUNT", 100, "MINUTE"}},
		{`INSERT INTO subscriptions (uuid, artifact_uuid, subscriber_id, subscription_token, subscription_token_hash, subscription_plan_uuid, organization_uuid) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			[]any{g.sub, g.apiArtifact, "subscriber", "tok-" + g.sub[:8], "hash-" + g.sub[:8], g.plan, g.org}},
		// Gateway + a deployment + its current status.
		{`INSERT INTO gateways (uuid, organization_uuid, handle, name, vhost, properties) VALUES (?, ?, ?, ?, ?, ?)`,
			[]any{g.gateway, g.org, "gw-" + g.gateway[:8], "gw", "localhost", []byte("{}")}},
		{`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES (?, ?, ?)`,
			[]any{g.depArtifact, "rest_api", g.org}},
		{`INSERT INTO deployments (uuid, name, artifact_uuid, organization_uuid, gateway_uuid, content) VALUES (?, ?, ?, ?, ?, ?)`,
			[]any{g.deploy, "d", g.depArtifact, g.org, g.gateway, []byte("x")}},
		{`INSERT INTO deployment_status (artifact_uuid, organization_uuid, gateway_uuid, deployment_uuid) VALUES (?, ?, ?, ?)`,
			[]any{g.depArtifact, g.org, g.gateway, g.deploy}},
		// An API key on the deployment artifact + its application mapping.
		{`INSERT INTO api_keys (uuid, artifact_uuid, name, masked_api_key, api_key_hashes) VALUES (?, ?, ?, ?, ?)`,
			[]any{g.apiKey, g.depArtifact, "key", "ab12", []byte("{}")}},
		{`INSERT INTO application_api_key_mappings (application_uuid, api_key_id) VALUES (?, ?)`,
			[]any{g.app, g.apiKey}},
		{`INSERT INTO application_artifact_mappings (application_uuid, artifact_uuid) VALUES (?, ?)`,
			[]any{g.app, g.depArtifact}},
	}
	for _, s := range stmts {
		if err := w.it.exec(s.query, s.args...); err != nil {
			return err
		}
	}
	w.g = g
	return nil
}

// --- shared step helpers ---------------------------------------------------

func (w *world) aCleanDatabase() error {
	return w.openDB()
}

func (w *world) anOrgAndProjectExist() error {
	return w.seedOrgProject("api")
}

func (w *world) aSeededObjectGraph() error {
	return w.seedOrgGraph()
}

// wantCount fails unless the table has the expected number of matching rows.
func (w *world) wantCount(table, col, val string, want int) error {
	got, err := w.it.count(table, col, val)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("[%s] %s where %s=%s: want %d rows, got %d", w.it.driver, table, col, val, want, got)
	}
	return nil
}
