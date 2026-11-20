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
package steps

import (
	"fmt"

	"github.com/cucumber/godog"

	corecatalog "github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/access"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/deploy"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/gateway"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/graphql"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/portal"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/project"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/promotion"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/restapi"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/secrets"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/selfhosted"
)

// Register binds the APIP cloud steps of every area for one resolved block.
func Register(sc *godog.ScenarioContext, topo *frameworkruntime.Topology, funnel *httpx.Funnel) {
	cloudbase.RegisterScenarioProject(sc)
	registerAreas(sc, cloudbase.NewSteps(topo, funnel))
}

// StepDefinitions lists every step pattern Register binds, with its handler.
func StepDefinitions() []cloudbase.StepDefinition {
	recorder := &cloudbase.StepRecorder{}
	registerAreas(recorder, &cloudbase.Steps{})
	return recorder.Definitions
}

func registerAreas(sc cloudbase.StepRegistrar, base *cloudbase.Steps) {
	base.RegisterSteps(sc)
	access.Register(sc, base)
	project.Register(sc, base)
	restapi.Register(sc, base)
	graphql.Register(sc, base)
	deploy.Register(sc, base)
	promotion.Register(sc, base)
	gateway.Register(sc, base)
	selfhosted.Register(sc, base)
	secrets.Register(sc, base)
	portal.Register(sc, base)
}

// RegisterDeleters installs cloud-target cleanup handlers for a block containing cloud-console.
func RegisterDeleters(reg *cleanup.Registry, topo *frameworkruntime.Topology) error {
	if reg == nil {
		return fmt.Errorf("cloud-console: cleanup registry is required")
	}
	if topo == nil {
		return fmt.Errorf("cloud-console: topology is required")
	}
	if _, err := topo.Component(corecatalog.Name); err != nil {
		return nil
	}
	client := httpx.NewClient(httpx.Options{MaxRetries: 1})
	for _, install := range []func(*cleanup.Registry, *frameworkruntime.Topology, *httpx.Client) error{
		restapi.RegisterDeleters, secrets.RegisterDeleters, gateway.RegisterDeleters,
		graphql.RegisterDeleters, portal.RegisterDeleters, selfhosted.RegisterDeleters,
	} {
		if err := install(reg, topo, client); err != nil {
			return err
		}
	}
	return nil
}
