# Semantic Prompt Guardrail

## Overview

The Semantic Prompt Guardrail validates prompts using semantic similarity matching against configured allow and deny phrase lists. Unlike keyword-based filtering, this guardrail understands the meaning of prompts by converting them to vector embeddings and comparing them using cosine similarity. This enables more intelligent content filtering that can catch semantically similar content even when exact keywords differ.

The policy uses embedding models (OpenAI, Mistral, or Azure OpenAI) to convert prompts and configured phrases into high-dimensional vectors, then performs similarity comparisons. Prompts are blocked if they are too similar to denied phrases or not similar enough to allowed phrases, based on configurable similarity thresholds.

## Features

- **Semantic similarity matching**: Uses embeddings to understand meaning, not just keywords
- **Allow/Deny phrase lists**: Configure lists of allowed and denied phrases for flexible filtering
- **Configurable similarity thresholds**: Control matching sensitivity separately for allow and deny lists (0.0 to 1.0)
- **Multiple embedding provider support**: Works with OpenAI, Mistral, and Azure OpenAI embedding services
- **JSONPath extraction**: Extract specific fields from request body for validation
- **Detailed assessment information**: Optional detailed violation information in error responses

## How It Works

1. **Text Extraction**: Extracts prompt text from the request body using JSONPath (if configured) or uses the entire request body
2. **Embedding Generation**: Generates a vector embedding from the extracted prompt using the configured embedding provider
3. **Validation Strategy**: The validation logic depends on which lists are configured:
   - **Deny list only**: Compares prompt embedding against all denied phrases. If any denied phrase has similarity >= `denySimilarityThreshold`, the request is blocked. Otherwise, it proceeds.
   - **Allow list only**: Compares prompt embedding against all allowed phrases. If no allowed phrase has similarity >= `allowSimilarityThreshold`, the request is blocked. Otherwise, it proceeds.
   - **Both lists**: First checks the deny list (blocks if similarity >= `denySimilarityThreshold`), then checks the allow list (blocks if similarity < `allowSimilarityThreshold`). Request proceeds only if it passes both checks.
4. **Validation Result**: Request proceeds if validation passes, or is blocked with HTTP 422 if validation fails

## Configuration

### Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `jsonPath` | string | No | `""` | JSONPath expression to extract the prompt from JSON payload. If empty, validates the entire payload as a string. Examples: `"$.messages[0].content"`, `"$.prompt"` |
| `allowSimilarityThreshold` | number | No | `0.65` | Minimum similarity threshold (0.0 to 1.0) for a prompt to be considered similar to an allowed phrase. Higher values mean stricter matching. If set, the prompt must match at least one allowed phrase within this threshold. |
| `denySimilarityThreshold` | number | No | `0.65` | Maximum similarity threshold (0.0 to 1.0) for a prompt to be considered similar to a denied phrase. If any denied phrase has similarity >= this threshold, the request is blocked. Higher values mean stricter blocking. |
| `allowedPhrases` | array | No* | `[]` | List of phrases that are considered safe. The prompt must match one of these within `allowSimilarityThreshold` if the threshold is configured. Embeddings are automatically generated during policy initialization. |
| `deniedPhrases` | array | No* | `[]` | List of phrases that should block the prompt when similar within the `denySimilarityThreshold`. Embeddings are automatically generated during policy initialization. |
| `showAssessment` | boolean | No | `false` | If `true`, includes detailed assessment information in error responses. If `false`, returns minimal error information. |

\* At least one of `allowedPhrases` or `deniedPhrases` must be provided.

### System Parameters (Required)

