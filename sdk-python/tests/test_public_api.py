"""Smoke tests for the standalone APIP SDK Core package."""

from __future__ import annotations

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
