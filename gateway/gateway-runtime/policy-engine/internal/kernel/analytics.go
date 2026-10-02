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

package kernel

import (
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/correlation"
)

// Constants for analytics metadata
const (
	Wso2MetadataPrefix = "x-wso2-"
	APIIDKey           = Wso2MetadataPrefix + "api-id"
	APINameKey         = Wso2MetadataPrefix + "api-name"
	APIVersionKey      = Wso2MetadataPrefix + "api-version"
	APITypeKey         = Wso2MetadataPrefix + "api-type"
	APIContextKey      = Wso2MetadataPrefix + "api-context"
	OperationPathKey   = Wso2MetadataPrefix + "operation-path"
	APIKindKey         = Wso2MetadataPrefix + "api-kind"
	ProjectIDKey       = Wso2MetadataPrefix + "project-id"

	// ResolvedOperationKey carries the canonical protocol operation the request
	// resolved to, on an API kind whose operation is not knowable from the route.
	//
	// It is stamped by the engine rather than by the analytics system policy on
	// purpose. That policy is conditionally injected — it is only in the chain when
	// a collector is enabled — so anything sourced from it is silently absent
	// otherwise; and the operation is the one dimension the whole A2A event is keyed
	// by. Stamping it here also means it cannot disagree with the chain that ran:
	// the value comes from SharedContext.ResolvedOperation, which the kernel derived
	// from the same chain key it bound.
	//
	// Absent for every kind whose route fixes its own chain, where OperationPath
	// already says what ran.
	ResolvedOperationKey = Wso2MetadataPrefix + "resolved-operation"

	// TerminalReasonKey names why the engine terminated a request, for the events
	// whose outcome the HTTP status alone does not explain.
	//
	// A denial raised by a policy and a failure returned by the upstream can arrive
	// downstream as the same status code, so without this an analytics consumer
	// computing a success rate cannot attribute a failure to the right component.
	// Only the engine can tell them apart, and only at the moment it emits the
	// response.
	//
	// Absent on a pass-through, which is the overwhelmingly common case: its outcome
	// is the upstream's and its status says so. The value is one of the
	// constants.TerminalReason* strings — a closed set, safe as a metric label.
	TerminalReasonKey = Wso2MetadataPrefix + "terminal-reason"
)

// analyticsRequestHeadersKey / analyticsResponseHeadersKey are the analytics-metadata
// keys the analytics system policy (gateway/system-policies/analytics) uses to carry
// captured request/response headers. They are excluded from what buildAnalyticsStruct
// sends to Envoy -- see its doc comment -- so they must be spelled out here rather than
// imported: the system policy is a separate Go module with no shared dependency on this
// package, and the two sides agree on the key names only by (documented) convention.
const (
	analyticsRequestHeadersKey  = "request_headers"
	analyticsResponseHeadersKey = "response_headers"
	// analyticsRequestPayloadKey / analyticsResponsePayloadKey carry captured bodies
	// (collector.request_body / collector.response_body), by the same convention.
	analyticsRequestPayloadKey  = "request_payload"
	analyticsResponsePayloadKey = "response_payload"
	// analyticsInternalLoopbackKey is the marker the analytics system policy stamps
	// on the LLM proxy's internal loopback hop, by the same convention.
	analyticsInternalLoopbackKey = "x-wso2-internal-loopback"
)

// correlatesInProcess reports whether this request may hand captured data to the
// ALS handler through the correlation store: the store exists, the request id is
// Envoy's x-request-id (the key the ALS side looks up), and the request is not the
// LLM proxy's internal loopback hop. That hop can share the outer call's request
// id, and its own access-log event is suppressed, so it keeps its data in Envoy
// metadata and never touches the outer call's entry. data is the analytics
// metadata being built, which can carry the loopback marker before
// execCtx.analyticsMetadata does (e.g. on a short-circuit); it may be nil.
func correlatesInProcess(execCtx *PolicyExecutionContext, data map[string]any) bool {
	if execCtx == nil || execCtx.server == nil || execCtx.server.correlationStore == nil ||
		!execCtx.requestIDFromHeader {
		return false
	}
	if _, ok := data[analyticsInternalLoopbackKey]; ok {
		return false
	}
	_, loopback := execCtx.analyticsMetadata[analyticsInternalLoopbackKey]
	return !loopback
}

