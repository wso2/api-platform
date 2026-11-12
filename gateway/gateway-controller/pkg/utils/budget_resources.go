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
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/lazyresourcexds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// LazyResourceTypeBudget is the lazy-resource type that carries budgets to the policy engine.
const LazyResourceTypeBudget = "Budget"

// BudgetLister reads the stored budgets.
type BudgetLister interface {
	GetAllConfigsByKind(kind string) ([]*models.StoredConfig, error)
}

// BudgetPublisher publishes the active budgets to the policy engine as one lazy resource per
// budget, keyed by the budget's name. Inactive budgets are stored but not published.
type BudgetPublisher struct {
	budgets   BudgetLister
	resources *lazyresourcexds.LazyResourceStateManager

	// mu serialises Publish so the last one to run always leaves the latest committed
	// budgets published.
	mu sync.Mutex
}

// NewBudgetPublisher creates a publisher reading budgets from budgets and publishing them
// through resources.
func NewBudgetPublisher(budgets BudgetLister, resources *lazyresourcexds.LazyResourceStateManager) *BudgetPublisher {
	return &BudgetPublisher{budgets: budgets, resources: resources}
}

// Publish makes the published budgets match the active budgets in the database. It stores
// before it removes, so no intermediate snapshot lacks a budget that is still active.
func (p *BudgetPublisher) Publish(correlationID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	stored, err := p.budgets.GetAllConfigsByKind(models.KindBudget)
	if err != nil {
		return fmt.Errorf("listing budgets: %w", err)
	}

	published := p.resources.GetResourcesByType(LazyResourceTypeBudget)
	active := make(map[string]struct{}, len(stored))
	for _, cfg := range stored {
		resource, ok, err := budgetResource(cfg)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		active[resource.ID] = struct{}{}
		if existing, found := published[resource.ID]; found && reflect.DeepEqual(existing.Resource, resource.Resource) {
			continue
		}
		if err := p.resources.StoreResource(resource, correlationID); err != nil {
			return fmt.Errorf("publishing budget %q: %w", resource.ID, err)
		}
	}

	for name := range published {
		if _, ok := active[name]; ok {
			continue
		}
		if err := p.resources.RemoveResourceByIDAndType(name, LazyResourceTypeBudget, correlationID); err != nil {
			return fmt.Errorf("removing budget %q: %w", name, err)
		}
	}
	return nil
}

// budgetResource builds the published resource for a stored budget. It reports false for an
// inactive budget, which is not published.
func budgetResource(cfg *models.StoredConfig) (*storage.LazyResource, bool, error) {
	budget, ok := cfg.SourceConfiguration.(api.BudgetRequest)
	if !ok {
		return nil, false, fmt.Errorf("budget %q has configuration of type %T", cfg.Handle, cfg.SourceConfiguration)
	}
	if budget.Spec.Status != nil && *budget.Spec.Status != api.BudgetStatusActive {
		return nil, false, nil
	}
	body := map[string]interface{}{
		"uuid": cfg.UUID,
		"name": budget.Metadata.Name,
	}
	fields := map[string]interface{}{
		"limit":    budget.Spec.Limit,
		"resource": budget.Spec.Resource,
	}
	if budget.Spec.Consumers != nil {
		fields["consumers"] = *budget.Spec.Consumers
	}
	if budget.Spec.Exemptions != nil {
		fields["exemptions"] = *budget.Spec.Exemptions
	}
	for key, value := range fields {
		plain, err := plainJSON(value)
		if err != nil {
			return nil, false, fmt.Errorf("budget %q: encoding %s: %w", cfg.Handle, key, err)
		}
		body[key] = plain
	}
	return &storage.LazyResource{ID: budget.Metadata.Name, ResourceType: LazyResourceTypeBudget, Resource: body}, true, nil
}

// plainJSON turns a typed value into maps, slices, strings, float64s and bools, the form a lazy
// resource carries over xDS.
func plainJSON(value any) (any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
