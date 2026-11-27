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

// Package interceptor provides a stateless interceptor service for the interceptor-service
// policy, which POSTs request- and response-phase data to {endpoint}/handle-request and
// {endpoint}/handle-response and applies the JSON reply.
//
// # Fixed replies
//
// POST /handle-request and POST /handle-response answer from a word in the payload's
// invocationContext.path: "block" short-circuits with a 403, "mutate" rewrites the path, a header
// and the body, "response-rewrite" sets an interceptor context and then rewrites the response.
// Any other path gets {}.
//
// # Scripted replies
//
// A scenario that needs a particular reply points the policy at
//
//	http://testbench:3003/scripted/<id>
//
// where <id> is a scenario-unique name of letters, digits, dashes and underscores. The policy
// then calls POST /scripted/<id>/handle-request and POST /scripted/<id>/handle-response. The reply
// is chosen by control headers the client sends on its own request; the policy forwards them to
// the interceptor inside the payload's requestHeaders (request phase by default; the response
// phase only with response.includeRequestHeaders). Names match without regard to case, and each
// phase reads only its own:
//
//	X-Testbench-Interceptor-Request-Reply     base64 of the exact bytes to answer with (default {})
//	X-Testbench-Interceptor-Request-Status    the status to answer with, 200-599 (default 200)
//	X-Testbench-Interceptor-Request-Delay-Ms  wait this long first, 0-10000
//	X-Testbench-Interceptor-Request-Mode      "reflect": answer with the payload received
//	X-Testbench-Interceptor-Request-Location  an absolute http or https URL to send as the
//	                                          Location header, for a redirect status
//
// and the same with Response in place of Request. The reply bytes are sent as given, so a
// scenario can answer with malformed JSON on purpose.
//
// Reflect returns what the interceptor received, so a scenario can read it from the client
// response. In the request phase the reply is a direct response: status 200, content type
// application/json, the payload as the body. In the response phase the reply replaces the
// response body with the payload and sets the content type. Reflect is also selected by the path
// POST /scripted/<id>/reflect/handle-request (or handle-response), so it works when no request
// headers reach the interceptor. Reflect cannot be combined with a reply, a status or a location.
//
// # HTTPS
//
// The same service answers on https://testbench:3027 when the testbench is given the Jev TLS
// certificate and key (TESTBENCH_JEV_TLS_CERT and TESTBENCH_JEV_TLS_KEY). The gateway runtime
// trusts that certificate, so a policy pointed at https://testbench:3027 or
// https://testbench:3027/scripted/<id> passes its certificate check. Every route behaves as on
// port 3003.
//
// # Validation
//
// A scripted call must carry a JSON object of the contract's shape: requestHeaders,
// responseHeaders and interceptorContext are objects of strings, requestBody and responseBody are
// strings, invocationContext is an object, and the response phase has an integer responseCode.
// A call that is not POST answers 405, one whose content type is not application/json 415, and
// one with a malformed payload or control header 400 with the reason. Oversized bodies answer
// 413. The service keeps no state: every reply derives from the request.
package interceptor
