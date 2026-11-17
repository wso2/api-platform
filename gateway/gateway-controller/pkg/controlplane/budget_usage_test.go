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
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/common/counterproto"
)

func usageReporter(now *time.Time) *BudgetUsageReporter {
	r := NewBudgetUsageReporter(slog.New(slog.NewTextHandler(io.Discard, nil)))
	r.now = func() time.Time { return *now }
	return r
}

// sent decodes what flush sent.
type sentUsage struct{ messages []BudgetUsageMessage }

func (s *sentUsage) send(data []byte) error {
	var m BudgetUsageMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	s.messages = append(s.messages, m)
	return nil
}

func TestBudgetUsage_ReportsLabelledAllowanceTotals(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	r := usageReporter(&now)
	window := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	spend := counterproto.KeyFor("budget", "a-1", window)
	unpriced := counterproto.KeyFor("budget-unpriced", "a-1", window)
	labels := map[string]string{"budget": "per-agent", "agent": "api-platform/support-agent"}
	other := counterproto.KeyFor("ratelimit", "x", time.Time{})

	r.Observe([]counterproto.Add{
		{Key: spend, Delta: 0.5, Labels: labels},
		{Key: unpriced, Delta: 1, Labels: labels},
		{Key: other, Delta: 1},
	}, []counterproto.Total{{Key: spend, Total: 80.14000000000001}, {Key: unpriced, Total: 3}, {Key: other, Total: 9}})

	s := &sentUsage{}
	r.flush(s.send)
	require.Len(t, s.messages, 1)
	assert.Equal(t, "budget.usage", s.messages[0].Type)
	assert.Equal(t, []AllowanceUsage{{
		Budget: "per-agent", PeriodStart: "2026-11-01", AllowanceUUID: "a-1",
		Allowance: map[string]string{"agent": "api-platform/support-agent"},
		Spent:     "80.14", UnpricedRequests: 3,
	}}, s.messages[0].Payload.Allowances)

	r.flush(s.send)
	assert.Len(t, s.messages, 1, "unchanged totals are not sent again")

	// Spend from another gateway arrives with a later report without adds for it.
	r.Observe(nil, []counterproto.Total{{Key: spend, Total: 95}})
	r.flush(s.send)
	require.Len(t, s.messages, 2)
	assert.Equal(t, "95", s.messages[1].Payload.Allowances[0].Spent)
}

func TestBudgetUsage_KeepsTotalsUntilSentAndResendsRecentOnes(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	r := usageReporter(&now)
	spend := counterproto.KeyFor("budget", "a-1", time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC))
	r.Observe([]counterproto.Add{{Key: spend, Delta: 1, Labels: map[string]string{"budget": "daily"}}},
		[]counterproto.Total{{Key: spend, Total: 1}})

	r.flush(func([]byte) error { return errors.New("not connected") })
	s := &sentUsage{}
	r.flush(s.send)
	require.Len(t, s.messages, 1, "a total that failed to send is sent later")
	assert.Empty(t, s.messages[0].Payload.Allowances[0].Allowance, "a shared budget's allowance has no values")

	now = now.Add(5 * time.Minute)
	r.markRecent()
	r.flush(s.send)
	assert.Len(t, s.messages, 2, "totals written in the last ten minutes are sent again every minute")

	now = now.Add(10 * time.Minute)
	r.markRecent()
	r.flush(s.send)
	assert.Len(t, s.messages, 2, "older totals are not resent")
	assert.Empty(t, r.entries, "and are forgotten")
}

func TestBudgetUsage_IgnoresUnlabelledCounters(t *testing.T) {
	now := time.Now()
	r := usageReporter(&now)
	spend := counterproto.KeyFor("budget", "a-1", time.Time{})
	r.Observe([]counterproto.Add{{Key: spend, Delta: 1}}, []counterproto.Total{{Key: spend, Total: 1}})
	assert.Empty(t, r.entries)
}

// AI cost stats: the cost, missing-price and usage-unavailable counters of one combination
// are reported as one costs entry, next to the allowances.
func TestBudgetUsage_ReportsCostTotals(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	r := usageReporter(&now)
	month := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	cost := counterproto.KeyFor("ai-cost", "c-1", month)
	missing := counterproto.KeyFor("ai-cost-missing-price", "c-1", month)
	unavailable := counterproto.KeyFor("ai-cost-usage-unavailable", "c-1", month)
	labels := map[string]string{
		"project": "api-platform", "agent": "support-agent", "apiKey": "support-prod",
		"llmProvider": "openai", "model": "gpt-4o",
	}
	spend := counterproto.KeyFor("budget", "a-1", month)

	r.Observe([]counterproto.Add{
		{Key: cost, Delta: 2, Labels: labels},
		{Key: missing, Delta: 1, Labels: labels},
		{Key: unavailable, Delta: 1, Labels: labels},
		{Key: spend, Delta: 2, Labels: map[string]string{"budget": "all"}},
	}, []counterproto.Total{{Key: cost, Total: 80.14}, {Key: missing, Total: 3}, {Key: unavailable, Total: 1}, {Key: spend, Total: 80.14}})

	s := &sentUsage{}
	r.flush(s.send)
	require.Len(t, s.messages, 1)
	assert.Equal(t, []CostUsage{{
		Month: "2026-11-01", CostKey: "c-1", Project: "api-platform", Agent: "support-agent", APIKey: "support-prod",
		LLMProvider: "openai", Model: "gpt-4o", Cost: "80.14", MissingPriceRequests: 3, UsageUnavailableRequests: 1,
	}}, s.messages[0].Payload.Costs)
	require.Len(t, s.messages[0].Payload.Allowances, 1, "allowances and costs travel together")

	r.Observe(nil, []counterproto.Total{{Key: missing, Total: 4}})
	r.flush(s.send)
	require.Len(t, s.messages, 2)
	assert.Empty(t, s.messages[1].Payload.Allowances)
	assert.Equal(t, int64(4), s.messages[1].Payload.Costs[0].MissingPriceRequests)
}
