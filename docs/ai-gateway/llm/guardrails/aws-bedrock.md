# AWS Bedrock Guardrail

## Overview

The AWS Bedrock Guardrail validates request or response body content against AWS Bedrock Guardrails, providing enterprise-grade content filtering, topic detection, word filtering, and PII detection with masking capabilities. This guardrail integrates with AWS Bedrock's comprehensive guardrail service to enforce content policies.

## Features

- Content policy filtering (hate, violence, sexual content, etc.)
- Topic policy detection and blocking
- Word policy filtering (custom and managed word lists)
- PII detection and masking/redaction
- Multiple authentication modes (role-based, static credentials, default credential chain)
- Supports JSONPath extraction to validate specific fields within JSON payloads
- Separate configuration for request and response phases
- Detailed assessment information from AWS Bedrock Guardrail

## Configuration

### Top-Level Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `region` | string | Yes | - | AWS region where the Bedrock Guardrail is located (e.g., "us-east-1", "ap-southeast-2"). |
| `guardrailID` | string | Yes | - | AWS Bedrock Guardrail identifier. |
| `guardrailVersion` | string | Yes | - | AWS Bedrock Guardrail version. Use "DRAFT" for draft guardrails or a numeric version (e.g., "1") for published guardrails. |
| `awsAccessKeyID` | string | No | - | AWS access key ID for static credentials or role assumption. |
| `awsSecretAccessKey` | string | No | - | AWS secret access key for static credentials or role assumption. |
| `awsSessionToken` | string | No | - | AWS session token for temporary credentials. |
| `awsRoleARN` | string | No | - | AWS IAM role ARN to assume for role-based authentication. |
| `awsRoleRegion` | string | No | - | AWS region for role assumption. Required if `awsRoleARN` is specified. |
| `awsRoleExternalID` | string | No | - | External ID for role assumption (optional). |

### Request Phase Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `jsonPath` | string | No | `""` | JSONPath expression to extract a specific value from JSON payload. If empty, validates the entire payload as a string. |
| `redactPII` | boolean | No | `false` | If `true`, redacts PII by replacing with "*****" (permanent, cannot be restored). If `false`, masks PII with placeholders that can be restored in responses. |
| `passthroughOnError` | boolean | No | `false` | If `true`, allows requests to proceed if AWS Bedrock Guardrail API call fails. If `false`, blocks requests on API errors. |
| `showAssessment` | boolean | No | `false` | If `true`, includes detailed assessment information from AWS Bedrock Guardrail in error responses. |

### Response Phase Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `jsonPath` | string | No | `""` | JSONPath expression to extract a specific value from JSON payload. If empty, validates the entire payload as a string. |
| `redactPII` | boolean | No | `false` | If `true`, redacts PII by replacing with "*****" (permanent, cannot be restored). If `false`, restores masked PII from request phase. |
| `passthroughOnError` | boolean | No | `false` | If `true`, allows requests to proceed if AWS Bedrock Guardrail API call fails. If `false`, blocks requests on API errors. |
| `showAssessment` | boolean | No | `false` | If `true`, includes detailed assessment information from AWS Bedrock Guardrail in error responses. |

## Authentication Modes

### 1. Default Credential Chain

Uses AWS SDK default credential chain (environment variables, IAM roles, credentials file):

```yaml
params:
  region: "us-east-1"
  guardrailID: "your-guardrail-id"
  guardrailVersion: "1"
  request:
    jsonPath: "$.messages[0].content"
```

### 2. Static Credentials

Uses provided access key and secret:

```yaml
params:
  region: "us-east-1"
  guardrailID: "your-guardrail-id"
  guardrailVersion: "1"
  awsAccessKeyID: "AKIAIOSFODNN7EXAMPLE"
  awsSecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
  request:
    jsonPath: "$.messages[0].content"
```

### 3. Role-Based Authentication (AssumeRole)

Assumes an IAM role for authentication:

```yaml
params:
  region: "us-east-1"
  guardrailID: "your-guardrail-id"
  guardrailVersion: "1"
  awsRoleARN: "arn:aws:iam::123456789012:role/BedrockGuardrailRole"
  awsRoleRegion: "us-east-1"
  request:
    jsonPath: "$.messages[0].content"
```

## JSONPath Support

The guardrail supports JSONPath expressions to extract and validate specific fields within JSON payloads. Common examples:

- `$.message` - Extracts the `message` field from the root object
- `$.data.content` - Extracts nested content from `data.content`
- `$.items[0].text` - Extracts text from the first item in an array
- `$.messages[0].content` - Extracts content from the first message in a messages array

If `jsonPath` is empty or not specified, the entire payload is treated as a string and validated.

## PII Masking Modes

### Masking Mode (`redactPII: false`)

- PII is replaced with placeholders (e.g., `EMAIL_0001`, `PHONE_0002`)
- Placeholders can be restored in responses
- Original PII values are stored temporarily for restoration
- Recommended when you need to preserve data for downstream processing

### Redaction Mode (`redactPII: true`)

