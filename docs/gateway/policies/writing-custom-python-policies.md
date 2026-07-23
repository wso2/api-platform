# Writing Custom Python Policies

This guide walks you through creating custom **Python policies** for the API Platform Gateway. Python policies let you tap into the rich Python ecosystem — AI/ML libraries, NLP toolkits, data transformation engines — to process API traffic directly at the gateway layer.

> For an overview of the gateway's dual-language policy architecture (Go vs. Python) and guidance on when to choose each language, see [Policy Languages and Runtimes](../policy-languages-and-runtimes.md).

---

## Prerequisites

| Requirement | Details |
|-------------|---------|
| **Python** | 3.10+ (the gateway runtime runs Python 3.10) |
| **`ap` CLI** | The `ap` command-line tool |
| **Docker** | Required by the `ap` CLI to build gateway images |
| **Gateway release archive** | The extracted gateway release (contains `build.yaml`, `docker-compose.yaml`, configs, etc.) |

---

## Policy Directory Structure

A custom Python policy uses the **simple (flat) layout** — three files in a single directory:

```
my-policy/
├── policy-definition.yaml   # Identity, version, and parameter schema
├── policy.py                # Policy implementation (entry point)
└── requirements.txt         # Third-party Python dependencies
```

| File | Required | Purpose |
|------|----------|---------|
| `policy-definition.yaml` | ✅ | Declares the policy name, version, description, and configurable parameter schema |
| `policy.py` | ✅ | Contains the policy class and the `get_policy()` factory function |
| `requirements.txt` | Optional | Lists third-party PyPI packages the policy depends on |

> **Convention:** Place your custom policy directories inside a `policies/` folder in your gateway project. For example: `policies/my-policy/`.

---

## Step 1: Define the Policy — `policy-definition.yaml`

The policy definition is a declarative YAML file that tells the gateway **what** the policy does, **which parameters** it accepts, and how those parameters are validated. The gateway validates incoming parameters against this schema *before* the Python code runs.

```yaml
name: my-policy
version: v1.0.0
displayName: My Policy
description: |
  A brief description of what this policy does.
  This text is rendered as documentation in the management console.

parameters:
  type: object
  additionalProperties: false
  properties:
    myParam:
      type: string
      description: "An example configurable parameter."
      default: "hello"
    enableFeature:
      type: boolean
      description: "Toggle an optional feature on or off."
      default: false
    maxItems:
      type: integer
      description: "Maximum number of items to process."
      default: 100
      minimum: 1
  required: []

systemParameters:
  type: object
  additionalProperties: false
  properties: {}
```

### Field Reference

