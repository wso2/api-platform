---
title: "Overview"
---

# Semantic Tool Filtering policy

## Overview
The **Semantic Tool Filtering** policy dynamically filters the tools provided within an API request based on their semantic relevance to the user query. This policy extracts both the query and the tool definitions from the incoming payload, generates embeddings for the query, and performs a similarity search against the provided tools. It then replaces the original tools array with a filtered subset, optimizing the request before it reaches the LLM.

This policy helps reduce token consumption and improve LLM response quality by sending only the most relevant tools for each request.

## Features
- **Semantic similarity-based filtering** of tools using embedding vectors.
- **Two selection modes**: "By Rank" (top-K) and "By Threshold".
- **Flexible Format Support**: Supports both JSON and text-based tool/query extraction.
- **Embedding cache** with LRU eviction to minimize redundant API calls.
- **Configurable JSONPath expressions** for payload extraction.
- **Mixed mode support** (extract query from JSON and tools from text, or vice versa).

## Configuration

### User Parameters (API Definition)

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| selectionMode | string | Yes | `By Rank` | Method used to filter tools: `By Rank` (selects top-K) or `By Threshold` (selects based on score). |
| limit | integer | No | `5` | The number of most relevant tools to include (used if selectionMode is `By Rank`). |
| threshold | number | No | `0.7` | Similarity threshold for filtering (0.0 to 1.0). Only tools with a score above this value are included (used if selectionMode is `By Threshold`). |
| queryJSONPath | string | Yes | `$.messages[-1].content` | JSONPath expression to extract the user's query from the request body. |
| toolsJSONPath | string | Yes | `$.tools` | JSONPath expression to extract the tools definition from the request body. It can point to the array itself, such as `$.tools`, or to the iterated object inside each array item, such as `$.tools[*].function`. |
| userQueryIsJson | boolean | Yes | `true` | Specifies format of user query. `true`: use `queryJSONPath`. `false`: extract from text using `<userq>` tags. |
| toolsIsJson | boolean | Yes | `true` | Specifies format of tools definition. `true`: use `toolsJSONPath`. `false`: extract from text using `<toolname>`/`<tooldescription>` tags. |

### System Parameters (From config.toml)

These parameters are configured in the gateway's `config.toml` to set up the embedding provider.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| embeddingProvider | string | Yes | - | Embedding provider: `OPENAI`, `MISTRAL`, or `AZURE_OPENAI`. |
| embeddingEndpoint | string | Yes | - | Endpoint URL for the embedding service. |
| embeddingModel | string | Conditional | - | Model name (e.g., `text-embedding-3-small` or `mistral-embed`). Required for `OPENAI` and `MISTRAL`; optional for `AZURE_OPENAI` (deployment name is derived from the endpoint). |
| apiKey | string | Yes | - | API key for the embedding service. |

#### Sample System Configuration

Add the following configuration section under the root level in your `config.toml` file:

```toml
embedding_provider = "MISTRAL" # Supported: MISTRAL, OPENAI, AZURE_OPENAI
embedding_provider_endpoint = "https://api.mistral.ai/v1/embeddings"
embedding_provider_model = "mistral-embed"
embedding_provider_dimension = 1024
embedding_provider_api_key = ""
```

### build.yaml

Add the following entry to the `policies` section in `/gateway/build.yaml`:

```yaml
- name: semantic-tool-filtering
  gomodule: github.com/wso2/gateway-controllers/policies/semantic-tool-filtering@v0
```

## Reference Scenarios

### Scenario 1: Filtering Tools by Rank (JSON Format)

This scenario demonstrates filtering tools to select the top 3 most relevant ones based on a user query in a JSON payload.

**Configuration:**

```yaml
type: http
policies:
  - policy:
      name: semantic-tool-filtering
      parameters:
        selectionMode: "By Rank"
        limit: 3
        queryJSONPath: "$.contents[0].parts[0].text"
        toolsJSONPath: "$.tools[0].function_declarations"
        userQueryIsJson: true
        toolsIsJson: true
```

**Request:**

```json
{
  "contents": [
    {
      "role": "user",
      "parts": [
        {
          "text": "Get weather forecast. what are the tools you have?"
        }
      ]
    }
  ],
  "tools": [
    {
      "function_declarations": [
        {
          "name": "get_weather",
          "description": "Get current weather and 7-day forecast for a location.",
          "parameters": { "type": "OBJECT", "properties": { "location": { "type": "string" } }, "required": ["location"] }
        },
        {
          "name": "book_venue",
          "description": "Reserve a conference room or meeting space.",
          "parameters": { "type": "OBJECT", "properties": { "location": { "type": "string" } }, "required": ["location"] }
        },
        {
           "name": "send_email",
           "description": "Send an email to a specific recipient.",
           "parameters": { "type": "OBJECT", "properties": { "recipient": { "type": "string" } }, "required": ["recipient"] }
        }
      ]
    }
  ]
}
```

The policy will interpret the request, calculate embeddings, and filter the `tools` array to include only the top 3 matches (e.g., `get_weather`, `book_venue`, `send_email`).

### Scenario 1b: OpenAI/Mistral-style Function Wrappers

If the request wraps each tool as an object with a nested `function`, configure the iterator path so the policy reads `name` and `description` from the nested object while still filtering the parent `tools` array.

**Configuration:**

```yaml
type: http
policies:
  - policy:
      name: semantic-tool-filtering
      parameters:
        selectionMode: "By Rank"
        limit: 3
        queryJSONPath: "$.messages[-1].content"
        toolsJSONPath: "$.tools[*].function"
        userQueryIsJson: true
        toolsIsJson: true
```

**Request:**

```json
{
  "messages": [
    {
      "role": "user",
      "content": "What tools can help me check the weather?"
    }
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "Get current weather"
      }
    },
    {
      "type": "function",
      "function": {
        "name": "send_email",
        "description": "Send an email notification"
      }
    }
  ]
}
```

### Scenario 2: Filtering Tools by Threshold

In this scenario, only tools with a semantic similarity score of 0.7 or higher are included.

**Configuration:**

```yaml
policies:
  - policy:
      name: semantic-tool-filtering
      parameters:
        selectionMode: "By Threshold"
        threshold: 0.7
        # Rest of the parameters
```

### Scenario 3: Text Format (XML-like Tags)

This scenario handles cases where the user query and tool definitions are embedded in a text payload using custom tags.

**Configuration:**

```yaml
policies:
  - policy:
      name: semantic-tool-filtering
      parameters:
        selectionMode: "By Rank"
        limit: 3
        userQueryIsJson: false
        toolsIsJson: false
        queryJSONPath: "$.messages[-1].content.text"
        toolsJSONPath: "$.messages[-1].content.text"

```

**Request Body:**

```json
{
  "contents": [
    {
      "parts": [
        {
          "text": "You are a logistics assistant with access to the following tools:\n\n<toolname>get_weather</toolname><tooldescription>Get current weather and 7-day forecast for a location</tooldescription>\n<toolname>book_venue</toolname><tooldescription>Reserve meeting spaces</tooldescription>\n\n<userq>I'm planning a corporate retreat in Denver next weekend. Check the weather.</userq>"
        }
      ]
    }
  ]
}
```

The policy extracts `<userq>` as the query and `<toolname>`/`<tooldescription>` as tools, then performs filtering. After the filtering process, the tags are removed.
