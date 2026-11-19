/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package helm

import (
	"fmt"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
)

// checkControllerAuthSupported refuses to install or upgrade a gateway chart that would
// leave the gateway-controller management API without authentication.
//
// Gateway charts that predate the generated controller admin credential have no
// gateway.config.controller.auth.basic.generate default. Given values that list no basic
// auth users (the default since the generated credential was introduced) and do not enable
// IDP auth, such a chart renders basic auth with no users, and the controller then accepts
// every request. This happens when the operator's base values are newer than the chart
// version it is configured to install.
func checkControllerAuthSupported(ch *chart.Chart, values map[string]interface{}) error {
	if ch == nil {
		return nil
	}
	if _, err := chartutil.Values(ch.Values).Table("gateway.config.controller.auth"); err != nil {
		return nil // not a gateway chart with controller auth settings
	}
	if chartGeneratesControllerAdmin(ch.Values) {
		return nil
	}
	merged, err := chartutil.CoalesceValues(ch, values)
	if err != nil {
		return fmt.Errorf("merge values with chart defaults: %w", err)
	}
	auth, _ := merged.Table("gateway.config.controller.auth")
	basic, _ := auth.Table("basic")
	idp, _ := auth.Table("idp")
	users, _ := basic["users"].([]interface{})
	basicEnabled, set := basic["enabled"].(bool)
	if !set {
		basicEnabled = true
	}
	idpEnabled, _ := idp["enabled"].(bool)
	if (basicEnabled && len(users) > 0) || idpEnabled {
		return nil
	}
	version := ""
	if ch.Metadata != nil {
		version = ch.Metadata.Version
	}
	return fmt.Errorf("gateway chart version %q does not generate the gateway-controller admin credential, so with no "+
		"gateway.config.controller.auth.basic.users and IDP auth off its management API would accept every request; "+
		"set gateway.helm.chartVersion to a chart version that supports gateway.config.controller.auth.basic.generate, "+
		"or list gateway.config.controller.auth.basic.users", version)
}

// chartGeneratesControllerAdmin reports whether the chart's default values define
// gateway.config.controller.auth.basic.generate.
func chartGeneratesControllerAdmin(defaults map[string]interface{}) bool {
	basic, err := chartutil.Values(defaults).Table("gateway.config.controller.auth.basic")
	if err != nil {
		return false
	}
	_, ok := basic["generate"]
	return ok
}
