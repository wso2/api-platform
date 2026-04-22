from __future__ import annotations

import json
import logging
import re
from dataclasses import dataclass
from typing import Any

from compression_prompt import Compressor, CompressorConfig
from compression_prompt.compressor import CompressionError, InputTooShortError, NegativeGainError
from wso2_gateway_policy_sdk import (
    BodyProcessingMode,
    ProcessingMode,
    RequestPolicy,
    UpstreamRequestModifications,
)


LOGGER = logging.getLogger(__name__)

DEFAULT_JSON_PATH = "$.messages[0].content"
OPEN_TAG = "<APIP-COMPRESS>"
CLOSE_TAG = "</APIP-COMPRESS>"
DYNAMIC_METADATA_NAMESPACE = "prompt_compressor"
_ARRAY_INDEX_RE = re.compile(r"^([A-Za-z0-9_]+)\[(-?\d+)\]$")


class JSONPathError(ValueError):
    """Raised when the configured JSONPath cannot be resolved safely."""


@dataclass(frozen=True)
class CompressionRule:
    upper_token_limit: int
    rule_type: str
    value: float


@dataclass(frozen=True)
class PolicyParams:
    json_path: str
    rules: tuple[CompressionRule, ...]


@dataclass(frozen=True)
class TransformationSummary:
    selective_mode: bool
    tagged_segments: int
    compressed_segments: int
    input_tokens_estimated: int
    output_tokens_estimated: int


class PromptCompressorPolicy(RequestPolicy):
    """Compress prompt text in buffered request payloads."""

    def __init__(self, policy_params: PolicyParams):
        self._params = policy_params
        self._compressor_cache: dict[float, Compressor] = {}

    def mode(self) -> ProcessingMode:
        return ProcessingMode(request_body_mode=BodyProcessingMode.BUFFER)

    def on_request_body(self, execution_ctx, ctx, params):
        if ctx.body is None or not ctx.body.present or ctx.body.content is None:
            return None

        if not self._params.rules:
            return None

        body_bytes = ctx.body.content or b""
        if not body_bytes:
            return None

        try:
            payload = json.loads(body_bytes)
        except (TypeError, ValueError, json.JSONDecodeError) as exc:
            LOGGER.warning("PromptCompressor: request body is not valid JSON: %s", exc)
            return None

        try:
            original_text = extract_string_value_from_jsonpath(payload, self._params.json_path)
        except JSONPathError as exc:
            LOGGER.warning(
                "PromptCompressor: failed to extract prompt from jsonPath %s: %s",
                self._params.json_path,
                exc,
            )
            return None

        updated_text, summary = self._transform_text(original_text)
        if updated_text == original_text:
            return None

        try:
            set_value_at_jsonpath(payload, self._params.json_path, updated_text)
        except JSONPathError as exc:
            LOGGER.warning(
                "PromptCompressor: failed to update jsonPath %s: %s",
                self._params.json_path,
                exc,
            )
            return None

        try:
            updated_body = json.dumps(payload, separators=(",", ":"), ensure_ascii=False).encode(
                "utf-8"
            )
        except (TypeError, ValueError) as exc:
            LOGGER.warning("PromptCompressor: failed to marshal updated JSON payload: %s", exc)
            return None

        dynamic_metadata = {
            DYNAMIC_METADATA_NAMESPACE: {
                "compression_applied": summary.compressed_segments > 0,
                "selective_mode": summary.selective_mode,
                "tagged_segments": summary.tagged_segments,
                "compressed_segments": summary.compressed_segments,
                "input_tokens_estimated": summary.input_tokens_estimated,
                "output_tokens_estimated": summary.output_tokens_estimated,
            }
        }
        return UpstreamRequestModifications(body=updated_body, dynamic_metadata=dynamic_metadata)

    def _transform_text(self, text: str) -> tuple[str, TransformationSummary]:
        if OPEN_TAG not in text and CLOSE_TAG not in text:
            compressed_text, compressed = self._compress_segment(text)
            summary = TransformationSummary(
                selective_mode=False,
                tagged_segments=0,
                compressed_segments=1 if compressed else 0,
                input_tokens_estimated=estimate_tokens(text),
                output_tokens_estimated=estimate_tokens(compressed_text),
            )
            return compressed_text, summary

        transformed_text, tagged_segments, compressed_segments = self._apply_selective_compression(text)
        summary = TransformationSummary(
            selective_mode=True,
            tagged_segments=tagged_segments,
            compressed_segments=compressed_segments,
            input_tokens_estimated=estimate_tokens(text),
            output_tokens_estimated=estimate_tokens(transformed_text),
        )
        return transformed_text, summary

    def _apply_selective_compression(self, text: str) -> tuple[str, int, int]:
        output: list[str] = []
        inside_region: list[str] = []
        plain_start = 0
        depth = 0
        index = 0
        tagged_segments = 0
        compressed_segments = 0

        while index < len(text):
            if text.startswith(OPEN_TAG, index):
                if depth == 0:
                    output.append(text[plain_start:index])
                    inside_region = []
                    tagged_segments += 1
                depth += 1
                index += len(OPEN_TAG)
                continue

            if text.startswith(CLOSE_TAG, index):
                if depth > 0:
                    depth -= 1
                    index += len(CLOSE_TAG)
                    if depth == 0:
                        original_region = "".join(inside_region)
                        updated_region, compressed = self._compress_segment(original_region)
                        output.append(updated_region)
                        if compressed:
                            compressed_segments += 1
                        plain_start = index
                        inside_region = []
                    continue

                output.append(text[plain_start:index])
                index += len(CLOSE_TAG)
                plain_start = index
                continue

            if depth > 0:
                inside_region.append(text[index])
            index += 1

        if depth > 0:
            output.append("".join(inside_region))
            plain_start = len(text)

        if plain_start < len(text):
            output.append(text[plain_start:])

        return "".join(output), tagged_segments, compressed_segments

    def _compress_segment(self, text: str) -> tuple[str, bool]:
        estimated_tokens = estimate_tokens(text)
        if estimated_tokens <= 0:
            return text, False

        rule = select_rule(self._params.rules, estimated_tokens)
        if rule is None:
            return text, False

        compression_ratio = resolve_ratio(rule, estimated_tokens)
        if compression_ratio is None or compression_ratio >= 1.0:
            return text, False

        compressor = self._get_compressor(compression_ratio)
        try:
            result = compressor.compress(text)
        except (InputTooShortError, NegativeGainError) as exc:
            LOGGER.debug("PromptCompressor: skipped compression for segment: %s", exc)
            return text, False
        except CompressionError as exc:
            LOGGER.warning("PromptCompressor: compression failed for segment: %s", exc)
            return text, False
        except Exception as exc:  # pragma: no cover - defensive guard for library/runtime issues
            LOGGER.warning("PromptCompressor: unexpected compression failure: %s", exc)
            return text, False

        compressed_text = result.compressed or text
        return compressed_text, compressed_text != text

    def _get_compressor(self, compression_ratio: float) -> Compressor:
        cache_key = round(compression_ratio, 6)
        compressor = self._compressor_cache.get(cache_key)
        if compressor is None:
            compressor = Compressor(
                CompressorConfig(
                    target_ratio=cache_key,
                    min_input_tokens=1,
                    min_input_bytes=1,
                )
            )
            self._compressor_cache[cache_key] = compressor
        return compressor


