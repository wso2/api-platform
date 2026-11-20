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

// RegisterSteps binds the steps that several areas share.
func (s *Steps) RegisterSteps(sc StepRegistrar) {
	sc.Step(`^I obtain an APIP cloud console token$`, s.ObtainToken)
	sc.Step(`^I find the APIP test project$`, s.FindTestProject)
	sc.Step(`^I find an active APIP gateway$`, s.FindActiveGateway)
	sc.Step(`^I create a synthetic REST API in the APIP test project$`, s.CreateDefaultAPI)
	sc.Step(`^I create a synthetic REST API in the APIP test project with:$`, s.CreateAPIWithValues)
	sc.Step(`^I create a new version "([^"]*)" of the synthetic REST API with:$`, s.CreateNewAPIVersion)
	sc.Step(`^I submit a synthetic REST API creation request with:$`, s.SubmitAPICreation)
	sc.Step(`^the cloud API response status code should be (\d+)$`, s.AssertCloudResponseStatus)
	sc.Step(`^I update the synthetic REST API with:$`, s.UpdateAPI)
	sc.Step(`^I deploy the (synthetic REST API|new API version|synthetic GraphQL API) to the active gateway$`, s.DeployAPI)
	sc.Step(`^I redeploy the synthetic REST API to the active gateway$`, s.RedeployAPI)
	sc.Step(`^I undeploy the (synthetic REST API|synthetic GraphQL API)$`, s.UndeployAPI)
	sc.Step(`^the (synthetic REST API|new API version|synthetic GraphQL API) deployment should become (DEPLOYED|UNDEPLOYED|FAILED)$`, s.WaitDeployment)
	sc.Step(`^I send a "([^"]*)" request to "([^"]*)" with header "([^"]*)" set to the (API key|previous API key|subscription token|previous subscription token|access token|access token with a forged signature) until status (\d+)$`, s.SendWithCredentialUntilStatus)
	sc.Step(`^I send a "([^"]*)" request to "([^"]*)" until status (\d+)$`, s.SendUntilStatus)
	sc.Step(`^the synthetic REST API data-plane response should contain post (\d+)$`, s.AssertAPIResponse)
	sc.Step(`^I delete the synthetic REST API$`, s.DeleteAPI)
	sc.Step(`^the cloud API response should report:$`, s.AssertResponseReports)
	sc.Step(`^I select an existing APIP environment for a (?:managed|self-hosted) gateway$`, s.SelectGatewayEnvironment)
}
