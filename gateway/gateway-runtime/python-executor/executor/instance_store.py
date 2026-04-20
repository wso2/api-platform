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

"""Thread-safe store for live Python policy instances."""

import logging
import threading
from dataclasses import dataclass
from typing import Optional

from wso2_gateway_policy_sdk import Policy, PolicyMetadata

logger = logging.getLogger(__name__)


@dataclass
class InstanceRecord:
    """Holds a policy instance."""
    policy: Policy
    policy_name: str
    policy_version: str
    metadata: PolicyMetadata
    active_count: int = 0
    pending_destroy: bool = False


class PolicyInstanceStore:
    """Maps instance_id → InstanceRecord.

    Thread-safe — guards all mutations with a lock.
    The store does not own instancing logic; it only holds references.
    """

    def __init__(self):
        self._lock = threading.Lock()
        self._instances: dict[str, InstanceRecord] = {}

    def put(
        self,
        instance_id: str,
        instance: Policy,
        policy_name: str,
        policy_version: str,
        metadata: PolicyMetadata,
    ) -> None:
        """Store an instance. Overwrites silently if the ID already exists."""
        with self._lock:
            self._instances[instance_id] = InstanceRecord(
                policy=instance,
                policy_name=policy_name,
                policy_version=policy_version,
                metadata=metadata,
            )

    def get(self, instance_id: str) -> Optional[InstanceRecord]:
        """Look up an instance record. Returns None if not found."""
        with self._lock:
            return self._instances.get(instance_id)

    def acquire_for_execution(self, instance_id: str) -> Optional[InstanceRecord]:
        """Increment active_count and return the record if it exists and is not pending destruction."""
        with self._lock:
            record = self._instances.get(instance_id)
            if record is not None and not record.pending_destroy:
                record.active_count += 1
                return record
            return None

    def release_execution(self, instance_id: str) -> bool:
        """Decrement active_count. Returns True if the instance should be closed and removed."""
        with self._lock:
            record = self._instances.get(instance_id)
            if record is not None:
                record.active_count -= 1
                if record.active_count == 0 and record.pending_destroy:
                    self._instances.pop(instance_id, None)
                    return True
        return False

    def mark_for_destruction(self, instance_id: str) -> Optional[tuple[Policy, bool]]:
        """Mark instance for destruction. Returns (policy, should_close_now) if found."""
        with self._lock:
            record = self._instances.get(instance_id)
            if record is not None:
                record.pending_destroy = True
                if record.active_count == 0:
                    self._instances.pop(instance_id, None)
                    return record.policy, True
                return record.policy, False
        return None

    def remove(self, instance_id: str) -> Optional[Policy]:
        """Remove and return a policy instance. Returns None if not found."""
        with self._lock:
            record = self._instances.pop(instance_id, None)
            return record.policy if record is not None else None

    def count(self) -> int:
        """Return the number of live instances."""
        with self._lock:
            return len(self._instances)

    def clear(self) -> list[Policy]:
        """Remove and return all policy instances (for shutdown cleanup)."""
        with self._lock:
            policies = [r.policy for r in self._instances.values()]
            self._instances.clear()
            return policies
