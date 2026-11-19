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

package access

import "github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"

const cloudConsoleExpiredTokenEnv = "CLOUD_CONSOLE_EXPIRED_ACCESS_TOKEN"

// RegisterIdentitySteps binds the steps that inspect the console token and send requests with
// controlled credentials.
func (s *Steps) RegisterIdentitySteps(sc cloudbase.StepRegistrar) {
	sc.Step(`^the APIP cloud console token should be a JWT that has not expired$`, s.assertTokenIsLiveJWT)
	sc.Step(`^the APIP cloud console token should carry an organisation claim$`, s.assertTokenCarriesOrganisation)
	sc.Step(`^I send an authenticated "([^"]*)" request to the APIP cloud path "([^"]*)"$`, s.sendAuthenticatedRequest)
	sc.Step(`^a "([^"]*)" request to the APIP cloud path "([^"]*)" should give the same result when the "([^"]*)" header names another organisation$`,
		s.assertForeignOrganisationHeaderIgnored)
	sc.Step(`^I send a "([^"]*)" request to the APIP cloud path "([^"]*)" without an Authorization header$`, s.sendWithoutAuthorization)
	sc.Step(`^I send a "([^"]*)" request to the APIP cloud path "([^"]*)" with the Authorization header "([^"]*)"$`, s.sendWithAuthorizationHeader)
	sc.Step(`^I send a "([^"]*)" request to the APIP cloud path "([^"]*)" bearing a token (carrying the real claims and key id with a forged signature|with no signature|signed with an unknown key|signed with an unknown shared secret)$`,
		s.sendWithForgedToken)
	sc.Step(`^I send a "([^"]*)" request to the APIP cloud path "([^"]*)" bearing an expired console token$`, s.sendWithExpiredToken)
	sc.Step(`^the cloud API error response should not expose internal detail$`, s.assertNoInternalDetail)
	sc.Step(`^the cloud API response status code should be a client error$`, s.assertCloudResponseClientError)
	sc.Step(`^the cloud API response body should (contain|not contain) "([^"]*)"$`, s.assertCloudResponseContains)
	sc.Step(`^the cloud API error code should be "([^"]*)"$`, s.assertCloudErrorCode)
}
