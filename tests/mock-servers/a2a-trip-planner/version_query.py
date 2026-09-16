"""Accepts the A2A protocol version stated as a query parameter.

A2A 1.0 §3.6.1 lets a client that cannot set request headers — a browser
following a link, an SSE consumer — state its protocol version as an
`A2A-Version` query parameter instead. The reference Python SDK does not
implement that half: `a2a/utils/version_validator.py` reads
`context.state['headers']` only, and a request that stated its version in the
query looks to it like a request that stated nothing, which §3.6.2 defines as
protocol version 0.3 — so a 1.0 handler refuses it.

This is an ASGI shim in front of the SDK, not a patch to it. It normalizes the
query form onto the header *before* routing, and then the SDK performs the
actual version check exactly as it always does. The agent's protocol handling
stays the reference implementation's; only the place the version is read from
is widened to the one the specification already allows.

Deliberately narrow:

  * The header wins whenever it is present. A request carrying both is the
    client's problem to keep consistent, and the gateway in front of this agent
    already rejects a request whose two statements disagree.
  * A repeated parameter with differing values injects nothing, so the request
    is read as having stated no version and is refused — the same fail-closed
    reading the gateway applies rather than picking one of the two.
  * Nothing else about the request is touched.
"""

from __future__ import annotations

from urllib.parse import parse_qsl

VERSION_HEADER = b'a2a-version'

# Case-sensitive, as query-parameter names are: "a2a-version=1.0" is a
# different parameter and is deliberately not read.
VERSION_QUERY_PARAM = 'A2A-Version'


def _stated_in_query(query_string: bytes) -> str | None:
    """Returns the single version stated in the query string, if there is one."""
    if not query_string:
        return None
    values = [
        value
        for name, value in parse_qsl(
            query_string.decode('latin-1'), keep_blank_values=True
        )
        if name == VERSION_QUERY_PARAM
    ]
    if not values:
        return None
    if len({*values}) > 1:
        # Contradictory statements: state nothing rather than choose.
        return None
    return values[0]


class A2AVersionQueryMiddleware:
    """Copies an `A2A-Version` query parameter onto the request header."""

    def __init__(self, app) -> None:
        self.app = app

    async def __call__(self, scope, receive, send) -> None:
        if scope['type'] != 'http':
            await self.app(scope, receive, send)
            return

        headers = scope.get('headers') or []
        if any(name.lower() == VERSION_HEADER for name, _ in headers):
            await self.app(scope, receive, send)
            return

        version = _stated_in_query(scope.get('query_string', b''))
        if version is None:
            await self.app(scope, receive, send)
            return

        # Copy rather than mutate: the scope's header list is the server's, and
        # a request that is retried or logged elsewhere should see what arrived.
        scope = dict(scope)
        scope['headers'] = [*headers, (VERSION_HEADER, version.encode('latin-1'))]
        await self.app(scope, receive, send)