These parameters are typically configured at the gateway level and automatically injected, or you can override those values from the params section in the api artifact definition file as well:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `embeddingProvider` | string | Yes | Embedding provider type. Must be one of: `OPENAI`, `MISTRAL`, `AZURE_OPENAI` |
| `embeddingEndpoint` | string | Yes | Endpoint URL for the embedding service. Examples: OpenAI: `https://api.openai.com/v1/embeddings`, Mistral: `https://api.mistral.ai/v1/embeddings`, Azure OpenAI: Your Azure OpenAI endpoint URL |
| `embeddingModel` | string | Conditional | - | Embedding model name. **Required for OPENAI and MISTRAL**, not required for AZURE_OPENAI (deployment name is in endpoint URL). Examples: OpenAI: `text-embedding-ada-002` or `text-embedding-3-small`, Mistral: `mistral-embed` |
| `apiKey` | string | Yes | API key for the embedding service authentication |

### Configuring System Parameters in config.yaml

System parameters can be configured globally in the gateway's `config.yaml` file. These values serve as defaults for all Semantic Prompt Guard policy instances and can be overridden per-policy in the API configuration if needed.

#### Location in config.yaml

Add the following configuration section to your `config.yaml` file:

```yaml
embedding_provider: "MISTRAL" # Supported: MISTRAL, OPENAI, AZURE_OPENAI
embedding_provider_endpoint: "https://api.mistral.ai/v1/embeddings"
embedding_provider_model: "mistral-embed"
embedding_provider_dimension: 1024
embedding_provider_api_key: ""
```

## Similarity Threshold Guidelines

The similarity thresholds control how similar prompts must be to trigger allow/deny decisions:

### Allow Similarity Threshold

- **0.95-1.0**: Very strict matching. Only near-identical prompts to allowed phrases will pass. Use for exact-match scenarios.
- **0.85-0.94**: Recommended for most use cases. Catches semantically equivalent prompts with some wording variation.
- **0.75-0.84**: More flexible matching. Useful for broader conceptual similarity.
- **0.60-0.74**: Very flexible. May allow loosely related prompts.
- **Below 0.60**: Not recommended. Risk of allowing unrelated prompts.

**Recommendation**: Start with 0.65 and adjust based on your use case. Monitor false positives/negatives to fine-tune.

### Deny Similarity Threshold

- **0.95-1.0**: Very strict blocking. Only near-identical prompts to denied phrases will be blocked.
- **0.85-0.94**: Recommended for most use cases. Catches semantically equivalent prompts with some wording variation.
- **0.75-0.84**: More flexible blocking. Useful for catching variations of prohibited content.
- **0.60-0.74**: Very flexible. May block loosely related prompts.
- **Below 0.60**: Not recommended. Risk of blocking legitimate prompts.

**Recommendation**: Start with 0.65 and adjust based on your use case. Monitor false positives to fine-tune.

## JSONPath Support

The guardrail supports JSONPath expressions to extract specific text from request bodies before validation. This is useful for:
- Extracting message content from chat completion requests
- Focusing on specific prompt fields while ignoring metadata
- Handling structured JSON payloads

### Common JSONPath Examples

- `$.messages[0].content` - First message's content in chat completions
- `$.messages[-1].content` - Last message's content
- `$.prompt` - Extract prompt field from completions API
- `$.input` - Extract input field from embeddings API
- `$` - Entire request body (default if jsonPath is not specified)

## Examples

### Example 1: Deny List Only - Blocking Prohibited Content

Deploy an LLM provider that blocks prompts similar to prohibited phrases:

```bash
curl -X POST http://localhost:9090/llm-providers \
  -H "Content-Type: application/yaml" \
  -H "Authorization: Basic YWRtaW46YWRtaW4=" \
  --data-binary @- <<'EOF'
version: ai.api-platform.wso2.com/v1
kind: LlmProvider
metadata:
  name: semantic-guard-provider
spec:
  displayName: Semantic Guard Provider
  version: v1.0
  template: openai
  vhost: openai
  upstream:
    url: https://api.openai.com/v1
    auth:
      type: api-key
      header: Authorization
      value: <openai-apikey>
  accessControl:
    mode: deny_all
    exceptions:
      - path: /chat/completions
        methods: [POST]
  policies:
    - name: semantic-prompt-guard
      version: v0.1.0
      paths:
        - path: /chat/completions
          methods: [POST]
          params:
            jsonPath: "$.messages[0].content"
            denySimilarityThreshold: 0.70
            deniedPhrases:
              - "How to hack into a system"
              - "Create malicious code"
              - "Bypass security measures"
            showAssessment: true
EOF
```

