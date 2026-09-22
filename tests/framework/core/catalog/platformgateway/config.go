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
	"time"

	"github.com/wso2/api-platform/tests/framework/core/components"
)

func gatewayVersionedHealthChecks() map[string]components.HealthCheck {
	return map[string]components.HealthCheck{
		"1.1.0": {
			Service:  svcController,
			Endpoint: "admin", Path: "/api/admin/v0.9/health",
			ExpectStatus: 200,
			Timeout:      3 * time.Minute, Interval: 2 * time.Second,
		},
	}
}

func gatewayConfigProfiles() map[string]components.ConfigProfile {
	return map[string]components.ConfigProfile{
		"1.1.0": {
			BaseConfigPath:    "tests/framework/core/catalog/platformgateway/resources/1.1.0/config.toml",
			SharedOverlayPath: "tests/framework/core/catalog/platformgateway/resources/1.1.0/gateway-controller-storage.toml",
		},
		"1.2.0": {
			BaseConfigPath:    "tests/framework/core/catalog/platformgateway/resources/1.2.0/config.toml",
			SharedOverlayPath: "tests/framework/core/catalog/platformgateway/resources/1.2.0/gateway-controller-storage.toml",
		},
	}
}

func gatewayRuntimeConfigProfiles() map[string]components.ConfigProfile {
	profiles := gatewayConfigProfiles()
	for version, profile := range profiles {
		profile.SharedOverlayPath = ""
		profiles[version] = profile
	}
	return profiles
}
