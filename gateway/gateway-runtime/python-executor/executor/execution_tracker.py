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

"""Thread-safe execution tracking for in-flight policy calls."""

from __future__ import annotations

import threading
from dataclasses import dataclass
from datetime import datetime

from sdk.types import ExecutionPhase


@dataclass(slots=True)
class ExecutionState:
    request_id: str
    instance_id: str
    phase: ExecutionPhase
    deadline: datetime | None
    cancelled: bool = False
    waiter_done: bool = False
    worker_done: bool = False


class ExecutionTracker:
    """Tracks cancellation and cleanup state for in-flight executions."""

    def __init__(self):
        self._lock = threading.Lock()
        self._executions: dict[str, ExecutionState] = {}

    def register(
        self,
        request_id: str,
        instance_id: str,
        phase: ExecutionPhase,
        deadline: datetime | None,
    ) -> None:
        with self._lock:
            if request_id in self._executions:
                raise ValueError(f"execution already registered: {request_id}")
            self._executions[request_id] = ExecutionState(
                request_id=request_id,
                instance_id=instance_id,
                phase=phase,
                deadline=deadline,
            )

    def cancel(self, request_id: str) -> bool:
        with self._lock:
            state = self._executions.get(request_id)
            if state is None:
                return False
            state.cancelled = True
            return True

    def is_cancelled(self, request_id: str) -> bool:
        with self._lock:
            state = self._executions.get(request_id)
            return False if state is None else state.cancelled

    def mark_waiter_done(self, request_id: str) -> None:
        with self._lock:
            state = self._executions.get(request_id)
            if state is None:
                return
            state.waiter_done = True
            if state.worker_done:
                self._executions.pop(request_id, None)

    def mark_worker_done(self, request_id: str) -> None:
        with self._lock:
            state = self._executions.get(request_id)
            if state is None:
                return
            state.worker_done = True
            if state.waiter_done:
                self._executions.pop(request_id, None)
