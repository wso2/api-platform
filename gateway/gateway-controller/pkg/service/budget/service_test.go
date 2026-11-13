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
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/lazyresourcexds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
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
	svc, db, _ := newTestServiceWithPath(t, pushEnabled)
	return svc, db
}

// newTestServiceWithPath also returns the SQLite file, for tests that read the rows directly.
func newTestServiceWithPath(t *testing.T, pushEnabled bool) (*Service, storage.Storage, string) {
	t.Helper()
	env := newTestEnv(t, pushEnabled, nil)
	return env.svc, env.db, env.path
}

// testEnv is a service over a fresh SQLite database that publishes to an in-memory
// lazy-resource manager.
type testEnv struct {
	svc       *Service
	db        storage.Storage
	path      string
	resources *lazyresourcexds.LazyResourceStateManager
	events    *recordingEventHub
}

// recordingEventHub records the events the service publishes for other replicas.
type recordingEventHub struct {
	events []eventhub.Event
}

func (h *recordingEventHub) Initialize() error            { return nil }
func (h *recordingEventHub) RegisterGateway(string) error { return nil }
func (h *recordingEventHub) PublishEvent(_ string, event eventhub.Event) error {
	h.events = append(h.events, event)
	return nil
}
func (h *recordingEventHub) Subscribe(string) (<-chan eventhub.Event, error) { return nil, nil }
func (h *recordingEventHub) Unsubscribe(string, <-chan eventhub.Event) error { return nil }
func (h *recordingEventHub) UnsubscribeAll(string) error                     { return nil }
func (h *recordingEventHub) CleanUpEvents() error                            { return nil }
func (h *recordingEventHub) Close() error                                    { return nil }

