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

package browser

import (
	_ "embed"
	"fmt"
	"os"
	"strconv"

	"github.com/wso2/api-platform/tests/framework/core/components"
)

// EnvHeaded, set to a true value (1, true), runs the UI suite in a visible browser on the
// host instead of the headless one in the block's container. For local debugging only.
const EnvHeaded = "UI_HEADED"

// EnvSlowMo slows every browser action by this many milliseconds, in headed mode only.
const EnvSlowMo = "UI_SLOWMO"

// ProxyEndpoint is the headed-mode endpoint the host browser browses through.
const ProxyEndpoint = "proxy"

const proxyPort = 3128

//go:embed headed-proxy.js
var headedProxy []byte

// Headed reports whether EnvHeaded asks for a visible browser.
func Headed() bool {
	on, _ := strconv.ParseBool(os.Getenv(EnvHeaded))
	return on
}

// SlowMo returns EnvSlowMo in milliseconds, 0 when unset.
func SlowMo() (float64, error) {
	raw := os.Getenv(EnvSlowMo)
	if raw == "" {
		return 0, nil
	}
	ms, err := strconv.ParseFloat(raw, 64)
	if err != nil || ms < 0 {
		return 0, fmt.Errorf("browser: %s=%q is not a non-negative number of milliseconds", EnvSlowMo, raw)
	}
	return ms, nil
}

// headed turns the browser component into the headed browser's foothold on the block's
// network: a forward proxy in the same container, at the same alias, in place of run-server.
func headed(def *components.Definition) *components.Definition {
	def.Cmd = []string{"node", "/opt/headed-proxy.js"}
	def.Files = []components.FileMount{{Content: headedProxy, ContainerPath: "/opt/headed-proxy.js", Mode: 0o644}}
	def.Endpoints = []components.Endpoint{
		{Name: ProxyEndpoint, Port: proxyPort, Scheme: "http", AwaitListening: true},
	}
	return def
}
