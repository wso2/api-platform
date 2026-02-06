/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package constants

const (
	ExtProcFilterName = "api_platform.policy_engine.envoy.filters.http.ext_proc"
	ExtProcFilter     = "envoy.filters.http.ext_proc"

	// Policy Engine Socket Path (matches gateway-controller constant)
	DefaultPolicyEngineSocketPath = "/var/run/policy-engine.sock"

	// Tracing Span Names
	SpanExternalProcessingProcess      = "external_processing.process"
	SpanProcessRequestHeaders          = "external_processing.process_request_headers"
	SpanProcessRequestBody             = "external_processing.process_request_body"
	SpanProcessResponseHeaders         = "external_processing.process_response_headers"
	SpanProcessResponseBody            = "external_processing.process_response_body"
	SpanPolicyRequestFormat            = "policy.request.%s"
	SpanPolicyResponseFormat           = "policy.response.%s"

	// Tracing Attributes
	AttrRouteName                 = "route_name"
	AttrAPIName                   = "api_name"
	AttrAPIVersion                = "api_version"
	AttrAPIContext                = "api_context"
	AttrOperationPath             = "operation_path"
	AttrPolicyCount               = "policy_count"
	AttrError                     = "error"
	AttrErrorReasonNoContext      = "no_execution_context"
	AttrPolicyName                = "policy.name"
	AttrPolicyVersion             = "policy.version"
	AttrPolicyEnabled             = "policy.enabled"
	AttrPolicySkipped             = "policy.skipped"
	AttrSkipReason                = "skip.reason"
	AttrSkipReasonConditionNotMet = "condition_not_met"
	AttrPolicyExecutionTimeNS     = "policy.execution_time_ns"
	AttrPolicyShortCircuit        = "policy.short_circuit"
)