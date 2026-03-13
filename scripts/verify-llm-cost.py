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
    REPO_ROOT, "gateway", "configs", "llm-pricing", "model_prices.json"
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
    service_tier: str = "",
    tiering_threshold_272k: int = 0,
    web_search_requests: int = 0,
    search_context_size: str = "",
    audio_input_tokens: int = 0,
    audio_output_tokens: int = 0,
) -> Tuple[float, Optional[str]]:
    """
    Python re-implementation of our Go genericCalculateCost().
    Handles tiering, cache token separation, service tier (flex/priority/batch),
    the above-272k context window tier (gpt-5.4, gpt-5.4-pro), web search
    tool call billing (flat rate or context-size-based), and audio token billing.

    service_tier: "" (standard) | "flex" | "priority" | "batch"
    tiering_threshold_272k: when > 0, tokens above this threshold use the
        _above_272k_tokens rate variants (OpenAI 1.05M context window models).
    web_search_requests: number of web search tool calls (0 = none).
    search_context_size: "low" | "medium" | "high"; defaults to "medium" for
        models with search_context_cost_per_query (matching LiteLLM behaviour).
    audio_input_tokens: audio tokens within prompt_tokens (billed at audio rate).
    audio_output_tokens: audio tokens within completion_tokens (billed at audio rate).
    """
    above_272k = tiering_threshold_272k > 0 and input_tokens_for_tiering > tiering_threshold_272k
    tiered = input_tokens_for_tiering > tiering_threshold

    if above_272k:
        input_rate  = prices.get("input_cost_per_token_above_272k_tokens",
                                  prices.get("input_cost_per_token", 0))
        output_rate = prices.get("output_cost_per_token_above_272k_tokens",
                                  prices.get("output_cost_per_token", 0))
        cr_rate     = prices.get("cache_read_input_token_cost_above_272k_tokens",
                                  prices.get("cache_read_input_token_cost", 0))
        cw5m_rate   = prices.get("cache_creation_input_token_cost", 0)
        cw1hr_rate  = cw5m_rate
    elif tiered:
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

    # Service tier overrides — mirrors Go's genericCalculateCost switch block.
    # Priority: check narrowest threshold first (272k → 200k → standard).
    # Flex/batch: apply their flat rate variants regardless of threshold.
    # Note: LiteLLM's generic_cost_per_token only supports "flex" and "priority"
    #       via its service_tier param; "batch" is not wired to _batches fields there.
    if service_tier == "priority":
        if above_272k:
            input_rate  = prices.get("input_cost_per_token_above_272k_tokens_priority", input_rate)
            output_rate = prices.get("output_cost_per_token_above_272k_tokens_priority", output_rate)
            cr_rate     = prices.get("cache_read_input_token_cost_above_272k_tokens_priority", cr_rate)
        elif tiered:
            input_rate  = prices.get("input_cost_per_token_above_200k_tokens_priority", input_rate)
            output_rate = prices.get("output_cost_per_token_above_200k_tokens_priority", output_rate)
            cr_rate     = prices.get("cache_read_input_token_cost_above_200k_tokens_priority", cr_rate)
        else:
            input_rate  = prices.get("input_cost_per_token_priority", input_rate)
            output_rate = prices.get("output_cost_per_token_priority", output_rate)
            cr_rate     = prices.get("cache_read_input_token_cost_priority", cr_rate)
    elif service_tier == "flex":
        input_rate  = prices.get("input_cost_per_token_flex", input_rate)
        output_rate = prices.get("output_cost_per_token_flex", output_rate)
        cr_rate     = prices.get("cache_read_input_token_cost_flex", cr_rate)
    elif service_tier == "batch":
        input_rate  = prices.get("input_cost_per_token_batches", input_rate)
        output_rate = prices.get("output_cost_per_token_batches", output_rate)

    regular = max(0, prompt_tokens - cache_write_5m - cache_write_1hr - cache_read - audio_input_tokens)
    regular_completion = max(0, completion_tokens - audio_output_tokens)
    cost = (regular        * input_rate
            + cache_write_5m  * cw5m_rate
            + cache_write_1hr * cw1hr_rate
            + cache_read      * cr_rate
            + regular_completion * output_rate)

    # Audio tokens are billed at their own per-token rates (not affected by service tier).
    audio_input_rate  = prices.get("input_cost_per_audio_token", 0)
    audio_output_rate = prices.get("output_cost_per_audio_token", 0)
    cost += audio_input_tokens * audio_input_rate + audio_output_tokens * audio_output_rate

    # Web search tool cost — mirrors Go's genericCalculateCost web search section.
    # Variable pricing (search-preview models): keyed by search_context_size.
    # Flat pricing (standard models): web_search_cost_per_request per call.
    if web_search_requests > 0:
        scq = prices.get("search_context_cost_per_query", {})
        if scq:
            size = search_context_size or "medium"
            rate = scq.get(f"search_context_size_{size}", 0)
            cost += web_search_requests * rate
        elif prices.get("web_search_cost_per_request", 0) > 0:
            cost += web_search_requests * prices["web_search_cost_per_request"]

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
        #
        # service_tier: "" (standard) | "flex" | "priority"
        #   "batch" is intentionally excluded — LiteLLM's generic_cost_per_token
        #   does not wire service_tier="batch" to the _batches rate fields, so
        #   comparing our (correct) batch pricing against LiteLLM would always
        #   produce false failures.
        def tc(name, model="gpt-4o-2024-11-20", **kw):
            defaults = dict(prompt_tokens=0, completion_tokens=0, cached_tokens=0,
                            service_tier="", web_search_requests=0, search_context_size="",
                            audio_input_tokens=0, audio_output_tokens=0)
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

            # gpt-5.4 — 1.05M context window; above 272k tokens triggers 2x/1.5x pricing
            tc("gpt5_4_standard",  model="gpt-5.4",
               prompt_tokens=1_000,   completion_tokens=500),
            tc("gpt5_4_above_272k", model="gpt-5.4",
               prompt_tokens=300_000, completion_tokens=50_000),
            tc("gpt5_4_flex",      model="gpt-5.4",
               prompt_tokens=1_000,   completion_tokens=500,  service_tier="flex"),
            tc("gpt5_4_priority",  model="gpt-5.4",
               prompt_tokens=1_000,   completion_tokens=500,  service_tier="priority"),
            tc("gpt5_4_cached",    model="gpt-5.4",
               prompt_tokens=1_000,   completion_tokens=500,  cached_tokens=800),
            tc("gpt5_4_flex_cached", model="gpt-5.4",
               prompt_tokens=1_000,   completion_tokens=500,  cached_tokens=800,
               service_tier="flex"),
            tc("gpt5_4_priority_cached", model="gpt-5.4",
               prompt_tokens=1_000,   completion_tokens=500,  cached_tokens=800,
               service_tier="priority"),

            # gpt-5.4-pro
            tc("gpt5_4_pro_standard", model="gpt-5.4-pro",
               prompt_tokens=1_000,   completion_tokens=500),
            tc("gpt5_4_pro_above_272k", model="gpt-5.4-pro",
               prompt_tokens=300_000, completion_tokens=50_000),
            tc("gpt5_4_pro_flex",  model="gpt-5.4-pro",
               prompt_tokens=1_000,   completion_tokens=500,  service_tier="flex"),

            # gpt-4.1
            tc("gpt4_1_standard",  model="gpt-4.1",
               prompt_tokens=1_000,   completion_tokens=500),
            tc("gpt4_1_priority",  model="gpt-4.1",
               prompt_tokens=1_000,   completion_tokens=500,  service_tier="priority"),
            tc("gpt4_1_priority_cached", model="gpt-4.1",
               prompt_tokens=1_000,   completion_tokens=500,  cached_tokens=800,
               service_tier="priority"),

            # Web search tool costs
            # Flat rate ($10/1k = $0.01/call) for standard models
            tc("web_search_flat_rate",  model="gpt-4o-2024-11-20",
               prompt_tokens=500,     completion_tokens=200,  web_search_requests=1),
            tc("web_search_flat_rate_gpt54", model="gpt-5.4",
               prompt_tokens=1_000,   completion_tokens=500,  web_search_requests=1),
            # Variable rate for search-preview models (search_context_cost_per_query)
            tc("web_search_preview_medium", model="gpt-4o-search-preview",
               prompt_tokens=1_000,   completion_tokens=500,
               web_search_requests=1, search_context_size="medium"),
            tc("web_search_preview_high",   model="gpt-4o-search-preview",
               prompt_tokens=1_000,   completion_tokens=500,
               web_search_requests=1, search_context_size="high"),

            # Audio token costs
            # gpt-4o-audio-preview: 120 prompt (80 text + 40 audio), 80 completion (50 text + 30 audio)
            #   text input:   80 × $2.50/M  = $0.000200
            #   audio input:  40 × $40.00/M = $0.001600
            #   text output:  50 × $10.00/M = $0.000500
            #   audio output: 30 × $80.00/M = $0.002400
            #   total:                         $0.004700
            tc("audio_preview_mixed", model="gpt-4o-audio-preview",
               prompt_tokens=120,    completion_tokens=80,
               audio_input_tokens=40, audio_output_tokens=30),
            # Pure text call on gpt-4o-audio-preview — audio token fields should have no effect
            tc("audio_preview_text_only", model="gpt-4o-audio-preview",
               prompt_tokens=100,    completion_tokens=50),
        ]

    def our_cost(self, tc: dict, prices: dict) -> Tuple[Optional[float], Optional[str]]:
        p = prices.get(tc["model"])
        if p is None:
            return None, f"model '{tc['model']}' not in model_prices.json"

        prompt_tokens      = tc["prompt_tokens"]
        completion_tokens  = tc["completion_tokens"]
        cached_tokens      = tc["cached_tokens"]
        service_tier       = tc.get("service_tier", "")

        # OpenAI: PromptTokens includes cached; genericCalculateCost subtracts.
        # tiering_threshold_272k=272_000 enables the above-272k rate tier for
        # gpt-5.4 and gpt-5.4-pro (1.05M context window); harmless for other models
        # since they don't define the _above_272k_tokens fields.
        cost, err = generic_calculate_cost(
            prompt_tokens=prompt_tokens,
            completion_tokens=completion_tokens,
            cache_write_5m=0,
            cache_write_1hr=0,
            cache_read=cached_tokens,
            input_tokens_for_tiering=prompt_tokens,
            tiering_threshold=200_000,
            prices=p,
            service_tier=service_tier,
            tiering_threshold_272k=272_000,
            web_search_requests=tc.get("web_search_requests", 0),
            search_context_size=tc.get("search_context_size", ""),
            audio_input_tokens=tc.get("audio_input_tokens", 0),
            audio_output_tokens=tc.get("audio_output_tokens", 0),
        )
        return cost, err

    def litellm_cost(self, tc: dict) -> Tuple[Optional[float], Optional[str]]:
        try:
            from litellm.litellm_core_utils.llm_cost_calc.utils import generic_cost_per_token
            from litellm.litellm_core_utils.llm_cost_calc.tool_call_cost_tracking import (
                StandardBuiltInToolCostTracking,
            )
            from litellm.types.utils import (
                PromptTokensDetailsWrapper, CompletionTokensDetailsWrapper, Usage,
            )
            import litellm as _litellm
        except ImportError as e:
            return None, f"litellm import error: {e}"

        service_tier = tc.get("service_tier", "") or None
        ptd = PromptTokensDetailsWrapper(
            cached_tokens=tc["cached_tokens"],
            audio_tokens=tc.get("audio_input_tokens", 0),
        )
        ctd = CompletionTokensDetailsWrapper(
            audio_tokens=tc.get("audio_output_tokens", 0),
        )
        usage = Usage(
            prompt_tokens=tc["prompt_tokens"],
            completion_tokens=tc["completion_tokens"],
            total_tokens=tc["prompt_tokens"] + tc["completion_tokens"],
            prompt_tokens_details=ptd,
            completion_tokens_details=ctd,
        )
        try:
            prompt_cost, completion_cost = generic_cost_per_token(
                model=tc["model"], usage=usage, custom_llm_provider="openai",
                service_tier=service_tier,
            )
            token_cost = prompt_cost + completion_cost
        except Exception as e:
            return None, str(e)

        # Add web search tool cost via LiteLLM's StandardBuiltInToolCostTracking.
        # LiteLLM only supports search_context_cost_per_query (search-preview models).
        # Standard models have no web_search_cost_per_request in LiteLLM's JSON,
        # so we skip those cases rather than producing a false 0.
        web_search_requests = tc.get("web_search_requests", 0)
        if web_search_requests > 0:
            try:
                model_info = _litellm.get_model_info(
                    model=tc["model"], custom_llm_provider="openai"
                )
            except Exception:
                model_info = None
            has_query_pricing = bool(
                model_info and model_info.get("search_context_cost_per_query")
            )
            if not has_query_pricing:
                return None, (
                    "LiteLLM has no web_search_cost_per_request for this model; "
                    "flat-rate web search cost not comparable via LiteLLM"
                )
            web_search_options = {"search_context_size": tc.get("search_context_size") or None}
            web_cost = StandardBuiltInToolCostTracking.get_cost_for_web_search(
                web_search_options=web_search_options,
                model_info=model_info,
            )
            token_cost += web_cost * web_search_requests

        return token_cost, None


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
                           audio_input_tokens=0, audio_output_tokens=0, image_output_tokens=0,
                           cached_audio_tokens=0,
                           tool_use_prompt_tokens=0, web_search_cost_per_request=0.0,
                           service_tier="", web_search_requests=0,
                           # thinking_mode: "inclusive" (thoughtsTokenCount already inside
                           # candidatesTokenCount) or "exclusive" (thoughts are separate).
                           thoughts_tokens=0, thinking_mode="")
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
            # Tiering is on PROMPT tokens only (not total). Prompt=80k < 128k so BASE
            # rates apply even though total=140k > 128k.
            tc("total_above_128k_prompt_below", prompt_tokens=80_000, completion_tokens=60_000),
            # gemini-2.5-pro: >200k threshold on prompt tokens
            tc("above_200k", model="gemini/gemini-2.5-pro",
               prompt_tokens=210_000, completion_tokens=50_000),
            # gemini-2.5-pro: above 200k with cache → tiered cache read rate
            tc("above_200k_with_cache", model="gemini/gemini-2.5-pro",
               prompt_tokens=210_000, completion_tokens=1_000,
               cached_tokens=50_000),
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
            # NOTE: cache_read_input_token_cost_per_audio_token is NOT implemented by LiteLLM.
            # LiteLLM bills all cached tokens at the text rate. Our Go code correctly uses
            # the audio cache rate when defined. This case is tested in unit tests only.
            # Gemini Live: grounding/web search tool tokens (billed at input rate as fallback)
            # tool_use_prompt_tokens are SEPARATE from prompt_tokens (not a subset).
            tc("tool_use_token_fallback", model="gemini/gemini-2.0-flash",
               prompt_tokens=100, completion_tokens=50,
               tool_use_prompt_tokens=25),
            # Gemini Live: grounding with fixed per-request fee (when web_search_cost_per_request set)
            tc("tool_use_fixed_fee", model="gemini/gemini-2.0-flash",
               prompt_tokens=100, completion_tokens=50,
               tool_use_prompt_tokens=25, web_search_cost_per_request=0.01),
            # ON_DEMAND_PRIORITY service tier: billed at _priority rate variants.
            # vertex_ai/gemini-3-flash-preview has input_cost_per_token_priority defined.
            tc("priority_standard", model="vertex_ai/gemini-3-flash-preview",
               prompt_tokens=1_000, completion_tokens=500,
               service_tier="priority"),
            # ON_DEMAND_PRIORITY + audio input: audio billed at standard audio rate (not priority)
            tc("priority_audio", model="vertex_ai/gemini-3-flash-preview",
               prompt_tokens=500, completion_tokens=200,
               audio_input_tokens=300, service_tier="priority"),
            # ON_DEMAND_PRIORITY + cache read: cached tokens billed at priority cache rate
            tc("priority_cache", model="vertex_ai/gemini-3-flash-preview",
               prompt_tokens=1_000, completion_tokens=200,
               cached_tokens=500, service_tier="priority"),
            # Grounding: Google AI Studio → per-query fee ($0.035 × N)
            tc("grounding_google_ai", model="gemini/gemini-2.0-flash",
               prompt_tokens=100, completion_tokens=50,
               web_search_requests=2),
            # Grounding: Vertex AI → flat fee ($0.035) regardless of query count
            tc("grounding_vertex_ai", model="vertex_ai/gemini-3-flash-preview",
               prompt_tokens=100, completion_tokens=50,
               web_search_requests=3),
            # No grounding queries → no extra cost
            tc("grounding_none", model="gemini/gemini-2.0-flash",
               prompt_tokens=100, completion_tokens=50,
               web_search_requests=0),
            # Thinking INCLUSIVE: thoughtsTokenCount is already included in candidatesTokenCount.
            # completion_tokens=80 (candidatesTokenCount, includes 30 thoughts), thoughts_tokens=30.
            # Go normalize: CompletionTokens=80, ReasoningTokens=30.
            # LiteLLM: completion=80, reasoning_tokens=30 → text=50, thoughts billed at reasoning rate.
            # gemini-2.5-flash-preview-04-17: input=1.5e-7, output=6e-7, reasoning=3.5e-6
            tc("thinking_inclusive", model="gemini/gemini-2.5-flash-preview-04-17",
               prompt_tokens=100, completion_tokens=80,
               thoughts_tokens=30, thinking_mode="inclusive"),
            # Thinking EXCLUSIVE: thoughtsTokenCount is NOT in candidatesTokenCount.
            # completion_tokens=50 (candidatesTokenCount, text only), thoughts_tokens=40.
            # Go normalize: CompletionTokens=50+40=90, ReasoningTokens=40.
            # LiteLLM: completion=90 (adjusted), reasoning_tokens=40 → text=50.
            tc("thinking_exclusive", model="gemini/gemini-2.5-flash-preview-04-17",
               prompt_tokens=100, completion_tokens=50,
               thoughts_tokens=40, thinking_mode="exclusive"),
        ]

    def our_cost(self, tc: dict, prices: dict) -> Tuple[Optional[float], Optional[str]]:
        # Gemini model names in our JSON use the "gemini/" prefix; Vertex AI uses "vertex_ai/"
        model_key = tc["model"]
        # Try exact key, then gemini/ prefix, then bare name
        p = prices.get(model_key) or prices.get("gemini/" + model_key) or prices.get(
            model_key.removeprefix("gemini/").removeprefix("vertex_ai/"))
        if p is None:
            return None, f"model '{model_key}' not in model_prices.json"

        prompt_tokens           = tc["prompt_tokens"]
        completion_tokens       = tc["completion_tokens"]
        cached_tokens           = tc["cached_tokens"]
        audio_input_tokens      = tc.get("audio_input_tokens", 0)
        audio_output_tokens     = tc.get("audio_output_tokens", 0)
        image_output_tokens     = tc.get("image_output_tokens", 0)
        tool_use_prompt_tokens  = tc.get("tool_use_prompt_tokens", 0)
        web_search_fixed_fee    = tc.get("web_search_cost_per_request", 0.0)
        service_tier            = tc.get("service_tier", "")
        web_search_requests     = tc.get("web_search_requests", 0)

        # Gemini tiering: based on PROMPT tokens only (not total), matching LiteLLM.
        # Check highest threshold first (200k > 128k).
        if prompt_tokens > 200_000 and p.get("input_cost_per_token_above_200k_tokens"):
            input_rate  = p.get("input_cost_per_token_above_200k_tokens",
                                 p.get("input_cost_per_token", 0))
            output_rate = p.get("output_cost_per_token_above_200k_tokens",
                                 p.get("output_cost_per_token", 0))
            cr_rate     = p.get("cache_read_input_token_cost_above_200k_tokens",
                                 p.get("cache_read_input_token_cost", 0))
            # Priority rates for >200k tier
            if service_tier == "priority":
                input_rate  = p.get("input_cost_per_token_above_200k_tokens_priority", input_rate)
                output_rate = p.get("output_cost_per_token_above_200k_tokens_priority", output_rate)
                cr_rate     = p.get("cache_read_input_token_cost_above_200k_tokens_priority", cr_rate)
        elif prompt_tokens > 128_000 and p.get("input_cost_per_token_above_128k_tokens"):
            input_rate  = p.get("input_cost_per_token_above_128k_tokens",
                                 p.get("input_cost_per_token", 0))
            output_rate = p.get("output_cost_per_token_above_128k_tokens",
                                 p.get("output_cost_per_token", 0))
            cr_rate     = p.get("cache_read_input_token_cost_above_128k_tokens",
                                 p.get("cache_read_input_token_cost", 0))
            # No priority variants exist for 128k tier — no override needed
        else:
            input_rate  = p.get("input_cost_per_token", 0)
            output_rate = p.get("output_cost_per_token", 0)
            cr_rate     = p.get("cache_read_input_token_cost", 0)
            # Priority rates for standard tier
            if service_tier == "priority":
                input_rate  = p.get("input_cost_per_token_priority", input_rate)
                output_rate = p.get("output_cost_per_token_priority", output_rate)
                cr_rate     = p.get("cache_read_input_token_cost_priority", cr_rate)

        # Audio input rate (falls back to standard input rate when absent)
        # Note: LiteLLM does NOT apply _priority suffix to audio token rates.
        audio_in_rate  = p.get("input_cost_per_audio_token", input_rate)
        # Audio output rate (falls back to standard output rate)
        audio_out_rate = p.get("output_cost_per_audio_token", output_rate)
        # Image output rate (falls back to standard output rate)
        image_out_rate = p.get("output_cost_per_image_token", output_rate)
        # Reasoning token rate (falls back to output rate when absent)
        reasoning_rate = p.get("output_cost_per_reasoning_token", output_rate)
        # Cached audio token rate: separate rate when model has cache_read_input_token_cost_per_audio_token
        cached_audio_rate = p.get("cache_read_input_token_cost_per_audio_token", 0)

        cached_audio_tokens = tc.get("cached_audio_tokens", 0)
        thoughts_tokens     = tc.get("thoughts_tokens", 0)
        thinking_mode       = tc.get("thinking_mode", "")

        # Thinking token normalization (mirrors Go's GeminiCalculator.Normalize):
        #   INCLUSIVE: thoughtsTokenCount already inside candidatesTokenCount.
        #              completion_tokens from tc is the full candidatesTokenCount.
        #   EXCLUSIVE: thoughtsTokenCount is separate; Go adds it to CompletionTokens.
        #              adjusted_completion = completion_tokens + thoughts_tokens.
        # In both cases, ReasoningTokens = thoughts_tokens.
        if thinking_mode == "exclusive":
            adjusted_completion = completion_tokens + thoughts_tokens
        else:
            adjusted_completion = completion_tokens

        # Regular text tokens (exclude audio input so it's billed separately)
        regular_prompt = max(0, prompt_tokens - cached_tokens - audio_input_tokens)
        # Regular text output (exclude audio, image, and reasoning tokens)
        regular_output = max(0, adjusted_completion - audio_output_tokens - image_output_tokens - thoughts_tokens)

        # Cache read cost: split text vs audio if model defines a separate audio cache rate
        if cached_audio_rate > 0:
            text_cached = max(0, cached_tokens - cached_audio_tokens)
            cache_read_cost = text_cached * cr_rate + cached_audio_tokens * cached_audio_rate
        else:
            cache_read_cost = cached_tokens * cr_rate

        cost = (regular_prompt * input_rate
                + cache_read_cost
                + audio_input_tokens * audio_in_rate
                + regular_output * output_rate
                + audio_output_tokens * audio_out_rate
                + image_output_tokens * image_out_rate
                + thoughts_tokens * reasoning_rate)

        # Gemini Live tool use: separate tokens from grounding/web search tools.
        # Prefer flat fee; fall back to per-token billing at standard input rate.
        if tool_use_prompt_tokens > 0:
            if web_search_fixed_fee > 0:
                cost += web_search_fixed_fee
            else:
                cost += tool_use_prompt_tokens * input_rate

        # Grounding / web search cost (from candidates[].groundingMetadata.webSearchQueries).
        # Google AI Studio (provider=gemini*): $0.035 per query.
        # Vertex AI (provider=vertex_ai*): $0.035 flat per call.
        GROUNDING_COST = 0.035
        if web_search_requests > 0:
            provider = p.get("provider", p.get("litellm_provider", ""))
            if provider.startswith("vertex_ai"):
                cost += GROUNDING_COST
            else:
                cost += web_search_requests * GROUNDING_COST

        return cost, None

    def litellm_cost(self, tc: dict) -> Tuple[Optional[float], Optional[str]]:
        try:
            from litellm.litellm_core_utils.llm_cost_calc.utils import generic_cost_per_token
            from litellm.types.utils import (PromptTokensDetailsWrapper,
                                             CompletionTokensDetailsWrapper, Usage)
        except ImportError as e:
            return None, f"litellm import error: {e}"

        model_key       = tc["model"]
        service_tier    = tc.get("service_tier", "")
        web_search_requests = tc.get("web_search_requests", 0)

        # Determine provider and bare model name for LiteLLM calls
        if model_key.startswith("vertex_ai/"):
            llm_provider = "vertex_ai"
            model = model_key.removeprefix("vertex_ai/")
        else:
            llm_provider = "gemini"
            model = model_key.removeprefix("gemini/")

        audio_input_tokens      = tc.get("audio_input_tokens", 0)
        audio_output_tokens     = tc.get("audio_output_tokens", 0)
        image_output_tokens     = tc.get("image_output_tokens", 0)
        tool_use_prompt_tokens  = tc.get("tool_use_prompt_tokens", 0)
        web_search_fixed_fee    = tc.get("web_search_cost_per_request", 0.0)
        thoughts_tokens         = tc.get("thoughts_tokens", 0)
        thinking_mode           = tc.get("thinking_mode", "")

        # Thinking mode adjustment (mirrors our Go normalization):
        #   INCLUSIVE: candidatesTokenCount already includes thoughts → pass as-is.
        #   EXCLUSIVE: Go adds thoughts to CompletionTokens → adjust before passing to LiteLLM.
        # LiteLLM treats completion_tokens as the total (text + reasoning) and derives
        # text = completion_tokens - reasoning_tokens internally.
        if thinking_mode == "exclusive":
            litellm_completion = tc["completion_tokens"] + thoughts_tokens
        else:
            litellm_completion = tc["completion_tokens"]

        ptd = PromptTokensDetailsWrapper(
            cached_tokens=tc["cached_tokens"],
            audio_tokens=audio_input_tokens if audio_input_tokens else None,
        )
        ctd = CompletionTokensDetailsWrapper(
            audio_tokens=audio_output_tokens if audio_output_tokens else None,
            image_tokens=image_output_tokens if image_output_tokens else None,
            reasoning_tokens=thoughts_tokens if thoughts_tokens else None,
        )
        usage = Usage(
            prompt_tokens=tc["prompt_tokens"],
            completion_tokens=litellm_completion,
            total_tokens=tc["prompt_tokens"] + litellm_completion,
            prompt_tokens_details=ptd,
            completion_tokens_details=ctd if (audio_output_tokens or image_output_tokens or thoughts_tokens) else None,
        )
        try:
            prompt_cost, completion_cost = generic_cost_per_token(
                model=model, usage=usage, custom_llm_provider=llm_provider,
                **({"service_tier": service_tier} if service_tier else {}),
            )
            base_cost = prompt_cost + completion_cost
        except Exception as e:
            return None, str(e)

        # LiteLLM's Gemini Live handler adds tool use cost on top of token costs.
        # Since generic_cost_per_token doesn't cover toolUsePromptTokenCount, we
        # replicate the Live handler's logic here:
        #   - flat fee when web_search_cost_per_request is set
        #   - per-token billing at input rate as fallback
        if tool_use_prompt_tokens > 0:
            if web_search_fixed_fee > 0:
                base_cost += web_search_fixed_fee
            else:
                try:
                    from litellm import get_model_info
                    info = get_model_info(model=model, custom_llm_provider=llm_provider)
                    input_rate = info.get("input_cost_per_token", 0.0) if info else 0.0
                except Exception:
                    input_rate = 0.0
                base_cost += tool_use_prompt_tokens * input_rate

        # Grounding / web search cost: LiteLLM's StandardBuiltInToolCostTracking
        # charges $0.035 per query (Google AI Studio) or $0.035 flat (Vertex AI).
        GROUNDING_COST = 0.035
        if web_search_requests > 0:
            if llm_provider.startswith("vertex_ai"):
                base_cost += GROUNDING_COST
            else:
                base_cost += web_search_requests * GROUNDING_COST

        return base_cost, None


