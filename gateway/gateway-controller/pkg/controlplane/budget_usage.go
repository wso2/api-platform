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
	"log/slog"
	"maps"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/wso2/api-platform/gateway/common/counterproto"
)

// Budget spend reaches the control plane the way deployment acknowledgements do: a
// usage message on the control-plane connection (Interfaces 4.2). The budget system
// policy labels each allowance's counters with the budget and the allocated values;
// the counter stream tells BudgetUsageReporter of every applied report, and it sends the
// allowances' totals about every second. Every minute it sends again every total written
// in the last ten minutes, so a lost message heals itself. While disconnected it keeps
// only the latest total per allowance and period.

// Counter namespaces the budget system policy charges: budget spend, and the AI cost
// stats ([ai_cost_stats]), which count every request's cost by its values.
const (
	usageNamespaceSpend    = "budget"
	usageNamespaceUnpriced = "budget-unpriced"
	usageLabelBudget       = "budget"

	usageNamespaceCost                 = "ai-cost"
	usageNamespaceCostMissingPrice     = "ai-cost-missing-price"
	usageNamespaceCostUsageUnavailable = "ai-cost-usage-unavailable"
)

// Kinds of usage entries.
const (
	usageKindAllowance = "allowance"
	usageKindCost      = "cost"
)

const (
	usageSendInterval   = time.Second
	usageResendInterval = time.Minute
	usageResendWindow   = 10 * time.Minute
	// usageMaxPerMessage bounds one usage message.
	usageMaxPerMessage = 500
)

// BudgetUsageMessage is the usage message.
type BudgetUsageMessage struct {
	Type    string             `json:"type"`
	Payload BudgetUsagePayload `json:"payload"`
}

// BudgetUsagePayload carries allowance totals and cost totals.
type BudgetUsagePayload struct {
	Allowances []AllowanceUsage `json:"allowances,omitempty"`
	Costs      []CostUsage      `json:"costs,omitempty"`
}

// CostUsage is what one combination of values cost in one month, across every gateway
// sharing the counter store (Interfaces 4.2). The control plane keeps the higher totals.
type CostUsage struct {
	// Month is the month's first day, UTC.
	Month string `json:"month"`
	// CostKey keys the combination; the other fields are its values, empty when unknown.
	CostKey                  string `json:"costKey"`
	Project                  string `json:"project,omitempty"`
	Agent                    string `json:"agent,omitempty"`
	APIKey                   string `json:"apiKey,omitempty"`
	User                     string `json:"user,omitempty"`
	LLMProvider              string `json:"llmProvider"`
	Model                    string `json:"model,omitempty"`
	Cost                     string `json:"cost"`
	MissingPriceRequests     int64  `json:"missingPriceRequests"`
	UsageUnavailableRequests int64  `json:"usageUnavailableRequests"`
}

// AllowanceUsage is one allowance's total in one period, across every gateway sharing
// the counter store. The control plane keeps the higher total.
type AllowanceUsage struct {
	// Budget is the budget's name; spend is keyed by name on every gateway.
	Budget string `json:"budget"`
	// PeriodStart is the period's first day, UTC.
	PeriodStart string `json:"periodStart"`
	// AllowanceUUID keys the allowance; Allowance holds the values it is separated by.
	AllowanceUUID    string            `json:"allowanceUuid"`
	Allowance        map[string]string `json:"allowance"`
	Spent            string            `json:"spent"`
	UnpricedRequests int64             `json:"unpricedRequests"`
}

// usageKey names one allowance in one period, or one cost combination in one month.
type usageKey struct {
	kind      string
	allowance string
	window    int64
}

type usageEntry struct {
	budget    string
	allowance map[string]string // an allowance's values, or a cost combination's
	spent     float64
	unpriced  float64
	// missingPrice and usageUnavailable count a cost combination's requests charged $0.
	missingPrice     float64
	usageUnavailable float64
	writtenAt        time.Time
	dirty            bool
}

// usageKindOf returns the kind of entry a counter namespace feeds, or "" for others.
func usageKindOf(namespace string) string {
	switch namespace {
	case usageNamespaceSpend, usageNamespaceUnpriced:
		return usageKindAllowance
	case usageNamespaceCost, usageNamespaceCostMissingPrice, usageNamespaceCostUsageUnavailable:
		return usageKindCost
	}
	return ""
}

// BudgetUsageReporter collects allowance totals from the counter stream and sends them
// to the control plane. It implements counters.Observer.
type BudgetUsageReporter struct {
	mu      sync.Mutex
	entries map[usageKey]*usageEntry
	now     func() time.Time
	logger  *slog.Logger
}

// NewBudgetUsageReporter returns an empty reporter.
func NewBudgetUsageReporter(logger *slog.Logger) *BudgetUsageReporter {
	return &BudgetUsageReporter{entries: map[usageKey]*usageEntry{}, now: time.Now, logger: logger}
}

