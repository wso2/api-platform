# Azure Content Safety

## Overview

The Azure Content Safety guardrail validates request or response body content against Microsoft Azure Content Safety API for content moderation. It detects and blocks harmful content across four categories: hate speech, sexual content, self-harm, and violence. Each category can be configured with a severity threshold (0-7) or disabled entirely, providing flexible content moderation policies tailored to your application's requirements.

The policy uses Azure Content Safety's text analysis API to evaluate content and blocks requests or responses that exceed configured severity thresholds. This enables enterprise-grade content filtering for LLM applications integrated with Azure services.

## Features

- **Multi-category detection**: Detects hate speech, sexual content, self-harm, and violence
- **Configurable severity thresholds**: Set per-category thresholds (0-7) or disable categories
- **Eight severity levels**: Uses Azure's 8-level severity scale (0=Safe, 7=Most severe)
- **JSONPath support**: Extract and validate specific fields within JSON payloads
- **Separate request/response configuration**: Independent configuration for request and response phases
- **Detailed assessment information**: Optional detailed violation information in error responses
- **Error handling**: Configurable passthrough behavior on API errors
- **Retry logic**: Automatic retry with exponential backoff for transient API failures

## Configuration

### Parameters

#### Request Phase

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `jsonPath` | string | No | `""` | JSONPath expression to extract a specific value from JSON payload. If empty, validates the entire payload as a string. |
| `passthroughOnError` | boolean | No | `false` | If `true`, allows requests to proceed if Azure Content Safety API call fails. If `false`, blocks requests on API errors. |
| `showAssessment` | boolean | No | `false` | If `true`, includes detailed assessment information in error responses. |
| `hateCategory` | integer | No | `-1` | Severity threshold for hate category (0-7). `-1` disables this category. Content with severity >= threshold will be blocked. |
| `sexualCategory` | integer | No | `-1` | Severity threshold for sexual category (0-7). `-1` disables this category. Content with severity >= threshold will be blocked. |
| `selfHarmCategory` | integer | No | `-1` | Severity threshold for self-harm category (0-7). `-1` disables this category. Content with severity >= threshold will be blocked. |
| `violenceCategory` | integer | No | `-1` | Severity threshold for violence category (0-7). `-1` disables this category. Content with severity >= threshold will be blocked. |

#### Response Phase

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `jsonPath` | string | No | `""` | JSONPath expression to extract a specific value from JSON payload. If empty, validates the entire payload as a string. |
| `passthroughOnError` | boolean | No | `false` | If `true`, allows requests to proceed if Azure Content Safety API call fails. If `false`, blocks requests on API errors. |
| `showAssessment` | boolean | No | `false` | If `true`, includes detailed assessment information in error responses. |
| `hateCategory` | integer | No | `-1` | Severity threshold for hate category (0-7). `-1` disables this category. Content with severity >= threshold will be blocked. |
| `sexualCategory` | integer | No | `-1` | Severity threshold for sexual category (0-7). `-1` disables this category. Content with severity >= threshold will be blocked. |
| `selfHarmCategory` | integer | No | `-1` | Severity threshold for self-harm category (0-7). `-1` disables this category. Content with severity >= threshold will be blocked. |
| `violenceCategory` | integer | No | `-1` | Severity threshold for violence category (0-7). `-1` disables this category. Content with severity >= threshold will be blocked. |

### System Parameters (Required)

These parameters are typically configured at the gateway level and automatically injected, or you can override those values from the params section in the api artifact definition file as well:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `azureContentSafetyEndpoint` | string | Yes | Azure Content Safety API endpoint URL (without trailing slash). Example: `https://your-resource.cognitiveservices.azure.com` |
| `azureContentSafetyKey` | string | Yes | Azure Content Safety API subscription key for authentication. Found in Azure Portal under your Content Safety resource's "Keys and Endpoint" section. |

### Configuring System Parameters in config.yaml

System parameters can be configured globally in the gateway's `config.yaml` file. These values serve as defaults for all Azure Content Safety guardrail policy instances and can be overridden per-policy in the API configuration if needed.

#### Location in config.yaml

Add the following configuration section to your `config.yaml` file:

