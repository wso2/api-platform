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

package budgetspend

import (
	"strings"
	"testing"
	"time"
)

func TestCounterIDsCarryTheBudget(t *testing.T) {
	id := CounterID(SpendID("team-a"), "0b0d4f0e-1111-5222-8333-444455556666")
	if !strings.HasPrefix(id, CounterIDPrefix("team-a")) {
		t.Fatalf("%s does not start with the budget's prefix", id)
	}
	if strings.HasPrefix(id, CounterIDPrefix("team-b")) {
		t.Fatal("another budget's prefix matches")
	}
	if got := Allowance(id); got != "0b0d4f0e-1111-5222-8333-444455556666" {
		t.Fatalf("Allowance = %s", got)
	}
	if SpendID("team-a") != SpendID("team-a") {
		t.Fatal("spend ID is not stable")
	}
}

func TestPeriods(t *testing.T) {
	now := time.Date(2026, 11, 18, 15, 4, 5, 0, time.UTC) // a Wednesday
	cases := []struct{ period, start, end string }{
		{PeriodDay, "2026-11-18", "2026-11-19"},
		{PeriodWeek, "2026-11-16", "2026-11-23"},
		{PeriodMonth, "2026-11-01", "2026-12-01"},
	}
	for _, c := range cases {
		start := PeriodStart(c.period, now)
		if got := start.Format(time.DateOnly); got != c.start {
			t.Errorf("%s start = %s, want %s", c.period, got, c.start)
		}
		if got := PeriodEnd(c.period, start).Format(time.DateOnly); got != c.end {
			t.Errorf("%s end = %s, want %s", c.period, got, c.end)
		}
	}
}

func TestFormatUSD(t *testing.T) {
	if got := FormatUSD(0.1 + 0.2); got != "0.3" {
		t.Fatalf("FormatUSD = %s", got)
	}
}
