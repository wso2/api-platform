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

package a2aagent

import (
	"strconv"
	"time"

	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/components"
)

// EnvImageA2ATripPlanner overrides the trip-planner image, for CI to pin a build.
const EnvImageA2ATripPlanner = "APIP_IT_IMAGE_A2A_TRIP_PLANNER"

// Name is the component name suite files reference and the DNS alias other containers use.
const Name = "a2a-trip-planner"

// Port is the container port serving both A2A bindings and the Agent Card.
const Port = 9099

const imgA2ATripPlanner = "ghcr.io/wso2/api-platform/a2a-trip-planner:test"

// TripPlanner returns the A2A trip-planner component definition.
//
// The image is built from tests/mock-servers/a2a-trip-planner by the framework Makefile's
// a2a-trip-planner target, exactly as the testbench image is. JSON-RPC is served at "/",
// HTTP+JSON under "/v1", and the public Agent Card at the well-known path, so an Agent proxy
// declares the transport prefixes "/" and "/v1".
func TripPlanner() *components.Definition {
	return &components.Definition{
		Name:  Name,
		Image: shared.Image(EnvImageA2ATripPlanner, imgA2ATripPlanner),
		Alias: Name,
		Env: map[string]string{
			"TRIP_PORT": strconv.Itoa(Port),
			// The agent advertises this base in its own card; a passthrough card is rewritten
			// by the gateway, so the value only needs to be the agent's in-network address.
			"TRIP_PUBLIC_URL": "http://" + Name + ":" + strconv.Itoa(Port),
		},
		Endpoints: []components.Endpoint{
			{Name: "http", Port: Port, Scheme: "http", AwaitListening: true},
		},
		// The Agent Card is exempt from A2A version negotiation, so it is the one route that
		// answers 200 without protocol headers once the agent is serving.
		Health: &components.HealthCheck{
			Endpoint: "http", Path: "/.well-known/agent-card.json", ExpectStatus: 200,
			Timeout: 60 * time.Second, Interval: time.Second,
		},
		Limits: components.ResourceLimits{CPUs: 0.5, MemoryMB: 256},
	}
}
