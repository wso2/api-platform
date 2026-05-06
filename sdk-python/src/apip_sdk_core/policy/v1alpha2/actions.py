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

"""Python action types for `apip_sdk_core.policy.v1alpha2`."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any


@dataclass(slots=True)
class DropHeaderAction:
    action: str = "deny"
    headers: list[str] = field(default_factory=list)


@dataclass(slots=True)
class ImmediateResponse:
    status_code: int = 500
    headers: dict[str, str] = field(default_factory=dict)
    body: bytes | None = None
    analytics_metadata: dict[str, Any] = field(default_factory=dict)
    dynamic_metadata: dict[str, dict[str, Any]] = field(default_factory=dict)
    analytics_header_filter: DropHeaderAction = field(default_factory=DropHeaderAction)


@dataclass(slots=True)
class UpstreamRequestHeaderModifications:
    headers_to_set: dict[str, str] = field(default_factory=dict)
    headers_to_remove: list[str] = field(default_factory=list)
    upstream_name: str | None = None
    path: str | None = None
    host: str | None = None
    method: str | None = None
    query_parameters_to_add: dict[str, list[str]] = field(default_factory=dict)
    query_parameters_to_remove: list[str] = field(default_factory=list)
    analytics_metadata: dict[str, Any] = field(default_factory=dict)
    dynamic_metadata: dict[str, dict[str, Any]] = field(default_factory=dict)
    analytics_header_filter: DropHeaderAction = field(default_factory=DropHeaderAction)


@dataclass(slots=True)
class UpstreamRequestModifications:
    body: bytes | None = None
    headers_to_set: dict[str, str] = field(default_factory=dict)
    headers_to_remove: list[str] = field(default_factory=list)
    upstream_name: str | None = None
    path: str | None = None
    host: str | None = None
    method: str | None = None
    query_parameters_to_add: dict[str, list[str]] = field(default_factory=dict)
    query_parameters_to_remove: list[str] = field(default_factory=list)
    analytics_metadata: dict[str, Any] = field(default_factory=dict)
    dynamic_metadata: dict[str, dict[str, Any]] = field(default_factory=dict)
    analytics_header_filter: DropHeaderAction = field(default_factory=DropHeaderAction)


@dataclass(slots=True)
class DownstreamResponseHeaderModifications:
    headers_to_set: dict[str, str] = field(default_factory=dict)
    headers_to_remove: list[str] = field(default_factory=list)
    analytics_metadata: dict[str, Any] = field(default_factory=dict)
    dynamic_metadata: dict[str, dict[str, Any]] = field(default_factory=dict)
    analytics_header_filter: DropHeaderAction = field(default_factory=DropHeaderAction)


@dataclass(slots=True)
class DownstreamResponseModifications:
    body: bytes | None = None
    status_code: int | None = None
    headers_to_set: dict[str, str] = field(default_factory=dict)
    headers_to_remove: list[str] = field(default_factory=list)
    analytics_metadata: dict[str, Any] = field(default_factory=dict)
    dynamic_metadata: dict[str, dict[str, Any]] = field(default_factory=dict)
    analytics_header_filter: DropHeaderAction = field(default_factory=DropHeaderAction)


@dataclass(slots=True)
class ForwardRequestChunk:
    body: bytes | None = None
    analytics_metadata: dict[str, Any] = field(default_factory=dict)
    dynamic_metadata: dict[str, dict[str, Any]] = field(default_factory=dict)


@dataclass(slots=True)
class ForwardResponseChunk:
    body: bytes | None = None
    analytics_metadata: dict[str, Any] = field(default_factory=dict)
    dynamic_metadata: dict[str, dict[str, Any]] = field(default_factory=dict)


@dataclass(slots=True)
class TerminateResponseChunk:
    body: bytes | None = None
    analytics_metadata: dict[str, Any] = field(default_factory=dict)
    dynamic_metadata: dict[str, dict[str, Any]] = field(default_factory=dict)


RequestHeaderAction = UpstreamRequestHeaderModifications | ImmediateResponse | None
RequestAction = UpstreamRequestModifications | ImmediateResponse | None
ResponseHeaderAction = DownstreamResponseHeaderModifications | ImmediateResponse | None
ResponseAction = DownstreamResponseModifications | ImmediateResponse | None
StreamingRequestAction = ForwardRequestChunk | None
StreamingResponseAction = ForwardResponseChunk | TerminateResponseChunk | None
