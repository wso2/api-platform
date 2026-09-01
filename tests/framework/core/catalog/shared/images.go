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

package shared

import (
	"os"
	"strings"

	"github.com/wso2/api-platform/tests/framework/core/components"
)

// Environment variables that override component images.
//
// The override is a full image reference, tag included:
//
//	IT_IMAGE_GATEWAY_CONTROLLER=ghcr.io/wso2/api-platform/gateway-controller:1.2.0
const (
	EnvImageGatewayController = "IT_IMAGE_GATEWAY_CONTROLLER"
	EnvImageGatewayRuntime    = "IT_IMAGE_GATEWAY_RUNTIME"
	EnvImageMockPlatformAPI   = "IT_IMAGE_MOCK_PLATFORM_API"
)

// EnvCoverageMode marks a run that collects runtime coverage data.
const EnvCoverageMode = "IT_COVERAGE"

// EnvGatewayFunctionalityType sets the functionalityType the running gateway registers with
// on the control plane. Default "regular"; the UI suite sets "ai" so the workspace — which
// lists only AI gateways — can deploy providers to the block's REAL gateway. Per suite, not
// per block: a suite mixing both types needs block wiring, which nothing needs yet.
const EnvGatewayFunctionalityType = "IT_GATEWAY_FUNCTIONALITY_TYPE"

func GatewayFunctionalityType() string {
	if v := strings.TrimSpace(os.Getenv(EnvGatewayFunctionalityType)); v != "" {
		return v
	}
	return "regular"
}

// CoverageMode reports whether this run collects runtime coverage.
func CoverageMode() bool {
	v := strings.TrimSpace(os.Getenv(EnvCoverageMode))
	return strings.EqualFold(v, "true") || v == "1"
}

// image builds an ImageRef whose reference can be overridden by an environment
// variable, falling back to the default the suites normally run.
func Image(envKey, defaultRef string) components.ImageRef {
	if override := strings.TrimSpace(os.Getenv(envKey)); override != "" {
		return components.ImageRef{Ref: override}
	}
	return components.ImageRef{Ref: defaultRef}
}
