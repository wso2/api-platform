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

// Package budgetspend names budget spend in the shared counters, for the budget system
// policy that counts it and the gateway-controller that reports and clears it.
package budgetspend

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Counter namespaces: USD charged, and requests charged $0 because they had no price.
const (
	NamespaceSpend    = "budget"
	NamespaceUnpriced = "budget-unpriced"
)

// spendNamespace roots the IDs spend is counted under. Never change it: every gateway
// sharing a counter store must derive the same IDs.
var spendNamespace = uuid.MustParse("8d740d5b-3b73-4e13-84a6-5336674f813f")

// SpendID roots a budget's counters: a UUIDv5 of its name, so the same budget on every
// gateway counts against one total whatever UUID each gateway gave it, and the shared
// store's keys hold no names.
func SpendID(name string) uuid.UUID {
	return uuid.NewSHA1(spendNamespace, []byte(name))
}

// CounterID is the ID of one allowance's counters: the budget's spend ID, then the
// allowance, so every counter of a budget can be found (and cleared) by its prefix.
func CounterID(spendID uuid.UUID, allowance string) string {
	return spendID.String() + "/" + allowance
}

// CounterIDPrefix is the prefix of every counter ID of the budget named name.
func CounterIDPrefix(name string) string {
	return SpendID(name).String() + "/"
}

// Allowance returns the allowance UUID of a counter ID; the ID itself when it has none.
func Allowance(counterID string) string {
	if _, allowance, ok := strings.Cut(counterID, "/"); ok {
		return allowance
	}
	return counterID
}

// Budget periods.
const (
	PeriodDay   = "day"
	PeriodWeek  = "week"
	PeriodMonth = "month"
)

// PeriodStart returns the start of the UTC calendar period that holds now: midnight,
// Monday midnight, or the 1st of the month.
func PeriodStart(period string, now time.Time) time.Time {
	now = now.UTC()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	switch period {
	case PeriodWeek:
		return midnight.AddDate(0, 0, -((int(midnight.Weekday()) + 6) % 7))
	case PeriodMonth:
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	default:
		return midnight
	}
}

// PeriodEnd returns when the period that starts at start resets.
func PeriodEnd(period string, start time.Time) time.Time {
	switch period {
	case PeriodWeek:
		return start.AddDate(0, 0, 7)
	case PeriodMonth:
		return start.AddDate(0, 1, 0)
	default:
		return start.AddDate(0, 0, 1)
	}
}

// FormatUSD writes an amount with at most six decimals, without float noise.
func FormatUSD(v float64) string {
	return strconv.FormatFloat(math.Round(v*1e6)/1e6, 'f', -1, 64)
}
