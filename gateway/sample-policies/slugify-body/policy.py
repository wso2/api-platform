# ==============================================================================
# Slugify Body Policy — policy.py
# ==============================================================================
#
# This is the **main entry point** for the slugify-body Python policy. The
# gateway's Python executor discovers and loads this file by convention: it
# looks for a `policy.py` module that exports a module-level factory function
# called `get_policy(metadata, params) -> Policy`.
#
# WHAT THIS POLICY DOES:
#   1. Receives the full buffered JSON request body.
#   2. Extracts a string field at the configured JSONPath (e.g. "$.title").
#   3. Converts that string into a URL-friendly slug using `python-slugify`.
#   4. Writes the slug back into the JSON payload.
#   5. Forwards the modified body to the upstream backend service.
#
# This sample is intentionally HEAVILY COMMENTED to serve as a reference for
# future Python policy developers. Every section explains not just *what* the
# code does, but *why* it does it and how it fits into the SDK contract.
#
# POLICY LIFECYCLE:
#   ┌─────────────────────────────────────────────────────────────────────────┐
#   │  1. Gateway starts → Python executor imports this module.              │
#   │  2. The executor calls get_policy(metadata, params) to create a        │
#   │     policy instance. This is the FACTORY FUNCTION (see Section 4).     │
#   │  3. The executor calls mode() on the returned instance to determine    │
#   │     which phases the policy participates in.                           │
#   │  4. The gateway buffers the request body (because we declared          │
#   │     BodyProcessingMode.BUFFER) and calls on_request_body().            │
#   │  5. The policy returns one of:                                         │
#   │     • UpstreamRequestModifications — forward a modified body           │
#   │     • ImmediateResponse — short-circuit with an error/response         │
#   │     • None — pass through the original request unchanged               │
#   └─────────────────────────────────────────────────────────────────────────┘
# ==============================================================================

# ==========================================================================
# SECTION 1: Imports
# ==========================================================================
#
# In a Python policy, you will typically import from two places:
#   1. `apip_sdk_core` — the official SDK that provides:
#      • Policy base classes (RequestPolicy, ResponsePolicy, etc.)
#      • Processing mode enums (BodyProcessingMode, HeaderProcessingMode)
#      • Action dataclasses (UpstreamRequestModifications, ImmediateResponse)
#      • Context types (ExecutionContext, RequestContext, etc.)
#   2. Your own third-party dependencies listed in `requirements.txt`.
#
# NOTE: The SDK (`apip-sdk-core`) is pre-installed in the gateway runtime.
#       You do NOT need to list it in requirements.txt.
# ==========================================================================

from __future__ import annotations  # Enable PEP 604 union syntax (X | Y) on Python 3.10

import json      # Standard library — for parsing and serialising JSON bodies
import logging   # Standard library — for structured logging within the policy
from typing import Any  # Standard library — for type annotations

# ---------------------------------------------------------------------------
# SDK Imports — the core building blocks for every Python policy
# ---------------------------------------------------------------------------
# • BodyProcessingMode : Enum that tells the gateway HOW to deliver the body.
#     - SKIP   : Don't read the body at all (header-only policies).
#     - BUFFER : Read the entire body into memory, then call on_request_body().
#     - STREAM : Deliver the body chunk-by-chunk via on_request_body_chunk().
#
# • ExecutionContext : Metadata about the current execution, including:
#     - request_id   : Unique ID for this request (useful for logging/tracing).
#     - phase        : Which pipeline phase is currently executing.
#     - deadline     : Optional deadline for time-sensitive operations.
#     - trace_id     : Distributed tracing ID (OpenTelemetry-compatible).
#     - is_cancelled(): Check if the request was cancelled by the client.
#
# • ImmediateResponse : Return this to short-circuit the request pipeline and
#   send a synthetic response directly back to the client (e.g. 400 Bad Request).
#
# • ProcessingMode : Dataclass that declares which pipeline phases (request
#   headers, request body, response headers, response body) this policy
#   participates in. The gateway uses this to skip unnecessary work.
#
# • RequestAction : Union type alias for all valid return types from
#   on_request_body(). It is: UpstreamRequestModifications | ImmediateResponse | None
#
# • RequestContext : The context object passed to on_request_body(). Contains:
#     - headers  : Read-only multi-value header wrapper (case-insensitive).
#     - body     : The buffered request body (Body dataclass with .content bytes).
#     - path     : The request path (e.g. "/api/v1/articles").
#     - method   : The HTTP method (e.g. "POST", "PUT").
#     - shared   : SharedContext with API metadata, auth context, and cross-policy
#                  mutable metadata bag.
#
# • RequestPolicy : The base class for policies that process buffered request
#   bodies. Subclass this and implement mode() + on_request_body().
#
# • UpstreamRequestModifications : Return this to forward a MODIFIED request to
#   the upstream backend. You can change the body, headers, path, method, etc.
# ---------------------------------------------------------------------------
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

