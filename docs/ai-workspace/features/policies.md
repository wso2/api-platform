# Policies

Policies are rules applied to traffic flowing through the gateway. They can be attached to LLM Providers, LLM Proxies, or MCP Proxies. Multiple policies form a chain that executes in order for each request and response.

Policies are configured in the AI Workspace UI and pushed to gateway runtimes via the Platform API. The gateway policy engine executes them as Envoy external processor extensions.

## Guardrails

Guardrails inspect or transform the content of requests and responses.

### Semantic Prompt Guard

Classifies prompts semantically and blocks or allows them based on similarity to a configured set of allowed or denied topic descriptions.

**Use when:** You need topic-based access control (e.g. block prompts about competitors, restrict to specific use cases).

Configuration:
- **Mode**: `allow` (block everything not in the allow list) or `deny` (block topics in the deny list)
- **Topics**: Natural language descriptions of allowed or denied topics
- **Threshold**: Similarity score threshold (0.0–1.0)

### PII Masking (Regex)

Detects personally identifiable information in requests and responses using configurable regex patterns and replaces matches with a placeholder.

**Use when:** You need to prevent PII from reaching the LLM or appearing in responses logged to analytics.

Configuration:
- **Patterns**: List of named regex patterns (e.g. `EMAIL`, `PHONE`, `SSN`)
- **Placeholder**: Replacement string (default: `[REDACTED]`)
- **Direction**: Apply to request, response, or both

### Azure Content Safety

Sends content to the Azure Content Safety API for moderation before forwarding to the LLM. Blocks requests or responses that exceed configured severity thresholds for hate speech, violence, sexual content, or self-harm.

**Use when:** You need enterprise-grade content moderation backed by Azure's safety models.

Configuration:
- **Azure endpoint** and **API key**
- **Category thresholds**: Per-category severity levels (0–6)

### Word Count Guardrail

Enforces minimum and/or maximum word count limits on request prompts or LLM responses.

**Use when:** You want to prevent very short or very long inputs/outputs (e.g. limit response length for cost control).

Configuration:
- **Min words** / **Max words**
- **Direction**: Request, response, or both

### Sentence Count Guardrail

Enforces minimum and/or maximum sentence count limits on request prompts or LLM responses.

Configuration:
- **Min sentences** / **Max sentences**
- **Direction**: Request, response, or both

---

## Rate Limiting

Rate limits cap how many requests or how much of a resource a consumer can use in a time window.

### Token-Based Rate Limit

Limits the number of LLM tokens (input + output) consumed in a rolling time window.

**Use when:** You want to control LLM spend and prevent runaway token usage per user or application.

Configuration:
- **Token limit**: Maximum tokens per window
- **Window**: Time window (`minute`, `hour`, `day`)
- **Key**: What to rate-limit by (API key, application, IP address)

### Cost-Based Rate Limit

Limits the estimated USD cost of LLM calls in a rolling time window, based on per-model token pricing.

**Use when:** You have per-team or per-application spending budgets.

Configuration:
- **Cost limit**: Maximum spend (USD) per window
- **Window**: Time window
- **Key**: What to rate-limit by

### Basic Rate Limit

Limits the number of HTTP requests in a rolling time window, regardless of token consumption.

**Use when:** You need simple request-count throttling (e.g. to protect against abuse).

Configuration:
- **Request limit**: Maximum requests per window
- **Window**: Time window
- **Key**: What to rate-limit by

---

## Other Policies

### Model Round Robin

Distributes requests across multiple models (of the same or different providers) using a round-robin strategy. Useful for balancing load or testing model variations.

Configuration:
- **Models**: Ordered list of model IDs to rotate through

### Semantic Cache

Caches LLM responses and returns the cached response for semantically similar future prompts (above a configurable similarity threshold), avoiding redundant LLM calls.

**Use when:** Your workload includes repeated or near-identical prompts (e.g. FAQ bots, repeated code-review tasks).

Configuration:
- **Similarity threshold**: Minimum cosine similarity for a cache hit (0.0–1.0)
- **TTL**: Cache entry lifetime

### Prompt Template

Wraps the user's prompt in a fixed template before sending it to the LLM. The user's input is injected into a `{{prompt}}` placeholder in the template.

**Use when:** You need to ensure every request carries a system instruction or format requirement, regardless of what the consumer sends.

Configuration:
- **Template**: String containing `{{prompt}}` where user input is inserted

### Prompt Decorator

Prepends and/or appends fixed text to every request prompt, without replacing the original.

**Use when:** You need to add a system prefix (e.g. "You are a helpful assistant for Acme Corp.") or suffix to every prompt.

Configuration:
- **Prepend**: Text added before the user's prompt
- **Append**: Text added after the user's prompt

---

## Attaching Policies

Policies are attached from the LLM Provider or LLM Proxy configuration pages. Navigate to the **Guardrails** or **Rate Limiting** tab of the resource and select or configure the policy.

Policy chains execute in the following order for each request:
1. Access control
2. Authentication
3. Rate limiting
4. Guardrails (request direction)
5. → LLM upstream call
6. Guardrails (response direction)
7. Response policies (semantic cache write, etc.)

## Writing Custom Policies

For self-hosted gateway deployments, you can write custom policies in Go using the Policy SDK. Custom policies are compiled into the gateway policy engine. See [`sdk/gateway/policy/v1alpha/`](../../sdk/gateway/policy/v1alpha/) for the SDK interfaces.
