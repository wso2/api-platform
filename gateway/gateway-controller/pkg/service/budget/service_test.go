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

package budget

import (
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

const validBudget = `{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "Budget",
  "metadata": {"name": "research-team-monthly"},
  "spec": {
    "limit": {"amount": "500.00", "period": "month", "action": "block", "blockAt": 95},
    "resource": {"type": "llmProvider", "allocation": "shared", "id": "openai"},
    "consumers": [{"type": "attribute", "allocation": "shared", "expression": "'research' in request.Auth.Claims[\"groups\"]"}]
  }
}`

func newTestService(t *testing.T, pushEnabled bool) (*Service, storage.Storage) {
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
	return NewService(db, logger, pushEnabled), db
}

func TestCreate_StoresBudgetWithDefaults(t *testing.T) {
	svc, db := newTestService(t, true)

	cfg, err := svc.Create(CreateParams{Body: []byte(validBudget)})
	require.NoError(t, err)

	assert.Equal(t, models.KindBudget, cfg.Kind)
	assert.Equal(t, "research-team-monthly", cfg.Handle)
	assert.Equal(t, cfg.Handle, cfg.DisplayName)
	assert.Equal(t, models.OriginGatewayAPI, cfg.Origin)
	assert.Equal(t, models.CPSyncStatusPending, cfg.CPSyncStatus)
	assert.Equal(t, models.StateDeployed, cfg.DesiredState)

	stored, err := db.GetConfig(cfg.UUID)
	require.NoError(t, err)
	budget, ok := stored.SourceConfiguration.(api.BudgetRequest)
	require.True(t, ok, "stored configuration is %T", stored.SourceConfiguration)
	assert.Equal(t, "500.00", budget.Spec.Limit.Amount)
	assert.Equal(t, 95, *budget.Spec.Limit.BlockAt)
	// Defaults come from the spec.
	assert.Equal(t, api.BudgetStatusActive, *budget.Spec.Status)
	assert.Equal(t, api.BudgetCurrencyUSD, *budget.Spec.Limit.Currency)
	assert.Equal(t, models.OriginGatewayAPI, stored.Origin)
}

func TestCreate_AcceptsYAML(t *testing.T) {
	svc, _ := newTestService(t, false)
	body := `apiVersion: gateway.api-platform.wso2.com/v1
kind: Budget
metadata:
  name: org-daily
spec:
  limit:
    amount: "50"
    period: day
    action: allow
  resource:
    type: llmProvider
    allocation: separate
`
	cfg, err := svc.Create(CreateParams{Body: []byte(body)})
	require.NoError(t, err)
	budget := cfg.SourceConfiguration.(api.BudgetRequest)
	assert.Equal(t, 100, *budget.Spec.Limit.BlockAt, "blockAt default from the spec")
	assert.Equal(t, api.BudgetResourceAllocationSeparate, budget.Spec.Resource.Allocation)
	assert.Empty(t, cfg.CPSyncStatus, "no push-up when deployment sync is off")
}

func TestCreate_DuplicateNameConflicts(t *testing.T) {
	svc, _ := newTestService(t, false)
	_, err := svc.Create(CreateParams{Body: []byte(validBudget)})
	require.NoError(t, err)

	_, err = svc.Create(CreateParams{Body: []byte(validBudget)})
	require.Error(t, err)
	assert.True(t, storage.IsConflictError(err), "got %v", err)
	assert.Contains(t, err.Error(), `"research-team-monthly"`)
}

func TestCreate_RejectsInvalidBudgets(t *testing.T) {
	const consumer = `{"type": "attribute", "allocation": "shared", "expression": "'research' in request.Auth.Claims[\"groups\"]"}`
	cases := []struct {
		name, from, to, field string
	}{
		{"bad name", `"research-team-monthly"`, `"Research Team"`, "metadata.name"},
		{"zero amount", `"500.00"`, `"0.00"`, "spec.limit.amount"},
		{"three decimals", `"500.00"`, `"5.001"`, "spec.limit.amount"},
		{"block at zero", `"blockAt": 95`, `"blockAt": 0`, "spec.limit.blockAt"},
		{"block at over 100", `"blockAt": 95`, `"blockAt": 101`, "spec.limit.blockAt"},
		{"bad period", `"period": "month"`, `"period": "year"`, "spec.limit.period"},
		{"bad action", `"action": "block"`, `"action": "warn"`, "spec.limit.action"},
		{"other currency", `"blockAt": 95`, `"blockAt": 95, "currency": "EUR"`, "spec.limit.currency"},
		{"wrong kind", `"kind": "Budget"`, `"kind": "RestApi"`, "kind"},
		{"no resource", `"resource": {"type": "llmProvider", "allocation": "shared", "id": "openai"},`, ``, "spec.resource"},
		{"no resource allocation", `"allocation": "shared", "id": "openai"`, `"id": "openai"`, "spec.resource.allocation"},
		{"separate resource with id", `"allocation": "shared", "id": "openai"`, `"allocation": "separate", "id": "openai"`, "spec.resource.id"},
		{"environments", `"resource":`, `"environments": {"ids": ["production"]}, "resource":`, "spec"},
		{"agent consumer", consumer, `{"type": "agent", "allocation": "shared", "expression": "true"}`, "spec.consumers[0].type"},
		{"consumer without allocation", consumer, `{"type": "attribute", "expression": "true"}`, "spec.consumers[0].allocation"},
		{"consumer without expression", consumer, `{"type": "attribute", "allocation": "shared"}`, "spec.consumers[0].expression"},
		{"consumer with id", consumer, `{"type": "attribute", "allocation": "shared", "expression": "true", "id": "x"}`, "spec.consumers[0]"},
		{"exemption with allocation", `"consumers":`, `"exemptions": [{"type": "attribute", "allocation": "shared", "expression": "true"}], "consumers":`, "spec.exemptions[0]"},
		{"unknown field", `"limit":`, `"owner": "me", "limit":`, "spec"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Contains(t, validBudget, tc.from, "test case does not apply")
			svc, _ := newTestService(t, false)
			body := strings.Replace(validBudget, tc.from, tc.to, 1)

			_, err := svc.Create(CreateParams{Body: []byte(body)})

			var invalid *ValidationError
			require.True(t, errors.As(err, &invalid), "got %v", err)
			fields := make([]string, 0, len(invalid.Errors))
			for _, fe := range invalid.Errors {
				fields = append(fields, fe.Field)
			}
			assert.Contains(t, fields, tc.field, "errors: %v", invalid.Errors)
		})
	}
}

