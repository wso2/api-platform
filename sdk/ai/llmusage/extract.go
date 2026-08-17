// Package llmusage extracts normalized LLM token usage from a provider
// response, using the field locations declared in the route's
// LlmProviderTemplate rather than per-provider Go code. Results are memoized
// in SharedContext so several policies on one route extract once.
package llmusage

import (
	"errors"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// Metadata keys the library reads from and writes to SharedContext.
const (
	// MetadataTemplateHandle is set by the kernel and names the route's template.
	MetadataTemplateHandle = "template_handle"

	// UsageKey holds the extracted Usage. The version suffix allows the shape
	// to change without breaking existing readers.
	UsageKey = "llm:usage:v1"

	// StatusKey reports whether extraction succeeded, so a consumer can tell
	// a genuinely free request from a failed extraction.
	StatusKey = "llm:usage:status"
)

// Status values written to StatusKey.
const (
	StatusExtracted       = "extracted"
	StatusTemplateMissing = "template_missing"
	StatusNoUsage         = "no_usage"
)

// ResourceTypeLLMProviderTemplate is the lazy-resource type holding templates.
const ResourceTypeLLMProviderTemplate = "LlmProviderTemplate"

// streamAccumKey and streamIndexKey hold per-request streaming state. Both are
// removed by Accumulate at end of stream.
const (
	streamAccumKey = "llm:usage:stream-accum"
	streamIndexKey = "llm:usage:stream-index"
)

// ErrNoTemplate reports that the route carries no resolvable template, so
// there are no field locations to extract from.
var ErrNoTemplate = errors.New("llmusage: no template for route")

// Get returns normalized usage for this request. The first call extracts and
// stores the result; later calls return the stored value, so several policies
// on one route pay for extraction once.
func Get(sc *policy.SharedContext, body, requestBody []byte, requestPath string) (Usage, error) {
	if sc == nil {
		return Usage{}, ErrNoTemplate
	}
	if sc.Metadata == nil {
		sc.Metadata = make(map[string]interface{})
	}

	if cached, ok := sc.Metadata[UsageKey].(Usage); ok {
		return cached, nil
	}

	template, err := templateForRoute(sc)
	if err != nil {
		sc.Metadata[StatusKey] = StatusTemplateMissing
		return Usage{}, err
	}

	usage, err := extractUsage(template, body, requestBody, requestPath)
	if err != nil {
		sc.Metadata[StatusKey] = StatusNoUsage
		return Usage{}, err
	}

	Publish(sc, usage)
	return usage, nil
}

// Publish stores usage and its status in SharedContext for other policies.
func Publish(sc *policy.SharedContext, u Usage) {
	if sc == nil {
		return
	}
	if sc.Metadata == nil {
		sc.Metadata = make(map[string]interface{})
	}

	sc.Metadata[UsageKey] = u
	if u.TotalInputTokens == 0 && u.OutputTokens == 0 {
		sc.Metadata[StatusKey] = StatusNoUsage
		return
	}
	sc.Metadata[StatusKey] = StatusExtracted
}

// Accumulate appends a stream chunk to the shared buffer and returns the buffer
// so far. Chunks are deduped by index, so several policies accumulating the
// same stream produce one buffer rather than one each. At end of stream the
// buffer is returned and the state removed.
func Accumulate(sc *policy.SharedContext, chunk *policy.StreamBody) []byte {
	if sc == nil || chunk == nil {
		return nil
	}
	if sc.Metadata == nil {
		sc.Metadata = make(map[string]interface{})
	}

	buffered, _ := sc.Metadata[streamAccumKey].([]byte)
	lastIndex, seen := sc.Metadata[streamIndexKey].(uint64)

	isNew := !seen || chunk.Index > lastIndex
	if isNew && len(chunk.Chunk) > 0 {
		buffered = append(buffered, chunk.Chunk...)
		sc.Metadata[streamAccumKey] = buffered
	}
	if isNew {
		sc.Metadata[streamIndexKey] = chunk.Index
	}

	if chunk.EndOfStream {
		delete(sc.Metadata, streamAccumKey)
		delete(sc.Metadata, streamIndexKey)
	}

	return buffered
}

// templateForRoute resolves the route's template from the lazy-resource store
// using the handle the kernel placed in SharedContext.
func templateForRoute(sc *policy.SharedContext) (map[string]interface{}, error) {
	handle, ok := sc.Metadata[MetadataTemplateHandle].(string)
	if !ok || handle == "" {
		return nil, ErrNoTemplate
	}

	resource, err := policy.GetLazyResourceStoreInstance().
		GetResourceByIDAndType(handle, ResourceTypeLLMProviderTemplate)
	if err != nil || resource == nil {
		return nil, ErrNoTemplate
	}

	spec, ok := resource.Resource["spec"].(map[string]interface{})
	if ok {
		return spec, nil
	}
	return resource.Resource, nil
}
