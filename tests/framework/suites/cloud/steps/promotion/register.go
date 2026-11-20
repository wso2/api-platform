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

package promotion

import "github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"

// RegisterPromotionSteps binds the steps that find a project bound to a deployment pipeline, choose
// a promotion chain of its pipeline and check what a promotion along it does to each environment.
func (s *Steps) RegisterPromotionSteps(sc cloudbase.StepRegistrar) {
	sc.Step(`^I find the pipeline-bound APIP project$`, s.findPipelineProject)
	sc.Step(`^I find a promotion chain of (\d+) environments with an active gateway each in the project's deployment pipeline$`, s.findPromotionChain)
	sc.Step(`^I create a synthetic REST API in the pipeline-bound APIP project$`, s.createAPIInPipelineProject)
	sc.Step(`^the deployment stage of pipeline environment (\d+) should show no gateway as deployed$`, s.assertStageHoldsNothing)
	sc.Step(`^the deployment response should list the gateway of pipeline environment (\d+)$`, s.assertDeploymentListsGatewayOf)
	sc.Step(`^the deployment stage of pipeline environment (\d+) should show its gateway running the rendered build$`, s.assertStageRunsRenderedBuild)
	sc.Step(`^the deployment stage of pipeline environment (\d+) should show its gateway as no longer deployed$`, s.assertStageNoLongerDeployed)
	sc.Step(`^I undeploy the promoted deployment from pipeline environment (\d+)$`, s.undeployFromPipelineEnvironment)
	sc.Step(`^the synthetic REST API should be DEPLOYED on every gateway the deployment request named$`, s.waitDeployedOnTargets)
	sc.Step(`^the synthetic REST API should hold (\d+) builds$`, s.assertBuildCount)
}