```yaml
azurecontentsafety_endpoint: "https://your-resource.cognitiveservices.azure.com"
azurecontentsafety_key: "<your-azure-content-safety-key>"
```

## Severity Levels

Azure Content Safety uses an 8-level severity scale (0-7):

- **0**: Safe - No harmful content detected
- **1-2**: Low severity - Mildly concerning content
- **3-4**: Medium severity - Moderately concerning content
- **5-6**: High severity - Highly concerning content
- **7**: Maximum severity - Most severe harmful content

**Threshold Configuration**:
- Set a threshold value (0-7) to block content at or above that severity level
- Set to `-1` to disable monitoring for that category
- Example: `hateCategory: 3` blocks content with hate severity >= 3

## JSONPath Support

The guardrail supports JSONPath expressions to extract and validate specific fields within JSON payloads. Common examples:

- `$.message` - Extracts the `message` field from the root object
- `$.data.content` - Extracts nested content from `data.content`
- `$.items[0].text` - Extracts text from the first item in an array
- `$.messages[0].content` - Extracts content from the first message in a messages array
- `$.messages[-1].content` - Extracts content from the last message in a messages array
- `$.choices[0].message.content` - Extracts assistant response content

If `jsonPath` is empty or not specified, the entire payload is treated as a string and validated.

## Examples

### Example 1: Basic Content Moderation

Deploy an LLM provider with Azure Content Safety validation:

```bash
curl -X POST http://localhost:9090/llm-providers \
  -H "Content-Type: application/yaml" \
  -H "Authorization: Basic YWRtaW46YWRtaW4=" \
  --data-binary @- <<'EOF'
version: ai.api-platform.wso2.com/v1
kind: LlmProvider
metadata:
  name: azure-safety-provider
spec:
  displayName: Azure Content Safety Provider
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
    - name: azure-content-safety-content-moderation
      version: v0.1.0
      paths:
        - path: /chat/completions
          methods: [POST]
          params:
            request:
              jsonPath: "$.messages[0].content"
              hateCategory: 2
              sexualCategory: 2
              selfHarmCategory: 2
              violenceCategory: 2
              showAssessment: true
            response:
              jsonPath: "$.choices[0].message.content"
              hateCategory: 2
              sexualCategory: 2
              selfHarmCategory: 2
              violenceCategory: 2
              showAssessment: true
EOF
```

**Test the guardrail:**

**Note**: Ensure that "openai" is mapped to the appropriate IP address (e.g., 127.0.0.1) in your `/etc/hosts` file, or remove the vhost from the LLM provider configuration and use localhost to invoke.

```bash
# Request with harmful content (should fail with HTTP 422)
curl -X POST http://openai:8080/chat/completions \
  -H "Content-Type: application/json" \
  -H "Host: openai" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "Harmful content here"
      }
    ]
  }'

# Valid request (should pass)
curl -X POST http://openai:8080/chat/completions \
  -H "Content-Type: application/json" \
  -H "Host: openai" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "Explain quantum computing in simple terms"
      }
    ]
  }'
```

### Example 2: Strict Moderation with All Categories

Configure strict moderation thresholds:

```yaml
policies:
  - name: azure-content-safety-content-moderation
    version: v0.1.0
    paths:
      - path: /chat/completions
        methods: [POST]
        params:
          request:
            jsonPath: "$.messages[-1].content"
            hateCategory: 1
            sexualCategory: 1
            selfHarmCategory: 1
            violenceCategory: 1
            showAssessment: true
            passthroughOnError: false
          response:
            jsonPath: "$.choices[0].message.content"
            hateCategory: 1
            sexualCategory: 1
            selfHarmCategory: 1
            violenceCategory: 1
            showAssessment: true
```

### Example 3: Selective Category Monitoring

Monitor only specific categories:

```yaml
policies:
  - name: azure-content-safety-content-moderation
    version: v0.1.0
    paths:
      - path: /chat/completions
        methods: [POST]
        params:
          request:
            jsonPath: "$.messages[0].content"
            hateCategory: 3
            sexualCategory: -1  # Disabled
            selfHarmCategory: 2
            violenceCategory: -1  # Disabled
```

