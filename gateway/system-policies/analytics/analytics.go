package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	"github.com/wso2/api-platform/sdk/core/utils"
)

const (
	// Analytics metadata keys for LLM token information
	// These match the keys defined in policy-engine/internal/analytics/analytics.go
	PromptTokenCountMetadataKey      = "aitoken:prompttokencount"
	CompletionTokenCountMetadataKey  = "aitoken:completiontokencount"
	TotalTokenCountMetadataKey       = "aitoken:totaltokencount"
	ModelIDMetadataKey               = "aitoken:modelid"
	AIProviderNameMetadataKey        = "ai:providername"
	AIProviderDisplayNameMetadataKey = "ai:providerdisplayname"
	ApplicationIDMetadataKey         = "x-wso2-application-id"
	ApplicationNameMetadataKey       = "x-wso2-application-name"
	InternalLoopbackMetadataKey      = "x-wso2-internal-loopback"

	// Auth-context metadata keys. Populated generically (auth-type-agnostic, via
	// SharedContext.AuthContext) by populateAuthAnalyticsMetadata below, so they work
	// uniformly for jwt-auth, opaque-token-auth, api-key-auth, mcp-auth, or any future
	// auth policy without that policy needing to export anything itself. Consumed by the
	// policy-engine's prepareAnalyticEvent (internal/analytics/analytics.go), which mirrors
	// these key names, and from there exposed to the stdout traffic-logging publisher's
	// global "$ctx:auth.*" properties.
	AuthUserIDMetadataKey       = "x-wso2-user-id"
	AuthTypeMetadataKey         = "x-wso2-auth-type"
	AuthIssuerMetadataKey       = "x-wso2-auth-issuer"
	AuthCredentialIDMetadataKey = "x-wso2-auth-credential-id"
	AuthTokenIDMetadataKey      = "x-wso2-auth-token-id"
	AuthAudienceMetadataKey     = "x-wso2-auth-audience"
	AuthScopesMetadataKey       = "x-wso2-auth-scopes"
	AuthPropertiesMetadataKey   = "x-wso2-auth-properties"
	AuthAuthorizedMetadataKey   = "x-wso2-auth-authorized"

	// Subscription metadata keys for subscription and monetization information.
	BillingCustomerIDMetadataKey     = "x-wso2-billing-customer-id"
	BillingSubscriptionIDMetadataKey = "x-wso2-billing-subscription-id"
	SubscriptionStatusMetadataKey    = "x-wso2-subscription-status"
	SubscriptionPlanNameMetadataKey  = "x-wso2-subscription-plan-name"

	// GenericMetadataKey carries the JSON-encoded contents of SharedContext.Metadata
	// (minus internal/reserved keys), populated by populateGenericMetadata below. Unlike
	// every other metadata key in this file, this is a raw, unfiltered passthrough of a
	// generic map[string]interface{} bag ANY policy (including third-party/Python
	// policies) can write to — not a specific named field extracted from a strongly-typed
	// source. Consumed by the policy-engine's prepareAnalyticEvent (internal/analytics/
	// analytics.go), which mirrors this key name, and from there exposed to the stdout
	// traffic-logging publisher's global "$ctx:metadata['<key>']" property.
	GenericMetadataKey = "x-wso2-metadata"

	// Lazy resource type for LLM provider templates
	lazyResourceTypeLLMProviderTemplate = "LlmProviderTemplate"
	// Lazy resource type for provider-to-template mapping
	lazyResourceTypeProviderTemplateMapping = "ProviderTemplateMapping"

	// SharedContext.Metadata key used to accumulate streaming response body chunks.
	// Deleted after EndOfStream processing to avoid memory leaks.
	analyticsStreamAccKey = "__analytics_stream_acc"

	// A2ARequestPropertiesKey and A2AResponsePropertiesKey carry the JSON-encoded
	// A2A analytics properties for an Agent request and its response. Consumed by
	// the policy-engine's prepareAnalyticEvent, which mirrors these key names as its
	// own constants — the two live in separate Go modules and cannot share one, so
	// each side has a test asserting the spelling (the same arrangement as the
	// auth-context keys above).
	A2ARequestPropertiesKey  = "a2a_request_properties"
	A2AResponsePropertiesKey = "a2a_response_properties"

	// SharedContext.Metadata keys used to time an A2A streaming response. Written
	// only for Agent requests and deleted when the stream ends, so they cost nothing
	// on any other kind.
	a2aRequestStartKey = "__a2a_request_start"
	a2aFirstEventKey   = "__a2a_first_event"

	// a2aStreamScanKey is how far the forwarded bytes have been scanned for the first
	// complete SSE event. Held so a stream that opens with a long run of heartbeats is
	// not rescanned from the start on every chunk. Dropped once an event is found.
	a2aStreamScanKey = "__a2a_stream_scan"
)

// A2A resolution attribute names, as the a2a resolver in the policy engine spells
// them into SharedContext.ResolutionAttributes.
//
// Read, never written: the resolver owns this map and a policy treats it as
// read-only. Mirrored string literals again, for the module boundary reason above.
const (
	a2aAttrMessageID       = "a2a.message.id"
	a2aAttrContextID       = "a2a.context.id"
	a2aAttrTaskID          = "a2a.task.id"
	a2aAttrTransport       = "a2a.transport"
	a2aAttrProtocolVersion = "a2a.protocol.version"
)

// a2aTransportHTTPJSON is the transport whose operation arguments travel in the path
// and the query string rather than in a request envelope. Spelled here because the
// value is the resolver's, mirrored across the module boundary like everything else
// above; the JSON-RPC value is never compared against, so it is not spelled at all.
const a2aTransportHTTPJSON = "HTTP+JSON"

// A2A response payload types.
//
// Closed set, and deliberately so: this is a dimension a dashboard groups by, and it
// is derived from an agent-authored document. Every value below corresponds to one of
// the response shapes A2A 1.0 defines for its eleven operations — the four members of
// the SendMessage/StreamResponse union, the four fixed collection and configuration
// results, plus the three outcomes that are not a document at all. Anything else the
// agent sends classifies as a2aPayloadUnknown rather than contributing a new value.
const (
	a2aPayloadTask           = "task"
	a2aPayloadMessage        = "message"
	a2aPayloadStatusUpdate   = "status_update"
	a2aPayloadArtifactUpdate = "artifact_update"
	a2aPayloadTaskList       = "task_list"
	a2aPayloadPushConfig     = "push_notification_config"
	a2aPayloadPushConfigList = "push_notification_config_list"
	a2aPayloadAgentCard      = "agent_card"
	a2aPayloadEmpty          = "empty"
	a2aPayloadError          = "error"
	a2aPayloadUnknown        = "unknown"
)

// maxA2AObservedValueBytes bounds an identifier or state read out of a response before
// it reaches an event.
//
// It is the same ceiling the a2a resolver applies to the identifiers it takes off the
// request (MaxResolutionAttributeValueBytes), for the same reason and with the same
// treatment: an over-long value is dropped, never truncated, because a truncated
// identifier is not a shorter identifier — it is a different one, and correlating on
// it would silently group unrelated requests.
const maxA2AObservedValueBytes = 256

// a2aTaskStates is the closed set of A2A 1.0 task states, keyed by the canonical
// protocol-enum spelling this policy reports.
//
// A state is validated against this set rather than passed through, because it is
// agent-authored and the section's cardinality rule sanctions it as a metric
// dimension — an unrecognised value is dropped, so a misbehaving agent cannot widen
// the dimension. See normalizeA2ATaskState for the spellings accepted on the way in.
var a2aTaskStates = map[string]struct{}{
	"TASK_STATE_UNSPECIFIED":    {},
	"TASK_STATE_SUBMITTED":      {},
	"TASK_STATE_WORKING":        {},
	"TASK_STATE_COMPLETED":      {},
	"TASK_STATE_FAILED":         {},
	"TASK_STATE_CANCELED":       {},
	"TASK_STATE_INPUT_REQUIRED": {},
	"TASK_STATE_REJECTED":       {},
	"TASK_STATE_AUTH_REQUIRED":  {},
}

var (
	// JSON Path expressions to extract MCP analytics properties from response body
	JsonRpcMethodJsonPath     = "$.method"
	McpCapabilityNameJsonPath = "$.params.name"
	McpResourceUriJsonPath    = "$.params.uri"
	ProtocolVersionJsonPath   = "$.params.protocolVersion"
	ClientNameJsonPath        = "$.params.clientInfo.name"
	ClientVersionJsonPath     = "$.params.clientInfo.version"

	ServerProtocolVersionJsonPath = "$.result.protocolVersion"
	ServerInfoNameJsonPath        = "$.result.serverInfo.name"
	ServerInfoVersionJsonPath     = "$.result.serverInfo.version"
	IsErrorJsonPath               = "$.result.isError"
	JsonRpcErrorCodeJsonPath      = "$.error.code"
)

// AnalyticsPolicy implements the default analytics data collection process.
type AnalyticsPolicy struct{}

type McpRequestAnalyticsProperties struct {
	JsonRpcMethod  string         `json:"jsonRpcMethod,omitempty"`
	Capability     string         `json:"capability,omitempty"`
	CapabilityName string         `json:"capabilityName,omitempty"`
	ClientInfo     *McpClientInfo `json:"clientInfo,omitempty"`
}

type McpClientInfo struct {
	RequestedProtocolVersion string `json:"requestedProtocolVersion"`
	Name                     string `json:"name"`
	Version                  string `json:"version"`
}

type McpServerInfo struct {
	ProtocolVersion string `json:"protocolVersion,omitempty"`
	Name            string `json:"name,omitempty"`
	Version         string `json:"version,omitempty"`
}

type McpResponseAnalyticsProperties struct {
	IsError    *bool          `json:"isError,omitempty"`
	ErrorCode  *int           `json:"errorCode,omitempty"`
	ServerInfo *McpServerInfo `json:"serverInfo,omitempty"`
}

// A2ARequestAnalyticsProperties are the request-side A2A dimensions.
//
// The first five are not parsed by this policy: the a2a resolver extracted them in the
// same pass that identified the operation, and this copies them out of
// SharedContext.ResolutionAttributes. That is the point — MCP's equivalent unmarshals
// the request body here, and the body is then unmarshalled again elsewhere for the
// same request.
//
// Transport and ProtocolVersion are bounded (a two-valued enum and a registry entry);
// the three identifiers are opaque and caller-controlled, so they belong in an event
// or a trace and never in a metric label or a rate-limit key.
//
// The last three are request *summaries* rather than identifiers, and they are the
// one part of an A2A request this policy does read for itself. They are analytics-only
// facts — nothing else in the gateway keys, routes or limits on how many parts a
// message had — so putting them in the resolver would widen its closed attribute set
// for a single consumer. All three are numbers or a boolean, so none of them can
// widen a dimension the way a caller-supplied string would; they are event values and
// histogram inputs, never metric labels.
//
// Each is a pointer because its zero value is meaningful: a message may genuinely
// carry zero parts, a history length of zero is a request for no history at all, and
// returnImmediately false is the protocol's own default rather than an absent field.
type A2ARequestAnalyticsProperties struct {
	Transport       string `json:"transport,omitempty"`
	ProtocolVersion string `json:"protocolVersion,omitempty"`
	MessageID       string `json:"messageId,omitempty"`
	ContextID       string `json:"contextId,omitempty"`
	TaskID          string `json:"taskId,omitempty"`

	InputPartCount    *int  `json:"inputPartCount,omitempty"`
	ReturnImmediately *bool `json:"returnImmediately,omitempty"`
	HistoryLength     *int  `json:"historyLength,omitempty"`
}