# ═══════════════════════════════════════════════════════════════════════════════
# Provider: Mistral
# ═══════════════════════════════════════════════════════════════════════════════

class MistralProvider(Provider):

    @property
    def name(self) -> str:
        return "Mistral"

    def test_cases(self) -> List[dict]:
        def tc(name, model="mistral/mistral-large-latest", **kw):
            defaults = dict(prompt_tokens=0, completion_tokens=0, audio_seconds=0)
            return {"name": name, "model": model, **defaults, **kw}

        return [
            # ── Core chat models ─────────────────────────────────────────────
            tc("large_basic",                 prompt_tokens=100,    completion_tokens=50),
            tc("large_large_request",         prompt_tokens=10_000, completion_tokens=2_000),
            tc("medium",  model="mistral/mistral-medium-latest",
                          prompt_tokens=500,  completion_tokens=200),
            tc("small",   model="mistral/mistral-small-latest",
                          prompt_tokens=500,  completion_tokens=200),
            tc("small_creative", model="mistral/labs-mistral-small-creative",
                          prompt_tokens=300,  completion_tokens=150),

            # ── Reasoning models (Magistral) ────────────────────────────────
            tc("magistral_medium", model="mistral/magistral-medium-latest",
                          prompt_tokens=1_000, completion_tokens=500),
            tc("magistral_small",  model="mistral/magistral-small-latest",
                          prompt_tokens=800,   completion_tokens=400),

            # ── Coding models ───────────────────────────────────────────────
            tc("codestral",       model="mistral/codestral-latest",
                          prompt_tokens=600,  completion_tokens=300),
            tc("devstral_medium", model="mistral/devstral-medium-latest",
                          prompt_tokens=400,  completion_tokens=200),
            tc("devstral_small",  model="mistral/devstral-small-latest",
                          prompt_tokens=400,  completion_tokens=200),

            # ── Edge models (Ministral) ─────────────────────────────────────
            tc("ministral_3b",  model="mistral/ministral-3b-latest",
                          prompt_tokens=200,  completion_tokens=100),
            tc("ministral_8b",  model="mistral/ministral-8b-latest",
                          prompt_tokens=200,  completion_tokens=100),
            tc("ministral_14b", model="mistral/ministral-14b-latest",
                          prompt_tokens=200,  completion_tokens=100),

            # ── Vision models (Pixtral) — image tokens in prompt_tokens ─────
            tc("pixtral_large", model="mistral/pixtral-large-latest",
                          prompt_tokens=5_000, completion_tokens=300),
            tc("pixtral_12b",   model="mistral/pixtral-12b-2409",
                          prompt_tokens=3_000, completion_tokens=200),

            # ── Legacy open models ───────────────────────────────────────────
            tc("nemo",         model="mistral/open-mistral-nemo",
                          prompt_tokens=400,  completion_tokens=200),
            tc("mistral_7b",   model="mistral/open-mistral-7b",
                          prompt_tokens=300,  completion_tokens=150),
            tc("mixtral_8x7b", model="mistral/open-mixtral-8x7b",
                          prompt_tokens=500,  completion_tokens=250),
            tc("mixtral_8x22b",model="mistral/open-mixtral-8x22b",
                          prompt_tokens=800,  completion_tokens=400),

            # ── Embedding models (output tokens = 0) ────────────────────────
            tc("embed",         model="mistral/mistral-embed",
                          prompt_tokens=500,  completion_tokens=0),
            tc("codestral_embed", model="mistral/codestral-embed-2505",
                          prompt_tokens=500,  completion_tokens=0),

            # ── Voxtral audio models (chat completions with audio seconds) ──
            # audio_seconds billed at input_cost_per_audio_per_second;
            # prompt_tokens are text-only tokens (no double-counting).
            tc("voxtral_small_text_only", model="mistral/voxtral-small-latest",
                          prompt_tokens=200,  completion_tokens=100, audio_seconds=0),
            tc("voxtral_small_with_audio", model="mistral/voxtral-small-latest",
                          prompt_tokens=50,   completion_tokens=100, audio_seconds=30),
            tc("voxtral_mini_text_only",  model="mistral/voxtral-mini-latest",
                          prompt_tokens=100,  completion_tokens=50,  audio_seconds=0),
            tc("voxtral_mini_with_audio", model="mistral/voxtral-mini-latest",
                          prompt_tokens=20,   completion_tokens=30,  audio_seconds=60),

            # ── Edge cases ───────────────────────────────────────────────────
            tc("zero_tokens"),
            tc("only_output",  completion_tokens=100),
        ]

    def our_cost(self, tc: dict, prices: dict) -> Tuple[Optional[float], Optional[str]]:
        p = prices.get(tc["model"])
        if p is None:
            return None, f"model '{tc['model']}' not in model_prices.json"

        token_cost = (tc["prompt_tokens"]     * p.get("input_cost_per_token", 0)
                    + tc["completion_tokens"] * p.get("output_cost_per_token", 0))
        audio_cost = tc.get("audio_seconds", 0) * p.get("input_cost_per_audio_per_second", 0)
        return token_cost + audio_cost, None

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
            token_cost = prompt_cost + completion_cost
        except Exception as e:
            return None, str(e)

        # LiteLLM doesn't model per-second audio billing for Voxtral;
        # we replicate our own formula for audio cases so we can cross-check.
        audio_seconds = tc.get("audio_seconds", 0)
        if audio_seconds:
            import json, pathlib
            prices_path = pathlib.Path(__file__).parent.parent / "gateway/system-policies/llm-cost/pricing/model_prices.json"
            with open(prices_path) as f:
                prices = json.load(f)
            p = prices.get(tc["model"], {})
            audio_cost = audio_seconds * p.get("input_cost_per_audio_per_second", 0)
            return token_cost + audio_cost, None

        return token_cost, None


