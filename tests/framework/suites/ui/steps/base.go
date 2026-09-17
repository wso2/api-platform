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

// Package steps holds suite-wide UI lifecycle and step definitions for godog features
// driving a real browser on the block's network.
package steps

import (
	"context"

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/coverage"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/suites/ui/steps/aiworkspace"
	"github.com/wso2/api-platform/tests/framework/suites/ui/steps/apiportal"
)

// assertTimeoutMs bounds every web-first assertion. Generous over a fresh SPA load,
// still far below any scenario timeout.
const assertTimeoutMs = 15000

const keyBrowserCoverageReset = "uiBrowserCoverageReset"

// UI holds what the UI steps need.
type UI struct {
	topo         *frameworkruntime.Topology
	expect       playwright.PlaywrightAssertions
	coverageSink *coverage.Sink
}

// New builds the step set for one block's topology.
func New(topo *frameworkruntime.Topology, coverageSink *coverage.Sink) *UI {
	return &UI{
		topo:         topo,
		expect:       playwright.NewPlaywrightAssertions(assertTimeoutMs),
		coverageSink: coverageSink,
	}
}

// Register wires the browser lifecycle and every step this suite provides.
func (u *UI) Register(sc *godog.ScenarioContext) {
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		ctx, err := u.openScenarioPage(ctx)
		if err != nil {
			return ctx, err
		}
		if u.coverageSink == nil || tcontext.Contains(ctx, keyBrowserCoverageReset) {
			return ctx, nil
		}
		if err := u.resetBrowserCoverage(ctx); err != nil {
			return ctx, err
		}
		if err := tcontext.Set(ctx, keyBrowserCoverageReset, true); err != nil {
			return ctx, err
		}
		return ctx, nil
	})
	sc.After(func(ctx context.Context, scn *godog.Scenario, scenarioErr error) (context.Context, error) {
		if scenarioErr != nil {
			name := artifactBaseName(scn.Name)
			u.saveFailureArtifacts(ctx, name)
			u.stopTracing(ctx, name, true)
		} else {
			u.stopTracing(ctx, "", false)
		}
		coverageErr := u.collectBrowserCoverage(ctx, scn.Name)
		u.closeScenarioPage(ctx)
		if coverageErr != nil {
			return ctx, coverageErr
		}
		// The step's own failure is already recorded; returning it again would re-report
		// it as a hook failure.
		return ctx, nil
	})

	apiportal.New(u.topo, u.expect, u.page).Register(sc)

	aiworkspace.New(u.topo, u.expect, u.page, markSensitiveArtifacts, u.browserFor,
		u.closeScenarioPage, newAppPage, u.coverageSink != nil).Register(sc)
}
