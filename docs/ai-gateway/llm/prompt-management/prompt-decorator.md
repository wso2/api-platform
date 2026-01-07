# Prompt Decorator

## Overview

The Prompt Decorator policy dynamically modifies prompts by prepending or appending custom content to specific fields in JSON payloads. This policy supports two decoration modes: **text prompt decoration** (for string content fields) and **chat prompt decoration** (for message arrays). It's useful for adding consistent instructions, system messages, or standardized prefixes/suffixes to prompts before they're sent to AI services.

## Features

- Two decoration modes: text decoration (string fields) and chat decoration (message arrays)
- Configurable prepend or append behavior
- JSONPath support for targeting specific fields in JSON payloads
- Flexible decoration format: simple strings or structured message objects
- Processes request body only (response phase not supported)

## Configuration

### Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `promptDecoratorConfig` | string | Yes | - | JSON string containing decoration configuration. For text decoration: `{"decoration": "string"}`. For chat decoration: `{"decoration": [{"role": "system", "content": "..."}]}` |
| `jsonPath` | string | Yes | - | JSONPath expression to locate the field to decorate. Use `$.messages[0].content` for text decoration, or `$.messages` for chat decoration. |
| `append` | boolean | No | `false` | If `true`, decoration is appended to the content. If `false`, decoration is prepended (default). |

## Decoration Modes

### Mode 1: Text Prompt Decoration

Text decoration is used when the JSONPath targets a string field (e.g., `$.messages[0].content`). The decoration can be:
- A simple string that gets prepended or appended to the content
- An array of decoration objects (their content fields are concatenated with newlines)

**Configuration Example:**
```json
{
  "decoration": "Summarize the following content in a concise, neutral, and professional tone. Structure the summary using bullet points if appropriate.\n\n"
}
```

**Behavior:**
- Decoration string is prepended or appended to the target content field
- A space is automatically added between the decoration and original content

### Mode 2: Chat Prompt Decoration

Chat decoration is used when the JSONPath targets an array field (e.g., `$.messages`). The decoration must be an array of message objects:

**Configuration Example:**
```json
{
  "decoration": [
    {
      "role": "system",
      "content": "You are a helpful hotel booking receptionist for the imaginary hotel 'Azure Horizon Resort'. Your job is to collect all the necessary booking details from guests."
    }
  ]
}
```

**Behavior:**
- Decoration messages are prepended or appended to the messages array
- Each decoration object must have `role` and `content` fields
- Multiple decoration messages can be added

## JSONPath Support

The decorator supports JSONPath expressions to target specific fields. Common examples:

- `$.messages[0].content` - First message's content field (text decoration)
- `$.messages[-1].content` - Last message's content field (text decoration)
- `$.messages` - Entire messages array (chat decoration)
- `$.data.text` - Nested text field (text decoration)

**Array Index Syntax:**
- Use `[0]` for first element, `[1]` for second, etc.
- Use `[-1]` for last element, `[-2]` for second-to-last, etc.

## Examples

### Example 1: Text Prompt Decoration - Summarization Directive

Add a summarization instruction to user prompts:

```bash
curl -X POST http://localhost:9090/llm-providers \
  -H "Content-Type: application/yaml" \
  -H "Authorization: Basic YWRtaW46YWRtaW4=" \
  --data-binary @- <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: summarization-provider
spec:
  displayName: Summarization Provider
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
    - name: prompt-decorator
      version: v0.1.0
      paths:
        - path: /chat/completions
          methods: [POST]
          params:
            promptDecoratorConfig: '{"decoration": "Summarize the following content in a concise, neutral, and professional tone. Structure the summary using bullet points if appropriate.\\n\\n"}'
            jsonPath: "$.messages[0].content"
            append: false
EOF
```

**Test the decorator:**

**Note**: Ensure that "openai" is mapped to the appropriate IP address (e.g., 127.0.0.1) in your `/etc/hosts` file, or remove the vhost from the LLM provider configuration and use localhost to invoke.

```bash
# Original request
curl -X POST http://openai:8080/chat/completions \
  -H "Content-Type: application/json" \
  -H "Host: openai" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "Large text block to summarize here..."
      }
    ]
  }'

# After decoration, the request sent to OpenAI will be:
# {
#   "model": "gpt-4",
#   "messages": [
#     {
#       "role": "user",
#       "content": "Summarize the following content in a concise, neutral, and professional tone. Structure the summary using bullet points if appropriate.\n\n Large text block to summarize here..."
#     }
#   ]
# }
```

