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

"""Python policy interfaces for `wso2_gateway_policy_sdk.policy.v1alpha2`."""

from __future__ import annotations

from abc import ABC, abstractmethod
from typing import Any

from .actions import (
    RequestAction,
    RequestHeaderAction,
    ResponseAction,
    ResponseHeaderAction,
    StreamingRequestAction,
    StreamingResponseAction,
)
from .types import (
    ExecutionContext,
    ProcessingMode,
    RequestContext,
    RequestHeaderContext,
    RequestStreamContext,
    ResponseContext,
    ResponseHeaderContext,
    ResponseStreamContext,
    StreamBody,
)


class Policy(ABC):
    """Base interface for all Python policies."""

    @abstractmethod
    def mode(self) -> ProcessingMode:
        """Return the processing requirements for each phase."""

    def close(self) -> None:
        """Release resources held by this policy instance."""


class RequestHeaderPolicy(Policy, ABC):
    """Processes request headers before the request body is read."""

    @abstractmethod
    def on_request_headers(
        self,
        execution_ctx: ExecutionContext,
        ctx: RequestHeaderContext,
        params: dict[str, Any],
    ) -> RequestHeaderAction:
        """Handle the request-header phase."""


class RequestPolicy(Policy, ABC):
    """Processes the complete buffered request body."""

    @abstractmethod
    def on_request_body(
        self,
        execution_ctx: ExecutionContext,
        ctx: RequestContext,
        params: dict[str, Any],
    ) -> RequestAction:
        """Handle the buffered request-body phase."""


class ResponseHeaderPolicy(Policy, ABC):
    """Processes response headers before the response body is read."""

    @abstractmethod
    def on_response_headers(
        self,
        execution_ctx: ExecutionContext,
        ctx: ResponseHeaderContext,
        params: dict[str, Any],
    ) -> ResponseHeaderAction:
        """Handle the response-header phase."""


class ResponsePolicy(Policy, ABC):
    """Processes the complete buffered response body."""

    @abstractmethod
    def on_response_body(
        self,
        execution_ctx: ExecutionContext,
        ctx: ResponseContext,
        params: dict[str, Any],
    ) -> ResponseAction:
        """Handle the buffered response-body phase."""


class StreamingRequestPolicy(RequestPolicy, ABC):
    """Processes the request body chunk-by-chunk with buffered fallback."""

    @abstractmethod
    def needs_more_request_data(self, accumulated: bytes) -> bool:
        """Return true when more bytes should be accumulated before dispatch."""

    @abstractmethod
    def on_request_body_chunk(
        self,
        execution_ctx: ExecutionContext,
        ctx: RequestStreamContext,
        chunk: StreamBody,
        params: dict[str, Any],
    ) -> StreamingRequestAction:
        """Handle a streamed request-body chunk."""


class StreamingResponsePolicy(ResponsePolicy, ABC):
    """Processes the response body chunk-by-chunk with buffered fallback."""

    @abstractmethod
    def needs_more_response_data(self, accumulated: bytes) -> bool:
        """Return true when more bytes should be accumulated before dispatch."""

    @abstractmethod
    def on_response_body_chunk(
        self,
        execution_ctx: ExecutionContext,
        ctx: ResponseStreamContext,
        chunk: StreamBody,
        params: dict[str, Any],
    ) -> StreamingResponseAction:
        """Handle a streamed response-body chunk."""
