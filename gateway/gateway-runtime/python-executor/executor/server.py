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
import logging
import os
import signal
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
        instance = self._store.remove(request.instance_id)
        if instance is None:
            logger.warning(
                f"DestroyPolicy: instance_id={request.instance_id} not found (already destroyed?)"
            )
            return proto.DestroyPolicyResponse(success=True)

        try:
            instance.close()
        except Exception as e:
            logger.warning(
                f"DestroyPolicy: close() raised for instance_id={request.instance_id}: {e}"
            )
            # Still consider success — the instance is removed from the store
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

        Architecture unchanged from current implementation:
        - Reader task: eagerly consumes request_iterator, spawns a processing task per request
        - Processing tasks: execute policy in thread pool, put result in response queue
        - Writer (this generator): yields responses from queue as they complete (out-of-order)

        The Go side correlates responses by request_id, so order doesn't matter.
        """
        response_queue: asyncio.Queue[proto.ExecutionResponse] = asyncio.Queue(
            maxsize=self._max_concurrent
        )
        in_flight: set[asyncio.Task] = set()
        reader_done = asyncio.Event()
        concurrency_limit = asyncio.Semaphore(self._max_concurrent)

        async def process_request(request: proto.ExecutionRequest) -> None:
            try:
                async with concurrency_limit:
                    loop = asyncio.get_event_loop()
                    response = await loop.run_in_executor(
                        self._executor,
                        self._execute_policy,
                        request,
                    )
                    await response_queue.put(response)
            except Exception as e:
                logger.exception(
                    f"Error executing policy {request.policy_name}:{request.policy_version}"
                )
                error_resp = self._error_response(
                    request.request_id, e, request.policy_name, request.policy_version
                )
                await response_queue.put(error_resp)

        async def reader() -> None:
            try:
                async for request in request_iterator:
                    task = asyncio.create_task(process_request(request))
                    in_flight.add(task)
                    task.add_done_callback(in_flight.discard)
            except asyncio.CancelledError:
                logger.info("Reader cancelled")
            except Exception:
                logger.exception("Reader encountered error")
            finally:
                reader_done.set()

        reader_task = asyncio.create_task(reader())

        try:
            while True:
                try:
                    response = response_queue.get_nowait()
                    yield response
                    continue
                except asyncio.QueueEmpty:
                    pass

                if reader_done.is_set() and len(in_flight) == 0 and response_queue.empty():
                    break

                try:
                    response = await asyncio.wait_for(response_queue.get(), timeout=0.1)
                    yield response
                except asyncio.TimeoutError:
                    continue

        except asyncio.CancelledError:
            logger.info("ExecuteStream cancelled, cleaning up")
            reader_task.cancel()
            try:
                await reader_task
            except asyncio.CancelledError:
                pass
            for task in list(in_flight):
                task.cancel()
            if in_flight:
                await asyncio.wait(in_flight, timeout=5.0)
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
        instance = self._store.get(request.instance_id)
        if instance is None:
            raise ValueError(
                f"No policy instance for instance_id={request.instance_id} "
                f"(policy={request.policy_name}:{request.policy_version})"
            )

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

    async def start(self):
        """Start the server."""
        logger.info(f"Starting Python Executor on {self.socket_path}")

        loaded_count = self._loader.load_policies()
        logger.info(f"Loaded {loaded_count} policy factories")

        self.server = aio.server(
            migration_thread_pool=futures.ThreadPoolExecutor(max_workers=self.worker_count),
            options=[
                ('grpc.max_send_message_length', 50 * 1024 * 1024),
                ('grpc.max_receive_message_length', 50 * 1024 * 1024),
            ],
        )

        servicer = PythonExecutorServicer(
            self._loader, self._store, self.worker_count, self.max_concurrent
        )
        proto_grpc.add_PythonExecutorServiceServicer_to_server(servicer, self.server)

        if os.path.exists(self.socket_path):
            os.remove(self.socket_path)

        self.server.add_insecure_port(f"unix:{self.socket_path}")
        await self.server.start()
        logger.info(f"Python Executor ready on {self.socket_path}")

    async def wait_for_termination(self):
        if self.server:
            await self.server.wait_for_termination()

    async def shutdown(self):
        """Shutdown: close all live policy instances, then stop server."""
        logger.info("Shutting down Python Executor...")

        # Close all live instances
        instances = self._store.clear()
        for instance in instances:
            try:
                instance.close()
            except Exception as e:
                logger.warning(f"Error closing policy instance during shutdown: {e}")

        if self.server:
            await self.server.stop(grace_period=5)
        logger.info("Python Executor shutdown complete")
