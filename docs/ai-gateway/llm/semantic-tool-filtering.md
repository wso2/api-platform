# Semantic Tool Filtering

## Overview

The Semantic Tool Filtering policy intelligently filters and ranks tools/functions provided in LLM API requests based on their semantic relevance to the user's query. Unlike traditional static tool selection, this policy uses vector similarity to dynamically determine which tools are most relevant to the current request, reducing token usage and improving response quality by sending only pertinent tools to the LLM.

The policy uses embedding models to convert both the user query and tool descriptions into high-dimensional vectors, then calculates cosine similarity scores to identify the most relevant tools. It supports two filtering modes: TOP_K (select a fixed number of top-scoring tools) and THRESHOLD (select all tools above a similarity threshold).

## Features

- **Vector-based relevance scoring**: Uses embeddings to calculate semantic similarity between user queries and tool descriptions
- **Multiple filtering modes**: 
  - **TOP_K**: Select a fixed number of most relevant tools
  - **THRESHOLD**: Select all tools exceeding a similarity threshold
- **Multiple embedding provider support**: Works with OpenAI, Mistral, and Azure OpenAI embedding services
- **Configurable JSONPath extraction**: Extract user queries and tools from any location in the request body
- **Smart tool description extraction**: Automatically tries multiple common fields (description, desc, summary, info, function.description, name)
- **Request body modification**: Dynamically updates the tools array in the request before forwarding to upstream
- **Preserves tool order by relevance**: Filtered tools are sorted by similarity score (highest first)
- **Transparent operation**: Works with OpenAI-compatible tool/function calling formats

## How It Works

### Request Phase

1. **Content Extraction**: Extracts the request body content
2. **Query Extraction**: Uses JSONPath (default: `$.messages[-1].content`) to extract the user's query from the request
3. **Tools Extraction**: Uses JSONPath (default: `$.tools`) to extract the tools array from the request
4. **Query Embedding**: Generates a vector embedding for the user query using the configured embedding provider
5. **Tool Embeddings**: Generates embeddings for each tool's description
6. **Similarity Calculation**: Calculates cosine similarity between the query embedding and each tool embedding
7. **Filtering**: 
   - **TOP_K mode**: Sorts tools by similarity score and selects the top K tools
   - **THRESHOLD mode**: Selects all tools with similarity score >= threshold
8. **Request Modification**: Updates the request body with the filtered tools array
9. **Upstream Call**: Forwards the modified request to the upstream LLM service

### Response Phase

No processing in response phase - this is a request-only policy.

## Configuration

### Policy Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `selectionMode` | string | Yes | `TOP_K` | Filtering mode. Must be one of: `TOP_K` (select fixed number of top tools), `THRESHOLD` (select tools above similarity threshold) |
| `topK` | integer | No | `5` | Number of top-scoring tools to include when selectionMode is TOP_K. Range: 0-20 |
| `similarityThreshold` | number | No | `0.7` | Minimum similarity score (0.0 to 1.0) for including tools when selectionMode is THRESHOLD. Higher values are more strict. |
| `jsonPath` | string | No | `$.messages[-1].content` | JSONPath expression to extract the user's query from the request body |
| `toolsPath` | string | No | `$.tools` | JSONPath expression to extract the tools array from the request body |

### System Parameters (Required)

These parameters are typically configured at the gateway level and automatically injected, or you can override those values from the params section in the API artifact definition file as well:

#### Embedding Provider Configuration

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `embeddingProvider` | string | Yes | Embedding provider type. Must be one of: `OPENAI`, `MISTRAL`, `AZURE_OPENAI` |
| `embeddingEndpoint` | string | Yes | Endpoint URL for the embedding service. Examples: OpenAI: `https://api.openai.com/v1/embeddings`, Mistral: `https://api.mistral.ai/v1/embeddings`, Azure OpenAI: Your Azure OpenAI endpoint URL |
| `embeddingModel` | string | Conditional | Embedding model name. **Required for OPENAI and MISTRAL**, not required for AZURE_OPENAI (deployment name is in endpoint URL). Examples: OpenAI: `text-embedding-ada-002` or `text-embedding-3-small`, Mistral: `mistral-embed` |
| `apiKey` | string | Yes | API key for the embedding service authentication. The authentication header is automatically set to `api-key` for Azure OpenAI and `Authorization` for other providers. |