# ---------------------------------------------------------------------------
# Third-party import — python-slugify
# ---------------------------------------------------------------------------
# This is the third-party library declared in requirements.txt. The gateway
# builder installs it into the runtime during the container build.
#
# `slugify()` converts any string into a URL-friendly slug:
#   slugify("Hello World!")     → "hello-world"
#   slugify("Héllo Wörld")     → "hello-world"     (with transliterate=True)
#   slugify("My Title", separator="_") → "my_title"
#
# Full API: https://github.com/un33k/python-slugify
# ---------------------------------------------------------------------------
from slugify import slugify


# ==========================================================================
# SECTION 2: Module-level logger
# ==========================================================================
#
# The Python executor captures log output from the standard `logging` module
# and forwards it to the gateway's structured logging pipeline. Use the
# standard severity levels:
#   • logger.debug()   — verbose tracing (only visible when debug is enabled)
#   • logger.info()    — normal operational messages
#   • logger.warning() — recoverable issues (e.g. malformed input, passthrough)
#   • logger.error()   — unrecoverable errors (policy will likely return an error)
#
# BEST PRACTICE: Always use __name__ as the logger name. This automatically
# includes the module path, making it easy to filter logs per policy.
# ==========================================================================
LOGGER = logging.getLogger(__name__)


