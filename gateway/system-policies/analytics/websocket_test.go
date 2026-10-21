package analytics

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// A WebSocketApi handshake must not be reported as an unknown API kind by any
// body-phase callback — each one would otherwise log an ERROR per connection.
func TestBodyCallbacks_WebSocketKindIsRecognised(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	shared := func() *policy.SharedContext {
		return &policy.SharedContext{APIKind: policy.APIKindWebSocket, Metadata: map[string]interface{}{}}
	}
	a := &AnalyticsPolicy{}

	a.OnRequestBody(context.Background(), &policy.RequestContext{SharedContext: shared()}, nil)
	a.OnResponseBody(context.Background(), &policy.ResponseContext{SharedContext: shared()}, nil)
	a.OnResponseBodyChunk(context.Background(), &policy.ResponseStreamContext{SharedContext: shared()},
		&policy.StreamBody{EndOfStream: true}, nil)

	if strings.Contains(buf.String(), "Invalid API kind") {
		t.Errorf("WebSocketApi must be a recognised API kind, got log output:\n%s", buf.String())
	}
}
