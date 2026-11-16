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

// Counter namespaces the budget system policy charges.
const (
	usageNamespaceSpend    = "budget"
	usageNamespaceUnpriced = "budget-unpriced"
	usageLabelBudget       = "budget"
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

// BudgetUsagePayload carries allowance totals.
type BudgetUsagePayload struct {
	Allowances []AllowanceUsage `json:"allowances"`
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

// usageKey names one allowance in one period.
type usageKey struct {
	allowance string
	window    int64
}

type usageEntry struct {
	budget    string
	allowance map[string]string
	spent     float64
	unpriced  float64
	writtenAt time.Time
	dirty     bool
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
		if !isBudgetNamespace(add.Key.Namespace) || add.Labels[usageLabelBudget] == "" {
			continue
		}
		key := usageKey{add.Key.ID, add.Key.Window}
		e := r.entries[key]
		if e == nil {
			e = &usageEntry{}
			r.entries[key] = e
		}
		e.budget = add.Labels[usageLabelBudget]
		e.allowance = maps.Clone(add.Labels)
		delete(e.allowance, usageLabelBudget)
	}
	now := r.now()
	for _, t := range totals {
		if !isBudgetNamespace(t.Key.Namespace) {
			continue
		}
		e := r.entries[usageKey{t.Key.ID, t.Key.Window}]
		if e == nil {
			continue
		}
		field := &e.spent
		if t.Key.Namespace == usageNamespaceUnpriced {
			field = &e.unpriced
		}
		if t.Total != *field {
			*field = t.Total
			e.dirty, e.writtenAt = true, now
		}
	}
}

func isBudgetNamespace(ns string) bool {
	return ns == usageNamespaceSpend || ns == usageNamespaceUnpriced
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
	r.mu.Lock()
	var batch []AllowanceUsage
	var keys []usageKey
	for key, e := range r.entries {
		if !e.dirty {
			continue
		}
		batch = append(batch, AllowanceUsage{
			Budget:           e.budget,
			PeriodStart:      time.Unix(key.window, 0).UTC().Format(time.DateOnly),
			AllowanceUUID:    key.allowance,
			Allowance:        maps.Clone(e.allowance),
			Spent:            formatUSD(e.spent),
			UnpricedRequests: int64(e.unpriced),
		})
		keys = append(keys, key)
	}
	r.mu.Unlock()

	for start := 0; start < len(batch); start += usageMaxPerMessage {
		end := min(start+usageMaxPerMessage, len(batch))
		data, err := json.Marshal(BudgetUsageMessage{Type: "usage", Payload: BudgetUsagePayload{Allowances: batch[start:end]}})
		if err != nil {
			r.logger.Error("Failed to encode budget usage", slog.Any("error", err))
			return
		}
		if err := send(data); err != nil {
			r.logger.Debug("Budget usage not sent; will retry", slog.Any("error", err))
			return
		}
		r.mu.Lock()
		for i, key := range keys[start:end] {
			// A total that changed while sending is still marked.
			if e := r.entries[key]; e != nil && formatUSD(e.spent) == batch[start+i].Spent &&
				int64(e.unpriced) == batch[start+i].UnpricedRequests {
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