// A2AResponseAnalyticsProperties are the response-side A2A dimensions — the ones only
// a policy that sees the response body can produce.
//
// IsError is what makes an A2A success rate correct rather than plausible: a JSON-RPC
// error travels inside a 200, so a rate computed from the HTTP status counts failed
// invocations as successes. It is always emitted for a response this policy could
// read, so a consumer can distinguish "succeeded" from "not determined".
//
// The streaming timings are measured by this policy from the gateway's own clock,
// because the ALS timepoints the event's Latencies come from cannot express them: a
// streaming response's last upstream byte arrives when the stream ends, so the backend
// latency of a stream is its whole duration and says nothing about when the first
// event reached the client.
//
// The error *message* is deliberately absent. It is agent-authored free text of
// unbounded length and unknown sensitivity; the code is the dimension a dashboard
// groups by.
//
// PayloadType and TaskState are bounded enums and safe to group by. The two
// identifiers are not, and they are reported separately from the request's rather than
// folded into them: an agent generates a task id and a context id the caller never
// sent, so overwriting the request's values would hide the moment correlation actually
// begins, and a mismatch between what was asked for and what came back would stop
// being diagnosable. The published model is one flat object, so they carry a `response`
// prefix — the only two fields here that need one, since no other response fact has a
// request-side counterpart.
//
// The field names here are the published names. This struct is serialized at the Envoy
// metadata boundary and unmarshalled on the other side into the collector's
// dto.A2AResponseAnalytics, so the two must agree; TestA2AKeySpellingsMatchThePolicyEngine
// pins them, because a one-sided rename is silent — the dimension just stops appearing.
type A2AResponseAnalyticsProperties struct {
	IsError            *bool  `json:"isError,omitempty"`
	ErrorCode          *int   `json:"errorCode,omitempty"`
	TimeToFirstEventMs *int64 `json:"timeToFirstEventMs,omitempty"`
	StreamDurationMs   *int64 `json:"streamDurationMs,omitempty"`
	IsStreaming        *bool  `json:"isStreaming,omitempty"`

	PayloadType       string `json:"payloadType,omitempty"`
	ResponseTaskID    string `json:"responseTaskId,omitempty"`
	ResponseContextID string `json:"responseContextId,omitempty"`
	TaskState         string `json:"taskState,omitempty"`
}

// LLMProviderAnalyticsInfo holds extracted token-related information from LLM provider responses
type LLMProviderAnalyticsInfo struct {
	ProviderName        *string // Provider name
	ProviderDisplayName *string // Provider display name
	PromptTokens        *int64  // Number of prompt tokens
	CompletionTokens    *int64  // Number of completion tokens
	TotalTokens         *int64  // Total number of tokens
	RemainingTokens     *int64  // Remaining tokens from rate limit headers
	RequestModel        *string // Model name from request
	ResponseModel       *string // Model name from response
}

var ins = &AnalyticsPolicy{}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	return ins, nil
}

// GetPolicyV2 is an alias for GetPolicy, provided for compatibility with the
// Builder-generated plugin registry which calls GetPolicyV2 on all plugins.
func GetPolicyV2(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	return GetPolicy(metadata, params)
}

// Mode returns the processing mode for this policy.
// ResponseBodyMode is BodyModeStream so the kernel keeps streaming enabled when
// all other policies in the chain also support streaming. The buffered fallback
// (OnResponseBody) is still called when the chain cannot stream.
func (a *AnalyticsPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeBuffer,
		ResponseHeaderMode: policy.HeaderModeProcess,
		ResponseBodyMode:   policy.BodyModeStream,
	}
}

// OnRequestHeaders collects analytics data available at the request-headers phase.
// OnRequestBody is called inline for bodyless requests when RequiresRequestBody is true,
// but OnRequestHeaders acts as a safety net for chains where that condition does not hold,
// and provides early capture of header-sourced analytics (application ID, MCP session ID).
func (a *AnalyticsPolicy) OnRequestHeaders(_ context.Context, reqCtx *policy.RequestHeaderContext, params map[string]interface{}) policy.RequestHeaderAction {
	slog.Debug("Analytics system policy: OnRequestHeaders called")
	analyticsMetadata := make(map[string]any)

	if reqCtx.Headers != nil {
		if appIDs := reqCtx.Headers.Get("x-wso2-application-id"); len(appIDs) > 0 {
			analyticsMetadata[ApplicationIDMetadataKey] = appIDs[0]
		}
		if appNames := reqCtx.Headers.Get("x-wso2-application-name"); len(appNames) > 0 {
			analyticsMetadata[ApplicationNameMetadataKey] = appNames[0]
		}
		// Marker stamped by the proxy on its internal loopback forward to the provider.
		// Normalized to "true" so the policy-engine can drop the duplicate provider hop.
		if loopback := reqCtx.Headers.Get(InternalLoopbackMetadataKey); len(loopback) > 0 && loopback[0] != "" {
			analyticsMetadata[InternalLoopbackMetadataKey] = "true"
		}
	}

	if reqCtx.SharedContext.APIKind == policy.APIKindMCP && reqCtx.Headers != nil {
		if sessionIDs := reqCtx.Headers.Get("mcp-session-id"); len(sessionIDs) > 0 {
			analyticsMetadata["mcp_session_id"] = sessionIDs[0]
		}
	}

	// A2A request dimensions are emitted here rather than only at the body phase
	// because this is the phase every A2A operation reaches with everything the
	// resolver stamped: seven of the eleven are GETs or DELETEs with no request body
	// at all. The resolver has already stamped the identifiers on both transports — a
	// body-resolved route runs its header policies at the request-body callback, after
	// the chain was bound — and the query string is on the path here, which is where
	// GetTask and ListTasks put their history length on the HTTP+JSON binding.
	//
	// The body-sourced summaries cannot be read here (RequestHeaderContext carries no
	// body), so OnRequestBody re-emits this block as a superset for a request that has
	// one. See populateA2ARequestMetadata.
	if reqCtx.SharedContext.APIKind == policy.APIKindAgent {
		populateA2ARequestMetadata(analyticsMetadata, reqCtx.SharedContext, reqCtx.Path, nil)
	}

	// Capture all request headers when enabled, so they flow into analytics events
	// (and the stdout/log publisher) without attaching a per-API header policy.
	if sendReqHeaders, _ := getHeaderFlags(params); sendReqHeaders && reqCtx.Headers != nil {
		if headers := serializeHeaders(reqCtx.Headers); headers != "" {
			analyticsMetadata["request_headers"] = headers
		}
	}

	if len(analyticsMetadata) > 0 {
		return policy.UpstreamRequestHeaderModifications{AnalyticsMetadata: analyticsMetadata}
	}
	return policy.UpstreamRequestHeaderModifications{}
}

// populateAuthAnalyticsMetadata walks authChain (handling layered/multi-auth via
// AuthContext.Previous) and, for the first layer that is both authenticated and has a
// non-empty subject, stamps auth-derived analytics metadata into analyticsMetadata —
// auth-type-agnostic, so it works uniformly for jwt-auth, opaque-token-auth, api-key-auth,
// mcp-auth, or any future auth policy without that policy needing to export anything
// itself; it only relies on the common AuthContext type any auth policy already populates.
// Scopes are sorted and space-joined and audience is comma-joined, matching the
// log-message policy's own $ctx:auth.* serialization choices for consistency across
// per-API and global traffic-log properties. Custom claims (Properties) are JSON-encoded,
// mirroring how captured headers are already carried as a JSON string elsewhere in this
// file. Fields are omitted (not empty-stringed) when absent, consistent with every other
// optional key in this policy.
func populateAuthAnalyticsMetadata(analyticsMetadata map[string]any, authChain *policy.AuthContext) {
	for authCtx := authChain; authCtx != nil; authCtx = authCtx.Previous {
		if !authCtx.Authenticated || authCtx.Subject == "" {
			continue
		}

		analyticsMetadata[AuthUserIDMetadataKey] = authCtx.Subject
		if authCtx.AuthType != "" {
			analyticsMetadata[AuthTypeMetadataKey] = authCtx.AuthType
		}
		if authCtx.Issuer != "" {
			analyticsMetadata[AuthIssuerMetadataKey] = authCtx.Issuer
		}
		if authCtx.CredentialID != "" {
			analyticsMetadata[AuthCredentialIDMetadataKey] = authCtx.CredentialID
		}
		if authCtx.TokenId != "" {
			analyticsMetadata[AuthTokenIDMetadataKey] = authCtx.TokenId
		}
		if len(authCtx.Audience) > 0 {
			analyticsMetadata[AuthAudienceMetadataKey] = strings.Join(authCtx.Audience, ",")
		}
		if len(authCtx.Scopes) > 0 {
			scopes := make([]string, 0, len(authCtx.Scopes))
			for name := range authCtx.Scopes {
				scopes = append(scopes, name)
			}
			sort.Strings(scopes)
			analyticsMetadata[AuthScopesMetadataKey] = strings.Join(scopes, " ")
		}
		if len(authCtx.Properties) > 0 {
			if data, err := json.Marshal(authCtx.Properties); err == nil {
				analyticsMetadata[AuthPropertiesMetadataKey] = string(data)
			} else {
				slog.Warn("Analytics system policy: failed to marshal auth properties", "error", err)
			}
		}
		// Authorized is distinct from Authenticated (which this block already gates on):
		// it reflects a separate authorization check (e.g. mcp-authz) and can genuinely be
		// false even for an authenticated request, so it's always stamped rather than
		// omitted-when-zero like the optional fields above.
		analyticsMetadata[AuthAuthorizedMetadataKey] = strconv.FormatBool(authCtx.Authorized)

		slog.Debug("Analytics system policy: auth-context metadata extracted",
			"subject", authCtx.Subject, "authType", authCtx.AuthType)
		return
	}
}

