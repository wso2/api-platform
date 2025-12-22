# AWS Bedrock Guardrail

## Overview

The AWS Bedrock Guardrail policy validates request or response body content against AWS Bedrock Guardrails, which provide enterprise-grade content filtering, topic detection, word filtering, and PII (Personally Identifiable Information) detection and masking. This guardrail enables you to enforce content safety policies consistently across your LLM applications using AWS Bedrock's managed guardrail service.

The policy supports multiple authentication modes including AWS IAM role assumption, static credentials, and default credential chain, making it flexible for various AWS deployment scenarios. It can mask or redact PII entities in requests and restore them in responses, ensuring data privacy while maintaining functionality.

## Features

- **Content filtering**: Detects and blocks prohibited content based on guardrail policies
- **Topic detection**: Validates content against configured topic restrictions
- **Word filtering**: Blocks content containing prohibited words or phrases
- **PII detection and masking**: Identifies and masks PII entities (emails, phone numbers, SSNs, etc.)
- **PII restoration**: Restores masked PII in responses when configured (masking mode)
- **PII redaction**: Permanently removes PII by replacing with "*****" (redaction mode)
- **Multiple authentication modes**: Supports role assumption, static credentials, or default AWS credential chain
- **JSONPath support**: Extract and validate specific fields within JSON payloads
- **Separate request/response configuration**: Independent configuration for request and response phases
- **Detailed assessment information**: Optional detailed violation information in error responses

## Configuration

### Parameters

#### Request Phase

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `jsonPath` | string | No | `""` | JSONPath expression to extract a specific value from JSON payload. If empty, validates the entire payload as a string. |
| `redactPII` | boolean | No | `false` | If `true`, redacts PII by replacing with "*****" (permanent). If `false`, masks PII with placeholders that can be restored in responses. |
| `passthroughOnError` | boolean | No | `false` | If `true`, allows requests to proceed if AWS Bedrock Guardrail API call fails. If `false`, blocks requests on API errors. |
| `showAssessment` | boolean | No | `false` | If `true`, includes detailed assessment information from AWS Bedrock Guardrail in error responses. |

#### Response Phase

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `jsonPath` | string | No | `""` | JSONPath expression to extract a specific value from JSON payload. If empty, validates the entire payload as a string. |
| `redactPII` | boolean | No | `false` | If `true`, redacts PII by replacing with "*****" (permanent). If `false`, restores masked PII from request phase. |
| `passthroughOnError` | boolean | No | `false` | If `true`, allows requests to proceed if AWS Bedrock Guardrail API call fails. If `false`, blocks requests on API errors. |
| `showAssessment` | boolean | No | `false` | If `true`, includes detailed assessment information from AWS Bedrock Guardrail in error responses. |

### System Parameters (Required)

These parameters are typically configured at the gateway level and automatically injected, or you can override those values from the params section in the api artifact definition file as well:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `region` | string | Yes | AWS region where the Bedrock Guardrail is located (e.g., "us-east-1", "us-west-2"). |
| `guardrailID` | string | Yes | AWS Bedrock Guardrail identifier (the unique ID of your guardrail). |
| `guardrailVersion` | string | Yes | AWS Bedrock Guardrail version (e.g., "DRAFT", "1", "2"). Use "DRAFT" for testing, numbered versions for production. |
| `awsAccessKeyID` | string | No | AWS access key ID (for static credentials or role assumption). If omitted, runtime uses default AWS credential chain (environment variables, IAM roles, etc.). |
| `awsSecretAccessKey` | string | No | AWS secret access key (for static credentials or role assumption). If omitted, runtime uses default AWS credential chain. |
| `awsSessionToken` | string | No | AWS session token (optional, for temporary credentials). |
| `awsRoleARN` | string | No | AWS IAM role ARN to assume (for role-based authentication). If specified, runtime assumes this role instead of using static credentials. |
| `awsRoleRegion` | string | No | AWS region for role assumption (required if `awsRoleARN` is specified). |
| `awsRoleExternalID` | string | No | External ID for role assumption (optional, for cross-account access security). |


### Configuring System Parameters in config.yaml

System parameters can be configured globally in the gateway's `config.yaml` file. These values serve as defaults for all AWS Bedrock Guardrail policy instances and can be overridden per-policy in the API configuration if needed.

#### Location in config.yaml

Add the following configuration section to your `config.yaml` file:

