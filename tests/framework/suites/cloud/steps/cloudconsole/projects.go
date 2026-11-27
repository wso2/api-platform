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

package cloudconsole

import (
	"path"
	"slices"
	"strings"
)

// The suite works in two projects the organisation already holds, each bound to a deployment pipeline
// of its own. It never uses the organisation's default project or default pipeline, which other
// consoles share.
//
// A scenario works in the project of its feature file, and a feature file is one runner:
// projectOne serves the runners that deploy to the organisation's gateway, projectTwo every other
// runner.
const (
	projectOne = "apip-test-project-1"
	projectTwo = "apip-test-project-2"
)

// keyScenarioProject holds the project the scenario's feature file works in, recorded before each
// scenario so the steps that find the project need no setting of their own.
const keyScenarioProject = "cloudScenarioProject"

// suiteProjects lists the projects the suite works in, in the order a report names them.
var suiteProjects = []string{projectOne, projectTwo}

// projectOneFeatures are the feature files that work in projectOne: those of the runners apip-basic,
// apip-deploy (with the managed gateway runtime, route and policy features), apip-gateway, apip-gateway-config,
// apip-graphql, apip-secrets and apip-self-hosted-gateway-runtime.
var projectOneFeatures = []string{
	"apip_basic.feature",
	"apip_deploy.feature",
	"apip_gateway.feature",
	"apip_gateway_config.feature",
	"apip_gateway_runtime.feature",
	"apip_routes.feature",
	"apip_policies.feature",
	"apip_graphql.feature",
	"apip_secrets.feature",
	"apip_self_hosted_gateway_runtime.feature",
}

// projectOfFeature returns the project the scenarios of a feature file work in.
func projectOfFeature(uri string) string {
	if slices.Contains(projectOneFeatures, path.Base(strings.ReplaceAll(uri, "\\", "/"))) {
		return projectOne
	}
	return projectTwo
}

// SuiteProjects returns the ids of the projects the suite works in.
func SuiteProjects() []string { return slices.Clone(suiteProjects) }
