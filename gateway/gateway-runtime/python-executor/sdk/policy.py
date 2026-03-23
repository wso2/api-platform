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

"""Python Policy SDK — mirrors the Go sdk/gateway/policy/v1alpha interface.

Every Python policy is a class that extends Policy and exports a
get_policy(metadata, params) factory function.  The factory controls
instancing — it may return a fresh instance, a cached singleton, or a
shared instance keyed by config hash, exactly like Go's GetPolicy.
"""

from abc import ABC, abstractmethod
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional, Union
from enum import Enum


# ---------------------- Processing Mode ----------------------

class HeaderProcessingMode(Enum):
    SKIP = "SKIP"
    PROCESS = "PROCESS"


class BodyProcessingMode(Enum):
    SKIP = "SKIP"
    BUFFER = "BUFFER"


@dataclass
class ProcessingMode:
    request_header_mode: HeaderProcessingMode = HeaderProcessingMode.PROCESS
    request_body_mode: BodyProcessingMode = BodyProcessingMode.SKIP
    response_header_mode: HeaderProcessingMode = HeaderProcessingMode.SKIP
    response_body_mode: BodyProcessingMode = BodyProcessingMode.SKIP


# ---------------------- Context Objects ----------------------

@dataclass
class SharedContext:
    project_id: str = ""
    request_id: str = ""
    metadata: Dict[str, Any] = field(default_factory=dict)
    api_id: str = ""
    api_name: str = ""
    api_version: str = ""
    api_kind: str = ""
    api_context: str = ""
    operation_path: str = ""
    auth_context: Dict[str, str] = field(default_factory=dict)


@dataclass
class Body:
    content: Optional[bytes] = None
    end_of_stream: bool = False
    present: bool = False


@dataclass
class RequestContext:
    shared: SharedContext
    headers: Dict[str, str]
    body: Optional[Body] = None
    path: str = ""
    method: str = ""
    authority: str = ""
    scheme: str = ""


@dataclass
class ResponseContext:
    shared: SharedContext
    request_headers: Dict[str, str] = field(default_factory=dict)
    request_body: Optional[Body] = None
    request_path: str = ""
    request_method: str = ""
    response_headers: Dict[str, str] = field(default_factory=dict)
    response_body: Optional[Body] = None
    response_status: int = 200


# ---------------------- Action Types ----------------------

@dataclass
class UpstreamRequestModifications:
    """Continue request to upstream with modifications."""
    set_headers: Dict[str, str] = field(default_factory=dict)
    remove_headers: List[str] = field(default_factory=list)
    append_headers: Dict[str, List[str]] = field(default_factory=dict)
    body: Optional[bytes] = None
    path: Optional[str] = None
    method: Optional[str] = None
    analytics_metadata: Dict[str, Any] = field(default_factory=dict)


@dataclass
class ImmediateResponse:
    """Short-circuit the chain and return response immediately."""
    status_code: int = 500
    headers: Dict[str, str] = field(default_factory=dict)
    body: Optional[bytes] = None
    analytics_metadata: Dict[str, Any] = field(default_factory=dict)


@dataclass
class UpstreamResponseModifications:
    """Modify response from upstream."""
    set_headers: Dict[str, str] = field(default_factory=dict)
    remove_headers: List[str] = field(default_factory=list)
    append_headers: Dict[str, List[str]] = field(default_factory=dict)
    body: Optional[bytes] = None
    status_code: Optional[int] = None
    analytics_metadata: Dict[str, Any] = field(default_factory=dict)


RequestAction = Optional[Union[UpstreamRequestModifications, ImmediateResponse]]
ResponseAction = Optional[UpstreamResponseModifications]


# ---------------------- Policy Metadata ----------------------

@dataclass
class PolicyMetadata:
    route_name: str = ""
    api_id: str = ""
    api_name: str = ""
    api_version: str = ""
    attached_to: str = ""  # "api" or "route"


# ---------------------- Policy ABC ----------------------

class Policy(ABC):
    """Base class for all Python policies.

    Lifecycle:
        1. get_policy(metadata, params) factory creates the instance
        2. on_request / on_response called per HTTP request
        3. close() called when the route is removed or replaced

    The factory controls instancing — return a singleton, a cached instance,
    or a fresh instance per route, exactly like Go's GetPolicy pattern.
    """

    @abstractmethod
    def on_request(self, ctx: RequestContext, params: Dict[str, Any]) -> RequestAction:
        """Execute during request phase."""
        ...

    @abstractmethod
    def on_response(self, ctx: ResponseContext, params: Dict[str, Any]) -> ResponseAction:
        """Execute during response phase."""
        ...

    @abstractmethod
    def mode(self) -> ProcessingMode:
        """Return the policy's processing mode for each phase.

        The Gateway kernel uses this to decide whether Envoy ext_proc should
        buffer request/response bodies before invoking this policy.
        """
        ...

    def close(self) -> None:
        """Release resources held by this policy instance.

        Called when the route is removed or replaced (via DestroyPolicy RPC).
        Override to close connections, stop background threads, flush caches, etc.
        Must be idempotent — may be called multiple times.
        Default implementation does nothing.
        """
        pass
