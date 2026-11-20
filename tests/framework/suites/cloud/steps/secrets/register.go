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

package secrets

import "github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"

// RegisterSecretSteps binds the steps that create, rotate, read and delete a secret and read back
// what a gateway sends upstream with it.
func (s *Steps) RegisterSecretSteps(sc cloudbase.StepRegistrar) {
	sc.Step(`^I create a synthetic secret$`, s.createSecret)
	sc.Step(`^I choose a synthetic secret handle without creating the secret$`, s.chooseSecretHandle)
	sc.Step(`^I rotate the synthetic secret$`, s.rotateSecret)
	sc.Step(`^I read the synthetic secret$`, s.readSecret)
	sc.Step(`^I list the secrets$`, s.listSecrets)
	sc.Step(`^I delete the synthetic secret$`, s.deleteSecret)
	sc.Step(`^the response should not reveal the synthetic secret's value$`, s.assertSecretValueHidden)
	sc.Step(`^the synthetic secret should be described by a hash$`, s.assertSecretHash)
	sc.Step(`^the echoed request header "([^"]*)" should carry the synthetic secret's value$`, s.assertEchoedSecret)
	sc.Step(`^a "([^"]*)" request to "([^"]*)" should echo the header "([^"]*)" with the rotated secret value$`, s.sendUntilEchoesRotated)
}
