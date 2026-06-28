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

	"github.com/cucumber/godog"
)

// registerCascadeSteps wires the foreign-key cascade-delete scenarios.
func registerCascadeSteps(ctx *godog.ScenarioContext, w *world) {
	ctx.Step(`^a seeded organization object graph$`, w.aSeededObjectGraph)

	ctx.Step(`^I delete the REST API artifact$`, w.deleteRESTAPIArtifact)
	ctx.Step(`^the subscription is removed$`, w.subscriptionRemoved)

	ctx.Step(`^I delete the gateway$`, w.deleteGateway)
	ctx.Step(`^the deployment is removed$`, w.deploymentRemoved)
	ctx.Step(`^the deployment status is removed$`, w.deploymentStatusRemoved)

	ctx.Step(`^I delete the application$`, w.deleteApplication)
	ctx.Step(`^the application api key mapping is removed$`, w.appKeyMappingRemoved)
	ctx.Step(`^the application artifact mapping is removed$`, w.appArtifactMappingRemoved)

	ctx.Step(`^I delete the project$`, w.deleteProject)
	ctx.Step(`^the application is removed$`, w.applicationRemoved)

	ctx.Step(`^I delete the subscription and its plan$`, w.deleteSubscriptionAndPlan)
	ctx.Step(`^the subscription plan limit is removed$`, w.subscriptionPlanLimitRemoved)

	ctx.Step(`^a WebSub API with (\d+) HMAC secrets$`, w.seedWebSubWithSecrets)
	ctx.Step(`^I delete the WebSub API artifact$`, w.deleteWebSubArtifact)
	ctx.Step(`^the HMAC secrets are removed$`, w.hmacSecretsRemoved)
	ctx.Step(`^the WebSub API row is removed$`, w.webSubRowRemoved)
}

// --- REST API delete cascade ------------------------------------------------

func (w *world) deleteRESTAPIArtifact() error {
	// Mirrors APIRepo.DeleteAPI ordering: rest_apis + artifacts cascade the rest.
	if err := w.it.exec(`DELETE FROM rest_apis WHERE uuid = ?`, w.g.apiArtifact); err != nil {
		return err
	}
	return w.it.exec(`DELETE FROM artifacts WHERE uuid = ?`, w.g.apiArtifact)
}

func (w *world) subscriptionRemoved() error {
	return w.wantCount("subscriptions", "uuid", w.g.sub, 0)
}

// --- Gateway delete cascade -------------------------------------------------

func (w *world) deleteGateway() error {
	return w.it.exec(`DELETE FROM gateways WHERE uuid = ? AND organization_uuid = ?`, w.g.gateway, w.g.org)
}

func (w *world) deploymentRemoved() error {
	return w.wantCount("deployments", "uuid", w.g.deploy, 0)
}

func (w *world) deploymentStatusRemoved() error {
	return w.wantCount("deployment_status", "deployment_uuid", w.g.deploy, 0)
}

// --- Application delete cascade ---------------------------------------------

func (w *world) deleteApplication() error {
	return w.it.exec(`DELETE FROM applications WHERE uuid = ? AND organization_uuid = ?`, w.g.app, w.g.org)
}

func (w *world) appKeyMappingRemoved() error {
	return w.wantCount("application_api_key_mappings", "api_key_id", w.g.apiKey, 0)
}

func (w *world) appArtifactMappingRemoved() error {
	return w.wantCount("application_artifact_mappings", "application_uuid", w.g.app, 0)
}

// --- Project delete cascade -------------------------------------------------

func (w *world) deleteProject() error {
	return w.it.exec(`DELETE FROM projects WHERE uuid = ?`, w.g.project)
}

func (w *world) applicationRemoved() error {
	return w.wantCount("applications", "uuid", w.g.app, 0)
}

// --- Subscription plan delete cascade ---------------------------------------

func (w *world) deleteSubscriptionAndPlan() error {
	// subscriptions.subscription_plan_uuid blocks plan deletion, so remove the
	// subscription first; the plan delete then cascades to subscription_plan_limits.
	if err := w.it.exec(`DELETE FROM subscriptions WHERE uuid = ?`, w.g.sub); err != nil {
		return err
	}
	return w.it.exec(`DELETE FROM subscription_plans WHERE uuid = ? AND organization_uuid = ?`, w.g.plan, w.g.org)
}

func (w *world) subscriptionPlanLimitRemoved() error {
	return w.wantCount("subscription_plan_limits", "uuid", w.g.planLimit, 0)
}

// --- WebSub API delete cascade ----------------------------------------------

func (w *world) seedWebSubWithSecrets(n int) error {
	orgUUID := id()
	projectUUID := id()
	artifactUUID := id()

	steps := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO organizations (uuid, handle, name, region) VALUES (?, ?, ?, ?)`,
			[]any{orgUUID, "wsc-" + orgUUID[:8], "cascade org", "us"}},
		{`INSERT INTO projects (uuid, handle, name, organization_uuid) VALUES (?, ?, ?, ?)`,
			[]any{projectUUID, "cascade-proj", "cascade-proj", orgUUID}},
		{`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES (?, ?, ?)`,
			[]any{artifactUUID, "WebSubApi", orgUUID}},
		{`INSERT INTO websub_apis (uuid, organization_uuid, handle, name, version, project_uuid, lifecycle_status, configuration) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			[]any{artifactUUID, orgUUID, "ws-api-" + artifactUUID[:8], "ws-api", "v1.0", projectUUID, "CREATED", []byte("{}")}},
	}
	for _, s := range steps {
		if err := w.it.exec(s.query, s.args...); err != nil {
			return err
		}
	}

	handles := []string{"github-secret", "gitlab-secret"}
	for i := range n {
		handle := fmt.Sprintf("secret-%d", i)
		if i < len(handles) {
			handle = handles[i]
		}
		if err := w.it.exec(
			`INSERT INTO websub_api_hmac_secrets (uuid, artifact_uuid, handle, encrypted_secret, status) VALUES (?, ?, ?, ?, ?)`,
			id(), artifactUUID, handle, []byte(fmt.Sprintf("enc%d", i+1)), "active"); err != nil {
			return err
		}
	}

	if err := w.wantCount("websub_api_hmac_secrets", "artifact_uuid", artifactUUID, n); err != nil {
		return fmt.Errorf("precondition: %w", err)
	}
	if err := w.wantCount("websub_apis", "uuid", artifactUUID, 1); err != nil {
		return fmt.Errorf("precondition: %w", err)
	}

	w.artifactUUID = artifactUUID
	return nil
}

func (w *world) deleteWebSubArtifact() error {
	return w.it.exec(`DELETE FROM artifacts WHERE uuid = ?`, w.artifactUUID)
}

func (w *world) hmacSecretsRemoved() error {
	return w.wantCount("websub_api_hmac_secrets", "artifact_uuid", w.artifactUUID, 0)
}

func (w *world) webSubRowRemoved() error {
	return w.wantCount("websub_apis", "uuid", w.artifactUUID, 0)
}
