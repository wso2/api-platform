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

"""Add Header Singleton Policy — sample Python policy

What this policy does
---------------------
Injects a configured HTTP header into the incoming request.
This policy is written as a global singleton. Instead of parsing
parameters in `get_policy` (which allocates a new object per-route),
this policy retains no internal configuration. It explicitly uses the
`params` dictionary passed to `on_request()` on the hot path to look
up the correct configuration for the route currently executing.
"""

import logging
from typing import Any, Dict

from sdk.policy import (
    Policy,
    PolicyMetadata,
    RequestContext,
    ResponseContext,
    RequestAction,
    ResponseAction,
    ProcessingMode,
    HeaderProcessingMode,
    BodyProcessingMode,
    UpstreamRequestModifications,
)

logger = logging.getLogger("add-header-singleton")

class AddHeaderSingletonPolicy(Policy):
    """Adds a configurable header to the request using a Singleton pattern.

    Since this exact instance is shared across all API routes, we absolutely 
    CANNOT save `headerName` or `headerValue` as instance attributes `self.XXX`.
    Doing so would cause terrible race conditions between different APIs!

    Instead, we pull the variables dynamically out of `params` every time.
    """

    # ------------------------------------------------------------------
    # mode — inform the Go kernel about buffering requirements
    # ------------------------------------------------------------------
    def mode(self) -> ProcessingMode:
        return ProcessingMode(
            request_header_mode=HeaderProcessingMode.PROCESS, # Only require Req Headers
            request_body_mode=BodyProcessingMode.SKIP,
            response_header_mode=HeaderProcessingMode.SKIP,
            response_body_mode=BodyProcessingMode.SKIP,
        )

    # ------------------------------------------------------------------
    # on_request — hot path
    # ------------------------------------------------------------------
    def on_request(self, ctx: RequestContext, params: Dict[str, Any]) -> RequestAction:
        # Since this is a singleton, we MUST read configuration out of `params`
        # instead of `self`. The Python server merges System + Route Params and 
        # sends them to us over the gRPC stream.
        
        header_name = str(params.get("headerName", "X-Singleton-Header"))
        header_value = str(params.get("headerValue", "True"))

        logger.debug(
            "on_request: Adding header %s: %s (Singleton Reference: %s)",
            header_name, header_value, id(self)
        )

        # Tell the Go Policy Engine to inject this header before forwarding upstream
        return UpstreamRequestModifications(
            set_headers={header_name: header_value}
        )

    def on_response(self, ctx: ResponseContext, params: Dict[str, Any]) -> ResponseAction:
        # No work to do on the response path
        return None

    def close(self) -> None:
        """Called when a route detaches.
        
        If 50 routes use this Singleton, the Python Server's Active Count 
        remains > 0. Once the VERY LAST route detaches (so Active Count drops 
        to exactly 0), only then will the server call this method!
        """
        logger.info("close: AddHeaderSingletonPolicy pointer finally destroyed")


# ======================================================================
# THE SINGLETON INSTANCE
# ======================================================================
# We create a single, global memory reference when the module imports.
_global_shared_instance = AddHeaderSingletonPolicy()

def get_policy(metadata: PolicyMetadata, params: Dict[str, Any]) -> Policy:
    """Invoked initially when the Go control plane binds a route.
    
    Instead of `return AddHeaderSingletonPolicy(args=...)`, we pass back
    the exact same pointer every single time for every route.
    """
    logger.info(
        "get_policy: Sharing SINGLETON policy pointer (id=%s) for route=%r api=%r",
        id(_global_shared_instance),
        metadata.route_name,
        metadata.api_name,
    )
    
    return _global_shared_instance
