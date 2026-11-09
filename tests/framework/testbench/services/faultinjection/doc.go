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

// Package faultinjection provides a block-partitioned proxy that injects scripted faults in front
// of another testbench service and counts the requests it receives.
//
// A policy that calls an external service retries, times out, or falls back when that service
// fails. The service itself cannot fail on cue, because the policy, not the test, decides when it
// is called. A scenario configures the policy's endpoint as
//
//	http://testbench:3026/<block>/<id>
//
// where <id> is a scenario-unique name, then stores a script for that id:
//
//	PUT /<block>/test/scripts/<id>
//	{"forwardPort": 3005, "steps": [{"status": 500}, {"delayMs": 2000}]}
//
// Each request to /<block>/<id>/<path> takes the script's next step:
//
//   - {"status": n} answers n (400-599) with a short plain-text body;
//   - {"delayMs": n} waits n milliseconds (at most maxDelay), then forwards the request.
//
// Storing a script checks forwardPort: a port where nothing listens, or one that serves TLS, is
// refused with 400.
//
// Once the steps are used up, every request is forwarded. Forwarding sends the request, with
// <path> and its query, to the testbench service on forwardPort. The method, the body and the
// end-to-end headers are kept. The rest changes as with any Go reverse proxy:
//
//   - the Host header names the target service, as a direct caller would;
//   - hop-by-hop headers (Connection, Keep-Alive, TE and those Connection names), Forwarded and
//     X-Forwarded-* are dropped;
//   - <path> is sent decoded, so an encoded slash (%2F) reaches the target as "/";
//   - query parameters that cannot be parsed are dropped.
//
// A partitioned target needs its own block segment at the start of <path>.
//
// GET /<block>/test/scripts/<id> reports {"id": ..., "received": n}: every request the script
// answered since it was stored. Storing a script for an existing id replaces it and resets the
// count. A request for an id with no script answers 404 and is not counted, so a scenario stores
// its script after the requests that wait for the API to become ready.
//
// A delay must end before the caller's own timeout: the testbench server and the suite's HTTP
// client both stop after 30 seconds, which bounds maxDelay.
package faultinjection