// internalMetadataKeys are the SharedContext.Metadata entries this policy writes for
// its own bookkeeping, and the ones populateGenericMetadata leaves out of the export.
//
// None of them is an analytics value. The accumulator is the streamed response body
// held for later parsing, so exporting it would duplicate the whole body — base64'd,
// since it is a []byte — into every traffic-log line. The three A2A marks are a
// request timestamp, a first-event timestamp and a scan offset: intermediate state the
// policy turns into TTFB and stream duration, which are exported properly under
// A2AResponsePropertiesKey. The marks are also written and cleared in different
// phases from this export — a2aRequestStartKey is set in the request phase and
// deleted only when the stream ends, while this runs at response headers — so
// without this filter every Agent request carries an internal timestamp into its
// traffic-log line as a "$ctx:metadata['__a2a_request_start']" field an operator has
// no use for.
//
// Named individually rather than filtered by the "__" prefix they happen to share:
// this bag is writable by any policy, third-party ones included, and a prefix rule
// would silently swallow a key someone else chose to spell that way.
var internalMetadataKeys = map[string]struct{}{
	analyticsStreamAccKey: {},
	a2aRequestStartKey:    {},
	a2aFirstEventKey:      {},
	a2aStreamScanKey:      {},
}

// populateGenericMetadata JSON-encodes SharedContext.Metadata (minus this policy's own
// internal bookkeeping keys, which are never meant for export) into analyticsMetadata
// under GenericMetadataKey, so the stdout traffic-logging publisher's
// "$ctx:metadata['<key>']" CEL surface can reach ANY key any policy has written to
// the shared metadata bag. Unlike populateAuthAnalyticsMetadata and the
// subscription-field copy below, which extract specific named fields from a
// strongly-typed source, this is a raw, unfiltered passthrough: any policy author
// writing to SharedContext.Metadata should assume its value is visible in traffic
// logs, since there is no masking config for this path (unlike masked_headers for
// captured headers).
func populateGenericMetadata(analyticsMetadata map[string]any, sharedMetadata map[string]interface{}) {
	if len(sharedMetadata) == 0 {
		return
	}
	filtered := make(map[string]interface{}, len(sharedMetadata))
	for k, v := range sharedMetadata {
		if _, internal := internalMetadataKeys[k]; internal {
			continue
		}
		filtered[k] = v
	}
	if len(filtered) == 0 {
		return
	}
	data, err := json.Marshal(filtered)
	if err != nil {
		slog.Warn("Analytics system policy: failed to marshal SharedContext.Metadata for traffic logging", "error", err)
		return
	}
	analyticsMetadata[GenericMetadataKey] = string(data)
}

// OnResponseHeaders collects analytics data available at the response-headers phase.
// Auth context and response headers are already populated here, so we emit them early
// rather than waiting for the body phase (which may not be reached for header-only responses).
func (a *AnalyticsPolicy) OnResponseHeaders(_ context.Context, respCtx *policy.ResponseHeaderContext, params map[string]interface{}) policy.ResponseHeaderAction {
	slog.Debug("Analytics system policy: OnResponseHeaders called")
	analyticsMetadata := make(map[string]any)

	populateAuthAnalyticsMetadata(analyticsMetadata, respCtx.SharedContext.AuthContext)

	// Subscription and monetization fields are written to SharedContext.Metadata by subscription-validation policy
	if md := respCtx.SharedContext.Metadata; md != nil {
		if v, ok := md[BillingCustomerIDMetadataKey].(string); ok && v != "" {
			analyticsMetadata[BillingCustomerIDMetadataKey] = v
		}
		if v, ok := md[BillingSubscriptionIDMetadataKey].(string); ok && v != "" {
			analyticsMetadata[BillingSubscriptionIDMetadataKey] = v
		}
		if v, ok := md[SubscriptionStatusMetadataKey].(string); ok && v != "" {
			analyticsMetadata[SubscriptionStatusMetadataKey] = v
		}
		if v, ok := md[SubscriptionPlanNameMetadataKey].(string); ok && v != "" {
			analyticsMetadata[SubscriptionPlanNameMetadataKey] = v
		}
	}

	populateGenericMetadata(analyticsMetadata, respCtx.SharedContext.Metadata)

	// Capture the response content type for all API kinds. The Envoy access log does
	// not carry response headers (no additional_response_headers_to_log is configured,
	// to avoid an uncontrolled header source)
	if respCtx.ResponseHeaders != nil {
		if contentTypes := respCtx.ResponseHeaders.Get("content-type"); len(contentTypes) > 0 {
			analyticsMetadata["response_content_type"] = contentTypes[0]
		}
	}

	if respCtx.SharedContext.APIKind == policy.APIKindMCP && respCtx.ResponseHeaders != nil {
		if sessionIDs := respCtx.ResponseHeaders.Get("mcp-session-id"); len(sessionIDs) > 0 {
			analyticsMetadata["mcp_session_id"] = sessionIDs[0]
		}
	}

	// Capture all response headers when enabled.
	if _, sendRespHeaders := getHeaderFlags(params); sendRespHeaders && respCtx.ResponseHeaders != nil {
		if headers := serializeHeaders(respCtx.ResponseHeaders); headers != "" {
			analyticsMetadata["response_headers"] = headers
		}
	}

	if len(analyticsMetadata) > 0 {
		return policy.DownstreamResponseHeaderModifications{AnalyticsMetadata: analyticsMetadata}
	}
	return policy.DownstreamResponseHeaderModifications{}
}

// OnRequestBody performs Analytics collection process during the request phase (buffered).
func (a *AnalyticsPolicy) OnRequestBody(_ context.Context, ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	slog.Debug("Analytics system policy: OnRequestBody called")
	sendReqBody, _ := getPayloadFlags(params)
	analyticsMetadata := make(map[string]any)

	// When request payload capture is enabled, capture the raw request body into analytics metadata.
	if sendReqBody && ctx != nil && ctx.Body != nil && len(ctx.Body.Content) > 0 {
		slog.Debug("Capturing request payload for analytics")
		analyticsMetadata["request_payload"] = string(ctx.Body.Content)
	}

	apiKind := ctx.SharedContext.APIKind
	switch apiKind {
	case policy.APIKindRestApi:
		// Collect analytics data for REST API scenario
	case policy.APIKindLlmProvider:
		// Collect analytics data for AI API(LLM Provider) specific scenario
	case policy.APIKindLlmProxy:
		// Collect analytics data for LLM Proxy specific scenario
	case policy.APIKindMCP:
		// Collect analytics data specific for MCP scenario from request
		if ctx.Headers != nil && len(ctx.Headers.GetAll()) > 0 {
			sessionIDs := ctx.Headers.Get("mcp-session-id")
			if len(sessionIDs) > 0 {
				analyticsMetadata["mcp_session_id"] = sessionIDs[0]
			}
		}

		if ctx != nil && ctx.Body != nil && len(ctx.Body.Content) > 0 {
			var mcpPayload map[string]interface{}
			if err := json.Unmarshal(ctx.Body.Content, &mcpPayload); err != nil {
				slog.Error("Failed to unmarshal MCP request body for analytics", "error", err)
				break
			}

			props := McpRequestAnalyticsProperties{}

			extractString := func(path string) string {
				val, err := utils.ExtractValueFromJsonpath(mcpPayload, path)
				if err != nil || val == nil {
					return ""
				}
				if s, ok := val.(string); ok {
					return s
				}
				return ""
			}

			props.JsonRpcMethod = extractString(JsonRpcMethodJsonPath)
			props.CapabilityName = extractString(McpCapabilityNameJsonPath)
			props.Capability = deriveMCPCapability(props.JsonRpcMethod)

			clientInfo := McpClientInfo{
				RequestedProtocolVersion: extractStringFromJsonpath(mcpPayload, ProtocolVersionJsonPath),
				Name:                     extractStringFromJsonpath(mcpPayload, ClientNameJsonPath),
				Version:                  extractStringFromJsonpath(mcpPayload, ClientVersionJsonPath),
			}
			if clientInfo.RequestedProtocolVersion != "" || clientInfo.Name != "" || clientInfo.Version != "" {
				props.ClientInfo = &clientInfo
			}

			if data, err := json.Marshal(props); err != nil {
				slog.Error("Failed to marshal MCP request analytics properties", "error", err)
			} else {
				analyticsMetadata["mcp_request_properties"] = string(data)
			}
		}
	case policy.APIKindAgent:
		// Three request summaries — how many parts the message carried, whether the
		// caller asked for an immediate return, and how much history it wanted — exist
		// only in the request body, which the header phase cannot see. They are
		// analytics-only facts, so unlike the identifiers they are not in the
		// resolver's closed attribute set and are read here instead.
		//
		// The whole block is rebuilt and re-emitted rather than the three fields being
		// added on their own: the metadata map is keyed, so a second partial write
		// under the same key would replace the header phase's dimensions instead of
		// extending them. Request-header policies always run before request-body
		// policies — including on the JSON-RPC route, where both run at the body
		// callback — so this superset is the value that survives.
		if ctx.Body != nil && len(ctx.Body.Content) > 0 {
			populateA2ARequestMetadata(analyticsMetadata, ctx.SharedContext, ctx.Path, ctx.Body.Content)
		}
	default:
		slog.Error("Invalid API kind")
	}

	if len(analyticsMetadata) > 0 {
		return policy.UpstreamRequestModifications{
			AnalyticsMetadata: analyticsMetadata,
		}
	}
	return nil
}

