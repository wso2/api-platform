package analytics

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// openAITemplateHandle is the template the model gateway's route names for every request.
// Stored under a handle no other test uses, so this file can add and remove it freely.
const openAITemplateHandle = "model-facts-openai"

// withOpenAITemplate makes the OpenAI-shaped template resolvable from the lazy resource
// store for the duration of a test.
func withOpenAITemplate(t *testing.T) {
	t.Helper()
	store := policy.GetLazyResourceStoreInstance()
	err := store.StoreResource(&policy.LazyResource{
		ID:           openAITemplateHandle,
		ResourceType: lazyResourceTypeLLMProviderTemplate,
		Resource: map[string]interface{}{
			"metadata": map[string]interface{}{"name": "openai"},
			"spec": map[string]interface{}{
				"displayName":      "OpenAI",
				"promptTokens":     map[string]interface{}{"location": "payload", "identifier": "$.usage.prompt_tokens"},
				"completionTokens": map[string]interface{}{"location": "payload", "identifier": "$.usage.completion_tokens"},
				"totalTokens":      map[string]interface{}{"location": "payload", "identifier": "$.usage.total_tokens"},
				"requestModel":     map[string]interface{}{"location": "payload", "identifier": "$.model"},
				"responseModel":    map[string]interface{}{"location": "payload", "identifier": "$.model"},
			},
		},
	})
	if err != nil {
		t.Fatalf("store template: %v", err)
	}
	t.Cleanup(func() {
		_ = store.RemoveResourceByIDAndType(openAITemplateHandle, lazyResourceTypeLLMProviderTemplate)
	})
}

// openAIResponse is what the collector sees on a model gateway response: the provider's
// reply, already translated back into the OpenAI shape.
const openAIResponse = `{"id":"c1","object":"chat.completion","model":"claude-sonnet",` +
	`"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18}}`

// modelMetadata is the metadata a model gateway request carries once its route job has
// selected a target, on top of the OpenAI scope the route job restored.
func modelMetadata() map[string]interface{} {
	return map[string]interface{}{
		"template_handle":           openAITemplateHandle,
		"provider_name":             "__wso2_models__",
		"selected_provider":         "anthropic-prod",
		modelHandleKey:              "chat-default",
		modelNameKey:                "chat",
		modelTargetKey:              "claude",
		modelProviderHandleKey:      "anthropic-prod",
		modelProviderTemplateKey:    "anthropic",
		modelProviderDisplayNameKey: "Anthropic Prod",
		modelUpstreamModelKey:       "claude-sonnet",
	}
}

// proxyMetadata is what an ordinary LlmProxy request carries: the kernel's route scope and
// nothing from the model system policy.
func proxyMetadata() map[string]interface{} {
	return map[string]interface{}{
		"template_handle": openAITemplateHandle,
		"provider_name":   "openai-prod",
	}
}

func bufferedResponse(metadata map[string]interface{}) *policy.ResponseContext {
	return &policy.ResponseContext{
		SharedContext:  &policy.SharedContext{APIKind: policy.APIKindLlmProxy, Metadata: metadata},
		RequestBody:    &policy.Body{Content: []byte(`{"model":"claude-sonnet"}`), Present: true, EndOfStream: true},
		RequestPath:    "/wso2/v1/chat/completions",
		ResponseBody:   &policy.Body{Content: []byte(openAIResponse), Present: true, EndOfStream: true},
		ResponseStatus: 200,
	}
}

