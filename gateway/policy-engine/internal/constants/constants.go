package constants

const (
	ExtProcFilterName = "api_platform.policy_engine.envoy.filters.http.ext_proc"
	ExtProcFilter     = "envoy.filters.http.ext_proc"

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