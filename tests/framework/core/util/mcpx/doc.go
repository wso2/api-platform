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

// Package mcpx drives MCP servers through the official Go MCP SDK and reports what each call
// returned in framework-owned types.
//
// A client built on the reference SDK runs the whole protocol itself: it chooses the handshake,
// carries the session id, unwraps event-stream answers and numbers the requests. A scenario that
// passes with it shows that a real MCP client can use the endpoint, where hand-built JSON-RPC
// bodies would only show that the endpoint accepts the test's own reading of the specification.
// Requests a conformant client cannot produce, such as malformed JSON or a header that
// contradicts the body, belong on the ordinary HTTP funnel instead.
//
// The SDK chooses the handshake from the server's answer, and a caller cannot pin it. It first
// sends server/discover (MCP 2026-07-28) and falls back to initialize (2025-11-25) only when the
// server answers "Method not found". A Session therefore records the JSON-RPC methods the client
// sent while it connected, so a caller can assert which handshake took place.
//
// Every request carries the caller's headers and Host override, except where the SDK sets a
// header itself: the protocol headers (Content-Type, Accept, Mcp-Protocol-Version,
// Mcp-Session-Id and the like) are the SDK's to choose. The SDK's own reconnection retries and
// its standalone event stream are disabled, so every answer a caller sees is the answer to one
// of its own requests.
package mcpx
