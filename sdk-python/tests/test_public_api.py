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
