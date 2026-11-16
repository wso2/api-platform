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
	"errors"
	"log/slog"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// Budgets created in the control plane reach the gateway the way subscription plans do:
// budget.created, budget.updated and budget.deleted events carry them on the control-plane
// connection, and on every connect the gateway fetches the full list (GET /budgets), saves
// the newer ones and drops the control-plane budgets no longer listed.

// BudgetEvent is a budget.created or budget.updated event.
type BudgetEvent struct {
	Type          string                   `json:"type"`
	Payload       utils.ControlPlaneBudget `json:"payload"`
	Timestamp     string                   `json:"timestamp"`
	CorrelationID string                   `json:"correlationId"`
}

// BudgetDeletedEvent is a budget.deleted event.
type BudgetDeletedEvent struct {
	Type    string `json:"type"`
	Payload struct {
		BudgetID string `json:"budgetId"`
	} `json:"payload"`
	Timestamp     string `json:"timestamp"`
	CorrelationID string `json:"correlationId"`
}

// SetBudgetApplier sets what stores the budgets the control plane sends: the budget service.
func (c *Client) SetBudgetApplier(applier utils.ControlPlaneBudgetApplier) {
	c.budgetApplierMu.Lock()
	defer c.budgetApplierMu.Unlock()
	c.budgetApplier = applier
}

func (c *Client) getBudgetApplier() utils.ControlPlaneBudgetApplier {
	c.budgetApplierMu.RLock()
	defer c.budgetApplierMu.RUnlock()
	return c.budgetApplier
}

// handleBudgetEvent processes budget.created and budget.updated events.
func (c *Client) handleBudgetEvent(event map[string]interface{}) {
	applier := c.getBudgetApplier()
	if applier == nil {
		c.logger.Debug("Ignoring a budget event: budgets are not enabled on this gateway")
		return
	}
	var evt BudgetEvent
	if err := utils.MapToStruct(event, &evt); err != nil {
		c.logger.Error("Failed to parse budget event", slog.Any("error", err))
		return
	}
	log := c.logger.With(slog.String("correlation_id", evt.CorrelationID), slog.String("event", evt.Type))
	if err := applier.ApplyControlPlaneBudget(evt.Payload, evt.CorrelationID, log); err != nil {
		log.Error("Failed to apply control-plane budget", slog.String("budget_id", evt.Payload.BudgetID), slog.Any("error", err))
	}
}

// handleBudgetDeletedEvent processes budget.deleted events.
func (c *Client) handleBudgetDeletedEvent(event map[string]interface{}) {
	applier := c.getBudgetApplier()
	if applier == nil {
		c.logger.Debug("Ignoring a budget event: budgets are not enabled on this gateway")
		return
	}
	var evt BudgetDeletedEvent
	if err := utils.MapToStruct(event, &evt); err != nil || evt.Payload.BudgetID == "" {
		c.logger.Error("Failed to parse budget.deleted event", slog.Any("error", err))
		return
	}
	log := c.logger.With(slog.String("correlation_id", evt.CorrelationID), slog.String("event", evt.Type))
	if err := applier.DeleteControlPlaneBudget(evt.Payload.BudgetID, evt.CorrelationID, log); err != nil {
		log.Error("Failed to delete control-plane budget", slog.String("budget_id", evt.Payload.BudgetID), slog.Any("error", err))
	}
}

// syncBudgets fetches the control-plane budgets this gateway enforces, saves the newer
// ones and deletes the stored control-plane budgets not listed. It runs on every connect.
func (c *Client) syncBudgets() {
	if c.isOnPrem() || c.apiUtilsService == nil {
		return
	}
	applier := c.getBudgetApplier()
	if applier == nil {
		return
	}
	budgets, err := c.apiUtilsService.FetchBudgets()
	if errors.Is(err, utils.ErrBudgetsNotSupported) {
		c.logger.Debug("The control plane does not support budgets; keeping the stored ones")
		return
	}
	if err != nil {
		c.logger.Warn("Failed to sync control-plane budgets", slog.Any("error", err))
		return
	}
	listed := make(map[string]bool, len(budgets))
	saved := 0
	for _, b := range budgets {
		select {
		case <-c.ctx.Done():
			return
		default:
		}
		listed[b.BudgetID] = true
		correlationID := utils.GenerateDeterministicUUIDv7(b.BudgetID, b.UpdatedAt)
		if err := applier.ApplyControlPlaneBudget(b, correlationID, c.logger); err != nil {
			c.logger.Warn("Failed to save control-plane budget during sync",
				slog.String("budget_id", b.BudgetID), slog.Any("error", err))
			continue
		}
		saved++
	}
	stored, err := applier.ControlPlaneBudgetIDs()
	if err != nil {
		c.logger.Warn("Failed to list control-plane budgets for reconciliation", slog.Any("error", err))
		return
	}
	removed := 0
	for _, id := range stored {
		if listed[id] {
			continue
		}
		if err := applier.DeleteControlPlaneBudget(id, utils.GenerateDeterministicUUIDv7(id, time.Now()), c.logger); err != nil {
			c.logger.Warn("Failed to delete a control-plane budget no longer listed",
				slog.String("budget_id", id), slog.Any("error", err))
			continue
		}
		removed++
	}
	c.logger.Info("Control-plane budget sync complete",
		slog.Int("listed", len(budgets)), slog.Int("saved", saved), slog.Int("removed", removed))
}