// streamedResponse runs the collector over openAIResponse delivered as two chunks and returns
// the metadata it published at the end of the stream.
func streamedResponse(t *testing.T, metadata map[string]interface{}) map[string]any {
	t.Helper()
	policyInstance := &AnalyticsPolicy{}
	ctx := &policy.ResponseStreamContext{
		SharedContext:  &policy.SharedContext{APIKind: policy.APIKindLlmProxy, Metadata: metadata},
		RequestBody:    &policy.Body{Content: []byte(`{"model":"claude-sonnet"}`), Present: true, EndOfStream: true},
		RequestPath:    "/wso2/v1/chat/completions",
		ResponseStatus: 200,
	}
	half := len(openAIResponse) / 2
	if _, ok := policyInstance.OnResponseBodyChunk(context.Background(), ctx,
		&policy.StreamBody{Chunk: []byte(openAIResponse[:half])}, nil).(policy.ForwardResponseChunk); !ok {
		t.Fatal("expected the first chunk to be forwarded")
	}
	action, ok := policyInstance.OnResponseBodyChunk(context.Background(), ctx,
		&policy.StreamBody{Chunk: []byte(openAIResponse[half:]), EndOfStream: true}, nil).(policy.ForwardResponseChunk)
	if !ok {
		t.Fatal("expected the last chunk to be forwarded")
	}
	return action.AnalyticsMetadata
}

func bufferedMetadata(t *testing.T, metadata map[string]interface{}) map[string]any {
	t.Helper()
	action := (&AnalyticsPolicy{}).OnResponseBody(context.Background(), bufferedResponse(metadata), nil)
	if action == nil {
		return nil // nothing to publish
	}
	mods, ok := action.(policy.DownstreamResponseModifications)
	if !ok {
		t.Fatalf("expected DownstreamResponseModifications, got %T", action)
	}
	return mods.AnalyticsMetadata
}

func headerMetadata(t *testing.T, metadata map[string]interface{}) map[string]any {
	t.Helper()
	action := (&AnalyticsPolicy{}).OnResponseHeaders(context.Background(), &policy.ResponseHeaderContext{
		SharedContext:  &policy.SharedContext{APIKind: policy.APIKindLlmProxy, Metadata: metadata},
		ResponseStatus: 200,
	}, nil)
	mods, ok := action.(policy.DownstreamResponseHeaderModifications)
	if !ok {
		t.Fatalf("expected DownstreamResponseHeaderModifications, got %T", action)
	}
	return mods.AnalyticsMetadata
}

// The metadata keys are written by the model system policy in another module. A rename on
// one side alone reports every model gateway request as an OpenAI one, silently. The policy's
// own pinning assertion is TestMetadataKeysMatchTheirConsumers in system-policies/model.
func TestModelMetadataKeySpellingsMatchTheModelSystemPolicy(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{modelHandleKey, "model.handle"},
		{modelNameKey, "model.name"},
		{modelTargetKey, "model.target"},
		{modelProviderHandleKey, "model.provider.handle"},
		{modelProviderTemplateKey, "model.provider.template"},
		{modelProviderDisplayNameKey, "model.provider.displayName"},
		{modelUpstreamModelKey, "model.upstreamModel"},
		{modelResponseModelKey, "model.responseModel"},
		{ModelPropertiesKey, "llm_model_properties"},
	} {
		if tc.got != tc.want {
			t.Errorf("got %q, want %q", tc.got, tc.want)
		}
	}
}

// ─── unchanged for every other request ───────────────────────────────────────

// An ordinary LlmProxy request carries none of the model keys, so every hook publishes
// exactly what it published before the model gateway existed: the template's provider, the
// usage, and no model block. Asserted as whole maps, so an extra key is a failure too.
func TestRequestsOutsideTheModelGatewayAreReportedUnchanged(t *testing.T) {
	withOpenAITemplate(t)

	want := map[string]any{
		PromptTokenCountMetadataKey:      "11",
		CompletionTokenCountMetadataKey:  "7",
		TotalTokenCountMetadataKey:       "18",
		ModelIDMetadataKey:               "claude-sonnet",
		RequestModelIDMetadataKey:        "claude-sonnet",
		AIProviderNameMetadataKey:        "openai",
		AIProviderDisplayNameMetadataKey: "OpenAI",
	}
	if got := bufferedMetadata(t, proxyMetadata()); !reflect.DeepEqual(got, want) {
		t.Errorf("buffered metadata =\n%v\nwant\n%v", got, want)
	}
	if got := streamedResponse(t, proxyMetadata()); !reflect.DeepEqual(got, want) {
		t.Errorf("streamed metadata =\n%v\nwant\n%v", got, want)
	}

	headers := headerMetadata(t, proxyMetadata())
	if _, present := headers[ModelPropertiesKey]; present {
		t.Errorf("a request outside the model gateway carried %s: %v", ModelPropertiesKey, headers)
	}
	var generic map[string]interface{}
	if err := json.Unmarshal([]byte(headers[GenericMetadataKey].(string)), &generic); err != nil {
		t.Fatalf("decode %s: %v", GenericMetadataKey, err)
	}
	if !reflect.DeepEqual(generic, proxyMetadata()) {
		t.Errorf("%s = %v, want the proxy's own metadata unchanged", GenericMetadataKey, generic)
	}
}

