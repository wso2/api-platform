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

package it

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

// envoyAdminURL is the Envoy admin endpoint exposed by the IT compose stack.
const envoyAdminURL = "http://localhost:9901"

// envoyAdminClient bounds admin queries so a stalled admin endpoint fails the
// step instead of hanging until the suite timeout.
var envoyAdminClient = &http.Client{Timeout: 10 * time.Second}

// rememberedClusterSets holds cluster-name sets captured during a scenario,
// keyed by name prefix. Cleared before each scenario. Safe under godog's
// default sequential execution; it would need per-scenario state if scenario
// parallelism is ever enabled.
var rememberedClusterSets = map[string][]string{}

// fetchEnvoyClusterNames returns the sorted, de-duplicated set of cluster
// names with the given prefix, parsed from the Envoy admin /clusters output
// (each line has the form "<cluster>::<key>::<value>").
func fetchEnvoyClusterNames(prefix string) ([]string, error) {
	resp, err := envoyAdminClient.Get(envoyAdminURL + "/clusters")
	if err != nil {
		return nil, fmt.Errorf("failed to query Envoy admin /clusters: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Envoy admin /clusters returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Envoy admin /clusters response: %w", err)
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(string(body), "\n") {
		name, _, ok := strings.Cut(line, "::")
		if !ok {
			continue
		}
		if strings.HasPrefix(name, prefix) {
			seen[name] = true
		}
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, nil
}

// RegisterEnvoyAdminSteps registers steps that assert Envoy cluster identity
// via the admin endpoint. Capturing the exact cluster-name set before an API
// update and asserting it is unchanged afterwards proves the cluster NAME
// survived the update; substring checks on /clusters alone cannot prove that
// (an implementation renaming one hashed cluster to another would still pass
// a "contains prefix" check).
func RegisterEnvoyAdminSteps(ctx *godog.ScenarioContext) {
	ctx.Before(func(c context.Context, sc *godog.Scenario) (context.Context, error) {
		rememberedClusterSets = map[string][]string{}
		return c, nil
	})

	ctx.Step(`^I capture the Envoy cluster names prefixed "([^"]*)"$`, func(prefix string) error {
		names, err := fetchEnvoyClusterNames(prefix)
		if err != nil {
			return err
		}
		if len(names) == 0 {
			return fmt.Errorf("no Envoy clusters with prefix %q found to capture", prefix)
		}
		rememberedClusterSets[prefix] = names
		return nil
	})

	// The set is observed over a settle window rather than once: an update
	// propagates to Envoy asynchronously, so a single immediate read could pass
	// against the pre-update state and miss a cluster rename that lands moments
	// later. Any change inside the window fails immediately.
	ctx.Step(`^the Envoy cluster names prefixed "([^"]*)" should be unchanged$`, func(prefix string) error {
		captured, ok := rememberedClusterSets[prefix]
		if !ok {
			return fmt.Errorf("no captured cluster set for prefix %q; use the capture step first", prefix)
		}
		deadline := time.Now().Add(6 * time.Second)
		for {
			current, err := fetchEnvoyClusterNames(prefix)
			if err != nil {
				return err
			}
			if strings.Join(captured, ",") != strings.Join(current, ",") {
				return fmt.Errorf("Envoy cluster set with prefix %q changed across the update: before=%v after=%v (cluster identity must be stable)", prefix, captured, current)
			}
			if time.Now().After(deadline) {
				return nil
			}
			time.Sleep(500 * time.Millisecond)
		}
	})
}