// OnResponseBody performs Analytics collection during the response phase (buffered fallback).
// Called when the chain is in buffered mode (e.g. another policy does not support streaming).
func (a *AnalyticsPolicy) OnResponseBody(_ context.Context, ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	slog.Debug("Analytics system policy: OnResponseBody called")
	_, sendRespBody := getPayloadFlags(params)

	analyticsMetadata := make(map[string]any)

	populateAuthAnalyticsMetadata(analyticsMetadata, ctx.SharedContext.AuthContext)

	apiKind := ctx.SharedContext.APIKind
	slog.Debug("API kind: ", "apiKind", apiKind)
	switch apiKind {
	case policy.APIKindRestApi:
		// Collect analytics data for REST API specific scenario
	case policy.APIKindLlmProvider, policy.APIKindLlmProxy:
		templateHandle, ok := ctx.SharedContext.Metadata["template_handle"].(string)
		slog.Info("Template handle(extracted from route metadata): ", "templateHandle", templateHandle)
		if !ok || templateHandle == "" {
			slog.Debug("No template handle found in route metadata for LLM API")
		} else {
			template, err := getTemplateByHandle(templateHandle)
			if err != nil {
				slog.Warn("Failed to get template from lazy resource cache", "templateHandle", templateHandle, "error", err)
			} else {
				tokenInfo, err := extractLLMProviderAnalyticsInfo(template, ctx)
				if err != nil {
					slog.Warn("Failed to extract LLM token info", "error", err)
				} else if tokenInfo != nil {
					slog.Debug("Extracted LLM token info",
						"promptTokens", tokenInfo.PromptTokens,
						"completionTokens", tokenInfo.CompletionTokens,
						"totalTokens", tokenInfo.TotalTokens,
						"remainingTokens", tokenInfo.RemainingTokens,
						"requestModel", tokenInfo.RequestModel,
						"responseModel", tokenInfo.ResponseModel,
						"providerName", tokenInfo.ProviderName,
						"providerDisplayName", tokenInfo.ProviderDisplayName,
					)
					populateTokenAnalyticsMetadata(analyticsMetadata, tokenInfo)
				}
			}
		}
	case policy.APIKindMCP:
		if ctx.ResponseHeaders != nil && len(ctx.ResponseHeaders.GetAll()) > 0 {
			if analyticsMetadata["mcp_session_id"] == nil {
				sessionIDs := ctx.ResponseHeaders.Get("mcp-session-id")
				if len(sessionIDs) > 0 {
					analyticsMetadata["mcp_session_id"] = sessionIDs[0]
				}
			}
		}

		if ctx != nil && ctx.ResponseBody != nil && len(ctx.ResponseBody.Content) > 0 {
			responseContent := ctx.ResponseBody.Content

			isSSE := isSSEContent(ctx.ResponseHeaders, responseContent)
			if isSSE {
				jsonData, err := parseSSEFirstDataEvent(responseContent)
				if err != nil {
					slog.Error("Failed to parse SSE response", "error", err)
				} else {
					responseContent = jsonData
				}
			}

			trimmed := bytes.TrimSpace(responseContent)
			if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
				var mcpResponsePayload map[string]interface{}
				if err := json.Unmarshal(trimmed, &mcpResponsePayload); err != nil {
					slog.Warn("Failed to unmarshal MCP response body for server info analytics", "error", err)
				} else {
					props := extractMCPResponseAnalyticsProps(mcpResponsePayload)
					if props != nil {
						if data, err := json.Marshal(props); err != nil {
							slog.Error("Failed to marshal MCP response analytics properties", "error", err)
						} else {
							analyticsMetadata["mcp_response_properties"] = string(data)
						}
					}
				}
			}
		}
	case policy.APIKindAgent:
		// The buffered path: either the chain cannot stream, or the response is a
		// single JSON document (every non-streaming A2A operation, and any error
		// response to a streaming one — a failed SendStreamingMessage answers with a
		// normal buffered error, not an empty stream).
		if ctx.SharedContext.ResolvedOperation != "" {
			populateA2AResponseMetadata(analyticsMetadata, ctx.SharedContext,
				a2aResponseBytes(ctx), ctx.ResponseHeaders, ctx.ResponseStatus, false)
		}
	default:
		slog.Error("Invalid API kind")
	}

	if sendRespBody {
		if ctx != nil && ctx.ResponseBody != nil && len(ctx.ResponseBody.Content) > 0 {
			slog.Debug("Capturing response payload for analytics")
			analyticsMetadata["response_payload"] = string(ctx.ResponseBody.Content)
		}
	}

	if len(analyticsMetadata) > 0 {
		return policy.DownstreamResponseModifications{
			AnalyticsMetadata: analyticsMetadata,
		}
	}
	return nil
}

// OnResponseBodyChunk handles streaming response body chunks.
// Chunks are accumulated in SharedContext.Metadata. On EndOfStream the accumulated
// bytes are parsed and analytics metadata is emitted on the final ResponseChunkAction.
func (a *AnalyticsPolicy) OnResponseBodyChunk(_ context.Context, ctx *policy.ResponseStreamContext, chunk *policy.StreamBody, params map[string]interface{}) policy.StreamingResponseAction {
	slog.Debug("Analytics system policy: OnResponseBodyChunk called")
	if ctx.SharedContext.Metadata == nil {
		ctx.SharedContext.Metadata = make(map[string]interface{})
	}

	if len(chunk.Chunk) > 0 {
		acc, _ := ctx.SharedContext.Metadata[analyticsStreamAccKey].([]byte)
		acc = append(acc, chunk.Chunk...)
		ctx.SharedContext.Metadata[analyticsStreamAccKey] = acc
		// Recorded here and nowhere else: by EndOfStream the moment is gone, and the
		// ALS timepoints the event's latencies come from cannot express it (a stream's
		// "last upstream byte" is the end of the stream). Agent only — every other kind
		// pays one comparison.
		if ctx.SharedContext.APIKind == policy.APIKindAgent {
			markA2AFirstEvent(ctx.SharedContext, acc, ctx.ResponseHeaders)
		}
	}

	if !chunk.EndOfStream {
		return policy.ForwardResponseChunk{}
	}

	// EndOfStream: consume accumulated bytes and emit analytics.
	accumulated, _ := ctx.SharedContext.Metadata[analyticsStreamAccKey].([]byte)
	delete(ctx.SharedContext.Metadata, analyticsStreamAccKey)

	analyticsMetadata := make(map[string]any)

	populateAuthAnalyticsMetadata(analyticsMetadata, ctx.SharedContext.AuthContext)

	apiKind := ctx.SharedContext.APIKind
	switch apiKind {
	case policy.APIKindRestApi:
		// No body analytics for REST API
	case policy.APIKindLlmProvider, policy.APIKindLlmProxy:
		templateHandle, ok := ctx.SharedContext.Metadata["template_handle"].(string)
		if ok && templateHandle != "" {
			template, err := getTemplateByHandle(templateHandle)
			if err != nil {
				slog.Warn("Failed to get template from lazy resource cache (streaming)", "templateHandle", templateHandle, "error", err)
			} else {
				var requestBodyBytes []byte
				if ctx.RequestBody != nil {
					requestBodyBytes = ctx.RequestBody.Content
				}
				// Streaming responses are SSE; the last data event carries usage fields.
				tokenInfo, err := extractLLMProviderAnalyticsInfoFromBytes(
					template, ctx.RequestHeaders, ctx.ResponseHeaders,
					requestBodyBytes, accumulated, ctx.RequestPath,
				)
				if err != nil {
					slog.Warn("Failed to extract LLM token info from streaming response", "error", err)
				} else if tokenInfo != nil {
					populateTokenAnalyticsMetadata(analyticsMetadata, tokenInfo)
				}
			}
		}
	case policy.APIKindMCP:
		if ctx.ResponseHeaders != nil {
			sessionIDs := ctx.ResponseHeaders.Get("mcp-session-id")
			if len(sessionIDs) > 0 {
				analyticsMetadata["mcp_session_id"] = sessionIDs[0]
			}
		}

		if len(accumulated) > 0 {
			mcpPayload := extractMCPPayloadFromAccumulated(accumulated, ctx.ResponseHeaders)
			if mcpPayload != nil {
				props := extractMCPResponseAnalyticsProps(mcpPayload)
				if props != nil {
					if data, err := json.Marshal(props); err != nil {
						slog.Error("Failed to marshal MCP response analytics properties (streaming)", "error", err)
					} else {
						analyticsMetadata["mcp_response_properties"] = string(data)
					}
				}
			}
		}
	case policy.APIKindAgent:
		// The streaming path. accumulated is the whole SSE stream by the time this
		// runs, so a JSON-RPC error delivered as a late event is still seen.
		if ctx.SharedContext.ResolvedOperation != "" {
			populateA2AResponseMetadata(analyticsMetadata, ctx.SharedContext,
				accumulated, ctx.ResponseHeaders, ctx.ResponseStatus, true)
		}
	default:
		slog.Debug("Analytics streaming: unhandled API kind", "kind", apiKind)
	}

	_, sendRespBody := getPayloadFlags(params)
	if sendRespBody && len(accumulated) > 0 {
		analyticsMetadata["response_payload"] = string(accumulated)
	}

	if len(analyticsMetadata) == 0 {
		return policy.ForwardResponseChunk{}
	}
	return policy.ForwardResponseChunk{AnalyticsMetadata: analyticsMetadata}
}

// NeedsMoreResponseData always returns false: each chunk is processed immediately
// and analytics data is accumulated internally in SharedContext.Metadata.
func (a *AnalyticsPolicy) NeedsMoreResponseData(accumulated []byte) bool {
	return false
}

// getTemplateByHandle retrieves a template from the lazy resource cache by its handle
func getTemplateByHandle(templateHandle string) (map[string]interface{}, error) {
	if templateHandle == "" {
		return nil, fmt.Errorf("template handle is empty")
	}

	store := policy.GetLazyResourceStoreInstance()
	if store == nil {
		return nil, fmt.Errorf("lazy resource store is not available")
	}

	resource, err := store.GetResourceByIDAndType(templateHandle, lazyResourceTypeLLMProviderTemplate)
	if err != nil {
		return nil, fmt.Errorf(
			"template with handle '%s' and type '%s' not found in cache: %w",
			templateHandle,
			lazyResourceTypeLLMProviderTemplate,
			err,
		)
	}

	if resource.Resource == nil {
		return nil, fmt.Errorf("template resource data is nil for handle '%s'", templateHandle)
	}

	return resource.Resource, nil
}

// extractLLMProviderAnalyticsInfo extracts LLM analytics from a buffered ResponseContext.
// For SSE content (buffered from a non-streaming path) the last data event is used.
func extractLLMProviderAnalyticsInfo(template map[string]interface{}, ctx *policy.ResponseContext) (*LLMProviderAnalyticsInfo, error) {
	var responseBodyBytes []byte
	if ctx.ResponseBody != nil {
		responseBodyBytes = ctx.ResponseBody.Content
	}

	var requestBodyBytes []byte
	if ctx.RequestBody != nil {
		requestBodyBytes = ctx.RequestBody.Content
	}

	return extractLLMProviderAnalyticsInfoFromBytes(
		template, ctx.RequestHeaders, ctx.ResponseHeaders,
		requestBodyBytes, responseBodyBytes, ctx.RequestPath,
	)
}

// extractLLMProviderAnalyticsInfoFromBytes extracts LLM analytics from raw byte slices.
// responseBodyBytes may be a full JSON body or accumulated SSE chunks; SSE is detected
// automatically. For SSE, all data events are merged (later events win on top-level key
// conflicts) so that providers like Anthropic — which send the model name in the first
// event (message_start) and final token counts in a middle event (message_delta) — are
// handled correctly alongside providers that put everything in the last event.
func extractLLMProviderAnalyticsInfoFromBytes(
	template map[string]interface{},
	requestHeaders, responseHeaders *policy.Headers,
	requestBodyBytes, responseBodyBytes []byte,
	requestPath string,
) (*LLMProviderAnalyticsInfo, error) {
	var responseJSON map[string]interface{}
	if len(responseBodyBytes) > 0 {
		// SSE responses: merge all data events so fields spread across events are visible.
		if merged, err := parseSSEMergedDataEvents(responseBodyBytes); err == nil {
			responseJSON = merged
		} else {
			// Plain JSON response.
			_ = json.Unmarshal(responseBodyBytes, &responseJSON)
		}
	}

	var requestJSON map[string]interface{}
	if len(requestBodyBytes) > 0 {
		_ = json.Unmarshal(requestBodyBytes, &requestJSON)
	}

	return extractLLMAnalyticsFromJSON(template, requestHeaders, responseHeaders, requestJSON, responseJSON, requestPath)
}

