"""Serves the trip-planner agent over both A2A HTTP bindings.

Route layout, which the gateway's Agent resource mirrors:

    /                          JSON-RPC binding (all eleven operations)
    /v1/...                    HTTP+JSON binding (one route per operation)
    /.well-known/agent-card.json   public Agent Card

Every route is wrapped in A2AVersionQueryMiddleware, which accepts the protocol
version stated as a query parameter as well as as a header (A2A 1.0 §3.6.1).
The reference SDK reads the header only; see version_query.py.

The two binding prefixes are the agent's own layout, not the gateway's. An
Agent resource declares them as `pathPrefix` values and they travel upstream
with the request, so these paths and those prefixes have to agree.
"""

from __future__ import annotations

import uvicorn

from a2a.server.request_handlers import DefaultRequestHandler
from a2a.server.routes import (
    create_agent_card_routes,
    create_jsonrpc_routes,
    create_rest_routes,
)
from a2a.server.tasks import (
    InMemoryPushNotificationConfigStore,
    InMemoryTaskStore,
)
from starlette.applications import Starlette

import config
from agent import TripPlannerExecutor, extended_card, public_card
from version_query import A2AVersionQueryMiddleware


def build_app() -> Starlette:
    """Assembles the ASGI app serving both bindings from one handler.

    One handler and one task store behind both bindings is the point: a task
    created over JSON-RPC must be readable over HTTP+JSON, which is what makes
    the gateway's "one canonical operation, two transports, one policy chain"
    claim testable end to end rather than only per-binding.
    """
    handler = DefaultRequestHandler(
        agent_executor=TripPlannerExecutor(),
        task_store=InMemoryTaskStore(),
        agent_card=public_card(),
        # Required for the four push-notification-config operations: without a
        # store the handler has nowhere to put a config, and the operations
        # fail despite the capability being declared.
        push_config_store=InMemoryPushNotificationConfigStore(),
        extended_agent_card=extended_card(),
    )

    routes = []
    routes.extend(create_agent_card_routes(public_card()))
    routes.extend(create_jsonrpc_routes(handler, config.RPC_PATH))
    routes.extend(create_rest_routes(handler, path_prefix=config.REST_PATH))
    return Starlette(routes=routes)


def main() -> None:
    # The version shim wraps the whole app rather than one binding: §3.6.1's
    # query alternative is a property of the protocol, not of a transport, and
    # an agent that honoured it on only one of its two bindings would be a
    # worse fixture than one that honoured it on neither.
    app = A2AVersionQueryMiddleware(build_app())
    print(
        f'Trip Planner listening on http://{config.BIND_HOST}:{config.PORT} '
        f'(JSON-RPC {config.RPC_PATH}, HTTP+JSON {config.REST_PATH})',
        flush=True,
    )
    uvicorn.run(app, host=config.BIND_HOST, port=config.PORT, log_level='warning')


if __name__ == '__main__':
    main()
