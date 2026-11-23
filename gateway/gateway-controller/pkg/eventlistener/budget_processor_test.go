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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wso2/api-platform/common/budgetspend"
	"github.com/wso2/api-platform/common/eventhub"
)

type fakeBudgetPublisher struct {
	calls []string
	err   error
}

func (f *fakeBudgetPublisher) Publish(correlationID string) error {
	f.calls = append(f.calls, "publish:"+correlationID)
	return f.err
}

func TestHandleEvent_Budget_Republishes(t *testing.T) {
	for _, action := range []string{"CREATE", "UPDATE", "DELETE"} {
		t.Run(action, func(t *testing.T) {
			budgets := &fakeBudgetPublisher{}
			listener := &EventListener{logger: newTestLogger(), budgets: budgets}

			listener.handleEvent(eventhub.Event{EventType: eventhub.EventTypeBudget, Action: action, EntityID: "research-monthly", EventID: "corr-1"})

			assert.Equal(t, []string{"publish:corr-1"}, budgets.calls)
		})
	}
}

func TestHandleEvent_Budget_UnknownActionIsIgnored(t *testing.T) {
	budgets := &fakeBudgetPublisher{}
	listener := &EventListener{logger: newTestLogger(), budgets: budgets}

	listener.handleEvent(eventhub.Event{EventType: eventhub.EventTypeBudget, Action: "RENAME", EntityID: "research-monthly"})

	assert.Empty(t, budgets.calls)
}

func TestHandleEvent_Budget_PublishFailureIsLogged(t *testing.T) {
	budgets := &fakeBudgetPublisher{err: errors.New("snapshot failed")}
	listener := &EventListener{logger: newTestLogger(), budgets: budgets}

	assert.NotPanics(t, func() {
		listener.handleEvent(eventhub.Event{EventType: eventhub.EventTypeBudget, Action: "UPDATE", EntityID: "research-monthly", EventID: "corr-2"})
	})
	assert.Equal(t, []string{"publish:corr-2"}, budgets.calls)
}

type fakeBudgetForgetter struct{ forgotten []string }

func (f *fakeBudgetForgetter) Forget(budget string) { f.forgotten = append(f.forgotten, budget) }

func TestHandleEvent_Budget_DeleteForgetsItsUsageOnThisReplica(t *testing.T) {
	budgets := &fakeBudgetPublisher{}
	forgetter := &fakeBudgetForgetter{}
	listener := &EventListener{logger: newTestLogger(), budgets: budgets}
	listener.SetBudgetForgetter(forgetter)

	for _, action := range []string{"CREATE", "UPDATE"} {
		listener.handleEvent(eventhub.Event{EventType: eventhub.EventTypeBudget, Action: action, EntityID: "research-monthly", EventID: "c-" + action})
	}
	assert.Empty(t, forgetter.forgotten, "creates and updates keep the totals")

	listener.handleEvent(eventhub.Event{EventType: eventhub.EventTypeBudget, Action: "DELETE", EntityID: "research-monthly", EventID: "c-del"})
	assert.Equal(t, []string{"research-monthly"}, forgetter.forgotten, "a delete on any replica forgets the budget here")
	assert.Equal(t, []string{"publish:c-CREATE", "publish:c-UPDATE", "publish:c-del"}, budgets.calls)
}

type recordingSpend struct{ deleted []string }

func (r *recordingSpend) Delete(_ context.Context, namespace, idPrefix string) error {
	r.deleted = append(r.deleted, namespace+"/"+idPrefix)
	return nil
}

func TestHandleEvent_Budget_DeleteClearsSpendKeptInThisReplicasMemory(t *testing.T) {
	spend := &recordingSpend{}
	listener := &EventListener{logger: newTestLogger(), budgets: &fakeBudgetPublisher{}}
	listener.SetLocalSpend(spend)

	listener.handleEvent(eventhub.Event{EventType: eventhub.EventTypeBudget, Action: "UPDATE", EntityID: "team-a", EventID: "c-1"})
	assert.Empty(t, spend.deleted, "an update keeps the spend")

	listener.handleEvent(eventhub.Event{EventType: eventhub.EventTypeBudget, Action: "DELETE", EntityID: "team-a", EventID: "c-2"})
	prefix := budgetspend.CounterIDPrefix("team-a")
	assert.Equal(t, []string{budgetspend.NamespaceSpend + "/" + prefix, budgetspend.NamespaceUnpriced + "/" + prefix}, spend.deleted,
		"a delete on another replica clears this replica's own spend for the budget")
}

func TestHandleEvent_Budget_DeleteLeavesASharedStoreAlone(t *testing.T) {
	listener := &EventListener{logger: newTestLogger(), budgets: &fakeBudgetPublisher{}}
	// No local spend set (Redis): nothing to clear here, and no panic.
	listener.handleEvent(eventhub.Event{EventType: eventhub.EventTypeBudget, Action: "DELETE", EntityID: "team-a", EventID: "c-1"})
}