- PII is permanently replaced with "*****"
- Cannot be restored in responses
- More secure but loses original data
- Recommended for maximum privacy protection

## Examples

### Example 1: Basic Configuration with Default Credentials

```yaml
policies:
  - name: AWSBedrockGuardrail
    version: v0.1.0
    enabled: true
    params:
      region: "us-east-1"
      guardrailID: "zs3gmghtidsa"
      guardrailVersion: "1"
      request:
        jsonPath: "$.messages[0].content"
        redactPII: false
        passthroughOnError: false
        showAssessment: true
      response:
        jsonPath: "$.choices[0].message.content"
        redactPII: false
        showAssessment: true
```

### Example 2: Static Credentials with PII Redaction

```yaml
policies:
  - name: AWSBedrockGuardrail
    version: v0.1.0
    enabled: true
    params:
      region: "ap-southeast-2"
      guardrailID: "your-guardrail-id"
      guardrailVersion: "DRAFT"
      awsAccessKeyID: "AKIAIOSFODNN7EXAMPLE"
      awsSecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
      request:
        jsonPath: "$.messages[0].content"
        redactPII: true
        showAssessment: false
```

### Example 3: Role-Based Authentication

```yaml
policies:
  - name: AWSBedrockGuardrail
    version: v0.1.0
    enabled: true
    params:
      region: "us-east-1"
      guardrailID: "your-guardrail-id"
      guardrailVersion: "1"
      awsRoleARN: "arn:aws:iam::123456789012:role/BedrockGuardrailRole"
      awsRoleRegion: "us-east-1"
      awsRoleExternalID: "external-id-123"
      request:
        jsonPath: "$.messages[0].content"
        passthroughOnError: true
        showAssessment: true
```

### Example 4: Error Passthrough Configuration

Allow requests to proceed if AWS API fails:

```yaml
policies:
  - name: AWSBedrockGuardrail
    version: v0.1.0
    enabled: true
    params:
      region: "us-east-1"
      guardrailID: "your-guardrail-id"
      guardrailVersion: "1"
      request:
        jsonPath: "$.messages[0].content"
        passthroughOnError: true
```

## Use Cases

1. **Content Moderation**: Filter harmful, violent, or inappropriate content using AWS Bedrock's content policies.

2. **Topic Control**: Block or allow content based on specific topics using topic policies.

3. **Word Filtering**: Enforce custom word lists or managed word lists to prevent specific terminology.

4. **PII Protection**: Detect and mask/redact personally identifiable information to protect user privacy.

5. **Compliance**: Meet regulatory requirements for content safety and data protection.

## Error Response

When validation fails, the guardrail returns an HTTP 446 status code. There are two types of errors:

### Guardrail Intervention (Content Blocked)

```json
{
    "code": 900514,
    "message": {
        "action": "GUARDRAIL_INTERVENED",
        "actionReason": "Violation of AWS Bedrock Guardrail detected.",
        "assessments": {
            "contentPolicy": {
                "filters": [
                    {
                        "action": "BLOCKED",
                        "confidence": "HIGH",
                        "type": "PROMPT_ATTACK"
                    },
                    {
                        "action": "BLOCKED",
                        "confidence": "HIGH",
                        "type": "MISCONDUCT"
                    }
                ]
            }
        },
        "direction": "REQUEST",
        "interveningGuardrail": "AWSBedrockGuardrail"
    },
    "type": "AWS_BEDROCK_GUARDRAIL"
}
```

### API Failure (Guardrail Call Failed)

```json
{
    "code": 900514,
    "message": {
        "action": "GUARDRAIL_INTERVENED",
        "actionReason": "Error calling AWS Bedrock Guardrail",
        "assessments": [
            "ApplyGuardrail API call failed: operation error Bedrock Runtime: ApplyGuardrail, https response error StatusCode: 400, RequestID: 9e570f97-6c04-42fc-a9e2-47e6c9cba0c2, ValidationException: Guardrail was enabled but input is in incorrect format."
        ],
        "direction": "REQUEST",
        "interveningGuardrail": "AWS Bedrock Guardrail"
    },
    "type": "AWS_BEDROCK_GUARDRAIL"
}
```

## Assessment Details

When `showAssessment: true`, the error response includes detailed information from AWS Bedrock Guardrail:

- **Content Policy**: Filters for hate, violence, sexual content, etc. with confidence levels
- **Topic Policy**: Detected topics and their actions
- **Word Policy**: Custom words and managed word lists that matched
- **Sensitive Information Policy**: PII entities and regex patterns detected

## Notes

- Ensure the guardrail ID and version exist in your AWS Bedrock console before use.
- Guardrail versions can be "DRAFT" for draft guardrails or numeric (e.g., "1", "2") for published guardrails.
- PII masking preserves data for restoration when `redactPII: false`; redaction is permanent when `redactPII: true`.
- The guardrail has a 30-second timeout for AWS API calls to prevent hanging requests.
- Use `passthroughOnError: true` carefully, as it may allow unsafe content through if the guardrail service is unavailable.
- Role-based authentication is recommended for production environments for better security and credential management.
