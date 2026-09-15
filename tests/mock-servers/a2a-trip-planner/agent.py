"""The trip-planner agent: its cards and its AgentExecutor.

This is a real A2A agent built on the official SDK, not a mock of one. The
gateway's A2A integration tests exist to check the gateway against the
protocol, and a hand-written stand-in would only ever check it against whatever
framing the stand-in chose. Everything protocol-shaped here — task lifecycle,
SSE framing, error shapes, the eleven operations — is the SDK's.

What is deliberately ours is the *content*: every string the agent produces is
fixed and derived from the request, so a feature file can assert on it exactly
rather than matching loosely and passing on a wrong answer.
"""

from __future__ import annotations

import asyncio
import re

from a2a.helpers import (
    get_message_text,
    new_task_from_user_message,
    new_text_part,
)
from a2a.server.agent_execution import AgentExecutor, RequestContext
from a2a.server.events import EventQueue
from a2a.server.tasks import TaskUpdater
from a2a.types import (
    AgentCapabilities,
    AgentCard,
    AgentInterface,
    AgentSkill,
    TaskState,
)

import config

# ─── Itinerary content ───────────────────────────────────────────────────────

DEFAULT_DESTINATION = 'Kandy'
DEFAULT_DAYS = 3

# Cycled by day index so an itinerary of any length is fully determined by its
# destination and day count. A feature file can therefore assert the whole
# artifact, not just its first line.
ACTIVITIES = (
    'Temple visits',
    'Botanical gardens',
    'A lake walk',
    'Tea country day trip',
    'Local markets',
)

# The three phases a streaming request reports before producing its artifact.
STREAM_LABELS = (
    'checking flights',
    'picking hotels',
    'building the day-by-day itinerary',
)

# "plan a trip to X slowly" parks the task in `working` instead of completing
# it. Without this every task is terminal by the time the next request arrives,
# and GetTask/ListTasks/CancelTask/SubscribeToTask would all be exercised
# against a finished task — which is not the state any of them exist for.
SLOW_MARKER = 'slowly'

_DAYS_PATTERN = re.compile(r'(\d+)[-\s]*day', re.IGNORECASE)
_DESTINATION_PATTERN = re.compile(r'\bto\s+([A-Za-z][A-Za-z\s]*?)\s*$', re.IGNORECASE)


def parse_request(text: str) -> tuple[str, int, bool]:
    """Extracts destination, day count, and slow mode from a message.

    Returns the defaults for anything absent rather than failing: the fixture's
    job is to be predictable, and a request it cannot parse is far more likely
    to be a scenario checking routing than one checking parsing.
    """
    cleaned = (text or '').strip()
    slow = SLOW_MARKER in cleaned.lower()

    # Strip the slow marker before reading the destination, or "to Kandy
    # slowly" yields a destination of "Kandy slowly".
    for_destination = re.sub(SLOW_MARKER, '', cleaned, flags=re.IGNORECASE).strip()
    # Trailing punctuation would otherwise become part of the destination.
    for_destination = for_destination.rstrip('.!?,;: ')

    days = DEFAULT_DAYS
    if match := _DAYS_PATTERN.search(cleaned):
        parsed = int(match.group(1))
        # A zero- or negative-day trip has no itinerary to render, and an
        # enormous one would make the artifact unbounded on caller-supplied
        # input. Clamp rather than reject: this is a fixture, and a scenario
        # that sends nonsense is testing something else.
        days = min(max(parsed, 1), 14)

    destination = DEFAULT_DESTINATION
    if match := _DESTINATION_PATTERN.search(for_destination):
        candidate = match.group(1).strip()
        if candidate:
            destination = candidate

    return destination, days, slow


def build_itinerary(destination: str, days: int) -> str:
    """Renders the itinerary artifact text. Fully determined by its inputs."""
    lines = [f'Trip plan for {destination}: {days} days']
    for day in range(1, days + 1):
        activity = ACTIVITIES[(day - 1) % len(ACTIVITIES)]
        lines.append(f'Day {day}: {activity} in {destination}')
    return '\n'.join(lines)


# ─── Agent cards ─────────────────────────────────────────────────────────────

_PLAN_SKILL = AgentSkill(
    id='plan_trip',
    name='Plan a trip',
    description='Builds a day-by-day itinerary for a destination.',
    tags=['travel', 'planning'],
    examples=['Plan a 3 day trip to Kandy'],
    input_modes=['text/plain'],
    output_modes=['text/plain'],
)

# Present only in the extended card. The scenario proving GetExtendedAgentCard
# is proxied to the agent rather than answered by the gateway keys on this skill
# id: the gateway serves the public card, which does not contain it, so its
# presence in a response is proof the request reached the agent.
_BOOK_SKILL = AgentSkill(
    id='book_trip',
    name='Book a trip',
    description='Books the itinerary. Available to authenticated callers only.',
    tags=['travel', 'booking'],
    examples=['Book the Kandy itinerary'],
    input_modes=['text/plain'],
    output_modes=['text/plain'],
)


def _capabilities() -> AgentCapabilities:
    """The three capability flags this fixture must declare.

    Each one gates operations in the SDK's own request handler, so a missing
    flag does not degrade gracefully — it turns the gated operations into
    errors:

      streaming            → SendStreamingMessage, SubscribeToTask
      push_notifications   → all four *TaskPushNotificationConfig operations
      extended_agent_card  → GetExtendedAgentCard

    All eleven operations have to work for this fixture to be worth anything,
    so all three are on.
    """
    return AgentCapabilities(
        streaming=True,
        push_notifications=True,
        extended_agent_card=True,
    )


