"""Smoke tests for the standalone APIP SDK Core package."""

from __future__ import annotations

import dataclasses
import importlib.resources as resources
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SRC = ROOT / "src"

if str(SRC) not in sys.path:
    sys.path.insert(0, str(SRC))

import apip_sdk_core
from apip_sdk_core import Headers
from apip_sdk_core.policy import v1alpha2
from apip_sdk_core.policy.v1alpha2 import AuthContext, SharedContext


class PublicAPITests(unittest.TestCase):
    def test_root_reexports_versioned_symbols(self) -> None:
        self.assertIs(apip_sdk_core.RequestPolicy, v1alpha2.RequestPolicy)
        self.assertIs(apip_sdk_core.ProcessingMode, v1alpha2.ProcessingMode)
        self.assertIn("RequestPolicy", apip_sdk_core.__all__)
        self.assertIn("policy", apip_sdk_core.__all__)

    def test_error_type_values_match_the_go_sdk(self) -> None:
        # These strings are on the wire — the gateway renders type into the error body for
        # every shape — and the Go SDK declares the same set in
        # sdk/core/policy/v1alpha2/action.go, asserted there by TestFaultTypeWireValues.
        # Spelled out literally on both sides on purpose: comparing a constant to itself
        # would pass through a rename and prove nothing, and a value changed on one side
        # only means the same failure is reported under two names depending on which
        # language the policy happens to be written in.
        self.assertIs(apip_sdk_core.FaultType, v1alpha2.FaultType)
        self.assertIn("FaultType", apip_sdk_core.__all__)

        expected = {
            "AUTHENTICATION": "authentication",
            "AUTHORIZATION": "authorization",
            "THROTTLING": "throttling",
            "GUARDRAIL": "guardrail",
            "VALIDATION": "validation",
            "MEDIATION": "mediation",
            "CONFIGURATION": "configuration",
            "UPSTREAM": "upstream",
            "ROUTING": "routing",
            "INTERNAL": "internal",
            "REQUEST_SIZE": "requestSize",
        }
        for name, wire in expected.items():
            self.assertEqual(getattr(v1alpha2.FaultType, name), wire, name)

        # Distinct, or two classes silently merge for anyone branching on type.
        values = [getattr(v1alpha2.FaultType, n) for n in expected]
        self.assertEqual(len(set(values)), len(values))

        # Plain str, so it drops straight into the field without conversion.
        err = v1alpha2.FaultDetails(type=v1alpha2.FaultType.GUARDRAIL)
        self.assertEqual(err.type, "guardrail")

    def test_error_source_values_match_the_go_sdk(self) -> None:
        # Also read by execution conditions as error.Source, so these strings are
        # configuration surface as well as wire surface: a value that differs between the two
        # SDKs means a deployment's condition matches Go policies and not Python ones.
        # Declared in sdk/core/policy/v1alpha2/context.go.
        self.assertIs(apip_sdk_core.FaultSource, v1alpha2.FaultSource)
        self.assertIn("FaultSource", apip_sdk_core.__all__)

        expected = {
            "GATEWAY": "gateway",
            "BACKEND": "backend",
            "ROUTER": "router",
            "NO_ROUTE": "noRoute",
            "UNKNOWN": "unknown",
        }
        for name, wire in expected.items():
            self.assertEqual(getattr(v1alpha2.FaultSource, name), wire, name)

        values = [getattr(v1alpha2.FaultSource, n) for n in expected]
        self.assertEqual(len(set(values)), len(values))

    def test_error_codes_match_the_go_sdk(self) -> None:
        # Declared in sdk/core/policy/v1alpha2/error_codes.go, with the integer form in the
        # gateway's internal/analytics. Spelled out literally on every side on purpose: these
        # are numbers a customer's error handler matches on, and a code that differs between
        # the two SDKs reports the same failure under two identifiers depending on which
        # language the policy happens to be written in.
        self.assertIs(apip_sdk_core.FaultCode, v1alpha2.FaultCode)
        self.assertIn("FaultCode", apip_sdk_core.__all__)

        expected = {
            "AUTH_GENERAL": "900900",
            "AUTH_INVALID_CREDENTIALS": "900901",
            "AUTH_MISSING_CREDENTIALS": "900902",
            "AUTH_TOKEN_EXPIRED": "900903",
            "AUTH_TOKEN_INACTIVE": "900904",
            "AUTH_INCORRECT_TOKEN_TYPE": "900905",
            "AUTH_BLOCKED": "900907",
            "AUTH_FORBIDDEN": "900908",
            "SUBSCRIPTION_INACTIVE": "900909",
            "INVALID_SCOPE": "900910",
            "THROTTLED_API": "900800",
            "THROTTLED_RESOURCE": "900802",
            "THROTTLED_APPLICATION": "900803",
            "THROTTLED_SUBSCRIPTION": "900804",
            "THROTTLED_BLOCKED": "900805",
            "THROTTLED_CUSTOM_POLICY": "900806",
            "UPSTREAM_UNREACHABLE": "101503",
            "UPSTREAM_TIMEOUT": "101504",
            "UPSTREAM_GENERIC": "101599",
            "UPSTREAM_UNAVAILABLE": "303001",
            "NO_ROUTE": "900906",
            "ENGINE_INTERNAL": "905003",
            "PAYLOAD_TOO_LARGE": "905004",
            "NO_POLICY_CHAIN": "905005",
            "RESOLUTION_BAD_REQUEST": "905006",
            "RESOLUTION_UNSUPPORTED_ENCODING": "905007",
        }
        for name, wire in expected.items():
            self.assertEqual(getattr(v1alpha2.FaultCode, name), wire, name)

        values = [getattr(v1alpha2.FaultCode, n) for n in expected]
        self.assertEqual(len(set(values)), len(values), "two names for one code")
        for name, wire in expected.items():
            self.assertEqual(len(wire), 6, f"{name} must be six digits")

        code = v1alpha2.FaultCode
        # Category membership is the whole mechanism: a code outside every range is filed as
        # "other" however specific its meaning.
        for name in ("AUTH_GENERAL", "AUTH_FORBIDDEN", "INVALID_SCOPE", "NO_ROUTE"):
            self.assertTrue(
                code.AUTH_FAILURE_RANGE_START
                <= int(getattr(code, name))
                < code.AUTH_FAILURE_RANGE_END,
                name,
            )
        for name in ("THROTTLED_API", "THROTTLED_BLOCKED", "THROTTLED_CUSTOM_POLICY"):
            self.assertTrue(
                code.THROTTLED_FAILURE_RANGE_START
                <= int(getattr(code, name))
                < code.THROTTLED_FAILURE_RANGE_END,
                name,
            )
        for name in ("UPSTREAM_UNREACHABLE", "UPSTREAM_TIMEOUT", "UPSTREAM_GENERIC"):
            self.assertTrue(
                code.TARGET_FAILURE_RANGE_START
                <= int(getattr(code, name))
                < code.TARGET_FAILURE_RANGE_END,
                name,
            )
        # The exception, and it has to be: APIM classifies 303001 by an explicit case, not by
        # range. Asserting it is OUTSIDE stops someone "fixing" it in and losing the
        # CONNECTION_SUSPENDED sub-category.
        self.assertFalse(
            code.TARGET_FAILURE_RANGE_START
            <= int(code.UPSTREAM_UNAVAILABLE)
            < code.TARGET_FAILURE_RANGE_END
        )
        # The policy space must be contiguous — a gap would be a range nobody may allocate in.
        self.assertEqual(code.SHIPPED_POLICY_RANGE_END, code.USER_DEFINED_RANGE_START)

    def test_guardrail_codes_match_the_go_sdk(self) -> None:
        # Declared in sdk/core/policy/v1alpha2/action.go and asserted there by
        # TestGuardrailCodesLandInTheirOwnGroup. Spelled out literally here for the same
        # reason as FaultType: a code that differs between the SDKs reports the same
        # rejection under two identifiers depending on the policy's language.
        self.assertIs(apip_sdk_core.GuardrailCode, v1alpha2.GuardrailCode)
        self.assertIn("GuardrailCode", apip_sdk_core.__all__)

        expected = {
            "INTERVENED": "906000",
            "HATE": "906001",
            "SEXUAL": "906002",
            "SELF_HARM": "906003",
            "VIOLENCE": "906004",
            "HARASSMENT": "906005",
            "DANGEROUS_ACTIVITY": "906006",
            "PROFANITY": "906007",
            "PROMPT_INJECTION": "906008",
            "UNSAFE_CONTENT": "906099",
            "PII": "906101",
            "CREDENTIAL": "906102",
            "WORD_COUNT": "906201",
            "SENTENCE_COUNT": "906202",
            "CONTENT_LENGTH": "906203",
            "SCHEMA": "906204",
            "PATTERN": "906205",
            "URL": "906206",
            "SEMANTIC_MATCH": "906301",
        }
        for name, wire in expected.items():
            self.assertEqual(getattr(v1alpha2.GuardrailCode, name), wire, name)

        values = [getattr(v1alpha2.GuardrailCode, n) for n in expected]
        self.assertEqual(len(set(values)), len(values))

        code = v1alpha2.GuardrailCode
        # The generic lives INSIDE the block, so "is this a guardrail rejection?" is one range
        # test — the same rule every other category follows.
        self.assertTrue(code.RANGE_START <= int(code.INTERVENED) < code.RANGE_END)

        # Group membership is the contract — a caller tests bounds rather than enumerating.
        groups = (
            (code.CONTENT_SAFETY_RANGE_START, code.CONTENT_SAFETY_RANGE_END,
             ("HATE", "SEXUAL", "SELF_HARM", "VIOLENCE", "HARASSMENT",
              "DANGEROUS_ACTIVITY", "PROFANITY", "PROMPT_INJECTION", "UNSAFE_CONTENT")),
            (code.SENSITIVE_DATA_RANGE_START, code.SENSITIVE_DATA_RANGE_END,
             ("PII", "CREDENTIAL")),
            (code.SHAPE_RANGE_START, code.SHAPE_RANGE_END,
             ("WORD_COUNT", "SENTENCE_COUNT", "CONTENT_LENGTH", "SCHEMA", "PATTERN", "URL")),
            (code.INTENT_RANGE_START, code.INTENT_RANGE_END, ("SEMANTIC_MATCH",)),
        )
        for start, end, names in groups:
            for name in names:
                self.assertTrue(
                    start <= int(getattr(code, name)) < end,
                    f"{name} is outside its group [{start}, {end})",
                )

        for start, end, _ in groups:
            self.assertFalse(
                start <= int(code.INTERVENED) < end,
                f"the generic code falls in the group [{start}, {end}); it belongs to the "
                f"block as a whole, so a group test must not match it",
            )

    def test_error_response_detail_blocks_are_exported(self) -> None:
        # A Python guardrail builds these by name, so they have to be reachable from the
        # package root — the Go side reaches them through the same import path.
        for name in ("FaultDetails", "GuardrailDetails", "JSONRPCError",
                     "GUARDRAIL_ACTION_INTERVENED"):
            self.assertIs(getattr(apip_sdk_core, name), getattr(v1alpha2, name), name)
            self.assertIn(name, apip_sdk_core.__all__, name)

        # Both detail blocks default to None, and that default is load-bearing: absent means
        # "not a guardrail error" / "not a JSON-RPC caller", which is what the gateway's
        # renderers test. A zero-valued block would be indistinguishable from a real one that
        # reported nothing.
        empty = v1alpha2.FaultDetails()
        self.assertIsNone(empty.guardrail)
        self.assertIsNone(empty.jsonrpc)

        # The action default saves every guardrail from restating the one legal value.
        self.assertEqual(
            v1alpha2.GUARDRAIL_ACTION_INTERVENED,
            v1alpha2.GuardrailDetails().action,
        )

    def test_error_policy_is_exported_and_independent(self) -> None:
        # A fault policy is selected by the gateway through this interface, so it has to be
        # reachable from the package root like every other policy type.
        self.assertIs(apip_sdk_core.FaultPolicy, v1alpha2.FaultPolicy)
        self.assertIs(apip_sdk_core.FaultContext, v1alpha2.FaultContext)
        self.assertIn("FaultPolicy", apip_sdk_core.__all__)

        # It must not extend any phase interface: a notifier implements on_fault and nothing
        # else, and the executor derives the capability from isinstance.
        self.assertTrue(issubclass(v1alpha2.FaultPolicy, v1alpha2.Policy))
        self.assertFalse(issubclass(v1alpha2.FaultPolicy, v1alpha2.ResponsePolicy))

        # The context declares its response fields directly and must NOT subclass the
        # response view. Inheritance made an FaultContext pass a ResponseContext annotation,
        # so a fault handler could be routed to response-phase code that would try to forward
        # a response which has already failed. The field NAMES still match, deliberately, so
        # a handler reads the same values it would in on_response_body.
        self.assertFalse(issubclass(v1alpha2.FaultContext, v1alpha2.ResponseContext))
        response_fields = {f.name for f in dataclasses.fields(v1alpha2.ResponseContext)}
        error_fields = {f.name for f in dataclasses.fields(v1alpha2.FaultContext)}
        self.assertTrue(
            response_fields <= error_fields,
            f"FaultContext must still carry every response field; missing {response_fields - error_fields}",
        )

        # on_fault returns a FaultResponse, not a response action.
        self.assertIs(apip_sdk_core.FaultResponse, v1alpha2.FaultResponse)
        self.assertIn("FaultResponse", apip_sdk_core.__all__)

    def test_package_includes_typed_marker(self) -> None:
        marker = resources.files("apip_sdk_core").joinpath("py.typed")
        self.assertTrue(marker.is_file())

    def test_headers_are_case_insensitive_and_defensive(self) -> None:
        headers = Headers({"X-Test": ["one", "two"]})

        values = headers.get("x-test")
        values.append("three")

        self.assertEqual(headers.get("X-Test"), ["one", "two"])
        self.assertEqual(headers.get_all(), {"x-test": ["one", "two"]})

    def test_shared_context_resolution_defaults_are_not_applicable(self) -> None:
        """Every API kind released before Agent resolves its chain from the route,
        so a policy has to be able to read the defaults as "not applicable" rather
        than testing for None first."""
        shared = SharedContext()

        self.assertEqual(shared.resolved_operation, "")
        self.assertEqual(shared.resolution_attributes, {})
        self.assertEqual(shared.resolution_attributes.get("a2a.context.id", ""), "")

    def test_shared_context_carries_the_resolved_operation_and_attributes(self) -> None:
        shared = SharedContext(
            api_kind="Agent",
            resolved_operation="SendMessage",
            resolution_attributes={
                "a2a.transport": "JSONRPC",
                "a2a.context.id": "ctx-1",
            },
        )

        self.assertEqual(shared.resolved_operation, "SendMessage")
        self.assertEqual(shared.resolution_attributes["a2a.context.id"], "ctx-1")

    def test_shared_context_positional_construction_is_unchanged(self) -> None:
        """SharedContext is a published dataclass with no kw_only, so its field
        order is its positional constructor. A field inserted ahead of
        auth_context would silently bind an existing caller's tenth positional
        argument to it and leave auth_context as None — so new fields go on the
        end, and this pins that."""
        auth = AuthContext(authenticated=True, subject="alice")

        shared = SharedContext(
            "proj", "req", {}, "id", "name", "1.0", "Agent", "/ctx", "/path", auth
        )

        self.assertIs(shared.auth_context, auth)
        self.assertEqual(shared.operation_path, "/path")
        self.assertEqual(shared.resolved_operation, "")
        self.assertEqual(shared.resolution_attributes, {})

    def test_shared_context_field_order_keeps_new_fields_last(self) -> None:
        """The positional contract above only holds while the fields added for
        Agent stay at the end. Asserted directly so a later reordering fails
        here rather than in someone else's policy."""
        names = [f.name for f in dataclasses.fields(SharedContext)]

        self.assertEqual(names[-2:], ["resolved_operation", "resolution_attributes"])
        self.assertEqual(names.index("auth_context"), len(names) - 3)

    def test_shared_context_instances_do_not_share_a_default_attribute_dict(self) -> None:
        """A mutable default would make one request's attributes visible on the
        next — the hazard the Go SDK's read-only wrapper exists to prevent, which
        here is handled by field(default_factory=dict)."""
        first, second = SharedContext(), SharedContext()

        first.resolution_attributes["a2a.context.id"] = "ctx-1"

        self.assertEqual(second.resolution_attributes, {})
