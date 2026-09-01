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

// Package httpx is the HTTP layer every step goes through. Two layers, deliberately
// separated:
//
//	Client   - the raw chokepoint. All HTTP passes through it, which makes it the one
//	           home for cross-cutting concerns: TLS trust, connection pooling,
//	           transparent retry of known-transient product errors.
//	Requests - a thin layer over Client that ADDITIONALLY publishes the response as
//	           the step's assertion target.
//
// The reason Requests exists at all is the stale-response trap. Request-making steps
// publish to a shared key that the generic assertion steps read back. If a step
// throws after an earlier step already stored a response, the old response lingers
// and the next assertion passes against the WRONG call — a false green. So every
// publishing call clears the stored response BEFORE issuing the request, leaving it
// absent rather than stale on failure. That ordering is the whole point; it cannot be
// bolted on at the call sites.
//
// Use Requests for the one request whose response the step publishes for a following
// assertion. Use Client directly ONLY for an intermediate read consumed locally
// within the same step (the GET half of a GET-mutate-PUT), which must not touch the
// published response. Do not add a "do not publish" flag to Requests: that collapses
// to exactly a Client call, so there is nothing left to centralise.
//
// Steps never hand-roll a retry around a request. Transient product errors are
// absorbed here, once; deadline-bounded polling belongs to the retry package.
package httpx