def _interfaces() -> list[AgentInterface]:
    base = config.PUBLIC_URL.rstrip('/')
    rest_path = config.REST_PATH if config.REST_PATH.startswith('/') else f'/{config.REST_PATH}'
    rpc_path = config.RPC_PATH if config.RPC_PATH.startswith('/') else f'/{config.RPC_PATH}'
    return [
        AgentInterface(
            protocol_binding='JSONRPC',
            url=f'{base}{rpc_path}',
            protocol_version='1.0',
        ),
        AgentInterface(
            protocol_binding='HTTP+JSON',
            url=f'{base}{rest_path}',
            protocol_version='1.0',
        ),
    ]


def public_card() -> AgentCard:
    """The card served at /.well-known/agent-card.json."""
    return AgentCard(
        name='Trip Planner',
        description='Plans trips. Public card.',
        version='1.0.0',
        capabilities=_capabilities(),
        supported_interfaces=_interfaces(),
        default_input_modes=['text/plain'],
        default_output_modes=['text/plain'],
        skills=[_PLAN_SKILL],
    )


def extended_card() -> AgentCard:
    """The card served by GetExtendedAgentCard.

    Distinguishable from the public card in both its description and its skill
    set, so a test can tell which one it received without relying on a subtle
    field difference.
    """
    card = public_card()
    card.description = 'Plans and books trips. Extended card.'
    card.skills.append(_BOOK_SKILL)
    return card


# ─── Executor ────────────────────────────────────────────────────────────────


class TripPlannerExecutor(AgentExecutor):
    """Plans trips, one task at a time.

    The same executor serves SendMessage and SendStreamingMessage: the SDK
    aggregates its events into a final Task for the former and forwards each as
    an SSE event for the latter. That is deliberate — it means the two bindings
    and the two message operations are all exercising one implementation, so a
    difference observed through the gateway is the gateway's.
    """

    def __init__(self) -> None:
        # One cancellation flag per in-flight task. `cancel` sets it and the
        # slow-mode hold polls it. Publishing a cancelled status without this
        # would leave the hold running: the task would report cancelled while
        # the executor kept working, and the event queue would stay open.
        self._cancellations: dict[str, asyncio.Event] = {}

    async def execute(
        self, context: RequestContext, event_queue: EventQueue
    ) -> None:
        """Plans a trip for a newly received message."""
        raw_text = get_message_text(context.message) if context.message else ''
        destination, days, slow = parse_request(raw_text)

        task = context.current_task
        if task is None:
            task = new_task_from_user_message(context.message)
            await event_queue.enqueue_event(task)

        updater = TaskUpdater(
            event_queue,
            task_id=task.id,
            context_id=task.context_id,
        )
        cancelled = asyncio.Event()
        self._cancellations[task.id] = cancelled

        try:
            await updater.start_work()

            for step in range(1, config.STREAM_STEPS + 1):
                if cancelled.is_set():
                    return
                if config.STREAM_STEP_DELAY > 0:
                    await asyncio.sleep(config.STREAM_STEP_DELAY)
                label = STREAM_LABELS[(step - 1) % len(STREAM_LABELS)]
                await updater.update_status(
                    TaskState.TASK_STATE_WORKING,
                    message=updater.new_agent_message(
                        [
                            new_text_part(
                                f'Planning step {step}/{config.STREAM_STEPS}: {label}'
                            )
                        ]
                    ),
                )

            if slow and not await self._hold(updater, cancelled):
                # Cancelled mid-hold. `cancel` already published the terminal
                # status, so completing here would move the task out of a
                # terminal state and emit an event after the last one.
                return

            if cancelled.is_set():
                return

            await updater.add_artifact(
                [new_text_part(build_itinerary(destination, days))],
                name='itinerary',
            )
            await updater.complete()
        finally:
            self._cancellations.pop(task.id, None)

    async def _hold(self, updater: TaskUpdater, cancelled: asyncio.Event) -> bool:
        """Keeps a task in `working` for the configured hold.

        Returns True if the hold ran to completion, False if it was cancelled.
        Waits on the cancellation event rather than sleeping, so a cancel takes
        effect within one tick instead of at the end of the hold.
        """
        remaining = config.SLOW_HOLD_SECONDS
        tick = max(config.SLOW_TICK, 0.05)
        while remaining > 0:
            try:
                await asyncio.wait_for(cancelled.wait(), timeout=min(tick, remaining))
            except TimeoutError:
                pass
            else:
                return False
            remaining -= tick
            if remaining > 0:
                await updater.update_status(
                    TaskState.TASK_STATE_WORKING,
                    message=updater.new_agent_message(
                        [new_text_part('Still planning...')]
                    ),
                )
        return True

    async def cancel(
        self, context: RequestContext, event_queue: EventQueue
    ) -> None:
        """Cancels the referenced task and stops its executor."""
        if not (context.task_id and context.context_id):
            return
        if flag := self._cancellations.get(context.task_id):
            flag.set()
        updater = TaskUpdater(
            event_queue,
            task_id=context.task_id,
            context_id=context.context_id,
        )
        await updater.cancel()
