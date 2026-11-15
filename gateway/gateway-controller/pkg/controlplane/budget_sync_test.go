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

package controlplane

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// fakeBudgetApplier records what the client asks of the budget service.
type fakeBudgetApplier struct {
	applied []utils.ControlPlaneBudget
	deleted []string
	stored  []string
}

func (f *fakeBudgetApplier) ApplyControlPlaneBudget(b utils.ControlPlaneBudget, _ string, _ *slog.Logger) error {
	f.applied = append(f.applied, b)
	return nil
}

func (f *fakeBudgetApplier) DeleteControlPlaneBudget(id, _ string, _ *slog.Logger) error {
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fakeBudgetApplier) ControlPlaneBudgetIDs() ([]string, error) { return f.stored, nil }

func TestSyncBudgets_SavesTheListedBudgetsAndDropsTheRest(t *testing.T) {
	updated := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	client := newManifestTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/internal/v1/budgets", r.URL.Path)
		assert.Equal(t, "test-token", r.Header.Get("api-key"))
		_ = json.NewEncoder(w).Encode([]utils.ControlPlaneBudget{{
			BudgetID: "b-listed", UpdatedAt: updated,
			Budget: json.RawMessage(`{"apiVersion":"gateway.api-platform.wso2.com/v1","kind":"Budget","metadata":{"name":"org-monthly"}}`),
		}})
	})
	applier := &fakeBudgetApplier{stored: []string{"b-listed", "b-gone"}}
	client.SetBudgetApplier(applier)

	client.syncBudgets()

	require.Len(t, applier.applied, 1)
	assert.Equal(t, "b-listed", applier.applied[0].BudgetID)
	assert.True(t, applier.applied[0].UpdatedAt.Equal(updated))
	assert.JSONEq(t, `{"apiVersion":"gateway.api-platform.wso2.com/v1","kind":"Budget","metadata":{"name":"org-monthly"}}`,
		string(applier.applied[0].Budget))
	assert.Equal(t, []string{"b-gone"}, applier.deleted, "a control-plane budget no longer listed is deleted")
}

func TestSyncBudgets_KeepsBudgetsWhenTheControlPlaneFails(t *testing.T) {
	client := newManifestTLSServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	applier := &fakeBudgetApplier{stored: []string{"b-1"}}
	client.SetBudgetApplier(applier)

	client.syncBudgets()

	assert.Empty(t, applier.applied)
	assert.Empty(t, applier.deleted, "a failed fetch never deletes")
}

func TestBudgetEvents_ReachTheBudgetService(t *testing.T) {
	client := createTestClient(t)
	applier := &fakeBudgetApplier{}
	client.SetBudgetApplier(applier)

	var created map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(`{
		"type": "budget.created",
		"payload": {
			"budgetId": "b-1",
			"updatedAt": "2026-11-14T10:00:00Z",
			"budget": {"apiVersion": "gateway.api-platform.wso2.com/v1", "kind": "Budget", "metadata": {"name": "org-monthly"}}
		},
		"timestamp": "2026-11-14T10:00:01Z",
		"correlationId": "c-1"
	}`), &created))
	client.handleBudgetEvent(created)
	require.Len(t, applier.applied, 1)
	assert.Equal(t, "b-1", applier.applied[0].BudgetID)
	assert.Contains(t, string(applier.applied[0].Budget), `"org-monthly"`)

	var deleted map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(`{"type": "budget.deleted", "payload": {"budgetId": "b-1"}}`), &deleted))
	client.handleBudgetDeletedEvent(deleted)
	assert.Equal(t, []string{"b-1"}, applier.deleted)
}

func TestBudgetEvents_IgnoredWithoutBudgets(t *testing.T) {
	client := createTestClient(t)
	client.handleBudgetEvent(map[string]interface{}{"type": "budget.created"})
	client.handleBudgetDeletedEvent(map[string]interface{}{"type": "budget.deleted"})
	client.syncBudgets()
}