### Configuring System Parameters in config.yaml

System parameters can be configured globally in the gateway's `config.yaml` file. These values serve as defaults for all Semantic Tool Filtering policy instances and can be overridden per-policy in the API configuration if needed.

#### Location in config.yaml

Add the following configuration section to your `config.yaml` file:

```yaml
embedding_provider: "MISTRAL" # Supported: MISTRAL, OPENAI, AZURE_OPENAI
embedding_provider_endpoint: "https://api.mistral.ai/v1/embeddings"
embedding_provider_model: "mistral-embed"
embedding_provider_api_key: ""
```

Alternatively, for OpenAI:

```yaml
embedding_provider: "OPENAI"
embedding_provider_endpoint: "https://api.openai.com/v1/embeddings"
embedding_provider_model: "text-embedding-3-small"
embedding_provider_api_key: ""
```

## JSONPath Support

The policy supports JSONPath expressions to extract both the user query and tools array from request bodies. This is useful for:
- Supporting various LLM API formats (OpenAI, Anthropic, custom APIs)
- Extracting queries from different message structures
- Handling tools in non-standard locations

### Common JSONPath Examples

**For Queries:**
- `$.messages[-1].content` - Last message's content (default, works with OpenAI chat completions)
- `$.messages[0].content` - First message's content
- `$.prompt` - Extract prompt field from completions API
- `$.input` - Extract input field from custom formats

**For Tools:**
- `$.tools` - Standard location (default, OpenAI format)
- `$.functions` - Alternative location for function calling
- `$.available_tools` - Custom tools location

## Tool Description Extraction

The policy automatically tries to extract descriptions from tools using the following fields (in order of preference):

1. `description` - Most common field
2. `desc` - Short form
3. `summary` - Alternative description field
4. `info` - Information field
5. `function.description` - Nested description (OpenAI functions format)
6. `name` - Fallback to tool name if no description exists

This ensures compatibility with various tool/function definition formats.

## Examples

### Example 1: Basic TOP_K Filtering with OpenAI

Deploy an API with semantic tool filtering using TOP_K mode:

```bash
curl -X POST http://localhost:9090/apis \
  -H "Content-Type: application/yaml" \
  -H "Authorization: Basic YWRtaW46YWRtaW4=" \
  --data-binary @- <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: llm-with-tool-filtering
spec:
  displayName: LLM API with Tool Filtering
  version: v1.0
  context: /llm/$version
  upstream:
    main:
      url: https://api.openai.com/v1
      auth:
        type: api-key
        header: Authorization
        value: Bearer <openai-apikey>
  operations:
    - method: POST
      path: /chat/completions
      policies:
        - name: semantic-tool-filtering
          version: v0.1.0
          params:
            selectionMode: TOP_K
            topK: 3
            jsonPath: "$.messages[-1].content"
            toolsPath: "$.tools"
EOF
```

**Test the tool filtering:**

```bash
curl -X POST http://localhost:8080/llm/v1.0/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "What is the weather like in San Francisco?"
      }
    ],
    "tools": [
      {
        "type": "function",
        "function": {
          "name": "get_weather",
          "description": "Get the current weather for a specific location"
        }
      },
      {
        "type": "function",
        "function": {
          "name": "get_stock_price",
          "description": "Get current stock market prices for a ticker symbol"
        }
      },
      {
        "type": "function",
        "function": {
          "name": "send_email",
          "description": "Send an email to a recipient"
        }
      },
      {
        "type": "function",
        "function": {
          "name": "book_flight",
          "description": "Book airline tickets for travel"
        }
      },
      {
        "type": "function",
        "function": {
          "name": "calculate",
          "description": "Perform mathematical calculations"
        }
      }
    ]
  }'
```

**Result**: The request forwarded to OpenAI will contain only the top 3 most relevant tools (likely `get_weather` with highest score, plus 2 others).

### Example 2: THRESHOLD Mode for Quality Filtering

Use THRESHOLD mode to only include highly relevant tools:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: llm-quality-tool-filtering
spec:
  displayName: LLM API with Quality Tool Filtering
  version: v1.0
  context: /llm-quality/$version
  upstream:
    main:
      url: https://api.openai.com/v1
      auth:
        type: api-key
        header: Authorization
        value: Bearer <openai-apikey>
  operations:
    - method: POST
      path: /chat/completions
      policies:
        - name: semantic-tool-filtering
          version: v0.1.0
          params:
            selectionMode: THRESHOLD
            similarityThreshold: 0.8
            jsonPath: "$.messages[-1].content"
            toolsPath: "$.tools"
