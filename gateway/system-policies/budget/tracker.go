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
	"sync"
	"time"
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

// keepPeriods is how long spend is kept after its period started; it covers the longest
// period with room to spare.
const keepPeriods = 40 * 24 * time.Hour

// tracker holds spend in memory: USD charged and requests charged $0 because they had no
// price, per allowance and period. It is shared by every route in this policy engine.
type tracker struct {
	mu        sync.Mutex
	spent     map[spendKey]float64
	unpriced  map[spendKey]int64
	lastPrune time.Time
}

func newTracker() *tracker {
	return &tracker{spent: map[spendKey]float64{}, unpriced: map[spendKey]int64{}}
}

// spentIn returns the USD charged to key so far.
func (t *tracker) spentIn(key spendKey) float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.spent[key]
}

// unpricedIn returns the requests charged $0 to key so far.
func (t *tracker) unpricedIn(key spendKey) int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.unpriced[key]
}

// charge adds cost to every key.
func (t *tracker) charge(keys []spendKey, cost float64, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, key := range keys {
		t.spent[key] += cost
	}
	t.pruneLocked(now)
}

// countUnpriced records one request charged $0 against every key.
func (t *tracker) countUnpriced(keys []spendKey, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, key := range keys {
		t.unpriced[key]++
	}
	t.pruneLocked(now)
}

// pruneLocked drops periods that started more than keepPeriods ago, at most once an hour.
func (t *tracker) pruneLocked(now time.Time) {
	if now.Sub(t.lastPrune) < time.Hour {
		return
	}
	t.lastPrune = now
	cutoff := now.Add(-keepPeriods)
	for key := range t.spent {
		if key.periodStart.Before(cutoff) {
			delete(t.spent, key)
		}
	}
	for key := range t.unpriced {
		if key.periodStart.Before(cutoff) {
			delete(t.unpriced, key)
		}
	}
}
