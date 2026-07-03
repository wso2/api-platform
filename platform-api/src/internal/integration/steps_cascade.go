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
	ctx.Step(`^the application is retained$`, w.applicationRetained)

	ctx.Step(`^I delete the subscription and its plan$`, w.deleteSubscriptionAndPlan)
	ctx.Step(`^the subscription plan limit is removed$`, w.subscriptionPlanLimitRemoved)
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

// applicationRetained verifies the application survives a project deletion.
// Applications are intentionally decoupled from a project's lifecycle: the
// applications.project_uuid column carries no cascading foreign key, so removing
// the project leaves the application row in place (it is only cascade-deleted via
// its organization).
func (w *world) applicationRetained() error {
	return w.wantCount("applications", "uuid", w.g.app, 1)
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
