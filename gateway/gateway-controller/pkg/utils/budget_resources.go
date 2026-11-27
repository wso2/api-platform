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
	"errors"
	"fmt"
	"reflect"
	"sync"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/lazyresourcexds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// ErrBudgetsNotSupported is a control plane that does not keep budgets: it does not
// serve GET /budgets.
var ErrBudgetsNotSupported = errors.New("the control plane does not support budgets")

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

// PublishedNames returns the names of the budgets published to the policy engine.
func (p *BudgetPublisher) PublishedNames() map[string]bool {
	published := p.resources.GetResourcesByType(LazyResourceTypeBudget)
	names := make(map[string]bool, len(published))
	for name := range published {
		names[name] = true
	}
	return names
}

// budgetResource builds the published resource for a stored budget. It reports false for an
// inactive budget, which is not published.
func budgetResource(cfg *models.StoredConfig) (*storage.LazyResource, bool, error) {
	budget, ok := cfg.SourceConfiguration.(api.BudgetRequest)
	if !ok {
		return nil, false, fmt.Errorf("budget %q has configuration of type %T", cfg.Handle, cfg.SourceConfiguration)
	}
	if budget.Spec.Status != nil && *budget.Spec.Status != api.BudgetSpecStatusActive {
		return nil, false, nil
	}
	body := map[string]interface{}{
		"uuid": cfg.UUID,
		"name": budget.Metadata.Name,
	}
	limit, err := plainJSON(budget.Spec.Limit)
	if err != nil {
		return nil, false, fmt.Errorf("budget %q: encoding limit: %w", cfg.Handle, err)
	}
	// splitBy is carried by the engine's resource and consumers below.
	delete(limit.(map[string]interface{}), "splitBy")
	body["limit"] = limit
	resource, consumers := engineSelection(budget.Spec)
	body["resource"] = resource
	if len(consumers) > 0 {
		body["consumers"] = consumers
	}
	if budget.Spec.Exemptions != nil {
		exemptions, err := plainJSON(*budget.Spec.Exemptions)
		if err != nil {
			return nil, false, fmt.Errorf("budget %q: encoding exemptions: %w", cfg.Handle, err)
		}
		body["exemptions"] = exemptions
	}
	return &storage.LazyResource{ID: budget.Metadata.Name, ResourceType: LazyResourceTypeBudget, Resource: body}, true, nil
}

// Allocation values of the budget system policy's model.
const (
	engineAllocationShared   = "shared"
	engineAllocationSeparate = "separate"
)

// engineSelection maps the kind's filters and splitBy to the budget system policy's model,
// which names usages and spend counters: one LLM provider resource and a list of
// consumers, each shared (a condition) or separate (one usage per value). An llmProvider
// filter is the shared provider resource with that id (several llmProvider filters are its
// ids: a request to any of them is covered); an llmProvider split is the separate provider
// resource; neither is all providers, shared. Every other filter is a shared consumer of
// its type (without an id it is any value of that type; several of one type are
// alternatives, which the policy engine groups), and every other split a separate consumer
// of its type.
func engineSelection(spec api.BudgetSpec) (map[string]interface{}, []interface{}) {
	resource := map[string]interface{}{"type": string(api.BudgetFilterTypeLlmProvider), "allocation": engineAllocationShared}
	var consumers []interface{}
	var providerIDs []interface{}
	if spec.Filters != nil {
		for _, f := range *spec.Filters {
			if f.Type == api.BudgetFilterTypeLlmProvider {
				if f.Id != nil {
					providerIDs = append(providerIDs, *f.Id)
				}
				continue
			}
			consumer := map[string]interface{}{"type": string(f.Type), "allocation": engineAllocationShared}
			if f.Id != nil {
				consumer["id"] = *f.Id
			}
			if f.Project != nil {
				consumer["project"] = *f.Project
			}
			if f.Expression != nil {
				consumer["expression"] = *f.Expression
			}
			consumers = append(consumers, consumer)
		}
	}
	switch len(providerIDs) {
	case 0:
	case 1:
		// One provider keeps the single id, so a stored budget's published form is unchanged.
		resource["id"] = providerIDs[0]
	default:
		resource["ids"] = providerIDs
	}
	if split := spec.Limit.SplitBy; split != nil {
		if split.Type == api.BudgetSplitTypeLlmProvider {
			resource["allocation"] = engineAllocationSeparate
			delete(resource, "id")
			delete(resource, "ids")
		} else {
			consumer := map[string]interface{}{"type": string(split.Type), "allocation": engineAllocationSeparate}
			if split.Expression != nil {
				consumer["expression"] = *split.Expression
			}
			consumers = append(consumers, consumer)
		}
	}
	return resource, consumers
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
