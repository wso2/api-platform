# Model Weighted Round Robin

## Overview

The Model Weighted Round Robin policy implements weighted round-robin load balancing for AI models. It distributes requests based on predefined weight values assigned to each model, enabling probabilistic control over request distribution and giving higher priority to models with greater processing power or availability. This policy is useful for distributing load proportionally across models based on their capacity, cost, or performance characteristics.

## Features

- Weighted distribution of requests across multiple models based on assigned weights
- Proportional request allocation (models with higher weights receive more requests)
- Automatic model suspension on failures (5xx or 429 responses)
- Configurable suspension duration for failed models
- Support for extracting model identifier from payload, headers, query parameters, or path parameters
- Dynamic model selection based on availability and weights

## Configuration

### Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `models` | array | Yes | - | List of models with weights for weighted round-robin distribution. Each model must have a `model` name and `weight`. The `endpoint` field is optional. |
| `suspendDuration` | integer | No | `0` | Suspend duration in seconds for failed models. If set to 0, failed model knowledge is not persisted. Must be >= 0. |

### Model Configuration

Each model in the `models` array is an object with the following properties:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `model` | string | Yes | The AI model name to use for load balancing. |
| `weight` | integer | Yes | The weight assigned to this model for distribution. Higher weights mean more requests will be routed to this model. Weight is relative to total weight of all models. Must be atleast 1. |
| `endpoint` | string | No | The endpoint URL for the model (optional). Must match pattern `^https?://` if provided. |

### LLM provider template

The policy requires `requestModel` configuration from the LLM provider template to extract the model identifier from the request. This configuration is mandatory and must be provided by the LLM provider template.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `requestModel.location` | string | Yes | Location of the model identifier: `payload`, `header`, `queryParam`, or `pathParam` |
| `requestModel.identifier` | string | Yes | JSONPath (for payload), header name (for header), query param name (for queryParam), or regex pattern (for pathParam) to extract model |

## How It Works

1. **Weight Calculation**: During policy initialization, the policy calculates the total weight of all configured models and builds a weighted sequence where each model appears a number of times proportional to its weight. This sequence is built once and reused for all requests.
2. **Model Selection**: On each request, the policy selects the next available model from the pre-computed weighted sequence using a round-robin algorithm.
3. **Model Extraction**: The policy extracts the original model from the request using the `requestModel` configuration and stores it for reference.
4. **Model Modification**: The policy modifies the request to use the selected model based on the `requestModel` configuration.
5. **Failure Handling**: If a model returns a 5xx or 429 response, and `suspendDuration` is configured, the model is suspended for the specified duration.
6. **Availability Check**: Suspended models are skipped during selection until their suspension period expires.

### Weight Distribution Example

If you configure three models with weights:
- Model A: weight 3
- Model B: weight 2
- Model C: weight 1

The weighted sequence would be: `[A, A, A, B, B, C]`, meaning:
- Model A receives 50% of requests (3 out of 6)
- Model B receives 33.3% of requests (2 out of 6)
- Model C receives 16.7% of requests (1 out of 6)

## Examples

### Example 1: Basic Weighted Round Robin with Payload-based Model

Deploy an LLM provider with weighted round-robin load balancing:

```bash
curl -X POST http://localhost:9090/llm-providers \
  -H "Content-Type: application/yaml" \
  -H "Authorization: Basic YWRtaW46YWRtaW4=" \
  --data-binary @- <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: weighted-round-robin-provider
spec:
  displayName: Weighted Round Robin Provider
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
    - name: model-weighted-round-robin
      version: v0.1.0
      paths:
        - path: /chat/completions
          methods: [POST]
          params:
            models:
              - model: gpt-4
                weight: 3
              - model: gpt-3.5-turbo
                weight: 2
              - model: gpt-4-turbo
                weight: 1
            suspendDuration: 60
EOF
```

**Test the weighted round-robin distribution:**

**Note**: Ensure that "openai" is mapped to the appropriate IP address (e.g., 127.0.0.1) in your `/etc/hosts` file, or remove the vhost from the LLM provider configuration and use localhost to invoke.

```bash
# Requests will be distributed: 50% gpt-4, 33.3% gpt-3.5-turbo, 16.7% gpt-4-turbo
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
- **Availability Check**: Suspended models are skipped during weighted round-robin selection until they recover
- **Weight Preservation**: When a model is suspended, the remaining models continue to be selected based on their relative weights

### Suspension Behavior

- If all models are suspended, the policy returns HTTP 503 with error: "All models are currently unavailable"
- Suspension period starts from the time of failure
- When a model is suspended, the weighted sequence is dynamically adjusted to exclude that model

## Use Cases

1. **Capacity-Based Distribution**: Distribute requests based on model capacity, giving higher weights to models that can handle more load.

2. **Cost Optimization**: Route more requests to cheaper models while maintaining some traffic to premium models for quality assurance.

3. **Performance Tiers**: Prioritize high-performance models for critical requests while using standard models for regular traffic.

4. **Gradual Migration**: Gradually shift traffic from old models to new models by adjusting weights over time.

5. **Multi-Provider Balancing**: Distribute requests across models from different providers with different weights based on SLA or cost agreements.

6. **A/B Testing with Bias**: Test different models with weighted traffic distribution to compare performance while maintaining a bias toward preferred models.

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

## Weight Calculation

The policy builds a weighted sequence by repeating each model a number of times equal to its weight:

- **Total Weight**: Sum of all model weights
- **Sequence Length**: Equal to the total weight
- **Distribution**: Each model appears in the sequence `weight` times
- **Proportional Selection**: Over time, each model receives requests proportional to `model_weight / total_weight`

### Example Weight Distribution

For models with weights [5, 3, 2]:
- Total weight: 10
- Sequence: [Model1, Model1, Model1, Model1, Model1, Model2, Model2, Model2, Model3, Model3]
- Model1: 50% of requests
- Model2: 30% of requests
- Model3: 20% of requests

## Notes

- The weighted sequence is pre-computed once during policy initialization and reused for all requests. It is not rebuilt on each request.
- The round-robin index is maintained per policy instance and increments for each request.
- Model selection follows the weighted sequence in a deterministic cyclic pattern.
- The original model from the request is stored in metadata but is replaced with the selected model for routing.
- If `suspendDuration` is 0, failed models are not suspended and will continue to be selected in the weighted round-robin cycle.
- Higher weights result in more frequent selection but do not guarantee exact proportional distribution in small request volumes.
- The weighted sequence ensures long-term proportional distribution, but short-term distribution may vary due to suspension and availability.
- The `requestModel` configuration is required and must be provided by the LLM provider template. There is no default behavior.
