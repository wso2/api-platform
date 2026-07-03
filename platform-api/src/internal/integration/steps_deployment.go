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
	"bytes"
	"fmt"
	"time"

	"github.com/cucumber/godog"

	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
)

// registerDeploymentSteps wires the deployment repository scenarios. They drive
// the deployment store through the real repo methods (create with limit
// enforcement, the deployment_status upsert, the FETCH-first current lookup and
// the ROW_NUMBER ranking) so the SQL Server-specific SQL is exercised on every
// engine — not just SQLite.
func registerDeploymentSteps(ctx *godog.ScenarioContext, w *world) {
	ctx.Step(`^a REST API and a gateway exist$`, w.seedAPIAndGateway)
	ctx.Step(`^I create (\d+) deployments for the API on the gateway$`, w.createDeployments)
	ctx.Step(`^I create (\d+) deployments for the API on the gateway with a hard limit of (\d+)$`, w.createDeploymentsWithLimit)
	ctx.Step(`^the current deployment status is "([^"]*)"$`, w.currentDeploymentStatusIs)
	ctx.Step(`^the API has an active deployment$`, w.apiHasActiveDeployment)
	ctx.Step(`^the API has no active deployment$`, w.apiHasNoActiveDeployment)
	ctx.Step(`^the current deployment for the gateway is the latest one$`, w.currentDeploymentIsLatest)
	ctx.Step(`^reading the current deployment back returns its content$`, w.currentDeploymentContentRoundTrip)
	ctx.Step(`^listing deployments with state returns (\d+)$`, w.listDeploymentsWithState)
	ctx.Step(`^I undeploy the current deployment$`, w.undeployCurrent)
	ctx.Step(`^there is no current deployment for the gateway$`, w.noCurrentDeployment)
	ctx.Step(`^the gateway retains at most (\d+) deployments$`, w.gatewayRetainsAtMost)
}

func (w *world) seedAPIAndGateway() error {
	// A deployment hangs off an artifact (the REST API) and a gateway.
	apiRepo := repository.NewAPIRepo(w.it.db)
	api := &model.API{
		Handle:          "dep-api-" + id()[:8],
		Name:            "Dep API " + id()[:6],
		Version:         "v1.0",
		ProjectID:       w.projID,
		OrganizationID:  w.orgID,
		LifeCycleStatus: "CREATED",
		Configuration:   model.RestAPIConfig{Name: "Dep API", Version: "v1.0"},
	}
	if err := apiRepo.CreateAPI(api); err != nil {
		return fmt.Errorf("[%s] CreateAPI (for deployment) failed: %w", w.it.driver, err)
	}
	w.depArtifactID = api.ID

	gwRepo := repository.NewGatewayRepo(w.it.db)
	gw := &model.Gateway{
		ID:                id(),
		OrganizationID:    w.orgID,
		Name:              "dep gateway",
		Handle:            "dep-gw-" + id()[:6],
		Endpoints:         []string{"https://localhost:8443", "wss://localhost:8444"},
		FunctionalityType: "REGULAR",
		Version:           "1.0.0",
		Properties:        map[string]interface{}{"region": "us"},
		CreatedBy:         "it-user",
	}
	if err := gwRepo.Create(gw); err != nil {
		return fmt.Errorf("[%s] gateway Create (for deployment) failed: %w", w.it.driver, err)
	}
	w.depGatewayID = gw.ID
	return nil
}

// createDeploymentsN creates n deployments through CreateWithLimitEnforcement,
// each marked DEPLOYED (which makes it the current deployment). Distinct,
// increasing created_at values keep the ordering deterministic for the ranking
// and current-lookup queries.
func (w *world) createDeploymentsN(n, hardLimit int) error {
	repo := repository.NewDeploymentRepo(w.it.db, repository.NewArtifactTableRegistry())
	base := time.Now()
	for i := range n {
		deployed := model.DeploymentStatusDeployed
		updatedAt := base.Add(time.Duration(i) * time.Second)
		content := []byte(fmt.Sprintf("deployment-content-%d-%s", i, id()[:6]))
		dep := &model.Deployment{
			Name:           fmt.Sprintf("dep-%d", i),
			ArtifactID:     w.depArtifactID,
			OrganizationID: w.orgID,
			GatewayID:      w.depGatewayID,
			Content:        content,
			CreatedBy:      "it-user",
			CreatedAt:      updatedAt,
			Status:         &deployed,
			UpdatedAt:      &updatedAt,
		}
		if err := repo.CreateWithLimitEnforcement(dep, hardLimit); err != nil {
			return fmt.Errorf("[%s] CreateWithLimitEnforcement %d failed: %w", w.it.driver, i, err)
		}
		if dep.DeploymentID == "" {
			return fmt.Errorf("[%s] CreateWithLimitEnforcement %d did not populate the deployment ID", w.it.driver, i)
		}
		w.depIDs = append(w.depIDs, dep.DeploymentID)
		w.lastDepContent = content
	}
	return nil
}

func (w *world) createDeployments(n int) error {
	w.depIDs = w.depIDs[:0]
	return w.createDeploymentsN(n, 10)
}