def get_policy(metadata, params):
    normalized = normalize_params(params)
    return PromptCompressorPolicy(normalized)


def normalize_params(params: dict[str, Any] | None) -> PolicyParams:
    params = params or {}

    json_path = params.get("jsonPath", DEFAULT_JSON_PATH)
    if not isinstance(json_path, str) or not json_path.strip():
        LOGGER.warning(
            "PromptCompressor: invalid jsonPath %r, falling back to %s",
            json_path,
            DEFAULT_JSON_PATH,
        )
        json_path = DEFAULT_JSON_PATH
    else:
        json_path = json_path.strip()

    raw_rules = params.get("rules", [])
    if not isinstance(raw_rules, list):
        LOGGER.warning("PromptCompressor: rules must be a list, got %s", type(raw_rules).__name__)
        raw_rules = []

    normalized_rules: list[CompressionRule] = []
    for index, raw_rule in enumerate(raw_rules):
        rule = normalize_rule(raw_rule)
        if rule is None:
            LOGGER.warning("PromptCompressor: dropping invalid rule at index %d: %r", index, raw_rule)
            continue
        normalized_rules.append(rule)

    normalized_rules.sort(key=lambda rule: (rule.upper_token_limit == -1, rule.upper_token_limit))

    return PolicyParams(
        json_path=json_path,
        rules=tuple(normalized_rules),
    )


def normalize_rule(raw_rule: Any) -> CompressionRule | None:
    if not isinstance(raw_rule, dict):
        return None

    upper_token_limit = coerce_int(raw_rule.get("upperTokenLimit"))
    rule_type = raw_rule.get("type")
    value = coerce_float(raw_rule.get("value"))

    if upper_token_limit is None or rule_type not in {"ratio", "token"} or value is None:
        return None

    if upper_token_limit < -1:
        return None

    if rule_type == "token" and value <= 0:
        return None

    if rule_type == "ratio" and value <= 0:
        return None

    return CompressionRule(
        upper_token_limit=upper_token_limit,
        rule_type=rule_type,
        value=value,
    )


