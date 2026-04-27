"""APIP SDK Core.

Recommended imports come from this package root:

    from apip_sdk_core import RequestPolicy, ProcessingMode

If callers need to pin a specific contract version explicitly, they can use:

    from apip_sdk_core.policy.v1alpha2 import RequestPolicy, ProcessingMode
"""

from importlib.metadata import PackageNotFoundError, version

from . import policy
from .policy import *  # noqa: F401,F403

try:
    __version__: str = version("apip-sdk-core")
except PackageNotFoundError:
    __version__ = "0.0.0+unknown"

__all__ = list(policy.__all__)
__all__.extend(["policy", "__version__"])
