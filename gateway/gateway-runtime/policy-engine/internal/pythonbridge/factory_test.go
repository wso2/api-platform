package pythonbridge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/kernel"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/pythonbridge/proto"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

func TestProcessingModeFromProto(t *testing.T) {
	mode, err := processingModeFromProto(&proto.ProcessingMode{
		RequestHeaderMode:  proto.HeaderProcessingMode_HEADER_PROCESSING_MODE_PROCESS,
		RequestBodyMode:    proto.BodyProcessingMode_BODY_PROCESSING_MODE_STREAM,
		ResponseHeaderMode: proto.HeaderProcessingMode_HEADER_PROCESSING_MODE_SKIP,
		ResponseBodyMode:   proto.BodyProcessingMode_BODY_PROCESSING_MODE_BUFFER,
	})
	require.NoError(t, err)

	assert.Equal(t, policy.HeaderModeProcess, mode.RequestHeaderMode)
	assert.Equal(t, policy.BodyModeStream, mode.RequestBodyMode)
	assert.Equal(t, policy.HeaderModeSkip, mode.ResponseHeaderMode)
	assert.Equal(t, policy.BodyModeBuffer, mode.ResponseBodyMode)
}

func TestProcessingModeFromProtoRejectsUnsupportedValues(t *testing.T) {
	_, err := processingModeFromProto(&proto.ProcessingMode{
		RequestHeaderMode: proto.HeaderProcessingMode_HEADER_PROCESSING_MODE_UNSPECIFIED,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request_header_mode")
}

func TestCapabilitiesFromProto(t *testing.T) {
	caps := capabilitiesFromProto(&proto.PolicyCapabilities{
		RequestHeaders:    true,
		RequestBody:       true,
		ResponseHeaders:   true,
		ResponseBody:      false,
		StreamingRequest:  true,
		StreamingResponse: false,
	})

	assert.Equal(t, policyCapabilities{
		requestHeaders:    true,
		requestBody:       true,
		responseHeaders:   true,
		responseBody:      false,
		streamingRequest:  true,
		streamingResponse: false,
	}, caps)
}

func TestValidateModeAndCapabilities(t *testing.T) {
	validMode := policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeStream,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeBuffer,
	}
	validCaps := policyCapabilities{
		requestHeaders:    true,
		requestBody:       true,
		responseHeaders:   false,
		responseBody:      true,
		streamingRequest:  true,
		streamingResponse: false,
	}
	if err := validateModeAndCapabilities(validMode, validCaps); err != nil {
		t.Fatalf("expected valid mode/capabilities, got error: %v", err)
	}

	invalidCaps := validCaps
	invalidCaps.requestBody = false
	if err := validateModeAndCapabilities(validMode, invalidCaps); err == nil {
		t.Fatal("expected validation to fail when streaming request fallback is missing")
	}
}

func TestBridgeImplementsAllInterfaces(t *testing.T) {
	var wrapped policy.Policy = &bridge{}

	_, hasRequestHeaders := wrapped.(policy.RequestHeaderPolicy)
	_, hasResponseHeaders := wrapped.(policy.ResponseHeaderPolicy)
	_, hasRequestBody := wrapped.(policy.RequestPolicy)
	_, hasResponseBody := wrapped.(policy.ResponsePolicy)
	_, hasStreamingRequest := wrapped.(policy.StreamingRequestPolicy)
	_, hasStreamingResponse := wrapped.(policy.StreamingResponsePolicy)

	assert.True(t, hasRequestHeaders, "bridge must implement RequestHeaderPolicy")
	assert.True(t, hasResponseHeaders, "bridge must implement ResponseHeaderPolicy")
	assert.True(t, hasRequestBody, "bridge must implement RequestPolicy")
	assert.True(t, hasResponseBody, "bridge must implement ResponsePolicy")
	assert.True(t, hasStreamingRequest, "bridge must implement StreamingRequestPolicy")
	assert.True(t, hasStreamingResponse, "bridge must implement StreamingResponsePolicy")
}

func TestBuildPolicyChain_UsesBridgeModeForParticipation(t *testing.T) {
	tests := []struct {
		name                 string
		mode                 policy.ProcessingMode
		wantRequestHeader    bool
		wantRequestStreaming bool
	}{
		{
			name: "buffered body with skipped headers",
			mode: policy.ProcessingMode{
				RequestHeaderMode: policy.HeaderModeSkip,
				RequestBodyMode:   policy.BodyModeBuffer,
			},
			wantRequestHeader:    false,
			wantRequestStreaming: false,
		},
		{
			name: "streaming body with processed headers",
			mode: policy.ProcessingMode{
				RequestHeaderMode: policy.HeaderModeProcess,
				RequestBodyMode:   policy.BodyModeStream,
			},
			wantRequestHeader:    true,
			wantRequestStreaming: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &registry.PolicyRegistry{
				Policies: make(map[string]*registry.PolicyEntry),
			}
			require.NoError(t, reg.SetConfig(map[string]interface{}{}))
			require.NoError(t, reg.Register(&policy.PolicyDefinition{
				Name:    "python-test",
				Version: "v1.0.0",
			}, func(metadata policy.PolicyMetadata, params map[string]interface{}) (policy.Policy, error) {
				return &bridge{mode: tt.mode}, nil
			}))

			specs := []policy.PolicySpec{
				{
					Name:       "python-test",
					Version:    "v1",
					Enabled:    true,
					Parameters: policy.PolicyParameters{Raw: map[string]interface{}{}},
				},
			}

			k := kernel.NewKernel()
			chain, err := k.BuildPolicyChain("route", specs, reg, policy.PolicyMetadata{})
			require.NoError(t, err)
			assert.Equal(t, tt.wantRequestHeader, chain.RequiresRequestHeader)
			assert.Equal(t, tt.wantRequestStreaming, chain.SupportsRequestStreaming)
		})
	}
}
