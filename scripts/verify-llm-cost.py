#!/usr/bin/env python3
"""
Verify cost calculation parity between our gateway implementation and LiteLLM
across all supported providers.

Usage:
    python3 scripts/verify-llm-cost.py [--provider anthropic|openai|gemini|mistral]

Requirements:
    pip install litellm
  or from source:
    pip install -e /path/to/litellm

Adding a new provider:
    1. Create a class that extends Provider (see existing providers below).
    2. Implement test_cases(), our_cost(), and litellm_cost().
    3. Register it by adding to PROVIDERS at the bottom of this file.
"""

import json
import os
import sys
from abc import ABC, abstractmethod
from typing import List, Optional, Tuple

# ── Paths ─────────────────────────────────────────────────────────────────────

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PRICES_PATH = os.path.join(
    REPO_ROOT, "gateway", "system-policies", "llm-cost", "pricing", "model_prices.json"
)

TOLERANCE = 1e-9
PASS_STR = "\033[92m✓ PASS\033[0m"
FAIL_STR = "\033[91m✗ FAIL\033[0m"
SKIP_STR = "\033[93m? SKIP\033[0m"


def load_prices() -> dict:
    with open(PRICES_PATH) as f:
        return json.load(f)


# ── Provider base class ───────────────────────────────────────────────────────

class Provider(ABC):
    """Base class for a provider verification plugin."""

    @property
    @abstractmethod
    def name(self) -> str:
        """Human-readable provider name (e.g. 'Anthropic')."""

    @abstractmethod
    def test_cases(self) -> List[dict]:
        """Return list of test case dicts. Each dict must have a 'name' key."""

    @abstractmethod
    def our_cost(self, tc: dict, prices: dict) -> Tuple[Optional[float], Optional[str]]:
        """Re-implement our Go logic. Return (cost, None) or (None, error_msg)."""

    @abstractmethod
    def litellm_cost(self, tc: dict) -> Tuple[Optional[float], Optional[str]]:
        """Call LiteLLM. Return (cost, None) or (None, error_msg)."""


# ── Shared Go genericCalculateCost logic ─────────────────────────────────────

def generic_calculate_cost(
    prompt_tokens: int,
    completion_tokens: int,
    cache_write_5m: int,
    cache_write_1hr: int,
    cache_read: int,
    input_tokens_for_tiering: int,
    tiering_threshold: int,
    prices: dict,
) -> Tuple[float, Optional[str]]:
    """
    Python re-implementation of our Go genericCalculateCost().
    Handles tiering, cache token separation, and cost assembly.
    Web search and geo/speed adjustments are provider-specific — apply on top.
    """
    tiered = input_tokens_for_tiering > tiering_threshold

    if tiered:
        input_rate  = prices.get("input_cost_per_token_above_200k_tokens",
                                  prices.get("input_cost_per_token", 0))
        output_rate = prices.get("output_cost_per_token_above_200k_tokens",
                                  prices.get("output_cost_per_token", 0))
        cw5m_rate   = prices.get("cache_creation_input_token_cost_above_200k_tokens",
                                  prices.get("cache_creation_input_token_cost", 0))
        cw1hr_rate  = cw5m_rate   # collapse to same tiered rate
        cr_rate     = prices.get("cache_read_input_token_cost_above_200k_tokens",
                                  prices.get("cache_read_input_token_cost", 0))
    else:
        input_rate  = prices.get("input_cost_per_token", 0)
        output_rate = prices.get("output_cost_per_token", 0)
        cw5m_rate   = prices.get("cache_creation_input_token_cost", 0)
        cw1hr_rate  = prices.get("cache_creation_input_token_cost_above_1hr", cw5m_rate)
        cr_rate     = prices.get("cache_read_input_token_cost", 0)

    regular = max(0, prompt_tokens - cache_write_5m - cache_write_1hr - cache_read)
    cost = (regular        * input_rate
            + cache_write_5m  * cw5m_rate
            + cache_write_1hr * cw1hr_rate
            + cache_read      * cr_rate
            + completion_tokens * output_rate)

    return cost, None


# ═══════════════════════════════════════════════════════════════════════════════
# Provider: Anthropic
# ═══════════════════════════════════════════════════════════════════════════════

