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

"""gRPC server implementation for Python Executor."""

import asyncio
import collections
import logging
import os
import signal
import stat
import uuid
from concurrent import futures
from typing import AsyncIterator, Optional

import grpc
from grpc import aio

from executor.policy_loader import PolicyLoader
from executor.instance_store import PolicyInstanceStore
from executor.translator import Translator
import proto.python_executor_pb2 as proto
import proto.python_executor_pb2_grpc as proto_grpc

logger = logging.getLogger(__name__)


class PythonExecutorServicer(proto_grpc.PythonExecutorServiceServicer):
    """gRPC servicer for Python Executor."""

    def __init__(
        self,
        policy_loader: PolicyLoader,
        instance_store: PolicyInstanceStore,
        worker_count: int = 4,
        max_concurrent: int = 100,
    ):
        self._loader = policy_loader
        self._store = instance_store
        self._translator = Translator()
        self._executor = futures.ThreadPoolExecutor(max_workers=worker_count)
        self._max_concurrent = max_concurrent
        logger.info(
            f"PythonExecutorServicer initialized with {worker_count} workers, "
            f"max_concurrent={max_concurrent}"
        )

    def close(self):
        """Shutdown the thread pool executor."""
        self._executor.shutdown(wait=True)

    # ------------------------------------------------------------------ #
    #  InitPolicy — called once per route during chain building           #
    # ------------------------------------------------------------------ #

    async def InitPolicy(
        self,
        request: proto.InitPolicyRequest,
        context: grpc.ServicerContext,
    ) -> proto.InitPolicyResponse:
        """Create a policy instance via the factory and store it."""
        try:
            loop = asyncio.get_event_loop()
            response = await loop.run_in_executor(
                self._executor,
                self._init_policy_sync,
                request,
            )
            return response
        except Exception as e:
            logger.exception(
                f"InitPolicy failed for {request.policy_name}:{request.policy_version}"
            )
            return proto.InitPolicyResponse(
                success=False,
                error_message=str(e),
            )

    def _init_policy_sync(
        self, request: proto.InitPolicyRequest
    ) -> proto.InitPolicyResponse:
        """Synchronous init — runs in thread pool."""
        factory = self._loader.get_factory(
            request.policy_name, request.policy_version
        )
        metadata = self._translator.to_python_policy_metadata(request.policy_metadata)
        params = self._translator.struct_to_dict(request.params)

        instance = factory(metadata, params)

        instance_id = str(uuid.uuid4())
        self._store.put(instance_id, instance)

        logger.info(
            f"InitPolicy OK: {request.policy_name}:{request.policy_version} "
            f"route={metadata.route_name} instance_id={instance_id}"
        )
        return proto.InitPolicyResponse(success=True, instance_id=instance_id)

    # ------------------------------------------------------------------ #
    #  DestroyPolicy — called when a route is removed or replaced         #
    # ------------------------------------------------------------------ #

    async def DestroyPolicy(
        self,
        request: proto.DestroyPolicyRequest,
        context: grpc.ServicerContext,
    ) -> proto.DestroyPolicyResponse:
        """Destroy a policy instance: call close() and remove from store."""
        try:
            loop = asyncio.get_event_loop()
            response = await loop.run_in_executor(
                self._executor,
                self._destroy_policy_sync,
                request,
            )
            return response
        except Exception as e:
            logger.exception(
                f"DestroyPolicy failed for instance_id={request.instance_id}"
            )
            return proto.DestroyPolicyResponse(
                success=False,
                error_message=str(e),
            )

    def _destroy_policy_sync(
        self, request: proto.DestroyPolicyRequest
    ) -> proto.DestroyPolicyResponse:
        """Synchronous destroy — runs in thread pool."""
        result = self._store.mark_for_destruction(request.instance_id)
        if result is None:
            logger.warning(
                f"DestroyPolicy: instance_id={request.instance_id} not found (already destroyed?)"
            )
            return proto.DestroyPolicyResponse(success=True)

        instance, should_close = result
        if should_close:
            try:
                instance.close()
            except Exception as e:
                logger.warning(
                    f"DestroyPolicy: close() raised for instance_id={request.instance_id}: {e}"
                )
                # Still consider success — the instance is marked and removed from the active store
                return proto.DestroyPolicyResponse(success=True, error_message=str(e))

        logger.info(f"DestroyPolicy OK: instance_id={request.instance_id}")
        return proto.DestroyPolicyResponse(success=True)

    # ------------------------------------------------------------------ #
    #  ExecuteStream — hot-path request/response execution                #
    # ------------------------------------------------------------------ #

    async def ExecuteStream(
        self,
        request_iterator: AsyncIterator[proto.ExecutionRequest],
        context: grpc.ServicerContext,
    ) -> AsyncIterator[proto.ExecutionResponse]:
        """Handle bidirectional streaming execution requests with concurrent fan-out.

        - Reader task: eagerly consumes request_iterator, spawns a processing task per request
        - Processing tasks: execute policy in thread pool, append result to deque and notify
        - Writer: wakes on asyncio.Condition, drains the deque, yields responses (out-of-order)

        The Go side correlates responses by request_id, so order doesn't matter.
        """
        response_deque: collections.deque[proto.ExecutionResponse] = collections.deque()
        response_ready = asyncio.Condition()
        in_flight: set[asyncio.Task] = set()
        reader_done = asyncio.Event()
        concurrency_limit = asyncio.Semaphore(self._max_concurrent)

        async def process_request(request: proto.ExecutionRequest) -> None:
            response: Optional[proto.ExecutionResponse] = None
            try:
                loop = asyncio.get_event_loop()
                response = await loop.run_in_executor(
                    self._executor,
                    self._execute_policy,
                    request,
                )
            except Exception as e:
                logger.exception(
                    f"Error executing policy {request.policy_name}:{request.policy_version}"
                )
                response = self._error_response(
                    request.request_id, e, request.policy_name, request.policy_version
                )

            async with response_ready:
                try:
                    if response is not None:
                        response_deque.append(response)
                finally:
                    # Keep in-flight updates under the same condition lock so
                    # writers observe deque/in_flight state changes atomically.
                    in_flight.discard(asyncio.current_task())
                    response_ready.notify()

            # Release capacity after notifying waiters to re-check in_flight.
            concurrency_limit.release()

        async def reader() -> None:
            try:
                async for request in request_iterator:
                    await concurrency_limit.acquire()
                    task = asyncio.create_task(process_request(request))
                    in_flight.add(task)
            except asyncio.CancelledError:
                logger.info("Reader cancelled")
            except Exception:
                logger.exception("Reader encountered error")
            finally:
                async with response_ready:
                    try:
                        reader_done.set()
                    finally:
                        response_ready.notify()

        reader_task = asyncio.create_task(reader())

        try:
            while True:
                async with response_ready:
                    while not response_deque:
                        if reader_done.is_set() and len(in_flight) == 0:
                            break
                        await response_ready.wait()
                    batch = list(response_deque)
                    response_deque.clear()

                if not batch and reader_done.is_set() and len(in_flight) == 0:
                    break

                for resp in batch:
                    yield resp

        except asyncio.CancelledError:
            logger.debug("ExecuteStream cancelled, cleaning up")
            reader_task.cancel()
            for task in list(in_flight):
                task.cancel()
            all_tasks = [reader_task] + list(in_flight)
            await asyncio.shield(asyncio.gather(*all_tasks, return_exceptions=True))
            raise
        finally:
            if not reader_task.done():
                reader_task.cancel()
                try:
                    await reader_task
                except asyncio.CancelledError:
                    pass

    def _execute_policy(self, request: proto.ExecutionRequest) -> proto.ExecutionResponse:
        """Execute a single policy request (runs in thread pool).

        Looks up the policy instance by instance_id, then calls
        on_request or on_response.
        """
        record = self._store.acquire_for_execution(request.instance_id)
        if record is None:
            raise ValueError(
                f"No policy instance for instance_id={request.instance_id} "
                f"(policy={request.policy_name}:{request.policy_version})"
            )

        try:
            instance = record.policy
            # Use per-call params from the request (Go sends the merged system+user params
            # on every call). This is necessary for singleton policies where one Python
            # object is shared across multiple routes with different params each.
            params = self._translator.struct_to_dict(request.params)
            shared_ctx = self._translator.to_python_shared_context(request.shared_context)

            if request.phase == "on_request":
                req_ctx = self._translator.to_python_request_context(
                    request.request_context, shared_ctx
                )
                action = instance.on_request(req_ctx, params)
                updated_metadata = self._dict_to_struct(shared_ctx.metadata)
                return proto.ExecutionResponse(
                    request_id=request.request_id,
                    request_result=self._translator.to_proto_request_action_result(action),
                    updated_metadata=updated_metadata,
                )

            elif request.phase == "on_response":
                resp_ctx = self._translator.to_python_response_context(
                    request.response_context, shared_ctx
                )
                action = instance.on_response(resp_ctx, params)
                updated_metadata = self._dict_to_struct(shared_ctx.metadata)
                return proto.ExecutionResponse(
                    request_id=request.request_id,
                    response_result=self._translator.to_proto_response_action_result(action),
                    updated_metadata=updated_metadata,
                )

            else:
                raise ValueError(f"Unknown phase: {request.phase}")

        finally:
            if self._store.release_execution(request.instance_id):
                try:
                    record.policy.close()
                    logger.info(f"Delayed DestroyPolicy OK: instance_id={request.instance_id}")
                except Exception as e:
                    logger.warning(
                        f"DestroyPolicy: close() raised during delayed destruction for instance_id={request.instance_id}: {e}"
                    )

    def _error_response(
        self,
        request_id: str,
        error: Exception,
        policy_name: str,
        policy_version: str,
    ) -> proto.ExecutionResponse:
        """Create an error response."""
        return proto.ExecutionResponse(
            request_id=request_id,
            error=proto.ExecutionError(
                message=str(error),
                policy_name=policy_name,
                policy_version=policy_version,
                error_type="execution_error",
            ),
        )

    @staticmethod
    def _dict_to_struct(d: dict):
        from google.protobuf.struct_pb2 import Struct
        s = Struct()
        if d:
            s.update(d)
        return s

    async def HealthCheck(
        self,
        request: proto.HealthCheckRequest,
        context: grpc.ServicerContext,
    ) -> proto.HealthCheckResponse:
        """Health check endpoint."""
        loaded = self._loader.get_loaded_policy_count()
        return proto.HealthCheckResponse(ready=True, loaded_policies=loaded)