// storeInProcess hands one captured header or body field to the correlation store
// and reports whether the store accepted it; only then may the field be left out
// of Envoy metadata. It runs while the ext_proc response for this phase is being
// built, before that response is sent, so Envoy cannot emit the request's
// access-log entry before the data is in the store. A field the store refuses (no
// free slot, a body over the size or byte budget) or a header value of an
// unrecognised shape stays in metadata, the pre-store path.
func storeInProcess(execCtx *PolicyExecutionContext, key string, value any) bool {
	var p correlation.Payload
	switch key {
	case analyticsRequestHeadersKey:
		p.RequestHeaders = normalizeAnalyticsHeaderValue(value)
	case analyticsResponseHeadersKey:
		p.ResponseHeaders = normalizeAnalyticsHeaderValue(value)
	case analyticsRequestPayloadKey:
		p.RequestBody, _ = value.(string)
	case analyticsResponsePayloadKey:
		p.ResponseBody, _ = value.(string)
	default:
		return false
	}
	return execCtx.server.correlationStore.Merge(execCtx.requestID, p)
}

// convertToStructValue converts a value to structpb.Value, handling complex types like map[string][]string
func convertToStructValue(value any) (*structpb.Value, error) {
	// Try direct conversion first (works for simple types)
	val, err := structpb.NewValue(value)
	if err == nil {
		return val, nil
	}

	// If direct conversion fails, serialize to JSON string for complex types
	// This handles cases like map[string][]string which protobuf doesn't support directly
	jsonBytes, jsonErr := json.Marshal(value)
	if jsonErr != nil {
		return nil, fmt.Errorf("failed to marshal value to JSON: %w", jsonErr)
	}

	return structpb.NewStringValue(string(jsonBytes)), nil
}

// buildAnalyticsStruct converts analytics metadata map to structpb.Struct
// If execCtx is provided, adds system-level metadata (API name, version, etc.) to analytics_data.metadata
//
// Captured request/response headers and bodies are handed to the in-process
// correlation store (internal/analytics/correlation, keyed by request id) and left
// out of the struct sent to Envoy, but only when the store accepts them (see
// storeInProcess). They used to make a full round trip -- encoded here, forwarded
// back on every later ext_proc message, echoed in the access-log entry's
// filter_metadata, and decoded again on the ALS side -- purely to correlate them
// back to their request, although the ext_proc and ALS handlers run in the same
// process. Anything the store refuses stays in the struct, and every other
// analytics_data field (API identity, auth context, subscription, AI/MCP metadata)
// is unaffected.
func buildAnalyticsStruct(analyticsData map[string]any, execCtx *PolicyExecutionContext) (*structpb.Struct, error) {
	// Start with the analytics data from policies
	fields := make(map[string]*structpb.Value)

	// Add policy-provided analytics data
	inProcess := correlatesInProcess(execCtx, analyticsData)
	for key, value := range analyticsData {
		if inProcess && storeInProcess(execCtx, key, value) {
			continue
		}
		val, err := convertToStructValue(value)
		if err != nil {
			return nil, fmt.Errorf("failed to convert analytics value for key %s: %w", key, err)
		}
		fields[key] = val
	}

	// Add system-level metadata if context is provided
	if execCtx != nil && execCtx.sharedCtx != nil {

		sharedCtx := execCtx.sharedCtx
		if sharedCtx.APIId != "" {
			fields[APIIDKey] = structpb.NewStringValue(sharedCtx.APIId)
		}
		if sharedCtx.APIName != "" {
			fields[APINameKey] = structpb.NewStringValue(sharedCtx.APIName)
		}
		if sharedCtx.APIVersion != "" {
			fields[APIVersionKey] = structpb.NewStringValue(sharedCtx.APIVersion)
		}
		if sharedCtx.APIContext != "" {
			fields[APIContextKey] = structpb.NewStringValue(sharedCtx.APIContext)
		}
		if sharedCtx.OperationPath != "" {
			fields[OperationPathKey] = structpb.NewStringValue(sharedCtx.OperationPath)
		}
		if sharedCtx.APIKind != "" {
			fields[APIKindKey] = structpb.NewStringValue(string(sharedCtx.APIKind))
		}
		if sharedCtx.ProjectID != "" {
			fields[ProjectIDKey] = structpb.NewStringValue(sharedCtx.ProjectID)
		}
		// Omitted rather than empty-stringed when the route resolved directly, so a
		// consumer can tell "this kind has no operation dimension" from "the
		// operation was not determined".
		if sharedCtx.ResolvedOperation != "" {
			fields[ResolvedOperationKey] = structpb.NewStringValue(sharedCtx.ResolvedOperation)
		}
	}

	return &structpb.Struct{Fields: fields}, nil
}