class AnthropicProvider(Provider):

    @property
    def name(self) -> str:
        return "Anthropic"

    def test_cases(self) -> List[dict]:
        # Convention (matches our Normalize()):
        #   input_tokens                – regular non-cached input only (Anthropic convention)
        #   output_tokens
        #   cache_creation_input_tokens – total cache writes (5m + 1hr summed)
        #   cache_read_input_tokens
        #   ephemeral_5m                – subset of cache_creation for 5-minute TTL
        #   ephemeral_1h                – subset of cache_creation for 1-hour TTL
        #   web_search_requests         – from usage.server_tool_use.web_search_requests
        #   inference_geo               – from usage.inference_geo (None = global)
        #   speed                       – from request body (None = not fast)
        def tc(name, model="claude-opus-4-6", **kw):
            defaults = dict(
                input_tokens=0, output_tokens=0,
                cache_creation_input_tokens=0, cache_read_input_tokens=0,
                ephemeral_5m=0, ephemeral_1h=0,
                web_search_requests=0, inference_geo=None, speed=None,
            )
            return {"name": name, "model": model, **defaults, **kw}

        return [
            # Basic — no cache, no geo/speed
            tc("basic_opus",   input_tokens=100, output_tokens=50),
            tc("basic_sonnet", model="claude-3-5-sonnet-20241022",
               input_tokens=200, output_tokens=100),
            tc("basic_haiku",  model="claude-3-5-haiku-20241022",
               input_tokens=500, output_tokens=250),
            tc("zero_tokens"),
            tc("zero_input_only_output", output_tokens=100),

            # Cache read
            tc("cache_read_only",             input_tokens=100, output_tokens=50,
               cache_read_input_tokens=300),
            tc("cache_read_zero_regular",     output_tokens=50,
               cache_read_input_tokens=500),

            # Cache write — 5-minute TTL
            tc("cache_write_5m",              input_tokens=100, output_tokens=50,
               cache_creation_input_tokens=500, ephemeral_5m=500),

            # Cache write — 1-hour TTL
            tc("cache_write_1hr",             input_tokens=100, output_tokens=50,
               cache_creation_input_tokens=500, ephemeral_1h=500),

            # Mixed cache write (5m + 1hr)
            tc("cache_write_mixed",           input_tokens=10, output_tokens=5,
               cache_creation_input_tokens=600, ephemeral_5m=100, ephemeral_1h=500),

            # No TTL breakdown → fallback to treating all as 5m
            tc("cache_write_no_ttl_breakdown", input_tokens=100, output_tokens=50,
               cache_creation_input_tokens=400),

            # Cache read + cache write
            tc("cache_read_and_write_5m",     input_tokens=50, output_tokens=25,
               cache_creation_input_tokens=200, cache_read_input_tokens=400,
               ephemeral_5m=200),
            tc("cache_read_and_write_mixed",  input_tokens=50, output_tokens=25,
               cache_creation_input_tokens=600, cache_read_input_tokens=400,
               ephemeral_5m=100, ephemeral_1h=500),

            # Geo multiplier
            tc("geo_us",             input_tokens=100, output_tokens=50, inference_geo="us"),
            tc("geo_global",         input_tokens=100, output_tokens=50, inference_geo="global"),
            tc("geo_not_available",  input_tokens=100, output_tokens=50, inference_geo="not_available"),

            # Speed multiplier
            tc("speed_fast",         input_tokens=100, output_tokens=50, speed="fast"),

            # Geo + speed combined
            tc("geo_and_speed",      input_tokens=20, output_tokens=10,
               inference_geo="us", speed="fast"),

            # Geo with cache carve-out
            tc("geo_with_cache",     input_tokens=100, output_tokens=50,
               cache_creation_input_tokens=200, cache_read_input_tokens=300,
               ephemeral_5m=200, inference_geo="us"),
            tc("geo_speed_mixed_cache", input_tokens=50, output_tokens=20,
               cache_creation_input_tokens=100, cache_read_input_tokens=150,
               ephemeral_5m=50, ephemeral_1h=50, inference_geo="us", speed="fast"),

            # Long context (>200k) tiering
            tc("long_context_200k",           input_tokens=250_000, output_tokens=5_000),
            tc("long_context_with_cache",     input_tokens=150_000, output_tokens=3_000,
               cache_creation_input_tokens=80_000, cache_read_input_tokens=40_000,
               ephemeral_5m=80_000),
            tc("long_context_at_threshold",   input_tokens=200_000, output_tokens=1_000),
            tc("long_context_above_threshold",input_tokens=200_001, output_tokens=1_000),

            # Web search (always billed at medium rate for Anthropic)
            tc("web_search_2_queries",        input_tokens=50, output_tokens=25,
               web_search_requests=2),
            tc("web_search_zero",             input_tokens=50, output_tokens=25),
            tc("web_search_with_geo_speed",   input_tokens=50, output_tokens=25,
               web_search_requests=3, inference_geo="us", speed="fast"),

            # All features combined
            tc("all_features",                input_tokens=100, output_tokens=50,
               cache_creation_input_tokens=600, cache_read_input_tokens=300,
               ephemeral_5m=100, ephemeral_1h=500,
               web_search_requests=2, inference_geo="us", speed="fast"),

            # Other models with cache
            tc("sonnet_with_cache", model="claude-3-5-sonnet-20241022",
               input_tokens=100, output_tokens=50,
               cache_creation_input_tokens=300, cache_read_input_tokens=200,
               ephemeral_5m=200, ephemeral_1h=100, web_search_requests=1),
            tc("haiku_with_cache_and_search", model="claude-3-5-haiku-20241022",
               input_tokens=200, output_tokens=100,
               cache_creation_input_tokens=400, cache_read_input_tokens=100,
               ephemeral_5m=400, web_search_requests=2),
        ]

    def our_cost(self, tc: dict, prices: dict) -> Tuple[Optional[float], Optional[str]]:
        p = prices.get(tc["model"])
        if p is None:
            return None, f"model '{tc['model']}' not in model_prices.json"

        cache_creation_total = tc["cache_creation_input_tokens"]
        cache_read           = tc["cache_read_input_tokens"]
        cache_5m             = tc["ephemeral_5m"]
        cache_1hr            = tc["ephemeral_1h"]
        web_search           = tc["web_search_requests"]
        inference_geo        = tc["inference_geo"]
        speed                = tc["speed"]

        # Anthropic convention: PromptTokens = input + cache_creation + cache_read
        prompt_tokens = tc["input_tokens"] + cache_creation_total + cache_read

        # No per-TTL breakdown → treat all cache writes as 5m
        if cache_5m == 0 and cache_1hr == 0 and cache_creation_total > 0:
            cache_5m = cache_creation_total

        # InputTokensForTiering = PromptTokens for Anthropic
        cost, err = generic_calculate_cost(
            prompt_tokens=prompt_tokens,
            completion_tokens=tc["output_tokens"],
            cache_write_5m=cache_5m,
            cache_write_1hr=cache_1hr,
            cache_read=cache_read,
            input_tokens_for_tiering=prompt_tokens,
            tiering_threshold=200_000,
            prices=p,
        )
        if err:
            return None, err

        # Web search cost (not affected by geo/speed multiplier)
        search_pricing = p.get("search_context_cost_per_query", {})
        web_cost = web_search * search_pricing.get("search_context_size_medium", 0.0)
        cost += web_cost

        # Geo/speed multiplier with cache carve-out (Anthropic-specific)
        pse = p.get("provider_specific_entry", {})
        if pse:
            multiplier = 1.0
            if inference_geo and inference_geo.lower() not in ("global", "not_available"):
                multiplier *= pse.get(inference_geo.lower(), 1.0)
            if speed == "fast":
                multiplier *= pse.get("fast", 1.0)
            if multiplier != 1.0:
                tiered = prompt_tokens > 200_000
                cw5m_rate = (p.get("cache_creation_input_token_cost_above_200k_tokens",
                                   p.get("cache_creation_input_token_cost", 0)) if tiered
                             else p.get("cache_creation_input_token_cost", 0))
                cw1hr_rate = (cw5m_rate if tiered
                              else p.get("cache_creation_input_token_cost_above_1hr", cw5m_rate))
                cr_rate = (p.get("cache_read_input_token_cost_above_200k_tokens",
                                  p.get("cache_read_input_token_cost", 0)) if tiered
                           else p.get("cache_read_input_token_cost", 0))
                cache_cost = (cache_5m * cw5m_rate + cache_1hr * cw1hr_rate
                              + cache_read * cr_rate)
                non_cache_non_web = cost - cache_cost - web_cost
                cost = non_cache_non_web * multiplier + cache_cost + web_cost

        return cost, None

    def litellm_cost(self, tc: dict) -> Tuple[Optional[float], Optional[str]]:
        try:
            import litellm
            from litellm.llms.anthropic.cost_calculation import (
                cost_per_token as anthropic_cost_per_token,
                get_cost_for_anthropic_web_search,
            )
            from litellm.types.utils import (
                CacheCreationTokenDetails, PromptTokensDetailsWrapper,
                ServerToolUse, Usage,
            )
        except ImportError as e:
            return None, f"litellm import error: {e}"

        cache_creation_total = tc["cache_creation_input_tokens"]
        cache_read  = tc["cache_read_input_tokens"]
        cache_5m    = tc["ephemeral_5m"]
        cache_1hr   = tc["ephemeral_1h"]
        web_search  = tc["web_search_requests"]

        total_prompt = tc["input_tokens"] + cache_creation_total + cache_read

        # Always set cache_creation_tokens=total so LiteLLM correctly computes
        # text_tokens = prompt_tokens - cache_creation - cache_read.
        # calculate_cache_writing_cost then uses cache_creation_token_details
        # (when present) and ignores cache_creation_tokens for the cost itself.
        cache_details = None
        if cache_5m > 0 or cache_1hr > 0:
            cache_details = CacheCreationTokenDetails(
                ephemeral_5m_input_tokens=cache_5m,
                ephemeral_1h_input_tokens=cache_1hr,
            )

        ptd = PromptTokensDetailsWrapper(
            cached_tokens=cache_read,
            cache_creation_tokens=cache_creation_total,
            cache_creation_token_details=cache_details,
        )
        server_tool = ServerToolUse(web_search_requests=web_search) if web_search > 0 else None
        usage = Usage(
            prompt_tokens=total_prompt,
            completion_tokens=tc["output_tokens"],
            total_tokens=total_prompt + tc["output_tokens"],
            prompt_tokens_details=ptd,
            server_tool_use=server_tool,
        )
        if tc["inference_geo"]:
            usage.inference_geo = tc["inference_geo"]
        if tc["speed"]:
            usage.speed = tc["speed"]

        try:
            prompt_cost, completion_cost = anthropic_cost_per_token(
                model=tc["model"], usage=usage
            )
        except Exception as e:
            return None, f"litellm cost_per_token error: {e}"

        total_cost = prompt_cost + completion_cost
        try:
            model_info = litellm.get_model_info(model=tc["model"],
                                                custom_llm_provider="anthropic")
            total_cost += get_cost_for_anthropic_web_search(
                model_info=model_info, usage=usage
            )
        except Exception:
            pass

        return total_cost, None


