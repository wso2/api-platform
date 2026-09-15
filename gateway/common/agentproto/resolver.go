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

package agentproto

// ResolverName is the policy-engine resolver that turns an A2A request into a
// canonical operation, and therefore into a policy chain key.
//
// The controller writes it into a route's resolver_name and the engine
// registers its resolver factory under it. A mismatch is silent in the worst
// way: the route ships, the engine finds no resolver for the name, and every
// request to that route fails to resolve — so the value lives here, spelled
// once, rather than as a literal on each side.
const ResolverName = "a2a"

// Transport is an A2A protocol binding as it travels in a route's
// resolver_config. The values match the management API's protocolBinding enum,
// so an operator reading a config dump sees the same spelling they wrote.
type Transport string

const (
	// TransportJSONRPC is the single-endpoint JSON-RPC binding. The operation is
	// carried in the request body, so a route bearing this transport resolves
	// per request.
	TransportJSONRPC Transport = "JSONRPC"
	// TransportHTTPJSON is the REST-shaped binding, one route per operation
	// binding. The operation is known from the route itself, so a route bearing
	// this transport resolves statically.
	TransportHTTPJSON Transport = "HTTP+JSON"
)

// ResolverConfig is the per-route configuration the controller attaches to
// every resolver-bearing A2A route and the engine reads once at xDS ingest.
//
// It is defined here, beside the operation tables it selects, because it is a
// wire contract between two modules that cannot import each other. The engine
// resolves ProtocolVersion through this package's registry and rejects a route
// naming a version it does not know — it never falls back to the newest or the
// only registered version, because an Agent's protocol version decides which
// operation set its policy chains were generated for.
//
// Operation is set only for TransportHTTPJSON, where the route itself
// identifies the operation. A JSONRPC route leaves it empty and reads the
// method out of the request body instead.
type ResolverConfig struct {
	ProtocolVersion ProtocolVersion `json:"protocolVersion"`
	Transport       Transport       `json:"transport"`
	Operation       Operation       `json:"operation,omitempty"`
}
