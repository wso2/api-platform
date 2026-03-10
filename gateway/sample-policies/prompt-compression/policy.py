# Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

"""Prompt Compression Policy for AI Gateway.

Compresses LLM prompts using the compression-prompt library to reduce
token usage while preserving semantic meaning. Uses statistical filtering
to identify and remove lower-importance words.

Features:
- Configurable compression ratio
- Automatic protection of code blocks and JSON
- Domain term preservation
- Minimum input threshold to avoid over-compressing short prompts
"""

import json
import re
from typing import Any, Dict, List, Optional, Union

from sdk.policy import (
    Policy,
    PolicyMetadata,
    RequestContext,
    ResponseContext,
    UpstreamRequestModifications,
    ImmediateResponse,
    RequestAction,
    ResponseAction,
)

# Import compression-prompt library
from compression_prompt import Compressor, CompressorConfig, StatisticalFilterConfig
from compression_prompt.compressor import CompressionError, InputTooShortError, NegativeGainError


class PromptCompressionPolicy(Policy):
    """Compresses prompts before sending to LLM providers."""

    def __init__(
        self,
        compression_ratio: float = 0.5,
        json_path: str = "$.messages[-1].content",
        min_input_tokens: int = 100,
        preserve_code_blocks: bool = True,
        preserve_json: bool = True,
        domain_terms: Optional[List[str]] = None,
    ):
        self._compression_ratio = compression_ratio
        self._json_path = json_path
        self._min_input_tokens = min_input_tokens
        self._preserve_code_blocks = preserve_code_blocks
        self._preserve_json = preserve_json
        self._domain_terms = domain_terms or []

        # Initialize the compressor with configuration
        filter_config = StatisticalFilterConfig(
            compression_ratio=compression_ratio,
            enable_protection_masks=(preserve_code_blocks or preserve_json),
            domain_terms=self._domain_terms,
        )
        
        config = CompressorConfig(
            target_ratio=compression_ratio,
            min_input_tokens=min_input_tokens,
        )
        
        self._compressor = Compressor(config=config, filter_config=filter_config)

    def on_request(self, ctx: RequestContext, params: Dict[str, Any]) -> RequestAction:
        """Process and compress the request body."""
        # Get request body
        if ctx.body is None or ctx.body.content is None or len(ctx.body.content) == 0:
            # No body to compress, continue unchanged
            return UpstreamRequestModifications()

        content = ctx.body.content

        # Parse JSON payload
        try:
            payload = json.loads(content)
        except json.JSONDecodeError as e:
            return self._build_error_response(
                "Invalid JSON in request body", 
                f"Failed to parse JSON: {str(e)}"
            )

        # Process based on json_path
        try:
            modified = self._process_payload(payload, self._json_path)
            if modified:
                # Serialize the modified payload
                updated_content = json.dumps(payload).encode('utf-8')
                return UpstreamRequestModifications(body=updated_content)
            else:
                # No modifications made, continue unchanged
                return UpstreamRequestModifications()
        except Exception as e:
            return self._build_error_response(
                "Prompt compression failed",
                str(e)
            )

    def on_response(self, ctx: ResponseContext, params: Dict[str, Any]) -> ResponseAction:
        """No response processing needed."""
        return None

    def _process_payload(self, payload: Dict[str, Any], json_path: str) -> bool:
        """Process the payload and compress text at the specified json_path.
        
        Returns True if modifications were made, False otherwise.
        """
        # Parse the JSONPath expression
        # Supported formats:
        # - $.messages[-1].content (last message content)
        # - $.messages[0].content (first message content)
        # - $.messages[*].content (all messages content)
        # - $.prompt (simple prompt field)
        
        path_parts = self._parse_json_path(json_path)
        if not path_parts:
            return False

        return self._apply_at_path(payload, path_parts)

    def _parse_json_path(self, json_path: str) -> List[str]:
        """Parse a JSONPath expression into path components."""
        if not json_path.startswith("$."):
            return []
        
        path = json_path[2:]  # Remove "$."
        if not path:
            return []
        
        # Split by '.' but preserve array indices like [-1], [0], [*]
        parts = []
        current = ""
        in_brackets = False
        
        for char in path:
            if char == '[':
                if current:
                    parts.append(current)
                    current = ""
                in_brackets = True
                current += char
            elif char == ']':
                current += char
                in_brackets = False
                parts.append(current)
                current = ""
            elif char == '.' and not in_brackets:
                if current:
                    parts.append(current)
                    current = ""
            else:
                current += char
        
        if current:
            parts.append(current)
        
        return parts

    def _apply_at_path(self, obj: Any, path_parts: List[str]) -> bool:
        """Apply compression at the specified path in the object.
        
        Returns True if modifications were made.
        """
        if not path_parts:
            return False

        current = obj
        
        # Navigate to the parent of the target
        for i, part in enumerate(path_parts[:-1]):
            next_obj = self._get_child(current, part)
            if next_obj is None:
                return False
            current = next_obj

        # Get the final path component
        final_part = path_parts[-1]
        
        # Check if we're processing a wildcard
        if final_part == "[*]":
            # Process all elements in the array
            if isinstance(current, list):
                modified = False
                for item in current:
                    # For [*].content pattern, we need to compress 'content' field of each item
                    if isinstance(item, dict) and "content" in item:
                        if self._compress_field(item, "content"):
                            modified = True
                return modified
            return False
        
        # Handle array index notation in the final part
        if final_part.startswith('[') and final_part.endswith(']'):
            # This shouldn't happen for the final part in normal cases,
            # but handle it just in case
            idx_str = final_part[1:-1]
            if not isinstance(current, list):
                return False
            
            if idx_str == "*":
                modified = False
                for item in current:
                    if self._compress_text_in_object(item):
                        modified = True
                return modified
            
            try:
                idx = int(idx_str)
                if idx < 0:
                    idx = len(current) + idx
                if 0 <= idx < len(current):
                    return self._compress_text_in_object(current[idx])
                return False
            except (ValueError, IndexError):
                return False
        
        # Handle direct field access
        if isinstance(current, dict):
            return self._compress_field(current, final_part)
        
        return False

    def _get_child(self, obj: Any, path_part: str) -> Any:
        """Get a child element from an object based on a path part."""
        # Handle array index notation
        if path_part.startswith('[') and path_part.endswith(']'):
            idx_str = path_part[1:-1]
            if not isinstance(obj, list):
                return None
            
            if idx_str == "*":
                return obj  # Return the whole array for wildcards
            
            try:
                idx = int(idx_str)
                if idx < 0:
                    idx = len(obj) + idx
                if 0 <= idx < len(obj):
                    return obj[idx]
                return None
            except (ValueError, IndexError):
                return None
        
        # Handle dictionary key access
        if isinstance(obj, dict):
            return obj.get(path_part)
        
        return None

    def _compress_field(self, obj: Dict[str, Any], field: str) -> bool:
        """Compress a specific field in a dictionary object.
        
        Returns True if the field was modified.
        """
        if field not in obj:
            return False
        
        value = obj[field]
        if not isinstance(value, str):
            return False
        
        compressed = self._compress_text(value)
        if compressed != value:
            obj[field] = compressed
            return True
        return False

    def _compress_text_in_object(self, obj: Any) -> bool:
        """Compress text content within an object (looking for 'content' field).
        
        Returns True if modifications were made.
        """
        if isinstance(obj, dict) and "content" in obj:
            return self._compress_field(obj, "content")
        return False

    def _compress_text(self, text: str) -> str:
        """Compress text using the compression-prompt library.
        
        Returns the compressed text, or the original text if compression
        is not beneficial or not possible.
        """
        if not text or len(text.strip()) == 0:
            return text

        try:
            result = self._compressor.compress(text)
            return result.compressed
        except InputTooShortError:
            # Input too short, return original
            return text
        except NegativeGainError:
            # Compression would increase size, return original
            return text
        except CompressionError as e:
            # Other compression error, return original
            return text
        except Exception:
            # Unexpected error, return original to be safe
            return text

    def _build_error_response(self, reason: str, details: str) -> ImmediateResponse:
        """Build an error response for when compression fails."""
        error_body = {
            "error": {
                "type": "PROMPT_COMPRESSION_ERROR",
                "message": reason,
                "details": details,
            }
        }
        
        return ImmediateResponse(
            status_code=500,
            headers={"Content-Type": "application/json"},
            body=json.dumps(error_body).encode('utf-8'),
        )


