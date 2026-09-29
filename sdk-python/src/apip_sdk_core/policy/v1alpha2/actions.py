# Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

"""Python action types for `apip_sdk_core.policy.v1alpha2`."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Final


@dataclass(slots=True)
class DropHeaderAction:
    action: str = "deny"
    headers: list[str] = field(default_factory=list)


# The value the shipped guardrail envelope uses for GuardrailDetails.action.
GUARDRAIL_ACTION_INTERVENED = "GUARDRAIL_INTERVENED"


@dataclass(slots=True)
class GuardrailDetails:
    """The detail a guardrail reports about an intervention.

    The field set mirrors the envelope the shipped guardrails already publish in their own
    response bodies, so adopting this type is a re-homing of an existing contract rather than a
    new one anyone has to learn.

    ``direction`` is deliberately not repeated here: it is on :class:`FaultDetails`, which
    every guardrail error already carries.

    ``assessments`` is the per-guardrail evidence for the intervention, and for a RESPONSE
    guardrail it frequently contains the very content the guardrail existed to stop from
    leaving. Populate it only where the operator opted in — the shipped guardrails gate it on
    their own ``showAssessment`` parameter, and send the metadata above it on every
    intervention. Leaving the whole block off means "no guardrail was involved", which is a
    different statement.
    """

    intervening_guardrail: str = ""
    action: str = GUARDRAIL_ACTION_INTERVENED
    action_reason: str = ""
    assessments: dict[str, Any] | None = None


@dataclass(slots=True)
class JSONRPCError:
    """JSON-RPC detail only the policy that read the request can supply.

    Both fields exist because the engine cannot obtain them. It derives a code from the HTTP
    status (4xx/5xx onto -32600/-32603), which is right for a gateway failure and wrong for a
    policy that parsed the request and knows the call was -32602; and it has no request id at
    all, because most errors are produced before any body has been read.

    ``code`` left as None means "derive it from the status". ``id`` left as None renders as
    JSON-RPC's null, which the spec requires for a request whose id could not be determined;
    set it to whatever the client sent — the protocol permits a string or a number.
    """

    code: int | None = None
    id: Any = None


class FaultType:
    """Canonical values for :attr:`FaultDetails.type`, gathered so every producer spells the
    same failure class the same way.

    These reach the client: ``type`` is rendered into the error body for every shape —
    top-level in JSON and XML, inside ``error.data`` for JSON-RPC — so a value here is a wire
    contract, not an internal label.

    **Canonical, not closed.** ``type`` stays a plain ``str`` and nothing validates it, so a
    policy solving a problem none of these describes may use its own value. What this buys is
    that the classes already in use are spelled one way. Pick the one that fits before adding
    a value.

    The first group is for a policy describing why *it* rejected the exchange:

    ``AUTHENTICATION``
        The caller could not be identified — credential missing, malformed, expired or
        unverifiable.
    ``AUTHORIZATION``
        The caller was identified but is not permitted — scope, ACL, subscription, tool or
        resource not allowed.
    ``THROTTLING``
        A rate limit or quota was reached.
    ``GUARDRAIL``
        Content inspection rejected the request or the response. Pair it with
        :attr:`FaultDetails.guardrail` so the assessment travels too.
    ``VALIDATION``
        The message did not satisfy a required shape — unparseable body, missing field,
        schema mismatch.
    ``MEDIATION``
        A transformation, translation or rewrite step failed.
    ``CONFIGURATION``
        The policy's own parameters are unusable. An *operator* error, not a caller error:
        the same request would fail identically from anyone, so it usually deserves a 5xx.

    The second group is set by the **gateway** for failures no policy produced. A policy has
    no reason to use these, but they appear in client-visible bodies:

    ``UPSTREAM``
        The proxy attempt could not be completed — no healthy host, reset, upstream timeout.
    ``ROUTING``
        No API matched the request.
    ``INTERNAL``
        The gateway itself failed, e.g. a policy chain that could not run.
    ``REQUEST_SIZE``
        The request body exceeded the configured decompression ceiling.
    """

    # Policy-produced classes.
    AUTHENTICATION: Final[str] = "authentication"
    AUTHORIZATION: Final[str] = "authorization"
    THROTTLING: Final[str] = "throttling"
    GUARDRAIL: Final[str] = "guardrail"
    VALIDATION: Final[str] = "validation"
    MEDIATION: Final[str] = "mediation"
    CONFIGURATION: Final[str] = "configuration"

    # Gateway-produced classes. Set by the engine, not by a policy.
    UPSTREAM: Final[str] = "upstream"
    ROUTING: Final[str] = "routing"
    INTERNAL: Final[str] = "internal"
    #: The one multi-word value, and the only one not a single lowercase word. Kept as-is
    #: because it is already what the gateway emits.
    REQUEST_SIZE: Final[str] = "requestSize"


class FaultCode:
    """Values for :attr:`FaultDetails.code`: the APIM error codes, gathered so a policy reuses
    one instead of re-typing the digits.

    The Go SDK declares the same set in ``sdk/core/policy/v1alpha2/error_codes.go``, and the
    gateway keeps the integer form in ``internal/analytics`` for analytics events. All three are
    pinned against each other by tests, because none of them can import the others.

    **Read versus write.** Two groups, and the distinction matters:

    - Codes a **policy** sets, describing why it rejected the exchange. Use these.
    - Codes the **gateway** sets, for failures no policy produced. A policy must not emit one —
      claiming an upstream timeout it did not observe misreports the failure to everything
      downstream. They are here because a **fault** policy has the opposite need: branching on
      ``err.code == FaultCode.UPSTREAM_TIMEOUT`` otherwise means hard-coding ``"101504"``.

    **Allocating a new one.** A code's numeric range decides its analytics *category*, so a
    code outside every range is filed as "other" however specific its meaning. Allocate in
    ``USER_DEFINED_RANGE_*`` and read
    ``docs/gateway/error-codes.md`` first.

    Content rejections have their own family — see :class:`GuardrailCode`.
    """

    # ── Codes a policy sets ──────────────────────────────────────────────────
    #
    # Authentication and authorization, from APIM's APISecurityConstants. All inside the
    # auth-failure range, so they classify as authentication failures.
    AUTH_GENERAL: Final[str] = "900900"
    AUTH_INVALID_CREDENTIALS: Final[str] = "900901"
    #: No credential presented at all. Distinct from invalid on purpose: one is a caller who
    #: tried, the other a caller who did not, and they need different client-side handling.
    AUTH_MISSING_CREDENTIALS: Final[str] = "900902"
    AUTH_TOKEN_EXPIRED: Final[str] = "900903"
    AUTH_TOKEN_INACTIVE: Final[str] = "900904"
    AUTH_INCORRECT_TOKEN_TYPE: Final[str] = "900905"
    AUTH_BLOCKED: Final[str] = "900907"
    #: The caller was identified and is not permitted. This is the AUTHORIZATION failure — pair
    #: it with :attr:`FaultType.AUTHORIZATION`, not ``AUTHENTICATION``.
    AUTH_FORBIDDEN: Final[str] = "900908"
    SUBSCRIPTION_INACTIVE: Final[str] = "900909"
    INVALID_SCOPE: Final[str] = "900910"

    # Throttling. The LEVEL is part of the code because that is what a caller needs in order
    # to react: an application limit means back off for this app, a blocked condition means
    # retrying will not help.
    THROTTLED_API: Final[str] = "900800"
    THROTTLED_RESOURCE: Final[str] = "900802"
    THROTTLED_APPLICATION: Final[str] = "900803"
    THROTTLED_SUBSCRIPTION: Final[str] = "900804"
    THROTTLED_BLOCKED: Final[str] = "900805"
    THROTTLED_CUSTOM_POLICY: Final[str] = "900806"

    # ── Codes the gateway sets — do not emit these ───────────────────────────
    UPSTREAM_UNREACHABLE: Final[str] = "101503"
    UPSTREAM_TIMEOUT: Final[str] = "101504"
    UPSTREAM_GENERIC: Final[str] = "101599"
    #: No healthy host. APIM classifies this by an explicit case rather than by range, giving
    #: it its own sub-category, which is why it sits outside the target-failure range.
    UPSTREAM_UNAVAILABLE: Final[str] = "303001"
    NO_ROUTE: Final[str] = "900906"
    ENGINE_INTERNAL: Final[str] = "905003"
    PAYLOAD_TOO_LARGE: Final[str] = "905004"
    NO_POLICY_CHAIN: Final[str] = "905005"
    RESOLUTION_BAD_REQUEST: Final[str] = "905006"
    RESOLUTION_UNSUPPORTED_ENCODING: Final[str] = "905007"

    # ── Where a new code may go ──────────────────────────────────────────────
    #
    # APIM assigns a fault category by testing `start <= code < end`, so membership is part of
    # the contract rather than a convention.
    AUTH_FAILURE_RANGE_START: Final[int] = 900900
    AUTH_FAILURE_RANGE_END: Final[int] = 901000
    THROTTLED_FAILURE_RANGE_START: Final[int] = 900800
    THROTTLED_FAILURE_RANGE_END: Final[int] = 900900
    TARGET_FAILURE_RANGE_START: Final[int] = 101500
    TARGET_FAILURE_RANGE_END: Final[int] = 101600
    #: For policies WSO2 ships. Allocated per policy.
    SHIPPED_POLICY_RANGE_START: Final[int] = 960000
    SHIPPED_POLICY_RANGE_END: Final[int] = 965000
    #: Reserved for a deployment's own policies. WSO2 never allocates inside it.
    USER_DEFINED_RANGE_START: Final[int] = 965000
    USER_DEFINED_RANGE_END: Final[int] = 970000


class GuardrailCode:
    """Values for :attr:`FaultDetails.code` when a guardrail rejected content.

    Every guardrail code lives in one block, so "is this a guardrail rejection?" is a single
    range test: ``906000 <= int(code) < 906400``.

    :attr:`INTERVENED` (906000) is the generic and the right answer for a guardrail with
    nothing more specific to say — which is most of them, since most have exactly one
    rejection scenario and the policy name already says which it was. It sits at the head of
    the block, outside all four groups below, so a group test does not match it.

    The block sits above the engine's own 905xxx codes and below the 96xxxx policy space,
    grouped so related rejections are adjacent:

    ==================  ===============================================================
    ``906001-906099``   what the content was judged to BE — a moderation service's verdict
    ``906100-906199``   what the content CONTAINED — sensitive data found in it
    ``906200-906299``   what SHAPE the content had — size, structure, pattern
    ``906300-906399``   what the content MEANT — intent matched against a configured policy
    ==================  ===============================================================

    The grouping is the contract, not decoration: a caller wanting "any content-safety
    rejection" tests ``906001 <= int(code) < 906100`` rather than enumerating categories, and
    that only works if a new category lands inside its own group. Allocate within a group; do
    not append to the end of the block.
    """

    #: The generic guardrail rejection. Prefer it unless a caller needs to distinguish this
    #: rejection from another one.
    INTERVENED: Final[str] = "906000"

    # 906001-906099 — what the content was judged to be. The first four are the categories
    # every major moderation service reports, and are the reason this block exists.
    HATE: Final[str] = "906001"
    SEXUAL: Final[str] = "906002"
    SELF_HARM: Final[str] = "906003"
    VIOLENCE: Final[str] = "906004"
    HARASSMENT: Final[str] = "906005"
    DANGEROUS_ACTIVITY: Final[str] = "906006"
    PROFANITY: Final[str] = "906007"
    #: A jailbreak or instruction-override attempt. In this group rather than the intent group
    #: because a detector reports it as a property of the content, the way it reports hate.
    PROMPT_INJECTION: Final[str] = "906008"
    #: The group's own generic: a moderation service said "unsafe" without a category this
    #: vocabulary covers. Prefer a specific code where the service gave one.
    UNSAFE_CONTENT: Final[str] = "906099"

    # 906100-906199 — what the content contained.
    PII: Final[str] = "906101"
    CREDENTIAL: Final[str] = "906102"

    # 906200-906299 — what shape the content had.
    WORD_COUNT: Final[str] = "906201"
    SENTENCE_COUNT: Final[str] = "906202"
    CONTENT_LENGTH: Final[str] = "906203"
    SCHEMA: Final[str] = "906204"
    PATTERN: Final[str] = "906205"
    URL: Final[str] = "906206"

    # 906300-906399 — what the content meant.
    SEMANTIC_MATCH: Final[str] = "906301"

    # Group boundaries, half-open. Exported so a caller can test group membership instead of
    # enumerating codes; hand-written bounds in each consumer would drift as categories are
    # added.
    RANGE_START: Final[int] = 906000
    RANGE_END: Final[int] = 906400
    # Starts at 906001: 906000 is INTERVENED, which belongs to the block as a whole rather
    # than to any one group.
    CONTENT_SAFETY_RANGE_START: Final[int] = 906001
    CONTENT_SAFETY_RANGE_END: Final[int] = 906100
    SENSITIVE_DATA_RANGE_START: Final[int] = 906100
    SENSITIVE_DATA_RANGE_END: Final[int] = 906200
    SHAPE_RANGE_START: Final[int] = 906200
    SHAPE_RANGE_END: Final[int] = 906300
    INTENT_RANGE_START: Final[int] = 906300
    INTENT_RANGE_END: Final[int] = 906400


class FaultSource:
    """Values for :attr:`FaultContext.source`: which actor produced the error response.

    These also reach an execution condition as ``error.Source``, so a value here is
    configuration surface — renaming one breaks every deployment whose conditions test for it.

    Unlike :class:`FaultType`, this set **is** closed. ``type`` describes what went wrong,
    which no vocabulary can enumerate ahead of the policies that will need it; this describes
    who produced the response, and a proxy has only so many actors.

    ``GATEWAY``
        The gateway built this itself — a policy or guardrail rejection, or an engine failure.
    ``BACKEND``
        The upstream returned this error status. The API is reachable and refusing; this is
        traffic passing through, not a gateway failure.
    ``ROUTER``
        The proxy could not complete the attempt — connection failure, timeout, no healthy
        host.
    ``NO_ROUTE``
        The request matched no API.
    ``UNKNOWN``
        The proxy sent no provenance, so who produced the response cannot be established — an
        older router, or a phase where it is not populated. Reported rather than guessed at.
    """

    GATEWAY: Final[str] = "gateway"
    BACKEND: Final[str] = "backend"
    ROUTER: Final[str] = "router"
    NO_ROUTE: Final[str] = "noRoute"
    UNKNOWN: Final[str] = "unknown"


@dataclass(slots=True)
class FaultDetails:
    """A policy's description of the failure it produced.

    Carries enough for the gateway to render the failure in the caller's protocol — JSON,
    JSON-RPC for MCP, XML — so a policy describes what went wrong instead of hand-writing a
    body per protocol.

    ``description`` is longer detail a renderer MAY withhold. For a guardrail rejection it is
    the content the guardrail blocked, so it must never be assumed safe to return.

    There is no ``policy`` field. Attribution is gateway-owned: the engine sets it from the
    chain it just executed, so a value supplied here would only be discarded.
    """

    code: str = ""
    #: The failure class. See :class:`FaultType` for the canonical values and what each one
    #: means — it is the one field a caller can branch on without a code table, so prefer a
    #: constant over a new string.
    type: str = ""
    direction: str = ""
    message: str = ""
    description: str = ""
    # JSON-RPC wire detail, for a policy on an MCP (or, later, A2A) API. None for every other
    # caller, and consumed only by the gateway's JSON-RPC renderer — the JSON and XML
    # renderers ignore it, having nowhere to put a JSON-RPC code.
    jsonrpc: JSONRPCError | None = None
    # Assessment detail for a guardrail intervention; None for every other failure. Unlike
    # jsonrpc this is DOMAIN detail, so every renderer emits it in its own shape.
    guardrail: GuardrailDetails | None = None


@dataclass(slots=True)
class ImmediateResponse:
    """Ends the chain and answers the client directly.

    **Whether this is a fault**: ``is_fault``, and nothing else. Not the status, and not a
    described ``fault``. A policy written before this field stays out of the fault flow, which
    is what makes the field safe to add to a shipped catalogue.

    The cost of that default is real and is accepted: an unmigrated policy's genuine rejection
    does not reach the fault flow either. The gateway logs the one shape most likely to be a
    mistake — a ``fault`` described with ``is_fault`` left False — so it can be diagnosed
    rather than guessed at.

    One path cannot use the field: a response the gateway merely forwarded. A backend's own 500
    arrives as no action at all, so it is classified by its status instead. The field answers
    "does the policy that built this response call it a failure", which only has an answer
    where there is an author.
    """

    status_code: int = 500
    headers: dict[str, str] = field(default_factory=dict)
    body: bytes | None = None
    analytics_metadata: dict[str, Any] = field(default_factory=dict)
    dynamic_metadata: dict[str, dict[str, Any]] = field(default_factory=dict)
    analytics_header_filter: DropHeaderAction = field(default_factory=DropHeaderAction)
    # Whether this response is a failure the fault flow should act on. False — the default —
    # means NO, and nothing opts in on the policy's behalf.
    is_fault: bool = False
    # Describes the failure for a renderer. Independent of ``is_fault``: setting it does not
    # make the response a fault. Both combinations are real — a rejection worth reporting sets
    # both, and a deliberate non-failure that still wants an error-shaped body (a canned 404,
    # an auth challenge, a cache miss) sets only this.
    fault: FaultDetails | None = None



@dataclass(slots=True)
class UpstreamRequestHeaderModifications:
    headers_to_set: dict[str, str] = field(default_factory=dict)
    headers_to_remove: list[str] = field(default_factory=list)
    upstream_name: str | None = None
    path: str | None = None
    host: str | None = None
    method: str | None = None
    query_parameters_to_add: dict[str, list[str]] = field(default_factory=dict)
    query_parameters_to_remove: list[str] = field(default_factory=list)
    analytics_metadata: dict[str, Any] = field(default_factory=dict)
    dynamic_metadata: dict[str, dict[str, Any]] = field(default_factory=dict)
    analytics_header_filter: DropHeaderAction = field(default_factory=DropHeaderAction)


@dataclass(slots=True)
class UpstreamRequestModifications:
    body: bytes | None = None
    headers_to_set: dict[str, str] = field(default_factory=dict)
    headers_to_remove: list[str] = field(default_factory=list)
    upstream_name: str | None = None
    path: str | None = None
    host: str | None = None
    method: str | None = None
    query_parameters_to_add: dict[str, list[str]] = field(default_factory=dict)
    query_parameters_to_remove: list[str] = field(default_factory=list)
    analytics_metadata: dict[str, Any] = field(default_factory=dict)
    dynamic_metadata: dict[str, dict[str, Any]] = field(default_factory=dict)
    analytics_header_filter: DropHeaderAction = field(default_factory=DropHeaderAction)


@dataclass(slots=True)
class DownstreamResponseHeaderModifications:
    headers_to_set: dict[str, str] = field(default_factory=dict)
    headers_to_remove: list[str] = field(default_factory=list)
    analytics_metadata: dict[str, Any] = field(default_factory=dict)
    dynamic_metadata: dict[str, dict[str, Any]] = field(default_factory=dict)
    analytics_header_filter: DropHeaderAction = field(default_factory=DropHeaderAction)


@dataclass(slots=True)
class DownstreamResponseModifications:
    """Forwards the response with mutations applied.

    **Whether this is a fault**: ``is_fault``. See :class:`ImmediateResponse`. It matters more
    here than anywhere else, because this action carries every ordinary response mutation and
    not only rejections — so setting ``status_code`` is NOT read as a rejection, and a policy
    relabelling the backend's error sets one too.
    """

    body: bytes | None = None
    status_code: int | None = None
    headers_to_set: dict[str, str] = field(default_factory=dict)
    headers_to_remove: list[str] = field(default_factory=list)
    analytics_metadata: dict[str, Any] = field(default_factory=dict)
    dynamic_metadata: dict[str, dict[str, Any]] = field(default_factory=dict)
    analytics_header_filter: DropHeaderAction = field(default_factory=DropHeaderAction)
    is_fault: bool = False
    fault: FaultDetails | None = None


@dataclass(slots=True)
class ForwardRequestChunk:
    body: bytes | None = None
    analytics_metadata: dict[str, Any] = field(default_factory=dict)
    dynamic_metadata: dict[str, dict[str, Any]] = field(default_factory=dict)


@dataclass(slots=True)
class ForwardResponseChunk:
    body: bytes | None = None
    analytics_metadata: dict[str, Any] = field(default_factory=dict)
    dynamic_metadata: dict[str, dict[str, Any]] = field(default_factory=dict)


@dataclass(slots=True)
class TerminateResponseChunk:
    body: bytes | None = None
    analytics_metadata: dict[str, Any] = field(default_factory=dict)
    dynamic_metadata: dict[str, dict[str, Any]] = field(default_factory=dict)
    # A stream ends both for a guardrail intervention and for a clean close after the
    # upstream's final event, and no status is left to infer from — it went out with the
    # headers. So the default is "not a failure"; a guardrail must set this to True.
    is_fault: bool = False
    fault: FaultDetails | None = None


RequestHeaderAction = UpstreamRequestHeaderModifications | ImmediateResponse | None
RequestAction = UpstreamRequestModifications | ImmediateResponse | None
ResponseHeaderAction = DownstreamResponseHeaderModifications | ImmediateResponse | None
@dataclass(slots=True)
class FaultResponse:
    """What a fault policy returns from ``on_fault``.

    Deliberately not one of the response actions. Those model a choice — forward the
    upstream response, or replace it — that does not arise on the fault path, because there
    is no upstream response to forward: the response is already an error, and every fault
    entry is editing the same object.

    Two fields from :class:`ImmediateResponse` are absent because they were misleading here:
    ``is_fault`` is meaningless once the fault flow is running, and ``headers`` replaced the
    whole map where an entry annotating an error wants to ADD a header without discarding the
    ones the error already carries.

    Every field is optional and merges over the error the client is receiving. Returning
    ``None`` from ``on_fault`` — rather than an empty instance — is how a handler says it
    changed nothing, which is the ordinary case for a notifier.
    """

    #: Overrides the error's status. None keeps it.
    status_code: int | None = None
    #: Replaces the error body. None leaves it alone; ``b""`` clears it. Setting a body also
    #: tells the gateway not to render its own — a fault entry that writes a body has decided
    #: what the client receives.
    body: bytes | None = None
    #: Re-describes the failure for a renderer or a later entry. None keeps the description.
    fault: FaultDetails | None = None
    #: Applied over the error's existing headers rather than replacing them.
    headers_to_set: dict[str, str] = field(default_factory=dict)
    headers_to_append: dict[str, list[str]] = field(default_factory=dict)
    headers_to_remove: list[str] = field(default_factory=list)
    analytics_metadata: dict[str, Any] = field(default_factory=dict)
    dynamic_metadata: dict[str, dict[str, Any]] = field(default_factory=dict)
    analytics_header_filter: DropHeaderAction = field(default_factory=DropHeaderAction)
    #: Stops the rest of the fault chain: entries after this one do not run. Separate from
    #: the fields above because ending the chain and replacing the response are different
    #: decisions, which returning an ``ImmediateResponse`` would fuse into one.
    final: bool = False


ResponseAction = DownstreamResponseModifications | ImmediateResponse | None
StreamingRequestAction = ForwardRequestChunk | None
StreamingResponseAction = ForwardResponseChunk | TerminateResponseChunk | None