### Example 4: Lenient Moderation

Allow more content with higher thresholds:

```yaml
policies:
  - name: azure-content-safety-content-moderation
    version: v0.1.0
    paths:
      - path: /chat/completions
        methods: [POST]
        params:
          request:
            jsonPath: "$.messages[0].content"
            hateCategory: 5
            sexualCategory: 5
            selfHarmCategory: 4
            violenceCategory: 5
            passthroughOnError: true
```

## Use Cases

1. **Content Safety**: Protect users from harmful, offensive, or inappropriate content in LLM interactions.

2. **Regulatory Compliance**: Meet content moderation requirements for regulated industries or geographies.

3. **Brand Safety**: Ensure LLM responses align with brand values and don't generate problematic content.

4. **User Protection**: Prevent exposure to self-harm content, especially important for mental health applications.

5. **Community Guidelines**: Enforce community standards for user-generated content processed through LLMs.

6. **Multi-tenant Applications**: Apply different moderation policies per tenant or application context.

7. **Gradual Rollout**: Start with lenient thresholds and tighten based on actual content patterns.

8. **Audit and Analytics**: Use detailed assessment information to analyze content patterns and refine policies.

## Severity Threshold Guidelines

**Recommended thresholds by use case**:

- **Strict (Family-friendly applications)**: 1-2 across all categories
- **Moderate (General business applications)**: 3-4 across all categories
- **Lenient (Technical/professional contexts)**: 5-6 for most categories, disable non-applicable ones
- **Educational/Research**: 4-5 with selective category monitoring

**Category-specific considerations**:

- **Hate**: Typically set to 2-3 for most applications
- **Sexual**: Set based on application context (1 for family apps, 3-4 for general use)
- **Self-harm**: Often set lower (1-2) due to safety concerns
- **Violence**: Depends on context (1-2 for general use, higher for educational/historical content)

## Error Response

When validation fails, the guardrail returns an HTTP 422 status code with the following structure:

```json
{
  "type": "AZURE_CONTENT_SAFETY_CONTENT_MODERATION",
  "message": {
    "action": "GUARDRAIL_INTERVENED",
    "interveningGuardrail": "azure-content-safety-content-moderation",
    "actionReason": "Violation of Azure content safety content moderation detected.",
    "direction": "REQUEST"
  }
}
```

If `showAssessment` is enabled, additional details are included:

```json
{
  "type": "AZURE_CONTENT_SAFETY_CONTENT_MODERATION",
  "message": {
    "action": "GUARDRAIL_INTERVENED",
    "interveningGuardrail": "azure-content-safety-content-moderation",
    "actionReason": "Violation of Azure content safety content moderation detected.",
    "direction": "REQUEST",
    "assessments": {
      "inspectedContent": "The content that was analyzed",
      "categories": [
        {
          "category": "Hate",
          "severity": 4,
          "result": "FAIL"
        },
        {
          "category": "Violence",
          "severity": 2,
          "result": "FAIL"
        }
      ]
    }
  }
}
```

## API Retry Logic

The policy includes automatic retry logic for transient API failures:

- **Maximum retries**: 5 attempts
- **Retry delay**: 1 second between attempts
- **Timeout**: 30 seconds per request
- Retries are automatic for network errors and HTTP 5xx responses

## Notes

- Azure Content Safety API requires an active Azure subscription and Content Safety resource.
- The API endpoint URL must not include a trailing slash (e.g., `https://resource.cognitiveservices.azure.com`).
- API keys are found in Azure Portal under your Content Safety resource's "Keys and Endpoint" section.
- Category thresholds are independent - you can disable any category by setting it to `-1`.
- Only categories with thresholds >= 0 are sent to the Azure API for analysis (performance optimization).
- JSONPath extraction failures result in error responses unless `passthroughOnError: true`.
- The policy validates both request and response phases independently when both are configured.
- Content is sent to Azure Content Safety API for analysis, so ensure compliance with data residency requirements.
- Rate limits may apply based on your Azure Content Safety subscription tier.
- The API uses Azure's 8-severity-level analysis, providing fine-grained control over content moderation.
- For production deployments, monitor API response times and adjust retry/timeout settings if needed.