// extractLLMAnalyticsFromJSON is the core extraction logic operating on pre-parsed JSON maps.
func extractLLMAnalyticsFromJSON(
	template map[string]interface{},
	requestHeaders, responseHeaders *policy.Headers,
	requestJSON, responseJSON map[string]interface{},
	requestPath string,
) (*LLMProviderAnalyticsInfo, error) {
	if template == nil {
		return nil, fmt.Errorf("template is nil")
	}

	spec, ok := template["spec"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("template spec is not a map")
	}

	info := &LLMProviderAnalyticsInfo{}

	extract := func(field string, fromRequest bool) (interface{}, error) {
		fieldCfg, ok := spec[field].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("field %s not found", field)
		}

		location, _ := fieldCfg["location"].(string)
		identifier, _ := fieldCfg["identifier"].(string)
		if identifier == "" {
			return nil, fmt.Errorf("identifier missing for %s", field)
		}

		switch strings.ToLower(location) {
		case "payload":
			var src map[string]interface{}
			if fromRequest {
				src = requestJSON
			} else {
				src = responseJSON
			}
			if src == nil {
				return nil, fmt.Errorf("payload not available for %s", field)
			}
			return utils.ExtractValueFromJsonpath(src, identifier)
		case "header":
			if fromRequest {
				if requestHeaders == nil {
					return nil, fmt.Errorf("request headers missing")
				}
				if v := requestHeaders.Get(identifier); len(v) > 0 {
					return v[0], nil
				}
			} else {
				if responseHeaders == nil {
					return nil, fmt.Errorf("response headers missing")
				}
				if v := responseHeaders.Get(identifier); len(v) > 0 {
					return v[0], nil
				}
			}
			return nil, fmt.Errorf("header %s not found", identifier)
		case "pathparam":
			// Model ids for providers like AWS Bedrock and Gemini live in the
			// request URL path (e.g. /model/{modelId}/converse), not the body or
			// a header. The identifier is a regex whose first capture group (when
			// present) is the value; otherwise the whole match is used. The path
			// is available for both buffered and streaming responses, so this
			// meters the model in either case.
			return extractPathParam(requestPath, identifier)
		default:
			return nil, fmt.Errorf("unsupported location %s", location)
		}
	}

	if v, err := extract("promptTokens", false); err == nil {
		if i, err := convertToInt64(v); err == nil {
			info.PromptTokens = &i
		}
	}
	if v, err := extract("completionTokens", false); err == nil {
		if i, err := convertToInt64(v); err == nil {
			info.CompletionTokens = &i
		}
	}
	if v, err := extract("totalTokens", false); err == nil {
		if i, err := convertToInt64(v); err == nil {
			info.TotalTokens = &i
		}
	}
	if v, err := extract("remainingTokens", false); err == nil {
		if i, err := convertToInt64(v); err == nil {
			info.RemainingTokens = &i
		}
	}
	if v, err := extract("requestModel", true); err == nil {
		if s, ok := v.(string); ok {
			info.RequestModel = &s
		}
	}
	if v, err := extract("responseModel", false); err == nil {
		if s, ok := v.(string); ok {
			info.ResponseModel = &s
		}
	}

	if md, ok := template["metadata"].(map[string]interface{}); ok {
		if nameVal, ok := md["name"].(string); ok && strings.TrimSpace(nameVal) != "" {
			info.ProviderName = &nameVal
		}
	}

	if displayName, ok := spec["displayName"].(string); ok && strings.TrimSpace(displayName) != "" {
		if info.ProviderName == nil {
			info.ProviderName = &displayName
		}
		info.ProviderDisplayName = &displayName
	}

	return info, nil
}

// pathParamRegexCache memoises compiled pathParam identifiers so the response
// data path does not recompile a provider template's regex on every request.
var pathParamRegexCache sync.Map // map[string]*regexp.Regexp

// extractPathParam applies a pathParam identifier (a regex) to the request URL
// path and returns the first capture group when the pattern defines one,
// otherwise the whole match. AWS Bedrock and Gemini carry the model id in the
// path (e.g. /model/{modelId}/converse), which is why request/response model is
// declared with `location: pathParam`. The path is present for both buffered
// and streaming responses, so the model meters in either case. Go's regexp is
// RE2 — identifiers must use a capture group, not lookaround.
func extractPathParam(requestPath, pattern string) (string, error) {
	if requestPath == "" {
		return "", fmt.Errorf("request path not available")
	}
	re, err := compilePathParamRegex(pattern)
	if err != nil {
		return "", err
	}
	match := re.FindStringSubmatch(requestPath)
	if match == nil {
		return "", fmt.Errorf("pathParam identifier did not match request path")
	}
	if len(match) > 1 {
		return match[1], nil
	}
	return match[0], nil
}

// compilePathParamRegex compiles (and caches) a pathParam identifier regex.
func compilePathParamRegex(pattern string) (*regexp.Regexp, error) {
	if cached, ok := pathParamRegexCache.Load(pattern); ok {
		return cached.(*regexp.Regexp), nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pathParam identifier %q: %w", pattern, err)
	}
	pathParamRegexCache.Store(pattern, re)
	return re, nil
}

// populateTokenAnalyticsMetadata copies LLM token fields into an analytics metadata map.
func populateTokenAnalyticsMetadata(analyticsMetadata map[string]any, tokenInfo *LLMProviderAnalyticsInfo) {
	if tokenInfo.PromptTokens != nil {
		analyticsMetadata[PromptTokenCountMetadataKey] = strconv.FormatInt(*tokenInfo.PromptTokens, 10)
	}
	if tokenInfo.CompletionTokens != nil {
		analyticsMetadata[CompletionTokenCountMetadataKey] = strconv.FormatInt(*tokenInfo.CompletionTokens, 10)
	}
	if tokenInfo.TotalTokens != nil {
		analyticsMetadata[TotalTokenCountMetadataKey] = strconv.FormatInt(*tokenInfo.TotalTokens, 10)
	}
	if tokenInfo.ResponseModel != nil {
		analyticsMetadata[ModelIDMetadataKey] = *tokenInfo.ResponseModel
	} else if tokenInfo.RequestModel != nil {
		analyticsMetadata[ModelIDMetadataKey] = *tokenInfo.RequestModel
	}
	if tokenInfo.ProviderName != nil {
		analyticsMetadata[AIProviderNameMetadataKey] = *tokenInfo.ProviderName
	}
	if tokenInfo.ProviderDisplayName != nil {
		analyticsMetadata[AIProviderDisplayNameMetadataKey] = *tokenInfo.ProviderDisplayName
	}
}

// ─── A2A (Agent) analytics ───────────────────────────────────────────────────

// populateA2ARequestMetadata emits the request-side A2A dimensions for one Agent
// request, and starts the clock the streaming timings are measured against.
//
// Called from both request phases with whatever that phase can see: the header phase
// passes the path and no body, the body phase passes both. The block it writes is
// therefore the fullest one available at the time, and the body phase's — a superset —
// is the one that survives, because the two writes share a metadata key.
//
// A request with no resolved operation is not an invocation of the agent — it is a
// fetch of the public Agent Card or a CORS preflight, both of which live on the
// Agent's own routes. Those get nothing here: the policy-engine classifies them from
// the operation's absence, and emitting invocation-shaped dimensions for them would
// let a downstream rollup count a client's card polling as agent traffic.
func populateA2ARequestMetadata(
	analyticsMetadata map[string]any,
	shared *policy.SharedContext,
	path string,
	body []byte,
) {
	if shared == nil || shared.ResolvedOperation == "" {
		return
	}

	// Read straight out of the resolver's output. The resolver already parsed the
	// request body once to find the operation; re-parsing it here is the duplication
	// SharedContext.ResolutionAttributes exists to remove.
	//
	// Get is right for all five rather than Lookup: an attribute the request did not
	// carry reads as "", and every field below is `omitempty`, so an absent
	// identifier is omitted from the event instead of being reported as empty.
	attrs := shared.ResolutionAttributes
	props := A2ARequestAnalyticsProperties{
		Transport:       attrs.Get(a2aAttrTransport),
		ProtocolVersion: attrs.Get(a2aAttrProtocolVersion),
		MessageID:       attrs.Get(a2aAttrMessageID),
		ContextID:       attrs.Get(a2aAttrContextID),
		TaskID:          attrs.Get(a2aAttrTaskID),
	}
	applyA2ARequestSummary(&props, props.Transport, path, body)

	// The canonical operation itself is not repeated here: the kernel stamps it
	// directly, so it is present whether or not this policy is in the chain.
	if data, err := json.Marshal(props); err != nil {
		slog.Error("Failed to marshal A2A request analytics properties", "error", err)
	} else {
		analyticsMetadata[A2ARequestPropertiesKey] = string(data)
	}

	// Set once, at whichever phase got here first. Time-to-first-event measures what a
	// caller waiting on an agent experiences, so it has to start at the earliest point
	// in the request this policy runs; re-stamping it at the body phase would silently
	// exclude every header policy — authentication included — from the measurement.
	if shared.Metadata != nil {
		if _, started := shared.Metadata[a2aRequestStartKey]; !started {
			shared.Metadata[a2aRequestStartKey] = time.Now()
		}
	}
}

// applyA2ARequestSummary fills in the three summaries of what the caller actually
// asked for: how many parts the message carried, whether it wanted an immediate
// return, and how much task history it wanted back.
//
// The extraction is driven by the shape of the request rather than by its operation
// name. That keeps the operation table — which lives in common/agentproto, on the
// other side of a module boundary this policy cannot cross — from being transcribed
// here to be kept in sync by hand. It is also exactly as precise: in A2A 1.0 only the
// two message-sending operations carry a `message`, and only they, GetTask and
// ListTasks carry a history length, each in the one place its own request shape
// defines.
//
// Anything the body does not yield is left absent rather than defaulted. The single
// exception is returnImmediately, whose absence the protocol itself defines as false,
// so a send request that omits `configuration` still reports the effective value it
// will be treated as having.
func applyA2ARequestSummary(props *A2ARequestAnalyticsProperties, transport, path string, body []byte) {
	// The HTTP+JSON binding puts GetTask's and ListTasks' history length in the query
	// string; both are GETs, so this is the only place it exists for them. Restricted
	// to that transport because a JSON-RPC request carries its arguments in the body
	// and any query string on that endpoint is not the protocol's.
	if transport == a2aTransportHTTPJSON {
		if length, ok := a2aQueryInt(path, "historyLength"); ok {
			props.HistoryLength = &length
		}
	}

	root := a2aParseObject(body)
	if root == nil {
		return
	}
	// JSON-RPC nests the operation's arguments under params; HTTP+JSON sends the
	// request message itself. A params that is not an object (a positional array, or
	// a malformed envelope) yields nothing rather than being coerced.
	args := root
	if params, isObject := root["params"].(map[string]interface{}); isObject {
		args = params
	}

	if message, isObject := args["message"].(map[string]interface{}); isObject {
		if parts, isArray := message["parts"].([]interface{}); isArray {
			count := len(parts)
			props.InputPartCount = &count
		}
		returnImmediately := false
		if configuration, isObject := args["configuration"].(map[string]interface{}); isObject {
			if value, isBool := configuration["returnImmediately"].(bool); isBool {
				returnImmediately = value
			}
			if length, ok := a2aInt(configuration["historyLength"]); ok {
				props.HistoryLength = &length
			}
		}
		props.ReturnImmediately = &returnImmediately
	}

	// GetTask and ListTasks put it directly in their arguments. Read after the send
	// case so a body-borne value always wins over one seen in the query string; the
	// two never coexist on a conformant request.
	if length, ok := a2aInt(args["historyLength"]); ok {
		props.HistoryLength = &length
	}
}

