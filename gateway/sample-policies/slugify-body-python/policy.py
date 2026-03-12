# Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""Slugify Body Policy — sample Python policy for the WSO2 API Gateway.

This file is the canonical reference for writing Python gateway policies.
Read it top-to-bottom to understand the full lifecycle:

  1.  get_policy(metadata, params) — factory called once per route at startup
  2.  on_request(ctx, params)      — called on every inbound HTTP request
  3.  on_response(ctx, params)     — called on every outbound HTTP response
  4.  close()                      — called when the route is removed

What this policy does
---------------------
Converts plain-text request bodies into URL-friendly slugs using the
third-party `python-slugify` library.

  "Hello World! How are you?" → "hello-world-how-are-you"

Parameters
----------
separator : str  (default "-")
    Character inserted between words. Common values: "-", "_".
max_length : int  (default 0 = unlimited)
    Truncate the resulting slug to at most this many characters.
"""

import logging
from typing import Any, Dict

# Third-party: pip install python-slugify
# The slug() function converts any unicode string into an ASCII slug.
# It handles accented characters, punctuation, and whitespace automatically.
from slugify import slugify

from sdk.policy import (
    ImmediateResponse,
    Policy,
    PolicyMetadata,
    RequestContext,
    ResponseContext,
    RequestAction,
    ResponseAction,
    UpstreamRequestModifications,
)

# Use a named logger matching the policy name — visible in structured gateway logs
logger = logging.getLogger("slugify-body")


class SlugifyBodyPolicy(Policy):
    """Converts the request body to a URL-safe slug.

    One instance of this class is created per route (see get_policy below).
    Config parsed from params is stored on the instance so on_request is
    fast — no argument parsing on the hot path.
    """

    def __init__(self, separator: str, max_length: int) -> None:
        # Store parsed config as instance attributes.
        # This mirrors the Go pattern of parsing params once in GetPolicy
        # and storing them on the policy struct.
        self._separator = separator
        self._max_length = max_length
        logger.debug(
            "SlugifyBodyPolicy instance created",
            extra={"separator": separator, "max_length": max_length},
        )

    # ------------------------------------------------------------------
    # on_request — request phase (hot path)
    # ------------------------------------------------------------------

    def on_request(self, ctx: RequestContext, params: Dict[str, Any]) -> RequestAction:
        """Slugify the request body.

        Phases:
        1. Guard — skip gracefully if there is no body.
        2. Decode — body bytes arrive as raw bytes; decode to str.
        3. Validate — reject empty text so upstream never receives an empty slug.
        4. Transform — call slugify() from the python-slugify library.
        5. Respond — return the modified body, preserving all other request attrs.
        """
        logger.info("on_request: slugify-body policy executing")

        # ── 1. Guard ────────────────────────────────────────────────────────
        # ctx.body is None when the request has no body (e.g. GET).
        # ctx.body.present is False when Envoy did not buffer (mode mismatch —
        # should not happen if processingMode is set correctly, but be safe).
        if ctx.body is None or not ctx.body.present:
            logger.info("on_request: no body present, passing request through unchanged")
            # Return empty modifications = continue request unchanged
            return UpstreamRequestModifications()

        raw_bytes = ctx.body.content or b""

        # ── 2. Decode ────────────────────────────────────────────────────────
        try:
            text = raw_bytes.decode("utf-8")
        except UnicodeDecodeError as exc:
            logger.warning("on_request: body is not valid UTF-8, rejecting: %s", exc)
            # ImmediateResponse short-circuits the chain and returns directly to
            # the client — the upstream service is never called.
            return ImmediateResponse(
                status_code=400,
                headers={"Content-Type": "text/plain; charset=utf-8"},
                body=b"slugify-body: request body must be valid UTF-8 text",
            )

        logger.debug("on_request: decoded body — %d chars: %r", len(text), text[:80])

        # ── 3. Validate ──────────────────────────────────────────────────────
        if not text.strip():
            logger.warning("on_request: body is blank, rejecting")
            return ImmediateResponse(
                status_code=400,
                headers={"Content-Type": "text/plain; charset=utf-8"},
                body=b"slugify-body: request body must not be empty",
            )

        # ── 4. Transform ─────────────────────────────────────────────────────
        # python-slugify handles unicode → ASCII transliteration, punctuation
        # stripping, whitespace collapsing, and optional max_length truncation.
        # Pass 0 for max_length to mean "no limit" (the library's own convention).
        slug = slugify(
            text,
            separator=self._separator,
            max_length=self._max_length,
        )
        logger.info(
            "on_request: slugified body — %r → %r",
            text[:60],
            slug[:60],
        )

        # ── 5. Respond ───────────────────────────────────────────────────────
        # Return UpstreamRequestModifications with only `body` set.
        # All headers, path, and method are left unchanged.
        return UpstreamRequestModifications(
            body=slug.encode("utf-8"),
        )

    # ------------------------------------------------------------------
    # on_response — response phase
    # ------------------------------------------------------------------

    def on_response(self, ctx: ResponseContext, params: Dict[str, Any]) -> ResponseAction:
        """No response-phase work needed for this policy.

        Return None to signal "pass through unchanged". Equivalent to Go's
        `return nil` in OnResponse.
        """
        logger.debug("on_response: no-op, passing response through unchanged")
        return None

    # ------------------------------------------------------------------
    # close — cleanup (optional)
    # ------------------------------------------------------------------

    def close(self) -> None:
        """Called when the route this instance belongs to is deregistered.

        This policy holds no external resources (no HTTP clients, DB
        connections, or background threads), so nothing needs to be torn down.
        Override this method if your policy opens connections in __init__.
        """
        logger.debug("close: SlugifyBodyPolicy instance released")


# ----------------------------------------------------------------------
# Factory function — the single required export for every Python policy
# ----------------------------------------------------------------------

def get_policy(metadata: PolicyMetadata, params: Dict[str, Any]) -> Policy:
    """Create a SlugifyBodyPolicy instance for one route.

    The gateway calls this function exactly once per route that attaches
    this policy, passing the merged system + user parameters resolved from
    the API configuration.  The returned instance is reused for every HTTP
    request on that route — construct it as if it is long-lived.

    This mirrors Go's GetPolicy(metadata PolicyMetadata, params map[string]interface{})
    (Policy, error) factory function.

    Args:
        metadata: Route and API metadata (route_name, api_id, api_name, …).
                  Useful for keyed caches or scoped logging.
        params:   Merged policy parameters (system params + user params).
                  System params come from the gateway config file.
                  User params come from the API attachment configuration.

    Returns:
        A fully initialised SlugifyBodyPolicy instance.
    """
    logger.info(
        "get_policy: creating SlugifyBodyPolicy instance for route=%r api=%r",
        metadata.route_name,
        metadata.api_name,
    )

    # Parse and validate params with safe defaults.
    # Perform all validation here — on_request should be as lean as possible.
    separator = str(params.get("separator", "-"))
    if separator not in ("-", "_", ".", "~"):
        logger.warning(
            "get_policy: unsupported separator %r, falling back to '-'", separator
        )
        separator = "-"

    max_length = int(params.get("maxLength", 0))
    if max_length < 0:
        logger.warning(
            "get_policy: maxLength %d is negative, treating as unlimited", max_length
        )
        max_length = 0

    return SlugifyBodyPolicy(separator=separator, max_length=max_length)
