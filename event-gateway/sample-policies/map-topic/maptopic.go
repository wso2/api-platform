package maptopic

import (
	"context"
	"log/slog"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// MapTopicPolicy routes messages to specific Kafka topics in WebBrokerApi channels.
// For mode="produceTo": Sets the target topic on outbound messages
// For mode="consumeFrom": Provides metadata for Kafka consumer subscription (no processing)
type MapTopicPolicy struct {
	mode  string
	topic string
}

// GetPolicy returns the policy instance.
func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	mode := ""
	topic := ""

	if modeVal, ok := params["mode"]; ok {
		if modeStr, ok := modeVal.(string); ok {
			mode = modeStr
		}
	}

	if topicVal, ok := params["topic"]; ok {
		if topicStr, ok := topicVal.(string); ok {
			topic = topicStr
		}
	}

	slog.Debug("[Map Topic]: Policy created", "mode", mode, "topic", topic)
	return &MapTopicPolicy{mode: mode, topic: topic}, nil
}

// Mode returns the processing mode for this policy.
func (p *MapTopicPolicy) Mode() policy.ProcessingMode {
	if p.mode == "produceTo" {
		// For produceTo mode, we need to process the request body to set the topic
		return policy.ProcessingMode{
			RequestHeaderMode:  policy.HeaderModeSkip,
			RequestBodyMode:    policy.BodyModeBuffer, // Need to set topic on message
			ResponseHeaderMode: policy.HeaderModeSkip,
			ResponseBodyMode:   policy.BodyModeSkip,
		}
	}
	// For consumeFrom mode, this is metadata-only, skip all processing
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeSkip,
	}
}

// OnRequestBody sets the Kafka topic for produceTo mode.
func (p *MapTopicPolicy) OnRequestBody(_ context.Context, ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	if p.mode == "produceTo" && p.topic != "" {
		// Store the topic in metadata so the hub can extract it and apply to the message
		ctx.Metadata["kafka.topic"] = p.topic
		slog.Debug("[Map Topic]: Set target topic in metadata", "topic", p.topic)
	}
	return nil
}