# ═══════════════════════════════════════════════════════════════════════════════
# Provider: OpenAI
# ═══════════════════════════════════════════════════════════════════════════════

class OpenAIProvider(Provider):

    @property
    def name(self) -> str:
        return "OpenAI"

    def test_cases(self) -> List[dict]:
        # OpenAI convention: prompt_tokens INCLUDES cached tokens (unlike Anthropic).
        # cached_tokens is a subset reported in prompt_tokens_details.
        def tc(name, model="gpt-4o-2024-11-20", **kw):
            defaults = dict(prompt_tokens=0, completion_tokens=0, cached_tokens=0)
            return {"name": name, "model": model, **defaults, **kw}

        return [
            tc("basic",                       prompt_tokens=100,  completion_tokens=50),
            tc("basic_4o_mini", model="gpt-4o-mini-2024-07-18",
               prompt_tokens=500,  completion_tokens=200),
            tc("zero_tokens"),
            tc("cached_tokens",               prompt_tokens=1000, completion_tokens=100,
               cached_tokens=800),
            tc("fully_cached",                prompt_tokens=500,  completion_tokens=100,
               cached_tokens=500),
            tc("large_request",               prompt_tokens=10_000, completion_tokens=2_000),
            tc("large_with_cache",            prompt_tokens=10_000, completion_tokens=2_000,
               cached_tokens=5_000),
        ]

    def our_cost(self, tc: dict, prices: dict) -> Tuple[Optional[float], Optional[str]]:
        p = prices.get(tc["model"])
        if p is None:
            return None, f"model '{tc['model']}' not in model_prices.json"

        prompt_tokens      = tc["prompt_tokens"]
        completion_tokens  = tc["completion_tokens"]
        cached_tokens      = tc["cached_tokens"]

        # OpenAI: PromptTokens includes cached; genericCalculateCost subtracts
        cost, err = generic_calculate_cost(
            prompt_tokens=prompt_tokens,
            completion_tokens=completion_tokens,
            cache_write_5m=0,
            cache_write_1hr=0,
            cache_read=cached_tokens,
            input_tokens_for_tiering=prompt_tokens,
            tiering_threshold=200_000,
            prices=p,
        )
        return cost, err

    def litellm_cost(self, tc: dict) -> Tuple[Optional[float], Optional[str]]:
        try:
            from litellm.litellm_core_utils.llm_cost_calc.utils import generic_cost_per_token
            from litellm.types.utils import PromptTokensDetailsWrapper, Usage
        except ImportError as e:
            return None, f"litellm import error: {e}"

        ptd = PromptTokensDetailsWrapper(cached_tokens=tc["cached_tokens"])
        usage = Usage(
            prompt_tokens=tc["prompt_tokens"],
            completion_tokens=tc["completion_tokens"],
            total_tokens=tc["prompt_tokens"] + tc["completion_tokens"],
            prompt_tokens_details=ptd,
        )
        try:
            prompt_cost, completion_cost = generic_cost_per_token(
                model=tc["model"], usage=usage, custom_llm_provider="openai"
            )
            return prompt_cost + completion_cost, None
        except Exception as e:
            return None, str(e)


