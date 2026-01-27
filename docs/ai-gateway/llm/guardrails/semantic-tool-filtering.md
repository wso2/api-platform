# Semantic Tool Filtering

The Semantic Tool Filtering policy dynamically filters tools provided in an API request based on their semantic relevance to the user query. This optimization reduces token usage and improves LLM response quality by only including contextually relevant tools.

## How It Works

1. **Extract Query**: The policy extracts the user query from the request body using JSONPath or text tags
2. **Extract Tools**: Tool definitions are extracted from JSON or text format
3. **Generate Embeddings**: The user query is converted to an embedding vector using the configured embedding provider
4. **Calculate Similarity**: Each tool's description is compared against the query using cosine similarity
5. **Filter Tools**: Tools are filtered based on rank (top-K) or similarity threshold
6. **Modify Request**: The original tools array is replaced with the filtered subset

## Configuration

### Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `selectionMode` | string | `"By Rank"` | Filter method: `"By Rank"` or `"By Threshold"` |
| `Limit` | integer | `5` | Number of top tools to include (used with `"By Rank"`) |
| `Threshold` | number | `0.7` | Similarity threshold 0.0-1.0 (used with `"By Threshold"`) |
| `queryJSONPath` | string | `"$.messages[-1].content"` | JSONPath to extract user query |
| `toolsJSONPath` | string | `"$.tools"` | JSONPath to extract tools array |
| `userQueryIsJson` | boolean | `true` | Extract query via JSONPath (`true`) or `<userq>` tags (`false`) |
| `toolsIsJson` | boolean | `true` | Extract tools via JSONPath (`true`) or text tags (`false`) |

### System Parameters

These are configured at the system level:

| Parameter | Description |
|-----------|-------------|
| `embeddingProvider` | Provider type: `OPENAI`, `MISTRAL`, or `AZURE_OPENAI` |
| `embeddingEndpoint` | Endpoint URL for the embedding service |
| `embeddingModel` | Model name (e.g., `text-embedding-3-small`, `mistral-embed`) |
| `apiKey` | API key for the embedding service |

## Examples

### Example 1: JSON Format with Top-K Selection

Filter tools using rank-based selection, keeping only the top 2 most relevant tools.

**API Configuration:**

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: tool-filtered-agent
spec:
  displayName: Gemini tool filtered agent
  version: v1.0
  template: "gemini"
  upstream:
    url: "https://generativelanguage.googleapis.com/v1beta/models"
    auth:
      type: api-key
      header: x-goog-api-key
      value: <API_KEY>
  accessControl:
    mode: deny_all
    exceptions:
      - path: /gemini-3-flash-preview:generateContent
        methods: [POST]
  policies:
    - name: semantic-tool-filtering
      version: v0.1.0
      paths:
        - path: /gemini-3-flash-preview:generateContent
          methods: [POST]
          params:
            selectionMode: "By Rank"
            Limit: 2
            queryJSONPath: "$.contents[0].parts[0].text"
            toolsJSONPath: "$.tools[0].function_declarations"
            userQueryIsJson: true
            toolsIsJson: true
```

**Request Body:**

```json
{
  "contents": [
    {
      "role": "user",
      "parts": [
        {
          "text": "I'm planning a corporate retreat in Denver for next weekend. Can you find the weather forecast, book a conference room for 15 people, find a highly-rated catering service that offers vegan options, and then email the itinerary to my assistant at sarah@company.com?"
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
          "parameters": {
            "type": "OBJECT",
            "properties": {
              "location": { "type": "string", "description": "The city and state, e.g. Denver, CO" }
            },
            "required": ["location"]
          }
        },
        {
          "name": "book_venue",
          "description": "Reserve a conference room or meeting space.",
          "parameters": {
            "type": "OBJECT",
            "properties": {
              "location": { "type": "string" },
              "capacity": { "type": "integer", "description": "Number of people" },
              "date": { "type": "string", "description": "ISO date format" }
            },
            "required": ["location", "capacity", "date"]
          }
        },
        {
          "name": "find_restaurants",
          "description": "Locate dining options based on cuisine and dietary needs.",
          "parameters": {
            "type": "OBJECT",
            "properties": {
              "location": { "type": "string" },
              "dietary_options": { "type": "array", "items": { "type": "string" }, "description": "e.g. ['vegan', 'gluten-free']" }
            },
            "required": ["location"]
          }
        },
        {
          "name": "calendar_add",
          "description": "Create a new event on the user's primary calendar.",
          "parameters": {
            "type": "OBJECT",
            "properties": {
              "summary": { "type": "string" },
              "start_time": { "type": "string" },
              "end_time": { "type": "string" }
            },
            "required": ["summary", "start_time"]
          }
        },
        {
          "name": "send_email",
          "description": "Send an email to a specific recipient.",
          "parameters": {
            "type": "OBJECT",
            "properties": {
              "recipient": { "type": "string", "description": "Email address" },
              "subject": { "type": "string" },
              "body": { "type": "string" }
            },
            "required": ["recipient", "body"]
          }
        }
      ]
    }
  ]
}
```

**Result:** Only the 2 most semantically relevant tools (e.g., `get_weather` and `book_venue`) are sent to the LLM.

---

### Example 2: Text Format with Threshold Selection

Use text tags to define tools and filter based on similarity threshold.

**API Configuration:**

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: tool-filtered-agent-text
spec:
  displayName: Gemini tool filtered agent (text mode)
  version: v1.0
  template: "gemini"
  upstream:
    url: "https://generativelanguage.googleapis.com/v1beta/models"
    auth:
      type: api-key
      header: x-goog-api-key
      value: <API_KEY>
  accessControl:
    mode: deny_all
    exceptions:
      - path: /gemini-3-flash-preview:generateContent
        methods: [POST]
  policies:
    - name: semantic-tool-filtering
      version: v0.1.0
      paths:
        - path: /gemini-3-flash-preview:generateContent
          methods: [POST]
          params:
            selectionMode: "By Threshold"
            Threshold: 0.9
            queryJSONPath: "$.contents[0].parts[0].text"
            toolsJSONPath: "$.contents[0].parts[0].text"
            userQueryIsJson: false
            toolsIsJson: false
```

