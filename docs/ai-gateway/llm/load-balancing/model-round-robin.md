# Model Round Robin

## Overview

The Model Round Robin policy implements round-robin load balancing for AI models. It distributes requests evenly across multiple configured AI models in a cyclic manner, ensuring equal request allocation over time and preventing overloading of any single model. This policy is useful for distributing load across multiple models, improving availability, and managing resource utilization.

## Features

- Even distribution of requests across multiple models in a cyclic pattern
- Automatic model suspension on failures (5xx or 429 responses)
- Configurable suspension duration for failed models
- Support for extracting model identifier from payload, headers, query parameters, or path parameters
- Dynamic model selection based on availability

## Configuration

### Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `models` | array | Yes | - | List of models for round-robin distribution. Each model must have a `model` name. |
| `suspendDuration` | integer | No | `0` | Suspend duration in seconds for failed models. If set to 0, failed model knowledge is not persisted. Must be >= 0. |

### Model Configuration

Each model in the `models` array is an object with the following properties:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `model` | string | Yes | The AI model name to use for load balancing. |

### LLM provider template

The policy requires `requestModel` configuration from the LLM provider template to extract the model identifier from the request. This configuration is mandatory and must be provided by the LLM provider template.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `requestModel.location` | string | Yes | Location of the model identifier: `payload`, `header`, `queryParam`, or `pathParam` |
| `requestModel.identifier` | string | Yes | JSONPath (for payload), header name (for header), query param name (for queryParam), or regex pattern (for pathParam) to extract model |

## How It Works

1. **Model Selection**: On each request, the policy selects the next available model in the configured list using a round-robin algorithm.
2. **Model Extraction**: The policy extracts the original model from the request (if configured) and stores it for reference.
3. **Model Modification**: The policy modifies the request to use the selected model based on the `requestModel` configuration.
4. **Failure Handling**: If a model returns a 5xx or 429 response, and `suspendDuration` is configured, the model is suspended for the specified duration.
5. **Availability Check**: Suspended models are skipped during selection until their suspension period expires.

## Examples

### Example 1: Basic Round Robin with Payload-based Model

Deploy an LLM provider with round-robin load balancing across multiple models:

```bash
curl -X POST http://localhost:9090/llm-providers \
  -H "Content-Type: application/yaml" \
  -H "Authorization: Basic YWRtaW46YWRtaW4=" \
  --data-binary @- <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: round-robin-provider
spec:
  displayName: Round Robin Provider
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
    - name: model-round-robin
      version: v0.1.0
      paths:
        - path: /chat/completions
          methods: [POST]
          params:
            models:
              - model: gpt-4
              - model: gpt-3.5-turbo
              - model: gpt-4-turbo
            suspendDuration: 60
EOF
```

**Test the round-robin distribution:**

**Note**: Ensure that "openai" is mapped to the appropriate IP address (e.g., 127.0.0.1) in your `/etc/hosts` file, or remove the vhost from the LLM provider configuration and use localhost to invoke.

```bash
# First request - will use gpt-4
curl -X POST http://openai:8080/chat/completions \
  -H "Content-Type: application/json" \
  -H "Host: openai" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "Hello"
      }
    ]
  }'

# Second request - will use gpt-3.5-turbo
curl -X POST http://openai:8080/chat/completions \
  -H "Content-Type: application/json" \
  -H "Host: openai" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "Hello"
      }
    ]
  }'

# Third request - will use gpt-4-turbo
curl -X POST http://openai:8080/chat/completions \
  -H "Content-Type: application/json" \
  -H "Host: openai" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "Hello"
      }
    ]
  }'
```

## Model Suspension

When a model returns a 5xx or 429 response, the policy can automatically suspend that model for a configurable duration:

- **Suspension Duration**: Configured via the `suspendDuration` parameter (in seconds)
- **Automatic Recovery**: Suspended models are automatically re-enabled after the suspension period expires
- **Availability Check**: Suspended models are skipped during round-robin selection until they recover

### Suspension Behavior

- Suspension is tracked per model across all requests
- If all models are suspended, the policy returns HTTP 503 with error: "All models are currently unavailable"
- Suspension period starts from the time of failure

## Use Cases

1. **Load Distribution**: Distribute requests evenly across multiple models to prevent overloading any single model.

2. **High Availability**: Automatically route requests to available models when some models are experiencing issues.

3. **Cost Optimization**: Distribute requests across different model tiers (e.g., expensive and cheaper models) to balance cost and performance.

4. **A/B Testing**: Test different models with equal traffic distribution to compare performance and quality.

5. **Multi-Provider Support**: Distribute requests across models from different providers while maintaining equal distribution.

## Request Model Locations

The policy supports extracting the model identifier from different locations in the request:

### Payload (JSONPath)

Extract model from JSON payload using JSONPath:

- **Location**: `payload`
- **Identifier**: JSONPath expression (e.g., `$.model`, `$.messages[0].model`)

### Header

Extract model from HTTP header:

- **Location**: `header`
- **Identifier**: Header name (e.g., `X-Model-Name`, `X-LLM-Model`)

### Query Parameter

Extract model from URL query parameter:

- **Location**: `queryParam`
- **Identifier**: Query parameter name (e.g., `model`, `llm_model`)

### Path Parameter

Extract model from URL path using regex:

- **Location**: `pathParam`
- **Identifier**: Regex pattern to match model in path (e.g., `models/([a-zA-Z0-9.\-]+)`)

**Note**: For path parameters, the regex pattern should include a capturing group to extract the model name. The policy uses the first capturing group as the model identifier.

## Notes

- The round-robin index is maintained per policy instance and increments for each request.
- Model selection is deterministic and follows a strict cyclic pattern.
- The original model from the request is stored in metadata but is replaced with the selected model for routing.
- If `suspendDuration` is 0, failed models are not suspended and will continue to be selected in the round-robin cycle.
- The `requestModel` configuration is required and must be provided by the LLM provider template.