func (w *world) createDeploymentsWithLimit(n, limit int) error {
	w.depIDs = w.depIDs[:0]
	return w.createDeploymentsN(n, limit)
}

func (w *world) currentDeploymentStatusIs(status string) error {
	repo := repository.NewDeploymentRepo(w.it.db, repository.NewArtifactTableRegistry())
	_, got, _, err := repo.GetStatus(w.depArtifactID, w.orgID, w.depGatewayID)
	if err != nil {
		return fmt.Errorf("[%s] GetStatus failed: %w", w.it.driver, err)
	}
	if string(got) != status {
		return fmt.Errorf("[%s] current deployment status: want %q, got %q", w.it.driver, status, got)
	}
	return nil
}

func (w *world) apiHasActiveDeployment() error {
	return w.activeDeploymentIs(true)
}

func (w *world) apiHasNoActiveDeployment() error {
	return w.activeDeploymentIs(false)
}

func (w *world) activeDeploymentIs(want bool) error {
	repo := repository.NewDeploymentRepo(w.it.db, repository.NewArtifactTableRegistry())
	active, err := repo.HasActiveDeployment(w.depArtifactID, w.orgID)
	if err != nil {
		return fmt.Errorf("[%s] HasActiveDeployment failed: %w", w.it.driver, err)
	}
	if active != want {
		return fmt.Errorf("[%s] HasActiveDeployment: want %v, got %v", w.it.driver, want, active)
	}
	return nil
}

func (w *world) currentDeploymentIsLatest() error {
	repo := repository.NewDeploymentRepo(w.it.db, repository.NewArtifactTableRegistry())
	got, err := repo.GetCurrentByGateway(w.depArtifactID, w.depGatewayID, w.orgID)
	if err != nil {
		return fmt.Errorf("[%s] GetCurrentByGateway failed: %w", w.it.driver, err)
	}
	want := w.depIDs[len(w.depIDs)-1]
	if got == nil || got.DeploymentID != want {
		return fmt.Errorf("[%s] GetCurrentByGateway: want latest %s, got %+v", w.it.driver, want, got)
	}
	return nil
}

func (w *world) currentDeploymentContentRoundTrip() error {
	repo := repository.NewDeploymentRepo(w.it.db, repository.NewArtifactTableRegistry())
	current := w.depIDs[len(w.depIDs)-1]
	got, err := repo.GetWithContent(current, w.depArtifactID, w.orgID)
	if err != nil {
		return fmt.Errorf("[%s] GetWithContent failed: %w", w.it.driver, err)
	}
	if got == nil || !bytes.Equal(got.Content, w.lastDepContent) {
		return fmt.Errorf("[%s] deployment content did not round-trip: want %q", w.it.driver, w.lastDepContent)
	}
	return nil
}

func (w *world) listDeploymentsWithState(want int) error {
	repo := repository.NewDeploymentRepo(w.it.db, repository.NewArtifactTableRegistry())
	list, err := repo.GetDeploymentsWithState(w.depArtifactID, w.orgID, nil, nil, 5)
	if err != nil {
		return fmt.Errorf("[%s] GetDeploymentsWithState failed: %w", w.it.driver, err)
	}
	if len(list) != want {
		return fmt.Errorf("[%s] GetDeploymentsWithState: want %d, got %d", w.it.driver, want, len(list))
	}
	return nil
}

func (w *world) undeployCurrent() error {
	repo := repository.NewDeploymentRepo(w.it.db, repository.NewArtifactTableRegistry())
	current, _, _, err := repo.GetStatus(w.depArtifactID, w.orgID, w.depGatewayID)
	if err != nil {
		return fmt.Errorf("[%s] GetStatus (before undeploy) failed: %w", w.it.driver, err)
	}
	if _, err := repo.SetCurrentWithDetails(w.depArtifactID, w.orgID, w.depGatewayID, current,
		model.DeploymentStatusUndeployed, "", nil, "undeployed by integration test"); err != nil {
		return fmt.Errorf("[%s] SetCurrentWithDetails (undeploy) failed: %w", w.it.driver, err)
	}
	return nil
}

func (w *world) noCurrentDeployment() error {
	repo := repository.NewDeploymentRepo(w.it.db, repository.NewArtifactTableRegistry())
	got, err := repo.GetCurrentByGateway(w.depArtifactID, w.depGatewayID, w.orgID)
	if err != nil {
		return fmt.Errorf("[%s] GetCurrentByGateway (after undeploy) failed: %w", w.it.driver, err)
	}
	if got != nil {
		return fmt.Errorf("[%s] GetCurrentByGateway after undeploy: want nil, got %+v", w.it.driver, got)
	}
	return nil
}

func (w *world) gatewayRetainsAtMost(max int) error {
	got, err := w.it.count("deployments", "gateway_uuid", w.depGatewayID)
	if err != nil {
		return err
	}
	if got < 1 || got > max {
		return fmt.Errorf("[%s] retained deployments: want 1..%d, got %d", w.it.driver, max, got)
	}
	return nil
}