**Test the guardrail:**

**Note**: Ensure that "openai" is mapped to the appropriate IP address (e.g., 127.0.0.1) in your `/etc/hosts` file, or remove the vhost from the LLM provider configuration and use localhost to invoke.

```bash
# Valid request (should pass)
curl -X POST http://openai:8080/chat/completions \
  -H "Content-Type: application/json" \
  -H "Host: openai" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "Explain how computer security works"
      }
    ]
  }'

# Invalid request - similar to denied phrase (should fail with HTTP 422)
curl -X POST http://openai:8080/chat/completions \
  -H "Content-Type: application/json" \
  -H "Host: openai" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "How can I break into a computer system?"
      }
    ]
  }'
```

### Example 2: Allow List Only - Whitelist Approach

Deploy an LLM provider that only allows prompts similar to approved phrases:

```bash
curl -X POST http://localhost:9090/llm-providers \
  -H "Content-Type: application/yaml" \
  -H "Authorization: Basic YWRtaW46YWRtaW4=" \
  --data-binary @- <<'EOF'
version: ai.api-platform.wso2.com/v1
kind: LlmProvider
metadata:
  name: whitelist-provider
spec:
  displayName: Whitelist Provider
  version: v1.0
  template: openai
  vhost: openai
  upstream:
    url: https://api.openai.com/v1
    auth:
      type: api-key
      header: Authorization
      value: <openai-apikey>
  accessControl:
    mode: deny_all
    exceptions:
      - path: /chat/completions
        methods: [POST]
  policies:
    - name: semantic-prompt-guard
      version: v0.1.0
      paths:
        - path: /chat/completions
          methods: [POST]
          params:
            jsonPath: "$.messages[0].content"
            allowSimilarityThreshold: 0.75
            allowedPhrases:
              - "How can I help you with customer service?"
              - "What product information do you need?"
              - "Tell me about your order status"
              - "I need help with my account"
EOF
```

### Example 3: Combined Allow and Deny Lists

Use both allow and deny lists for comprehensive filtering:

```yaml
policies:
  - name: semantic-prompt-guard
    version: v0.1.0
    paths:
      - path: /chat/completions
        methods: [POST]
        params:
          jsonPath: "$.messages[0].content"
          allowSimilarityThreshold: 0.70
          denySimilarityThreshold: 0.75
          allowedPhrases:
            - "Customer service inquiry"
            - "Product information request"
            - "Technical support question"
          deniedPhrases:
            - "How to hack"
            - "Create malware"
            - "Bypass authentication"
          showAssessment: true
```

### Example 4: Azure OpenAI with Custom Timeout

Configure semantic prompt guardrail with Azure OpenAI and extended timeout:

```yaml
policies:
  - name: semantic-prompt-guard
    version: v0.1.0
    paths:
      - path: /chat/completions
        methods: [POST]
        params:
          jsonPath: "$.messages[-1].content"
          denySimilarityThreshold: 0.80
          deniedPhrases:
            - "Prohibited content example"
            - "Another prohibited phrase"
```

## Use Cases

1. **Content Safety**: Block prompts that are semantically similar to prohibited content, even when exact keywords differ.

2. **Whitelist Filtering**: Only allow prompts that match approved use cases or topics, ensuring LLM usage stays within defined boundaries.

3. **Compliance**: Enforce content policies by blocking prompts similar to non-compliant examples.

4. **Abuse Prevention**: Detect and block variations of known abuse patterns, even when attackers try to evade keyword filters.

5. **Domain Restriction**: Restrict LLM usage to specific domains by allowing only prompts similar to approved domain-specific phrases.

