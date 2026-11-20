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
	"path"
	"slices"
	"strings"
)

const (
	ProjectOne = "apip-test-project-1"
	ProjectTwo = "apip-test-project-2"
)

// KeyScenarioProject holds the project the scenario's feature file works in, recorded before each
// scenario so the steps that find the project need no setting of their own.
const KeyScenarioProject = "cloudScenarioProject"

// SuiteProjectIDs lists the projects the suite works in, in the order a report names them.
var SuiteProjectIDs = []string{ProjectOne, ProjectTwo}

// projectOneFeatures are the feature files, one for each runner, that work in projectOne: the runners
// apip-restapi, apip-deploy, apip-gateway, apip-gateway-config, apip-graphql, apip-secrets and
// apip-self-hosted-gateway-runtime.
var projectOneFeatures = []string{
	"apip_restapi.feature",
	"apip_deploy.feature",
	"apip_gateway.feature",
	"apip_gateway_config.feature",
	"apip_graphql.feature",
	"apip_secrets.feature",
	"apip_self_hosted_gateway_runtime.feature",
}

// ProjectOfFeature returns the project the scenarios of a feature file work in.
func ProjectOfFeature(uri string) string {
	if slices.Contains(projectOneFeatures, path.Base(strings.ReplaceAll(uri, "\\", "/"))) {
		return ProjectOne
	}
	return ProjectTwo
}

// SuiteProjects returns the ids of the projects the suite works in.
func SuiteProjects() []string { return slices.Clone(SuiteProjectIDs) }