// a2aParseObject decodes a request or response body that is expected to be a single
// JSON object, and returns nil for anything else.
//
// Nil is not an error here. The agent is the component that validates A2A payloads,
// and a body this policy cannot read costs only a summary dimension — reporting a
// guess would be worse than reporting nothing.
func a2aParseObject(body []byte) map[string]interface{} {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(trimmed, &parsed); err != nil {
		slog.Debug("Failed to unmarshal A2A request body for analytics", "error", err)
		return nil
	}
	return parsed
}

// a2aInt reads a JSON number that is meant to be a protocol int32 field.
//
// A fractional or out-of-range value is refused rather than rounded or wrapped: the
// field is a count, and a value the protocol cannot hold is not a count this gateway
// should report as one.
func a2aInt(value interface{}) (int, bool) {
	number, isNumber := value.(float64)
	if !isNumber || number != math.Trunc(number) ||
		number < math.MinInt32 || number > math.MaxInt32 {
		return 0, false
	}
	return int(number), true
}

// a2aQueryInt reads one integer query parameter off a request path.
//
// It scans the raw pairs rather than calling url.ParseQuery for the same reason the
// resolver's version check does: ParseQuery returns the pairs it could decode
// alongside an error for the ones it could not, so an unrelated malformed parameter
// elsewhere in the query string would otherwise discard this one. A parameter that is
// not this one is never decoded and never interpreted — it stays the agent's to
// reject. Repeats resolve first-wins, matching what a standard parser would hand the
// agent.
func a2aQueryInt(path, name string) (int, bool) {
	_, query, hasQuery := strings.Cut(path, "?")
	if !hasQuery {
		return 0, false
	}
	for rest := query; rest != ""; {
		var pair string
		pair, rest, _ = strings.Cut(rest, "&")
		if pair == "" {
			continue
		}
		rawName, rawValue, _ := strings.Cut(pair, "=")
		decodedName, err := url.QueryUnescape(rawName)
		if err != nil || decodedName != name {
			continue
		}
		decodedValue, err := url.QueryUnescape(rawValue)
		if err != nil {
			return 0, false
		}
		parsed, err := strconv.ParseInt(decodedValue, 10, 32)
		if err != nil {
			return 0, false
		}
		return int(parsed), true
	}
	return 0, false
}

// populateA2AResponseMetadata emits the response-side A2A dimensions: whether the
// invocation actually failed, and — for a streamed response — when its first event
// reached the client and how long the stream lasted.
//
// streamed distinguishes the two response paths rather than being inferred from the
// content type: a chain that cannot stream (any policy in it needing a buffered
// response body) delivers an SSE response through the buffered path, and reporting it
// as streamed would put a stream duration on a response the gateway held whole.
func populateA2AResponseMetadata(
	analyticsMetadata map[string]any,
	shared *policy.SharedContext,
	responseBody []byte,
	responseHeaders *policy.Headers,
	responseStatus int,
	streamed bool,
) {
	props := A2AResponseAnalyticsProperties{IsStreaming: &streamed}

	observation := observeA2AResponse(responseBody, responseHeaders)

	// isError is always emitted for a response body this policy could read, so a
	// consumer can tell a determined success from an undetermined outcome. A body it
	// could not read leaves it unset rather than false — claiming success for a
	// response nobody parsed is the silently-wrong-dashboard case.
	if payload := observation.outcomeEnvelope; payload != nil {
		isError := false
		if errVal, hasError := payload["error"]; hasError && errVal != nil {
			isError = true
		}
		props.IsError = &isError
		if code, err := extractIntFromJsonpath(payload, JsonRpcErrorCodeJsonPath); err == nil {
			props.ErrorCode = &code
		}
	}

	props.PayloadType = a2aPayloadTypeFor(observation, responseBody, responseStatus)
	props.ResponseTaskID = observation.taskID
	props.ResponseContextID = observation.contextID
	props.TaskState = observation.taskState

	if streamed && shared != nil {
		populateA2AStreamTimings(&props, shared.Metadata)
	}

	if data, err := json.Marshal(props); err != nil {
		slog.Error("Failed to marshal A2A response analytics properties", "error", err)
	} else {
		analyticsMetadata[A2AResponsePropertiesKey] = string(data)
	}
}

// markA2AFirstEvent records when the first event of a streamed A2A response first
// became deliverable to the client.
//
// "Event", not "byte". An SSE stream may open with keep-alive comments (": ping"), a
// retry directive, or the front half of an event split across chunks — none of which a
// client dispatches. An agent that thinks for thirty seconds while pinging every five
// would otherwise report a time-to-first-event near zero, so the metric would look best
// exactly when the agent is slowest, which is worse than not having it. The mark is
// therefore taken when the forwarded bytes first complete an event block carrying a
// data field: the moment a client's SSE parser fires.
//
// Framing is decided from the response's declared content type, never by sniffing the
// bytes: whether a stream has event framing is a property of the response, and a first
// chunk containing only a comment is precisely what would sniff wrong. A response that
// declares no event framing has no events to distinguish, so its first forwarded chunk
// is its first delivery.
func markA2AFirstEvent(shared *policy.SharedContext, forwarded []byte, responseHeaders *policy.Headers) {
	if _, seen := shared.Metadata[a2aFirstEventKey]; seen {
		return
	}

	if !declaresSSEFraming(responseHeaders) {
		shared.Metadata[a2aFirstEventKey] = time.Now()
		return
	}

	scanned, _ := shared.Metadata[a2aStreamScanKey].(int)
	resume, found := firstSSEDataEventEnd(forwarded, scanned)
	if !found {
		// Remember how far the scan got, so a stream that opens with a long run of
		// heartbeats is not rescanned from the start on every chunk.
		shared.Metadata[a2aStreamScanKey] = resume
		return
	}
	shared.Metadata[a2aFirstEventKey] = time.Now()
	delete(shared.Metadata, a2aStreamScanKey)
}

// declaresSSEFraming reports whether the response declared itself Server-Sent Events.
//
// Deliberately header-only, unlike isSSEContent, which also sniffs. Sniffing is right
// when the whole body is in hand and the question is how to parse it; it is wrong here,
// where the body is a prefix that may not yet contain anything recognisable.
func declaresSSEFraming(responseHeaders *policy.Headers) bool {
	if responseHeaders == nil {
		return false
	}
	contentTypes := responseHeaders.Get("content-type")
	return len(contentTypes) > 0 &&
		strings.Contains(strings.ToLower(contentTypes[0]), "text/event-stream")
}

// firstSSEDataEventEnd scans body from scanFrom for the end of the first complete SSE
// event that carries a data field.
//
// It returns the index just past that event and true; or, when none has arrived yet,
// the index a later scan may resume from and false. Resuming matters because this runs
// per chunk: without it a stream that opens with many heartbeats would rescan its whole
// prefix every time.
//
// A block with no data field — a ": keep-alive" comment, a bare "retry:" directive — is
// a real SSE block but not one a client dispatches, so it is skipped rather than
// counted. A block with no terminator yet is incomplete: the client has not seen an
// event, so neither have we.
func firstSSEDataEventEnd(body []byte, scanFrom int) (int, bool) {
	if scanFrom < 0 || scanFrom > len(body) {
		scanFrom = 0
	}
	for start := scanFrom; ; {
		end, sepLen := sseBlockEnd(body[start:])
		if end < 0 {
			return start, false
		}
		block := body[start : start+end]
		start += end + sepLen
		if sseBlockHasData(block) {
			return start, true
		}
	}
}

// sseBlockEnd returns the offset of the blank line ending the first event block in b
// and the length of that terminator, or -1 when the block is not terminated yet. Both
// LF and CRLF line endings occur in practice.
func sseBlockEnd(b []byte) (int, int) {
	lf := bytes.Index(b, []byte("\n\n"))
	crlf := bytes.Index(b, []byte("\r\n\r\n"))
	switch {
	case crlf >= 0 && (lf < 0 || crlf < lf):
		return crlf, 4
	case lf >= 0:
		return lf, 2
	default:
		return -1, 0
	}
}

// sseDataField is the SSE field name both scanners on the A2A path key off — the one
// that ends the timing scan (sseBlockHasData) and the one the observation reads
// (sseDataValue). Spelled once because the two must agree: a line one of them counts
// as an event and the other skips is a stream that reports a first-event time and no
// observation, which surfaces as an outcome of UNKNOWN with every field absent.
//
// The separator space is **not** part of the field. Per the SSE field-parsing rule
// (WHATWG HTML, server-sent events), the value is everything after the colon with a
// single leading space removed if present — so "data:{...}" and "data: {...}" are the
// same field, and an agent that omits the optional space must be read the same way as
// one that sends it.
const sseDataField = "data:"

// sseDataFieldBytes is sseDataField for the byte-oriented scanner, which runs over
// every forwarded chunk and so avoids a per-line string conversion.
var sseDataFieldBytes = []byte(sseDataField)

// sseDataValue reports whether line is an SSE data field and returns its value, with
// the optional single separator space removed. See sseDataField for the rule.
func sseDataValue(line string) (string, bool) {
	value, isData := strings.CutPrefix(line, sseDataField)
	if !isData {
		return "", false
	}
	// Exactly one space, never all leading whitespace: a second space is content the
	// agent sent, and this must not silently rewrite a payload.
	return strings.TrimPrefix(value, " "), true
}

// sseBlockHasData reports whether an event block carries a data field, which is what
// separates an event a client dispatches from a comment or a directive.
func sseBlockHasData(block []byte) bool {
	for line := range bytes.SplitSeq(block, []byte("\n")) {
		if bytes.HasPrefix(bytes.TrimSuffix(line, []byte("\r")), sseDataFieldBytes) {
			return true
		}
	}
	return false
}