// newTestEnv builds a testEnv. A non-nil publisher replaces the real one.
func newTestEnv(t *testing.T, pushEnabled bool, publisher Publisher) testEnv {
	t.Helper()
	metrics.Init()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	path := filepath.Join(t.TempDir(), "gateway.db")
	db, err := storage.NewStorage(storage.BackendConfig{
		Type:       "sqlite",
		SQLitePath: path,
		GatewayID:  "test-gateway",
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	seedLLMProvider(t, db, "openai")
	store := storage.NewLazyResourceStore(logger)
	resources := lazyresourcexds.NewLazyResourceStateManager(store, lazyresourcexds.NewLazyResourceSnapshotManager(store, logger), logger)
	if publisher == nil {
		publisher = utils.NewBudgetPublisher(db, resources)
	}
	events := &recordingEventHub{}
	return testEnv{svc: NewService(db, publisher, events, "test-gateway", logger, pushEnabled), db: db, path: path, resources: resources, events: events}
}

// seedLLMProvider stores an LLM provider so budgets can name it.
func seedLLMProvider(t *testing.T, db storage.Storage, name string) {
	t.Helper()
	provider := api.LLMProviderConfiguration{
		ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProviderConfigurationKindLlmProvider,
		Metadata:   api.Metadata{Name: name},
	}
	now := time.Now()
	require.NoError(t, db.SaveConfig(&models.StoredConfig{
		UUID: "0193f1c2-0000-7000-8000-" + fmt.Sprintf("%012x", len(name)), Kind: models.KindLlmProvider,
		Handle: name, DisplayName: name, Version: "v1.0",
		Configuration: provider, SourceConfiguration: provider,
		DesiredState: models.StateDeployed, Origin: models.OriginGatewayAPI,
		CreatedAt: now, UpdatedAt: now,
	}))
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

	err = svc.Delete("from-control-plane", "", nil)
	assert.True(t, errors.Is(err, ErrReadOnly), "delete: got %v", err)
}

func TestDelete(t *testing.T) {
	svc, db := newTestService(t, false)
	created, err := svc.Create(CreateParams{Body: []byte(validBudget)})
	require.NoError(t, err)

	require.NoError(t, svc.Delete("research-team-monthly", "", nil))

	_, err = svc.Get("research-team-monthly")
	assert.True(t, errors.Is(err, ErrNotFound), "got %v", err)
	_, err = db.GetConfig(created.UUID)
	assert.True(t, storage.IsNotFoundError(err), "artifact row should be gone: %v", err)

	err = svc.Delete("research-team-monthly", "", nil)
	assert.True(t, errors.Is(err, ErrNotFound), "got %v", err)
}

func TestCreate_RejectsUnknownProvider(t *testing.T) {
	svc, _ := newTestService(t, false)

	_, err := svc.Create(CreateParams{Body: []byte(strings.Replace(validBudget, `"id": "openai"`, `"id": "mistral"`, 1))})

	var invalid *ValidationError
	require.True(t, errors.As(err, &invalid), "got %v", err)
	assert.Equal(t, "spec.resource.id", invalid.Errors[0].Field)
	assert.Contains(t, invalid.Errors[0].Message, `"mistral"`)
}

// budgetShape is one kind of budget the design offers, written as the spec a user sends.
type budgetShape struct {
	name string
	spec map[string]any
}

func attributeConsumers(n int, allocation string) []any {
	out := make([]any, n)
	for i := range out {
		out[i] = map[string]any{
			"type": "attribute", "allocation": allocation,
			"expression": fmt.Sprintf("request.Auth.Claims[\"team\"] == \"team-%d\"", i),
		}
	}
	return out
}

func attributeExemptions(n int) []any {
	out := make([]any, n)
	for i := range out {
		out[i] = map[string]any{"type": "attribute", "expression": fmt.Sprintf("request.Auth.Claims[\"sub\"] == \"svc-%d\"", i)}
	}
	return out
}

func limit(amount, period, action string) map[string]any {
	return map[string]any{"amount": amount, "period": period, "action": action}
}

func supportedBudgetShapes() []budgetShape {
	oneProvider := map[string]any{"type": "llmProvider", "allocation": "shared", "id": "openai"}
	allProviders := map[string]any{"type": "llmProvider", "allocation": "shared"}
	eachProvider := map[string]any{"type": "llmProvider", "allocation": "separate"}
	return []budgetShape{
		// Who pays: the whole gateway, a group sharing one pool, or each member on its own.
		{"gateway-wide on one provider", map[string]any{"limit": limit("1000", "month", "block"), "resource": oneProvider}},
		{"gateway-wide across all providers", map[string]any{"limit": limit("1000", "month", "block"), "resource": allProviders}},
		{"each provider its own limit", map[string]any{"limit": limit("1000", "month", "block"), "resource": eachProvider}},
		{"shared attribute consumer", map[string]any{"limit": limit("500.00", "month", "block"), "resource": oneProvider, "consumers": attributeConsumers(1, "shared")}},
		{"separate attribute consumer", map[string]any{"limit": limit("25", "day", "block"), "resource": allProviders, "consumers": attributeConsumers(1, "separate")}},
		{"separate consumer per separate provider", map[string]any{"limit": limit("25", "day", "block"), "resource": eachProvider, "consumers": attributeConsumers(1, "separate")}},
		{"several consumers, mixed allocation", map[string]any{"limit": limit("300", "week", "block"), "resource": oneProvider,
			"consumers": append(attributeConsumers(1, "shared"), attributeConsumers(2, "separate")...)}},
		{"most consumers allowed", map[string]any{"limit": limit("300", "week", "block"), "resource": allProviders, "consumers": attributeConsumers(20, "shared")}},
		// Exemptions.
		{"one exemption", map[string]any{"limit": limit("1000", "month", "block"), "resource": oneProvider, "exemptions": attributeExemptions(1)}},
		{"most exemptions allowed", map[string]any{"limit": limit("1000", "month", "block"), "resource": oneProvider, "exemptions": attributeExemptions(100)}},
		// Periods, actions, threshold, state and amount bounds.
		{"weekly", map[string]any{"limit": limit("70", "week", "block"), "resource": allProviders}},
		{"daily", map[string]any{"limit": limit("10", "day", "block"), "resource": allProviders}},
		{"warn only (allow)", map[string]any{"limit": limit("10", "day", "allow"), "resource": allProviders}},
		{"block at 1 percent", map[string]any{"limit": map[string]any{"amount": "10", "period": "day", "action": "block", "blockAt": 1}, "resource": allProviders}},
		{"block at 100 percent explicitly", map[string]any{"limit": map[string]any{"amount": "10", "period": "day", "action": "block", "blockAt": 100}, "resource": allProviders}},
		{"inactive", map[string]any{"status": "inactive", "limit": limit("10", "day", "block"), "resource": allProviders}},
		{"currency given", map[string]any{"limit": map[string]any{"amount": "10", "currency": "USD", "period": "day", "action": "block"}, "resource": allProviders}},
		{"smallest amount", map[string]any{"limit": limit("0.01", "day", "block"), "resource": allProviders}},
		{"largest amount", map[string]any{"limit": limit("999999999999.99", "month", "block"), "resource": allProviders}},
		{"everything at once", map[string]any{"status": "active",
			"limit":     map[string]any{"amount": "500.00", "currency": "USD", "period": "month", "action": "block", "blockAt": 95},
			"resource":  oneProvider,
			"consumers": attributeConsumers(3, "separate"), "exemptions": attributeExemptions(2)}},
	}
}

// withDefaults adds the values the spec fills in when a request leaves them out.
func withDefaults(spec map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range spec {
		out[k] = v
	}
	if _, ok := out["status"]; !ok {
		out["status"] = "active"
	}
	l := map[string]any{}
	for k, v := range out["limit"].(map[string]any) {
		l[k] = v
	}
	if _, ok := l["currency"]; !ok {
		l["currency"] = "USD"
	}
	if _, ok := l["blockAt"]; !ok {
		l["blockAt"] = 100
	}
	out["limit"] = l
	return out
}

// normalise round-trips a value through JSON so numbers and slices compare by value.
func normalise(t *testing.T, v any) any {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	var out any
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

type artifactRow struct {
	kind, handle, displayName, version, desiredState, origin string
	cpSyncStatus                                             sql.NullString
	updatedAt                                                time.Time
}

func readRows(t *testing.T, raw *sql.DB, uuid string) (*artifactRow, string, bool) {
	t.Helper()
	var row artifactRow
	err := raw.QueryRow(`SELECT kind, handle, display_name, version, desired_state, origin, cp_sync_status, updated_at
		FROM artifacts WHERE uuid = ?`, uuid).Scan(&row.kind, &row.handle, &row.displayName, &row.version,
		&row.desiredState, &row.origin, &row.cpSyncStatus, &row.updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		var budgets int
		require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM budgets WHERE uuid = ?`, uuid).Scan(&budgets))
		assert.Zero(t, budgets, "budgets row left behind after its artifacts row was deleted")
		return nil, "", false
	}
	require.NoError(t, err)
	var configuration string
	require.NoError(t, raw.QueryRow(`SELECT configuration FROM budgets WHERE uuid = ? AND gateway_id = 'test-gateway'`, uuid).Scan(&configuration))
	return &row, configuration, true
}

// TestBudgetShapes_RoundTripThroughStorage checks that every budget the design offers is accepted
// and that what the REST layer takes is exactly what the artifacts and budgets tables hold,
// through create, read, update and delete.
func TestBudgetShapes_RoundTripThroughStorage(t *testing.T) {
	for i, shape := range supportedBudgetShapes() {
		t.Run(shape.name, func(t *testing.T) {
			svc, _, dbPath := newTestServiceWithPath(t, true)
			raw, err := sql.Open("sqlite3", dbPath)
			require.NoError(t, err)
			t.Cleanup(func() { _ = raw.Close() })

			name := fmt.Sprintf("shape-%02d", i)
			request := map[string]any{
				"apiVersion": "gateway.api-platform.wso2.com/v1", "kind": "Budget",
				"metadata": map[string]any{"name": name}, "spec": shape.spec,
			}
			body, err := json.Marshal(request)
			require.NoError(t, err)
			want := normalise(t, withDefaults(shape.spec))

			// Create
			created, err := svc.Create(CreateParams{Body: body})
			require.NoError(t, err)

			// The artifacts row carries the kind's identity and state; the budgets row the spec.
			row, configuration, ok := readRows(t, raw, created.UUID)
			require.True(t, ok)
			assert.Equal(t, "Budget", row.kind)
			assert.Equal(t, name, row.handle)
			assert.Equal(t, name, row.displayName)
			assert.Equal(t, "v1", row.version)
			assert.Equal(t, "deployed", row.desiredState)
			assert.Equal(t, "gateway_api", row.origin)
			assert.Equal(t, "pending", row.cpSyncStatus.String)
			var stored map[string]any
			require.NoError(t, json.Unmarshal([]byte(configuration), &stored))
			assert.Equal(t, want, normalise(t, stored["spec"]), "budgets.configuration spec")
			assert.Equal(t, name, stored["metadata"].(map[string]any)["name"])

			// Read returns the same spec.
			got, err := svc.Get(name)
			require.NoError(t, err)
			assert.Equal(t, want, normalise(t, got.SourceConfiguration.(api.BudgetRequest).Spec), "GET spec")

			// Update an editable field; the row changes and nothing else does.
			updated := withDefaults(shape.spec)
			updatedLimit := updated["limit"].(map[string]any)
			updatedLimit["amount"] = "42.50"
			updated["status"] = "inactive"
			request["spec"] = updated
			body, err = json.Marshal(request)
			require.NoError(t, err)
			_, err = svc.Update(UpdateParams{Name: name, Body: body})
			require.NoError(t, err)

			after, configuration, ok := readRows(t, raw, created.UUID)
			require.True(t, ok)
			assert.False(t, after.updatedAt.Before(row.updatedAt), "updated_at went backwards")
			require.NoError(t, json.Unmarshal([]byte(configuration), &stored))
			assert.Equal(t, normalise(t, updated), normalise(t, stored["spec"]), "budgets.configuration after update")

			// Delete removes both rows.
			require.NoError(t, svc.Delete(name, "", nil))
			_, _, ok = readRows(t, raw, created.UUID)
			assert.False(t, ok, "artifacts row still there after delete")
		})
	}
}

// TestBudgetShapes_RejectsUnsupported lists the shapes the design describes that this iteration
// does not take, and the list limits, so a change that starts accepting one is noticed.
func TestBudgetShapes_RejectsUnsupported(t *testing.T) {
	allProviders := map[string]any{"type": "llmProvider", "allocation": "shared"}
	consumerOf := func(kind string) []any {
		return []any{map[string]any{"type": kind, "allocation": "shared", "expression": "true", "id": "x"}}
	}
	cases := []struct {
		name  string
		spec  map[string]any
		field string
	}{
		{"agent consumer", map[string]any{"limit": limit("10", "day", "block"), "resource": allProviders, "consumers": consumerOf("agent")}, "spec.consumers[0].type"},
		{"project consumer", map[string]any{"limit": limit("10", "day", "block"), "resource": allProviders, "consumers": consumerOf("project")}, "spec.consumers[0].type"},
		{"API key consumer", map[string]any{"limit": limit("10", "day", "block"), "resource": allProviders, "consumers": consumerOf("apiKey")}, "spec.consumers[0].type"},
		{"LLM proxy consumer", map[string]any{"limit": limit("10", "day", "block"), "resource": allProviders, "consumers": consumerOf("llmProxy")}, "spec.consumers[0].type"},
		{"environments", map[string]any{"limit": limit("10", "day", "block"), "resource": allProviders, "environments": map[string]any{"ids": []any{"production"}}}, "spec"},
		{"separate provider with id", map[string]any{"limit": limit("10", "day", "block"), "resource": map[string]any{"type": "llmProvider", "allocation": "separate", "id": "openai"}}, "spec.resource.id"},
		{"too many consumers", map[string]any{"limit": limit("10", "day", "block"), "resource": allProviders, "consumers": attributeConsumers(21, "shared")}, "spec.consumers"},
		{"too many exemptions", map[string]any{"limit": limit("10", "day", "block"), "resource": allProviders, "exemptions": attributeExemptions(101)}, "spec.exemptions"},
		{"amount too large", map[string]any{"limit": limit("1000000000000", "day", "block"), "resource": allProviders}, "spec.limit.amount"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, db := newTestService(t, false)
			body, err := json.Marshal(map[string]any{
				"apiVersion": "gateway.api-platform.wso2.com/v1", "kind": "Budget",
				"metadata": map[string]any{"name": "rejected"}, "spec": tc.spec,
			})
			require.NoError(t, err)

			_, err = svc.Create(CreateParams{Body: body})

			var invalid *ValidationError
			require.True(t, errors.As(err, &invalid), "got %v", err)
			fields := make([]string, 0, len(invalid.Errors))
			for _, fe := range invalid.Errors {
				fields = append(fields, fe.Field)
			}
			assert.Contains(t, fields, tc.field, "errors: %v", invalid.Errors)
			stored, err := db.GetAllConfigsByKind(models.KindBudget)
			require.NoError(t, err)
			assert.Empty(t, stored, "a rejected budget was stored")
		})
	}
}

func TestService_PublishesActiveBudgetsToThePolicyEngine(t *testing.T) {
	env := newTestEnv(t, false, nil)
	published := func() map[string]*storage.LazyResource {
		return env.resources.GetResourcesByType(utils.LazyResourceTypeBudget)
	}

	_, err := env.svc.Create(CreateParams{Body: []byte(validBudget)})
	require.NoError(t, err)
	require.Contains(t, published(), "research-team-monthly")
	assert.Equal(t, "500.00", published()["research-team-monthly"].Resource["limit"].(map[string]interface{})["amount"])

	// An edit reaches the policy engine.
	_, err = env.svc.Update(UpdateParams{Name: "research-team-monthly", Body: []byte(strings.Replace(validBudget, `"500.00"`, `"650.00"`, 1))})
	require.NoError(t, err)
	assert.Equal(t, "650.00", published()["research-team-monthly"].Resource["limit"].(map[string]interface{})["amount"])

	// Deactivating withdraws it; reactivating sends it again.
	inactive := strings.Replace(validBudget, `"limit":`, `"status": "inactive", "limit":`, 1)
	_, err = env.svc.Update(UpdateParams{Name: "research-team-monthly", Body: []byte(inactive)})
	require.NoError(t, err)
	assert.Empty(t, published())
	_, err = env.svc.Update(UpdateParams{Name: "research-team-monthly", Body: []byte(validBudget)})
	require.NoError(t, err)
	assert.Contains(t, published(), "research-team-monthly")

	// Deleting withdraws it.
	require.NoError(t, env.svc.Delete("research-team-monthly", "", nil))
	assert.Empty(t, published())
}

func TestService_InactiveBudgetIsStoredButNotPublished(t *testing.T) {
	env := newTestEnv(t, false, nil)
	inactive := strings.Replace(validBudget, `"limit":`, `"status": "inactive", "limit":`, 1)

	_, err := env.svc.Create(CreateParams{Body: []byte(inactive)})

	require.NoError(t, err)
	_, err = env.svc.Get("research-team-monthly")
	require.NoError(t, err)
	assert.Empty(t, env.resources.GetResourcesByType(utils.LazyResourceTypeBudget))
}

// failingPublisher always fails, standing in for a policy-engine update that cannot be made.
type failingPublisher struct{}

func (failingPublisher) Publish(string) error { return errors.New("snapshot failed") }

func TestService_PublishFailureKeepsTheChange(t *testing.T) {
	env := newTestEnv(t, false, failingPublisher{})

	_, err := env.svc.Create(CreateParams{Body: []byte(validBudget)})

	assert.ErrorIs(t, err, ErrPublish)
	_, err = env.svc.Get("research-team-monthly")
	assert.NoError(t, err, "the budget is saved even though publishing failed")
}

func TestService_TellsOtherReplicas(t *testing.T) {
	env := newTestEnv(t, false, nil)

	_, err := env.svc.Create(CreateParams{Body: []byte(validBudget), CorrelationID: "corr-1"})
	require.NoError(t, err)
	_, err = env.svc.Update(UpdateParams{Name: "research-team-monthly", Body: []byte(strings.Replace(validBudget, `"500.00"`, `"650.00"`, 1)), CorrelationID: "corr-2"})
	require.NoError(t, err)
	require.NoError(t, env.svc.Delete("research-team-monthly", "corr-3", nil))

	require.Len(t, env.events.events, 3)
	for i, action := range []string{"CREATE", "UPDATE", "DELETE"} {
		event := env.events.events[i]
		assert.Equal(t, eventhub.EventTypeBudget, event.EventType)
		assert.Equal(t, action, event.Action)
		assert.Equal(t, "research-team-monthly", event.EntityID)
		assert.Equal(t, fmt.Sprintf("corr-%d", i+1), event.EventID)
		assert.Equal(t, "test-gateway", event.GatewayID)
	}
}

func TestService_RejectedChangesTellNoOne(t *testing.T) {
	env := newTestEnv(t, false, nil)

	_, err := env.svc.Create(CreateParams{Body: []byte(strings.Replace(validBudget, `"500.00"`, `"0"`, 1))})
	require.Error(t, err)
	require.Error(t, env.svc.Delete("missing", "", nil))

	assert.Empty(t, env.events.events)
}

func TestService_PublishFailureStillTellsOtherReplicas(t *testing.T) {
	env := newTestEnv(t, false, failingPublisher{})

	_, err := env.svc.Create(CreateParams{Body: []byte(validBudget)})

	assert.ErrorIs(t, err, ErrPublish)
	assert.Len(t, env.events.events, 1, "the change is saved, so other replicas must still sync it")
}

func TestCreate_UUIDFromTheArtifactIDAnnotation(t *testing.T) {
	env := newTestEnv(t, false, nil)
	const given = "0193f1c2-1111-7000-8000-000000000001"
	annotated := strings.Replace(validBudget, `"metadata": {"name": "research-team-monthly"}`,
		`"metadata": {"name": "research-team-monthly", "annotations": {"gateway.api-platform.wso2.com/artifact-id": "`+given+`"}}`, 1)

	cfg, err := env.svc.Create(CreateParams{Body: []byte(annotated)})

	require.NoError(t, err)
	assert.Equal(t, given, cfg.UUID)

	// The annotation is the budget's identity: it cannot change.
	changed := strings.Replace(annotated, given, "0193f1c2-2222-7000-8000-000000000002", 1)
	_, err = env.svc.Update(UpdateParams{Name: "research-team-monthly", Body: []byte(changed)})
	var invalid *ValidationError
	require.True(t, errors.As(err, &invalid), "got %v", err)
	assert.Equal(t, "metadata.annotations.gateway.api-platform.wso2.com/artifact-id", invalid.Errors[0].Field)
}

func TestCreate_GeneratesAUUIDWithoutTheAnnotation(t *testing.T) {
	first := newTestEnv(t, false, nil)
	second := newTestEnv(t, false, nil) // another gateway

	a, err := first.svc.Create(CreateParams{Body: []byte(validBudget)})
	require.NoError(t, err)
	b, err := second.svc.Create(CreateParams{Body: []byte(validBudget)})
	require.NoError(t, err)

	assert.NotEmpty(t, a.UUID)
	assert.NotEqual(t, a.UUID, b.UUID, "each gateway assigns its own; spend is shared by name")
}

func TestCreate_RejectsAnInvalidArtifactID(t *testing.T) {
	env := newTestEnv(t, false, nil)
	annotated := strings.Replace(validBudget, `"metadata": {"name": "research-team-monthly"}`,
		`"metadata": {"name": "research-team-monthly", "annotations": {"gateway.api-platform.wso2.com/artifact-id": "not-a-uuid"}}`, 1)

	_, err := env.svc.Create(CreateParams{Body: []byte(annotated)})

	var invalid *ValidationError
	require.True(t, errors.As(err, &invalid), "got %v", err)
}