// The helpers are no-ops on a map without the model keys, whatever else it holds.
func TestModelHelpersLeaveOtherRequestsAlone(t *testing.T) {
	for name, metadata := range map[string]map[string]interface{}{
		"nil":            nil,
		"empty":          {},
		"proxy":          proxyMetadata(),
		"partial facts":  {modelHandleKey: "chat-default", modelProviderHandleKey: "anthropic-prod"},
		"non-string key": {modelProviderTemplateKey: 7},
	} {
		t.Run(name, func(t *testing.T) {
			analyticsMetadata := map[string]any{
				AIProviderNameMetadataKey:        "openai",
				AIProviderDisplayNameMetadataKey: "OpenAI",
				ModelIDMetadataKey:               "gpt-4o",
			}
			before := map[string]any{}
			for k, v := range analyticsMetadata {
				before[k] = v
			}
			populateModelAnalyticsMetadata(analyticsMetadata, metadata)
			applyModelProvider(analyticsMetadata, metadata)
			applyModelResponseModel(analyticsMetadata, metadata)
			if !reflect.DeepEqual(analyticsMetadata, before) {
				t.Fatalf("metadata changed to %v", analyticsMetadata)
			}
		})
	}
}

// ─── the model gateway ───────────────────────────────────────────────────────

// The provider reported is the one the request went to, not the OpenAI template every model
// gateway route names; the usage is read from the translated response as before.
func TestModelGatewayRequestsReportTheTargetProvider(t *testing.T) {
	withOpenAITemplate(t)

	want := map[string]any{
		PromptTokenCountMetadataKey:      "11",
		CompletionTokenCountMetadataKey:  "7",
		TotalTokenCountMetadataKey:       "18",
		ModelIDMetadataKey:               "claude-sonnet",
		RequestModelIDMetadataKey:        "claude-sonnet",
		AIProviderNameMetadataKey:        "anthropic",
		AIProviderDisplayNameMetadataKey: "Anthropic Prod",
	}
	if got := bufferedMetadata(t, modelMetadata()); !reflect.DeepEqual(got, want) {
		t.Errorf("buffered metadata =\n%v\nwant\n%v", got, want)
	}
	if got := streamedResponse(t, modelMetadata()); !reflect.DeepEqual(got, want) {
		t.Errorf("streamed metadata =\n%v\nwant\n%v", got, want)
	}
}

// A provider without a display name is reported by its handle.
func TestTheProviderHandleStandsInForAMissingDisplayName(t *testing.T) {
	withOpenAITemplate(t)

	metadata := modelMetadata()
	metadata[modelProviderDisplayNameKey] = ""
	got := bufferedMetadata(t, metadata)
	if got[AIProviderNameMetadataKey] != "anthropic" {
		t.Errorf("%s = %v, want anthropic", AIProviderNameMetadataKey, got[AIProviderNameMetadataKey])
	}
	if got[AIProviderDisplayNameMetadataKey] != "anthropic-prod" {
		t.Errorf("%s = %v, want anthropic-prod", AIProviderDisplayNameMetadataKey, got[AIProviderDisplayNameMetadataKey])
	}
}

// Where usage could not be read, no provider is introduced: a provider name alone would make
// the event report a request of unknown usage as one that used no tokens.
func TestNoProviderIsReportedWhereUsageCouldNotBeRead(t *testing.T) {
	withoutTemplate := func() map[string]interface{} {
		metadata := modelMetadata()
		metadata["template_handle"] = "model-facts-missing-template"
		return metadata
	}
	if got := bufferedMetadata(t, withoutTemplate()); got[AIProviderNameMetadataKey] != nil {
		t.Errorf("buffered: provider reported without usage: %v", got)
	}
	if got := streamedResponse(t, withoutTemplate()); got[AIProviderNameMetadataKey] != nil {
		t.Errorf("streamed: provider reported without usage: %v", got)
	}
}

