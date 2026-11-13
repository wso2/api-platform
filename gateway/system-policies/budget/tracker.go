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

	"github.com/wso2/api-platform/sdk/core/counters"
)

// Budget periods, as in the Budget kind's spec.limit.period.
const (
	periodDay   = "day"
	periodWeek  = "week"
	periodMonth = "month"
)

// periodStart returns the start of the UTC calendar period that holds now: midnight,
// Monday midnight, or the 1st of the month.
func periodStart(period string, now time.Time) time.Time {
	now = now.UTC()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	switch period {
	case periodWeek:
		return midnight.AddDate(0, 0, -((int(midnight.Weekday()) + 6) % 7))
	case periodMonth:
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	default:
		return midnight
	}
}

// periodEnd returns when the period that starts at start resets.
func periodEnd(period string, start time.Time) time.Time {
	switch period {
	case periodWeek:
		return start.AddDate(0, 0, 7)
	case periodMonth:
		return start.AddDate(0, 1, 0)
	default:
		return start.AddDate(0, 0, 1)
	}
}

// spendKey names one allowance in one period.
type spendKey struct {
	allowance   string
	periodStart time.Time
}

// Counter namespaces: USD charged, and requests charged $0 because they had no price.
const (
	namespaceSpend    = "budget"
	namespaceUnpriced = "budget-unpriced"
)

// counterService is the part of sdk/core/counters the tracker uses.
type counterService interface {
	Add(key counters.Key, delta float64)
	Total(key counters.Key) (float64, bool)
}

// installedCounters uses the counter service the policy engine installed: synced with the
// gateway-controller, which keeps the totals in Redis when one is configured, so every
// gateway sharing it sees one total.
type installedCounters struct{}

func (installedCounters) Add(key counters.Key, delta float64)    { counters.Add(key, delta) }
func (installedCounters) Total(key counters.Key) (float64, bool) { return counters.Total(key) }

// tracker charges spend per allowance and period to the shared counters.
type tracker struct {
	counters counterService
}

func newTracker(c counterService) *tracker {
	return &tracker{counters: c}
}

func spendCounter(key spendKey) counters.Key {
	return counters.Key{Namespace: namespaceSpend, ID: key.allowance, Window: key.periodStart}
}

func unpricedCounter(key spendKey) counters.Key {
	return counters.Key{Namespace: namespaceUnpriced, ID: key.allowance, Window: key.periodStart}
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

// charge adds cost to every key.
func (t *tracker) charge(keys []spendKey, cost float64) {
	for _, key := range keys {
		t.counters.Add(spendCounter(key), cost)
	}
}

// countUnpriced records one request charged $0 against every key.
func (t *tracker) countUnpriced(keys []spendKey) {
	for _, key := range keys {
		t.counters.Add(unpricedCounter(key), 1)
	}
}
