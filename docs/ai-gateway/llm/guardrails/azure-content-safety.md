# Azure Content Safety Content Moderation Guardrail

## Overview

The Azure Content Safety Content Moderation Guardrail validates request or response body content against Azure Content Safety API for content moderation. This guardrail provides enterprise-grade content filtering to detect and block harmful content including hate speech, sexual content, self-harm, and violence.

## Features

- Multi-category content detection (hate, sexual, self-harm, violence)
- Configurable severity thresholds for each category
- Supports JSONPath extraction to validate specific fields within JSON payloads
- Error passthrough option for graceful degradation
- Separate configuration for request and response phases
- Detailed assessment information in error responses

## Configuration

### Request Phase Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `azureContentSafetyEndpoint` | string | Yes | - | Azure Content Safety API endpoint URL (without trailing slash). Example: "https://your-resource.cognitiveservices.azure.com". |
| `azureContentSafetyKey` | string | Yes | - | Azure Content Safety API subscription key. |
| `hateCategory` | integer | No | `-1` | Severity threshold for hate category (0-7). -1 disables this category. Content with severity >= threshold will be blocked. |
| `sexualCategory` | integer | No | `-1` | Severity threshold for sexual category (0-7). -1 disables this category. Content with severity >= threshold will be blocked. |
| `selfHarmCategory` | integer | No | `-1` | Severity threshold for self-harm category (0-7). -1 disables this category. Content with severity >= threshold will be blocked. |
| `violenceCategory` | integer | No | `-1` | Severity threshold for violence category (0-7). -1 disables this category. Content with severity >= threshold will be blocked. |
| `jsonPath` | string | No | `""` | JSONPath expression to extract a specific value from JSON payload. If empty, validates the entire payload as a string. |
| `passthroughOnError` | boolean | No | `false` | If `true`, allows requests to proceed if Azure Content Safety API call fails. If `false`, blocks requests on API errors. |
| `showAssessment` | boolean | No | `false` | If `true`, includes detailed assessment information in error responses. |

### Response Phase Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `azureContentSafetyEndpoint` | string | Yes | - | Azure Content Safety API endpoint URL (without trailing slash). |
| `azureContentSafetyKey` | string | Yes | - | Azure Content Safety API subscription key. |
| `hateCategory` | integer | No | `-1` | Severity threshold for hate category (0-7). -1 disables this category. |
| `sexualCategory` | integer | No | `-1` | Severity threshold for sexual category (0-7). -1 disables this category. |
| `selfHarmCategory` | integer | No | `-1` | Severity threshold for self-harm category (0-7). -1 disables this category. |
| `violenceCategory` | integer | No | `-1` | Severity threshold for violence category (0-7). -1 disables this category. |
| `jsonPath` | string | No | `""` | JSONPath expression to extract a specific value from JSON payload. If empty, validates the entire payload as a string. |
| `passthroughOnError` | boolean | No | `false` | If `true`, allows requests to proceed if Azure Content Safety API call fails. If `false`, blocks requests on API errors. |
| `showAssessment` | boolean | No | `false` | If `true`, includes detailed assessment information in error responses. |

## Severity Levels

Azure Content Safety uses a severity scale from 0 to 7 for each category:

- **0-2**: Safe content
- **3-4**: Low severity
- **5-6**: Medium severity
- **7**: High severity

Setting a threshold means content with severity **greater than or equal to** that threshold will be blocked. For example:
- `hateCategory: 4` blocks content with hate severity >= 4
- `hateCategory: -1` disables hate category checking entirely

## JSONPath Support

The guardrail supports JSONPath expressions to extract and validate specific fields within JSON payloads. Common examples:

- `$.message` - Extracts the `message` field from the root object
- `$.data.content` - Extracts nested content from `data.content`
- `$.items[0].text` - Extracts text from the first item in an array
- `$.messages[0].content` - Extracts content from the first message in a messages array

If `jsonPath` is empty or not specified, the entire payload is treated as a string and validated.

## Examples

### Example 1: Basic Configuration

Block content with medium to high severity in all categories:

```yaml
policies:
  - name: AzureContentSafetyContentModeration
    version: v1.0.0
    enabled: true
    params:
      request:
        azureContentSafetyEndpoint: "https://your-resource.cognitiveservices.azure.com"
        azureContentSafetyKey: "your-subscription-key"
        hateCategory: 4
        sexualCategory: 4
        selfHarmCategory: 4
        violenceCategory: 4
        jsonPath: "$.messages[0].content"
        showAssessment: true
```

### Example 2: Selective Category Filtering

Only filter hate speech and violence, disable sexual and self-harm:

