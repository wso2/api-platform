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

// Package entra provides a tenant-partitioned Microsoft Entra ID token endpoint, a managed identity
// endpoint in the shape Azure Container Apps serves, and two Entra-protected resources, for
// policies that authenticate to Azure with a workload identity or a managed identity.
//
// The gateway runtime is configured the way the AKS Workload Identity webhook configures a pod:
// AZURE_AUTHORITY_HOST is https://testbench:3028/ (this service over TLS, with the certificate the
// runtime already trusts), and AZURE_FEDERATED_TOKEN_FILE holds a fixture service account token.
// The routes, each after a tenant that is also the partition, are:
//
//	GET  /<tenant>/v2.0/.well-known/openid-configuration   the tenant's issuer and token endpoint
//	POST /<tenant>/oauth2/v2.0/token   client credentials grant with a jwt-bearer client assertion
//	GET  /<tenant>/test/requests       {"count": n, "requests": [...]}, every token request, oldest first
//	*    /<tenant>/foundry/...         a resource accepting tokens for https://ai.azure.com
//	*    /<tenant>/cognitive/...       a resource accepting tokens for https://cognitiveservices.azure.com
//
// A tenant is one path segment of 1 to 64 lowercase letters, digits and dashes, other than the
// reserved v1, test and testbench. The runtime's AZURE_TENANT_ID is a single tenant that every
// block shares, so a scenario that counts token requests, or uses a tenant prefix below, names a
// tenant of its own in the policy's tenantId. The service keeps the first 256 requests of each
// partition (the count stays exact beyond that) and up to 4096 partitions; a request for a new
// partition after that is answered with 507.
//
// MSAL, which the Azure Identity SDK uses, reads a tenant's OpenID configuration before its first
// token request there. The service answers it with the issuer and token endpoint on the scheme and
// host the request came in on, as MSAL requires of the authority, and does not log it.
//
// The token endpoint answers on both ports and checks the form as Entra ID does: grant_type
// client_credentials, a client_id, a scope of exactly one resource's /.default (which openid,
// offline_access and profile may accompany, as MSAL sends it) and a non-empty client assertion of
// type urn:ietf:params:oauth:client-assertion-type:jwt-bearer, with no parameter given twice;
// anything else is answered with 400 and Entra ID's error shape. Other parameters, such as MSAL's
// client_info, are accepted. Every error body's trace_id and
// correlation_id are fresh random GUIDs, as Entra ID's are, so they never carry the tenant. A
// token is "testbench-entra." followed by the base64url encoding of {"aud", "tid", "appid", "n"}:
// the resource the scope names, the tenant, the client_id and the request's position for the
// tenant. The request log records the scope as sent. It is valid for 3599 seconds.
//
// A tenant prefix selects how the token endpoint answers. A one-time answer goes to the tenant's
// first well-formed request; a request answered 400 does not use it up.
//
//	invalid-client-   401 invalid_client, error code 700213, every time
//	reject-once-      the same 401 for the tenant's first request, then tokens
//	throttle-once-    429 with Retry-After: 1 for the first request, then tokens
//	error-once-       500 with an empty body for the first request, then tokens
//	refresh-soon-     tokens, the first one with "refresh_in": 2
//	unavailable-      503, every time
//	slow-             tokens after three seconds, or nothing if the caller gives up first
//
// Every request is logged with the status it was answered with, or 0 when the caller left before
// the answer, as a slow- request does when its caller gives up.
//
// The managed identity endpoint stands in for the one Azure Container Apps (and App Service) runs
// beside an app with a managed identity. The runtime's IDENTITY_ENDPOINT is
// https://testbench:3028/${BLOCK}/msi/token and its IDENTITY_HEADER is IdentityHeader, so the Azure
// Identity SDK sends every managed identity token request there, under its own block's partition:
//
//	GET  /<block>/msi/token?api-version=2019-08-01&resource=<r>[&client_id=<id>]
//	GET  /<block>/test/managed-identity/<client_id>   the block's requests for that identity
//	GET  /<block>/test/managed-identity/system-assigned   the block's requests without client_id
//
// It checks the request as Container Apps does: exactly one X-IDENTITY-HEADER carrying
// IdentityHeader, or it answers 403 with an empty body; then a query string that parses cleanly,
// no repeated query parameter, api-version 2019-08-01 and a resource, or it answers 400 with
// Container Apps' {"statusCode", "message", "correlationId"} body. It answers in Container Apps'
// shape: access_token, expires_on in Unix seconds, resource, token_type and client_id, every field a
// string. Its tokens carry the resource as the audience, "managed-identity" as the tenant and the
// client_id, or "system-assigned" without one, as the appid. Counters and request logs belong to
// the block and the identity together, so a scenario that names its own identity, a fresh UUID,
// owns its log, and blocks sharing a testbench never see each other's requests. The resource
// selects how it answers, again counting only well-formed requests:
//
//	api://not-found-once       404 for the identity's first request, then tokens
//	api://throttle-once        429 for the first request, then tokens
//	api://error-once           500 for the first request, then tokens
//	api://identity-not-found   400 every time, with Container Apps' message for an identity the app
//	                           doesn't have ("No User Assigned or Delegated Managed Identity found
//	                           ..." with client_id, "Unable to load the proper Managed Identity."
//	                           without)
//	api://unknown-resource     400 every time, Entra ID's AADSTS500011 refusal passed on as
//	                           Container Apps does, with "errorCode": "invalid_resource"
//
// A protected resource answers a valid token with a chat completion that echoes the request
// headers ("headers") and the token's claims ("token"), and anything else with 401. Every answer
// reports the bearer token it received in X-Testbench-Entra-Token and, for a token this service
// issued, its tenant, audience and appid in X-Testbench-Entra-Tid, X-Testbench-Entra-Aud and
// X-Testbench-Entra-Appid.
//
// Issued tokens are unsigned on purpose: a resource checks only a token's audience, so it accepts
// a token built by hand or issued for another tenant. The policies under test fetch and forward
// tokens; validating them is the real resource's job, not something the tests exercise.
//
// The mock simplifies Entra ID and Container Apps where the policies do not depend on the difference:
//
//   - The token endpoint accepts any non-empty client assertion without validating it.
//   - The AADSTS codes of the 400 answers approximate Entra ID's; Entra ID may send a different
//     code for the same mistake.
//   - The managed identity endpoint accepts only api-version 2019-08-01, ignores principal_id and
//     mi_res_id, answers a request without client_id with client_id "system-assigned" rather than
//     the app identity's real client ID, and issues tokens valid for 3599 seconds rather than
//     Container Apps' 24 hours. Its 400 messages for a malformed request are its own.
//
// The fixture federated token is "testbench-federated-identity-token" and every issued token
// starts with "testbench-entra.", so a response body or log line without either discloses no
// credential.
//
// The oauth2 service is not reused: it serves plain HTTP only, a different path, one state per
// block rather than per tenant, ignores client assertions, and has neither Entra ID's error shape
// nor a resource that checks a token's audience.
package entra
