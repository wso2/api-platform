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

package handlers

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/budgetspend"
	"github.com/wso2/api-platform/gateway/common/counterproto"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/counters"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/budget"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

const testBudgetBody = `{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "Budget",
  "metadata": {"name": "research-team-monthly"},
  "spec": {
    "limit": {"amount": "500.00", "period": "month", "action": "block"},
    "filters": [{"type": "attribute", "expression": "'research' in request.Auth.Claims[\"groups\"]"}]
  }
}`

func newBudgetTestServer(t *testing.T) *APIServer {
	t.Helper()
	metrics.Init()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := storage.NewStorage(storage.BackendConfig{
		Type:       "sqlite",
		SQLitePath: filepath.Join(t.TempDir(), "gateway.db"),
		GatewayID:  "test-gateway",
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	publisher := utils.NewBudgetPublisher(db, newTestLazyResourceManager())
	return &APIServer{
		db: db, logger: logger, store: storage.NewConfigStore(),
		budgetPublisher: publisher, budgetsEnabled: true,
		budgetService: budget.NewService(db, publisher, &mockEventHub{}, "test-gateway", logger, false),
	}
}

// With budgets off (ai_cost_management.budgets.enabled), the budget API is not there: every
// operation answers 404 and nothing is stored.
func TestBudgetAPIAnswers404WhenBudgetsAreDisabled(t *testing.T) {
	server := newBudgetTestServer(t)
	server.budgetsEnabled = false
	rec := postBudget(server, `{"apiVersion":"gateway.api-platform.wso2.com/v1","kind":"Budget","metadata":{"name":"x"},"spec":{"limit":{"amount":"1","period":"day","action":"allow"}}}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "ai_cost_management.budgets.enabled")

	list := httptest.NewRecorder()
	server.ListBudgets(list, httptest.NewRequest(http.MethodGet, "/budgets", nil))
	assert.Equal(t, http.StatusNotFound, list.Code)
	stored, err := server.budgetService.List()
	require.NoError(t, err)
	assert.Empty(t, stored)
}

func postBudget(server *APIServer, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/budgets", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.CreateBudget(rec, req)
	return rec
}

func TestCreateBudget_Created(t *testing.T) {
	server := newBudgetTestServer(t)

	rec := postBudget(server, testBudgetBody)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var got api.Budget
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "research-team-monthly", got.Metadata.Name)
	assert.Equal(t, api.BudgetKindBudget, got.Kind)
	assert.Equal(t, 100, *got.Spec.Limit.BlockAt)
	require.NotNil(t, got.Status)
	assert.Equal(t, "research-team-monthly", *got.Status.Id)
	assert.Equal(t, api.BudgetStatusStateDeployed, *got.Status.State)
}

func TestCreateBudget_ValidationErrorsListFields(t *testing.T) {
	server := newBudgetTestServer(t)
	body := strings.Replace(testBudgetBody, `"type": "attribute"`, `"type": "team"`, 1)

	rec := postBudget(server, body)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	var got api.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.NotNil(t, got.Errors)
	require.NotEmpty(t, *got.Errors)
	assert.Equal(t, "spec.filters[0].type", *(*got.Errors)[0].Field)
	assert.Contains(t, *(*got.Errors)[0].Message, "allowed values")
}

func budgetRequest(server *APIServer, method, id, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/budgets/"+id, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	switch method {
	case http.MethodGet:
		server.GetBudget(rec, req, id)
	case http.MethodPut:
		server.UpdateBudget(rec, req, id)
	case http.MethodDelete:
		server.DeleteBudget(rec, req, id)
	}
	return rec
}

func TestBudgetCRUD(t *testing.T) {
	server := newBudgetTestServer(t)
	require.Equal(t, http.StatusCreated, postBudget(server, testBudgetBody).Code)

	// List
	req := httptest.NewRequest(http.MethodGet, "/budgets", nil)
	rec := httptest.NewRecorder()
	server.ListBudgets(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var list struct {
		Count   int          `json:"count"`
		Budgets []api.Budget `json:"budgets"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	assert.Equal(t, 1, list.Count)

	// Get
	rec = budgetRequest(server, http.MethodGet, "research-team-monthly", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, http.StatusNotFound, budgetRequest(server, http.MethodGet, "missing", "").Code)

	// Update: an editable field changes, a fixed one is refused.
	rec = budgetRequest(server, http.MethodPut, "research-team-monthly", strings.Replace(testBudgetBody, `"500.00"`, `"600.00"`, 1))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var got api.Budget
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "600.00", got.Spec.Limit.Amount)

	rec = budgetRequest(server, http.MethodPut, "research-team-monthly", strings.Replace(testBudgetBody, `"period": "month"`, `"period": "day"`, 1))
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "spec.limit.period")

	// Delete
	assert.Equal(t, http.StatusOK, budgetRequest(server, http.MethodDelete, "research-team-monthly", "").Code)
	assert.Equal(t, http.StatusNotFound, budgetRequest(server, http.MethodDelete, "research-team-monthly", "").Code)
}

func TestCreateBudget_DuplicateNameConflicts(t *testing.T) {
	server := newBudgetTestServer(t)
	require.Equal(t, http.StatusCreated, postBudget(server, testBudgetBody).Code)

	rec := postBudget(server, testBudgetBody)

	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
}

func TestConfigDumpListsBudgets(t *testing.T) {
	server := newBudgetTestServer(t)
	require.Equal(t, http.StatusCreated, postBudget(server, testBudgetBody).Code)
	paused := strings.NewReplacer(`"research-team-monthly"`, `"paused-daily"`, `"limit":`, `"status": "inactive", "limit":`).Replace(testBudgetBody)
	require.Equal(t, http.StatusCreated, postBudget(server, paused).Code)

	dump, err := server.BuildConfigDumpResponse(server.logger)

	require.NoError(t, err)
	require.NotNil(t, dump.Budgets)
	budgets := *dump.Budgets
	require.Len(t, budgets, 2)
	assert.Equal(t, 2, *dump.Statistics.TotalBudgets)
	// Name order; only the active budget is enforced.
	assert.Equal(t, "paused-daily", (*budgets[0].Configuration)["metadata"].(map[string]interface{})["name"])
	assert.False(t, *budgets[0].Metadata.Enforced)
	assert.Equal(t, "research-team-monthly", (*budgets[1].Configuration)["metadata"].(map[string]interface{})["name"])
	assert.True(t, *budgets[1].Metadata.Enforced)
	assert.Equal(t, "gateway_api", *budgets[1].Metadata.Origin)
	assert.Equal(t, "500.00", (*budgets[1].Configuration)["spec"].(map[string]interface{})["limit"].(map[string]interface{})["amount"])
	assert.NotNil(t, budgets[1].Id)
}

// fakeLabels names allowances as the usage reporter does.
type fakeLabels struct {
	byBudget map[string]map[string]map[string]string
}

func (f *fakeLabels) Labels(b string) map[string]map[string]string { return f.byBudget[b] }
func (f *fakeLabels) Forget(b string)                              { delete(f.byBudget, b) }

// The budget shows its spend this period, its usage lists each allowance highest first,
// and deleting it clears its spend, so a budget made again under its name starts at $0.
func TestBudgetSpendIsShownAndClearedOnDelete(t *testing.T) {
	server := newBudgetTestServer(t)
	store := counters.NewMemory(0)
	prefix := budgetspend.CounterIDPrefix("research-team-monthly")
	labels := &fakeLabels{byBudget: map[string]map[string]map[string]string{
		"research-team-monthly": {prefix + "a": {"attribute": "alice@wso2.com"}},
	}}
	server.SetBudgetSpend(store, labels)
	require.Equal(t, http.StatusCreated, postBudget(server, testBudgetBody).Code)

	month := budgetspend.PeriodStart("month", time.Now())
	adds := []counterproto.Add{
		{Key: counterproto.KeyFor("budget", prefix+"a", month), Delta: 1.5},
		{Key: counterproto.KeyFor("budget", prefix+"b", month), Delta: 2.25},
		{Key: counterproto.KeyFor("budget-unpriced", prefix+"b", month), Delta: 3},
		{Key: counterproto.KeyFor("budget", prefix+"a", month.AddDate(0, -1, 0)), Delta: 100}, // last month
	}
	_, err := store.Sync(context.Background(), "engine-1", "r1", adds, nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	server.GetBudget(rec, httptest.NewRequest(http.MethodGet, "/budgets/research-team-monthly", nil), "research-team-monthly")
	require.Equal(t, http.StatusOK, rec.Code)
	var got api.Budget
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.NotNil(t, got.Status.CurrentPeriod)
	assert.Equal(t, "3.75", got.Status.CurrentPeriod.Spent)
	assert.Equal(t, int64(3), got.Status.CurrentPeriod.UnpricedRequests)
	assert.Equal(t, month.Format(time.DateOnly), got.Status.CurrentPeriod.PeriodStart.Format(time.DateOnly))

	limit := 1
	rec = httptest.NewRecorder()
	server.GetBudgetUsage(rec, httptest.NewRequest(http.MethodGet, "/budgets/research-team-monthly/budget-usage", nil), "research-team-monthly", api.GetBudgetUsageParams{Limit: &limit})
	require.Equal(t, http.StatusOK, rec.Code)
	var page api.BudgetUsageListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
	assert.Equal(t, 2, page.Total)
	require.Len(t, page.Allowances, 1)
	assert.Equal(t, "2.25", page.Allowances[0].Spent, "highest spend first")
	assert.Empty(t, page.Allowances[0].Allowance, "values not known on this gateway")

	bad := 101
	rec = httptest.NewRecorder()
	server.GetBudgetUsage(rec, httptest.NewRequest(http.MethodGet, "/", nil), "research-team-monthly", api.GetBudgetUsageParams{Limit: &bad})
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = httptest.NewRecorder()
	server.DeleteBudget(rec, httptest.NewRequest(http.MethodDelete, "/budgets/research-team-monthly", nil), "research-team-monthly")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	left, err := store.Totals(context.Background(), "budget", prefix)
	require.NoError(t, err)
	assert.Empty(t, left, "the deleted budget's spend is cleared")
	assert.Empty(t, labels.byBudget, "and its allowance names")

	require.Equal(t, http.StatusCreated, postBudget(server, testBudgetBody).Code)
	rec = httptest.NewRecorder()
	server.GetBudget(rec, httptest.NewRequest(http.MethodGet, "/budgets/research-team-monthly", nil), "research-team-monthly")
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "0", got.Status.CurrentPeriod.Spent, "made again: starts at $0")
}
