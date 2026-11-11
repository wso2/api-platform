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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/catalog"
	"github.com/wso2/api-platform/tests/framework/core/components"
	"github.com/wso2/api-platform/tests/framework/core/topology"
)

const suiteDir = "tests/framework/suites/it"

// fixture writes a small suite directory:
//   - portal-core and portal-billing (blocks/portal.yaml) start the API Portal and run
//     features using the portal steps;
//   - gateway-core (blocks/gateway.yaml, two variants) starts a Gateway and runs a feature
//     using the Gateway steps;
//   - api-core (blocks/api.yaml) starts the Platform API and runs a feature using only
//     shared steps.
//
// The portal steps import the Platform API steps.
func fixture(t *testing.T) *index {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"features/portal/login.feature": "Feature: x\n  Scenario: y\n    When I log in to the portal\n" +
			"    Then I send a request with body:\n      \"\"\"\n      When I am not a step\n      \"\"\"\n",
		"features/portal/billing.feature": "Feature: x\n  Scenario: y\n    Given a portal plan \"gold\"\n" +
			"    When I create it from \"resources/templates/plan.yaml\"\n",
		"features/gateway/route.feature": "Feature: x\n  Scenario: y\n    When I route through the gateway\n",
		"features/api/list.feature":      "Feature: x\n  Scenario Outline: y\n    Then the response has <count> items\n",
		"features/unused.feature":        "Feature: x\n",
		"steps/base.go":                  stepFile("steps", `^I send a request with body:$`, `^I create it from "([^"]*)"$`),
		"steps/common/common.go":         stepFile("common", `^the shared step$`),
		"steps/apiportal/portal.go": `package apiportal

import _ "example/suites/it/steps/platformapi"
` + stepFile("", `^I log in to the portal$`, `^a portal plan "([^"]*)"$`),
		"steps/apiportal/portal_test.go":   "package apiportal\n",
		"steps/platformapi/api.go":         stepFile("platformapi", `^the response has (\d+) items$`),
		"steps/platformgateway/gateway.go": stepFile("platformgateway", `^I route through the gateway$`),
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	block := func(name, source, file string, features []string, started ...string) topology.ResolvedBlock {
		b := topology.ResolvedBlock{Name: name, Source: source, File: file,
			Runners: []topology.Runner{{Name: "r", Features: features}}}
		for _, c := range started {
			b.Components = append(b.Components, topology.ResolvedComponent{Def: &components.Definition{Name: c}})
		}
		return b
	}
	resolved := &topology.Resolved{Blocks: []topology.ResolvedBlock{
		block("portal-core", "portal-core", "blocks/portal.yaml", []string{"features/portal/login.feature"}, "api-portal", "platform-api"),
		block("portal-billing", "portal-billing", "blocks/portal.yaml", []string{"features/portal/billing.feature"}, "api-portal", "platform-api"),
		block("gateway-core/sqlite", "gateway-core", "blocks/gateway.yaml", []string{"features/gateway/route.feature"}, "platform-gateway"),
		block("gateway-core/postgres", "gateway-core", "blocks/gateway.yaml", []string{"features/gateway/route.feature"}, "platform-gateway"),
		block("api-core", "api-core", "blocks/api.yaml", []string{"features/api/list.feature"}, "platform-api"),
	}}
	idx, err := buildIndex(resolved, dir, suiteDir)
	require.NoError(t, err)
	return idx
}

// stepFile is Go source registering patterns, with a package clause unless pkg is empty.
func stepFile(pkg string, patterns ...string) string {
	var b strings.Builder
	if pkg != "" {
		b.WriteString("package " + pkg + "\n")
	}
	b.WriteString("\nfunc register(sc interface{ Step(any, any) }) {\n")
	for _, p := range patterns {
		b.WriteString("\tsc.Step(`" + p + "`, nil)\n")
	}
	b.WriteString("}\n")
	return b.String()
}

