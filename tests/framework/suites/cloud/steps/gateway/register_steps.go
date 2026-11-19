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

package gateway

import (
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
)

// Steps holds the step bindings of this package on top of the shared core.
type Steps struct{ *cloudbase.Steps }

// Register binds the steps of this package.
func Register(sc cloudbase.StepRegistrar, base *cloudbase.Steps) {
	s := &Steps{Steps: base}
	s.RegisterGatewaySteps(sc)
	s.RegisterGatewayConfigSteps(sc)
	sc.Step(`^I select an existing APIP environment other than the active gateway's for a managed gateway$`, s.SelectOtherGatewayEnvironment)
}

// RegisterDeleters installs the cleanup handlers of this package's resources.
func RegisterDeleters(reg *cleanup.Registry, topo *frameworkruntime.Topology, client *httpx.Client) error {
	if err := reg.RegisterDeleter(KindDefaultRestore, RestoreDefaultDeleter(topo, client)); err != nil {
		return err
	}
	return reg.RegisterDeleter(cleanup.KindGateway, GatewayDeleter(topo, client))
}
