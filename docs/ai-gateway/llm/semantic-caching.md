# Semantic Caching

## Overview

The Semantic Cache policy enables intelligent response caching for LLM (Large Language Model) APIs using vector similarity search. Unlike traditional key-based caching, semantic caching understands the meaning of requests and can serve cached responses for semantically similar queries, even when the exact wording differs. This dramatically improves performance and reduces costs by avoiding redundant API calls to upstream LLM services.

The policy uses embedding models to convert request text into high-dimensional vectors, then performs similarity searches in a vector database to find previously cached responses. If a similar request is found within the configured similarity threshold, the cached response is returned immediately without calling the upstream service.

## Features

- **Vector-based similarity matching**: Uses embeddings to find semantically similar requests, not just exact matches
- **Multiple embedding provider support**: Works with OpenAI, Mistral, and Azure OpenAI embedding services
- **Multiple vector database support**: Supports Redis and Milvus as vector storage backends
- **Configurable similarity threshold**: Control cache hit sensitivity (0.0 to 1.0)
- **JSONPath extraction**: Extract specific fields from request body for embedding generation
- **Automatic cache management**: Stores successful responses (200) automatically after upstream calls
- **Immediate response on cache hit**: Returns cached response with `X-Cache-Status: HIT` header without upstream call
- **TTL support**: Configurable time-to-live for cache entries

## How It Works

### Request Phase

1. **Text Extraction**: Extracts text from the request body using JSONPath (if configured) or uses the entire request body
2. **Embedding Generation**: Generates a vector embedding from the extracted text using the configured embedding provider
3. **Cache Lookup**: Searches the vector database for semantically similar cached responses using cosine similarity
4. **Threshold Check**: If a similar embedding is found with similarity >= similarityThreshold, returns the cached response immediately
5. **Cache Miss**: If no similar response is found, the request proceeds to the upstream service

### Response Phase

1. **Success Check**: Only processes responses with 200 status codes
2. **Embedding Retrieval**: Retrieves the embedding generated during the request phase from metadata
3. **Response Storage**: Stores the response payload along with its embedding in the vector database
4. **TTL Application**: Applies the configured TTL to the cache entry

## Configuration

### Policy Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `similarityThreshold` | number | Yes | - | Similarity threshold for cache hits (0.0 to 1.0). Higher values require more similarity. For example, 0.9 means 90% similarity required. Recommended: 0.85-0.95 for strict matching, 0.70-0.85 for more flexible matching. |
| `jsonPath` | string | No | `""` | JSONPath expression to extract text from request body for embedding generation. If empty, uses the entire request body. Example: `"$.messages[0].content"` to extract the first message's content. |

### System Parameters (Required)

These parameters are typically configured at the gateway level and automatically injected, or you can override those values from the params section in the api artifact definition file as well:

#### Embedding Provider Configuration

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `embeddingProvider` | string | Yes | Embedding provider type. Must be one of: `OPENAI`, `MISTRAL`, `AZURE_OPENAI` |
| `embeddingEndpoint` | string | Yes | Endpoint URL for the embedding service. Examples: OpenAI: `https://api.openai.com/v1/embeddings`, Mistral: `https://api.mistral.ai/v1/embeddings`, Azure OpenAI: Your Azure OpenAI endpoint URL |
| `embeddingModel` | string | Conditional | - | Embedding model name. **Required for OPENAI and MISTRAL**, not required for AZURE_OPENAI (deployment name is in endpoint URL). Examples: OpenAI: `text-embedding-ada-002` or `text-embedding-3-small`, Mistral: `mistral-embed` |
| `embeddingDimension` | integer | Yes | Dimension of embedding vectors. Common values: 1536 (OpenAI ada-002), 1024 (Mistral). Must match the model's output dimension. |
| `apiKey` | string | Yes | API key for the embedding service authentication. The authentication header is automatically set to `api-key` for Azure OpenAI and `Authorization` for other providers. |