// populateA2AStreamTimings fills in the two timings a streamed A2A response has and a
// buffered one does not, and clears the marks it read.
//
// Time-to-first-event is measured from the request-header phase, not from the response
// headers: what a caller waiting on an agent experiences is the delay from asking to
// seeing the first event, and the agent's own think time before it emits anything is
// the largest part of that. Stream duration is measured from the first event so the
// two do not double-count the same wait.
//
// The marks are deleted after use for the same reason the chunk accumulator is: the
// shared context outlives this callback.
func populateA2AStreamTimings(props *A2AResponseAnalyticsProperties, metadata map[string]interface{}) {
	if metadata == nil {
		return
	}
	firstEvent, hasFirstEvent := metadata[a2aFirstEventKey].(time.Time)
	start, hasStart := metadata[a2aRequestStartKey].(time.Time)
	// Cleared unconditionally, including on the paths below that report nothing: a
	// half-cleared set of marks is how a later stream on the same context would inherit
	// a stale one.
	delete(metadata, a2aFirstEventKey)
	delete(metadata, a2aRequestStartKey)
	delete(metadata, a2aStreamScanKey)

	if !hasFirstEvent {
		// A stream that ended without ever completing an event — no events at all, or
		// only heartbeats. Neither timing is meaningful, and inventing a zero for
		// either would read as an instant response rather than as an empty one.
		return
	}

	if hasStart {
		ttfe := firstEvent.Sub(start).Milliseconds()
		props.TimeToFirstEventMs = &ttfe
	}

	duration := time.Since(firstEvent).Milliseconds()
	props.StreamDurationMs = &duration
}

// a2aResponseObservation is everything one A2A response yielded: the envelope its
// outcome is read from, what kind of payload it carried, and the identifiers and task
// state the agent reported back.
//
// It is one struct rather than several scans because a streaming response is scanned
// once: the identifiers and the state are cumulative over the whole stream while the
// outcome is decided by a single event, and walking the events twice to answer the two
// questions separately would be the duplication the resolver seam exists to avoid.
type a2aResponseObservation struct {
	// outcomeEnvelope is the one event or document the outcome is read from: the error
	// if the response carried one, otherwise the last event carrying a result. Nil
	// when nothing readable said either — which is what leaves isError absent rather
	// than false.
	outcomeEnvelope map[string]interface{}

	// payloadType is the kind of the most recently observed payload. Empty when
	// nothing was observed at all; the caller resolves that against the status.
	payloadType string

	// The latest non-empty value each of these had in any observed payload. Latest
	// rather than first because a stream reports a task's progress: the final status
	// update is the state the invocation actually reached, and an id may appear only
	// once, in whichever event first carried it.
	taskID    string
	contextID string
	taskState string
}

// observeA2AResponse scans an A2A response — a single JSON document or an SSE stream —
// for everything the response side reports.
//
// On a stream it prefers an error over a result and stops there, scanning until it
// finds one. That is the opposite of the MCP helper's first-match rule, and
// deliberately so: an A2A streaming response is a sequence of task updates that can
// end in a JSON-RPC error after any number of successful events, so stopping at the
// first event carrying a result would report every late failure as a success.
//
// Nothing here delays or buffers the stream on its own account: the chunks were
// forwarded as they arrived and this reads the copy the policy had already accumulated
// for the outcome, at end of stream.
func observeA2AResponse(body []byte, responseHeaders *policy.Headers) a2aResponseObservation {
	var observation a2aResponseObservation
	if len(bytes.TrimSpace(body)) == 0 {
		return observation
	}

	if !isSSEContent(responseHeaders, body) {
		// A body that is not a JSON object is an HTTP+JSON error document, a sterile
		// gateway response, or something non-JSON. None of them is a payload; the
		// status carries that outcome.
		if payload := a2aParseObject(body); payload != nil {
			observation.observe(payload)
		}
		return observation
	}

	for line := range strings.SplitSeq(string(body), "\n") {
		data, isData := sseDataValue(strings.TrimSpace(line))
		if !isData {
			continue
		}
		if data == "" || data == "[DONE]" {
			continue
		}
		var event map[string]interface{}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		if terminal := observation.observe(event); terminal {
			break
		}
	}
	return observation
}

// observe folds one response document into the observation, reporting whether it
// terminates the scan.
//
// The two bindings deliver the same payloads differently: JSON-RPC wraps each in an
// envelope with a result field, HTTP+JSON sends the document itself. Only the wrapped
// form can state an outcome, which is why an unwrapped payload contributes its type
// and identifiers without making outcomeEnvelope non-nil — an HTTP+JSON response's
// outcome is its status, and claiming otherwise here would put an isError on a
// response that never carried one.
func (o *a2aResponseObservation) observe(document map[string]interface{}) bool {
	if errVal, hasError := document["error"]; hasError && errVal != nil {
		o.outcomeEnvelope = document
		o.payloadType = a2aPayloadError
		return true // an error anywhere in the stream decides the outcome
	}

	payload, wrapped := document["result"]
	if wrapped {
		o.outcomeEnvelope = document
	} else {
		payload = document
	}
	o.observePayload(payload)
	return false
}

// observePayload classifies one A2A result payload and records what it reported.
//
// The A2A 1.0 response shapes are distinguishable by their own fields, so this reads
// the document rather than the operation. Both spellings of each payload occur: the
// send and stream operations wrap their result in the union field that names it
// (`task`, `message`, `statusUpdate`, `artifactUpdate`), while the operations with a
// fixed result — GetTask, CancelTask, the push-notification-config family — send the
// bare document. The union is therefore matched first, so a Task that happens to be
// wrapped is not mistaken for a bare one.
func (o *a2aResponseObservation) observePayload(payload interface{}) {
	document, isObject := payload.(map[string]interface{})
	if !isObject {
		// A result explicitly present and null: the JSON-RPC form of a delete, whose
		// protocol response is google.protobuf.Empty.
		if payload == nil {
			o.payloadType = a2aPayloadEmpty
		}
		return
	}

	switch {
	case a2aIsObject(document["task"]):
		o.payloadType = a2aPayloadTask
		o.observeTask(document["task"].(map[string]interface{}))
	case a2aIsObject(document["message"]):
		o.payloadType = a2aPayloadMessage
		o.observeIdentifiers(document["message"].(map[string]interface{}))
	case a2aIsObject(document["statusUpdate"]):
		o.payloadType = a2aPayloadStatusUpdate
		o.observeStatusUpdate(document["statusUpdate"].(map[string]interface{}))
	case a2aIsObject(document["artifactUpdate"]):
		o.payloadType = a2aPayloadArtifactUpdate
		o.observeIdentifiers(document["artifactUpdate"].(map[string]interface{}))
	case a2aIsArray(document["tasks"]):
		// A list reports no identifiers: it describes many tasks, and picking one of
		// them to correlate on would be arbitrary.
		o.payloadType = a2aPayloadTaskList
	case a2aIsArray(document["configs"]):
		o.payloadType = a2aPayloadPushConfigList
	case a2aIsObject(document["status"]) && a2aString(document["id"]) != "":
		o.payloadType = a2aPayloadTask
		o.observeTask(document)
	case a2aString(document["messageId"]) != "":
		o.payloadType = a2aPayloadMessage
		o.observeIdentifiers(document)
	case a2aString(document["url"]) != "" && a2aString(document["taskId"]) != "":
		o.payloadType = a2aPayloadPushConfig
		o.observeIdentifiers(document)
	case a2aString(document["protocolVersion"]) != "" && document["skills"] != nil:
		o.payloadType = a2aPayloadAgentCard
	case len(document) == 0:
		o.payloadType = a2aPayloadEmpty
	default:
		o.payloadType = a2aPayloadUnknown
	}
}

// observeTask records what a Task reported. A Task names itself with `id`, unlike
// every event, which names the task it belongs to with `taskId`.
func (o *a2aResponseObservation) observeTask(task map[string]interface{}) {
	o.recordTaskID(a2aString(task["id"]))
	o.recordContextID(a2aString(task["contextId"]))
	if status, isObject := task["status"].(map[string]interface{}); isObject {
		o.recordTaskState(a2aString(status["state"]))
	}
}

// observeStatusUpdate records what a TaskStatusUpdateEvent reported — the only
// streamed event that carries a state.
func (o *a2aResponseObservation) observeStatusUpdate(update map[string]interface{}) {
	o.observeIdentifiers(update)
	if status, isObject := update["status"].(map[string]interface{}); isObject {
		o.recordTaskState(a2aString(status["state"]))
	}
}

// observeIdentifiers records the task and context a document belongs to, for the
// shapes that reference a task rather than being one.
func (o *a2aResponseObservation) observeIdentifiers(document map[string]interface{}) {
	o.recordTaskID(a2aString(document["taskId"]))
	o.recordContextID(a2aString(document["contextId"]))
}

// recordTaskID and recordContextID keep the latest non-empty value seen. Over-long
// values are dropped rather than truncated, per maxA2AObservedValueBytes.
func (o *a2aResponseObservation) recordTaskID(value string) {
	if value != "" && len(value) <= maxA2AObservedValueBytes {
		o.taskID = value
	}
}

func (o *a2aResponseObservation) recordContextID(value string) {
	if value != "" && len(value) <= maxA2AObservedValueBytes {
		o.contextID = value
	}
}

// recordTaskState keeps the latest recognised state. An unrecognised one is dropped,
// not reported: this is a dimension a dashboard groups by, so a value outside the
// protocol's own set would widen it on an agent's say-so.
func (o *a2aResponseObservation) recordTaskState(value string) {
	if state := normalizeA2ATaskState(value); state != "" {
		o.taskState = state
	}
}

// normalizeA2ATaskState maps a reported state onto the canonical protocol-enum
// spelling, or returns "" if it is not an A2A 1.0 state.
//
// Both spellings are accepted because both are in circulation: the 1.0 protobuf
// enumeration serializes as TASK_STATE_COMPLETED, while the JSON forms that preceded
// it — and the hyphenated variants of the two-word states — spell the same value
// "completed" and "input-required". Folding them onto one spelling is what makes the
// dimension aggregate; leaving them apart would split one state across three labels.
func normalizeA2ATaskState(value string) string {
	if value == "" || len(value) > 64 {
		return ""
	}
	candidate := strings.ToUpper(strings.ReplaceAll(value, "-", "_"))
	if !strings.HasPrefix(candidate, "TASK_STATE_") {
		candidate = "TASK_STATE_" + candidate
	}
	if _, known := a2aTaskStates[candidate]; !known {
		return ""
	}
	return candidate
}

