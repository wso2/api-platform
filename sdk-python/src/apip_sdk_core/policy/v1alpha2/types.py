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

"""Python SDK types for `apip_sdk_core.policy.v1alpha2`."""

from __future__ import annotations

from collections.abc import Iterable, Iterator, Mapping
from dataclasses import dataclass, field
from datetime import datetime
from enum import Enum
from typing import Any, Callable


class HeaderProcessingMode(Enum):
    SKIP = "SKIP"
    PROCESS = "PROCESS"


class BodyProcessingMode(Enum):
    SKIP = "SKIP"
    BUFFER = "BUFFER"
    STREAM = "STREAM"


class ExecutionPhase(Enum):
    REQUEST_HEADERS = "request_headers"
    REQUEST_BODY = "request_body"
    RESPONSE_HEADERS = "response_headers"
    RESPONSE_BODY = "response_body"
    NEEDS_MORE_REQUEST_DATA = "needs_more_request_data"
    REQUEST_BODY_CHUNK = "request_body_chunk"
    NEEDS_MORE_RESPONSE_DATA = "needs_more_response_data"
    RESPONSE_BODY_CHUNK = "response_body_chunk"
    CANCEL = "cancel"


@dataclass(slots=True)
class ProcessingMode:
    request_header_mode: HeaderProcessingMode = HeaderProcessingMode.SKIP
    request_body_mode: BodyProcessingMode = BodyProcessingMode.SKIP
    response_header_mode: HeaderProcessingMode = HeaderProcessingMode.SKIP
    response_body_mode: BodyProcessingMode = BodyProcessingMode.SKIP


class Headers:
    """Read-only multi-value header wrapper."""

    def __init__(self, values: Mapping[str, Iterable[str]] | None = None):
        normalized: dict[str, tuple[str, ...]] = {}
        for name, header_values in (values or {}).items():
            normalized[name.lower()] = tuple(header_values)
        self._values = normalized

    def get(self, name: str) -> list[str]:
        """Return all values for the given header name."""
        return list(self._values.get(name.lower(), ()))

    def has(self, name: str) -> bool:
        """Return true when the header exists."""
        return name.lower() in self._values

    def get_all(self) -> dict[str, list[str]]:
        """Return a defensive copy of all headers."""
        return {name: list(values) for name, values in self._values.items()}

    def iterate(self) -> Iterator[tuple[str, list[str]]]:
        """Yield `(name, values)` pairs using defensive copies."""
        for name, values in self._values.items():
            yield name, list(values)

    def __iter__(self) -> Iterator[str]:
        return iter(self._values)

    def __len__(self) -> int:
        return len(self._values)


@dataclass(slots=True)
class AuthContext:
    authenticated: bool = False
    authorized: bool = False
    auth_type: str = ""
    subject: str = ""
    issuer: str = ""
    audience: list[str] = field(default_factory=list)
    scopes: dict[str, bool] = field(default_factory=dict)
    credential_id: str = ""
    properties: dict[str, str] = field(default_factory=dict)
    previous: "AuthContext | None" = None


@dataclass(slots=True)
class SharedContext:
    project_id: str = ""
    request_id: str = ""
    metadata: dict[str, Any] = field(default_factory=dict)
    api_id: str = ""
    api_name: str = ""
    api_version: str = ""
    api_kind: str = ""
    api_context: str = ""
    operation_path: str = ""
    auth_context: AuthContext | None = None


@dataclass(slots=True)
class Body:
    content: bytes | None = None
    end_of_stream: bool = False
    present: bool = False


@dataclass(slots=True)
class StreamBody:
    chunk: bytes = b""
    end_of_stream: bool = False
    index: int = 0


@dataclass(slots=True)
class RequestHeaderContext:
    shared: SharedContext
    headers: Headers = field(default_factory=Headers)
    path: str = ""
    method: str = ""
    authority: str = ""
    scheme: str = ""
    vhost: str = ""


@dataclass(slots=True)
class RequestContext:
    shared: SharedContext
    headers: Headers = field(default_factory=Headers)
    body: Body | None = None
    path: str = ""
    method: str = ""
    authority: str = ""
    scheme: str = ""
    vhost: str = ""


@dataclass(slots=True)
class ResponseHeaderContext:
    shared: SharedContext
    request_headers: Headers = field(default_factory=Headers)
    request_body: Body | None = None
    request_path: str = ""
    request_method: str = ""
    response_headers: Headers = field(default_factory=Headers)
    response_status: int = 200


@dataclass(slots=True)
class ResponseContext:
    shared: SharedContext
    request_headers: Headers = field(default_factory=Headers)
    request_body: Body | None = None
    request_path: str = ""
    request_method: str = ""
    response_headers: Headers = field(default_factory=Headers)
    response_body: Body | None = None
    response_status: int = 200


@dataclass(slots=True)
class RequestStreamContext:
    shared: SharedContext
    headers: Headers = field(default_factory=Headers)
    path: str = ""
    method: str = ""
    authority: str = ""
    scheme: str = ""
    vhost: str = ""


@dataclass(slots=True)
class ResponseStreamContext:
    shared: SharedContext
    request_headers: Headers = field(default_factory=Headers)
    request_body: Body | None = None
    request_path: str = ""
    request_method: str = ""
    response_headers: Headers = field(default_factory=Headers)
    response_status: int = 200


@dataclass(slots=True)
class PolicyMetadata:
    route_name: str = ""
    api_id: str = ""
    api_name: str = ""
    api_version: str = ""
    attached_to: str = ""


@dataclass(slots=True)
class ExecutionContext:
    request_id: str
    phase: ExecutionPhase
    deadline: datetime | None = None
    route_name: str = ""
    policy_name: str = ""
    policy_version: str = ""
    trace_id: str | None = None
    span_id: str | None = None
    _is_cancelled: Callable[[], bool] | None = None

    def is_cancelled(self) -> bool:
        """Return true when the current execution has been cancelled."""
        if self._is_cancelled is None:
            return False
        return self._is_cancelled()
