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
	"time"

	"github.com/google/uuid"
	"github.com/wso2/api-platform/common/budgetspend"
	"github.com/wso2/api-platform/sdk/core/counters"
)

// Budget periods, as in the Budget kind's spec.limit.period, and their UTC bounds.
const (
	periodDay   = budgetspend.PeriodDay
	periodWeek  = budgetspend.PeriodWeek
	periodMonth = budgetspend.PeriodMonth
)

var (
	periodStart = budgetspend.PeriodStart
	periodEnd   = budgetspend.PeriodEnd
)

// spendKey names one usage in one period.
type spendKey struct {
	spendID     uuid.UUID // the budget's: its counters are found and cleared by it
	usage       string
	periodStart time.Time
}

// Counter namespaces: USD charged, and requests charged $0 because they had no price.
const (
	namespaceSpend    = budgetspend.NamespaceSpend
	namespaceUnpriced = budgetspend.NamespaceUnpriced
)

// counterService is the part of sdk/core/counters the tracker uses.
type counterService interface {
	Add(key counters.Key, delta float64)
	Total(key counters.Key) (float64, bool)
	Describe(key counters.Key, labels map[string]string)
	Forget(namespace, idPrefix string)
}

// installedCounters uses the counter service the policy engine installed: synced with the
// gateway-controller, which keeps the totals in Redis when one is configured, so every
// gateway sharing it sees one total.
type installedCounters struct{}

func (installedCounters) Add(key counters.Key, delta float64)    { counters.Add(key, delta) }
func (installedCounters) Total(key counters.Key) (float64, bool) { return counters.Total(key) }
func (installedCounters) Describe(key counters.Key, labels map[string]string) {
	counters.Describe(key, labels)
}
func (installedCounters) Forget(namespace, idPrefix string) { counters.Forget(namespace, idPrefix) }

// tracker charges spend per usage and period to the shared counters.
type tracker struct {
	counters counterService
}

func newTracker(c counterService) *tracker {
	return &tracker{counters: c}
}

func spendCounter(key spendKey) counters.Key {
	return counters.Key{Namespace: namespaceSpend, ID: budgetspend.CounterID(key.spendID, key.usage), Window: key.periodStart}
}

func unpricedCounter(key spendKey) counters.Key {
	return counters.Key{Namespace: namespaceUnpriced, ID: budgetspend.CounterID(key.spendID, key.usage), Window: key.periodStart}
}

// spentIn returns the USD charged to key so far, across every gateway sharing the store,
// plus this engine's charges not yet reported. Before the shared total first arrives it
// is only this engine's charges: a first request is admitted at once.
func (t *tracker) spentIn(key spendKey) float64 {
	total, _ := t.counters.Total(spendCounter(key))
	return total
}

// unpricedIn returns the requests charged $0 to key so far.
func (t *tracker) unpricedIn(key spendKey) int64 {
	total, _ := t.counters.Total(unpricedCounter(key))
	return int64(total)
}

// charge adds cost to every key, describing each counter so the gateway-controller can
// report the usage's spend to the control plane.
func (t *tracker) charge(keys []admitted, cost float64) {
	for _, key := range keys {
		counter := spendCounter(key.spendKey)
		t.counters.Describe(counter, key.labels)
		t.counters.Add(counter, cost)
	}
}

// countUnpriced records one request charged $0 against every key.
func (t *tracker) countUnpriced(keys []admitted) {
	for _, key := range keys {
		counter := unpricedCounter(key.spendKey)
		t.counters.Describe(counter, key.labels)
		t.counters.Add(counter, 1)
	}
}

// forget drops this engine's copy of a deleted budget's spend, as the gateway-controller
// clears it in the store, so a budget made again under the name starts from nothing.
func (t *tracker) forget(name string) {
	prefix := budgetspend.CounterIDPrefix(name)
	t.counters.Forget(namespaceSpend, prefix)
	t.counters.Forget(namespaceUnpriced, prefix)
}