func TestAffected(t *testing.T) {
	idx := fixture(t)
	cases := []struct {
		name    string
		changed []string
		blocks  string // "" is every block
		label   string
	}{
		{"a feature file runs the blocks that list it",
			[]string{suiteDir + "/features/portal/login.feature"}, "portal-core", "1 of 4 blocks"},
		{"a feature no block lists runs nothing on its own, so every block",
			[]string{suiteDir + "/features/unused.feature"}, "", "all blocks"},
		{"a block file runs the blocks it declares",
			[]string{suiteDir + "/blocks/portal.yaml"}, "portal-billing,portal-core", "2 of 4 blocks"},
		// api-core counts as a user: its outline step matches no pattern.
		{"a product step package runs its users and one sample block per other block file",
			[]string{suiteDir + "/steps/apiportal/portal.go"},
			"api-core,gateway-core/sqlite,portal-billing,portal-core", "3 of 4 blocks + 1 sample block"},
		{"a package another imports runs the importer's users too",
			[]string{suiteDir + "/steps/platformapi/api.go"},
			"api-core,gateway-core/sqlite,portal-billing,portal-core", "3 of 4 blocks + 1 sample block"},
		{"a sample whose block is already selected is not added again",
			[]string{suiteDir + "/steps/platformgateway/gateway.go"},
			"api-core,gateway-core,portal-core", "2 of 4 blocks + 1 sample block"},
		{"product source runs every block that starts the product",
			[]string{"portals/api-portal/src/app.js"}, "portal-billing,portal-core", "2 of 4 blocks"},
		{"a resource runs the blocks of the features naming it",
			[]string{suiteDir + "/resources/templates/plan.yaml"}, "portal-billing", "1 of 4 blocks"},
		{"a resource no feature names runs every block",
			[]string{suiteDir + "/resources/fixtures/demo.yaml"}, "", "all blocks"},
		{"shared step code runs every block",
			[]string{suiteDir + "/steps/common/common.go"}, "", "all blocks"},
		{"the steps package itself runs every block",
			[]string{suiteDir + "/steps/base.go"}, "", "all blocks"},
		{"a step unit test alone selects nothing, so every block",
			[]string{suiteDir + "/steps/apiportal/portal_test.go"}, "", "all blocks"},
		{"a step unit test beside a feature adds nothing",
			[]string{suiteDir + "/steps/apiportal/portal_test.go", suiteDir + "/features/portal/login.feature"},
			"portal-core", "1 of 4 blocks"},
		{"the suite wiring runs every block, even beside a feature",
			[]string{suiteDir + "/suite_test.go", suiteDir + "/features/portal/login.feature"}, "", "all blocks"},
		{"framework code runs every block",
			[]string{"tests/framework/core/topology/load.go"}, "", "all blocks"},
		{"one shared file among targeted ones runs every block",
			[]string{suiteDir + "/features/portal/login.feature", "common/x.go"}, "", "all blocks"},
		{"several files combine",
			[]string{suiteDir + "/features/portal/login.feature", "gateway/x.go"}, "gateway-core,portal-core", "2 of 4 blocks"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sel := idx.affected(tc.changed)
			require.Equal(t, tc.blocks, sel.blocks())
			require.Equal(t, tc.label, idx.label(sel))
		})
	}
}

func TestFeatureStepsSkipDocStrings(t *testing.T) {
	steps := featureSteps("Feature: x\n  Scenario: y\n    Given a\n    \"\"\"\n    When not a step\n    \"\"\"\n    * b\n    | When | cell |\n")
	require.Equal(t, []string{"a", "b"}, steps)
}

// An outline step no pattern matches makes its feature count as using every package, so a
// change to any of them runs its block.
func TestUnmatchedStepUsesEveryPackage(t *testing.T) {
	idx := fixture(t)
	require.True(t, idx.stepPackageSources["platformgateway"]["api-core"])
}

// The real suite: every feature a block lists exists and its step patterns compile.
func TestRealSuite(t *testing.T) {
	registry, err := catalog.Registry()
	require.NoError(t, err)
	resolved, err := topology.LoadFile("../../suites/it/it-suite.yaml", registry)
	require.NoError(t, err)
	idx, err := buildIndex(resolved, "../../suites/it", suiteDir)
	require.NoError(t, err)

	sel := idx.affected([]string{suiteDir + "/features/apiportal/ai/discovery.feature"})
	require.False(t, sel.all)
	for name := range sel.sources {
		require.True(t, strings.HasPrefix(name, "api-portal"), "an API Portal feature selected %s", name)
	}
}
