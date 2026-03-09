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

"""Thread-safe store for live Python policy instances."""

import logging
import threading
from typing import Optional

from sdk.policy import Policy

logger = logging.getLogger(__name__)


class PolicyInstanceStore:
    """Maps instance_id → Policy instance.

    Thread-safe — guards all mutations with a lock.
    The store does not own instancing logic; it only holds references.
    """

    def __init__(self):
        self._lock = threading.Lock()
        self._instances: dict[str, Policy] = {}

    def put(self, instance_id: str, instance: Policy) -> None:
        """Store an instance. Overwrites silently if the ID already exists."""
        with self._lock:
            self._instances[instance_id] = instance

    def get(self, instance_id: str) -> Optional[Policy]:
        """Look up an instance. Returns None if not found."""
        with self._lock:
            return self._instances.get(instance_id)

    def remove(self, instance_id: str) -> Optional[Policy]:
        """Remove and return an instance. Returns None if not found."""
        with self._lock:
            return self._instances.pop(instance_id, None)

    def count(self) -> int:
        """Return the number of live instances."""
        with self._lock:
            return len(self._instances)

    def clear(self) -> list[Policy]:
        """Remove and return all instances (for shutdown cleanup)."""
        with self._lock:
            instances = list(self._instances.values())
            self._instances.clear()
            return instances
