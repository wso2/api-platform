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

// Package capture provides a partitioned upstream that remembers the last request it received
// per path. It exists for policies that rewrite a request on the way in and rewrite the
// response back on the way out (masking, redaction): the client-visible response can never show
// what the upstream actually received, because the response-side rewrite has already reversed
// it by the time a test observes that response. This service lets a test ask the upstream
// directly, in a second call, independent of any response rewriting. A path can also be scripted
// to answer with a given status, Content-Type and body, so the upstream can return what a policy's
// response handling needs, such as XML or a malformed body.
//
// Every route is under the block segment, http://testbench:3010/<block>/...:
//
//   - Any other path records the request (method, path, query, headers, body) under its path,
//     replacing an earlier record for the same path, and answers with that record as JSON.
//   - GET /test/captured?path=<path> returns the record for <path>, or 204 when there is none.
//   - POST /test/reset clears the block's records and scripted responses.
//   - GET /test/health reports the service as healthy.
//
// A scenario that needs the upstream to answer with something other than JSON, such as XML or
// a malformed body, scripts the answer for a path first:
//
//	PUT /<block>/test/responses?path=<path>
//	{"status": 201, "contentType": "application/xml", "body": "<ack>received</ack>"}
//
// Every later request to <path> is still recorded, and is answered with the script's status
// (200 when left out), its Content-Type (none when left out) and its body, verbatim, with a
// Content-Length. Storing a script for a path again replaces it. A path with no script answers
// the JSON record, as before. A script applies to every request in the block that uses its path,
// so a scenario scripts a path unique to it, such as one that includes its API's unique name.
//
// The script document is refused with 400 when it has unknown fields, a status outside 200-599,
// a body with status 204 or 304, a control character other than a tab in contentType, or a path
// that is empty, does not start with "/", or is under /test/.
package capture