#### Vector Database Configuration

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `vectorStoreProvider` | string | Yes | Vector database provider. Must be one of: `REDIS`, `MILVUS` |
| `dbHost` | string | Yes | Vector database host address |
| `dbPort` | integer | Yes | Vector database port number |
| `username` | string | No | Database username for authentication (if required) |
| `password` | string | No | Database password for authentication (if required) |
| `database` | string | No | Database name or index number (for Redis) |
| `ttl` | integer | No | Time-to-live for cache entries in seconds. Default is 3600 (1 hour). Set to 0 for no expiration. |


### Configuring System Parameters in config.toml

System parameters can be configured globally in the gateway's `config.toml` file. These values serve as defaults for all Semantic Cache policy instances and can be overridden per-policy in the API configuration if needed.

#### Location in config.toml

Add the following configuration section to your `config.toml` file:

```toml
embedding_provider = "MISTRAL" # Supported: MISTRAL, OPENAI, AZURE_OPENAI
embedding_provider_endpoint = "https://api.mistral.ai/v1/embeddings"
embedding_provider_model = "mistral-embed"
embedding_provider_dimension = 1024
embedding_provider_api_key = ""

vector_db_provider = "REDIS" # Supported: REDIS, MILVUS
vector_db_provider_host = "redis"
vector_db_provider_port = 6379
vector_db_provider_database = "0"
vector_db_provider_username = "default"
vector_db_provider_password = "default"
vector_db_provider_ttl = 3600
```

## JSONPath Support

The policy supports JSONPath expressions to extract specific text from request bodies before generating embeddings. This is useful for:
- Extracting message content from chat completion requests
- Focusing on specific prompt fields while ignoring metadata
- Handling structured JSON payloads

### Common JSONPath Examples

- `$.messages[0].content` - First message's content in chat completions
- `$.messages[-1].content` - Last message's content
- `$.prompt` - Extract prompt field from completions API
- `$.input` - Extract input field from embeddings API
- `$` - Entire request body (default if jsonPath is not specified)

## Examples

### Example 1: OpenAI Embeddings with Redis

Deploy an LLM provider with semantic caching using OpenAI embeddings and Redis vector store:

```bash
curl -X POST http://localhost:9090/llm-providers \
  -H "Content-Type: application/yaml" \
  -H "Authorization: Basic YWRtaW46YWRtaW4=" \
  --data-binary @- <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: cached-chat-provider
spec:
  displayName: OpenAI Cached Provider
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
    - name: semantic-cache
      version: v0.1.0
      paths:
        - path: /chat/completions
          methods: [POST]
          params:
            similarityThreshold: 0.85
            jsonPath: "$.messages[0].content"
EOF
```

**Test the semantic cache:**

**Note**: Ensure that "openai" is mapped to the appropriate IP address (e.g., 127.0.0.1) in your `/etc/hosts` file, or remove the vhost from the LLM provider configuration and use localhost to invoke.

```bash
# First request - cache miss, will call upstream
curl -X POST http://openai:8080/chat/completions \
  -H "Content-Type: application/json" \
  -H "Host: openai" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "Explain quantum computing in simple terms"
      }
    ]
  }'

# Second request with similar but different wording - cache hit!
curl -X POST http://openai:8080/chat/completions \
  -H "Content-Type: application/json" \
  -H "Host: openai" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "Can you describe quantum computing using simple language?"
      }
    ]
  }'
# Response will include: X-Cache-Status: HIT
```

## Use Cases

1. **Cost Reduction**: Reduce API costs by serving cached responses for similar queries, especially valuable for expensive LLM API calls.

2. **Latency Improvement**: Return cached responses in milliseconds instead of waiting for LLM processing (typically 1-5 seconds), significantly improving user experience.

3. **Rate Limit Management**: Reduce the number of upstream API calls, helping stay within rate limits and avoiding throttling.

4. **Consistency**: Ensure users receive consistent responses for semantically equivalent queries, improving predictability.

5. **Offline Resilience**: Serve cached responses even when upstream services experience temporary outages.

6. **A/B Testing**: Compare cached vs. fresh responses to evaluate prompt engineering improvements.

