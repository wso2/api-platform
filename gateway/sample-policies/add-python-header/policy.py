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

"""Sample Python policy that adds a custom header to requests.

Demonstrates the class-based Policy pattern with a get_policy() factory.
This is a simple stateless policy — the factory returns a new instance
each time (no caching needed for this use case).
"""

from typing import Any, Dict

from sdk.policy import (
    Policy,
    PolicyMetadata,
    RequestContext,
    ResponseContext,
    UpstreamRequestModifications,
    RequestAction,
    ResponseAction,
)


class AddPythonHeaderPolicy(Policy):
    """Adds a configurable header to upstream requests."""

    def __init__(self, header_name: str, header_value: str):
        self._header_name = header_name
        self._header_value = header_value

    def on_request(self, ctx: RequestContext, params: Dict[str, Any]) -> RequestAction:
        return UpstreamRequestModifications(
            set_headers={self._header_name: self._header_value}
        )

    def on_response(self, ctx: ResponseContext, params: Dict[str, Any]) -> ResponseAction:
        return None


def get_policy(metadata: PolicyMetadata, params: Dict[str, Any]) -> Policy:
    """Factory function — mirrors Go's GetPolicy."""
    header_name = params.get("headerName", "X-Python-Policy")
    header_value = params.get("headerValue", "hello-from-python")
    return AddPythonHeaderPolicy(header_name, header_value)
