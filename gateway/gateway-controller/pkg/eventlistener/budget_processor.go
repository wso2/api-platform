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

package eventlistener

import (
	"context"
	"log/slog"
	"time"

	"github.com/wso2/api-platform/common/budgetspend"
	"github.com/wso2/api-platform/common/eventhub"
)

// BudgetPublisher republishes the active budgets to the policy engine from the database.
type BudgetPublisher interface {
	Publish(correlationID string) error
}

// processBudgetEvent brings this replica's policy engine in line with a budget written on
// any replica. The event names the budget but carries no budget, so every action
// republishes the whole set from the database.
func (l *EventListener) processBudgetEvent(event eventhub.Event) {
	switch event.Action {
	case "CREATE", "UPDATE", "DELETE":
	default:
		l.logger.Warn("Unknown budget event action",
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID))
		return
	}

	log := l.logger.With(
		slog.String("budget", event.EntityID),
		slog.String("action", event.Action),
		slog.String("event_id", event.EventID))

	if event.Action == "DELETE" {
		// The replica that deleted it has forgotten it; every other one does too.
		if l.budgetForgetter != nil {
			l.budgetForgetter.Forget(event.EntityID)
		}
		if l.localSpend != nil {
			l.clearLocalSpend(event.EntityID, log)
		}
	}
	if err := l.budgets.Publish(event.EventID); err != nil {
		log.Error("Failed to publish budgets for replica sync", slog.Any("error", err))
		return
	}
	log.Info("Budget change applied from replica sync")
}

// clearLocalSpend deletes a budget's spend from this replica's in-memory counters, as the
// replica that deleted it did with its own.
func (l *EventListener) clearLocalSpend(name string, log *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	prefix := budgetspend.CounterIDPrefix(name)
	for _, namespace := range []string{budgetspend.NamespaceSpend, budgetspend.NamespaceUnpriced} {
		if err := l.localSpend.Delete(ctx, namespace, prefix); err != nil {
			log.Error("Failed to clear the deleted budget's spend on this replica", slog.Any("error", err))
			return
		}
	}
	log.Info("Deleted budget's spend cleared on this replica")
}