# ==========================================================================
# SECTION 3: The Policy Class
# ==========================================================================
#
# NAMING CONVENTION:
#   • The class name should be PascalCase and descriptive.
#   • The class itself is NOT auto-discovered. The executor finds policies
#     via the module-level `get_policy()` factory function (see Section 4),
#     which instantiates and returns the class.
#
# WHICH BASE CLASS TO USE:
#   ┌────────────────────────┬─────────────────────────────────────────────┐
#   │ Base Class             │ When to use                                 │
#   ├────────────────────────┼─────────────────────────────────────────────┤
#   │ RequestHeaderPolicy    │ Modify/inspect request headers only         │
#   │ RequestPolicy          │ Read/modify the full buffered request body  │
#   │ ResponseHeaderPolicy   │ Modify/inspect response headers only        │
#   │ ResponsePolicy         │ Read/modify the full buffered response body │
#   │ StreamingRequestPolicy │ Process request body chunk-by-chunk         │
#   │ StreamingResponsePolicy│ Process response body chunk-by-chunk        │
#   └────────────────────────┴─────────────────────────────────────────────┘
#
# For this policy, we use `RequestPolicy` because we need to:
#   1. Read the entire request body (JSON parsing requires the full payload).
#   2. Modify a field in the body and forward the updated version upstream.
# ==========================================================================
class SlugifyBodyPolicy(RequestPolicy):
    """Slugify a string field in the JSON request body.

    This policy extracts a string value from the request body at a configurable
    JSONPath, converts it to a URL-friendly slug using `python-slugify`, and
    forwards the modified body to the upstream service.

    Parameters (configured in policy-definition.yaml):
        jsonPath      : Dot-notation path to the target string field.
        separator     : Word separator character (default: "-").
        transliterate : Whether to convert Unicode to ASCII (default: true).
        lowercase     : Whether to force lowercase output (default: true).
        maxLength     : Maximum slug length; 0 = unlimited (default: 0).
    """

    # ======================================================================
    # SECTION 3a: mode() — Declare processing requirements
    # ======================================================================
    #
    # The `mode()` method is called ONCE when the policy is loaded. It tells
    # the gateway which pipeline phases this policy participates in. The
    # gateway uses this information to optimise the request processing
    # pipeline — phases set to SKIP are never invoked.
    #
    # AVAILABLE MODES:
    #   For headers:
    #     • HeaderProcessingMode.SKIP    — don't call the header hook
    #     • HeaderProcessingMode.PROCESS — call the header hook
    #
    #   For bodies:
    #     • BodyProcessingMode.SKIP   — don't read the body at all
    #     • BodyProcessingMode.BUFFER — buffer the entire body, then call hook
    #     • BodyProcessingMode.STREAM — deliver body chunk-by-chunk
    #
    # For this policy we only need the buffered request body. We explicitly
    # leave all other modes as SKIP (the default) to minimise overhead.
    # ======================================================================
    def mode(self) -> ProcessingMode:
        """Declare that this policy needs the full buffered request body.

        Returns:
            ProcessingMode: Configuration with request_body_mode set to BUFFER.
                All other modes default to SKIP, meaning this policy does NOT
                process request headers, response headers, or response bodies.
        """
        return ProcessingMode(
            # We need the FULL request body buffered in memory so we can parse
            # it as JSON. If we only needed headers, we'd use
            # HeaderProcessingMode.PROCESS on request_header_mode instead.
            request_body_mode=BodyProcessingMode.BUFFER,

            # NOTE: All other modes default to SKIP. We include them here
            # explicitly (commented out) for educational purposes:
            #
            # request_header_mode=HeaderProcessingMode.SKIP,   # default
            # response_header_mode=HeaderProcessingMode.SKIP,  # default
            # response_body_mode=BodyProcessingMode.SKIP,      # default
        )

    # ======================================================================
    # SECTION 3b: on_request_body() — The main policy logic
    # ======================================================================
    #
    # This is the hook the gateway calls with the fully buffered request body.
    # It receives three arguments:
    #
    # 1. execution_ctx (ExecutionContext):
    #    • request_id  — unique request identifier for logging/tracing.
    #    • phase       — the current execution phase (always REQUEST_BODY here).
    #    • deadline    — optional processing deadline.
    #    • trace_id    — distributed tracing header value.
    #    • is_cancelled() — check if the client disconnected.
    #
    # 2. ctx (RequestContext):
    #    • headers     — read-only, case-insensitive header wrapper.
    #                    Use ctx.headers.get("content-type") to read values.
    #    • body        — a Body dataclass with:
    #                      .content (bytes | None) — the raw body bytes.
    #                      .present (bool) — whether a body was sent.
    #                      .end_of_stream (bool) — always True for BUFFER mode.
    #    • path        — the request path (e.g. "/api/v1/articles").
    #    • method      — the HTTP method (e.g. "POST").
    #    • shared      — SharedContext containing:
    #                      .api_name, .api_version, .api_id — API metadata.
    #                      .auth_context — authentication/authorisation state.
    #                      .metadata — mutable dict for cross-policy data sharing.
    #
    # 3. params (dict[str, Any]):
    #    • The user-configured parameters from the API deployment manifest.
    #    • Values are already validated against the JSON Schema defined in
    #      policy-definition.yaml. Default values are applied automatically.
    #    • For this policy: jsonPath, separator, transliterate, lowercase, maxLength.
    #
    # RETURN VALUES (RequestAction — a union type):
    #    • UpstreamRequestModifications — forward a modified request upstream.
    #       Fields: body, headers_to_set, headers_to_remove, path, method, host,
    #               query_parameters_to_add, query_parameters_to_remove,
    #               analytics_metadata, dynamic_metadata.
    #    • ImmediateResponse — short-circuit and send a response to the client.
    #       Fields: status_code, headers, body, analytics_metadata.
    #    • None — pass through the original request without any modifications.
    # ======================================================================
    def on_request_body(
        self,
        execution_ctx: ExecutionContext,
        ctx: RequestContext,
        params: dict[str, Any],
    ) -> RequestAction:
        """Process the buffered request body and slugify the target field.

        Args:
            execution_ctx: Execution metadata (request ID, tracing, deadline).
            ctx: The full request context (headers, body, path, method, shared).
            params: User-configured parameters from policy-definition.yaml.

        Returns:
            - UpstreamRequestModifications with the slugified body on success.
            - ImmediateResponse (400) if the body is not valid JSON.
            - ImmediateResponse (422) if the target field is missing or not a string.
            - None if no body is present (passthrough).
        """
        # ------------------------------------------------------------------
        # STEP 1: Guard — check that a request body is actually present.
        # ------------------------------------------------------------------
        # The gateway sets ctx.body to None or ctx.body.present to False when
        # the client sends a request without a body (e.g. a GET request that
        # somehow has this policy attached). In such cases, we simply pass
        # through without modification.
        if ctx.body is None or not ctx.body.present or ctx.body.content is None:
            LOGGER.info(
                "[SlugifyBody] request_id=%s — No request body present, passing through.",
                execution_ctx.request_id,
            )
            # Returning None means "don't modify anything; forward as-is".
            return None

        # ------------------------------------------------------------------
        # STEP 2: Extract the raw body bytes.
        # ------------------------------------------------------------------
        # ctx.body.content is `bytes`. For JSON parsing, we decode to a string.
        raw_body: bytes = ctx.body.content

        # ------------------------------------------------------------------
        # STEP 3: Parse the body as JSON.
        # ------------------------------------------------------------------
        # If the body is not valid JSON, we return an ImmediateResponse with a
        # 400 status code. This short-circuits the request pipeline — the
        # upstream backend never sees this request.
        try:
            payload: dict[str, Any] = json.loads(raw_body)
        except (json.JSONDecodeError, TypeError, ValueError) as exc:
            LOGGER.warning(
                "[SlugifyBody] request_id=%s — Request body is not valid JSON: %s",
                execution_ctx.request_id,
                exc,
            )
            # ----- Returning an ImmediateResponse -----
            # This immediately sends a response back to the client without
            # forwarding to the upstream backend. Useful for validation errors.
            #
            # Fields:
            #   status_code : HTTP status code (int).
            #   headers     : Response headers (dict[str, str]).
            #   body        : Response body (bytes | None).
            return ImmediateResponse(
                status_code=400,
                body=json.dumps({
                    "error": "invalid_request_body",
                    "message": "Request body must be valid JSON.",
                }).encode("utf-8"),
                headers={"content-type": "application/json"},
            )

        # ------------------------------------------------------------------
        # STEP 4: Read parameters from the `params` dict.
        # ------------------------------------------------------------------
        # Parameters are supplied by the API deployer in the deployment manifest
        # and validated by the gateway against the JSON Schema in
        # policy-definition.yaml. Default values are applied before they reach
        # this code, so we can safely use .get() with the same defaults as a
        # safety net.
        json_path: str = params.get("jsonPath", "$.title")
        separator: str = params.get("separator", "-")
        transliterate: bool = params.get("transliterate", True)
        lowercase: bool = params.get("lowercase", True)
        # NOTE: JSON deserialization may deliver integers as floats; the
        # slugify library's smart_truncate does string[:max_length] which
        # requires a Python int. Always cast to int explicitly.
        max_length: int = int(params.get("maxLength", 0))

        LOGGER.debug(
            "[SlugifyBody] request_id=%s — Parameters: jsonPath=%s, separator=%s, "
            "transliterate=%s, lowercase=%s, maxLength=%d",
            execution_ctx.request_id,
            json_path,
            separator,
            transliterate,
            lowercase,
            max_length,
        )

        # ------------------------------------------------------------------
        # STEP 5: Resolve the JSONPath and extract the target value.
        # ------------------------------------------------------------------
        # We use a simple dot-notation resolver (defined below). For production
        # policies that need full JSONPath support, consider using a library
        # like `jsonpath-ng`.
        target_value = _resolve_json_path(payload, json_path)

        # If the path doesn't resolve or the value is not a string, return a
        # 422 Unprocessable Entity response.
        if target_value is None:
            LOGGER.warning(
                "[SlugifyBody] request_id=%s — JSONPath '%s' did not resolve to any value.",
                execution_ctx.request_id,
                json_path,
            )
            return ImmediateResponse(
                status_code=422,
                body=json.dumps({
                    "error": "field_not_found",
                    "message": f"JSONPath '{json_path}' did not resolve to a value in the request body.",
                }).encode("utf-8"),
                headers={"content-type": "application/json"},
            )

        if not isinstance(target_value, str):
            LOGGER.warning(
                "[SlugifyBody] request_id=%s — Value at '%s' is %s, expected string.",
                execution_ctx.request_id,
                json_path,
                type(target_value).__name__,
            )
            return ImmediateResponse(
                status_code=422,
                body=json.dumps({
                    "error": "invalid_field_type",
                    "message": f"Value at '{json_path}' must be a string, got {type(target_value).__name__}.",
                }).encode("utf-8"),
                headers={"content-type": "application/json"},
            )

        # ------------------------------------------------------------------
        # STEP 6: Perform the slugification using the third-party library.
        # ------------------------------------------------------------------
        # This is where the `python-slugify` library (imported at the top as
        # `slugify`) does the heavy lifting. It handles:
        #   • Unicode transliteration (e.g. "ü" → "u")
        #   • Whitespace and special character replacement
        #   • Consecutive separator collapsing
        #   • Optional case conversion and length truncation
        #
        # The `slugify()` function signature (relevant parameters):
        #   slugify(
        #       text,            — the input string
        #       separator="-",   — word separator
        #       lowercase=True,  — force lowercase output
        #       max_length=0,    — truncate slug (0 = unlimited)
        #   )
        slug: str = slugify(
            text=target_value,
            separator=separator,
            lowercase=lowercase,
            max_length=max_length if max_length > 0 else 0,
        )

        LOGGER.info(
            "[SlugifyBody] request_id=%s — Slugified '%s' at '%s' → '%s'",
            execution_ctx.request_id,
            # Truncate long input values in logs to avoid log flooding.
            target_value[:80] + ("..." if len(target_value) > 80 else ""),
            json_path,
            slug,
        )

        # ------------------------------------------------------------------
        # STEP 7: Write the slug back into the payload.
        # ------------------------------------------------------------------
        _set_json_path(payload, json_path, slug)

        # ------------------------------------------------------------------
        # STEP 8: Serialise and return the modified body.
        # ------------------------------------------------------------------
        # We return an `UpstreamRequestModifications` dataclass. The gateway
        # will forward this modified body to the upstream backend service.
        #
        # IMPORTANT: The `body` field must be `bytes`, not `str`. Always
        # encode with UTF-8.
        #
        # OTHER FIELDS YOU CAN SET (all optional):
        #   • headers_to_set    — dict[str, str] of headers to add/overwrite.
        #   • headers_to_remove — list[str] of header names to strip.
        #   • path              — override the upstream request path.
        #   • method            — override the HTTP method.
        #   • host              — override the Host header / upstream host.
        #   • query_parameters_to_add    — dict[str, list[str]]
        #   • query_parameters_to_remove — list[str]
        #   • analytics_metadata  — dict for policy analytics data.
        #   • dynamic_metadata    — dict for Envoy dynamic metadata.
        modified_body: bytes = json.dumps(payload, ensure_ascii=False).encode("utf-8")

        return UpstreamRequestModifications(
            # The modified JSON body with the slugified field.
            body=modified_body,
            # Update Content-Length to match the new body size, since
            # slugification may change the byte length.
            headers_to_set={
                "content-length": str(len(modified_body)),
            },
        )


