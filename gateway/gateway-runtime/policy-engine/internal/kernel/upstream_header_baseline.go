/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package kernel

import (
	"strings"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// headerBaseline is the value a header held BEFORE any per-attempt chain changed it.
//
// Present distinguishes "the header was there with this value" from "the header was absent",
// which is the difference between restoring a value and removing the header outright. A zero
// headerBaseline therefore means absent, not empty-valued.
type headerBaseline struct {
	Values  []string
	Present bool
}

// headerRestore is one undo operation: put a header back the way the post-downstream request
// had it. Remove is set when it was absent, in which case Values is meaningless.
type headerRestore struct {
	Name   string
	Values []string
	Remove bool
}

// touchedHeaderNames returns the lower-cased names a chain's results changed.
//
// Read from the results the chain already produced rather than diffed from the header map:
// the results ARE the record of what the chain intended, and a diff would additionally pick up
// anything the executor changed for its own reasons.
func touchedHeaderNames(result *executor.RequestHeaderExecutionResult) []string {
	if result == nil {
		return nil
	}
	var names []string
	for _, pr := range result.Results {
		if pr.Skipped || pr.Action == nil {
			continue
		}
		mods, ok := pr.Action.(policy.UpstreamRequestHeaderModifications)
		if !ok {
			continue
		}
		for k := range mods.HeadersToSet {
			names = append(names, strings.ToLower(k))
		}
		for k := range mods.HeadersToAppend {
			names = append(names, strings.ToLower(k))
		}
		for _, k := range mods.HeadersToRemove {
			names = append(names, strings.ToLower(k))
		}
	}
	return names
}

// captureHeaderBaselines records what each named header held in pre.
//
// pre must be the header map as it stood BEFORE the chain ran, since that is the value a later
// attempt has to be restored to. Names already held by the store are skipped by the store's own
// first-touch-wins merge, not here.
func captureHeaderBaselines(pre map[string][]string, names []string) map[string]headerBaseline {
	if len(names) == 0 {
		return nil
	}
	captured := make(map[string]headerBaseline, len(names))
	for _, name := range names {
		if _, seen := captured[name]; seen {
			continue
		}
		values, present := pre[name]
		if !present {
			captured[name] = headerBaseline{}
			continue
		}
		captured[name] = headerBaseline{Values: append([]string(nil), values...), Present: true}
	}
	return captured
}

// headerRestoresFor returns the operations that undo earlier attempts' header changes.
//
// Only headers a per-attempt chain actually touched appear here, which is what keeps the client's
// own headers, the downstream chain's headers, and the ones Envoy manages for itself
// (x-request-id, x-envoy-*, trace propagation) outside this mechanism's reach entirely — no
// exemption list is needed because they are never recorded in the first place (FR-038b).
//
// A header whose current value already matches its baseline produces no operation: the common
// case is a retry onto a provider whose chain sets the same header, and emitting a redundant
// restore there would be undone microseconds later by the chain itself.
func headerRestoresFor(baselines map[string]headerBaseline, current map[string][]string) []headerRestore {
	if len(baselines) == 0 {
		return nil
	}
	var restores []headerRestore
	for name, base := range baselines {
		existing, present := current[name]
		if present == base.Present && equalValues(existing, base.Values) {
			continue
		}
		if !base.Present {
			restores = append(restores, headerRestore{Name: name, Remove: true})
			continue
		}
		restores = append(restores, headerRestore{Name: name, Values: append([]string(nil), base.Values...)})
	}
	return restores
}

// applyHeaderRestores rewrites the in-memory header map so the chain about to run sees the
// post-downstream request rather than the previous attempt's leftovers.
//
// Applied to the map the policies read as well as emitted to Envoy, because a policy that
// inspects a header before deciding must see the same request the provider will receive.
func applyHeaderRestores(values map[string][]string, restores []headerRestore) {
	for _, r := range restores {
		if r.Remove {
			delete(values, r.Name)
			continue
		}
		values[r.Name] = append([]string(nil), r.Values...)
	}
}

func equalValues(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
