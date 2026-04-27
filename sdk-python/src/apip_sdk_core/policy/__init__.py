"""Versioned Python policy interfaces for APIP SDK Core."""

from . import v1alpha2
from .v1alpha2 import *  # noqa: F401,F403

__all__ = [*v1alpha2.__all__, "v1alpha2"]
