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
	"context"
	"fmt"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/testpki"
	"github.com/wso2/api-platform/tests/framework/testbench/services/tlsbackend"
)

// The TLS echo backends the gateway dials are the tls-backend component's. The required
// backend answers 400 to a request without a certificate it trusts; the optional backend
// always answers 200 and reports the subject of any presented certificate, or an empty one.
var echoBackendPorts = map[string]int{
	"required": tlsbackend.PortA,
	"optional": tlsbackend.PortOptional,
}

func (g *Gateway) registerEchoBackendSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I store the URL of the "(required|optional)" TLS backend as "([^"]*)"$`, g.storeEchoBackendURL)
}

// storeEchoBackendURL stores the address the gateway dials for a backend, so a feature
// names a backend by kind and not by port.
func (g *Gateway) storeEchoBackendURL(ctx context.Context, kind, key string) error {
	port, ok := echoBackendPorts[kind]
	if !ok {
		return fmt.Errorf("unknown TLS backend kind %q", kind)
	}
	local, ok := tcontext.LocalOf(ctx)
	if !ok || local == nil {
		return fmt.Errorf("cannot store the %s TLS backend URL %q without runner context", kind, key)
	}
	local.Set(key, fmt.Sprintf("https://%s:%d", testpki.TLSBackendHost, port))
	return nil
}
