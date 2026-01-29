# LLM Provider Templates

## Overview

LLM Provider Templates define the characteristics and behaviors specific to an AI service provider, such as OpenAI, Azure OpenAI, Anthropic, or other LLM platforms. These templates describe how the gateway should interpret and extract usage and operational metadata from LLM provider responses, including:

- **Token Usage Metrics**: Prompt tokens, completion tokens, total tokens, and remaining tokens
- **Model Information**: Request and response model identifiers
- **Rate Limiting Data**: Remaining token allowances from response

## Out-of-the-Box Supported Templates

The API Platform Gateway ships with the following pre-configured LLM provider templates that platform administrators can use immediately without any additional configuration:

| Template ID | Display Name | Provider |
|-------------|--------------|----------|
| `openai` | OpenAI | OpenAI Provider |
| `azure-openai` | Azure OpenAI | Microsoft Azure OpenAI Provider |
| `anthropic` | Anthropic | Anthropic Claude Provider |
| `gemini` | Gemini | Google Gemini Provider |
| `mistralai` | MistralAI | Mistral AI Provider |
| `awsbedrock` | AWS Bedrock | Amazon Bedrock Provider |
| `azureai-foundry` | Azure AI Foundry | Microsoft Azure AI Foundry Provider |

These templates are automatically loaded when the gateway starts and are immediately available for use when creating LLM providers.

## Template Structure

Each LLM provider template follows a standard YAML structure:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
metadata:
  name: <template-id>
spec:
  displayName: <Display Name>
  # Provider characteristics
  promptTokens:
    location: <payload|header|pathParam>
    identifier: <extraction-pattern>
  completionTokens:
    location: <payload|header|pathParam>
    identifier: <extraction-pattern>
  totalTokens:
    location: <payload|header|pathParam>
    identifier: <extraction-pattern>
  remainingTokens:
    location: <payload|header|pathParam>
    identifier: <extraction-pattern>
  requestModel:
    location: <payload|header|pathParam>
    identifier: <extraction-pattern>
  responseModel:
    location: <payload|header|pathParam>
    identifier: <extraction-pattern>
```

### Metadata Extraction Patterns

Templates support three types of extraction locations:

- **`payload`**: Extract from JSON response body using JSONPath expressions (e.g., `$.usage.prompt_tokens`)
- **`header`**: Extract from HTTP response headers using header name (e.g., `x-ratelimit-remaining-tokens`)
- **`pathParam`**: Extract from URL path using regular expressions (e.g., `(?<=models/)[a-zA-Z0-9.\-]+`)

## Template Details

### OpenAI

The OpenAI template extracts metadata from OpenAI API responses.

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
metadata:
  name: openai
spec:
  displayName: OpenAI
  promptTokens:
    location: payload
    identifier: $.usage.prompt_tokens
  completionTokens:
    location: payload
    identifier: $.usage.completion_tokens
  totalTokens:
    location: payload
    identifier: $.usage.total_tokens
  remainingTokens:
    location: header
    identifier: x-ratelimit-remaining-tokens
  requestModel:
    location: payload
    identifier: $.model
  responseModel:
    location: payload
    identifier: $.model
```

### Azure OpenAI

The Azure OpenAI template is compatible with Microsoft's Azure OpenAI Service API.

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
metadata:
  name: azure-openai
spec:
  displayName: Azure OpenAI
  promptTokens:
    location: payload
    identifier: $.usage.input_tokens
  completionTokens:
    location: payload
    identifier: $.usage.output_tokens
  totalTokens:
    location: payload
    identifier: $.usage.total_tokens
  remainingTokens:
    location: header
    identifier: x-ratelimit-remaining-tokens
  requestModel:
    location: payload
    identifier: $.model
  responseModel:
    location: payload
    identifier: $.model
```

### Anthropic

The Anthropic template extracts metadata from Anthropic Claude API responses.

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
metadata:
  name: anthropic
spec:
  displayName: Anthropic
  promptTokens:
    location: payload
    identifier: $.usage.input_tokens
  completionTokens:
    location: payload
    identifier: $.usage.output_tokens
  remainingTokens:
    location: header
    identifier: anthropic-ratelimit-tokens-remaining
  requestModel:
    location: payload
    identifier: $.model
  responseModel:
    location: payload
    identifier: $.model
```

### Gemini

The Gemini template is designed for Google's Gemini API.

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
metadata:
  name: gemini
spec:
  displayName: Gemini
  promptTokens:
    location: payload
    identifier: $.usageMetadata.promptTokenCount
  completionTokens:
    location: payload
    identifier: $.usageMetadata.candidatesTokenCount
  totalTokens:
    location: payload
    identifier: $.usageMetadata.totalTokenCount
  remainingTokens:
    location: header
    identifier: x-ratelimit-remaining-tokens
  requestModel:
    location: pathParam
    identifier: (?<=models/)[a-zA-Z0-9.\-]+
  responseModel:
    location: payload
    identifier: $.modelVersion
```

### MistralAI

The MistralAI template supports Mistral AI's API.

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
metadata:
  name: mistralai
spec:
  displayName: MistralAI
  promptTokens:
    location: payload
    identifier: $.usage.prompt_tokens
  completionTokens:
    location: payload
    identifier: $.usage.completion_tokens
  totalTokens:
    location: payload
    identifier: $.usage.total_tokens
  remainingTokens:
    location: header
    identifier: x-ratelimit-remaining-tokens
  requestModel:
    location: payload
    identifier: $.model
  responseModel:
    location: payload
    identifier: $.model
```

### AWS Bedrock

The AWS Bedrock template is designed for Amazon Bedrock's unified API.

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
metadata:
  name: awsbedrock