// a2aPayloadTypeFor settles what kind of payload the response carried.
//
// An error status decides on its own, and has to: the HTTP+JSON binding is REST-shaped
// and answers a failure with a real error status and an error document that is not an
// A2A payload at all, so classifying that document by its shape would report it as
// `unknown` rather than as the error it is. Below that, what was observed wins, and a
// successful response with no body at all is empty rather than unknown — that is the
// 204 from DeleteTaskPushNotificationConfig, whose emptiness is its result.
func a2aPayloadTypeFor(observation a2aResponseObservation, body []byte, status int) string {
	switch {
	case status >= 400:
		return a2aPayloadError
	case observation.payloadType != "":
		return observation.payloadType
	case len(bytes.TrimSpace(body)) == 0:
		return a2aPayloadEmpty
	default:
		return a2aPayloadUnknown
	}
}

// a2aIsObject, a2aIsArray and a2aString read one field of a decoded JSON document
// without asserting anything about the rest of it. A field of the wrong type is
// treated as absent: the agent owns the document, and a malformed one costs a
// dimension rather than producing a wrong one.
func a2aIsObject(value interface{}) bool {
	_, isObject := value.(map[string]interface{})
	return isObject
}

func a2aIsArray(value interface{}) bool {
	_, isArray := value.([]interface{})
	return isArray
}

func a2aString(value interface{}) string {
	text, isString := value.(string)
	if !isString {
		return ""
	}
	return text
}

// a2aResponseBytes returns the response body of a buffered A2A response, or nil when
// there is none — a 204, a 304 on the card route, or a response the chain never read.
func a2aResponseBytes(ctx *policy.ResponseContext) []byte {
	if ctx == nil || ctx.ResponseBody == nil {
		return nil
	}
	return ctx.ResponseBody.Content
}

// extractMCPPayloadFromAccumulated finds the first MCP JSON-RPC result or error event
// from accumulated SSE bytes, or parses the bytes directly as JSON.
func extractMCPPayloadFromAccumulated(accumulated []byte, responseHeaders *policy.Headers) map[string]interface{} {
	if isSSEContent(responseHeaders, accumulated) {
		lines := strings.Split(string(accumulated), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				continue
			}
			var obj map[string]interface{}
			if err := json.Unmarshal([]byte(data), &obj); err != nil {
				continue
			}
			// Prefer events that carry a result or error field.
			if _, hasResult := obj["result"]; hasResult {
				return obj
			}
			if _, hasError := obj["error"]; hasError {
				return obj
			}
		}
		return nil
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(accumulated, &obj); err != nil {
		return nil
	}
	return obj
}

// extractMCPResponseAnalyticsProps builds MCP analytics properties from a parsed JSON-RPC payload.
// Returns nil when there is no relevant data to report.
func extractMCPResponseAnalyticsProps(payload map[string]interface{}) *McpResponseAnalyticsProperties {
	props := McpResponseAnalyticsProperties{}

	serverInfo := McpServerInfo{
		ProtocolVersion: extractStringFromJsonpath(payload, ServerProtocolVersionJsonPath),
		Name:            extractStringFromJsonpath(payload, ServerInfoNameJsonPath),
		Version:         extractStringFromJsonpath(payload, ServerInfoVersionJsonPath),
	}

	if serverInfo.Name != "" || serverInfo.Version != "" {
		props.ServerInfo = &serverInfo
	}

	// isError denotes whether the JSON-RPC response represents an error. It is true when a
	// protocol-level error object is present ($.error) or when a tool result explicitly sets
	// result.isError=true; false otherwise. Always emitted so consumers get a definitive flag.
	isError := false
	if errVal, hasError := payload["error"]; hasError && errVal != nil {
		isError = true
	} else if resultIsError, err := extractBoolFromJsonpath(payload, IsErrorJsonPath); err == nil {
		isError = resultIsError
	}
	props.IsError = &isError

	errorCode, err := extractIntFromJsonpath(payload, JsonRpcErrorCodeJsonPath)
	if err == nil {
		props.ErrorCode = &errorCode
	}

	if props.IsError != nil || props.ErrorCode != nil || props.ServerInfo != nil {
		return &props
	}
	return nil
}

// isSSEContent returns true when content is Server-Sent Events format, detected via
// Content-Type header or content structure.
func isSSEContent(headers *policy.Headers, content []byte) bool {
	if headers != nil {
		contentTypes := headers.Get("content-type")
		if len(contentTypes) > 0 && strings.Contains(strings.ToLower(contentTypes[0]), "text/event-stream") {
			return true
		}
	}
	s := string(content)
	return strings.HasPrefix(s, "event:") || strings.Contains(s, "\ndata:")
}

// parseSSEFirstDataEvent returns the JSON bytes from the first "data:" line in SSE content.
// Used for MCP responses where the relevant event is typically the first one.
func parseSSEFirstDataEvent(sseContent []byte) ([]byte, error) {
	lines := strings.Split(string(sseContent), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			return []byte(strings.TrimPrefix(line, "data: ")), nil
		}
	}
	return nil, fmt.Errorf("no data field found in SSE response")
}

// parseSSEMergedDataEvents merges all non-[DONE] data events from SSE content into a single
// JSON map. Keys from later events overwrite keys from earlier events at the top level, so
// providers like Anthropic that spread data across multiple events are handled correctly:
//
//   - message_start → contributes "message" (contains "model") and nested "usage"
//   - message_delta → contributes top-level "usage" with final input/output token counts
//   - message_stop  → contributes only "type"; no useful fields
//
// After the merge the caller can resolve e.g. "$.message.model" and "$.usage.output_tokens"
// from a single map instead of having to scan individual events.
func parseSSEMergedDataEvents(sseContent []byte) (map[string]interface{}, error) {
	lines := strings.Split(string(sseContent), "\n")
	merged := make(map[string]interface{})
	found := false
	var currentData []string

	flushEvent := func() {
		if len(currentData) == 0 {
			return
		}
		payload := strings.Join(currentData, "\n")
		currentData = currentData[:0]
		if payload == "[DONE]" {
			return
		}
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(payload), &obj); err != nil {
			return
		}
		for k, v := range obj {
			merged[k] = v
		}
		found = true
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			currentData = append(currentData, strings.TrimPrefix(line, "data: "))
		} else if line == "" {
			flushEvent()
		}
	}
	flushEvent()

	if !found {
		return nil, fmt.Errorf("no valid data events found in SSE response")
	}
	return merged, nil
}

// convertToInt64 converts various numeric types to int64
func convertToInt64(value interface{}) (int64, error) {
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(v.Uint()), nil
	case reflect.Float32, reflect.Float64:
		return int64(v.Float()), nil
	case reflect.String:
		s := v.String()
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i, nil
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return int64(f), nil
		}
		return 0, fmt.Errorf("cannot convert string %q to int64", s)
	default:
		return 0, fmt.Errorf("cannot convert type %T to int64", value)
	}
}

// getPayloadFlags derives per-direction payload capture flags from policy parameters.
// New parameters request_body and response_body take precedence. When neither
// is provided, the deprecated allow_payloads flag is used as a fallback, mapping to
// both directions for backward compatibility.
func getPayloadFlags(params map[string]interface{}) (sendRequestBody, sendResponseBody bool) {
	if params == nil {
		return false, false
	}

	hasReq, hasResp := false, false

	if raw, ok := params["request_body"]; ok {
		sendRequestBody = parseBoolLike(raw)
		hasReq = true
	}
	if raw, ok := params["response_body"]; ok {
		sendResponseBody = parseBoolLike(raw)
		hasResp = true
	}

	// If either of the new flags has been explicitly configured, do not consult
	// the deprecated allow_payloads flag.
	if hasReq || hasResp {
		return sendRequestBody, sendResponseBody
	}

	// Backward compatibility: fall back to allow_payloads when new flags are absent.
	if raw, ok := params["allow_payloads"]; ok {
		if parseBoolLike(raw) {
			return true, true
		}
	}

	return false, false
}

// parseBoolLike interprets bool and common string ("true"/"1"/"yes") representations
// of a boolean policy parameter.
func parseBoolLike(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		lower := strings.ToLower(strings.TrimSpace(val))
		return lower == "true" || lower == "1" || lower == "yes"
	default:
		return false
	}
}

// getHeaderFlags derives per-direction header capture flags from policy parameters.
func getHeaderFlags(params map[string]interface{}) (sendRequestHeaders, sendResponseHeaders bool) {
	if params == nil {
		return false, false
	}
	if raw, ok := params["request_headers"]; ok {
		sendRequestHeaders = parseBoolLike(raw)
	}
	if raw, ok := params["response_headers"]; ok {
		sendResponseHeaders = parseBoolLike(raw)
	}
	return sendRequestHeaders, sendResponseHeaders
}

// serializeHeaders renders all headers as a JSON object string ({"name":"v1, v2"}),
// matching the request_headers/response_headers format the analytics engine reads.
// Returns "" when there are no headers. Sensitive values are not masked here; the
// stdout/log publisher applies masked_headers on output.
func serializeHeaders(headers *policy.Headers) string {
	all := headers.GetAll()
	if len(all) == 0 {
		return ""
	}
	flat := make(map[string]string, len(all))
	for name, values := range all {
		flat[name] = strings.Join(values, ", ")
	}
	data, err := json.Marshal(flat)
	if err != nil {
		slog.Error("Failed to marshal headers for analytics", "error", err)
		return ""
	}
	return string(data)
}

// Helper to extract string values via JSONPath
func extractStringFromJsonpath(payload map[string]interface{}, path string) string {
	val, err := utils.ExtractValueFromJsonpath(payload, path)
	if err != nil || val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}

// Helper to extract int values via JSONPath
func extractIntFromJsonpath(payload map[string]interface{}, path string) (int, error) {
	val, err := utils.ExtractValueFromJsonpath(payload, path)
	if err != nil || val == nil {
		return 0, fmt.Errorf("value not found at path %s", path)
	}
	switch v := val.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("unexpected type %T at path %s", val, path)
	}
}

// Helper to extract bool values via JSONPath
func extractBoolFromJsonpath(payload map[string]interface{}, path string) (bool, error) {
	val, err := utils.ExtractValueFromJsonpath(payload, path)
	if err != nil || val == nil {
		return false, fmt.Errorf("value not found at path %s", path)
	}
	switch v := val.(type) {
	case bool:
		return v, nil
	case string:
		lower := strings.ToLower(strings.TrimSpace(v))
		return lower == "true" || lower == "1" || lower == "yes", nil
	default:
		return false, fmt.Errorf("unexpected type %T at path %s", val, path)
	}
}

// deriveMCPCapability maps an MCP JSON-RPC method to a capability type for analytics.
// It returns "" when the method has no mapping, in which case the caller omits the
// field (mirrors carbon-apimgt's SynapseAnalyticsDataProvider behavior).
func deriveMCPCapability(method string) string {
	switch {
	case strings.HasPrefix(method, "tools/"):
		return "TOOL"
	case strings.HasPrefix(method, "resources/"):
		return "RESOURCE"
	case strings.HasPrefix(method, "prompts/"):
		return "PROMPT"
	default:
		return ""
	}
}
