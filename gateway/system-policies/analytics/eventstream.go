package analytics

import (
	"encoding/binary"
	"encoding/json"
	"strings"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// Bedrock's streaming APIs (converse-stream, invoke-with-response-stream) answer in Amazon's
// binary event-stream framing, which no template JSON path can address. The decoder below turns
// a complete stream back into the JSON the awsbedrock template reads, so streamed Bedrock calls
// report their usage like buffered ones.

const (
	eventStreamContentType = "application/vnd.amazon.eventstream"
	eventStreamPreludeLen  = 12
	eventStreamOverhead    = 16
	eventStreamMaxFrame    = 16 * 1024 * 1024
)

type eventStreamFrame struct {
	eventType string
	payload   []byte
}

// isEventStreamContent reports whether the response is declared as an Amazon event stream.
func isEventStreamContent(headers *policy.Headers) bool {
	if headers == nil {
		return false
	}
	contentTypes := headers.Get("content-type")
	return len(contentTypes) > 0 && strings.Contains(strings.ToLower(contentTypes[0]), eventStreamContentType)
}

// decodeEventStream returns the JSON carried by a complete Amazon event-stream body: the
// metadata event of a Converse stream (which holds the usage), or the merged model-native chunks
// of an InvokeModelWithResponseStream stream. A body that cannot be decoded is returned as is.
func decodeEventStream(body []byte) []byte {
	frames, ok := eventStreamFrames(body)
	if !ok {
		return body
	}
	var metadata []byte
	var chunks [][]byte
	for _, frame := range frames {
		switch frame.eventType {
		case "metadata":
			if json.Valid(frame.payload) {
				metadata = frame.payload
			}
		case "chunk":
			if payload, ok := invokeChunkPayload(frame.payload); ok {
				chunks = append(chunks, payload)
			}
		}
	}
	if metadata != nil {
		return metadata
	}
	if merged, ok := mergeStreamEvents(chunks); ok {
		return merged
	}
	return body
}

// invokeChunkPayload unwraps one InvokeModelWithResponseStream chunk. On the wire a chunk is the
// raw model JSON, but intermediaries may serialize the SDK union member instead, as
// {"bytes":"<base64>"} or {"chunk":{"bytes":"<base64>"}}; all three forms are accepted.
func invokeChunkPayload(payload []byte) ([]byte, bool) {
	if !json.Valid(payload) {
		return nil, false
	}
	var envelope struct {
		Bytes []byte `json:"bytes"`
		Chunk *struct {
			Bytes []byte `json:"bytes"`
		} `json:"chunk"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, false
	}
	switch {
	case len(envelope.Bytes) != 0:
		payload = envelope.Bytes
	case envelope.Chunk != nil && len(envelope.Chunk.Bytes) != 0:
		payload = envelope.Chunk.Bytes
	}
	return payload, json.Valid(payload)
}

// mergeStreamEvents merges streamed JSON events into one object. Later events win at the top
// level, but "usage" maps are deep-merged so that input tokens from an early event survive a
// later event that only carries output tokens.
func mergeStreamEvents(events [][]byte) ([]byte, bool) {
	merged := make(map[string]interface{})
	for _, data := range events {
		var event map[string]interface{}
		if err := json.Unmarshal(data, &event); err != nil {
			continue
		}
		for key, value := range event {
			if key == "usage" {
				if incoming, ok := value.(map[string]interface{}); ok {
					if existing, ok := merged[key].(map[string]interface{}); ok {
						for k, v := range incoming {
							existing[k] = v
						}
						continue
					}
				}
			}
			merged[key] = value
		}
	}
	if len(merged) == 0 {
		return nil, false
	}
	out, err := json.Marshal(merged)
	return out, err == nil
}

// eventStreamFrames splits a complete event-stream body into frames. Integrity (the prelude and
// message CRCs) is left to the transport; malformed lengths or headers make the body undecodable
// rather than risking an out-of-bounds read.
func eventStreamFrames(data []byte) ([]eventStreamFrame, bool) {
	var frames []eventStreamFrame
	for offset := 0; offset < len(data); {
		remaining := data[offset:]
		if len(remaining) < eventStreamPreludeLen {
			return nil, false
		}
		totalLen := int(binary.BigEndian.Uint32(remaining[0:4]))
		headersLen := int(binary.BigEndian.Uint32(remaining[4:8]))
		if totalLen < eventStreamOverhead || totalLen > eventStreamMaxFrame ||
			headersLen > totalLen-eventStreamOverhead || totalLen > len(remaining) {
			return nil, false
		}
		headersEnd := eventStreamPreludeLen + headersLen
		eventType, ok := eventStreamEventType(remaining[eventStreamPreludeLen:headersEnd])
		if !ok {
			return nil, false
		}
		frames = append(frames, eventStreamFrame{eventType: eventType, payload: remaining[headersEnd : totalLen-4]})
		offset += totalLen
	}
	return frames, len(frames) != 0
}

// eventStreamEventType walks a frame's packed headers and returns its :event-type value.
func eventStreamEventType(data []byte) (string, bool) {
	var eventType string
	for offset := 0; offset < len(data); {
		nameLen := int(data[offset])
		offset++
		if offset+nameLen >= len(data) {
			return "", false
		}
		name := string(data[offset : offset+nameLen])
		offset += nameLen
		valueType := data[offset]
		offset++

		var valueLen, valueOffset int
		switch valueType {
		case 0, 1: // true, false
		case 2: // byte
			valueLen = 1
		case 3: // short
			valueLen = 2
		case 4: // integer
			valueLen = 4
		case 5, 8: // long, timestamp
			valueLen = 8
		case 9: // UUID
			valueLen = 16
		case 6, 7: // byte array, string
			if offset+2 > len(data) {
				return "", false
			}
			valueLen = int(binary.BigEndian.Uint16(data[offset : offset+2]))
			valueOffset = 2
		default:
			return "", false
		}
		if offset+valueOffset+valueLen > len(data) {
			return "", false
		}
		if name == ":event-type" && valueType == 7 {
			eventType = string(data[offset+valueOffset : offset+valueOffset+valueLen])
		}
		offset += valueOffset + valueLen
	}
	return eventType, true
}