def get_policy(metadata: PolicyMetadata, params: Dict[str, Any]) -> Policy:
    """Factory function - creates a PromptCompressionPolicy instance.
    
    The factory controls instancing - a fresh instance is created per route
    with the specified configuration parameters.
    """
    # Extract parameters with defaults
    compression_ratio = params.get("compressionRatio", 0.5)
    json_path = params.get("jsonPath", "$.messages[-1].content")
    min_input_tokens = params.get("minInputTokens", 100)
    preserve_code_blocks = params.get("preserveCodeBlocks", True)
    preserve_json = params.get("preserveJson", True)
    domain_terms = params.get("domainTerms", [])
    
    # Validate compression ratio
    if not isinstance(compression_ratio, (int, float)):
        compression_ratio = 0.5
    compression_ratio = max(0.1, min(0.9, float(compression_ratio)))
    
    # Validate min_input_tokens
    if not isinstance(min_input_tokens, int):
        min_input_tokens = 100
    min_input_tokens = max(50, min_input_tokens)
    
    # Ensure domain_terms is a list
    if not isinstance(domain_terms, list):
        domain_terms = []
    
    return PromptCompressionPolicy(
        compression_ratio=compression_ratio,
        json_path=json_path,
        min_input_tokens=min_input_tokens,
        preserve_code_blocks=preserve_code_blocks,
        preserve_json=preserve_json,
        domain_terms=domain_terms,
    )