// normalizeAnalyticsHeaderValue converts a captured header value out of
// analytics metadata into the flat map[string]string shape the correlation store
// carries. Before this store existed, EVERY shape below reached the ALS side only
// after a JSON-encode (here) -> Envoy echo -> JSON-decode round trip; this
// reproduces that same flattening natively, so switching to the in-process store
// is not a behavior change for any policy's contribution regardless of its shape:
//
//   - map[string]string -- the analytics system policy's own capture (see
//     flattenHeaders in gateway/system-policies/analytics/analytics.go) and any
//     third-party policy already producing this shape: used as-is.
//   - map[string][]string -- finalizeAnalyticsHeaders' output (the
//     AnalyticsHeaderFilter path in translator.go): flattened to each header's
//     FIRST value only, matching what the old round trip produced (JSON-encoding
//     a map[string][]string, then decoding it back, discarded every value but the
//     first -- see parseHeadersFromString's multi-value fallback branch in
//     internal/analytics/publishers/log.go).
//   - string -- a policy that already JSON-encodes its own capture (e.g. a
//     third-party/Python policy mirroring the pre-existing convention): decoded
//     the same way parseHeadersFromString always has.
//
// Any other shape yields nil, exactly as today's metadata-decode path would
// silently ignore a value it doesn't recognize.
func normalizeAnalyticsHeaderValue(v any) map[string]string {
	switch headers := v.(type) {
	case nil:
		return nil
	case map[string]string:
		return headers
	case map[string][]string:
		out := make(map[string]string, len(headers))
		for k, vs := range headers {
			if len(vs) > 0 {
				out[k] = vs[0]
			}
		}
		return out
	case string:
		return parseJSONHeaderString(headers)
	default:
		return nil
	}
}

// parseJSONHeaderString mirrors internal/analytics/publishers/log.go's
// parseHeadersFromString exactly (duplicated rather than imported: that package
// depends on this one's sibling internal/analytics tree, and importing it here
// would risk a cycle for a four-line helper).
func parseJSONHeaderString(raw string) map[string]string {
	if raw == "" {
		return nil
	}
	var single map[string]string
	if err := json.Unmarshal([]byte(raw), &single); err == nil {
		return single
	}
	var multi map[string][]string
	if err := json.Unmarshal([]byte(raw), &multi); err == nil {
		out := make(map[string]string, len(multi))
		for k, vs := range multi {
			if len(vs) > 0 {
				out[k] = vs[0]
			}
		}
		return out
	}
	return nil
}

// extractMetadataFromRouteMetadata extracts the metadata from the route metadata
func extractMetadataFromRouteMetadata(routeMeta RouteMetadata) map[string]interface{} {
	metadata := make(map[string]interface{})
	if routeMeta.APIName != "" {
		metadata[APINameKey] = routeMeta.APIName
	}
	if routeMeta.APIVersion != "" {
		metadata[APIVersionKey] = routeMeta.APIVersion
	}
	if routeMeta.Context != "" {
		metadata[APIContextKey] = routeMeta.Context
	}
	if routeMeta.OperationPath != "" {
		metadata[OperationPathKey] = routeMeta.OperationPath
	}
	if routeMeta.APIKind != "" {
		metadata[APIKindKey] = routeMeta.APIKind
	}
	if routeMeta.ProjectID != "" {
		metadata[ProjectIDKey] = routeMeta.ProjectID
	}
	return metadata
}