# ═══════════════════════════════════════════════════════════════════════════════
# Provider: Gemini
# ═══════════════════════════════════════════════════════════════════════════════

class GeminiProvider(Provider):

    @property
    def name(self) -> str:
        return "Gemini"

    def test_cases(self) -> List[dict]:
        # Gemini: promptTokenCount INCLUDES cachedContentTokenCount.
        # Tiering threshold: 128k (not 200k).
        def tc(name, model="gemini/gemini-1.5-pro", **kw):
            defaults = dict(prompt_tokens=0, completion_tokens=0, cached_tokens=0,
                           audio_input_tokens=0, audio_output_tokens=0, image_output_tokens=0)
            return {"name": name, "model": model, **defaults, **kw}

        return [
            tc("basic",                     prompt_tokens=100,     completion_tokens=50),
            tc("basic_flash", model="gemini/gemini-1.5-flash",
               prompt_tokens=500,     completion_tokens=200),
            tc("zero_tokens"),
            tc("with_cache",                prompt_tokens=5_000,   completion_tokens=100,
               cached_tokens=4_000),
            tc("below_128k",                prompt_tokens=100_000, completion_tokens=1_000),
            tc("above_128k",                prompt_tokens=150_000, completion_tokens=2_000),
            tc("above_128k_with_cache",     prompt_tokens=200_000, completion_tokens=5_000,
               cached_tokens=100_000),
            # Audio input tokens (Gemini 2.5 Flash has a separate audio input rate)
            tc("audio_input", model="gemini/gemini-2.5-flash",
               prompt_tokens=500, completion_tokens=100,
               audio_input_tokens=200),
            # Audio output tokens (Gemini Live native audio models)
            tc("audio_output", model="gemini-live-2.5-flash-preview-native-audio-09-2025",
               prompt_tokens=100, completion_tokens=300,
               audio_output_tokens=200),
            # Image output tokens (Gemini image generation models)
            tc("image_output", model="gemini-2.5-flash-image",
               prompt_tokens=200, completion_tokens=350,
               image_output_tokens=300),
        ]

    def our_cost(self, tc: dict, prices: dict) -> Tuple[Optional[float], Optional[str]]:
        # Gemini model names in our JSON use the "gemini/" prefix
        model_key = tc["model"]
        # Try exact key, then gemini/ prefix, then bare name
        p = prices.get(model_key) or prices.get("gemini/" + model_key) or prices.get(
            model_key.removeprefix("gemini/"))
        if p is None:
            return None, f"model '{model_key}' not in model_prices.json"

        prompt_tokens      = tc["prompt_tokens"]
        completion_tokens  = tc["completion_tokens"]
        cached_tokens      = tc["cached_tokens"]
        audio_input_tokens = tc.get("audio_input_tokens", 0)
        audio_output_tokens = tc.get("audio_output_tokens", 0)
        image_output_tokens = tc.get("image_output_tokens", 0)

        # Gemini tiering: >128k tokens (total prompt+completion)
        total_tokens = prompt_tokens + completion_tokens
        tiered = total_tokens > 128_000
        if tiered:
            input_rate  = p.get("input_cost_per_token_above_128k_tokens",
                                 p.get("input_cost_per_token", 0))
            output_rate = p.get("output_cost_per_token_above_128k_tokens",
                                 p.get("output_cost_per_token", 0))
            cr_rate     = p.get("cache_read_input_token_cost_above_128k_tokens",
                                 p.get("cache_read_input_token_cost", 0))
        else:
            input_rate  = p.get("input_cost_per_token", 0)
            output_rate = p.get("output_cost_per_token", 0)
            cr_rate     = p.get("cache_read_input_token_cost", 0)

        # Audio input rate (falls back to standard input rate when absent)
        audio_in_rate  = p.get("input_cost_per_audio_token", input_rate)
        # Audio output rate (falls back to standard output rate)
        audio_out_rate = p.get("output_cost_per_audio_token", output_rate)
        # Image output rate (falls back to standard output rate)
        image_out_rate = p.get("output_cost_per_image_token", output_rate)

        # Regular text tokens (exclude audio input so it's billed separately)
        regular_prompt = max(0, prompt_tokens - cached_tokens - audio_input_tokens)
        # Regular text output (exclude audio/image output)
        regular_output = max(0, completion_tokens - audio_output_tokens - image_output_tokens)

        cost = (regular_prompt * input_rate
                + cached_tokens * cr_rate
                + audio_input_tokens * audio_in_rate
                + regular_output * output_rate
                + audio_output_tokens * audio_out_rate
                + image_output_tokens * image_out_rate)
        return cost, None

    def litellm_cost(self, tc: dict) -> Tuple[Optional[float], Optional[str]]:
        try:
            from litellm.litellm_core_utils.llm_cost_calc.utils import generic_cost_per_token
            from litellm.types.utils import (PromptTokensDetailsWrapper,
                                             CompletionTokensDetailsWrapper, Usage)
        except ImportError as e:
            return None, f"litellm import error: {e}"

        # Strip "gemini/" prefix for LiteLLM model lookup
        model = tc["model"].removeprefix("gemini/")

        audio_input_tokens  = tc.get("audio_input_tokens", 0)
        audio_output_tokens = tc.get("audio_output_tokens", 0)
        image_output_tokens = tc.get("image_output_tokens", 0)

        ptd = PromptTokensDetailsWrapper(
            cached_tokens=tc["cached_tokens"],
            audio_tokens=audio_input_tokens if audio_input_tokens else None,
        )
        ctd = CompletionTokensDetailsWrapper(
            audio_tokens=audio_output_tokens if audio_output_tokens else None,
            image_tokens=image_output_tokens if image_output_tokens else None,
        )
        usage = Usage(
            prompt_tokens=tc["prompt_tokens"],
            completion_tokens=tc["completion_tokens"],
            total_tokens=tc["prompt_tokens"] + tc["completion_tokens"],
            prompt_tokens_details=ptd,
            completion_tokens_details=ctd if (audio_output_tokens or image_output_tokens) else None,
        )
        try:
            prompt_cost, completion_cost = generic_cost_per_token(
                model=model, usage=usage, custom_llm_provider="gemini"
            )
            return prompt_cost + completion_cost, None
        except Exception as e:
            return None, str(e)


