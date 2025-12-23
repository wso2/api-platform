# URL Guardrail

## Overview

The URL Guardrail validates URLs found in request or response body content by checking their reachability and validity. This guardrail helps prevent broken links, malicious URLs, and ensures that referenced resources are accessible.

## Features

- Validates URLs via DNS resolution or HTTP HEAD requests
- Supports JSONPath extraction to validate specific fields within JSON payloads
- Configurable timeout for URL validation
- Separate configuration for request and response phases
- Optional detailed assessment information including invalid URLs in error responses

## Configuration

### Parameters

#### Request Phase

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `jsonPath` | string | No | `""` | JSONPath expression to extract a specific value from JSON payload. If empty, validates the entire payload as a string. |
| `onlyDNS` | boolean | No | `false` | If `true`, validates URLs only via DNS resolution (faster, less reliable). If `false`, validates URLs via HTTP HEAD request (slower, more reliable). |
| `timeout` | integer | No | `3000` | Timeout in milliseconds for DNS lookup or HTTP HEAD request. Default is 3000ms (3 seconds). |
| `showAssessment` | boolean | No | `false` | If `true`, includes detailed assessment information including invalid URLs in error responses. |

#### Response Phase

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `jsonPath` | string | No | `""` | JSONPath expression to extract a specific value from JSON payload. If empty, validates the entire payload as a string. |
| `onlyDNS` | boolean | No | `false` | If `true`, validates URLs only via DNS resolution (faster, less reliable). If `false`, validates URLs via HTTP HEAD request (slower, more reliable). |
| `timeout` | integer | No | `3000` | Timeout in milliseconds for DNS lookup or HTTP HEAD request. Default is 3000ms (3 seconds). |
| `showAssessment` | boolean | No | `false` | If `true`, includes detailed assessment information including invalid URLs in error responses. |

## JSONPath Support

The guardrail supports JSONPath expressions to extract and validate specific fields within JSON payloads. Common examples:

- `$.message` - Extracts the `message` field from the root object
- `$.data.content` - Extracts nested content from `data.content`
- `$.items[0].text` - Extracts text from the first item in an array
- `$.messages[0].content` - Extracts content from the first message in a messages array

If `jsonPath` is empty or not specified, the entire payload is treated as a string and validated.

## URL Validation Modes

### DNS-Only Validation (`onlyDNS: true`)

- Faster validation method
- Only checks if the domain name resolves via DNS
- Does not verify HTTP/HTTPS accessibility
- Less reliable for detecting broken links
- Suitable for quick validation when HTTP checks are not necessary

### HTTP HEAD Request Validation (`onlyDNS: false`)

- More thorough validation method
- Performs DNS lookup and HTTP HEAD request
- Verifies that the URL is actually reachable
- More reliable for detecting broken or inaccessible URLs
- Slower due to network request overhead
- Recommended for production use

## Examples

### Example 1: Basic URL Validation

Deploy an LLM provider that validates URLs in request content using HTTP HEAD requests:

```bash
curl -X POST http://localhost:9090/llm-providers \
  -H "Content-Type: application/yaml" \
  -H "Authorization: Basic YWRtaW46YWRtaW4=" \
  --data-binary @- <<'EOF'
version: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: url-guardrail-provider
spec:
  displayName: URL Guardrail Provider
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
      - path: /models
        methods: [GET]
      - path: /models/{modelId}
        methods: [GET]
  policies:
    - name: url-guardrail
      version: v0.1.0
      paths:
        - path: /chat/completions
          methods: [POST]
          params:
            request:
              jsonPath: "$.messages[0].content"
              onlyDNS: false
              timeout: 5000
EOF
```

**Test the guardrail:**

**Note**: Ensure that "openai" is mapped to the appropriate IP address (e.g., 127.0.0.1) in your `/etc/hosts` file. or remove the vhost from the llm provider configuration and use localhost to invoke.

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
        "content": "Visit https://www.example.com for more information"
      }
    ]
  }'

# Invalid request - invalid URL (should fail with HTTP 422)
curl -X POST http://openai:8080/chat/completions \
  -H "Content-Type: application/json" \
  -H "Host: openai" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "Visit https://invalid-url-that-does-not-exist-12345.com"
      }
    ]
  }'
```

### Additional Configuration Options

You can customize the guardrail behavior by modifying the `policies` section:

- **Request and Response Validation**: Configure both `request` and `response` parameters to validate URLs in both directions. Use `showAssessment: true` to include detailed assessment information including invalid URLs in error responses.

- **DNS-Only Validation**: Set `onlyDNS: true` for faster validation that only checks DNS resolution. This is less reliable but faster than HTTP HEAD validation.

- **HTTP HEAD Validation**: Set `onlyDNS: false` (default) for more thorough validation that performs both DNS lookup and HTTP HEAD request to verify URL reachability.

- **Timeout Configuration**: Adjust the `timeout` parameter (in milliseconds) based on network conditions and acceptable latency. Default is 3000ms (3 seconds).

- **Full Payload Validation**: Omit the `jsonPath` parameter to validate URLs in the entire request body without JSONPath extraction.

- **Field-Specific Validation**: Use `jsonPath` to extract and validate URLs from specific fields within JSON payloads (e.g., `"$.messages[0].content"` for message content or `"$.choices[0].message.content"` for response content).

## Use Cases

1. **Link Validation**: Ensure all URLs in AI-generated content are valid and accessible.

2. **Security**: Detect and block potentially malicious or suspicious URLs.

3. **Quality Assurance**: Prevent broken links from being included in responses.

4. **Content Moderation**: Validate URLs before allowing them in user-generated content.

5. **Resource Verification**: Ensure referenced resources are available before processing.

## Error Response

When validation fails, the guardrail returns an HTTP 422 status code with the following structure:

```json
{
  "type": "URL_GUARDRAIL",
  "message": {
    "action": "GUARDRAIL_INTERVENED",
    "interveningGuardrail": "url-guardrail",
    "actionReason": "Violation of url validity detected.",
    "direction": "REQUEST"
  }
}
```

If `showAssessment` is enabled, additional details including invalid URLs are included:

```json
{
  "type": "URL_GUARDRAIL",
  "message": {
    "action": "GUARDRAIL_INTERVENED",
    "interveningGuardrail": "url-guardrail",
    "actionReason": "Violation of url validity detected.",
    "assessments": {
      "invalidUrls": [
        "http://example.com/suspicious-link",
        "https://foo.bar.baz"
      ],
      "message": "One or more URLs in the payload failed validation."
    },
    "direction": "REQUEST"
  }
}
```

## Notes

- URL validation extracts all URLs from the content using pattern matching.
- DNS-only validation is faster but less reliable than HTTP HEAD validation.
- Timeout values should be set based on network conditions and acceptable latency.
- HTTP HEAD requests may fail for URLs that require specific headers or authentication.
- Some URLs may be temporarily unavailable; consider retry logic for production use.
- When using JSONPath, if the path does not exist or the extracted value is not a string, validation will fail.
- The guardrail validates all URLs found in the content; if any URL is invalid, validation fails.