# ==========================================================================
# SECTION 4: The get_policy() factory function  (REQUIRED)
# ==========================================================================
#
# Every Python policy module MUST export a module-level function called
# `get_policy`. The Python executor calls this function at startup to
# instantiate the policy. If this function is missing, the executor will
# log an error and skip the policy entirely:
#
#   "Policy module ... does not export 'get_policy' factory"
#
# SIGNATURE:
#   def get_policy(metadata, params) -> Policy
#
# ARGUMENTS:
#   metadata — a PolicyMetadata dataclass containing deployment context:
#              • route_name    — the route this policy instance is attached to.
#              • api_id        — the API identifier.
#              • api_name      — the human-readable API name.
#              • api_version   — the deployed API version.
#              • attached_to   — the attachment point (e.g. "request").
#
#   params   — a dict[str, Any] of user-configured parameters from the API
#              deployment manifest. These have already been validated against
#              the JSON Schema in policy-definition.yaml.
#
# RETURN VALUE:
#   Must return an instance of a class that extends one of the SDK Policy
#   base classes (RequestPolicy, ResponsePolicy, etc.).
#
# TIPS:
#   • Use this function to pre-process or normalise parameters before
#     passing them to the policy constructor.
#   • You can also use it for one-time expensive setup (loading models,
#     connecting to caches, etc.) that should happen once per policy
#     instance rather than once per request.
#   • The function name MUST be exactly `get_policy` — it is not
#     configurable.
# ==========================================================================
def get_policy(metadata, params):
    """Factory function — called by the Python executor to create the policy.

    This is the **required entry point** for every Python policy. The executor
    imports this module and calls ``get_policy(metadata, params)`` to obtain a
    ``Policy`` instance.

    Args:
        metadata: PolicyMetadata with deployment context (route, API info).
        params: dict[str, Any] of validated user-configured parameters.

    Returns:
        An instance of SlugifyBodyPolicy ready to process requests.
    """
    # For this simple policy, we just pass params through. More complex
    # policies might normalise params, pre-build caches, or load ML models
    # here (see prompt-compressor for an example with parameter normalisation).
    LOGGER.info(
        "[SlugifyBody] Factory: creating policy instance for route=%s, api=%s",
        getattr(metadata, "route_name", "unknown"),
        getattr(metadata, "api_name", "unknown"),
    )
    return SlugifyBodyPolicy()


