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

"""Python SDK surface for APIP SDK Core gateway policies."""

from .actions import (
    DownstreamResponseHeaderModifications,
    DownstreamResponseModifications,
    DropHeaderAction,
    ForwardRequestChunk,
    ForwardResponseChunk,
    ImmediateResponse,
    RequestAction,
    RequestHeaderAction,
    ResponseAction,
    ResponseHeaderAction,
    StreamingRequestAction,
    StreamingResponseAction,
    TerminateResponseChunk,
    UpstreamRequestHeaderModifications,
    UpstreamRequestModifications,
)
from .policy import (
    Policy,
    RequestHeaderPolicy,
    RequestPolicy,
    ResponseHeaderPolicy,
    ResponsePolicy,
    StreamingRequestPolicy,
    StreamingResponsePolicy,
)
from .types import (
    AuthContext,
    Body,
    BodyProcessingMode,
    ExecutionContext,
    ExecutionPhase,
    HeaderProcessingMode,
    Headers,
    PolicyMetadata,
    ProcessingMode,
    RequestContext,
    RequestHeaderContext,
    RequestStreamContext,
    ResponseContext,
    ResponseHeaderContext,
    ResponseStreamContext,
    SharedContext,
    StreamBody,
)

__all__ = [
    "AuthContext",
    "Body",
    "BodyProcessingMode",
    "DownstreamResponseHeaderModifications",
    "DownstreamResponseModifications",
    "DropHeaderAction",
    "ExecutionContext",
    "ExecutionPhase",
    "ForwardRequestChunk",
    "ForwardResponseChunk",
    "HeaderProcessingMode",
    "Headers",
    "ImmediateResponse",
    "Policy",
    "PolicyMetadata",
    "ProcessingMode",
    "RequestAction",
    "RequestContext",
    "RequestHeaderAction",
    "RequestHeaderContext",
    "RequestHeaderPolicy",
    "RequestPolicy",
    "RequestStreamContext",
    "ResponseAction",
    "ResponseContext",
    "ResponseHeaderAction",
    "ResponseHeaderContext",
    "ResponseHeaderPolicy",
    "ResponsePolicy",
    "ResponseStreamContext",
    "SharedContext",
    "StreamBody",
    "StreamingRequestAction",
    "StreamingRequestPolicy",
    "StreamingResponseAction",
    "StreamingResponsePolicy",
    "TerminateResponseChunk",
    "UpstreamRequestHeaderModifications",
    "UpstreamRequestModifications",
]
