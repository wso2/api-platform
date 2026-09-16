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

package platformgateway

import (
	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
)

// Register binds all platform-gateway-specific steps.
func Register(sc *godog.ScenarioContext, topo *runtime.Topology, funnel *httpx.Funnel) {
	s := &Steps{topo: topo, funnel: funnel}
	sc.Step(`^I send a GET request to the gateway controller admin health endpoint$`, s.controllerHealth)
	sc.Step(`^I send a GET request to the router ready endpoint$`, s.routerReady)
	sc.Step(`^I send a GET request to the router ready endpoint until status (\d+)$`, s.routerReadyUntil)
	sc.Step(`^I send a GET request to the policy engine health endpoint$`, s.policyEngineHealth)
	sc.Step(`^I check the health of all gateway services$`, s.checkAllHealth)
	sc.Step(`^I stop the gateway service "([^"]*)"$`, s.stopService)
	sc.Step(`^I start the gateway service "([^"]*)"$`, s.startService)
	sc.Step(`^I restart the "([^"]*)" service$`, s.restartService)
	sc.Step(`^I wait for the gateway controller health endpoint$`, s.waitForController)
	sc.Step(`^the response should indicate healthy status$`, s.responseIndicatesHealthy)
	sc.Step(`^the health check should report service "([^"]*)" as unhealthy$`, s.serviceUnhealthy)
	sc.Step(`^all services should report healthy status$`, s.allServicesHealthy)
	sc.Step(`^the resource creation response should indicate successful deployment$`, s.resourceCreationSucceeded)
	sc.Step(`^the API update response should indicate successful deployment$`, s.apiUpdateSucceeded)
	sc.Step(`^the API retrieval response should describe API "([^"]*)" at context "([^"]*)"$`,
		s.apiRetrievalSucceeded)
}
