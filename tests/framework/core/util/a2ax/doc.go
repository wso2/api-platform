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

// Package a2ax drives A2A agents through the official Go A2A SDK and reports what each call
// returned in framework-owned types.
//
// The conformant path of an A2A scenario is a cross-SDK interoperability check: a client built
// on the reference Go SDK talks, through the gateway, to an agent built on the reference Python
// SDK. Hand-built JSON-RPC envelopes and REST paths would make both sides of that check the
// test's own reading of the specification, so a field name misread the same way twice would
// pass. Requests a conformant client cannot produce, such as a missing protocol version or an
// unknown method, belong on the ordinary HTTP funnel instead.
//
// Every Client is bound to exactly one endpoint and one protocol binding, with the SDK's
// transport defaults disabled, so a client asked for JSON-RPC cannot fall back to HTTP+JSON
// and turn a scenario claiming both bindings into one that exercised one binding twice. A
// Client is never built by following the URL inside a card unless the caller asks for that
// explicitly through InterfaceURL: a passthrough card advertises the upstream agent's own
// address, and a client that followed it would bypass the gateway entirely.
//
// SDK results are converted into Outcome values rather than re-encoded as response bytes, so an
// assertion over them can never be mistaken for an assertion over what the gateway returned.
// A failed call is recorded in Outcome.Err rather than returned, because whether a call was
// meant to be refused is the caller's assertion to make.
package a2ax
