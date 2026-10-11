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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"sync/atomic"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/correlation"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/headers"
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

// The captured header and body keys (request_headers, response_headers,
// request_payload, response_payload) are defined once, in internal/analytics, and
// shared with the ALS side. The analytics system policy is a separate Go module and
// spells out the same names by documented convention.
const (
	// analyticsInternalLoopbackKey is the marker the analytics system policy stamps
	// on the LLM proxy's internal loopback hop, by the same convention.
	analyticsInternalLoopbackKey = "x-wso2-internal-loopback"
)

// correlationTokenPrefix makes tokens unique across policy-engine restarts, and
// correlationTokenSeq unique within one process.
var (
	correlationTokenPrefix = func() string {
		b := make([]byte, 6)
		if _, err := rand.Read(b); err != nil {
			panic(fmt.Sprintf("generating correlation token prefix: %v", err))
		}
		return hex.EncodeToString(b) + "-"
	}()
	correlationTokenSeq atomic.Uint64
)

// newCorrelationToken returns a fresh correlation-store key. The request id cannot
// serve as the key: Envoy keeps a client-supplied x-request-id, so concurrent
// requests can share one and would overwrite or consume each other's entry.
func newCorrelationToken() string {
	return correlationTokenPrefix + strconv.FormatUint(correlationTokenSeq.Add(1), 36)
}

// correlatesInProcess reports whether this request may hand captured data to the
// ALS handler through the correlation store: the store exists and the request is
// not the LLM proxy's internal loopback hop, whose own access-log event is
// suppressed, so it keeps its data in Envoy metadata. data is the analytics
// metadata being built, which can carry the loopback marker before
// execCtx.analyticsMetadata does (e.g. on a short-circuit); it may be nil.
func correlatesInProcess(execCtx *PolicyExecutionContext, data map[string]any) bool {
	if execCtx == nil || execCtx.server == nil || execCtx.server.correlationStore == nil {
		return false
	}
	if _, ok := data[analyticsInternalLoopbackKey]; ok {
		return false
	}
	_, loopback := execCtx.analyticsMetadata[analyticsInternalLoopbackKey]
	return !loopback
}

// isCorrelatedField reports whether key is one of the captured header or body
// fields the correlation store carries.
func isCorrelatedField(key string) bool {
	switch key {
	case analytics.RequestHeadersKey, analytics.ResponseHeadersKey,
		analytics.RequestPayloadKey, analytics.ResponsePayloadKey:
		return true
	}
	return false
}

// noteRoutedPath records the :path a phase's policies rewrote the request to, so
// the ignore-path decision also covers the rewritten path (see routedPath).
func (ec *PolicyExecutionContext) noteRoutedPath(path *string) {
	if ec == nil || path == nil || *path == ec.routedPath {
		return
	}
	ec.routedPath = *path
	ec.pathIgnoredKnown = false
}

// pathIgnoredByCollector reports whether Envoy may skip this request's access-log
// entry because its client path or rewritten path is under
// collector.ignore_path_prefixes. Cached until the path is rewritten again.
func (ec *PolicyExecutionContext) pathIgnoredByCollector() bool {
	if !ec.pathIgnoredKnown {
		store := ec.server.correlationStore
		ec.pathIgnored = store.IgnoresPath(ec.clientPath) ||
			(ec.routedPath != "" && store.IgnoresPath(ec.routedPath))
		ec.pathIgnoredKnown = true
	}
	return ec.pathIgnored
}

// sameFieldValue reports whether a and b are the same captured value: the same
// string, or the same map instance. Response phases re-send the request-phase
// analytics unchanged, so this avoids merging those fields again.
func sameFieldValue(a, b any) bool {
	switch av := a.(type) {
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case map[string]string, map[string][]string, map[string]interface{}:
		ra, rb := reflect.ValueOf(a), reflect.ValueOf(b)
		return ra.Type() == rb.Type() && ra.UnsafePointer() == rb.UnsafePointer()
	}
	return false
}

// storeInProcess hands one captured header or body field to the correlation store
// and reports whether the store accepted it; only then may the field be left out
// of Envoy metadata. It runs while the ext_proc response for this phase is being
// built, before that response is sent, so Envoy cannot emit the request's
// access-log entry before the data is in the store. A field the store refuses (no
// free slot, a body over the size limit, headers and bodies over the byte budget)
// stays in metadata, the pre-store path. The caller has already checked that key
// is a correlated field and that the request's path is not ignored.
//
// The stream's token is created only when the store accepts its first field, so a
// stream whose fields all stay in metadata sends no token and its access-log entry
// is not counted as a store miss.
//
// A header field always replaces the stored one, even when it is empty or of a
// shape that cannot be decoded: a later phase may have filtered the headers an
// earlier phase captured (analytics header filter), and the stored copy must not
// outlive that. When a newer value of an already stored field is refused, the
// stored copy is cleared, so the newer value left in metadata is the one logged.
func storeInProcess(execCtx *PolicyExecutionContext, key string, value any) bool {
	if prev, ok := execCtx.storedFields[key]; ok && sameFieldValue(prev, value) {
		return true
	}
	var p correlation.Payload
	var field correlation.Field
	switch key {
	case analytics.RequestHeadersKey:
		p.RequestHeaders, field = capturedHeaders(value), correlation.FieldRequestHeaders
	case analytics.ResponseHeadersKey:
		p.ResponseHeaders, field = capturedHeaders(value), correlation.FieldResponseHeaders
	case analytics.RequestPayloadKey:
		p.RequestBody, _ = value.(string)
		field = correlation.FieldRequestBody
	case analytics.ResponsePayloadKey:
		p.ResponseBody, _ = value.(string)
		field = correlation.FieldResponseBody
	}
	store := execCtx.server.correlationStore
	var ok bool
	if execCtx.correlationToken != "" {
		ok = store.Update(execCtx.correlationToken, p)
	} else if token := newCorrelationToken(); store.Merge(token, p) {
		execCtx.correlationToken, ok = token, true
	}
	if !ok {
		if _, stored := execCtx.storedFields[key]; stored {
			store.Clear(execCtx.correlationToken, field)
			delete(execCtx.storedFields, key)
		}
		return false
	}
	if execCtx.storedFields == nil {
		execCtx.storedFields = make(map[string]any, 4)
	}
	execCtx.storedFields[key] = value
	return true
}

