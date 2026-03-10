---
title: "Overview"
---
# Prompt Compression

## Overview

The Prompt Compression policy automatically compresses LLM prompts before sending them to upstream providers. Using statistical analysis, it identifies and removes lower-importance words while preserving semantic meaning, critical terms, code blocks, and structured data. This reduces token usage, lowers costs, and decreases latency without degrading response quality.

This is a **Python-based policy** that runs in the Python executor runtime.

## Features

- **Statistical compression**: Uses the `compression-prompt` library for intelligent text compression
- **Configurable compression ratio**: Adjust balance between size reduction and quality retention
- **Content protection**: Automatically preserves code blocks, JSON structures, and technical identifiers
- **Domain term preservation**: Configure specific terms that must never be removed
- **Minimum input threshold**: Avoid over-compressing short prompts
- **JSONPath support**: Target specific fields in request payloads

## How It Works

The policy uses the [compression-prompt](https://pypi.org/project/compression-prompt/) library which implements:

1. **Word importance scoring**: Each word is scored based on rarity, position, grammatical role, and contextual significance
2. **Intelligent filtering**: Lower importance words (filler language, redundant phrases) are removed based on target ratio
3. **Semantic preservation**: Critical terms, technical identifiers, structured content, and logical connections are maintained
4. **Multilingual support**: Works with 10+ languages including English, Spanish, Portuguese, French, German, Italian, Russian, Chinese, Japanese, Arabic, and Hindi

## Configuration

### User Parameters (API Definition)

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `compressionRatio` | number | No | `0.5` | Target compression ratio (0.1 to 0.9). 0.5 means ~50% size reduction. Lower = more aggressive. |
| `jsonPath` | string | No | `$.messages[-1].content` | JSONPath to the text field to compress. See examples below. |
| `minInputTokens` | integer | No | `100` | Minimum input size to trigger compression (shorter inputs pass through). Roughly 4 chars = 1 token. |
| `preserveCodeBlocks` | boolean | No | `true` | Protect code blocks (```...```) from compression. |
| `preserveJson` | boolean | No | `true` | Protect JSON structures from compression. |
| `domainTerms` | string[] | No | `[]` | List of domain-specific terms to always preserve. |

### JSONPath Examples

The `jsonPath` parameter supports the following patterns:

| Pattern | Description |
|---------|-------------|
| `$.messages[-1].content` | Compress the last message's content (default) |
| `$.messages[0].content` | Compress the first message's content |
| `$.messages[*].content` | Compress all message contents individually |
| `$.prompt` | Compress a simple prompt field |

## Reference Scenarios

### Example 1: Basic Compression

Compress user prompts to reduce token usage by ~40%:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: compressed-provider
spec:
  displayName: Compressed Provider
  version: v1.0
  template: openai
  vhost: openai
  upstream:
    url: "https://api.openai.com/v1"
    auth:
      type: api-key
      header: Authorization
      value: Bearer <openai-apikey>
  accessControl:
    mode: deny_all
    exceptions:
      - path: /chat/completions
        methods: [POST]
  policies:
    - name: prompt-compression
      version: v0
      paths:
        - path: /chat/completions
          methods: [POST]
          params:
            compressionRatio: 0.6
            jsonPath: "$.messages[-1].content"
```

**Test the compression:**

```bash
# Request with a verbose prompt
curl -X POST http://openai:8080/chat/completions \
  -H "Content-Type: application/json" \
  -H "Host: openai" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "I am writing to you today to ask for your assistance with a very important matter that has come to my attention recently regarding the implementation of various software solutions in our enterprise environment."
      }
    ]
  }'

# The prompt will be compressed to something like:
# "writing today assistance important matter recently implementation software solutions enterprise environment"
```

### Example 2: Preserve Domain Terms

Compress but preserve specific technical terms:

```yaml
policies:
  - name: prompt-compression
    version: v0
    paths:
      - path: /chat/completions
        methods: [POST]
        params:
          compressionRatio: 0.5
          jsonPath: "$.messages[-1].content"
          domainTerms:
            - "GraphQL"
            - "REST"
            - "OAuth"
            - "JWT"
            - "Microservices"
```

### Example 3: Compress All Messages

Compress the entire conversation history:

```yaml
policies:
  - name: prompt-compression
    version: v0
    paths:
      - path: /chat/completions
        methods: [POST]
        params:
          compressionRatio: 0.5
          jsonPath: "$.messages[*].content"
          minInputTokens: 200
```

### Example 4: Aggressive Compression

For cost-sensitive scenarios with longer prompts:

```yaml
policies:
  - name: prompt-compression
    version: v0
    paths:
      - path: /chat/completions
        methods: [POST]
        params:
          compressionRatio: 0.3
          jsonPath: "$.messages[-1].content"
          minInputTokens: 500
          preserveCodeBlocks: true
          preserveJson: true
```

**Note:** Compression ratios below 0.3 may significantly impact quality. Monitor your use case carefully.

## Compression Behavior

### What Gets Preserved

The policy automatically protects:

- **Code blocks**: Content within triple backticks (```)
- **JSON structures**: Objects and arrays with proper syntax
- **Technical identifiers**: CamelCase, snake_case, UPPER_SNAKE_CASE
- **Paths and URLs**: File paths and web URLs
- **Hashes and numbers**: Hex strings and large numbers
- **Negations**: "not", "no", "never", "don't", etc.
- **Comparators**: `!=`, `<=`, `>=`, `<`, `>`, `==`, etc.
- **Domain terms**: User-specified critical terms

### What Gets Removed

The compression removes:

- **Stop words**: "the", "a", "an", "and", "in", "on", etc. (context-aware)
- **Filler phrases**: Redundant expressions and hedging language
- **Low-importance words**: Words that don't contribute significantly to meaning

### Edge Cases

1. **Short inputs**: Prompts shorter than `minInputTokens` pass through unchanged
2. **Negative compression**: If compression would increase size, original is kept
3. **Empty/invalid content**: Passes through without modification
4. **Invalid JSON**: Returns error response

## Error Response

When the policy encounters an error, it returns an HTTP 500 with:

```json
{
  "error": {
    "type": "PROMPT_COMPRESSION_ERROR",
    "message": "Prompt compression failed",
    "details": "Detailed error description"
  }
}
```

## Dependencies

This policy requires the following Python packages (automatically installed):

```
compression-prompt>=0.1.2
```

## Performance Considerations

- **Processing time**: Compression adds ~10-50ms depending on prompt length
- **Memory usage**: Minimal overhead, processes text in-memory
- **Best for**: Long prompts (>100 tokens), RAG contexts, conversation histories
- **Not recommended for**: Very short prompts, code-heavy prompts, structured data

## Notes

- The policy only processes request bodies (response phase is not supported)
- Compression is lossy - some semantic information may be reduced
- Monitor your application's performance with different compression ratios
- Use `domainTerms` to protect industry-specific terminology
- The compression-prompt library estimates tokens as `len(text) // 4` (rough approximation)
