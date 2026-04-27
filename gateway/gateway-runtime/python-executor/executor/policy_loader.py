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

"""Policy discovery and import for Python policy factories."""

import importlib
import logging
import os
import sys
from typing import Callable, Dict, Optional

from wso2_gateway_policy_sdk import Policy, PolicyMetadata

logger = logging.getLogger(__name__)

# Type alias for the factory function signature.
# Mirrors Go's PolicyFactory: func(metadata, params) -> (Policy, error)
PolicyFactory = Callable[[PolicyMetadata, Dict], Policy]


class PolicyLoader:
    """Loads Python policy factory functions from the generated registry.

    At startup, imports each policy module and extracts its get_policy()
    factory function.  The factory is stored — instances are created later
    via InitPolicy RPC.
    """

    def __init__(self):
        self._factories: Dict[str, PolicyFactory] = {}
        self._loaded = False

    def load_policies(self) -> int:
        """Load all policy factories from the registry.

        Returns:
            Number of factories loaded.
        """
        if self._loaded:
            return len(self._factories)

        executor_dir = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
        if executor_dir not in sys.path:
            sys.path.insert(0, executor_dir)

        try:
            from python_policy_registry import PYTHON_POLICIES
            registry = PYTHON_POLICIES
            logger.info(f"Loaded policy registry with {len(registry)} entries")
        except ImportError:
            logger.warning("No policy registry found (python_policy_registry.py). "
                          "No Python policies will be available.")
            registry = {}

        for policy_key, module_path in registry.items():
            self._load_policy(policy_key, module_path)

        self._loaded = True
        return len(self._factories)

    def _load_policy(self, policy_key: str, module_path: str) -> bool:
        """Load a single policy factory from its module.

        The module must export a callable get_policy(metadata, params) -> Policy.
        """
        try:
            module = importlib.import_module(module_path)

            if not hasattr(module, 'get_policy'):
                logger.error(
                    f"Policy module {module_path} does not export 'get_policy' factory"
                )
                return False

            factory = getattr(module, 'get_policy')
            if not callable(factory):
                logger.error(f"Policy module {module_path} 'get_policy' is not callable")
                return False

            self._factories[policy_key] = factory
            logger.info(f"Loaded policy factory: {policy_key} from {module_path}")
            return True

        except Exception as e:
            logger.error(f"Failed to load policy {policy_key} from {module_path}: {e}")
            return False

    def get_factory(self, name: str, version: str) -> PolicyFactory:
        """Get the factory function for a policy.

        Args:
            name: Policy name.
            version: Policy version.

        Returns:
            The get_policy factory callable.

        Raises:
            KeyError: If policy not found.
        """
        key = f"{name}:{version}"
        if key in self._factories:
            return self._factories[key]

        # Fallback: try major version only (e.g., "my-policy:v1.0.0" -> "my-policy:v1")
        major_version = version.split('.')[0] if '.' in version else version
        key_major = f"{name}:{major_version}"
        if key_major in self._factories:
            return self._factories[key_major]

        raise KeyError(f"Policy factory not found: {name}:{version}")

    def get_loaded_policies(self) -> list:
        """Get list of loaded policy keys."""
        return list(self._factories.keys())

    def get_loaded_policy_count(self) -> int:
        """Get the number of loaded policy factories."""
        return len(self._factories)