# ==========================================================================
# SECTION 5: Helper functions
# ==========================================================================
#
# These are internal utility functions. They are NOT part of the SDK contract.
# You can structure your helpers however you like — in the same file, in a
# separate module, or even in a sub-package.
# ==========================================================================


def _resolve_json_path(obj: Any, path: str) -> Any:
    """Resolve a simple dot-notation JSONPath and return the value.

    This is a minimal JSONPath resolver that supports only simple dot-separated
    keys. For production policies that need array indexing, wildcards, or
    recursive descent, consider using a library like ``jsonpath-ng``.

    Supported paths:
        "$.title"            → obj["title"]
        "$.article.headline" → obj["article"]["headline"]
        "$.data.name"        → obj["data"]["name"]

    Args:
        obj: The parsed JSON object (usually a dict).
        path: A dot-notation path starting with "$." (e.g. "$.title").

    Returns:
        The value at the path, or None if any segment is missing.
    """
    # Strip the leading "$." prefix. If the path doesn't start with "$.", we
    # still try to resolve it as-is for flexibility.
    if path.startswith("$."):
        path = path[2:]

    # Walk the object tree segment by segment.
    current = obj
    for segment in path.split("."):
        if isinstance(current, dict) and segment in current:
            current = current[segment]
        else:
            # Path segment not found — return None to signal a miss.
            return None

    return current


def _set_json_path(obj: Any, path: str, value: Any) -> None:
    """Set a value at a simple dot-notation JSONPath.

    Walks the object tree to the parent of the target field and sets the value.
    Assumes the path is valid and all intermediate segments exist (i.e.,
    ``_resolve_json_path`` returned a non-None value for this path).

    Args:
        obj: The parsed JSON object (usually a dict) to modify in-place.
        path: A dot-notation path starting with "$." (e.g. "$.title").
        value: The new value to set at the path.
    """
    # Strip the leading "$." prefix.
    if path.startswith("$."):
        path = path[2:]

    segments = path.split(".")

    # Walk to the PARENT of the target field.
    current = obj
    for segment in segments[:-1]:
        current = current[segment]

    # Set the target field to the new value.
    current[segments[-1]] = value