| Field | Description |
|-------|-------------|
| `name` | Unique policy identifier. Must be **lowercase, kebab-case** (e.g., `my-policy`). Used in API deployment manifests to reference this policy. |
| `version` | Semantic version prefixed with `v` (e.g., `v1.0.0`). The major version is used as the policy version qualifier. |
| `displayName` | Human-readable name shown in the management console. |
| `description` | Multi-line description rendered as documentation. |
| `parameters` | A [JSON Schema (draft-7)](https://json-schema.org/draft-07/json-schema-release-notes) object describing user-configurable parameters. Supports `type`, `default`, `enum`, `minimum`, `maximum`, `pattern`, and other standard JSON Schema keywords. |
| `systemParameters` | JSON Schema for operator-level configuration injected by the gateway runtime (not set per-API). Most policies leave this empty. |

### Tips

- Always include `description` and `default` for every property — this makes the policy usable out-of-the-box with zero configuration.
- Use `additionalProperties: false` to catch typos in parameter names at validation time.
- Use `enum` to restrict values to a known set.
- List truly mandatory parameters in `required`.

---

## Step 2: Implement the Policy — `policy.py`

The `policy.py` file is the main entry point. The gateway's Python executor discovers and loads this file by convention. It must export a module-level factory function called **`get_policy()`**.

### 2.1 Imports

Every Python policy imports from the **`apip_sdk_core`** SDK package, which is **pre-installed** in the gateway runtime — you do **not** need to list it in `requirements.txt`.

```python
from __future__ import annotations

import json
import logging
from typing import Any

from apip_sdk_core import (
    BodyProcessingMode,
    HeaderProcessingMode,
    ExecutionContext,
    ImmediateResponse,
    ProcessingMode,
    RequestAction,
    RequestContext,
    RequestPolicy,
    UpstreamRequestModifications,
)
```

### 2.2 Logger

The Python executor captures output from the standard `logging` module and forwards it to the gateway's structured logging pipeline.

```python
LOGGER = logging.getLogger(__name__)
```

| Level | Use for |
|-------|---------|
| `LOGGER.debug()` | Verbose tracing (only visible when debug is enabled) |
| `LOGGER.info()` | Normal operational messages |
| `LOGGER.warning()` | Recoverable issues (e.g., malformed input, passthrough) |
| `LOGGER.error()` | Unrecoverable errors |

> **Best Practice:** Always use `__name__` as the logger name. This automatically includes the module path, making it easy to filter logs per policy.

---

### 2.3 Choosing a Base Class

The SDK provides six base classes. Choose the one that matches the pipeline phase your policy needs to participate in:

| Base Class | Hook Method | Use When You Need To… |
|------------|-------------|----------------------|
| `RequestHeaderPolicy` | `on_request_headers()` | Inspect or modify **request headers** only (no body access) |
| `RequestPolicy` | `on_request_body()` | Read or modify the **full buffered request body** |
| `ResponseHeaderPolicy` | `on_response_headers()` | Inspect or modify **response headers** only |
| `ResponsePolicy` | `on_response_body()` | Read or modify the **full buffered response body** |
| `StreamingRequestPolicy` | `on_request_body_chunk()` | Process the **request body chunk-by-chunk** (streaming) |
| `StreamingResponsePolicy` | `on_response_body_chunk()` | Process the **response body chunk-by-chunk** (streaming) |

> **Note:** `StreamingRequestPolicy` and `StreamingResponsePolicy` also extend their buffered counterparts, so you must implement the buffered hook as a fallback.

---

### 2.4 The `mode()` Method

Every policy must implement `mode()` to declare which pipeline phases it participates in. The gateway uses this to skip unnecessary work — phases set to `SKIP` are never invoked.

```python
def mode(self) -> ProcessingMode:
    return ProcessingMode(
        request_header_mode=HeaderProcessingMode.SKIP,      # default
        request_body_mode=BodyProcessingMode.BUFFER,         # read full body
        response_header_mode=HeaderProcessingMode.SKIP,      # default
        response_body_mode=BodyProcessingMode.SKIP,          # default
    )
```

**Processing mode options:**

| Mode Type | Values | Behaviour |
|-----------|--------|-----------|
| `HeaderProcessingMode` | `SKIP`, `PROCESS` | Skip or process headers at that phase |
| `BodyProcessingMode` | `SKIP`, `BUFFER`, `STREAM` | Skip the body, buffer it entirely, or deliver it chunk-by-chunk |

> Only set the modes your policy actually needs. Leaving everything else as `SKIP` (the default) minimises per-request overhead.

---

### 2.5 Writing a Request Body Policy

The most common pattern — read the full request body, transform it, and forward upstream.

```python
class MyPolicy(RequestPolicy):
    """Process and transform the buffered request body."""

    def mode(self) -> ProcessingMode:
        return ProcessingMode(
            request_body_mode=BodyProcessingMode.BUFFER,
        )

    def on_request_body(
        self,
        execution_ctx: ExecutionContext,
        ctx: RequestContext,
        params: dict[str, Any],
    ) -> RequestAction:
        # Guard: check that a body is present
        if ctx.body is None or not ctx.body.present or ctx.body.content is None:
            return None  # passthrough — no modification

        # Parse the body
        raw_body: bytes = ctx.body.content
        # ... your transformation logic here ...
        modified_body: bytes = raw_body  # replace with actual transformation

        # Forward the modified body upstream
        return UpstreamRequestModifications(
            body=modified_body,
            headers_to_set={
                "content-length": str(len(modified_body)),
            },
        )
```

---

### 2.6 Writing a Response Body Policy

Intercept and transform the response coming back from the upstream backend before it reaches the client.

```python
from apip_sdk_core import (
    BodyProcessingMode,
    ProcessingMode,
    ExecutionContext,
    ResponseAction,
    ResponseContext,
    ResponsePolicy,
    DownstreamResponseModifications,
    ImmediateResponse,
)

class MyResponsePolicy(ResponsePolicy):
    """Process and transform the buffered response body."""

    def mode(self) -> ProcessingMode:
        return ProcessingMode(
            response_body_mode=BodyProcessingMode.BUFFER,
        )

    def on_response_body(
        self,
        execution_ctx: ExecutionContext,
        ctx: ResponseContext,
        params: dict[str, Any],
    ) -> ResponseAction:
        if ctx.response_body is None or not ctx.response_body.present:
            return None  # passthrough

        raw_body: bytes = ctx.response_body.content
        # ... your transformation logic here ...
        modified_body: bytes = raw_body

        return DownstreamResponseModifications(
            body=modified_body,
            headers_to_set={
                "content-length": str(len(modified_body)),
            },
        )
```

---

### 2.7 Writing a Request Header Policy

Inspect or modify request headers without reading the body. This is the most lightweight policy type.

```python
from apip_sdk_core import (
    HeaderProcessingMode,
    ProcessingMode,
    ExecutionContext,
    RequestHeaderAction,
    RequestHeaderContext,
    RequestHeaderPolicy,
    UpstreamRequestHeaderModifications,
    ImmediateResponse,
)

class MyHeaderPolicy(RequestHeaderPolicy):
    """Inspect and modify request headers."""

    def mode(self) -> ProcessingMode:
        return ProcessingMode(
            request_header_mode=HeaderProcessingMode.PROCESS,
        )

    def on_request_headers(
        self,
        execution_ctx: ExecutionContext,
        ctx: RequestHeaderContext,
        params: dict[str, Any],
    ) -> RequestHeaderAction:
        # Read a header value
        auth_values = ctx.headers.get("authorization")

        # Add or overwrite headers
        return UpstreamRequestHeaderModifications(
            headers_to_set={
                "x-custom-header": "injected-value",
            },
            headers_to_remove=["x-unwanted-header"],
        )
```

---

### 2.8 Writing a Response Header Policy

Modify response headers before they reach the client.

```python
from apip_sdk_core import (
    HeaderProcessingMode,
    ProcessingMode,
    ExecutionContext,
    ResponseHeaderAction,
    ResponseHeaderContext,
    ResponseHeaderPolicy,
    DownstreamResponseHeaderModifications,
)

class MyResponseHeaderPolicy(ResponseHeaderPolicy):
    """Modify response headers before reaching the client."""

    def mode(self) -> ProcessingMode:
        return ProcessingMode(
            response_header_mode=HeaderProcessingMode.PROCESS,
        )

    def on_response_headers(
        self,
        execution_ctx: ExecutionContext,
        ctx: ResponseHeaderContext,
        params: dict[str, Any],
    ) -> ResponseHeaderAction:
        return DownstreamResponseHeaderModifications(
            headers_to_set={
                "x-processed-by": "my-response-header-policy",
            },
        )
```

---

### 2.9 Writing a Streaming Request Policy

Process the request body chunk-by-chunk for large payloads or real-time processing. You must also implement the buffered `on_request_body()` as a fallback.

```python
from apip_sdk_core import (
    BodyProcessingMode,
    ProcessingMode,
    ExecutionContext,
    RequestAction,
    RequestContext,
    StreamingRequestPolicy,
    RequestStreamContext,
    StreamBody,
    StreamingRequestAction,
    ForwardRequestChunk,
    UpstreamRequestModifications,
)

class MyStreamingRequestPolicy(StreamingRequestPolicy):
    """Process request body chunk-by-chunk."""

    def mode(self) -> ProcessingMode:
        return ProcessingMode(
            request_body_mode=BodyProcessingMode.STREAM,
        )

    def needs_more_request_data(self, accumulated: bytes) -> bool:
        """Return True to accumulate more data before processing."""
        return len(accumulated) < 1024  # accumulate at least 1 KB

    def on_request_body_chunk(
        self,
        execution_ctx: ExecutionContext,
        ctx: RequestStreamContext,
        chunk: StreamBody,
        params: dict[str, Any],
    ) -> StreamingRequestAction:
        # Transform and forward each chunk
        transformed = chunk.chunk.upper()
        return ForwardRequestChunk(body=transformed)

    def on_request_body(
        self,
        execution_ctx: ExecutionContext,
        ctx: RequestContext,
        params: dict[str, Any],
    ) -> RequestAction:
        """Buffered fallback — used when streaming is not possible."""
        if ctx.body is None or not ctx.body.present:
            return None
        return UpstreamRequestModifications(body=ctx.body.content)
```

---

### 2.10 Writing a Streaming Response Policy

Process the response body chunk-by-chunk. Similar pattern to the streaming request policy.

```python
from apip_sdk_core import (
    BodyProcessingMode,
    ProcessingMode,
    ExecutionContext,
    ResponseAction,
    ResponseContext,
    StreamingResponsePolicy,
    ResponseStreamContext,
    StreamBody,
    StreamingResponseAction,
    ForwardResponseChunk,
    TerminateResponseChunk,
    DownstreamResponseModifications,
)

class MyStreamingResponsePolicy(StreamingResponsePolicy):
    """Process response body chunk-by-chunk."""

    def mode(self) -> ProcessingMode:
        return ProcessingMode(
            response_body_mode=BodyProcessingMode.STREAM,
        )

    def needs_more_response_data(self, accumulated: bytes) -> bool:
        return len(accumulated) < 512

    def on_response_body_chunk(
        self,
        execution_ctx: ExecutionContext,
        ctx: ResponseStreamContext,
        chunk: StreamBody,
        params: dict[str, Any],
    ) -> StreamingResponseAction:
        if chunk.end_of_stream:
            return TerminateResponseChunk(body=chunk.chunk)
        return ForwardResponseChunk(body=chunk.chunk)

    def on_response_body(
        self,
        execution_ctx: ExecutionContext,
        ctx: ResponseContext,
        params: dict[str, Any],
    ) -> ResponseAction:
        """Buffered fallback."""
        if ctx.response_body is None or not ctx.response_body.present:
            return None
        return DownstreamResponseModifications(body=ctx.response_body.content)
```

---

### 2.11 The `get_policy()` Factory Function

Every Python policy module **must** export a module-level function called `get_policy`. The Python executor calls this at startup to instantiate the policy. If this function is missing, the executor will skip the policy entirely.

```python
def get_policy(metadata, params):
    """Factory function — called by the Python executor to create the policy.

    Args:
        metadata: PolicyMetadata with deployment context.
            - metadata.route_name  — the route this instance is attached to
            - metadata.api_id      — the API identifier
            - metadata.api_name    — the human-readable API name
            - metadata.api_version — the deployed API version
            - metadata.attached_to — the attachment point (e.g., "request")
        params: dict[str, Any] of validated, user-configured parameters.

    Returns:
        An instance of your policy class.
    """
    return MyPolicy()
```

> **Tips:**
> - Use this function for one-time expensive setup — loading ML models, building caches, pre-processing parameters — that should happen once per policy instance rather than once per request.
> - The function name **must** be exactly `get_policy` — it is not configurable.

---

## Step 3: Declare Dependencies — `requirements.txt`

If your policy depends on third-party PyPI packages, list them in `requirements.txt`. The gateway builder reads this file during the container image build and installs the packages into the Python runtime.

```txt
# Third-party dependencies for my-policy.
# The SDK (apip-sdk-core) is pre-installed — do NOT list it here.

requests>=2.31.0,<3.0.0
python-slugify>=8.0.4,<9.0.0
```

### Best Practices

- **Keep the list minimal** — every extra package increases image size and attack surface.
- **Do NOT list `apip-sdk-core`** — it is pre-installed in the runtime.
- **Use bounded version constraints** (e.g., `>=X.Y.Z,<X+1.0.0`) for reproducible builds.
- Test locally with `pip install -r requirements.txt` before building.

> If your policy has no third-party dependencies, you can omit `requirements.txt` entirely.

---

## Return Types Quick Reference

What your hook method returns determines how the gateway processes the request/response:

### Request-Side Returns

| Return Value | Effect |
|--------------|--------|
| `UpstreamRequestModifications(body=..., headers_to_set=..., ...)` | Forward a **modified** request to the upstream backend |
| `UpstreamRequestHeaderModifications(headers_to_set=..., ...)` | Modify request headers only (header policy) |
| `ImmediateResponse(status_code=..., body=..., headers=...)` | **Short-circuit** the pipeline — send a response directly to the client without reaching the backend |
| `None` | **Passthrough** — forward the original request unchanged |

### Response-Side Returns

| Return Value | Effect |
|--------------|--------|
| `DownstreamResponseModifications(body=..., headers_to_set=..., ...)` | Send a **modified** response to the client |
| `DownstreamResponseHeaderModifications(headers_to_set=..., ...)` | Modify response headers only (header policy) |
| `ImmediateResponse(status_code=..., body=..., headers=...)` | Replace the response entirely |
| `None` | **Passthrough** — forward the original response unchanged |

### Streaming Returns

| Return Value | Effect |
|--------------|--------|
| `ForwardRequestChunk(body=...)` | Forward a transformed request chunk |
| `ForwardResponseChunk(body=...)` | Forward a transformed response chunk |
| `TerminateResponseChunk(body=...)` | Forward the final response chunk and signal end-of-stream |
| `None` | Drop the chunk (do not forward) |

---

## Context Objects Reference

### `ExecutionContext`

Available in every hook — provides request-scoped metadata.

| Field | Type | Description |
|-------|------|-------------|
| `request_id` | `str` | Unique identifier for this request |
| `phase` | `ExecutionPhase` | Current pipeline phase (`REQUEST_HEADERS`, `REQUEST_BODY`, etc.) |
| `deadline` | `datetime \| None` | Optional processing deadline |
| `trace_id` | `str \| None` | Distributed tracing ID (OpenTelemetry-compatible) |
| `span_id` | `str \| None` | Current span ID |
| `is_cancelled()` | `bool` | Check if the client disconnected |

### `RequestContext`

Available in `on_request_body()`.

| Field | Type | Description |
|-------|------|-------------|
| `headers` | `Headers` | Read-only, case-insensitive request headers (**current** — may reflect mutations by earlier header-phase policies) |
| `body` | `Body \| None` | Buffered request body (`.content: bytes`, `.present: bool`) |
| `path` | `str` | Request path (e.g., `/api/v1/items`) |
| `method` | `str` | HTTP method (e.g., `POST`) |
| `downstream` | `DownstreamContext \| None` | Snapshot of the **original** client request (`.request.headers: Headers`). `None` on older gateways — see [Validating original headers](#validating-original-headers-downstream--upstream) |
| `upstream` | `UpstreamRequestContext \| None` | Resolved upstream target for this request (`.name: str`, `.url: str`, `.base_path: str`). `None` on older gateways |
| `shared` | `SharedContext` | API metadata, auth context, and cross-policy metadata bag |

### `RequestHeaderContext`

Available in `on_request_headers()`.

| Field | Type | Description |
|-------|------|-------------|
| `headers` | `Headers` | Read-only, case-insensitive request headers (**current**) |
| `path` | `str` | Request path |
| `method` | `str` | HTTP method |
| `authority` | `str` | Request authority |
| `scheme` | `str` | Request scheme (`http`/`https`) |
| `downstream` | `DownstreamContext \| None` | Snapshot of the **original** client request (`.request.headers: Headers`). `None` on older gateways |
| `upstream` | `UpstreamRequestContext \| None` | Resolved upstream target for this request (`.name: str`, `.url: str`, `.base_path: str`). `None` on older gateways |
| `shared` | `SharedContext` | API metadata, auth context, and cross-policy metadata bag |

### `ResponseContext`

Available in `on_response_body()`.

| Field | Type | Description |
|-------|------|-------------|
| `request_headers` | `Headers` | Original request headers |
| `request_body` | `Body \| None` | Original request body |
| `request_path` | `str` | Original request path |
| `request_method` | `str` | Original HTTP method |
| `response_headers` | `Headers` | Response headers (**current** — may reflect mutations by earlier header-phase policies) |
| `response_body` | `Body \| None` | Buffered response body |
| `response_status` | `int` | HTTP status code from upstream |
| `downstream` | `DownstreamContext \| None` | Snapshot of the **original** client request (`.request.headers: Headers`). `None` on older gateways |
| `upstream` | `UpstreamResponseContext \| None` | Resolved upstream target plus a snapshot of the **original** upstream response (`.name: str`, `.url: str`, `.base_path: str`, `.response.headers: Headers`). `None` on older gateways |
| `shared` | `SharedContext` | API metadata, auth context, and cross-policy metadata bag |

### `ResponseHeaderContext`

Available in `on_response_headers()`.

| Field | Type | Description |
|-------|------|-------------|
| `request_headers` | `Headers` | Original request headers |
| `response_headers` | `Headers` | Response headers (**current**) |
| `response_status` | `int` | HTTP status code from upstream |
| `downstream` | `DownstreamContext \| None` | Snapshot of the **original** client request (`.request.headers: Headers`). `None` on older gateways |
| `upstream` | `UpstreamResponseContext \| None` | Resolved upstream target plus a snapshot of the **original** upstream response (`.name: str`, `.url: str`, `.base_path: str`, `.response.headers: Headers`). `None` on older gateways |
| `shared` | `SharedContext` | API metadata, auth context, and cross-policy metadata bag |

### `SharedContext`

Available via `ctx.shared` in all context objects — provides API-level metadata and a cross-policy data bag.

| Field | Type | Description |
|-------|------|-------------|
| `api_id` | `str` | API identifier |
| `api_name` | `str` | Human-readable API name |
| `api_version` | `str` | API version |
| `api_kind` | `str` | API kind |
| `api_context` | `str` | API context path |
| `operation_path` | `str` | Matched operation path |
| `request_id` | `str` | Request identifier |
| `auth_context` | `AuthContext \| None` | Authentication and authorisation state |
| `metadata` | `dict[str, Any]` | **Mutable** dictionary for sharing data between policies in the same request pipeline |

### `Headers`

Read-only, case-insensitive header wrapper.

| Method | Returns | Description |
|--------|---------|-------------|
| `get(name)` | `list[str]` | All values for the header |
| `has(name)` | `bool` | Whether the header exists |
| `get_all()` | `dict[str, list[str]]` | Defensive copy of all headers |
| `iterate()` | `Iterator[tuple[str, list[str]]]` | Iterate over `(name, values)` pairs |

### Validating original headers (`downstream` / `upstream`)

The gateway runs **every** policy's header phase before **any** policy's body
phase, and header mutations are applied in place. So if a later policy rewrites
a header during its header phase, a body-phase validator reading `headers` will
see the *rewritten* value — not what the client (or backend) actually sent.

When your policy needs to validate against the **original** value, read from the
snapshot instead:

- `ctx.downstream.request.headers` — the original **client request** headers.
- `ctx.upstream.response.headers` — the original **upstream response** headers
  (response-phase contexts only).

These snapshots are captured before any policy mutation.

**Always nil-check and fall back.** `downstream`/`upstream` — and their nested
`request`/`response` — are `None` on older gateways that predate this feature. A
`None` snapshot means "not available", so fall back to the mutable headers so
your policy keeps working across gateway versions:

```python
def on_request_body(self, ctx: RequestContext, params: dict) -> RequestAction | None:
    # Prefer the pristine client headers; fall back on older gateways.
    headers = ctx.headers
    if (
        ctx.downstream is not None
        and ctx.downstream.request is not None
        and ctx.downstream.request.headers is not None
    ):
        headers = ctx.downstream.request.headers

    token = headers.get("authorization")  # exactly what the client sent
    if not token or not self._valid(token[0]):
        return ImmediateResponse(
            status_code=401,
            headers={"content-type": "application/json"},
            body=b'{"error":"unauthorized","message":"Invalid or expired credentials."}',
        )
    return None
```

For a response-phase policy validating a header the **backend** set, use
`ctx.upstream.response.headers` with the same nil-check pattern (guarding
`ctx.upstream`, `ctx.upstream.response`, then `.headers`).

> **Note:** `headers` and `downstream.request.headers` are identical unless
> another policy actually mutated the header, so test the mutate-then-validate
> ordering explicitly — not just the happy path.

---

## Step 4: Register and Build

### 4.1 Register the Policy in `build.yaml`

Add your custom policy to the `policies` section of your gateway project's `build.yaml` using the `filePath` field:

```yaml
version: v1
gateway:
  version: 1.1.0
policies:
  # ... existing policies ...

  # Your custom Python policy (local)
  - name: my-policy
    filePath: policies/my-policy
```

> **Important:**
> - The `name` must exactly match the `name` field in your `policy-definition.yaml`.
> - The `filePath` is relative to the directory containing `build.yaml`.
> - The builder **auto-detects** the policy language by examining the directory contents — if it finds `.py` files (and no `go.mod`), it treats the policy as Python. You do not need to specify the language explicitly.

### 4.2 Build the Gateway Image

Run the `ap` CLI to build a custom gateway image that includes your policy:

```shell
ap gateway image build
```

The builder will:

1. **Discover** your policy from the `filePath` and auto-detect it as Python.
2. **Validate** the directory structure (`policy.py` and `policy-definition.yaml` must exist).
3. **Parse** `policy-definition.yaml` and verify the name matches `build.yaml`.
4. **Install** dependencies from `requirements.txt` into the Python runtime.
5. **Register** the policy in the gateway's policy registry.
6. **Build** a Docker image containing your custom policy alongside all other declared policies.

### 4.3 Run the Gateway

Start the gateway using `docker-compose`:

```shell
docker compose up -d
```

Your custom Python policy is now available and can be attached to APIs through the gateway's API deployment manifests.

---

## Complete Example

Here is a minimal, end-to-end Python policy that converts a JSON field to uppercase:

**`policies/my-policy/policy-definition.yaml`**

```yaml
name: my-policy
version: v1.0.0
displayName: My Policy
description: |
  Converts the value at the configured JSON field to uppercase.

parameters:
  type: object
  additionalProperties: false
  properties:
    fieldName:
      type: string
      description: "The JSON field name to transform."
      default: "title"
  required: []

systemParameters:
  type: object
  additionalProperties: false
  properties: {}
```

**`policies/my-policy/policy.py`**

```python
from __future__ import annotations

import json
import logging
from typing import Any

from apip_sdk_core import (
    BodyProcessingMode,
    ExecutionContext,
    ImmediateResponse,
    ProcessingMode,
    RequestAction,
    RequestContext,
    RequestPolicy,
    UpstreamRequestModifications,
)

LOGGER = logging.getLogger(__name__)


class MyPolicy(RequestPolicy):
    """Convert a JSON field value to uppercase."""

    def mode(self) -> ProcessingMode:
        return ProcessingMode(
            request_body_mode=BodyProcessingMode.BUFFER,
        )

    def on_request_body(
        self,
        execution_ctx: ExecutionContext,
        ctx: RequestContext,
        params: dict[str, Any],
    ) -> RequestAction:
        if ctx.body is None or not ctx.body.present or ctx.body.content is None:
            return None

        try:
            payload = json.loads(ctx.body.content)
        except (json.JSONDecodeError, TypeError):
            return ImmediateResponse(
                status_code=400,
                body=json.dumps({"error": "Request body must be valid JSON."}).encode(),
                headers={"content-type": "application/json"},
            )

        field_name = params.get("fieldName", "title")
        if field_name in payload and isinstance(payload[field_name], str):
            payload[field_name] = payload[field_name].upper()

        modified = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        return UpstreamRequestModifications(
            body=modified,
            headers_to_set={"content-length": str(len(modified))},
        )


def get_policy(metadata, params):
    LOGGER.info("Creating MyPolicy for route=%s", getattr(metadata, "route_name", "unknown"))
    return MyPolicy()
```

> **Note:** This example has no third-party dependencies, so `requirements.txt` is not needed.

---

## Best Practices

| Area | Recommendation |
|------|----------------|
| **Body encoding** | The `body` field in all modification dataclasses expects `bytes`, not `str`. Always `.encode("utf-8")` before returning. |
| **Content-Length** | When modifying a body, update the `content-length` header to match the new byte length. |
| **Parameter casting** | JSON deserialization may deliver integers as floats. Always cast explicitly (e.g., `int(params.get("maxLength", 0))`). |
| **Error handling** | Return `ImmediateResponse` with appropriate status codes (`400`, `422`, etc.) for validation failures. Never let unhandled exceptions propagate. |
| **Logging** | Use the standard `logging` module with `__name__`. Avoid logging sensitive request content — log metadata (lengths, field names) instead. |
| **Dependencies** | Keep `requirements.txt` minimal. Use bounded version constraints. Do not list `apip-sdk-core`. |
| **Naming** | Policy `name` must be lowercase, kebab-case. The class name should be PascalCase. |
| **Passthrough** | Return `None` when the policy has nothing to do (e.g., no body present, wrong content type). This avoids unnecessary overhead. |
| **Cross-policy data** | Use `ctx.shared.metadata` (a mutable `dict`) to pass data between policies in the same request pipeline. |
| **Resource cleanup** | Override `close()` on your policy class if you hold resources (connections, file handles, model instances) that need cleanup. |

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Policy not loading | Missing `get_policy()` function | Ensure `policy.py` exports a module-level `get_policy(metadata, params)` function |
| `ModuleNotFoundError` for a third-party package | Package not in `requirements.txt` | Add the package to `requirements.txt` and rebuild |
| Name mismatch error during build | `name` in `build.yaml` ≠ `name` in `policy-definition.yaml` | Make both names identical |
| `500` errors at runtime | Unhandled exception in policy code | Add try/except blocks and return `ImmediateResponse` for expected errors |
| Parameter type mismatch | JSON delivers numbers as floats | Explicitly cast with `int()`, `str()`, etc. |
| Policy not detected as Python | Directory contains a `go.mod` file | Remove `go.mod` — the builder prioritises Go when `go.mod` is present |

---

## Related Documentation

- [Policy Languages and Runtimes](../policy-languages-and-runtimes.md) — Dual-language architecture overview and language selection guide
- [Customizing the Gateway by Adding and Removing Policies](../../cli/customizing-gateway-policies.md) — `ap gateway image build` CLI reference and `build.yaml` format