func TestCreate_ReportsEveryError(t *testing.T) {
	svc, _ := newTestService(t, false)
	body := strings.NewReplacer(`"blockAt": 95`, `"blockAt": 0`, `"period": "month"`, `"period": "year"`).Replace(validBudget)

	_, err := svc.Create(CreateParams{Body: []byte(body)})

	var invalid *ValidationError
	require.True(t, errors.As(err, &invalid), "got %v", err)
	assert.Len(t, invalid.Errors, 2, "errors: %v", invalid.Errors)
}

func TestCreate_RejectsUnparseableBody(t *testing.T) {
	svc, _ := newTestService(t, false)
	_, err := svc.Create(CreateParams{Body: []byte(`{"kind": `)})
	var invalid *ValidationError
	require.True(t, errors.As(err, &invalid), "got %v", err)
}

func TestGetAndList(t *testing.T) {
	svc, _ := newTestService(t, false)
	_, err := svc.Create(CreateParams{Body: []byte(validBudget)})
	require.NoError(t, err)
	_, err = svc.Create(CreateParams{Body: []byte(strings.Replace(validBudget, "research-team-monthly", "org-daily", 1))})
	require.NoError(t, err)

	cfg, err := svc.Get("research-team-monthly")
	require.NoError(t, err)
	assert.Equal(t, "research-team-monthly", cfg.Handle)

	_, err = svc.Get("missing")
	assert.True(t, errors.Is(err, ErrNotFound), "got %v", err)

	all, err := svc.List()
	require.NoError(t, err)
	assert.Len(t, all, 2)
}

