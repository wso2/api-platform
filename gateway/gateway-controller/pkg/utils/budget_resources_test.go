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

package utils

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/lazyresourcexds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// fakeBudgetRows is a BudgetLister over a fixed list, standing in for the database.
type fakeBudgetRows struct {
	rows    []*models.StoredConfig
	listErr error
}

func (f *fakeBudgetRows) GetAllConfigsByKind(kind string) ([]*models.StoredConfig, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []*models.StoredConfig
	for _, row := range f.rows {
		if row.Kind == kind {
			out = append(out, row)
		}
	}
	return out, nil
}

func newTestBudgetPublisher(t *testing.T, rows *fakeBudgetRows) (*BudgetPublisher, *lazyresourcexds.LazyResourceStateManager) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := storage.NewLazyResourceStore(logger)
	manager := lazyresourcexds.NewLazyResourceStateManager(store, lazyresourcexds.NewLazyResourceSnapshotManager(store, logger), logger)
	return NewBudgetPublisher(rows, manager), manager
}

func budgetRow(name, amount string, status api.BudgetSpecStatus) *models.StoredConfig {
	blockAt := 95
	currency := api.USD
	provider := "openai"
	filters := []api.BudgetFilter{{Type: api.BudgetFilterTypeLlmProvider, Id: &provider}}
	split := &api.BudgetSplit{
		Type:       api.BudgetSplitTypeAttribute,
		Expression: func() *string { e := `request.Auth.Claims["sub"]`; return &e }(),
	}
	budget := api.BudgetRequest{
		ApiVersion: api.BudgetRequestApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.BudgetRequestKindBudget,
		Metadata:   api.BudgetMetadata{Name: name},
		Spec: api.BudgetSpec{
			Status: &status,
			Limit: api.BudgetLimit{
				Amount: amount, Currency: &currency, Period: api.BudgetLimitPeriodMonth,
				Action: api.Block, BlockAt: &blockAt, SplitBy: split,
			},
			Filters: &filters,
		},
	}
	return &models.StoredConfig{
		UUID: "uuid-" + name, Kind: models.KindBudget, Handle: name,
		Configuration: budget, SourceConfiguration: budget,
	}
}

func TestBudgetPublisher_PublishesActiveBudgets(t *testing.T) {
	rows := &fakeBudgetRows{rows: []*models.StoredConfig{
		budgetRow("research-monthly", "500.00", api.BudgetSpecStatusActive),
		budgetRow("paused", "10", api.BudgetSpecStatusInactive),
	}}
	publisher, manager := newTestBudgetPublisher(t, rows)

	require.NoError(t, publisher.Publish(""))

	published := manager.GetResourcesByType(LazyResourceTypeBudget)
	require.Len(t, published, 1, "inactive budgets are not published")
	body := published["research-monthly"].Resource
	assert.Equal(t, "uuid-research-monthly", body["uuid"])
	assert.Equal(t, "research-monthly", body["name"])
	limit := body["limit"].(map[string]interface{})
	assert.Equal(t, "500.00", limit["amount"])
	assert.Equal(t, "month", limit["period"])
	assert.Equal(t, "block", limit["action"])
	assert.Equal(t, float64(95), limit["blockAt"])
	assert.NotContains(t, limit, "splitBy", "the split is carried by the engine's resource and consumers")
	assert.Equal(t, "openai", body["resource"].(map[string]interface{})["id"])
	consumers := body["consumers"].([]interface{})
	require.Len(t, consumers, 1)
	assert.Equal(t, "separate", consumers[0].(map[string]interface{})["allocation"])
	assert.NotContains(t, body, "exemptions", "absent lists are left out")
	assert.NotContains(t, body, "status", "only active budgets are sent")
}

func TestBudgetPublisher_FollowsChanges(t *testing.T) {
	rows := &fakeBudgetRows{rows: []*models.StoredConfig{
		budgetRow("a", "100", api.BudgetSpecStatusActive),
		budgetRow("b", "200", api.BudgetSpecStatusActive),
	}}
	publisher, manager := newTestBudgetPublisher(t, rows)
	require.NoError(t, publisher.Publish(""))

	// a is updated, b is deactivated, c is created.
	rows.rows = []*models.StoredConfig{
		budgetRow("a", "150", api.BudgetSpecStatusActive),
		budgetRow("b", "200", api.BudgetSpecStatusInactive),
		budgetRow("c", "300", api.BudgetSpecStatusActive),
	}
	require.NoError(t, publisher.Publish(""))
	published := manager.GetResourcesByType(LazyResourceTypeBudget)
	require.Len(t, published, 2)
	assert.Equal(t, "150", published["a"].Resource["limit"].(map[string]interface{})["amount"])
	assert.Contains(t, published, "c")
	assert.NotContains(t, published, "b")

	// Everything deleted.
	rows.rows = nil
	require.NoError(t, publisher.Publish(""))
	assert.Empty(t, manager.GetResourcesByType(LazyResourceTypeBudget))
}

