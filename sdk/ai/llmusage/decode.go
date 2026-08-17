package llmusage

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/wso2/api-platform/sdk/core/utils"
)

// decodeBody turns a response body into a single JSON object suitable for
// JSONPath extraction. Server-sent event streams are merged so values spread
// across events — the model in an early event, usage in the last — end up in
// one view. Anything else is returned unchanged.
func decodeBody(body []byte, requestPath string) []byte {
	if isSSE(body) {
		if merged, ok := mergeSSEEvents(body); ok {
			return merged
		}
	}
	return body
}

// isSSE reports whether the body looks like a server-sent event stream.
func isSSE(body []byte) bool {
	trimmed := bytes.TrimLeft(body, " \t\r\n")
	return bytes.HasPrefix(trimmed, []byte("data:")) ||
		bytes.HasPrefix(trimmed, []byte("event:"))
}

// mergeSSEEvents shallow-merges every JSON event in a stream, later events
// winning. Empty strings never displace a value already seen, so a trailing
// event with an empty model cannot erase a real one.
func mergeSSEEvents(body []byte) ([]byte, bool) {
	merged := make(map[string]interface{})
	found := false

	for _, line := range bytes.Split(body, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
			continue
		}

		var event map[string]interface{}
		if err := json.Unmarshal(payload, &event); err != nil {
			continue
		}
		found = true
		for key, value := range event {
			if str, ok := value.(string); ok && str == "" {
				continue
			}
			merged[key] = value
		}
	}

	if !found {
		return nil, false
	}
	out, err := json.Marshal(merged)
	if err != nil {
		return nil, false
	}
	return out, true
}

// extractUsage reads every field the template declares out of the response and
// normalizes the result. A field whose path is absent from the response
// contributes zero; that is not an error, since providers omit fields routinely.
func extractUsage(template map[string]interface{}, body, requestBody []byte, requestPath string) (Usage, error) {
	fields, accounting := resolveFields(template, requestPath)
	decoded := decodeBody(body, requestPath)

	// Confirm the body is usable before reading fields, so a malformed
	// response is reported rather than silently yielding zeros.
	var probe interface{}
	if err := json.Unmarshal(decoded, &probe); err != nil {
		return Usage{}, err
	}

	raw := rawCounts{
		InputTokens:        readInt(decoded, fields, "promptTokens", requestPath),
		OutputTokens:       readInt(decoded, fields, "completionTokens", requestPath),
		TotalTokens:        readInt(decoded, fields, "totalTokens", requestPath),
		CachedTokens:       readInt(decoded, fields, "cachedTokens", requestPath),
		CacheWriteTokens:   readInt(decoded, fields, "cacheWriteTokens", requestPath),
		CacheWrite1hTokens: readInt(decoded, fields, "cacheWrite1hTokens", requestPath),
		ReasoningTokens:    readInt(decoded, fields, "reasoningTokens", requestPath),
		AudioInputTokens:   readInt(decoded, fields, "audioInputTokens", requestPath),
		AudioOutputTokens:  readInt(decoded, fields, "audioOutputTokens", requestPath),
		ServiceTier:        readString(decoded, fields, "serviceTier", requestPath),
	}

	raw.Model, raw.ModelCandidates = resolveModel(decoded, requestBody, fields, requestPath)

	return normalize(raw, accounting), nil
}

// resolveModel prefers the model the response reports, falling back to the one
// the request asked for. Both candidates are returned in the order tried.
func resolveModel(body, requestBody []byte, fields map[string]fieldSpec, requestPath string) (string, []string) {
	var candidates []string

	if name := readString(body, fields, "responseModel", requestPath); name != "" {
		candidates = append(candidates, name)
	}
	if len(requestBody) > 0 {
		if name := readString(requestBody, fields, "requestModel", requestPath); name != "" {
			candidates = append(candidates, name)
		}
	}

	if len(candidates) == 0 {
		return "", nil
	}
	return candidates[0], candidates
}

// readInt reads a declared field as an integer, returning zero when the field
// is not declared or the path is absent from the payload.
func readInt(payload []byte, fields map[string]fieldSpec, name, requestPath string) int64 {
	value := readString(payload, fields, name, requestPath)
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return int64(parsed)
}

// readString reads a declared field, trying its identifier and then each
// fallback in order and returning the first that yields a non-empty value.
// Payload fields are read from the given JSON document; pathParam fields are
// matched against the request path.
func readString(payload []byte, fields map[string]fieldSpec, name, requestPath string) string {
	spec, ok := fields[name]
	if !ok {
		return ""
	}

	for _, identifier := range spec.Identifiers {
		var value string
		switch spec.Location {
		case locationPayload:
			extracted, err := utils.ExtractStringValueFromJsonpath(payload, identifier)
			if err != nil {
				continue
			}
			value = strings.TrimSpace(extracted)
		case locationPathParam:
			value = extractFromPath(requestPath, identifier)
		default:
			return ""
		}
		if value != "" {
			return mapValue(spec, value)
		}
	}
	return ""
}

// mapValue translates a value the provider reported into the vocabulary the
// library reports. A value absent from the map is returned unchanged, so a
// field with no valueMap behaves as though the map were not there.
func mapValue(spec fieldSpec, raw string) string {
	if spec.ValueMap == nil || raw == "" {
		return raw
	}
	if mapped, ok := spec.ValueMap[raw]; ok {
		return mapped
	}
	return raw
}

// locationPayload is the ExtractionIdentifier location for a response body.
const locationPayload = "payload"

// locationPathParam is the ExtractionIdentifier location for the request path.
const locationPathParam = "pathParam"
