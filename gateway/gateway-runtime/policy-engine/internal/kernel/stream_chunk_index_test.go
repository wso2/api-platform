package kernel

import (
	"context"
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// indexRecordingPolicy records the Index of every chunk the kernel delivers.
type indexRecordingPolicy struct {
	seen []uint64
}

func (p *indexRecordingPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{ResponseBodyMode: policy.BodyModeStream}
}

func (p *indexRecordingPolicy) OnResponseBody(_ context.Context, _ *policy.ResponseContext, _ map[string]interface{}) policy.ResponseAction {
	return policy.DownstreamResponseModifications{}
}

// NeedsMoreResponseData returns false so the kernel flushes every chunk, which
// is what a policy doing its own accumulation asks for.
func (p *indexRecordingPolicy) NeedsMoreResponseData(_ []byte) bool { return false }

func (p *indexRecordingPolicy) OnResponseBodyChunk(_ context.Context, _ *policy.ResponseStreamContext, chunk *policy.StreamBody, _ map[string]interface{}) policy.StreamingResponseAction {
	p.seen = append(p.seen, chunk.Index)
	return policy.ForwardResponseChunk{}
}

// A policy that stitches chunks together tells a new chunk from a redelivery by
// its Index, so the kernel must advance it on every delivery. When it does not,
// such a policy keeps only the first chunk and silently works off a partial
// body — which is how streaming LLM responses ended up priced at zero.
func TestStreamingResponse_ChunkIndexAdvancesPerDelivery(t *testing.T) {
	rec := &indexRecordingPolicy{}

	kernel := NewKernel()
	server := NewExternalProcessorServer(kernel, newTestExecutor(), config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)
	chain := &registry.PolicyChain{
		RequiresResponseBody:      true,
		SupportsResponseStreaming: true,
		Policies:                  []policy.Policy{rec},
		PolicySpecs:               []policy.PolicySpec{{Enabled: true}},
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)

	execCtx.buildRequestContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}}, RouteMetadata{})
	execCtx.buildResponseContexts(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":status", RawValue: []byte("200")},
				{Key: "content-type", RawValue: []byte("text/event-stream")},
			},
		},
	})

	chunks := []string{
		"data: {\"model\":\"m\"}\n\n",
		"data: {\"choices\":[]}\n\n",
		"data: {\"usage\":{\"prompt_tokens\":6}}\n\n",
		"data: [DONE]\n\n",
	}
	for i, c := range chunks {
		_, err := execCtx.processStreamingResponseBody(context.Background(), &extprocv3.HttpBody{
			Body:        []byte(c),
			EndOfStream: i == len(chunks)-1,
		})
		require.NoError(t, err)
	}

	require.Len(t, rec.seen, len(chunks), "every chunk should reach the policy")
	for i := 1; i < len(rec.seen); i++ {
		assert.Greater(t, rec.seen[i], rec.seen[i-1],
			"chunk %d index %d must exceed chunk %d index %d; equal indices make accumulating policies discard the chunk",
			i, rec.seen[i], i-1, rec.seen[i-1])
	}
}
