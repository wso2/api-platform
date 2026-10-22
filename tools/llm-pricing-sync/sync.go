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

package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// deadTierPrefixes hold keys azure-llm-cost never reads: it spells the Global
// tier "global-standard", and LiteLLM's "global" prefix predates it.
var deadTierPrefixes = []string{"azure/global/", "azure_ai/global/"}

// globalStandardPrefix duplicates the base key, which this tool writes at the
// Global rate. Such keys are removed once the base key has been refreshed.
const globalStandardPrefix = "azure/global-standard/"

// staleKeys are entries with no Azure meter behind them. Model router is
// billed as the models it routes to, never as itself.
var staleKeys = []string{"azure_ai/model_router"}

// syncReport lists what applying prices changed.
type syncReport struct {
	added, updated, removed []string
}

func (r syncReport) changed() bool {
	return len(r.added)+len(r.updated)+len(r.removed) > 0
}

// applyPrices writes p into f and prunes the keys the policy cannot use.
func applyPrices(f *pricingFile, p prices) (syncReport, error) {
	var report syncReport
	keys := make([]string, 0, len(p))
	for key := range p {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	refreshed := map[string]bool{}
	for _, key := range keys {
		if err := upsertEntry(f, key, p[key], &report); err != nil {
			return syncReport{}, err
		}
		refreshed[strings.ToLower(key)] = true
	}
	pruneKeys(f, refreshed, &report)
	return report, nil
}

// upsertEntry replaces an entry's managed rates, keeping every other field, or
// adds a new entry carrying only the rates and its provider.
func upsertEntry(f *pricingFile, key string, rates map[string]float64, report *syncReport) error {
	existing, found := f.keyFold(key)
	fields := map[string]json.RawMessage{}
	if found {
		if err := json.Unmarshal(f.entries[existing], &fields); err != nil {
			return fmt.Errorf("decoding pricing entry %q: %w", existing, err)
		}
		for name := range fields {
			if managedFields[name] {
				delete(fields, name)
			}
		}
	} else {
		provider, err := json.Marshal(strings.SplitN(key, "/", 2)[0])
		if err != nil {
			return fmt.Errorf("encoding provider for %q: %w", key, err)
		}
		fields["provider"] = provider
		fields["mode"] = json.RawMessage(`"chat"`)
	}
	for name, rate := range rates {
		fields[name] = json.RawMessage(formatRate(rate))
	}

	raw, err := encodeEntry(fields)
	if err != nil {
		return fmt.Errorf("encoding pricing entry %q: %w", key, err)
	}
	switch {
	case !found:
		f.insert(key, raw)
		report.added = append(report.added, key)
	case string(raw) != string(f.entries[existing]):
		f.entries[existing] = raw
		report.updated = append(report.updated, existing)
	}
	return nil
}

// pruneKeys removes dead and stale keys. A global-standard duplicate goes only
// once its base key was refreshed, so the base is known to hold the Global rate.
func pruneKeys(f *pricingFile, refreshed map[string]bool, report *syncReport) {
	stale := map[string]bool{}
	for _, k := range staleKeys {
		stale[k] = true
	}
	for _, key := range append([]string(nil), f.keys...) {
		remove := stale[key]
		for _, prefix := range deadTierPrefixes {
			remove = remove || strings.HasPrefix(key, prefix)
		}
		if model, ok := strings.CutPrefix(key, globalStandardPrefix); ok {
			remove = remove || refreshed["azure/"+strings.ToLower(model)]
		}
		if remove {
			f.remove(key)
			report.removed = append(report.removed, key)
		}
	}
}
