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

package testbench

import (
	"time"

	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/components"
	"github.com/wso2/api-platform/tests/framework/testbench/services/analytics"
	"github.com/wso2/api-platform/tests/framework/testbench/services/backend"
	"github.com/wso2/api-platform/tests/framework/testbench/services/bedrock"
	"github.com/wso2/api-platform/tests/framework/testbench/services/contentsafety"
	"github.com/wso2/api-platform/tests/framework/testbench/services/echo"
	"github.com/wso2/api-platform/tests/framework/testbench/services/embeddings"
	"github.com/wso2/api-platform/tests/framework/testbench/services/interceptor"
	"github.com/wso2/api-platform/tests/framework/testbench/services/jwks"
	"github.com/wso2/api-platform/tests/framework/testbench/services/mcp"
	"github.com/wso2/api-platform/tests/framework/testbench/services/openai"
)

// EnvImageTestbench overrides the testbench image, for CI to pin a build.
const EnvImageTestbench = "APIP_IT_IMAGE_TESTBENCH"

const imgTestbench = "ghcr.io/wso2/api-platform/testbench:test"

// Testbench is every mock service the suites need, in ONE container.
//
// It replaces a set of separately built images — one per mock, each its own Go module — with a
// single codebase a test developer can extend. Adding a service there costs a package; adding
// one here costs a port in the endpoint list below.
//
// ONE alias, one port per service. The port identifies the service, exactly as product-apim's
// node-app-server does with 3000-3020. An earlier design gave each service its own network
// alias so feature hostnames would not change; that was dropped because the port already
// identifies the service and per-service aliases imply separately restartable containers,
// which is the illusion this consolidation exists to remove.
//
// The PORTS BELOW ARE THE CONTAINER PORTS, and they are what a feature writes into an upstream
// URL (http://testbench:3001) for the gateway to resolve over docker DNS. Host ports are
// ephemeral and reached through the instance accessors instead.
func Testbench() *components.Definition {
	d := &components.Definition{
		Name:  "testbench",
		Image: shared.Image(EnvImageTestbench, imgTestbench),
		Alias: "testbench",
		Endpoints: []components.Endpoint{
			{Name: "backend", Port: backend.Port, Scheme: "http", AwaitListening: true},
			{Name: "jwks", Port: jwks.Port, Scheme: "http", AwaitListening: true},
			{Name: "echo", Port: echo.Port, Scheme: "http", AwaitListening: true},
			{Name: "interceptor", Port: interceptor.Port, Scheme: "http", AwaitListening: true},
			{Name: "bedrock", Port: bedrock.Port, Scheme: "http", AwaitListening: true},
			{Name: "openai", Port: openai.Port, Scheme: "http", AwaitListening: true},
			{Name: "mcp", Port: mcp.Port, Scheme: "http", AwaitListening: true},
			{Name: "embeddings", Port: embeddings.Port, Scheme: "http", AwaitListening: true},
			{Name: "content-safety", Port: contentsafety.Port, Scheme: "http", AwaitListening: true},
			// The analytics collector is STATEFUL and shared anyway, which every other entry
			// here is not: it partitions its buffers by block key, so the address a caller
			// uses is http://testbench:3007/<block> rather than the bare port. See
			// testbench/services/analytics and testbench.Partitioned.
			{Name: "analytics", Port: analytics.Port, Scheme: "http", AwaitListening: true},
		},
		// Every service answers the same health path on its own port, so gating on one is
		// gating on the process. AwaitListening above already proves each port is bound.
		Health: &components.HealthCheck{
			Endpoint: "jwks", Path: "/testbench/health", ExpectStatus: 200,
			Timeout: 60 * time.Second, Interval: time.Second,
		},
		Limits: components.ResourceLimits{CPUs: 0.5, MemoryMB: 256},
	}

	// Shared across blocks. Every hosted service either derives its response from the request or
	// partitions what it retains by block key — the registry REFUSES to register a stateful
	// service that does neither — so no block can observe another through it.
	d.Shared = true
	return d
}
