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
	currency := api.BudgetCurrencyUSD
	provider := "openai"
	consumers := []api.BudgetConsumer{{
		Type: api.BudgetConsumerTypeAttribute, Allocation: api.BudgetConsumerAllocationSeparate,
		Expression: `request.Auth.Claims["sub"] != ""`,
	}}
	budget := api.BudgetRequest{
		ApiVersion: api.BudgetRequestApiVersionV1,
		Kind:       api.BudgetRequestKindBudget,
		Metadata:   api.BudgetMetadata{Name: name},
		Spec: api.BudgetSpec{
			Status: &status,
			Limit: api.BudgetLimit{
				Amount: amount, Currency: &currency, Period: api.BudgetPeriodMonth,
				Action: api.BudgetActionBlock, BlockAt: &blockAt,
			},
			Resource:  api.BudgetResource{Type: api.BudgetResourceTypeLlmProvider, Allocation: api.BudgetResourceAllocationShared, Id: &provider},
			Consumers: &consumers,
		},
	}
	return &models.StoredConfig{
		UUID: "uuid-" + name, Kind: models.KindBudget, Handle: name,
		Configuration: budget, SourceConfiguration: budget,
	}
}

func TestBudgetPublisher_PublishesActiveBudgets(t *testing.T) {
	rows := &fakeBudgetRows{rows: []*models.StoredConfig{
		budgetRow("research-monthly", "500.00", api.BudgetStatusActive),
		budgetRow("paused", "10", api.BudgetStatusInactive),
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
	assert.Equal(t, "openai", body["resource"].(map[string]interface{})["id"])
	consumers := body["consumers"].([]interface{})
	require.Len(t, consumers, 1)
	assert.Equal(t, "separate", consumers[0].(map[string]interface{})["allocation"])
	assert.NotContains(t, body, "exemptions", "absent lists are left out")
	assert.NotContains(t, body, "status", "only active budgets are sent")
}

func TestBudgetPublisher_FollowsChanges(t *testing.T) {
	rows := &fakeBudgetRows{rows: []*models.StoredConfig{
		budgetRow("a", "100", api.BudgetStatusActive),
		budgetRow("b", "200", api.BudgetStatusActive),
	}}
	publisher, manager := newTestBudgetPublisher(t, rows)
	require.NoError(t, publisher.Publish(""))

	// a is updated, b is deactivated, c is created.
	rows.rows = []*models.StoredConfig{
		budgetRow("a", "150", api.BudgetStatusActive),
		budgetRow("b", "200", api.BudgetStatusInactive),
		budgetRow("c", "300", api.BudgetStatusActive),
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
	rows := &fakeBudgetRows{rows: []*models.StoredConfig{budgetRow("a", "100", api.BudgetStatusActive)}}
	publisher, manager := newTestBudgetPublisher(t, rows)
	require.NoError(t, publisher.Publish(""))

	rows.listErr = errors.New("database unavailable")
	require.Error(t, publisher.Publish(""))

	assert.Len(t, manager.GetResourcesByType(LazyResourceTypeBudget), 1)
}

func TestBudgetPublisher_RejectsWrongConfigurationType(t *testing.T) {
	row := budgetRow("a", "100", api.BudgetStatusActive)
	row.SourceConfiguration = map[string]any{"spec": "not a budget"}
	publisher, _ := newTestBudgetPublisher(t, &fakeBudgetRows{rows: []*models.StoredConfig{row}})

	assert.Error(t, publisher.Publish(""))
}
