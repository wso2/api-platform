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
	// This header is set by the policy engine when UpstreamName is used
	// Envoy routes configured with cluster_header will read this to determine the target cluster
	TargetUpstreamHeader = "x-target-upstream"

	// UpstreamDefinitionClusterPrefix is the prefix used for clusters created from upstreamDefinitions
	// Must match the gateway-controller constant
	UpstreamDefinitionClusterPrefix = "upstream_"

	// CurrentUpstreamKey is the dynamic metadata key for the request's current effective
	// upstream (an UpstreamInfo object: cluster_name/url/base_path). Always present —
	// starts as the route's compiled-in default and is overwritten in place whenever a
	// dynamic-routing policy (UpstreamName) redirects the request, so
	// downstream consumers (Lua filter, analytics, response-phase processing) always see
	// the request's actual destination.
	CurrentUpstreamKey = "current_upstream"

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

	// NodeMetaResolutionProtocolVersion is the Node.Metadata key carrying the
	// operation-resolution protocol version this runtime implements. The control
	// plane reads it to decide whether this runtime can be sent routes that need
	// request-time operation resolution.
	NodeMetaResolutionProtocolVersion = "resolution_protocol_version"
	// NodeMetaSupportedResolvers is the Node.Metadata key carrying the sorted list
	// of operation resolvers registered in this runtime.
	NodeMetaSupportedResolvers = "supported_resolvers"

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
	AttrResolverName              = "resolver.name"
	AttrPolicyChainKey            = "policy_chain_key"
	AttrResolvedOperation         = "resolver.operation"

	// Terminal-outcome attributes. The status code itself is recorded under the
	// OTel semantic-convention key http.response.status_code by
	// tracing.RecordHTTPOutcome; these two are policy-engine specific.
	AttrTerminalReason  = "terminal.reason"
	AttrTerminalErrorID = "terminal.error_id"

	// Values for AttrTerminalReason. A 4xx keeps span status Unset, so these are
	// the tag that keeps denials queryable.
	TerminalReasonUpstream             = "upstream_response"      // pass-through with no policy status override; status came from the backend unmodified. The one reason exempt from the Error span status — see tracing.upstreamFaultReasons.
	TerminalReasonPolicyStatusOverride = "policy_status_override" // a response-body policy set DownstreamResponseModifications.StatusCode
	TerminalReasonPolicyDenied         = "policy_denied"          // a policy returned an ImmediateResponse
	TerminalReasonPolicyError          = "policy_error"           // handlePolicyError generated a 500
	TerminalReasonPayloadTooLarge      = "payload_too_large"      // handlePayloadTooLarge generated a 413
	TerminalReasonNoPolicyChain        = "no_policy_chain"        // route resolved but no chain registered
	TerminalReasonUnknownMessageType   = "unknown_message_type"   // unrecognised ext_proc message
	TerminalReasonProcessingFailed     = "processing_failed"      // a phase returned a fatal (stream-ending) error with no ImmediateResponse to classify

	// TerminalReasonResolutionFailed marks a request whose logical operation could not
	// be resolved to a policy chain. It exists because the status alone cannot identify
	// one — an unknown-operation failure is an HTTP 404 just like an Envoy route miss.
	TerminalReasonResolutionFailed = "resolution_failed"

	// Analytics metadata and property keys shared across packages.
	GuardrailHitMetadataKey  = "isGuardrailHit"
	GuardrailNameMetadataKey = "guardrailName"
	LLMCostMetadataKey       = "x-llm-cost"
	LLMCostPropertyKey       = "llmCost"
)
