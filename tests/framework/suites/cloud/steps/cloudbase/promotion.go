/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cloudbase

import (
	"fmt"
	"slices"
	"strings"
)

// NumberedKey names the nth of a numbered set of context keys.
func NumberedKey(prefix string, n int) string { return fmt.Sprintf("%s%d", prefix, n) }

// InEnvironment rejects a gateway of any environment but the given one.
func InEnvironment(environment string) GatewayFilter {
	return func(item map[string]any) string {
		if got, _ := item["environment"].(string); strings.TrimSpace(got) != environment {
			return fmt.Sprintf("in environment %q, not %q", got, environment)
		}
		return ""
	}
}

// PromotionChain finds, in a pipeline's promotion paths, length environments a build passes
// through one after the other. The first is an entry environment: no path leads to it, so it takes
// a deployment directly. Each next one is a target of the one before it, and of no earlier one, so
// a promotion that skips an environment of the chain has no path. usable reports whether an
// environment may be part of the chain. It returns nil when the paths hold no such chain.
func PromotionChain(paths []PromotionPath, length int, usable func(environment string) bool) []string {
	targets := map[string][]string{}
	isTarget := map[string]bool{}
	var order []string
	note := func(environment string) {
		if !slices.Contains(order, environment) {
			order = append(order, environment)
		}
	}
	for _, path := range paths {
		source := strings.TrimSpace(path.SourceEnvironment)
		if source == "" {
			continue
		}
		note(source)
		for _, target := range path.TargetEnvironments {
			if target = strings.TrimSpace(target); target == "" {
				continue
			}
			note(target)
			isTarget[target] = true
			if !slices.Contains(targets[source], target) {
				targets[source] = append(targets[source], target)
			}
		}
	}
	targetOfAny := func(sources []string, environment string) bool {
		for _, source := range sources {
			if slices.Contains(targets[source], environment) {
				return true
			}
		}
		return false
	}
	var extend func(chain []string) []string
	extend = func(chain []string) []string {
		if len(chain) == length {
			return chain
		}
		for _, next := range targets[chain[len(chain)-1]] {
			if !usable(next) || slices.Contains(chain, next) || targetOfAny(chain[:len(chain)-1], next) {
				continue
			}
			if found := extend(append(slices.Clone(chain), next)); found != nil {
				return found
			}
		}
		return nil
	}
	for _, entry := range order {
		if isTarget[entry] || !usable(entry) {
			continue
		}
		if found := extend([]string{entry}); found != nil {
			return found
		}
	}
	return nil
}