# ═══════════════════════════════════════════════════════════════════════════════
# Provider: Mistral
# ═══════════════════════════════════════════════════════════════════════════════

class MistralProvider(Provider):

    @property
    def name(self) -> str:
        return "Mistral"

    def test_cases(self) -> List[dict]:
        # Mistral: simple input + output, no caching, no multipliers.
        def tc(name, model="mistral/mistral-large-latest", **kw):
            defaults = dict(prompt_tokens=0, completion_tokens=0)
            return {"name": name, "model": model, **defaults, **kw}

        return [
            tc("basic",                  prompt_tokens=100,    completion_tokens=50),
            tc("basic_small", model="mistral/mistral-small-latest",
               prompt_tokens=500,    completion_tokens=200),
            tc("zero_tokens"),
            tc("large_request",          prompt_tokens=10_000, completion_tokens=2_000),
            tc("only_output",            completion_tokens=100),
        ]

    def our_cost(self, tc: dict, prices: dict) -> Tuple[Optional[float], Optional[str]]:
        p = prices.get(tc["model"])
        if p is None:
            return None, f"model '{tc['model']}' not in model_prices.json"

        cost = (tc["prompt_tokens"]     * p.get("input_cost_per_token", 0)
                + tc["completion_tokens"] * p.get("output_cost_per_token", 0))
        return cost, None

    def litellm_cost(self, tc: dict) -> Tuple[Optional[float], Optional[str]]:
        try:
            from litellm.litellm_core_utils.llm_cost_calc.utils import generic_cost_per_token
            from litellm.types.utils import Usage
        except ImportError as e:
            return None, f"litellm import error: {e}"

        model = tc["model"].removeprefix("mistral/")
        usage = Usage(
            prompt_tokens=tc["prompt_tokens"],
            completion_tokens=tc["completion_tokens"],
            total_tokens=tc["prompt_tokens"] + tc["completion_tokens"],
        )
        try:
            prompt_cost, completion_cost = generic_cost_per_token(
                model=model, usage=usage, custom_llm_provider="mistral"
            )
            return prompt_cost + completion_cost, None
        except Exception as e:
            return None, str(e)