# ═══════════════════════════════════════════════════════════════════════════════
# Provider: Azure OpenAI
# ═══════════════════════════════════════════════════════════════════════════════

class AzureOpenAIProvider(Provider):
    """Azure OpenAI Chat Completions — standard deployment pricing (Tuesday release scope).

    Test case model keys use the full Azure pricing key ("azure/<model>") since
    that is what the llm-cost policy resolves to after applying the model_prefix
    fix. This validates that our per-token rates match LiteLLM for Azure.

    Limitations (known, deferred to GA):
    - Only standard pricing; Global/EU deployment types are separate entries.
    - Responses API (deployment-name in response.model) not yet handled.
    """

    @property
    def name(self) -> str:
        return "Azure OpenAI"

    def test_cases(self) -> List[dict]:
        def tc(name, model="azure/gpt-4o-mini-2024-07-18", **kw):
            defaults = dict(prompt_tokens=0, completion_tokens=0, cached_tokens=0)
            return {"name": name, "model": model, **defaults, **kw}

        return [
            # gpt-4o-mini (standard)
            tc("mini_basic",     prompt_tokens=100,   completion_tokens=50),
            tc("mini_large",     prompt_tokens=5_000, completion_tokens=1_000),
            tc("mini_cached",    prompt_tokens=1_000, completion_tokens=100, cached_tokens=800),
            tc("zero_tokens"),

            # gpt-4o (standard)
            tc("gpt4o_basic", model="azure/gpt-4o-2024-11-20",
               prompt_tokens=100,   completion_tokens=50),
            tc("gpt4o_large", model="azure/gpt-4o-2024-11-20",
               prompt_tokens=5_000, completion_tokens=1_000),
            tc("gpt4o_cached", model="azure/gpt-4o-2024-11-20",
               prompt_tokens=1_000, completion_tokens=100, cached_tokens=800),

            # gpt-4o 2024-08-06 (standard)
            tc("gpt4o_aug_basic", model="azure/gpt-4o-2024-08-06",
               prompt_tokens=100,   completion_tokens=50),

            # gpt-4-turbo (standard, no cache rate)
            tc("gpt4_turbo_basic", model="azure/gpt-4-turbo-2024-04-09",
               prompt_tokens=100,   completion_tokens=50),
            tc("gpt4_turbo_cached", model="azure/gpt-4-turbo-2024-04-09",
               prompt_tokens=1_000, completion_tokens=100, cached_tokens=800),
        ]

    def our_cost(self, tc: dict, prices: dict) -> Tuple[Optional[float], Optional[str]]:
        p = prices.get(tc["model"])
        if p is None:
            return None, f"model '{tc['model']}' not in model_prices.json"

        cost, err = generic_calculate_cost(
            prompt_tokens=tc["prompt_tokens"],
            completion_tokens=tc["completion_tokens"],
            cache_write_5m=0,
            cache_write_1hr=0,
            cache_read=tc["cached_tokens"],
            input_tokens_for_tiering=tc["prompt_tokens"],
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

        # Strip the "azure/" prefix — LiteLLM resolves it via custom_llm_provider="azure".
        bare_model = tc["model"].removeprefix("azure/")

        ptd = PromptTokensDetailsWrapper(cached_tokens=tc["cached_tokens"])
        usage = Usage(
            prompt_tokens=tc["prompt_tokens"],
            completion_tokens=tc["completion_tokens"],
            total_tokens=tc["prompt_tokens"] + tc["completion_tokens"],
            prompt_tokens_details=ptd,
        )
        try:
            prompt_cost, completion_cost = generic_cost_per_token(
                model=bare_model, usage=usage, custom_llm_provider="azure",
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
    AzureOpenAIProvider(),
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