### Example 2: Chat Prompt Decoration - System Persona

Add a system message to define AI behavior:

```bash
curl -X POST http://localhost:9090/llm-providers \
  -H "Content-Type: application/yaml" \
  -H "Authorization: Basic YWRtaW46YWRtaW4=" \
  --data-binary @- <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: hotel-booking-provider
spec:
  displayName: Hotel Booking Provider
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
    - name: prompt-decorator
      version: v0.1.0
      paths:
        - path: /chat/completions
          methods: [POST]
          params:
            promptDecoratorConfig: '{"decoration": [{"role": "system", "content": "You are a helpful hotel booking receptionist for Azure Horizon Resort. Collect booking details: name, NIC, check-in time, staying duration (nights), and room type (single, double, suite). Ask one detail at a time in a friendly tone."}]}'
            jsonPath: "$.messages"
            append: false
EOF
```

**Test the decorator:**

```bash
# Original request
curl -X POST http://openai:8080/chat/completions \
  -H "Content-Type: application/json" \
  -H "Host: openai" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "Hi, I would like to book a room."
      }
    ]
  }'

# After decoration, the request sent to OpenAI will be:
# {
#   "model": "gpt-4",
#   "messages": [
#     {
#       "role": "system",
#       "content": "You are a helpful hotel booking receptionist for Azure Horizon Resort. Collect booking details: name, NIC, check-in time, staying duration (nights), and room type (single, double, suite). Ask one detail at a time in a friendly tone."
#     },
#     {
#       "role": "user",
#       "content": "Hi, I would like to book a room."
#     }
#   ]
# }
```

### Example 3: Append Mode - Adding Suffix Instructions

Append instructions to the end of user messages:

```yaml
policies:
  - name: prompt-decorator
    version: v0.1.0
    paths:
      - path: /chat/completions
        methods: [POST]
        params:
          promptDecoratorConfig: '{"decoration": "\\n\\nPlease respond in JSON format."}'
          jsonPath: "$.messages[-1].content"
          append: true
```

## Use Cases

1. **Consistent Instructions**: Prepend standardized instructions or guidelines to all prompts to ensure consistent AI behavior.

2. **System Personas**: Inject system messages to define AI personality, role, or behavior before user interactions.

3. **Quality Enhancement**: Add formatting instructions (e.g., "respond in bullet points", "use professional tone") to improve response quality.

4. **Context Addition**: Prepend contextual information or background details to enrich prompts.

5. **Multi-turn Conversations**: Add system messages at the beginning of chat conversations to set conversation rules.

6. **Compliance**: Append compliance-related instructions or disclaimers to prompts.

7. **Output Formatting**: Add instructions for specific output formats (JSON, markdown, structured text) to prompts.

## Configuration Reference

### Text Decoration Configuration

```json
{
  "decoration": "Your decoration string here"
}
```

- Simple string that will be prepended or appended to the target content
- A space is automatically added between decoration and original content

### Chat Decoration Configuration

```json
{
  "decoration": [
    {
      "role": "system",
      "content": "Your system message content"
    }
  ]
}
```

- Array of message objects
- Each object must have `role` (e.g., "system", "user", "assistant") and `content` fields
- Messages are prepended or appended to the messages array in the order specified

## Error Response

When the policy encounters an error (e.g., invalid JSONPath, missing fields), it returns an HTTP 500 status code with the following structure:

```json
{
  "type": "PROMPT_DECORATOR_ERROR",
  "message": "Error description here"
}
```

## Notes

- The policy only processes request bodies.
- For text decoration, a space is automatically added between the decoration and original content.
- JSONPath expressions must correctly identify the target field. Invalid paths will result in errors.
- When decorating message arrays, ensure the target field is actually an array of message objects.
- The `append: false` (default) means decoration is prepended. Set `append: true` to append decoration.
- Decoration objects in chat mode must have both `role` and `content` fields; both are required.
- Negative array indices (e.g., `[-1]` for last element) are supported in JSONPath expressions.
- When using text decoration with an array of decoration objects, their content fields are concatenated with newlines (`\n`).