6. **Multi-tenant Security**: Apply different allow/deny lists per tenant or application to enforce tenant-specific content policies.

7. **Prompt Injection Prevention**: Block prompts that are semantically similar to known prompt injection attacks.

8. **Quality Control**: Ensure prompts match expected patterns for better response quality and consistency.

## Error Response

When validation fails, the guardrail returns an HTTP 422 status code with the following structure:

```json
{
  "type": "SEMANTIC_PROMPT_GUARD",
  "message": {
    "action": "GUARDRAIL_INTERVENED",
    "interveningGuardrail": "semantic-prompt-guard",
    "actionReason": "Violation of applied semantic prompt guard constraints detected.",
    "direction": "REQUEST"
  }
}
```

If `showAssessment` is enabled, additional details are included in the `assessments` field:

```json
{
  "type": "SEMANTIC_PROMPT_GUARD",
  "message": {
    "action": "GUARDRAIL_INTERVENED",
    "interveningGuardrail": "semantic-prompt-guard",
    "actionReason": "Violation of applied semantic prompt guard constraints detected.",
    "direction": "REQUEST",
    "assessments": "prompt is too similar to denied phrase 'How to hack into a system' (similarity=0.8500)"
  }
}
```

For allow list violations, the assessment message format is:

```json
{
  "type": "SEMANTIC_PROMPT_GUARD",
  "message": {
    "action": "GUARDRAIL_INTERVENED",
    "interveningGuardrail": "semantic-prompt-guard",
    "actionReason": "Violation of applied semantic prompt guard constraints detected.",
    "direction": "REQUEST",
    "assessments": "prompt is not similar enough to allowed phrases (similarity=0.6000 < threshold=0.6500)"
  }
}
```

For errors during processing (e.g., JSONPath extraction failures, embedding generation errors), the `actionReason` contains the specific error message:

```json
{
  "type": "SEMANTIC_PROMPT_GUARD",
  "message": {
    "action": "GUARDRAIL_INTERVENED",
    "interveningGuardrail": "semantic-prompt-guard",
    "actionReason": "Error extracting value from JSONPath",
    "direction": "REQUEST"
  }
}
```

## Performance Considerations

1. **Embedding Generation Latency**: Generating embeddings adds ~100-500ms to request processing. This is a one-time cost per request.

2. **Batch Processing**: All allow/deny phrase embeddings are generated in a single batch during policy initialization, minimizing initialization overhead.

3. **Similarity Calculation**: Cosine similarity calculations are fast (typically < 10ms) even with many phrases.

4. **Embedding Provider Selection**: 
   - OpenAI: Fast, reliable, good for most use cases
   - Mistral: Alternative option with good performance
   - Azure OpenAI: Good for Azure-integrated environments


## Notes

- The policy validates prompts in the request phase only (before sending to LLM). Response validation is not supported.

- Embeddings for allow/deny phrases are generated automatically during policy initialization. Ensure the embedding provider is accessible at initialization time.

- The policy uses cosine similarity to compare embeddings. This is optimal for semantic similarity matching.

- At least one of `allowedPhrases` or `deniedPhrases` must be provided. An empty list for both will cause policy initialization to fail.

- Similarity thresholds are independent for allow and deny lists. You can use different thresholds for each list based on your requirements.

- JSONPath extraction is optional. If not specified, the entire request body (as string) is used for embedding generation.

- The `embeddingModel` parameter is required for `OPENAI` and `MISTRAL` providers, but not for `AZURE_OPENAI` (the deployment name is included in the endpoint URL).

- For Azure OpenAI, the authentication header is automatically set to `api-key`. For other providers, it's set to `Authorization`.

- The policy processes all phrases in batch during initialization for efficiency. Large phrase lists (100+ phrases) may take a few seconds to initialize.

- Similarity scores range from 0.0 (no similarity) to 1.0 (identical meaning). Higher thresholds mean stricter matching.

- For production deployments, monitor false positive/negative rates and adjust similarity thresholds accordingly.