func TestBudgetPublisher_LeavesOtherTypesAlone(t *testing.T) {
	publisher, manager := newTestBudgetPublisher(t, &fakeBudgetRows{})
	require.NoError(t, manager.StoreResource(&storage.LazyResource{
		ID: "openai", ResourceType: LazyResourceTypeLLMProviderTemplate, Resource: map[string]interface{}{"x": "y"},
	}, ""))

	require.NoError(t, publisher.Publish(""))

	_, ok := manager.GetResourceByIDAndType("openai", LazyResourceTypeLLMProviderTemplate)
	assert.True(t, ok)
}

func TestBudgetPublisher_ListErrorKeepsPublishedBudgets(t *testing.T) {
	rows := &fakeBudgetRows{rows: []*models.StoredConfig{budgetRow("a", "100", api.BudgetSpecStatusActive)}}
	publisher, manager := newTestBudgetPublisher(t, rows)
	require.NoError(t, publisher.Publish(""))

	rows.listErr = errors.New("database unavailable")
	require.Error(t, publisher.Publish(""))

	assert.Len(t, manager.GetResourcesByType(LazyResourceTypeBudget), 1)
}

func TestBudgetPublisher_RejectsWrongConfigurationType(t *testing.T) {
	row := budgetRow("a", "100", api.BudgetSpecStatusActive)
	row.SourceConfiguration = map[string]any{"spec": "not a budget"}
	publisher, _ := newTestBudgetPublisher(t, &fakeBudgetRows{rows: []*models.StoredConfig{row}})

	assert.Error(t, publisher.Publish(""))
}

func TestEngineSelection_MapsFiltersAndSplitToTheEngineModel(t *testing.T) {
	s := func(v string) *string { return &v }
	cases := []struct {
		name      string
		filters   []api.BudgetFilter
		split     *api.BudgetSplit
		resource  map[string]interface{}
		consumers []interface{}
	}{
		{
			name:     "neither: all providers, shared",
			resource: map[string]interface{}{"type": "llmProvider", "allocation": "shared"},
		},
		{
			name:     "llmProvider filter: that provider, shared",
			filters:  []api.BudgetFilter{{Type: api.BudgetFilterTypeLlmProvider, Id: s("openai")}},
			resource: map[string]interface{}{"type": "llmProvider", "allocation": "shared", "id": "openai"},
		},
		{
			name:     "llmProvider split: each provider",
			split:    &api.BudgetSplit{Type: api.BudgetSplitTypeLlmProvider},
			resource: map[string]interface{}{"type": "llmProvider", "allocation": "separate"},
		},
		{
			name: "other filters shared in order, split separate last",
			filters: []api.BudgetFilter{
				{Type: api.BudgetFilterTypeLlmProvider, Id: s("openai")},
				{Type: api.BudgetFilterTypeAgent, Project: s("online-shop"), Id: s("returns-bot")},
				{Type: api.BudgetFilterTypeApiKey, Id: s("team-alpha-key")},
				{Type: api.BudgetFilterTypeAttribute, Expression: s(`'R&D' in request.Auth.Claims["groups"]`)},
			},
			split:    &api.BudgetSplit{Type: api.BudgetSplitTypeAttribute, Expression: s("request.Auth.Subject")},
			resource: map[string]interface{}{"type": "llmProvider", "allocation": "shared", "id": "openai"},
			consumers: []interface{}{
				map[string]interface{}{"type": "agent", "allocation": "shared", "project": "online-shop", "id": "returns-bot"},
				map[string]interface{}{"type": "apiKey", "allocation": "shared", "id": "team-alpha-key"},
				map[string]interface{}{"type": "attribute", "allocation": "shared", "expression": `'R&D' in request.Auth.Claims["groups"]`},
				map[string]interface{}{"type": "attribute", "allocation": "separate", "expression": "request.Auth.Subject"},
			},
		},
		{
			name: "filters without an id: any value of their type, shared",
			filters: []api.BudgetFilter{
				{Type: api.BudgetFilterTypeAgent},
				{Type: api.BudgetFilterTypeApiKey},
			},
			resource: map[string]interface{}{"type": "llmProvider", "allocation": "shared"},
			consumers: []interface{}{
				map[string]interface{}{"type": "agent", "allocation": "shared"},
				map[string]interface{}{"type": "apiKey", "allocation": "shared"},
			},
		},
		{
			name:     "agent split",
			filters:  []api.BudgetFilter{{Type: api.BudgetFilterTypeProject, Id: s("online-shop")}},
			split:    &api.BudgetSplit{Type: api.BudgetSplitTypeAgent},
			resource: map[string]interface{}{"type": "llmProvider", "allocation": "shared"},
			consumers: []interface{}{
				map[string]interface{}{"type": "project", "allocation": "shared", "id": "online-shop"},
				map[string]interface{}{"type": "agent", "allocation": "separate"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := api.BudgetSpec{Limit: api.BudgetLimit{SplitBy: tc.split}}
			if tc.filters != nil {
				spec.Filters = &tc.filters
			}
			resource, consumers := engineSelection(spec)
			assert.Equal(t, tc.resource, resource)
			assert.Equal(t, tc.consumers, consumers)
		})
	}
}
