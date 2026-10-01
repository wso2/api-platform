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
	"fmt"
	"maps"

	"github.com/wso2/api-platform/tests/framework/core/components"
)

const (
	// svcXDSController is the controller that feeds the runtime over xDS when the gateway
	// runs two controllers.
	svcXDSController = "gateway-controller-xds"

	// EndpointXDSControllerAdmin is the admin API of the controller that feeds the runtime,
	// present only when that controller is not the one serving the management API.
	EndpointXDSControllerAdmin = "xds-admin"

	// xdsControllerAdminPort matches resources/controller-xds.toml.
	xdsControllerAdminPort = 9094

	xdsControllerComposeFile = "tests/framework/core/catalog/platformgateway/docker-compose.xds-controller.yaml"
	xdsControllerConfigName  = "controller-xds.toml"
	xdsControllerConfigFile  = "tests/framework/core/catalog/platformgateway/resources/controller-xds.toml"
)

// Validate rejects a controller count the gateway cannot run.
func (w *PlatformGatewayWiring) Validate() error {
	if w.Controllers < 0 || w.Controllers > 2 {
		return fmt.Errorf("controllers must be 1 or 2, got %d", w.Controllers)
	}
	return nil
}

// applyPlatformGatewayWiring selects the two-controller gateway for controllers: 2 and
// keeps the registered definition otherwise.
func applyPlatformGatewayWiring(def *components.Definition, wiring any) (*components.Definition, error) {
	w, ok := wiring.(*PlatformGatewayWiring)
	if !ok {
		return nil, fmt.Errorf("platform-gateway wiring must be %T, got %T", &PlatformGatewayWiring{}, wiring)
	}
	if w.Controllers < 2 {
		return def, nil
	}
	return withXDSController(def)
}

// withXDSController returns a copy of def that runs a second controller on the same
// PostgreSQL database and points the runtime at it. The management API, metrics and every
// other endpoint stay on the first controller.
func withXDSController(def *components.Definition) (*components.Definition, error) {
	if def == nil || def.Compose == nil || def.DB == nil {
		return nil, fmt.Errorf("the two-controller gateway needs a compose-backed definition with storage")
	}
	if _, exists := def.Endpoint(EndpointXDSControllerAdmin); exists {
		return nil, fmt.Errorf("%s already runs a dedicated xDS controller", def)
	}
	if !def.DB.Supports(components.Postgres) {
		return nil, fmt.Errorf("%s does not support postgres, which the two controllers share", def)
	}
	out := *def

	compose := *def.Compose
	compose.ComposeOverrideFiles = append(append([]string(nil), def.Compose.ComposeOverrideFiles...), xdsControllerComposeFile)
	compose.Services = append(append([]string(nil), def.Compose.Services...), svcXDSController)
	compose.StagedFiles = maps.Clone(def.Compose.StagedFiles)
	if compose.StagedFiles == nil {
		compose.StagedFiles = map[string]string{}
	}
	compose.StagedFiles[xdsControllerConfigName] = xdsControllerConfigFile
	compose.Env = maps.Clone(def.Compose.Env)
	compose.CoverageServices = append(append([]components.CoverageService(nil), def.Compose.CoverageServices...),
		components.CoverageService{Name: svcXDSController, Types: []string{"go"}})
	out.Compose = &compose

	out.Endpoints = append(append([]components.Endpoint(nil), def.Endpoints...),
		components.Endpoint{Name: EndpointXDSControllerAdmin, Port: xdsControllerAdminPort, Scheme: "http", Service: svcXDSController})

	// Two controllers share one store only through a database server.
	db := *def.DB
	db.Supported = []components.DBType{components.Postgres}
	db.Schema = map[components.DBType][]string{components.Postgres: def.DB.Schema[components.Postgres]}
	db.SelfMigrates = nil
	out.DB = &db

	return &out, nil
}