// The model block names the Model, the target and the provider behind it.
func TestModelGatewayRequestsCarryTheModelBlock(t *testing.T) {
	headers := headerMetadata(t, modelMetadata())

	raw, ok := headers[ModelPropertiesKey].(string)
	if !ok {
		t.Fatalf("expected %s as a JSON string, got %v", ModelPropertiesKey, headers)
	}
	var got ModelAnalyticsProperties
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("decode %s: %v", ModelPropertiesKey, err)
	}
	want := ModelAnalyticsProperties{
		Handle:              "chat-default",
		Name:                "chat",
		Target:              "claude",
		UpstreamModel:       "claude-sonnet",
		ProviderHandle:      "anthropic-prod",
		ProviderTemplate:    "anthropic",
		ProviderDisplayName: "Anthropic Prod",
	}
	if got != want {
		t.Fatalf("%s = %+v, want %+v", ModelPropertiesKey, got, want)
	}
}

// A request that failed after its target was chosen still reports the target: the fault
// hook delegates to the response-headers hook, which carries the block.
func TestAFailedModelGatewayRequestStillCarriesTheModelBlock(t *testing.T) {
	response := (&AnalyticsPolicy{}).OnFault(context.Background(), &policy.FaultContext{
		SharedContext:  &policy.SharedContext{APIKind: policy.APIKindLlmProxy, Metadata: modelMetadata()},
		ResponseStatus: 502,
	}, nil)
	if response == nil {
		t.Fatal("expected fault analytics")
	}
	if _, ok := response.AnalyticsMetadata[ModelPropertiesKey].(string); !ok {
		t.Fatalf("fault analytics carried no %s: %v", ModelPropertiesKey, response.AnalyticsMetadata)
	}
}

// The response the collector reads names the Model, as the client received it. The model ID is
// the model that served the request: the provider's own name for it, which the route job
// recorded before replacing it, or the target's upstream model where the response named none.
// The Model's name stays the request model.
func TestModelGatewayRequestsReportTheServingModel(t *testing.T) {
	withOpenAITemplate(t)
	answered := `{"id":"c1","object":"chat.completion","model":"chat",` +
		`"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18}}`

	for name, tc := range map[string]struct{ recorded, want string }{
		"the provider's name": {"claude-sonnet-20250929", "claude-sonnet-20250929"},
		"none recorded":       {"", "claude-sonnet"},
	} {
		t.Run(name, func(t *testing.T) {
			metadata := modelMetadata()
			if tc.recorded != "" {
				metadata[modelResponseModelKey] = tc.recorded
			}
			ctx := bufferedResponse(metadata)
			ctx.RequestBody.Content = []byte(`{"model":"chat"}`)
			ctx.ResponseBody.Content = []byte(answered)
			action, ok := (&AnalyticsPolicy{}).OnResponseBody(context.Background(), ctx, nil).(policy.DownstreamResponseModifications)
			if !ok {
				t.Fatal("expected DownstreamResponseModifications")
			}
			got := action.AnalyticsMetadata
			if got[ModelIDMetadataKey] != tc.want {
				t.Errorf("%s = %v, want %s", ModelIDMetadataKey, got[ModelIDMetadataKey], tc.want)
			}
			if got[RequestModelIDMetadataKey] != "chat" {
				t.Errorf("%s = %v, want the Model's name", RequestModelIDMetadataKey, got[RequestModelIDMetadataKey])
			}
		})
	}

	metadata := modelMetadata()
	metadata[modelResponseModelKey] = "claude-sonnet-20250929"
	if got := streamedResponse(t, metadata); got[ModelIDMetadataKey] != "claude-sonnet-20250929" {
		t.Errorf("streamed: %s = %v, want claude-sonnet-20250929", ModelIDMetadataKey, got[ModelIDMetadataKey])
	}
}
