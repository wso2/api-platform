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

import "context"

// Admin services of the gateway's controllers.
const (
	managementControllerAdmin = "gateway-controller-admin"
	xdsControllerAdmin        = "gateway-controller-xds-admin"
)

// xdsControllerAdminEndpoint names the admin endpoint of a controller that feeds the runtime
// apart from the management controller.
const xdsControllerAdminEndpoint = "xds-admin"

// runtimeControllerAdmin names the admin service of the controller that feeds the runtime
// over xDS, and reports whether that is a controller other than the management one.
func (g *Gateway) runtimeControllerAdmin() (string, bool, error) {
	inst, err := g.topo.Component("platform-gateway")
	if err != nil {
		return "", false, err
	}
	if _, separate := inst.Definition().Endpoint(xdsControllerAdminEndpoint); separate {
		return xdsControllerAdmin, true, nil
	}
	return managementControllerAdmin, false, nil
}

// requireRuntimeControllerAgrees tolerates a controller that feeds the runtime apart from
// the management controller while it has not yet received whether an API attaches
// mtls-auth. The management controller's view is the one the runtime must reach.
func (g *Gateway) requireRuntimeControllerAgrees(ctx context.Context, attached bool) error {
	service, separate, err := g.runtimeControllerAdmin()
	if err != nil || !separate {
		return err
	}
	fed, err := g.controllerAttachesMTLSAuth(ctx, service)
	if err != nil {
		return err
	}
	if fed != attached {
		return tolerated("the controller that feeds the runtime attaches mtls-auth: %t, the management controller: %t", fed, attached)
	}
	return nil
}