7. **Development/Testing**: Speed up development cycles by reusing cached responses during testing.

8. **High-Traffic Applications**: Handle high volumes of similar queries efficiently without overwhelming upstream services.

## Similarity Threshold Guidelines

The `similarityThreshold` parameter controls how similar requests must be to trigger a cache hit:

- **0.95-1.0**: Very strict matching. Only near-identical requests will hit cache. Use for exact-match scenarios.
- **0.85-0.94**: Recommended for most use cases. Catches semantically equivalent requests with some wording variation.
- **0.75-0.84**: More flexible matching. Useful for broader conceptual similarity.
- **0.60-0.74**: Very flexible. May return cached responses for loosely related queries.
- **Below 0.60**: Not recommended. Risk of returning irrelevant cached responses.

**Recommendation**: Start with 0.85 and adjust based on your use case. Monitor cache hit rates and response relevance to fine-tune.

## Cache Behavior

### Cache Hit

When a similar request is found:
- Returns cached response immediately (no upstream call)
- Adds `X-Cache-Status: HIT` header to response
- Status code: 200 (from cached response)
- Response time: Typically < 50ms (vs. 1-5 seconds for LLM APIs)

### Cache Miss

When no similar request is found:
- Request proceeds to upstream service normally
- Response is cached after successful upstream call (200 status)
- Subsequent similar requests may hit the cache

### Cache Storage

- Only successful responses (200 status code) are cached
- Responses are stored with their embeddings in the vector database
- TTL is applied to all cache entries
- Each route/API maintains a separate cache namespace to avoid cross-contamination

## Error Handling

The policy is designed to be resilient:

- **Embedding Generation Failure**: If embedding generation fails, the request proceeds to upstream (cache is skipped)
- **Vector Database Unavailable**: If the vector database is unreachable, requests proceed to upstream
- **Cache Storage Failure**: If storing a response fails, the response is still returned to the client (cache operation is non-blocking)
- **Invalid JSONPath**: If JSONPath extraction fails, the entire request body is used for embedding generation

These behaviors ensure that caching failures do not block legitimate requests. The system gracefully degrades to direct upstream calls when caching is unavailable.

## Performance Considerations

1. **Embedding Generation Latency**: Generating embeddings adds ~100-500ms to request processing. This is offset by cache hits that save 1-5 seconds.

2. **Vector Database Performance**: 
   - Redis with RedisSearch: Fast queries, good for smaller datasets (< 1M vectors)
   - Milvus: Optimized for large-scale vector search, better for > 1M vectors

3. **Cache Hit Rate**: Aim for 20-40% cache hit rate for cost-effective caching. Below 10% may not justify the overhead.

4. **Embedding Dimension**: Higher dimensions (e.g., 1536) provide better accuracy but increase storage and search time. Choose based on your quality requirements.

5. **Index Creation**: Vector database indexes are created automatically on first use. This may take a few seconds for large datasets.

## Notes

- The policy requires both request and response phases to function properly (generates embeddings in request phase, stores responses in response phase).

- Embedding generation adds latency to each request (~100-500ms). This overhead is typically offset by the performance gains from cache hits.

- Cache entries are scoped per route/API to prevent cross-contamination between different APIs or routes.

- Only responses with 200 status code are cached. Errors and non-200 responses are never cached.

- The similarity search uses cosine similarity to compare embeddings. This is optimal for semantic similarity matching.

- Vector database indexes are created automatically when the policy is first used. Ensure your vector database has sufficient resources.

- The policy maintains provider instances per route for efficiency. Configuration changes require policy reinitialization.

- TTL of 0 means no expiration. Use with caution as it may lead to unbounded cache growth.

- JSONPath extraction is optional. If not specified, the entire request body (as string) is used for embedding generation.

- The policy stores embeddings in metadata between request and response phases. Ensure metadata persistence is enabled in your gateway configuration.

- For production deployments, monitor cache hit rates, embedding generation latency, and vector database performance metrics to optimize configuration.