**Request Body:**

```json
{
  "contents": [
    {
      "parts": [
        {
          "text": "## Role: Executive Logistics Orchestrator\nYou are the intelligent core of the 'ExecuFlow' agentic platform. Your mission is to transform complex user requests into organized corporate events.\n\n## Application Flow & Integrated Toolset\n\n### Phase 1: Environmental & Contextual Analysis\n* **Environment Check:** Use <toolname>get_weather</toolname> (<tooldescription>Get current weather and 7-day forecast for a location</tooldescription>) to ensure conditions are suitable.\n* **Navigation Planning:** Use <toolname>map_directions</toolname> (<tooldescription>Get estimated travel time and routes between two points</tooldescription>) to account for transit buffers.\n\n### Phase 2: Infrastructure & Logistics Execution\n* **Venue Procurement:** Utilize <toolname>book_venue</toolname> (<tooldescription>Reserve meeting spaces or conference rooms</tooldescription>) to lock in work locations.\n* **Travel Arrangements:** Deploy <toolname>book_flight</toolname> (<tooldescription>Search and book airline tickets</tooldescription>), <toolname>hotel_search</toolname> (<tooldescription>Find and book accommodations based on dates and budget</tooldescription>), or <toolname>ride_share</toolname> (<tooldescription>Request a car from Uber or Lyft services</tooldescription>) as needed.\n* **Financial Processing:** Use <toolname>currency_converter</toolname> (<tooldescription>Convert values between different international currencies</tooldescription>) if needed.\n\n### Phase 3: Qualitative Research & Customization\n* **General Intelligence:** Use <toolname>search_web</toolname> (<tooldescription>Search the web for general information and reviews</tooldescription>) to find the best-rated vendors.\n* **Dietary & Dining:** Use <toolname>find_restaurants</toolname> (<tooldescription>Locate dining options based on cuisine and dietary needs</tooldescription>) for catering.\n\n### Phase 4: Finalization & Communication\n* **Calendar Integration:** Use <toolname>calendar_add</toolname> (<tooldescription>Create a new event on the user's primary calendar</tooldescription>) to block the schedule.\n* **Stakeholder Delivery:** Use <toolname>send_email</toolname> (<tooldescription>Send an email to a specific recipient with a subject and body</tooldescription>) to dispatch confirmations.\n\n<userq>I'm planning a corporate retreat in Denver for next weekend. Can you find the weather forecast, book a conference room for 15 people, find a highly-rated catering service that offers vegan options, and then email the itinerary to my assistant at sarah@company.com?</userq>"
        }
      ]
    }
  ]
}
```

**Text Format Tags:**

- **User Query:** `<userq>...</userq>` - Wraps the user's query
- **Tool Name:** `<toolname>...</toolname>` - Defines a tool's name
- **Tool Description:** `<tooldescription>...</tooldescription>` - Defines a tool's description

**Result:** Only tools with similarity score ≥ 0.9 are retained in the text content.

---

## Selection Modes

### By Rank

Selects a fixed number of the most relevant tools based on similarity scores.

```yaml
params:
  selectionMode: "By Rank"
  Limit: 5  # Keep top 5 tools
```

### By Threshold

Selects all tools that exceed a minimum similarity score.

```yaml
params:
  selectionMode: "By Threshold"
  Threshold: 0.7  # Keep tools with score >= 0.7
```

## Format Options

| userQueryIsJson | toolsIsJson | Description |
|-----------------|-------------|-------------|
| `true` | `true` | Both query and tools extracted from JSON using JSONPath |
| `false` | `false` | Both query and tools extracted from text using tags |
| `true` | `false` | Query from JSON, tools from text tags |
| `false` | `true` | Query from text tags, tools from JSON |

## Best Practices

1. **Choose appropriate thresholds**: Start with a threshold of 0.7 and adjust based on your use case
2. **Use Top-K for predictable results**: When you need a consistent number of tools
3. **Use Threshold for quality filtering**: When relevance is more important than quantity
4. **Optimize JSONPath expressions**: Ensure paths correctly target your request structure
5. **Cache benefits**: Tool embeddings are cached per API, improving performance on subsequent requests