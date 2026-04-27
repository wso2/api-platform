"""WSO2 Gateway Policy SDK.

Recommended imports come from this package root:

    from wso2_gateway_policy_sdk import RequestPolicy, ProcessingMode

If callers need to pin a specific contract version explicitly, they can use:

    from wso2_gateway_policy_sdk.policy.v1alpha2 import RequestPolicy, ProcessingMode
"""

from . import policy
from .policy import *  # noqa: F401,F403

__all__ = [*policy.__all__, "policy"]
