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

// Package aws provides the testbench's AWS services. The AWS service, on port 80, plays the AWS
// endpoints the gateway's AWS Authentication policy reaches, and verifies AWS Signature Version 4
// (SigV4) request signatures independently of the AWS SDK the gateway uses. The Bedrock service,
// on port 3006, plays the Bedrock Runtime ApplyGuardrail API the AWS Bedrock Guardrail policy
// calls.
//
// The first two segments of a request path to the AWS service name the AWS service and region
// being addressed, as an AWS endpoint's host name does:
//
//	/sts/<region>                          STS AssumeRole (sts.go), for policies that assume an
//	                                       IAM role, through AWS_ENDPOINT_URL_STS
//	/<any other service>/<region>/...      the upstream (upstream.go), an AWS-hosted backend such
//	                                       as an API Gateway API, for the AWS Authentication
//	                                       policy; for example http://testbench/execute-api/us-east-1
//
// The upstream and STS verify a request's signature before anything else, for their own service
// and, for the upstream, its own region; signature verification and the fixture credentials are
// shared (signature.go, credentials.go). The service keeps no state between requests: every answer
// depends only on the request and the service's clock, so concurrent callers never affect one
// another.
//
// Verification follows SigV4 as the AWS SDKs implement it for services other than Amazon S3: the
// path is URI-encoded twice in the canonical request, and the signature, credential scope, session
// token and expiry are checked. X-Amz-Date is accepted within five minutes of the service's clock.
// The tests check the verifier against the official AWS SigV4 test suite and the AWS SDK for Go v2
// signer. The error codes are representative AWS codes, not those of one particular service. IAM
// policy evaluation is reduced to a per-credential list of permitted resources, and S3-specific
// signing (single path encoding, UNSIGNED-PAYLOAD, chunked payloads), SigV4a and presigned
// (query-string) authentication are not supported.
//
// The upstream reflects a verified request as JSON, with the verified access key ID in the
// X-Aws-Access-Key-Id response header, and rejects any other request with 403 and an AWS error
// code. Like AWS, it authorizes a caller only after its signature verifies, and looks up the
// resource only after that. A credential restricted to some resources is rejected elsewhere with
// 403 AccessDeniedException. The upstream has every resource except those under
// /<service>/<region>/nonexistent, for which it answers 404 ResourceNotFoundException.
//
// STS answers AssumeRole, signed with a permanent fixture credential, with temporary credentials
// that expire after a short lifetime. Those credentials are derived from their expiry time rather
// than stored. The name of the role a request assumes can select an STS failure or a shorter
// credential lifetime, after the request's signature verifies:
//
//	testbench-sts-unavailable    always 503 ServiceUnavailable
//	testbench-sts-intermittent   503 ServiceUnavailable for the first 10 seconds of every 20, by
//	                             the service's clock (Unix time), and otherwise assumed with
//	                             credentials that expire after 3 seconds
//	testbench-sts-unresponsive   held without an answer until the caller gives up, or for at
//	                             most 90 seconds, then 503 ServiceUnavailable
//	testbench-sts-short-lived    always assumed, with credentials that expire after 3 seconds
//
// Any other role is assumed normally.
//
// The Bedrock service (bedrock.go) listens on its own port, which the gateway runtime reaches
// through AWS_ENDPOINT_URL_BEDROCK_RUNTIME as http://testbench:3006. It decides ApplyGuardrail from
// the text it is given and does not verify signatures: text naming violence, hate or illegal
// content is blocked (GUARDRAIL_INTERVENED); email addresses at example.com are masked ("mask-"
// addresses, restorable) or redacted; text asking to simulate an error gets 500; anything else
// passes (NONE) and is echoed back, as Bedrock always returns the text in outputs[0]. It keeps no
// state either.
//
// The fixture credentials are:
//
//	AKIATESTPERMANENT001 / testPermanentSecretAccessKey000000000001   permanent
//	AKIATESTPERMANENT002 / testPermanentSecretAccessKey000000000002   permanent
//	AKIATESTWORKLOAD0001 / testWorkloadSecretAccessKey0000000000001   permanent, workload
//	AKIATESTRESTRICTED01 / testRestrictedSecretAccessKey00000000001   permanent, permitted only
//	                                                                   /items and below
//	ASIATESTTEMPORARY001 / testTemporarySecretAccessKey000000000001   temporary, session token
//	                       testTemporarySessionToken0001
//	ASIATESTEXPIRED00001 / testExpiredSecretAccessKey00000000000001   temporary and expired,
//	                       testExpiredSessionToken0001                session token
//
// A request to the upstream may list access key IDs in the X-Testbench-Revoked-Access-Keys
// header, separated by commas. The upstream treats those keys as deactivated for that request,
// which lets a scenario model a key deactivated in AWS without shared state.
//
// A request to the upstream may set the X-Testbench-Signer-Clock-Skew header to a Go duration such
// as "10m" or "-10m". The upstream then judges X-Amz-Date as if the signer's clock ran that far
// ahead of (or, when negative, behind) its own, which lets a scenario model clock skew on the
// signing gateway. "0s" models a correct clock.
package aws
