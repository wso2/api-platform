package analytics

import (
	"encoding/json"
	"log/slog"
)

// ─── Model gateway analytics ─────────────────────────────────────────────────
//
// A model endpoint serves the Models that name it on routes of one inbound format (OpenAI,
// Anthropic or Gemini), each request dispatched to that Model's chain and from there to one of
// its target providers. Its events are reported under the endpoint, which compiles as an
// LlmProxy, but the endpoint's own template is its inbound one for every request — the route
// has to name one scope for all of them — so without this the provider of every model
// endpoint request would be reported as the inbound format's vendor.
//
// The facts come from the model system policy's route job, which records the target it
// selected into SharedContext.Metadata. They are read here in the response phases only: on
// the request the collector runs first in the chain, before the target has been chosen.

// Metadata keys written by the model system policy (wso2_apip_sys_model) when it selects a
// target. Read, never written. The policy lives in another module, so the literals are mirrored
// here and pinned by a test on each side.
const (
	modelHandleKey              = "model.handle"
	modelNameKey                = "model.name"
	modelTargetKey              = "model.target"
	modelProviderHandleKey      = "model.provider.handle"
	modelProviderTemplateKey    = "model.provider.template"
	modelProviderDisplayNameKey = "model.provider.displayName"
	modelUpstreamModelKey       = "model.upstreamModel"
	modelResponseModelKey       = "model.responseModel"
)

// ModelPropertiesKey carries the JSON-encoded model gateway facts for a request, in the same
// way A2ARequestPropertiesKey and the MCP property blocks carry theirs. Consumed by the
// policy-engine's prepareAnalyticEvent, which must mirror this key name to surface the block
// on the event.
const ModelPropertiesKey = "llm_model_properties"

// ModelAnalyticsProperties are the model gateway facts for one request: which Model it asked
// for, which of the Model's targets served it, and the provider behind that target.
//
// Every value is gateway configuration rather than request content — the route job resolves
// whatever the client or the router named to one of the Model's configured targets, and
// records that target's configured values — so none of them is caller-controlled and the
// block cannot widen a dimension.
type ModelAnalyticsProperties struct {
	// Handle and Name identify the Model: the handle it is managed by and the name clients
	// request it by.
	Handle string `json:"handle,omitempty"`
	Name   string `json:"name,omitempty"`
	// Target is the name of the Model's target that served the request, and UpstreamModel
	// the model that target asked its provider for.
	Target        string `json:"target,omitempty"`
	UpstreamModel string `json:"upstreamModel,omitempty"`
	// ProviderHandle, ProviderTemplate and ProviderDisplayName describe the provider behind
	// the target: the LlmProvider it is, the template its wire format follows, and the name
	// it is displayed under.
	ProviderHandle      string `json:"providerHandle,omitempty"`
	ProviderTemplate    string `json:"providerTemplate"`
	ProviderDisplayName string `json:"providerDisplayName,omitempty"`
}

// modelFacts returns the model gateway facts recorded for this request, or false when the
// request is not one — which is every request on every other route, and a model gateway
// request a policy refused before its target was chosen.
//
// The provider template is the test. It is the one fact the provider report below needs, and
// the route job records it with all the others, so a request without it has none of them.
func modelFacts(metadata map[string]interface{}) (ModelAnalyticsProperties, bool) {
	template := metadataStringValue(metadata, modelProviderTemplateKey)
	if template == "" {
		return ModelAnalyticsProperties{}, false
	}
	return ModelAnalyticsProperties{
		Handle:              metadataStringValue(metadata, modelHandleKey),
		Name:                metadataStringValue(metadata, modelNameKey),
		Target:              metadataStringValue(metadata, modelTargetKey),
		UpstreamModel:       metadataStringValue(metadata, modelUpstreamModelKey),
		ProviderHandle:      metadataStringValue(metadata, modelProviderHandleKey),
		ProviderTemplate:    template,
		ProviderDisplayName: metadataStringValue(metadata, modelProviderDisplayNameKey),
	}, true
}

// populateModelAnalyticsMetadata emits the model gateway facts block. A request that is not a
// model gateway request gets nothing, so every other event is unchanged.
//
// Called from OnResponseHeaders, which every response with an upstream reaches and which
// OnFault delegates to, so a request that failed after its target was chosen still reports
// it.
func populateModelAnalyticsMetadata(analyticsMetadata map[string]any, metadata map[string]interface{}) {
	facts, ok := modelFacts(metadata)
	if !ok {
		return
	}
	data, err := json.Marshal(facts)
	if err != nil {
		slog.Error("Failed to marshal model gateway analytics properties", "error", err)
		return
	}
	analyticsMetadata[ModelPropertiesKey] = string(data)
}

// applyModelProvider reports the target's provider in place of the template's.
//
// The usage itself is right as extracted: the collector reads the response after the
// provider's transformer turned it back into the endpoint's format, so the inbound template's
// field locations are the correct ones to read it with. Only the provider those locations came from
// is wrong, and it is replaced with the provider the request actually went to — its template
// as the provider name, which is the vendor an LlmProvider route reports, and its display
// name (or its handle, when it has none) as the display name.
//
// It replaces and never introduces. A provider name is what makes the policy engine build
// the event's AI block, token usage included, so setting one where extraction produced none
// would publish a request whose usage could not be read as one that used no tokens.
func applyModelProvider(analyticsMetadata map[string]any, metadata map[string]interface{}) {
	if _, extracted := analyticsMetadata[AIProviderNameMetadataKey]; !extracted {
		return
	}
	facts, ok := modelFacts(metadata)
	if !ok {
		return
	}
	analyticsMetadata[AIProviderNameMetadataKey] = facts.ProviderTemplate
	displayName := facts.ProviderDisplayName
	if displayName == "" {
		displayName = facts.ProviderHandle
	}
	if displayName != "" {
		analyticsMetadata[AIProviderDisplayNameMetadataKey] = displayName
	}
}

// applyModelResponseModel reports the model that served the request as the model ID, in place
// of the Model's name.
//
// The route job answers the client in the Model's name, so the response the collector reads
// names the Model, not the model behind it. The route job records the provider's own value
// before replacing it, and that is the model ID; a response that named none is reported by its
// target's upstream model. The Model's name is still reported, as the request model and in the
// model block.
//
// Like applyModelProvider it replaces and never introduces: it changes a model ID extraction
// reported, and adds none where extraction reported none.
func applyModelResponseModel(analyticsMetadata map[string]any, metadata map[string]interface{}) {
	if _, extracted := analyticsMetadata[ModelIDMetadataKey]; !extracted {
		return
	}
	facts, ok := modelFacts(metadata)
	if !ok {
		return
	}
	model := metadataStringValue(metadata, modelResponseModelKey)
	if model == "" {
		model = facts.UpstreamModel
	}
	if model != "" {
		analyticsMetadata[ModelIDMetadataKey] = model
	}
}

// metadataStringValue reads one string from the shared metadata bag, treating anything else as
// absent.
func metadataStringValue(metadata map[string]interface{}, key string) string {
	if metadata == nil {
		return ""
	}
	value, _ := metadata[key].(string)
	return value
}