```yaml
policies:
  - name: AzureContentSafetyContentModeration
    version: v1.0.0
    enabled: true
    params:
      request:
        azureContentSafetyEndpoint: "https://your-resource.cognitiveservices.azure.com"
        azureContentSafetyKey: "your-subscription-key"
        hateCategory: 5
        sexualCategory: -1
        selfHarmCategory: -1
        violenceCategory: 5
        jsonPath: "$.messages[0].content"
```

### Example 3: Strict Filtering

Block even low-severity content:

```yaml
policies:
  - name: AzureContentSafetyContentModeration
    version: v1.0.0
    enabled: true
    params:
      request:
        azureContentSafetyEndpoint: "https://your-resource.cognitiveservices.azure.com"
        azureContentSafetyKey: "your-subscription-key"
        hateCategory: 2
        sexualCategory: 2
        selfHarmCategory: 2
        violenceCategory: 2
        jsonPath: "$.messages[0].content"
        showAssessment: true
```

### Example 4: Response Validation

Validate AI-generated responses for harmful content:

```yaml
policies:
  - name: AzureContentSafetyContentModeration
    version: v1.0.0
    enabled: true
    params:
      response:
        azureContentSafetyEndpoint: "https://your-resource.cognitiveservices.azure.com"
        azureContentSafetyKey: "your-subscription-key"
        hateCategory: 4
        sexualCategory: 4
        selfHarmCategory: 4
        violenceCategory: 4
        jsonPath: "$.choices[0].message.content"
        passthroughOnError: false
        showAssessment: true
```

### Example 5: Error Passthrough

Allow requests to proceed if Azure API fails:

```yaml
policies:
  - name: AzureContentSafetyContentModeration
    version: v1.0.0
    enabled: true
    params:
      request:
        azureContentSafetyEndpoint: "https://your-resource.cognitiveservices.azure.com"
        azureContentSafetyKey: "your-subscription-key"
        hateCategory: 4
        sexualCategory: 4
        selfHarmCategory: 4
        violenceCategory: 4
        jsonPath: "$.messages[0].content"
        passthroughOnError: true
```

## Use Cases

1. **Content Moderation**: Filter harmful, inappropriate, or unsafe content from user inputs and AI responses.

2. **Safety Enforcement**: Protect users from exposure to hate speech, violence, or self-harm content.

3. **Compliance**: Meet regulatory requirements for content safety in AI applications.

4. **User Protection**: Ensure AI-generated content meets safety standards before delivery.

5. **Platform Safety**: Maintain a safe environment for all users by blocking harmful content.

## Error Response

When validation fails, the guardrail returns an HTTP 446 status code with the following structure:

```json
{
    "code": 900514,
    "message": {
        "action": "GUARDRAIL_INTERVENED",
        "actionReason": "Violation of Azure content safety content moderation detected.",
        "direction": "REQUEST",
        "interveningGuardrail": "AzureContentSafetyGuardrail"
    },
    "type": "AZURE_CONTENT_SAFETY_CONTENT_MODERATION"
}
```

If `showAssessment` is enabled, detailed assessment information is included:

```json
{
    "code": 900514,
    "message": {
        "action": "GUARDRAIL_INTERVENED",
        "actionReason": "Violation of Azure content safety content moderation detected.",
        "assessments": {
            "categories": [
                {
                    "category": "Hate",
                    "result": "PASS",
                    "severity": 0,
                    "threshold": 3
                },
                {
                    "category": "Sexual",
                    "result": "PASS",
                    "severity": 0,
                    "threshold": 2
                },
                {
                    "category": "SelfHarm",
                    "result": "PASS",
                    "severity": 0,
                    "threshold": 1
                },
                {
                    "category": "Violence",
                    "result": "FAIL",
                    "severity": 2,
                    "threshold": 1
                }
            ],
            "inspectedContent": "I need to buy guns."
        },
        "direction": "REQUEST",
        "interveningGuardrail": "Azure Content Safety Guardrail"
    },
    "type": "AZURE_CONTENT_SAFETY_CONTENT_MODERATION"
}
```

## Notes

- Ensure your Azure Content Safety resource is properly configured and the endpoint URL is correct.
- The subscription key must have appropriate permissions for the Content Safety API.
- Severity thresholds should be set based on your application's safety requirements and user expectations.
- Lower thresholds (0-2) are more restrictive and may block legitimate content; higher thresholds (5-7) are more permissive.
- Use `passthroughOnError: true` carefully, as it may allow unsafe content through if the Azure service is unavailable.
- The guardrail makes synchronous API calls to Azure; consider latency implications for high-throughput scenarios.
- When using JSONPath, if the path does not exist or the extracted value is not a string, validation will fail.
- Assessment details provide visibility into which categories triggered the block and their severity levels.
