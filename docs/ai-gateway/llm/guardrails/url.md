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

Validate URLs in request content using HTTP HEAD requests:

```yaml
policies:
  - name: URLGuardrail
    version: v1.0.0
    enabled: true
    params:
      request:
        jsonPath: "$.messages[0].content"
        onlyDNS: false
        timeout: 5000
```

### Example 2: Fast DNS-Only Validation

Quick validation using DNS resolution only:

```yaml
policies:
  - name: URLGuardrail
    version: v1.0.0
    enabled: true
    params:
      request:
        jsonPath: "$.messages[0].content"
        onlyDNS: true
        timeout: 2000
```

### Example 3: Response URL Validation

Ensure AI responses contain only valid, reachable URLs:

```yaml
policies:
  - name: URLGuardrail
    version: v1.0.0
    enabled: true
    params:
      response:
        jsonPath: "$.choices[0].message.content"
        onlyDNS: false
        timeout: 3000
        showAssessment: true
```

### Example 4: Full Payload Validation

Validate URLs in the entire request body:

```yaml
policies:
  - name: URLGuardrail
    version: v1.0.0
    enabled: true
    params:
      request:
        onlyDNS: false
        timeout: 5000
```

## Use Cases

1. **Link Validation**: Ensure all URLs in AI-generated content are valid and accessible.

2. **Security**: Detect and block potentially malicious or suspicious URLs.

3. **Quality Assurance**: Prevent broken links from being included in responses.

4. **Content Moderation**: Validate URLs before allowing them in user-generated content.

5. **Resource Verification**: Ensure referenced resources are available before processing.

## Error Response

When validation fails, the guardrail returns an HTTP 446 status code with the following structure:

```json
{
  "code": 900514,
  "type": "URL_GUARDRAIL",
  "message": {
    "action": "GUARDRAIL_INTERVENED",
    "interveningGuardrail": "URLGuardrail",
    "actionReason": "Violation of url validity detected.",
    "direction": "REQUEST"
  }
}
```

If `showAssessment` is enabled, additional details including invalid URLs are included:

```json
{
    "code": 900514,
    "type": "URL_GUARDRAIL",
    "message": {
        "action": "GUARDRAIL_INTERVENED",
        "interveningGuardrail": "URLGuardrail",
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