```yaml
awsbedrock_guardrail_region: "us-east-1" 
awsbedrock_guardrail_id: "your-guardrail-id"
awsbedrock_guardrail_version: "DRAFT"
awsbedrock_access_key_id: ""
awsbedrock_secret_access_key: "" 
awsbedrock_session_token: ""
awsbedrock_role_arn: ""
awsbedrock_role_region: ""
awsbedrock_role_external_id: ""
```

### Authentication Modes

The policy supports three authentication modes, in priority order:

1. **Role Assumption** (if `awsRoleARN` is provided): Assumes the specified IAM role for authentication. Requires `awsRoleRegion` if specified.
2. **Static Credentials** (if `awsAccessKeyID` and `awsSecretAccessKey` are provided): Uses provided AWS credentials directly.
3. **Default Credential Chain** (if neither role nor static credentials are provided): Uses AWS SDK default credential chain:
   - Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
   - IAM instance profile roles (for EC2/ECS)
   - AWS credentials file (`~/.aws/credentials`)
   - IAM roles for service accounts (for EKS)

## JSONPath Support

The guardrail supports JSONPath expressions to extract and validate specific fields within JSON payloads. Common examples:

- `$.message` - Extracts the `message` field from the root object
- `$.data.content` - Extracts nested content from `data.content`
- `$.items[0].text` - Extracts text from the first item in an array
- `$.messages[0].content` - Extracts content from the first message in a messages array
- `$.messages[-1].content` - Extracts content from the last message in a messages array

If `jsonPath` is empty or not specified, the entire payload is treated as a string and validated.

## PII Handling

### Masking Mode (redactPII: false)

When `redactPII` is `false`:
- **Request phase**: PII entities are masked with placeholders like `EMAIL_0001`, `PHONE_0002`, etc.
- **Response phase**: Masked PII is restored to original values automatically
- Use this mode when you need PII to flow through the system but want it masked during processing

### Redaction Mode (redactPII: true)

When `redactPII` is `true`:
- PII entities are permanently replaced with `*****`
- Original values cannot be restored
- Use this mode when you want to completely remove PII from content

## Examples

### Example 1: Basic Guardrail with Static Credentials

Deploy an LLM provider with AWS Bedrock Guardrail validation:

```bash
curl -X POST http://localhost:9090/llm-providers \
  -H "Content-Type: application/yaml" \
  --data-binary @- <<'EOF'
version: ai.api-platform.wso2.com/v1
kind: LlmProvider
metadata:
  name: bedrock-guardrail-provider
spec:
  displayName: AWS Bedrock Guardrail Provider
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
    - name: AWSBedrockGuardrail
      version: v0.1.0
      paths:
        - path: /chat/completions
          methods: [POST]
          params:
            request:
              jsonPath: "$.messages[0].content"
              redactPII: false
              showAssessment: true
            response:
              jsonPath: "$.choices[0].message.content"
              redactPII: false
              showAssessment: true
EOF
```

**Test the guardrail:**

**Note**: Ensure that "openai" is mapped to the appropriate IP address (e.g., 127.0.0.1) in your `/etc/hosts` file, or remove the vhost from the LLM provider configuration and use localhost to invoke.

```bash
# Request with prohibited content (should fail with HTTP 422)
curl -X POST http://openai:8080/chat/completions \
  -H "Content-Type: application/json" \
  -H "Host: openai" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "This is prohibited content"
      }
    ]
  }'

# Request with PII (should mask PII and proceed)
curl -X POST http://openai:8080/chat/completions \
  -H "Content-Type: application/json" \
  -H "Host: openai" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "Contact me at user@example.com or call 555-123-4567"
      }
    ]
  }'
```

### Example 2: Role-Based Authentication

Configure guardrail with IAM role assumption:

```yaml
policies:
  - name: AWSBedrockGuardrail
    version: v0.1.0
    paths:
      - path: /chat/completions
        methods: [POST]
        params:
          request:
            jsonPath: "$.messages[0].content"
            redactPII: true
            passthroughOnError: false
          response:
            jsonPath: "$.choices[0].message.content"
            redactPII: true
```

### Example 3: Default Credential Chain (EC2/ECS)

Use default AWS credential chain (no explicit credentials needed):

```yaml
policies:
  - name: AWSBedrockGuardrail
    version: v0.1.0
    paths:
      - path: /chat/completions
        methods: [POST]
        params:
          request:
            jsonPath: "$.messages[-1].content"
            showAssessment: true
```