# ── Registered providers ──────────────────────────────────────────────────────
# Add new providers here. Each entry is instantiated once at startup.

PROVIDERS: List[Provider] = [
    AnthropicProvider(),
    OpenAIProvider(),
    GeminiProvider(),
    MistralProvider(),
]


# ── Runner ────────────────────────────────────────────────────────────────────

def run_provider(provider: Provider, prices: dict) -> Tuple[int, int, int]:
    """Run all test cases for a provider. Returns (passed, failed, skipped)."""
    col_name  = 38
    col_our   = 18
    col_ll    = 18
    col_delta = 14
    header = (f"{'Test Case':<{col_name}} {'Our Cost':>{col_our}} "
              f"{'LiteLLM Cost':>{col_ll}} {'Delta':>{col_delta}}  Result")
    sep = "─" * len(header)

    print(f"\n{'═' * len(header)}")
    print(f"  {provider.name}")
    print(f"{'═' * len(header)}")
    print(header)
    print(sep)

    passed = failed = skipped = 0
    for tc in provider.test_cases():
        name = tc["name"]
        our_cost, our_err = provider.our_cost(tc, prices)
        ll_cost,  ll_err  = provider.litellm_cost(tc)

        if our_err or ll_err:
            err = our_err or ll_err
            print(f"{name:<{col_name}} {'N/A':>{col_our}} {'N/A':>{col_ll}} "
                  f"{'N/A':>{col_delta}}  {SKIP_STR}  ({err})")
            skipped += 1
            continue

        delta = abs(our_cost - ll_cost)
        ok = delta <= TOLERANCE
        result = PASS_STR if ok else FAIL_STR
        passed += ok
        failed += not ok

        print(f"{name:<{col_name}} {our_cost:>{col_our}.10f} "
              f"{ll_cost:>{col_ll}.10f} {delta:>{col_delta}.2e}  {result}")

    print(sep)
    print(f"  {passed} passed, {failed} failed, {skipped} skipped "
          f"({len(provider.test_cases())} cases)")
    return passed, failed, skipped


def main():
    import argparse
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--provider", metavar="NAME",
                        help="Run only this provider (default: all)")
    args = parser.parse_args()

    prices = load_prices()

    providers = PROVIDERS
    if args.provider:
        name_lower = args.provider.lower()
        providers = [p for p in PROVIDERS if p.name.lower() == name_lower]
        if not providers:
            print(f"Unknown provider '{args.provider}'. "
                  f"Available: {', '.join(p.name for p in PROVIDERS)}")
            sys.exit(1)

    total_passed = total_failed = total_skipped = 0
    for provider in providers:
        p, f, s = run_provider(provider, prices)
        total_passed  += p
        total_failed  += f
        total_skipped += s

    total = total_passed + total_failed + total_skipped
    print(f"\n{'━' * 60}")
    print(f"  TOTAL: {total_passed} passed, {total_failed} failed, "
          f"{total_skipped} skipped out of {total} cases")
    print(f"{'━' * 60}")

    if total_failed:
        sys.exit(1)


if __name__ == "__main__":
    main()
