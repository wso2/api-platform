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

	// Dynamic metadata key for target upstream/cluster routing
	// Used by policies to dynamically select which upstream definition to route to
	TargetUpstreamNameKey = "target_upstream_name"

	// Dynamic metadata key for the full cluster name (with prefix)
	// This is read by Lua filter to set the x-target-upstream header
	TargetUpstreamClusterKey = "target_upstream_cluster"

	// Header name for target upstream cluster routing
	// This header is set by the policy engine when SetUpstreamName is used
	// Envoy routes configured with cluster_header will read this to determine the target cluster
	TargetUpstreamHeader = "x-target-upstream"

	// UpstreamDefinitionClusterPrefix is the prefix used for clusters created from upstreamDefinitions
	// Must match the gateway-controller constant
	UpstreamDefinitionClusterPrefix = "upstream_"

	// Policy Engine Socket Path (matches gateway-controller constant)
	DefaultPolicyEngineSocketPath = "/var/run/api-platform/policy-engine.sock"

	// Gateway Analytics Socket Path (matches gateway-controller constant)
	DefaultALSSocketPath = "/var/run/api-platform/gateway-analytics.sock"

	// ALS Log Name (matches gateway-controller constant)
	DefaultALSLogName = "envoy_access_log"

	// xDS Client Constants
	// NodeID identifies this policy engine instance to the xDS server
	XDSNodeID = "policy-engine"
	// Cluster identifies the cluster this policy engine belongs to
	XDSCluster = "policy-engine-cluster"

	// Tracing Span Names
	SpanExternalProcessingProcess = "external_processing.process"
	SpanProcessRequestHeaders     = "external_processing.process_request_headers"
	SpanProcessRequestBody        = "external_processing.process_request_body"
	SpanProcessResponseHeaders    = "external_processing.process_response_headers"
	SpanProcessResponseBody       = "external_processing.process_response_body"
	SpanPolicyRequestFormat       = "policy.request.%s"
	SpanPolicyResponseFormat      = "policy.response.%s"

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
