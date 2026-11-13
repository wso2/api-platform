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
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
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
    "resource": {"type": "llmProvider", "allocation": "shared"},
    "consumers": [{"type": "attribute", "allocation": "shared", "expression": "'research' in request.Auth.Claims[\"groups\"]"}]
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
		budgetPublisher: publisher,
		budgetService:   budget.NewService(db, publisher, &mockEventHub{}, "test-gateway", logger, false),
	}
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
	assert.Equal(t, api.ResourceStatusStateDeployed, *got.Status.State)
}

func TestCreateBudget_ValidationErrorsListFields(t *testing.T) {
	server := newBudgetTestServer(t)
	body := strings.Replace(testBudgetBody, `"type": "attribute"`, `"type": "agent"`, 1)

	rec := postBudget(server, body)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	var got api.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.NotNil(t, got.Errors)
	require.NotEmpty(t, *got.Errors)
	assert.Equal(t, "spec.consumers[0].type", *(*got.Errors)[0].Field)
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
