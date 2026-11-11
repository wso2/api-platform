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

// Package entra provides a tenant-partitioned Microsoft Entra ID token endpoint and two
// Entra-protected resources, for policies that authenticate to Azure with a workload identity.
//
// The gateway runtime is configured the way the AKS Workload Identity webhook configures a pod:
// AZURE_AUTHORITY_HOST is https://testbench:3028/ (this service over TLS, with the certificate the
// runtime already trusts), and AZURE_FEDERATED_TOKEN_FILE holds a fixture service account token.
// The routes, each after a scenario-unique tenant that is also the partition, are:
//
//	POST /<tenant>/oauth2/v2.0/token   client credentials grant with a jwt-bearer client assertion
//	GET  /<tenant>/test/requests       {"count": n, "requests": [...]}, every token request, oldest first
//	*    /<tenant>/foundry/...         a resource accepting tokens for https://ai.azure.com
//	*    /<tenant>/cognitive/...       a resource accepting tokens for https://cognitiveservices.azure.com
//
// The token endpoint answers on both ports and checks the form as Entra ID does: grant_type
// client_credentials, a client_id, a scope ending in /.default and a non-empty client assertion of
// type urn:ietf:params:oauth:client-assertion-type:jwt-bearer, with no parameter given twice;
// anything else is answered with 400 and Entra ID's error shape. A token is "testbench-entra." followed by the base64url encoding of
// {"aud", "tid", "appid", "n"}: the scope without /.default, the tenant, the client_id and the
// request's position for the tenant. It is valid for 3599 seconds.
//
// A tenant prefix selects how the token endpoint answers:
//
//	invalid-client-   401 invalid_client, error code 700213, every time
//	reject-once-      the same 401 for the tenant's first request, then tokens
//	throttle-once-    429 with Retry-After: 1 for the first request, then tokens
//	error-once-       500 with an empty body for the first request, then tokens
//	refresh-soon-     tokens, the first one with "refresh_in": 2
//	unavailable-      503, every time
//	slow-             tokens after three seconds, or nothing if the caller gives up first
//
// A protected resource answers a valid token with a chat completion that echoes the request
// headers ("headers") and the token's claims ("token"), and anything else with 401. Every answer
// reports the bearer token it received in X-Testbench-Entra-Token and, for a token this service
// issued, its tenant and audience in X-Testbench-Entra-Tid and X-Testbench-Entra-Aud.
//
// The fixture federated token is "testbench-federated-identity-token" and every issued token
// starts with "testbench-entra.", so a response body or log line without either discloses no
// credential.
//
// The oauth2 service is not reused: it serves plain HTTP only, a different path, one state per
// block rather than per tenant, ignores client assertions, and has neither Entra ID's error shape
// nor a resource that checks a token's audience.
package entra
