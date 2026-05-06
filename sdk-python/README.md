# APIP SDK Core

[![PyPI version](https://img.shields.io/pypi/v/apip-sdk-core)](https://pypi.org/project/apip-sdk-core/)
[![Python versions](https://img.shields.io/pypi/pyversions/apip-sdk-core)](https://pypi.org/project/apip-sdk-core/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue)](https://github.com/wso2/api-platform/blob/main/LICENSE)

Python interfaces for writing **gateway policies** on the [WSO2 API Platform](https://github.com/wso2/api-platform).

A policy is a plain Python class that the gateway executor loads at runtime. It receives a rich, fully-typed execution context and returns a structured action that the gateway applies to the live request or response—no configuration files, no generated stubs, no runtime dependencies.

---

## Contents

- [Requirements](#requirements)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Policy types](#policy-types)
  - [RequestHeaderPolicy](#requestheaderpolicy)
  - [RequestPolicy](#requestpolicy)
  - [ResponseHeaderPolicy](#responseheaderpolicy)
  - [ResponsePolicy](#responsepolicy)
  - [StreamingRequestPolicy](#streamingrequestpolicy)
  - [StreamingResponsePolicy](#streamingresponsepolicy)
- [Processing mode](#processing-mode)
- [Actions reference](#actions-reference)
- [Context types](#context-types)
- [Versioned imports](#versioned-imports)
- [Development](#development)
- [License](#license)

---

## Requirements

- Python **3.10** or newer
- No third-party runtime dependencies

---

## Installation

```bash
pip install apip-sdk-core
```

---

## Quick start

### 1 · Add a request header

```python
from typing import Any
from apip_sdk_core import (
    ExecutionContext,
    HeaderProcessingMode,
    ProcessingMode,
    RequestHeaderAction,
    RequestHeaderContext,
    RequestHeaderPolicy,
    UpstreamRequestHeaderModifications,
)


class AddCorrelationId(RequestHeaderPolicy):
    """Injects an X-Correlation-ID header into every upstream request."""

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
        return UpstreamRequestHeaderModifications(
            headers_to_set={"x-correlation-id": execution_ctx.request_id},
        )
```

### 2 · Validate and rewrite a buffered request body

```python
import json
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


class EnforceJsonSchema(RequestPolicy):
    """Rejects requests that are not valid JSON objects."""

    def mode(self) -> ProcessingMode:
        return ProcessingMode(request_body_mode=BodyProcessingMode.BUFFER)

    def on_request_body(
        self,
        execution_ctx: ExecutionContext,
        ctx: RequestContext,
        params: dict[str, Any],
    ) -> RequestAction:
        raw = ctx.body.content if ctx.body else None
        try:
            data = json.loads(raw or b"{}")
        except json.JSONDecodeError:
            return ImmediateResponse(
                status_code=400,
                body=b'{"error":"request body must be valid JSON"}',
                headers={"content-type": "application/json"},
            )
        # Optionally rewrite the body before forwarding
        return UpstreamRequestModifications(body=json.dumps(data).encode())
```

### 3 · Stream and log response chunks

```python
import logging
from typing import Any
from apip_sdk_core import (
    BodyProcessingMode,
    ExecutionContext,
    ForwardResponseChunk,
    ProcessingMode,
    ResponseContext,
    ResponseAction,
    ResponseStreamContext,
    StreamBody,
    StreamingResponseAction,
    StreamingResponsePolicy,
    UpstreamRequestModifications,
)

logger = logging.getLogger(__name__)


class StreamingLogger(StreamingResponsePolicy):
    """Logs every streamed response chunk and passes it through unchanged."""

    def mode(self) -> ProcessingMode:
        return ProcessingMode(response_body_mode=BodyProcessingMode.STREAM)

    def needs_more_response_data(self, accumulated: bytes) -> bool:
        return False  # dispatch every chunk immediately

    def on_response_body(
        self,
        execution_ctx: ExecutionContext,
        ctx: ResponseContext,
        params: dict[str, Any],
    ) -> ResponseAction:
        return None  # fallback – never called in STREAM mode

    def on_response_body_chunk(
        self,
        execution_ctx: ExecutionContext,
        ctx: ResponseStreamContext,
        chunk: StreamBody,
        params: dict[str, Any],
    ) -> StreamingResponseAction:
        logger.debug(
            "chunk #%d  eos=%s  bytes=%d",
            chunk.index,
            chunk.end_of_stream,
            len(chunk.chunk),
        )
        return ForwardResponseChunk(body=chunk.chunk)
```

---

## Policy types

All policy interfaces live in `apip_sdk_core` (re-exported from `apip_sdk_core.policy.v1alpha2`).

| Interface | Hook called | Typical use |
|---|---|---|
| `RequestHeaderPolicy` | `on_request_headers` | Auth, routing, header injection |
| `RequestPolicy` | `on_request_body` | Body validation, transformation |
| `ResponseHeaderPolicy` | `on_response_headers` | CORS, response header rewriting |
| `ResponsePolicy` | `on_response_body` | Response body transformation |
| `StreamingRequestPolicy` | `on_request_body_chunk` | Streaming body inspection |
| `StreamingResponsePolicy` | `on_response_body_chunk` | Streaming response inspection / SSE |

Every interface extends the base `Policy` class:

```python
class Policy(ABC):
    @abstractmethod
    def mode(self) -> ProcessingMode: ...

    def close(self) -> None: ...   # optional – release resources
```

### RequestHeaderPolicy

Called **before** the request body is read.

```python
class RequestHeaderPolicy(Policy, ABC):
    @abstractmethod
    def on_request_headers(
        self,
        execution_ctx: ExecutionContext,
        ctx: RequestHeaderContext,
        params: dict[str, Any],
    ) -> RequestHeaderAction: ...
```

**Returns** `UpstreamRequestHeaderModifications | ImmediateResponse | None`

### RequestPolicy

Called once with the **complete buffered** request body.

```python
class RequestPolicy(Policy, ABC):
    @abstractmethod
    def on_request_body(
        self,
        execution_ctx: ExecutionContext,
        ctx: RequestContext,
        params: dict[str, Any],
    ) -> RequestAction: ...
```

**Returns** `UpstreamRequestModifications | ImmediateResponse | None`

### ResponseHeaderPolicy

Called **before** the response body is read.

```python
class ResponseHeaderPolicy(Policy, ABC):
    @abstractmethod
    def on_response_headers(
        self,
        execution_ctx: ExecutionContext,
        ctx: ResponseHeaderContext,
        params: dict[str, Any],
    ) -> ResponseHeaderAction: ...
```

**Returns** `DownstreamResponseHeaderModifications | ImmediateResponse | None`

### ResponsePolicy

Called once with the **complete buffered** response body.

```python
class ResponsePolicy(Policy, ABC):
    @abstractmethod
    def on_response_body(
        self,
        execution_ctx: ExecutionContext,
        ctx: ResponseContext,
        params: dict[str, Any],
    ) -> ResponseAction: ...
```

**Returns** `DownstreamResponseModifications | ImmediateResponse | None`

### StreamingRequestPolicy

Extends `RequestPolicy`. Called **chunk-by-chunk** for the request body.

```python
class StreamingRequestPolicy(RequestPolicy, ABC):
    @abstractmethod
    def needs_more_request_data(self, accumulated: bytes) -> bool: ...

    @abstractmethod
    def on_request_body_chunk(
        self,
        execution_ctx: ExecutionContext,
        ctx: RequestStreamContext,
        chunk: StreamBody,
        params: dict[str, Any],
    ) -> StreamingRequestAction: ...
```

**Returns** `ForwardRequestChunk | None`

### StreamingResponsePolicy

Extends `ResponsePolicy`. Called **chunk-by-chunk** for the response body.

```python
class StreamingResponsePolicy(ResponsePolicy, ABC):
    @abstractmethod
    def needs_more_response_data(self, accumulated: bytes) -> bool: ...

    @abstractmethod
    def on_response_body_chunk(
        self,
        execution_ctx: ExecutionContext,
        ctx: ResponseStreamContext,
        chunk: StreamBody,
        params: dict[str, Any],
    ) -> StreamingResponseAction: ...
```

**Returns** `ForwardResponseChunk | TerminateResponseChunk | None`

---

## Processing mode

The `mode()` method declares which pipeline phases your policy wants to be called for. The gateway uses this to skip unnecessary work.

```python
@dataclass(slots=True)
class ProcessingMode:
    request_header_mode:  HeaderProcessingMode = HeaderProcessingMode.SKIP
    request_body_mode:    BodyProcessingMode   = BodyProcessingMode.SKIP
    response_header_mode: HeaderProcessingMode = HeaderProcessingMode.SKIP
    response_body_mode:   BodyProcessingMode   = BodyProcessingMode.SKIP
```

### `HeaderProcessingMode`

| Value | Meaning |
|---|---|
| `SKIP` | Do not invoke the header hook for this policy |
| `PROCESS` | Invoke the header hook |

### `BodyProcessingMode`

| Value | Meaning |
|---|---|
| `SKIP` | Do not read the body for this policy |
| `BUFFER` | Buffer the entire body and call the body hook once |
| `STREAM` | Call the chunk hook for each body segment |

---

## Actions reference

Return an action dataclass (or `None` to pass through unchanged) from your policy hook.

### Request actions

| Dataclass | Effect |
|---|---|
| `UpstreamRequestHeaderModifications` | Modify headers / path / method / query params before forwarding to upstream |
| `UpstreamRequestModifications` | Modify body **and** headers / path / method / query params before forwarding |
| `ImmediateResponse` | Short-circuit the request and return a response to the client immediately |
| `None` | Pass through without modifications |

### Response actions

| Dataclass | Effect |
|---|---|
| `DownstreamResponseHeaderModifications` | Modify response headers before sending to client |
| `DownstreamResponseModifications` | Modify response body and headers before sending to client |
| `ImmediateResponse` | Replace the upstream response with a synthetic one |
| `None` | Pass through without modifications |

### Streaming actions

| Dataclass | Effect |
|---|---|
| `ForwardRequestChunk` | Forward the (optionally rewritten) chunk upstream |
| `ForwardResponseChunk` | Forward the (optionally rewritten) chunk downstream |
| `TerminateResponseChunk` | Send a final chunk downstream and end the stream |
| `None` | Drop the chunk (use with care) |

### `ImmediateResponse` fields

```python
@dataclass(slots=True)
class ImmediateResponse:
    status_code: int = 500
    headers: dict[str, str] = field(default_factory=dict)
    body: bytes | None = None
    analytics_metadata: dict[str, Any] = field(default_factory=dict)
    dynamic_metadata: dict[str, dict[str, Any]] = field(default_factory=dict)
    analytics_header_filter: DropHeaderAction = field(default_factory=DropHeaderAction)
```

---

## Context types

### `ExecutionContext`

Injected into every hook. Contains tracing information and a cancellation check.

```python
@dataclass(slots=True)
class ExecutionContext:
    request_id: str
    phase: ExecutionPhase
    deadline: datetime | None = None
    route_name: str = ""
    policy_name: str = ""
    policy_version: str = ""
    trace_id: str | None = None
    span_id: str | None = None

    def is_cancelled(self) -> bool: ...  # check before expensive operations
```

### `SharedContext`

Accessible via `ctx.shared` on every context object.

```python
@dataclass(slots=True)
class SharedContext:
    project_id: str
    request_id: str
    api_id: str
    api_name: str
    api_version: str
    api_kind: str
    api_context: str
    operation_path: str
    auth_context: AuthContext | None
    metadata: dict[str, Any]  # cross-policy mutable bag
```

### `AuthContext`

Populated when authentication has run upstream of your policy.

```python
@dataclass(slots=True)
class AuthContext:
    authenticated: bool
    authorized: bool
    auth_type: str      # e.g. "JWT", "APIKey"
    subject: str
    issuer: str
    audience: list[str]
    scopes: dict[str, bool]
    credential_id: str
    properties: dict[str, str]
    previous: AuthContext | None  # previous auth layer result
```

### `Headers`

Read-only multi-value header wrapper. All lookups are **case-insensitive**.

```python
headers = ctx.headers

headers.get("content-type")      # -> list[str]
headers.has("authorization")     # -> bool
headers.get_all()                 # -> dict[str, list[str]]

for name, values in headers.iterate():
    print(name, values)
```

---

## Versioned imports

The top-level package always re-exports the **latest stable contract**. Use these imports in most cases:

```python
from apip_sdk_core import RequestPolicy, ProcessingMode, ImmediateResponse
```

If you need to pin to a specific contract version (e.g. in a library):

```python
from apip_sdk_core.policy.v1alpha2 import RequestPolicy, ProcessingMode
```

The current stable contract is `v1alpha2`.

---

## Development

### Build

```bash
python -m build
```

### Run tests

```bash
python -m unittest discover -s tests
```

### Type-check

```bash
mypy src/
```

---

## License

Apache License 2.0 — see [LICENSE](https://github.com/wso2/api-platform/blob/main/LICENSE) for details.

© 2026 WSO2 LLC.