// capturedHeaders returns a captured header value in a shape the store and every
// publisher understand, keeping repeated values separate when the source did
// (analytics header filter): map[string]string and map[string][]string are kept
// as they are, anything else is decoded with headers.Values. An empty or
// undecodable value becomes an empty map: "no headers", which still replaces an
// earlier stored value.
func capturedHeaders(value any) any {
	switch h := value.(type) {
	case map[string]string:
		if h != nil {
			return h
		}
	case map[string][]string:
		if h != nil {
			return h
		}
	default:
		if v := headers.Values(value); v != nil {
			return v
		}
	}
	return map[string]string{}
}

// releaseCorrelationEntry moves a stream's stored fields back into fields (the
// analytics_data about to be sent to Envoy) and drops its entry and token. It runs
// when a policy rewrote the path into collector.ignore_path_prefixes after fields
// were stored: Envoy may then never send the access-log entry, so the entry would
// be orphaned, while a route that does not rewrite still logs the request and
// needs the fields back in metadata. Fields this phase sets itself are left as
// they are.
func releaseCorrelationEntry(execCtx *PolicyExecutionContext, analyticsData map[string]any, fields map[string]*structpb.Value) {
	stored, ok := execCtx.server.correlationStore.Take(execCtx.correlationToken)
	execCtx.correlationToken = ""
	execCtx.storedFields = nil
	if !ok {
		return
	}
	restore := func(key string, value any, set bool) {
		if _, own := analyticsData[key]; own || !set {
			return
		}
		if v, err := convertToStructValue(value); err == nil {
			fields[key] = v
		}
	}
	restore(analytics.RequestHeadersKey, stored.RequestHeaders, stored.RequestHeaders != nil)
	restore(analytics.ResponseHeadersKey, stored.ResponseHeaders, stored.ResponseHeaders != nil)
	restore(analytics.RequestPayloadKey, stored.RequestBody, stored.RequestBody != "")
	restore(analytics.ResponsePayloadKey, stored.ResponseBody, stored.ResponseBody != "")
}

// encodeAnalyticsValue is convertToStructValue that, for captured header and body
// fields, reuses the previous phase's encoding when the same value is sent again.
// Later phases re-send the request-phase analytics unchanged, and a header map is
// JSON-encoded on each send when it stays in metadata (store off, TCP mode, or a
// refused field).
func encodeAnalyticsValue(execCtx *PolicyExecutionContext, key string, value any, correlated bool) (*structpb.Value, error) {
	if !correlated || execCtx == nil {
		return convertToStructValue(value)
	}
	if prev, ok := execCtx.encodedFields[key]; ok && sameFieldValue(prev.value, value) {
		return prev.encoded, nil
	}
	val, err := convertToStructValue(value)
	if err != nil {
		return nil, err
	}
	if execCtx.encodedFields == nil {
		execCtx.encodedFields = make(map[string]encodedField, 4)
	}
	execCtx.encodedFields[key] = encodedField{value: value, encoded: val}
	return val, nil
}

// encodedField is a captured field's value and its encoding for analytics_data.
type encodedField struct {
	value   any
	encoded *structpb.Value
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
// correlation store (internal/analytics/correlation, keyed by the stream's
// correlation token) and left
// out of the struct sent to Envoy, but only when the store accepts them (see
// storeInProcess); the struct then carries the stream's correlation token
// (analytics.CorrelationTokenKey) instead. They used to make a full round trip -- encoded here, forwarded
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
	ignored := inProcess && execCtx.pathIgnoredByCollector()
	for key, value := range analyticsData {
		// The token key is reserved for the stream's own token (added below). A
		// policy-supplied value could point this request's access-log entry at
		// another request's stored fields.
		if key == analytics.CorrelationTokenKey {
			continue
		}
		correlated := isCorrelatedField(key)
		if inProcess && !ignored && correlated && storeInProcess(execCtx, key, value) {
			continue
		}
		val, err := encodeAnalyticsValue(execCtx, key, value, correlated)
		if err != nil {
			return nil, fmt.Errorf("failed to convert analytics value for key %s: %w", key, err)
		}
		fields[key] = val
	}
	if ignored && execCtx.correlationToken != "" {
		releaseCorrelationEntry(execCtx, analyticsData, fields)
	}
	// Every phase repeats the token once the stream has one, so whichever
	// analytics_data Envoy ends up with tells the ALS handler where this request's
	// stored fields are.
	if execCtx != nil && execCtx.correlationToken != "" {
		fields[analytics.CorrelationTokenKey] = structpb.NewStringValue(execCtx.correlationToken)
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
