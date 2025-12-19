# Prompt Template

## Overview

The Prompt Template policy enables dynamic prompt transformation by replacing `template://` URI patterns in JSON payloads with predefined templates. Template placeholders are resolved using parameters passed in the URI query string, allowing you to standardize and reuse prompts across different API calls. This is particularly useful for AI/LLM APIs where consistent prompt formatting improves response quality and maintainability.

## Features

- Pattern-based template matching using `template://` URI format
- Parameter substitution with `[[parameter-name]]` placeholder syntax
- Multiple templates per policy configuration
- JSON-safe string replacement and escaping
- Processes entire JSON payload as string to find and replace patterns

## Configuration

### Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `promptTemplateConfig` | string | Yes | - | JSON string containing an array of template objects. Each template must have a `name` and `prompt` field. Example: `[{"name": "translate", "prompt": "Translate from [[from]] to [[to]]: [[text]]"}]` |
| `jsonPath` | string | No | `$.messages[0].content` | **Note**: Currently not used. The policy searches the entire JSON payload string for `template://` patterns. This parameter is reserved for future use. |

### Template Configuration Format

The `promptTemplateConfig` must be a JSON array of template objects:

```json
[
  {
    "name": "template-name",
    "prompt": "Template prompt with [[placeholder]] syntax"
  }
]
```

Each template object contains:
- **name**: Unique identifier for the template (used in `template://` URIs)
- **prompt**: The template string with `[[parameter-name]]` placeholders that will be replaced

## Template Syntax

### Template URI Format

Templates are referenced in JSON payloads using the following URI format:

```
template://<template-name>?<param1>=<value1>&<param2>=<value2>
```

Example:
```
template://translate?from=english&to=spanish&text=Hello world
```

### Placeholder Syntax

Within template prompts, use double square brackets to define placeholders:

```
[[parameter-name]]
```

During resolution, placeholders are replaced with values from the URI query parameters. Parameter names are case-sensitive and must match exactly between the placeholder and the URI parameter.

Example template:
```
Translate the following text from [[from]] to [[to]]: [[text]]
```

When called with `template://translate?from=english&to=spanish&text=Hello`, the resolved prompt would be:
```
Translate the following text from english to spanish: Hello
```

## Examples

### Example 1: Translation Template

Deploy an LLM provider with a translation prompt template:

```bash
curl -X POST http://localhost:9090/llm-providers \
  -H "Content-Type: application/yaml" \
  --data-binary @- <<'EOF'
version: ai.api-platform.wso2.com/v1
kind: llm/provider
spec:
  name: translation-provider
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
    - name: PromptTemplate
      version: v0.1.0
      paths:
        - path: /chat/completions
          methods: [POST]
          params:
            promptTemplateConfig: '[{"name": "translate", "prompt": "Translate the following text from [[from]] to [[to]]: [[text]]"}]'
EOF
```

**Test the template:**

**Note**: Ensure that "openai" is mapped to the appropriate IP address (e.g., 127.0.0.1) in your `/etc/hosts` file, or remove the vhost from the LLM provider configuration and use localhost to invoke.

```bash
curl -X POST http://openai:8080/chat/completions \
  -H "Content-Type: application/json" \
  -H "Host: openai" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "template://translate?from=english&to=spanish&text=Hello world"
      }
    ]
  }'
```

The policy will transform the request to:

```json
{
  "model": "gpt-4",
  "messages": [
    {
      "role": "user",
      "content": "Translate the following text from english to spanish: Hello world"
    }
  ]
}
```

### Example 2: Summarization Template

Create a template for summarizing content with configurable length:

```bash
curl -X POST http://localhost:9090/llm-providers \
  -H "Content-Type: application/yaml" \
  --data-binary @- <<'EOF'
version: ai.api-platform.wso2.com/v1
kind: llm/provider
spec:
  name: summarization-provider
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
    - name: PromptTemplate
      version: v0.1.0
      paths:
        - path: /chat/completions
          methods: [POST]
          params:
            promptTemplateConfig: '[{"name": "summarize", "prompt": "Summarize the following content in [[length]] words: [[content]]"}]'
EOF
```

**Test with template:**

```bash
curl -X POST http://openai:8080/chat/completions \
  -H "Content-Type: application/json" \
  -H "Host: openai" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "template://summarize?length=50&content=Artificial intelligence is a branch of computer science that aims to create intelligent machines capable of performing tasks that typically require human intelligence."
      }
    ]
  }'
```

### Example 3: Multiple Templates

Configure multiple templates in a single policy:

```yaml
policies:
  - name: PromptTemplate
    version: v0.1.0
    paths:
      - path: /chat/completions
        methods: [POST]
        params:
          promptTemplateConfig: |
            [
              {
                "name": "translate",
                "prompt": "Translate from [[from]] to [[to]]: [[text]]"
              },
              {
                "name": "summarize",
                "prompt": "Summarize in [[length]] words: [[content]]"
              },
              {
                "name": "explain",
                "prompt": "Explain [[topic]] to a [[audience]] audience: [[question]]"
              }
            ]
```

## Use Cases

1. **Standardized Prompts**: Ensure consistent prompt formatting across different API consumers by centralizing prompt definitions.

2. **Reusable Templates**: Create library of common prompts (translation, summarization, explanation) that can be reused across multiple APIs.

3. **Parameterized Prompts**: Allow dynamic content insertion while maintaining consistent prompt structure and quality.

4. **Multi-language Support**: Use templates with language parameters to standardize prompts for different locales.

5. **Prompt Versioning**: Update prompt templates centrally without requiring changes to client applications.

## Template Pattern Matching

The policy uses regex pattern matching to find `template://` URIs in the JSON payload:

- **Pattern**: `template://[a-zA-Z0-9_-]+\?[^\s"']*`
- **Location**: Searches the entire JSON payload as a string
- **Replacement**: Each matched pattern is replaced with the resolved template string (JSON-escaped)

### Pattern Details

- Template names can contain letters, numbers, underscores, and hyphens
- Query parameters can contain any characters except spaces, quotes, or single quotes
- Multiple template:// patterns can exist in a single payload
- Each pattern is resolved independently

## Error Handling

If a template:// pattern references a template name that doesn't exist in the configuration, the pattern is left unchanged (no replacement occurs). This allows for graceful handling of missing templates.

When template resolution fails (e.g., invalid JSON escaping), the specific pattern is skipped and other patterns continue to be processed.

## Notes

- Template names are case-sensitive and must match exactly between the URI reference and the configuration.
- Parameter names in placeholders `[[param]]` are case-sensitive and must match query parameter names exactly.
- Query parameter values are URL-decoded before being inserted into templates.
- The resolved template string is JSON-escaped (special characters like quotes, newlines are escaped) before replacement.
- The policy processes the entire JSON payload as a string, so templates can be used anywhere in the JSON structure.
- Multiple `template://` patterns can appear in a single payload and will all be processed.
- The `jsonPath` parameter is currently reserved for future functionality and is not used in the current implementation.