def coerce_int(value: Any) -> int | None:
    if isinstance(value, bool):
        return None
    if isinstance(value, int):
        return value
    if isinstance(value, float):
        if value.is_integer():
            return int(value)
        return None
    if isinstance(value, str):
        try:
            float_value = float(value)
        except ValueError:
            return None
        if float_value.is_integer():
            return int(float_value)
    return None


def coerce_float(value: Any) -> float | None:
    if isinstance(value, bool):
        return None
    if isinstance(value, (int, float)):
        return float(value)
    if isinstance(value, str):
        try:
            return float(value)
        except ValueError:
            return None
    return None


def estimate_tokens(text: str) -> int:
    return len(text) // 4


def select_rule(rules: tuple[CompressionRule, ...], estimated_tokens: int) -> CompressionRule | None:
    for rule in rules:
        if rule.upper_token_limit == -1 or estimated_tokens <= rule.upper_token_limit:
            return rule
    return None


def resolve_ratio(rule: CompressionRule, estimated_tokens: int) -> float | None:
    if estimated_tokens <= 0:
        return None

    if rule.rule_type == "ratio":
        return min(rule.value, 1.0)

    target_tokens = int(rule.value)
    if target_tokens <= 0 or target_tokens >= estimated_tokens:
        return None
    return target_tokens / estimated_tokens


def extract_string_value_from_jsonpath(data: Any, json_path: str) -> str:
    value = extract_value_from_jsonpath(data, json_path)
    if not isinstance(value, str):
        raise JSONPathError("value at JSONPath is not a string")
    return value


def extract_value_from_jsonpath(data: Any, json_path: str) -> Any:
    current = data
    for component in split_json_path(json_path):
        current = get_path_component(current, component)
    return current


def set_value_at_jsonpath(data: Any, json_path: str, value: str) -> None:
    components = split_json_path(json_path)
    if not components:
        raise JSONPathError("empty JSONPath")

    current = data
    for component in components[:-1]:
        current = get_path_component(current, component)

    final_component = components[-1]
    match = _ARRAY_INDEX_RE.match(final_component)
    if match:
        array_name, index = parse_array_component(final_component)
        if not isinstance(current, dict):
            raise JSONPathError(f"invalid structure for key: {array_name}")
        if array_name not in current:
            raise JSONPathError(f"key not found: {array_name}")
        array_value = current.get(array_name)
        if not isinstance(array_value, list):
            raise JSONPathError(f"not an array: {array_name}")
        try:
            array_value[index] = value
        except IndexError as exc:
            raise JSONPathError(f"array index out of range: {index}") from exc
        return

    if not isinstance(current, dict):
        raise JSONPathError(f"invalid structure for key: {final_component}")
    if final_component not in current:
        raise JSONPathError(f"key not found: {final_component}")
    current[final_component] = value


def split_json_path(json_path: str) -> list[str]:
    if not isinstance(json_path, str):
        raise JSONPathError("jsonPath must be a string")
    parts = json_path.split(".")
    if parts and parts[0] == "$":
        parts = parts[1:]
    if not parts or any(not part for part in parts):
        raise JSONPathError("empty JSONPath")
    return parts


def get_path_component(current: Any, component: str) -> Any:
    match = _ARRAY_INDEX_RE.match(component)
    if match:
        array_name, index = parse_array_component(component)
        if not isinstance(current, dict):
            raise JSONPathError(f"invalid structure for key: {array_name}")
        if array_name not in current:
            raise JSONPathError(f"key not found: {array_name}")
        array_value = current.get(array_name)
        if not isinstance(array_value, list):
            raise JSONPathError(f"not an array: {array_name}")
        try:
            return array_value[index]
        except IndexError as exc:
            raise JSONPathError(f"array index out of range: {index}") from exc

    if not isinstance(current, dict):
        raise JSONPathError(f"invalid structure for key: {component}")
    if component not in current:
        raise JSONPathError(f"key not found: {component}")
    return current[component]


def parse_array_component(component: str) -> tuple[str, int]:
    match = _ARRAY_INDEX_RE.match(component)
    if not match:
        raise JSONPathError(f"invalid array component: {component}")

    name = match.group(1)
    raw_index = int(match.group(2))
    return name, raw_index
