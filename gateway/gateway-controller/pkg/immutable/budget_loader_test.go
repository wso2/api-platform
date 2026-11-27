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

package immutable

import (
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/lazyresourcexds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/budget"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

const budgetArtifact = `apiVersion: gateway.api-platform.wso2.com/v1
kind: Budget
metadata:
  name: org-daily
spec:
  limit:
    amount: "50.00"
    period: day
    action: allow
`

func TestLoadArtifacts_AppliesBudget(t *testing.T) {
	dir := t.TempDir()
	writeArtifact(t, dir, "org-daily.yaml", budgetArtifact)
	gw, db := newAgentOnlyGW(t, dir)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := storage.NewLazyResourceStore(logger)
	resources := lazyresourcexds.NewLazyResourceStateManager(store, lazyresourcexds.NewLazyResourceSnapshotManager(store, logger), logger)
	gw.SetBudgetService(budget.NewService(db, utils.NewBudgetPublisher(db, resources), discardEventHub{}, "test-gateway", logger, false))

	require.NoError(t, gw.LoadArtifacts(logger))

	cfg, err := db.GetConfigByKindAndHandle(models.KindBudget, "org-daily")
	require.NoError(t, err)
	assert.Equal(t, models.OriginGatewayAPI, cfg.Origin)
}

func TestLoadArtifacts_BudgetWithBudgetsDisabledFails(t *testing.T) {
	dir := t.TempDir()
	writeArtifact(t, dir, "org-daily.yaml", budgetArtifact)
	gw, _ := newAgentOnlyGW(t, dir)

	err := gw.LoadArtifacts(slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "budgets are not enabled")
}

// A Budget file copied from the control plane keeps its displayName, description and
// warnings; the loader applies it and leaves them out.
func TestLoadArtifacts_BudgetIgnoresControlPlaneOnlyFields(t *testing.T) {
	dir := t.TempDir()
	writeArtifact(t, dir, "org-daily.yaml", budgetArtifact+`  displayName: Org daily
  description: All LLM spend in the organization
  warnings:
    thresholds: [80, 100]
    channels: [finance-webhook]
`)
	gw, db := newAgentOnlyGW(t, dir)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := storage.NewLazyResourceStore(logger)
	resources := lazyresourcexds.NewLazyResourceStateManager(store, lazyresourcexds.NewLazyResourceSnapshotManager(store, logger), logger)
	gw.SetBudgetService(budget.NewService(db, utils.NewBudgetPublisher(db, resources), discardEventHub{}, "test-gateway", logger, false))

	require.NoError(t, gw.LoadArtifacts(logger))

	cfg, err := db.GetConfigByKindAndHandle(models.KindBudget, "org-daily")
	require.NoError(t, err)
	stored, err := json.Marshal(cfg.SourceConfiguration)
	require.NoError(t, err)
	assert.NotContains(t, string(stored), `"displayName"`)
	assert.NotContains(t, string(stored), `"description"`)
	assert.NotContains(t, string(stored), `"warnings"`)
}

// A Budget file may list several projects: they are alternatives, and the loader keeps
// every entry in order.
func TestLoadArtifacts_BudgetWithSeveralFiltersOfOneType(t *testing.T) {
	dir := t.TempDir()
	writeArtifact(t, dir, "two-projects.yaml", `apiVersion: gateway.api-platform.wso2.com/v1
kind: Budget
metadata:
  name: two-projects
spec:
  limit:
    amount: "50.00"
    period: day
    action: allow
  filters:
    - type: project
      id: project-a
    - type: project
      id: project-b
`)
	gw, db := newAgentOnlyGW(t, dir)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := storage.NewLazyResourceStore(logger)
	resources := lazyresourcexds.NewLazyResourceStateManager(store, lazyresourcexds.NewLazyResourceSnapshotManager(store, logger), logger)
	gw.SetBudgetService(budget.NewService(db, utils.NewBudgetPublisher(db, resources), discardEventHub{}, "test-gateway", logger, false))

	require.NoError(t, gw.LoadArtifacts(logger))

	cfg, err := db.GetConfigByKindAndHandle(models.KindBudget, "two-projects")
	require.NoError(t, err)
	stored, err := json.Marshal(cfg.SourceConfiguration)
	require.NoError(t, err)
	assert.Contains(t, string(stored), `"id":"project-a"`)
	assert.Contains(t, string(stored), `"id":"project-b"`)
}

// The same project listed twice is refused, like in the management API.
func TestLoadArtifacts_BudgetWithTheSameFilterTwiceFails(t *testing.T) {
	dir := t.TempDir()
	writeArtifact(t, dir, "twice.yaml", `apiVersion: gateway.api-platform.wso2.com/v1
kind: Budget
metadata:
  name: twice
spec:
  limit:
    amount: "50.00"
    period: day
    action: allow
  filters:
    - type: project
      id: project-a
    - type: project
      id: project-a
`)
	gw, db := newAgentOnlyGW(t, dir)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := storage.NewLazyResourceStore(logger)
	resources := lazyresourcexds.NewLazyResourceStateManager(store, lazyresourcexds.NewLazyResourceSnapshotManager(store, logger), logger)
	gw.SetBudgetService(budget.NewService(db, utils.NewBudgetPublisher(db, resources), discardEventHub{}, "test-gateway", logger, false))

	err := gw.LoadArtifacts(logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "spec.filters[1]")
}