// Observe records the totals of budget allowances: an allowance is known once an add
// labels it, and its totals are taken from every later report, including spend from
// other gateways sharing the store.
func (r *BudgetUsageReporter) Observe(adds []counterproto.Add, totals []counterproto.Total) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, add := range adds {
		kind := usageKindOf(add.Key.Namespace)
		if kind == "" || len(add.Labels) == 0 || (kind == usageKindAllowance && add.Labels[usageLabelBudget] == "") {
			continue
		}
		key := usageKey{kind, add.Key.ID, add.Key.Window}
		e := r.entries[key]
		if e == nil {
			e = &usageEntry{}
			r.entries[key] = e
		}
		e.allowance = maps.Clone(add.Labels)
		if kind == usageKindAllowance {
			e.budget = add.Labels[usageLabelBudget]
			delete(e.allowance, usageLabelBudget)
		}
	}
	now := r.now()
	for _, t := range totals {
		kind := usageKindOf(t.Key.Namespace)
		if kind == "" {
			continue
		}
		e := r.entries[usageKey{kind, t.Key.ID, t.Key.Window}]
		if e == nil {
			continue
		}
		field := &e.spent
		switch t.Key.Namespace {
		case usageNamespaceUnpriced:
			field = &e.unpriced
		case usageNamespaceCostMissingPrice:
			field = &e.missingPrice
		case usageNamespaceCostUsageUnavailable:
			field = &e.usageUnavailable
		}
		if t.Total != *field {
			*field = t.Total
			e.dirty, e.writtenAt = true, now
		}
	}
}

// run sends changed totals every second and every recent total every minute until done
// is closed. send fails while the control plane is not connected; the totals are then
// kept and sent once it is.
func (r *BudgetUsageReporter) run(done <-chan struct{}, send func([]byte) error) {
	sendTicker := time.NewTicker(usageSendInterval)
	resendTicker := time.NewTicker(usageResendInterval)
	defer sendTicker.Stop()
	defer resendTicker.Stop()
	for {
		select {
		case <-done:
			return
		case <-resendTicker.C:
			r.markRecent()
		case <-sendTicker.C:
			r.flush(send)
		}
	}
}

// markRecent marks every total written in the last ten minutes for sending again and
// forgets older ones.
func (r *BudgetUsageReporter) markRecent() {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	for key, e := range r.entries {
		switch {
		case now.Sub(e.writtenAt) <= usageResendWindow:
			e.dirty = true
		case !e.dirty:
			delete(r.entries, key)
		}
	}
}

// flush sends the changed totals; on failure they stay marked and are sent later.
func (r *BudgetUsageReporter) flush(send func([]byte) error) {
	type pending struct {
		key  usageKey
		sent usageEntry
	}
	r.mu.Lock()
	var batch []pending
	for key, e := range r.entries {
		if e.dirty {
			batch = append(batch, pending{key: key, sent: *e})
		}
	}
	r.mu.Unlock()

	for start := 0; start < len(batch); start += usageMaxPerMessage {
		end := min(start+usageMaxPerMessage, len(batch))
		var payload BudgetUsagePayload
		for _, p := range batch[start:end] {
			e, day := p.sent, time.Unix(p.key.window, 0).UTC().Format(time.DateOnly)
			if p.key.kind == usageKindCost {
				payload.Costs = append(payload.Costs, CostUsage{
					Month: day, CostKey: p.key.allowance,
					Project: e.allowance["project"], Agent: e.allowance["agent"], APIKey: e.allowance["apiKey"],
					User: e.allowance["user"], LLMProvider: e.allowance["llmProvider"], Model: e.allowance["model"],
					Cost: formatUSD(e.spent), MissingPriceRequests: int64(e.missingPrice),
					UsageUnavailableRequests: int64(e.usageUnavailable),
				})
				continue
			}
			payload.Allowances = append(payload.Allowances, AllowanceUsage{
				Budget: e.budget, PeriodStart: day, AllowanceUUID: p.key.allowance,
				Allowance: maps.Clone(e.allowance), Spent: formatUSD(e.spent), UnpricedRequests: int64(e.unpriced),
			})
		}
		data, err := json.Marshal(BudgetUsageMessage{Type: "usage", Payload: payload})
		if err != nil {
			r.logger.Error("Failed to encode usage", slog.Any("error", err))
			return
		}
		if err := send(data); err != nil {
			r.logger.Debug("Usage not sent; will retry", slog.Any("error", err))
			return
		}
		r.mu.Lock()
		for _, p := range batch[start:end] {
			// A total that changed while sending is still marked.
			if e := r.entries[p.key]; e != nil && e.spent == p.sent.spent && e.unpriced == p.sent.unpriced &&
				e.missingPrice == p.sent.missingPrice && e.usageUnavailable == p.sent.usageUnavailable {
				e.dirty = false
			}
		}
		r.mu.Unlock()
	}
}

// SetBudgetUsageReporter makes the client send budget spend to the control plane. Call
// before Start.
func (c *Client) SetBudgetUsageReporter(r *BudgetUsageReporter) {
	c.budgetUsage = r
}

// sendBudgetUsage sends a usage message; only Agent Manager control planes take it.
func (c *Client) sendBudgetUsage(data []byte) error {
	if c.IsOnPrem() {
		return nil
	}
	return c.sendMessage(data)
}

// formatUSD writes an amount with at most six decimals, without float noise.
func formatUSD(v float64) string {
	return strconv.FormatFloat(math.Round(v*1e6)/1e6, 'f', -1, 64)
}
