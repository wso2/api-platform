"""Environment-sourced configuration for the trip-planner fixture.

Every knob here exists because some integration-test scenario needs to change
the agent's timing without rebuilding the image. Defaults are chosen so the
agent behaves correctly with no environment at all — a scenario that does not
care about pacing gets a fast, deterministic agent.
"""

from __future__ import annotations

import os


def _int(name: str, default: int) -> int:
    raw = os.environ.get(name, '').strip()
    if not raw:
        return default
    try:
        return int(raw)
    except ValueError:
        return default


def _float(name: str, default: float) -> float:
    raw = os.environ.get(name, '').strip()
    if not raw:
        return default
    try:
        return float(raw)
    except ValueError:
        return default


def _str(name: str, default: str) -> str:
    raw = os.environ.get(name, '').strip()
    return raw or default


# Where the agent listens. The bind host defaults to all interfaces because the
# agent's only deployment is a container that must be reachable from outside it.
BIND_HOST = _str('TRIP_BIND_HOST', '0.0.0.0')  # noqa: S104
PORT = _int('TRIP_PORT', 9099)

# Where each protocol binding is mounted. These are the paths the gateway's
# Agent `pathPrefix` values mirror: a prefix belongs to the agent's own layout
# and travels upstream with the request, so changing one here without changing
# the Agent resource breaks routing.
RPC_PATH = _str('TRIP_RPC_PATH', '/')
REST_PATH = _str('TRIP_REST_PATH', '/v1')

# How many paced status updates a streaming request emits before the itinerary
# artifact. More than one, and spread over time, so a client can observe an
# event arriving while the task is still running — the property that
# distinguishes a real stream from a buffered response delivered in SSE framing.
STREAM_STEPS = _int('TRIP_STREAM_STEPS', 3)
STREAM_STEP_DELAY = _float('TRIP_STREAM_STEP_DELAY', 0.4)

# Slow mode: how long a "plan ... slowly" request holds its task in `working`,
# and how often it emits a keep-working status while it does. This is what makes
# GetTask, ListTasks, CancelTask and SubscribeToTask act on a live task instead
# of one that already reached a terminal state before the next request arrived.
SLOW_HOLD_SECONDS = _float('TRIP_SLOW_HOLD_SECONDS', 30.0)
SLOW_TICK = _float('TRIP_SLOW_TICK', 1.0)

# The externally reachable base URL the agent advertises in its own card. Only
# the passthrough-card path reads this — a managed card is written by the Agent
# resource, not by the agent.
PUBLIC_URL = _str('TRIP_PUBLIC_URL', f'http://localhost:{PORT}')