func TestUpdate_ChangesEditableFields(t *testing.T) {
	svc, _ := newTestService(t, true)
	_, err := svc.Create(CreateParams{Body: []byte(validBudget)})
	require.NoError(t, err)

	body := strings.NewReplacer(
		`"amount": "500.00"`, `"amount": "750.00"`,
		`"action": "block"`, `"action": "allow"`,
		`"blockAt": 95`, `"blockAt": 90`,
		`"limit":`, `"status": "inactive", "exemptions": [{"type": "attribute", "expression": "request.Auth.Subject == 'alice'"}], "limit":`,
	).Replace(validBudget)

	updated, err := svc.Update(UpdateParams{Name: "research-team-monthly", Body: []byte(body)})
	require.NoError(t, err)
	assert.Equal(t, models.CPSyncStatusPending, updated.CPSyncStatus)

	got, err := svc.Get("research-team-monthly")
	require.NoError(t, err)
	b := got.SourceConfiguration.(api.BudgetRequest)
	assert.Equal(t, "750.00", b.Spec.Limit.Amount)
	assert.Equal(t, api.BudgetActionAllow, b.Spec.Limit.Action)
	assert.Equal(t, 90, *b.Spec.Limit.BlockAt)
	assert.Equal(t, api.BudgetStatusInactive, *b.Spec.Status)
	require.NotNil(t, b.Spec.Exemptions)
	assert.Len(t, *b.Spec.Exemptions, 1)
}

func TestUpdate_RejectsFixedFieldChanges(t *testing.T) {
	cases := []struct {
		name, from, to, field string
	}{
		{"period", `"period": "month"`, `"period": "week"`, "spec.limit.period"},
		{"resource", `"id": "openai"`, `"id": "anthropic"`, "spec.resource"},
		{"consumers", `'research'`, `'finance'`, "spec.consumers"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newTestService(t, false)
			_, err := svc.Create(CreateParams{Body: []byte(validBudget)})
			require.NoError(t, err)

			_, err = svc.Update(UpdateParams{Name: "research-team-monthly", Body: []byte(strings.Replace(validBudget, tc.from, tc.to, 1))})

			var invalid *ValidationError
			require.True(t, errors.As(err, &invalid), "got %v", err)
			require.Len(t, invalid.Errors, 1)
			assert.Equal(t, tc.field, invalid.Errors[0].Field)
		})
	}
}

func TestUpdate_NameMustMatchAndBudgetMustExist(t *testing.T) {
	svc, _ := newTestService(t, false)
	_, err := svc.Create(CreateParams{Body: []byte(validBudget)})
	require.NoError(t, err)

	_, err = svc.Update(UpdateParams{Name: "other", Body: []byte(validBudget)})
	var invalid *ValidationError
	require.True(t, errors.As(err, &invalid), "got %v", err)
	assert.Equal(t, "metadata.name", invalid.Errors[0].Field)

	body := strings.Replace(validBudget, "research-team-monthly", "missing", 1)
	_, err = svc.Update(UpdateParams{Name: "missing", Body: []byte(body)})
	assert.True(t, errors.Is(err, ErrNotFound), "got %v", err)
}

func TestControlPlaneBudgetsAreReadOnly(t *testing.T) {
	svc, db := newTestService(t, false)
	created, err := svc.Create(CreateParams{Body: []byte(validBudget)})
	require.NoError(t, err)
	// Store a second budget as if it came from the control plane.
	cp := *created
	cp.UUID = "0193f1c2-7a4b-7c3d-9e8f-123456789abc"
	cp.Handle, cp.DisplayName = "from-control-plane", "from-control-plane"
	b := cp.SourceConfiguration.(api.BudgetRequest)
	b.Metadata.Name = cp.Handle
	cp.SourceConfiguration, cp.Configuration = b, b
	cp.Origin, cp.CPSyncStatus = models.OriginControlPlane, ""
	cp.CreatedAt, cp.UpdatedAt = time.Now(), time.Now()
	require.NoError(t, db.SaveConfig(&cp))

	body := strings.Replace(validBudget, "research-team-monthly", "from-control-plane", 1)
	_, err = svc.Update(UpdateParams{Name: "from-control-plane", Body: []byte(body)})
	assert.True(t, errors.Is(err, ErrReadOnly), "update: got %v", err)

	err = svc.Delete("from-control-plane", nil)
	assert.True(t, errors.Is(err, ErrReadOnly), "delete: got %v", err)
}

func TestDelete(t *testing.T) {
	svc, db := newTestService(t, false)
	created, err := svc.Create(CreateParams{Body: []byte(validBudget)})
	require.NoError(t, err)

	require.NoError(t, svc.Delete("research-team-monthly", nil))

	_, err = svc.Get("research-team-monthly")
	assert.True(t, errors.Is(err, ErrNotFound), "got %v", err)
	_, err = db.GetConfig(created.UUID)
	assert.True(t, storage.IsNotFoundError(err), "artifact row should be gone: %v", err)

	err = svc.Delete("research-team-monthly", nil)
	assert.True(t, errors.Is(err, ErrNotFound), "got %v", err)
}
