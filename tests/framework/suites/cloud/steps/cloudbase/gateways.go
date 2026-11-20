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

package cloudbase

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
)

// ScenarioGatewayDescription is the description a suite run gives the gateways it creates for its
// scenarios when the scenario names none. It tells a run that is pointed at a gateway whether the
// gateway belongs to a suite run, whatever its name looks like.
const ScenarioGatewayDescription = "Created by the APIP cloud integration suite for one of its runs and removed when the run ends."

// ExpectedGatewayID is the id the platform registers a gateway under: the environment and the
// handle, which is the display name folded to lower case when none is given.
func ExpectedGatewayID(environment, displayName, handle string) string {
	if handle = strings.TrimSpace(handle); handle == "" {
		handle = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(displayName), " ", "-"))
	}
	return environment + "-" + handle
}

// SyntheticGatewayBase is the base of the names a suite run gives its managed gateways. The unique
// generator adds 11 characters to it, and the platform limits a handle to 37 minus twice the
// environment name's length, which leaves 15 characters in "development".
const SyntheticGatewayBase = "sg"

// GatewayRequestAccepted reports whether the platform took a gateway request: it answers 201 when
// the gateway is registered at once and 202 when its build was started.
func GatewayRequestAccepted(status int) bool {
	return status == http.StatusCreated || status == http.StatusAccepted
}

// SameValue compares a value read from JSON with the value a table cell stands for.
func SameValue(got, want any) bool {
	if reflect.DeepEqual(got, want) {
		return true
	}
	left, leftErr := json.Marshal(got)
	right, rightErr := json.Marshal(want)
	return leftErr == nil && rightErr == nil && string(left) == string(right)
}