```

**Result**: Only tools with ≥80% similarity to the query will be included. If no tools meet the threshold, all tools are passed through.

### Example 3: Custom JSONPath for Anthropic Claude Format

Configure for Anthropic's tool format:

```yaml
operations:
  - method: POST
    path: /messages
    policies:
      - name: semantic-tool-filtering
        version: v0.1.0
        params:
          selectionMode: TOP_K
          topK: 5
          jsonPath: "$.messages[-1].content[0].text"
          toolsPath: "$.tools"
```

### Example 4: LLM Provider Configuration

Deploy an LLM provider with tool filtering:

```bash
curl -X POST http://localhost:9090/llm-providers \
  -H "Content-Type: application/yaml" \
  -H "Authorization: Basic YWRtaW46YWRtaW4=" \
  --data-binary @- <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: openai-filtered-tools
spec:
  displayName: OpenAI with Tool Filtering
  version: v1.0
  template: openai
  vhost: openai-filtered
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
    - name: semantic-tool-filtering
      version: v0.1.0
      paths:
        - path: /chat/completions
          methods: [POST]
          params:
            selectionMode: TOP_K
            topK: 5
            similarityThreshold: 0.75
            jsonPath: "$.messages[-1].content"
            toolsPath: "$.tools"
EOF
```

## Use Cases

1. **Token Reduction**: Reduce the number of tokens sent to LLMs by filtering out irrelevant tools, lowering API costs especially for models with high token prices.

2. **Improved Response Quality**: Help LLMs focus on relevant tools only, reducing confusion and improving the quality of tool selection and execution.

3. **Large Tool Libraries**: Efficiently manage APIs with dozens or hundreds of available tools by dynamically selecting only relevant ones per request.

4. **Context Window Management**: Keep requests within context window limits by reducing tool definitions, especially important for models with smaller context windows.

5. **Multi-tenant Applications**: Different users may have access to different tool sets - dynamically filter based on the specific query context.

6. **Function Calling Optimization**: Improve function calling accuracy by presenting LLMs with a focused, relevant subset of available functions.

7. **Prompt Injection Protection**: Reduce risk by limiting tool exposure - only semantically relevant tools are visible to the LLM per request.

8. **Cost-Performance Trade-off**: Balance between providing comprehensive tool access and minimizing token usage costs.

## Selection Mode Guidelines

### TOP_K Mode

Best for scenarios where you want:
- **Predictable token usage**: Always sends exactly K tools (or fewer if less than K tools are available)
- **Consistent behavior**: Same number of tools per request regardless of similarity scores
- **Simple configuration**: Just set the number of tools to include

**Recommended K values:**
- `K=3`: Very focused, minimal token usage, may miss some relevant tools
- `K=5`: Good balance for most use cases (default)
- `K=7-10`: More comprehensive, better coverage but higher token cost
- `K=15-20`: Maximum flexibility, use when tool descriptions are cheap/short

### THRESHOLD Mode

Best for scenarios where you want:
- **Quality over quantity**: Only include truly relevant tools
- **Variable filtering**: Number of tools varies based on query relevance
- **Strict relevance**: Prevent off-topic tool exposure

**Threshold Recommendations:**
- **0.85-1.0**: Very strict, only near-perfect matches (may result in no tools being filtered)
- **0.75-0.84**: Recommended for most use cases, good balance of relevance and coverage
- **0.65-0.74**: More lenient, broader tool selection
- **Below 0.65**: Not recommended, may include irrelevant tools

**Recommendation**: Start with TOP_K mode (K=5) for predictable behavior, switch to THRESHOLD mode (0.75-0.80) if you need quality-based filtering.

## Similarity Threshold Guidelines

The `similarityThreshold` parameter (used in THRESHOLD mode) controls minimum relevance:

- **0.90-1.0**: Extremely strict. Only tools that are almost exactly about the query topic.
- **0.80-0.89**: Strict matching. Tools must be highly relevant to the query.
- **0.70-0.79**: Recommended range. Good balance of relevance and coverage.
- **0.60-0.69**: Lenient matching. May include tangentially related tools.
- **Below 0.60**: Not recommended. Risk of including irrelevant tools.

## Filtering Behavior

### Tool Filtering

When filtering is successful:
- Tools array in the request is replaced with the filtered subset
- Tools are sorted by similarity score (highest first)
- Request is modified before forwarding to upstream
- Original tool structure is preserved

### No Filtering Cases

The policy skips filtering and forwards the original request in these scenarios:
- Empty request body
- Query extraction fails (JSONPath returns empty)
- Tools extraction fails (JSONPath returns empty or invalid format)
- No tools in the array
- All tool embeddings fail to generate
- Embedding provider errors

### Edge Cases

- **Fewer tools than topK**: Returns all available tools sorted by relevance
- **No tools meet threshold**: Returns all original tools (avoids breaking the request)
- **Tool description missing**: Uses tool name as fallback description
- **Embedding generation failure**: Skips that specific tool (continues with others)

## Error Handling

The policy is designed to be resilient:

- **Embedding Generation Failure**: If query embedding generation fails, the original request is forwarded unchanged (logs error, continues gracefully)
- **Tool Embedding Failure**: Individual tools that fail embedding generation are skipped; remaining tools are still filtered
- **JSONPath Extraction Failure**: If extraction fails, returns immediate error response (400 status)
- **Invalid Tools Format**: If tools array is malformed, returns immediate error response
- **Provider Initialization Failure**: Policy initialization fails (prevents deployment with invalid config)

Error responses follow this format:

```json
{
  "error": "SemanticToolFiltering",
  "message": "Descriptive error message",
  "details": "Additional error context (if available)"
}
```

## Performance Considerations

1. **Embedding Generation Latency**: 
   - Query embedding: ~50-200ms
   - Each tool embedding: ~50-200ms
   - Total overhead: ~50-200ms + (number_of_tools × 50-200ms)
   - For 10 tools: approximately 0.5-2 seconds additional latency

2. **Recommendation**: Use for APIs with >5-10 tools where the token savings justify the latency overhead.

3. **Embedding Caching**: The policy does not cache embeddings. For frequently used tools, consider:
   - Pre-computing and caching tool embeddings at the application level
   - Using consistent tool descriptions across requests

4. **Batch Processing**: Tool embeddings are generated sequentially. For large tool sets (>20 tools), expect noticeable latency.

5. **Similarity Calculation**: Cosine similarity calculation is fast (~microseconds per comparison) and not a performance bottleneck.

6. **Token Savings**: 
   - Each tool typically uses 50-200 tokens (depending on description length)
   - Filtering from 20 tools to 5 tools can save 750-3000 tokens per request
   - At GPT-4 pricing ($0.03/1K input tokens), this saves $0.0225-$0.09 per request

7. **Optimal Configuration**:
   - For latency-sensitive applications: TOP_K mode with K≤5
   - For cost-sensitive applications: TOP_K mode with K=3
   - For quality-sensitive applications: THRESHOLD mode with threshold=0.8

## Notes

- The policy only processes request phase. Response phase is a no-op.

- Embedding generation adds latency proportional to the number of tools. Consider this trade-off against token savings.

- Tool descriptions should be clear and descriptive. Vague descriptions (e.g., "utility function") will result in poor similarity matching.

- The policy maintains the structure of tool objects - only the array is filtered, individual tool structures are preserved.

- Cosine similarity ranges from -1 to 1, but for embeddings typically ranges from 0 to 1. Scores above 0.7 indicate reasonable relevance.

- The policy works best with tools that have distinct, descriptive descriptions. Overlapping or ambiguous descriptions may reduce filtering accuracy.

- For optimal results, ensure all tools have meaningful descriptions. Tools without descriptions fall back to using the tool name, which may produce less accurate similarity scores.

- The policy is stateless - each request is processed independently without any cross-request state or caching.

- Large tool sets (>50 tools) may experience significant latency due to embedding generation. Consider pre-filtering at the application level for very large tool libraries.

- The policy automatically handles both OpenAI's `tools` format (with nested `function` object) and simpler formats (flat description field).

- JSONPath extraction errors result in immediate error responses to avoid silent failures and unexpected behavior.

- For production deployments, monitor:
  - Embedding generation latency
  - Average number of tools before/after filtering
  - Token usage reduction
  - Query-tool similarity score distributions

- The filtered tools array maintains semantic relevance ordering (highest similarity first), which may help LLMs make better tool selection decisions.