class PythonExecutorServer:
    """Python Executor gRPC server."""

    def __init__(self, socket_path: str, worker_count: int = 4, max_concurrent: int = 100):
        self.socket_path = socket_path
        self.worker_count = worker_count
        self.max_concurrent = max_concurrent
        self.server: Optional[aio.Server] = None
        self._loader = PolicyLoader()
        self._store = PolicyInstanceStore()
        self._servicer: Optional[PythonExecutorServicer] = None
        self._aio_worker_executor: Optional[futures.ThreadPoolExecutor] = None

    async def start(self):
        """Start the server."""
        logger.info(f"Starting Python Executor on {self.socket_path}")

        loaded_count = self._loader.load_policies()
        logger.info(f"Loaded {loaded_count} policy factories")

        self._aio_worker_executor = futures.ThreadPoolExecutor(max_workers=self.worker_count)
        self.server = aio.server(
            migration_thread_pool=self._aio_worker_executor,
            options=[
                ('grpc.max_send_message_length', 50 * 1024 * 1024),
                ('grpc.max_receive_message_length', 50 * 1024 * 1024),
            ],
        )

        self._servicer = PythonExecutorServicer(
            self._loader, self._store, self.worker_count, self.max_concurrent
        )
        proto_grpc.add_PythonExecutorServiceServicer_to_server(self._servicer, self.server)

        if os.path.exists(self.socket_path):
            try:
                st = os.stat(self.socket_path)
                if stat.S_ISSOCK(st.st_mode):
                    os.remove(self.socket_path)
                else:
                    raise RuntimeError(f"Path exists but is not a socket: {self.socket_path}")
            except OSError as e:
                logger.error(f"Error checking or removing socket path at {self.socket_path}: {e}")
                raise

        self.server.add_insecure_port(f"unix:{self.socket_path}")
        await self.server.start()
        logger.info(f"Python Executor ready on {self.socket_path}")

    async def wait_for_termination(self):
        if self.server:
            await self.server.wait_for_termination()

    async def shutdown(self):
        """Shutdown: close all live policy instances, then stop server."""
        logger.info("Shutting down Python Executor...")

        # 1. Stop accepting new requests and allow in-flight RPCs to finish
        if self.server:
            await self.server.stop(grace=5)

        # 2. Stop accepting new thread pool tasks from the servicer
        if self._servicer:
            self._servicer.close()

        # 3. Clean up policy instances now that no requests are touching them
        instances = self._store.clear()
        for instance in instances:
            try:
                instance.close()
            except Exception as e:
                logger.warning(f"Error closing policy instance during shutdown: {e}")

        # 4. Shut down the core IO/server thread pool last
        if self._aio_worker_executor:
            self._aio_worker_executor.shutdown(wait=True)

        logger.info("Python Executor shutdown complete")
