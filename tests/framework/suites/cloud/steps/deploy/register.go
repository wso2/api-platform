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

package deploy

import "github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"

const keyEchoBackendURL = "cloudEchoBackendURL"

// RegisterDeploymentSteps binds the build and deployment steps.
func (s *Steps) RegisterDeploymentSteps(sc cloudbase.StepRegistrar) {
	sc.Step(`^I prepare a build of the synthetic REST API with metadata:$`, s.prepareBuild)
	sc.Step(`^the prepared build should have a day-and-index identifier$`, s.assertBuildIDShape)
	sc.Step(`^the synthetic REST API builds should include the prepared build$`, s.assertBuildListed)
	sc.Step(`^the synthetic REST API builds should list the rendered builds newest first$`, s.assertBuildsNewestFirst)
	sc.Step(`^the deployment should have rendered a build of the API$`, s.recordRenderedBuild)
	sc.Step(`^I request a deployment of the synthetic REST API with:$`, s.requestDeployment)
	sc.Step(`^the deployment response should list the active gateway$`, s.assertDeploymentListsActiveGateway)
	sc.Step(`^the deployed build should be the rendered build$`, s.assertDeployedBuildIsPrepared)
	sc.Step(`^the synthetic REST API deployments should have one stage per environment of its project's deployment pipeline$`, s.assertStagesMatchPipeline)
	sc.Step(`^the deployment stage of the active gateway's environment should show the active gateway as ([A-Z_]+)$`, s.assertActiveGatewayStageStatus)
	sc.Step(`^the synthetic REST API deployment should report a status reason code$`, s.assertDeploymentStatusReason)
	sc.Step(`^an echo backend is available for deployment overrides$`, s.useEchoBackend)
	sc.Step(`^the deployment response should report the echo backend as the production endpoint$`, s.assertDeploymentReportsEchoEndpoint)
	sc.Step(`^the echo backend should have received a request for "([^"]*)"$`, s.assertEchoBackendReceived)
}
