from __future__ import annotations

import importlib.util
import json
import sys
import unittest
from pathlib import Path

from wso2_gateway_policy_sdk import Body, RequestContext, SharedContext, UpstreamRequestModifications


def _load_policy_module():
    test_file = Path(__file__).resolve()
    repo_root = test_file.parents[4]
    if str(repo_root) not in sys.path:
        sys.path.insert(0, str(repo_root))

    policy_path = (
        repo_root / "gateway" / "sample-policies" / "prompt-compressor" / "policy.py"
    )
    module_name = "prompt_compressor_policy_test"
    spec = importlib.util.spec_from_file_location(module_name, policy_path)
    module = importlib.util.module_from_spec(spec)
    assert spec is not None
    assert spec.loader is not None
    sys.modules[module_name] = module
    spec.loader.exec_module(module)
    return module


policy_module = _load_policy_module()


class PromptCompressorPolicyTest(unittest.TestCase):
    def _request_context(self, payload: dict) -> RequestContext:
        return RequestContext(
            shared=SharedContext(),
            body=Body(
                content=json.dumps(payload).encode("utf-8"),
                present=True,
            ),
        )

    def test_normalize_params_sorts_rules_and_drops_invalid_entries(self):
        params = policy_module.normalize_params(
            {
                "jsonPath": "$.messages[0].content",
                "rules": [
                    {"upperTokenLimit": -1, "type": "token", "value": 100},
                    {"upperTokenLimit": 500, "type": "ratio", "value": 0.8},
                    {"upperTokenLimit": 99, "type": "ratio", "value": 0.9},
                    {"upperTokenLimit": 50, "type": "ratio", "value": 0},
                    {"upperTokenLimit": "bad", "type": "ratio", "value": 0.5},
                ],
            }
        )

        self.assertEqual("$.messages[0].content", params.json_path)
        self.assertEqual(3, len(params.rules))
        self.assertEqual([99, 500, -1], [rule.upper_token_limit for rule in params.rules])
        self.assertEqual(["ratio", "ratio", "token"], [rule.rule_type for rule in params.rules])

    def test_on_request_body_compresses_whole_target(self):
        policy = policy_module.get_policy(
            None,
            {
                "rules": [
                    {"upperTokenLimit": -1, "type": "ratio", "value": 0.5},
                ]
            },
        )
        payload = {
            "messages": [
                {
                    "content": (
                        "This is a short prompt with some context and several extra filler "
                        "words that should be removable without changing the core meaning."
                    )
                }
            ]
        }

        action = policy.on_request_body(None, self._request_context(payload), {})

        self.assertIsInstance(action, UpstreamRequestModifications)
        updated_payload = json.loads(action.body.decode("utf-8"))
        updated_content = updated_payload["messages"][0]["content"]
        self.assertNotEqual(payload["messages"][0]["content"], updated_content)
        self.assertLess(len(updated_content), len(payload["messages"][0]["content"]))
        self.assertTrue(
            action.dynamic_metadata[policy_module.DYNAMIC_METADATA_NAMESPACE][
                "compression_applied"
            ]
        )

    def test_on_request_body_selective_mode_strips_tags_when_compression_is_skipped(self):
        policy = policy_module.get_policy(
            None,
            {
                "rules": [
                    {"upperTokenLimit": -1, "type": "ratio", "value": 0.1},
                ]
            },
        )
        tagged = (
            "Keep this intro. "
            "<APIP-COMPRESS>/tmp/file.json /var/log/app.log request_id ABC_DEF 123456</APIP-COMPRESS> "
            "Keep this ending."
        )
        payload = {"messages": [{"content": tagged}]}

        action = policy.on_request_body(None, self._request_context(payload), {})

        self.assertIsInstance(action, UpstreamRequestModifications)
        updated_payload = json.loads(action.body.decode("utf-8"))
        updated_content = updated_payload["messages"][0]["content"]
        self.assertEqual(
            "Keep this intro. /tmp/file.json /var/log/app.log request_id ABC_DEF 123456 Keep this ending.",
            updated_content,
        )
        metadata = action.dynamic_metadata[policy_module.DYNAMIC_METADATA_NAMESPACE]
        self.assertFalse(metadata["compression_applied"])
        self.assertTrue(metadata["selective_mode"])
        self.assertEqual(1, metadata["tagged_segments"])
        self.assertEqual(0, metadata["compressed_segments"])

    def test_on_request_body_returns_none_for_missing_json_path(self):
        policy = policy_module.get_policy(
            None,
            {
                "jsonPath": "$.messages[0].missing",
                "rules": [
                    {"upperTokenLimit": -1, "type": "ratio", "value": 0.5},
                ],
            },
        )
        payload = {"messages": [{"content": "Nothing should happen here."}]}

        action = policy.on_request_body(None, self._request_context(payload), {})

        self.assertIsNone(action)

    def test_transform_text_removes_stray_tags_without_leaking_them(self):
        policy = policy_module.get_policy(
            None,
            {
                "rules": [
                    {"upperTokenLimit": -1, "type": "ratio", "value": 0.5},
                ]
            },
        )

        transformed, summary = policy._transform_text(
            "before <APIP-COMPRESS>inside only and never closed"
        )

        self.assertEqual("before inside only and never closed", transformed)
        self.assertTrue(summary.selective_mode)
        self.assertEqual(1, summary.tagged_segments)
        self.assertEqual(0, summary.compressed_segments)

    def test_transform_text_preserves_plain_text_around_stray_closing_tags(self):
        policy = policy_module.get_policy(
            None,
            {
                "rules": [
                    {"upperTokenLimit": -1, "type": "ratio", "value": 0.5},
                ]
            },
        )

        transformed, summary = policy._transform_text(
            "before </APIP-COMPRESS> after"
        )

        self.assertEqual("before  after", transformed)
        self.assertTrue(summary.selective_mode)
        self.assertEqual(0, summary.tagged_segments)
        self.assertEqual(0, summary.compressed_segments)


if __name__ == "__main__":
    unittest.main()
