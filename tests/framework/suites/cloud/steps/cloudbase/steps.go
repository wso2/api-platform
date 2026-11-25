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
	"context"

	"github.com/cucumber/godog"

	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// NewSteps returns the shared step core for one resolved block.
func NewSteps(topo *frameworkruntime.Topology, funnel *httpx.Funnel) *Steps {
	return &Steps{Topo: topo, Funnel: funnel, Client: funnel.Client()}
}

// RegisterScenarioProject records the project of each scenario's feature file before the scenario
// runs, so the steps that find the project need no setting of their own.
func RegisterScenarioProject(sc *godog.ScenarioContext) {
	sc.Before(func(ctx context.Context, scenario *godog.Scenario) (context.Context, error) {
		return ctx, tcontext.Set(ctx, KeyScenarioProject, ProjectOfFeature(scenario.Uri))
	})
}
