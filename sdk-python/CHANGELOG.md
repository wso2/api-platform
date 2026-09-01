# Changelog

All notable changes to **apip-sdk-core** will be documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added

- `SharedContext.resolved_operation` — the canonical protocol operation the request
  resolved to, for an API kind whose operation cannot be read off the route
  (`operation_path` is identical for every operation on a multiplexed endpoint).
- `SharedContext.resolution_attributes` — the protocol-derived request facts the
  route's resolver captured while identifying that operation, so a policy does not
  re-parse the request body and a request-header-phase policy can read a body-sourced
  value at all.

Both are empty for every API kind released before Agent, so existing policies are
unaffected.

---

## [0.1.0] — 2026-04-23

### Added

- Initial release of the `apip-sdk-core` Python SDK.
- `policy.v1alpha2` contract — a Python mirror of the Go `sdk/core/policy/v1alpha2` package.
- **Policy interfaces**: `Policy`, `RequestHeaderPolicy`, `RequestPolicy`,
  `ResponseHeaderPolicy`, `ResponsePolicy`, `StreamingRequestPolicy`,
  `StreamingResponsePolicy`.
- **Action types**: `UpstreamRequestHeaderModifications`,
  `UpstreamRequestModifications`, `DownstreamResponseHeaderModifications`,
  `DownstreamResponseModifications`, `ImmediateResponse`, `ForwardRequestChunk`,
  `ForwardResponseChunk`, `TerminateResponseChunk`.
- **Context types**: `RequestHeaderContext`, `RequestContext`,
  `ResponseHeaderContext`, `ResponseContext`, `RequestStreamContext`,
  `ResponseStreamContext`, `SharedContext`, `AuthContext`, `ExecutionContext`,
  `PolicyMetadata`.
- **Value types**: `Headers` (case-insensitive, read-only, multi-value),
  `Body`, `StreamBody`, `ProcessingMode`, `HeaderProcessingMode`,
  `BodyProcessingMode`, `ExecutionPhase`.
- `py.typed` marker for full PEP 561 type-checker support.
- Top-level re-exports so `from apip_sdk_core import RequestPolicy` works
  without importing the versioned subpackage.
- `__version__` attribute on the root package.
- Zero runtime dependencies — standard library only.
- Python 3.10–3.13 support.
- Apache 2.0 licence.