spec:
  displayName: AWS Bedrock
  promptTokens:
    location: payload
    identifier: $.usage.inputTokens
  completionTokens:
    location: payload
    identifier: $.usage.outputTokens
  totalTokens:
    location: payload
    identifier: $.usage.totalTokens
  requestModel:
    location: pathParam
    identifier: (?<=model/)[a-zA-Z0-9.:-]+(?=/)
  responseModel:
    location: pathParam
    identifier: (?<=model/)[a-zA-Z0-9.:-]+(?=/)
```

### Azure AI Foundry

The Azure AI Foundry template supports Microsoft's Azure AI Foundry platform.

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
metadata:
  name: azureai-foundry
spec:
  displayName: Azure AI Foundry
  promptTokens:
    location: payload
    identifier: $.usage.prompt_tokens
  completionTokens:
    location: payload
    identifier: $.usage.completion_tokens
  totalTokens:
    location: payload
    identifier: $.usage.total_tokens
  remainingTokens:
    location: header
    identifier: x-ratelimit-remaining-tokens
  requestModel:
    location: payload
    identifier: $.model
  responseModel:
    location: payload
    identifier: $.model
```

## Creating an LLM Provider with a Template

To create an LLM provider using any of the out-of-the-box templates:

```bash
curl -X POST http://localhost:9090/llm-providers \
  -H "Content-Type: application/yaml" \
  -H "Authorization: Basic YWRtaW46YWRtaW4=" \
  --data-binary @- <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: <unique-id>
spec:
  displayName: <display-name>
  version: v1.0
  template: <template-id>
  upstream:
    url: https://api.openai.com/v1
    auth:
      type: api-key
      header: <auth-header>
      value: <key>
  accessControl:
    mode: deny_all
    exceptions:
      - path: /chat/completions
        methods: [POST]
      - path: /models
        methods: [GET]
      - path: /models/{modelId}
        methods: [GET]
EOF
```

Replace the placeholders:
- `<unique-id>`: Unique identifier for your provider (e.g., `my-openai-provider`)
- `<display-name>`: Human-readable name (e.g., `My OpenAI Provider`)
- `<template-id>`: One of the supported template IDs (`openai`, `azure-openai`, `anthropic`, `gemini`, `mistralai`, `awsbedrock`, `azureai-foundry`)
- `<auth-header>`: Authentication header name (e.g., `Authorization` for most providers)
- `<key>`: Your API key with appropriate prefix (e.g., `Bearer sk-...` for OpenAI)

The gateway automatically uses the template's metadata extraction patterns to:
- Extract token usage information from responses
- Track model usage for analytics
- Enable token-based rate limiting policies
- Provide consistent monitoring across different LLM providers

## Managing Templates

### Listing Available Templates

To list all available LLM provider templates:

```bash
curl -X GET http://localhost:9090/llm-provider-templates \
  -H "Authorization: Basic YWRtaW46YWRtaW4="
```

### Retrieving a Specific Template

To retrieve details of a specific template:

```bash
curl -X GET http://localhost:9090/llm-provider-templates/openai \
  -H "Authorization: Basic YWRtaW46YWRtaW4="
```

### Creating Custom Templates

Platform administrators can create custom templates for LLM providers not covered by the out-of-the-box templates:

```bash
curl -X POST http://localhost:9090/llm-provider-templates \
  -H "Content-Type: application/yaml" \
  -H "Authorization: Basic YWRtaW46YWRtaW4=" \
  --data-binary @- <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
metadata:
  name: custom-provider
spec:
  displayName: Custom Provider
  totalTokens:
    location: payload
    identifier: $.tokens.total
EOF
```

### Updating Templates

To update an existing custom template:

```bash
curl -X PUT http://localhost:9090/llm-provider-templates/custom-provider \
  -H "Content-Type: application/yaml" \
  -H "Authorization: Basic YWRtaW46YWRtaW4=" \
  --data-binary @- <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
metadata:
  name: custom-provider
spec:
  displayName: Custom Provider Updated
  promptTokens:
    location: payload
    identifier: $.usage.input_tokens
  # ... other fields
EOF
```

### Deleting Custom Templates

To delete a custom template:

```bash
curl -X DELETE http://localhost:9090/llm-provider-templates/custom-provider \
  -H "Authorization: Basic YWRtaW46YWRtaW4="
```

**Note**: Out-of-the-box templates cannot be deleted or modified. Only custom templates created by platform administrators can be updated or deleted.

## Template Field Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `apiVersion` | string | Yes | API version, must be `gateway.api-platform.wso2.com/v1alpha1` |
| `kind` | string | Yes | Resource kind, must be `LlmProviderTemplate` |
| `metadata.name` | string | Yes | Unique identifier for the template (used as template ID) |
| `spec.displayName` | string | Yes | Human-readable name for the template |
| `spec.promptTokens` | object | No | Configuration for extracting prompt/input token count |
| `spec.completionTokens` | object | No | Configuration for extracting completion/output token count |
| `spec.totalTokens` | object | No | Configuration for extracting total token count |
| `spec.remainingTokens` | object | No | Configuration for extracting remaining token allowance |
| `spec.requestModel` | object | No | Configuration for extracting request model identifier |
| `spec.responseModel` | object | No | Configuration for extracting response model identifier |

### Extraction Configuration Object

Each extraction configuration object has the following structure:

| Field | Type | Values | Description |
|-------|------|--------|-------------|
| `location` | string | `payload`, `header`, `pathParam` | Where to extract the value from |
| `identifier` | string | - | JSONPath (for payload), header name (for header), or regex pattern (for pathParam) |
