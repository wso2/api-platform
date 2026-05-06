import unittest
from datetime import datetime, timezone

from executor.execution_tracker import ExecutionTracker
from apip_sdk_core import ExecutionPhase


class ExecutionTrackerTest(unittest.TestCase):
    def test_cancellation_state_cleans_up_after_waiter_and_worker_finish(self):
        tracker = ExecutionTracker()
        tracker.register(
            request_id="req-1",
            instance_id="instance-1",
            phase=ExecutionPhase.REQUEST_BODY,
            deadline=datetime.now(timezone.utc),
        )

        self.assertTrue(tracker.cancel("req-1"))
        self.assertTrue(tracker.is_cancelled("req-1"))

        tracker.mark_waiter_done("req-1")
        self.assertIn("req-1", tracker._executions)

        tracker.mark_worker_done("req-1")
        self.assertNotIn("req-1", tracker._executions)
        self.assertFalse(tracker.is_cancelled("req-1"))

    def test_duplicate_registration_is_rejected(self):
        tracker = ExecutionTracker()
        tracker.register(
            request_id="req-1",
            instance_id="instance-1",
            phase=ExecutionPhase.REQUEST_HEADERS,
            deadline=None,
        )

        with self.assertRaisesRegex(ValueError, "execution already registered"):
            tracker.register(
                request_id="req-1",
                instance_id="instance-1",
                phase=ExecutionPhase.REQUEST_HEADERS,
                deadline=None,
            )