### Example 4: PII Masking with Restoration

Configure to mask PII in requests and restore in responses:

```yaml
policies:
  - name: AWSBedrockGuardrail
    version: v0.1.0
    paths:
      - path: /chat/completions
        methods: [POST]
        params:
          request:
            jsonPath: "$.messages[0].content"
            redactPII: false  # Mask mode - uses placeholders
            showAssessment: false
          response:
            jsonPath: "$.choices[0].message.content"
            redactPII: false  # Restore masked PII
```

## Use Cases

1. **Content Safety**: Enforce enterprise content policies to prevent inappropriate or harmful content from being processed or returned.

2. **Compliance**: Meet regulatory requirements (HIPAA, GDPR, etc.) by detecting and masking PII in LLM interactions.

3. **Topic Control**: Restrict LLM usage to approved topics only, preventing misuse or access to sensitive domains.

4. **Data Privacy**: Mask sensitive information during processing while maintaining the ability to restore it in responses when needed.

5. **Prohibited Word Filtering**: Block content containing prohibited words, phrases, or patterns defined in your guardrail.

6. **Multi-tenant Security**: Isolate content policies per tenant or application using different guardrail configurations.

7. **Audit and Monitoring**: Use detailed assessment information to audit content violations and improve policies.

## Error Response

When validation fails, the guardrail returns an HTTP 422 status code with the following structure:

```json
{
  "type": "AWS_BEDROCK_GUARDRAIL",
  "message": {
    "action": "GUARDRAIL_INTERVENED",
    "interveningGuardrail": "AWS Bedrock Guardrail",
    "actionReason": "Violation of AWS Bedrock Guardrail detected.",
    "direction": "REQUEST"
  }
}
```

If `showAssessment` is enabled, additional details are included:

```json
{
  "type": "AWS_BEDROCK_GUARDRAIL",
  "message": {
    "action": "GUARDRAIL_INTERVENED",
    "interveningGuardrail": "AWS Bedrock Guardrail",
    "actionReason": "Violation of AWS Bedrock Guardrail detected.",
    "direction": "REQUEST",
    "assessments": {
      "topicPolicy": {
        "topics": ["Topic1", "Topic2"]
      },
      "contentPolicy": {
        "filters": ["Filter1"]
      },
      "sensitiveInformationPolicy": {
        "piiEntities": [...],
        "regexes": [...]
      }
    }
  }
}
```

## Guardrail Response Actions

The policy processes AWS Bedrock Guardrail responses based on the `Action` field and `ActionReason`:

- **`NONE`**: No violation detected, content passes validation and proceeds normally.

- **`GUARDRAIL_INTERVENED`** with `ActionReason: "Guardrail blocked."`: Content violation detected (content policy, topic policy, or word policy violation). The request/response is blocked with HTTP 422 error.

- **`GUARDRAIL_INTERVENED`** with `ActionReason: "Guardrail masked."`: PII masking was applied. The policy behavior depends on configuration:
  - If `redactPII: false` and this is a request: PII is masked with placeholders (e.g., `EMAIL_0001`, `PHONE_0002`) and stored for restoration in the response phase.
  - If `redactPII: false` and this is a response: Previously masked PII is restored from placeholders to original values.
  - If `redactPII: true`: PII is permanently redacted with `*****` replacement, and the modified content proceeds.

The policy distinguishes between blocking violations and masking operations using the `ActionReason` field to determine the appropriate handling.

## Notes

- The guardrail must be created in AWS Bedrock before use. Use AWS Console, CLI, or SDK to create guardrails with your policies.
- Guardrail version "DRAFT" is useful for testing. Use numbered versions (e.g., "1", "2") for production.
- PII masking with restoration (`redactPII: false`) stores mapping between original and masked values in request metadata, which is used during response processing.
- When using role assumption, ensure the IAM role has `bedrock:InvokeGuardrail` permission.
- The policy uses AWS SDK v2 for authentication and API calls.
- JSONPath extraction failures result in error responses unless `passthroughOnError: true`.
- Content modifications (PII masking) are applied to the payload and forwarded to upstream if no blocking violation occurs.
- The policy validates both request and response phases independently when both are configured.
- Ensure your guardrail is in the specified AWS region; cross-region calls are not supported.

