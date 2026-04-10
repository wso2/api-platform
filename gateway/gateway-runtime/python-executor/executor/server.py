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

"""gRPC server implementation for the Python executor."""

from __future__ import annotations

import asyncio
import collections
import logging
import os
import stat
import uuid
from concurrent import futures
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import AsyncIterator, Optional

import grpc
from grpc import aio

from executor.execution_tracker import ExecutionTracker
from executor.instance_store import InstanceRecord, PolicyInstanceStore
from executor.policy_loader import PolicyLoader
from executor.translator import Translator
import proto.python_executor_pb2 as proto
import proto.python_executor_pb2_grpc as proto_grpc
from sdk.policy import (
    Policy,
    RequestHeaderPolicy,
    RequestPolicy,
    ResponseHeaderPolicy,
    ResponsePolicy,
    StreamingRequestPolicy,
    StreamingResponsePolicy,
)
from sdk.types import BodyProcessingMode, ExecutionContext, HeaderProcessingMode

logger = logging.getLogger(__name__)


@dataclass(slots=True)
class PolicyCapabilities:
    request_headers: bool
    request_body: bool
    response_headers: bool
    response_body: bool
    streaming_request: bool
    streaming_response: bool


class PythonExecutorServicer(proto_grpc.PythonExecutorServiceServicer):
    """gRPC servicer for Python policy execution."""

    _PAYLOAD_TO_PHASE = {
        "request_headers": proto.PHASE_REQUEST_HEADERS,
        "request_body": proto.PHASE_REQUEST_BODY,
        "response_headers": proto.PHASE_RESPONSE_HEADERS,
        "response_body": proto.PHASE_RESPONSE_BODY,
        "needs_more_request_data": proto.PHASE_NEEDS_MORE_REQUEST_DATA,
        "request_chunk": proto.PHASE_REQUEST_BODY_CHUNK,
        "needs_more_response_data": proto.PHASE_NEEDS_MORE_RESPONSE_DATA,
        "response_chunk": proto.PHASE_RESPONSE_BODY_CHUNK,
        "cancel_execution": proto.PHASE_CANCEL,
    }

    def __init__(
        self,
        policy_loader: PolicyLoader,
        instance_store: PolicyInstanceStore,
        execution_tracker: ExecutionTracker,
        executor: futures.ThreadPoolExecutor,
        max_concurrent: int = 100,
        timeout: int = 30,
    ):
        self._loader = policy_loader
        self._store = instance_store
        self._tracker = execution_tracker
        self._translator = Translator()
        self._executor = executor
        self._max_concurrent = max_concurrent
        self._timeout = timeout
        logger.info(
            "PythonExecutorServicer initialized with shared executor, max_concurrent=%s, timeout=%ss",
            max_concurrent,
            timeout,
        )

    def close(self) -> None:
        """Servicer-specific shutdown logic is handled by the server wrapper."""

    async def InitPolicy(
        self,
        request: proto.InitPolicyRequest,
        context: grpc.ServicerContext,
    ) -> proto.InitPolicyResponse:
        """Create a policy instance and return its validated capabilities."""
        try:
            loop = asyncio.get_running_loop()
            return await loop.run_in_executor(self._executor, self._init_policy_sync, request)
        except Exception as exc:
            logger.exception(
                "InitPolicy failed for %s:%s",
                request.policy_name,
                request.policy_version,
            )
            return proto.InitPolicyResponse(success=False, error_message=str(exc))

    def _init_policy_sync(self, request: proto.InitPolicyRequest) -> proto.InitPolicyResponse:
        factory = self._loader.get_factory(request.policy_name, request.policy_version)
        metadata = self._translator.to_python_policy_metadata(request.policy_metadata)
        params = self._translator.struct_to_dict(request.params)

        instance = factory(metadata, params)
        mode = instance.mode()
        capabilities = self._validate_policy_contract(instance, mode)

        instance_id = str(uuid.uuid4())
        self._store.put(
            instance_id=instance_id,
            instance=instance,
            policy_name=request.policy_name,
            policy_version=request.policy_version,
            metadata=metadata,
        )

        logger.info(
            "InitPolicy OK: %s:%s route=%s instance_id=%s",
            request.policy_name,
            request.policy_version,
            metadata.route_name,
            instance_id,
        )
        return proto.InitPolicyResponse(
            success=True,
            instance_id=instance_id,
            processing_mode=self._translator.to_proto_processing_mode(mode),
            capabilities=proto.PolicyCapabilities(
                request_headers=capabilities.request_headers,
                request_body=capabilities.request_body,
                response_headers=capabilities.response_headers,
                response_body=capabilities.response_body,
                streaming_request=capabilities.streaming_request,
                streaming_response=capabilities.streaming_response,
            ),
        )

    async def DestroyPolicy(
        self,
        request: proto.DestroyPolicyRequest,
        context: grpc.ServicerContext,
    ) -> proto.DestroyPolicyResponse:
        """Destroy a policy instance after in-flight executions finish."""
        try:
            loop = asyncio.get_running_loop()
            return await loop.run_in_executor(self._executor, self._destroy_policy_sync, request)
        except Exception as exc:
            logger.exception("DestroyPolicy failed for instance_id=%s", request.instance_id)
            return proto.DestroyPolicyResponse(success=False, error_message=str(exc))

    def _destroy_policy_sync(self, request: proto.DestroyPolicyRequest) -> proto.DestroyPolicyResponse:
        result = self._store.mark_for_destruction(request.instance_id)
        if result is None:
            logger.warning(
                "DestroyPolicy: instance_id=%s not found (already destroyed?)",
                request.instance_id,
            )
            return proto.DestroyPolicyResponse(success=True)

        instance, should_close = result
        if should_close:
            try:
                instance.close()
            except Exception as exc:
                logger.warning(
                    "DestroyPolicy: close() raised for instance_id=%s: %s",
                    request.instance_id,
                    exc,
                )
                return proto.DestroyPolicyResponse(success=True, error_message=str(exc))

        logger.info("DestroyPolicy OK: instance_id=%s", request.instance_id)
        return proto.DestroyPolicyResponse(success=True)

    async def ExecuteStream(
        self,
        request_iterator: AsyncIterator[proto.StreamRequest],
        context: grpc.ServicerContext,
    ) -> AsyncIterator[proto.StreamResponse]:
        """Handle concurrent hot-path execution over the shared bidirectional stream."""
        response_deque: collections.deque[proto.StreamResponse] = collections.deque()
        response_ready = asyncio.Condition()
        in_flight: set[asyncio.Task] = set()
        reader_done = asyncio.Event()
        concurrency_limit = asyncio.Semaphore(self._max_concurrent)

        async def enqueue_response(response: proto.StreamResponse | None) -> None:
            async with response_ready:
                if response is not None:
                    response_deque.append(response)
                response_ready.notify()

        async def process_request(request: proto.StreamRequest) -> None:
            response: Optional[proto.StreamResponse] = None
            worker_started = False
            keep_timeout_response = False
            try:
                if self._tracker.is_cancelled(request.request_id):
                    return

                timeout_seconds = self._effective_timeout_seconds(request)
                if timeout_seconds <= 0:
                    self._tracker.cancel(request.request_id)
                    response = self._error_response(
                        request_id=request.request_id,
                        error=TimeoutError("execution deadline exceeded before dispatch"),
                        policy_name=request.policy_name,
                        policy_version=request.policy_version,
                        error_type="timeout",
                    )
                    return

                loop = asyncio.get_running_loop()
                future = loop.run_in_executor(self._executor, self._execute_request, request)
                worker_started = True
                response = await asyncio.wait_for(future, timeout=timeout_seconds)

                if self._tracker.is_cancelled(request.request_id):
                    response = None
            except asyncio.TimeoutError:
                already_cancelled = self._tracker.is_cancelled(request.request_id)
                self._tracker.cancel(request.request_id)
                if not already_cancelled:
                    keep_timeout_response = True
                    response = self._error_response(
                        request_id=request.request_id,
                        error=TimeoutError(
                            f"execution timed out after {timeout_seconds:.3f}s"
                        ),
                        policy_name=request.policy_name,
                        policy_version=request.policy_version,
                        error_type="timeout",
                    )
            except Exception as exc:
                logger.exception(
                    "Error executing policy %s:%s",
                    request.policy_name,
                    request.policy_version,
                )
                if not self._tracker.is_cancelled(request.request_id):
                    response = self._error_response(
                        request_id=request.request_id,
                        error=exc,
                        policy_name=request.policy_name,
                        policy_version=request.policy_version,
                        error_type="execution_error",
                    )
            finally:
                if not worker_started:
                    self._tracker.mark_worker_done(request.request_id)
                if (
                    response is not None
                    and self._tracker.is_cancelled(request.request_id)
                    and not keep_timeout_response
                ):
                    response = None
                self._tracker.mark_waiter_done(request.request_id)
                async with response_ready:
                    if response is not None:
                        response_deque.append(response)
                    in_flight.discard(asyncio.current_task())
                    response_ready.notify()
                concurrency_limit.release()

        async def reader() -> None:
            try:
                async for request in request_iterator:
                    payload_name = request.WhichOneof("payload")
                    if payload_name is None:
                        await enqueue_response(
                            self._error_response(
                                request_id=request.request_id,
                                error=ValueError("stream request payload is required"),
                                policy_name=request.policy_name,
                                policy_version=request.policy_version,
                                error_type="protocol_error",
                            )
                        )
                        continue

                    if payload_name == "cancel_execution":
                        cancelled = self._tracker.cancel(request.request_id)
                        logger.debug(
                            "Received cancellation for request_id=%s target_phase=%s cancelled=%s",
                            request.request_id,
                            request.cancel_execution.target_phase,
                            cancelled,
                        )
                        continue

                    try:
                        phase = self._validate_request_phase(request, payload_name)
                        deadline = self._deadline_from_request(request)
                        self._tracker.register(
                            request_id=request.request_id,
                            instance_id=request.instance_id,
                            phase=phase,
                            deadline=deadline,
                        )
                    except Exception as exc:
                        await enqueue_response(
                            self._error_response(
                                request_id=request.request_id,
                                error=exc,
                                policy_name=request.policy_name,
                                policy_version=request.policy_version,
                                error_type="protocol_error",
                            )
                        )
                        continue

                    await concurrency_limit.acquire()
                    try:
                        task = asyncio.create_task(process_request(request))
                        in_flight.add(task)
                    except BaseException:
                        concurrency_limit.release()
                        self._tracker.mark_worker_done(request.request_id)
                        self._tracker.mark_waiter_done(request.request_id)
                        raise
            except asyncio.CancelledError:
                logger.info("ExecuteStream reader cancelled")
            except Exception:
                logger.exception("ExecuteStream reader encountered an error")
            finally:
                async with response_ready:
                    reader_done.set()
                    response_ready.notify()

        reader_task = asyncio.create_task(reader())

        try:
            while True:
                async with response_ready:
                    while not response_deque:
                        if reader_done.is_set() and not in_flight:
                            break
                        await response_ready.wait()
                    batch = list(response_deque)
                    response_deque.clear()

                if not batch and reader_done.is_set() and not in_flight:
                    break

                for response in batch:
                    yield response
        except asyncio.CancelledError:
            logger.debug("ExecuteStream cancelled, cleaning up")
            reader_task.cancel()
            for task in list(in_flight):
                task.cancel()
            all_tasks = [reader_task, *list(in_flight)]
            await asyncio.shield(asyncio.gather(*all_tasks, return_exceptions=True))
            raise
        finally:
            if not reader_task.done():
                reader_task.cancel()
                try:
                    await reader_task
                except asyncio.CancelledError:
                    pass

    def _execute_request(self, request: proto.StreamRequest) -> proto.StreamResponse:
        """Execute a single request in the worker pool."""
        record = self._store.acquire_for_execution(request.instance_id)
        if record is None:
            raise ValueError(
                f"no policy instance for instance_id={request.instance_id} "
                f"(policy={request.policy_name}:{request.policy_version})"
            )

        try:
            params = self._translator.struct_to_dict(request.params)
            shared_ctx = self._translator.to_python_shared_context(request.shared_context)
            execution_ctx = self._build_execution_context(request, record)
            payload_name = request.WhichOneof("payload")
            if payload_name is None:
                raise ValueError("stream request payload is required")

            if payload_name == "request_headers":
                policy = self._require_policy_interface(record.policy, RequestHeaderPolicy, payload_name)
                ctx = self._translator.to_python_request_header_context(
                    request.request_headers.context,
                    shared_ctx,
                )
                action = policy.on_request_headers(execution_ctx, ctx, params)
                return self._response_with_request_header_action(
                    request.request_id,
                    shared_ctx.metadata,
                    action,
                )

            if payload_name == "request_body":
                policy = self._require_policy_interface(record.policy, RequestPolicy, payload_name)
                ctx = self._translator.to_python_request_context(
                    request.request_body.context,
                    shared_ctx,
                )
                action = policy.on_request_body(execution_ctx, ctx, params)
                return self._response_with_request_action(
                    request.request_id,
                    shared_ctx.metadata,
                    action,
                )

            if payload_name == "response_headers":
                policy = self._require_policy_interface(record.policy, ResponseHeaderPolicy, payload_name)
                ctx = self._translator.to_python_response_header_context(
                    request.response_headers.context,
                    shared_ctx,
                )
                action = policy.on_response_headers(execution_ctx, ctx, params)
                return self._response_with_response_header_action(
                    request.request_id,
                    shared_ctx.metadata,
                    action,
                )

            if payload_name == "response_body":
                policy = self._require_policy_interface(record.policy, ResponsePolicy, payload_name)
                ctx = self._translator.to_python_response_context(
                    request.response_body.context,
                    shared_ctx,
                )
                action = policy.on_response_body(execution_ctx, ctx, params)
                return self._response_with_response_action(
                    request.request_id,
                    shared_ctx.metadata,
                    action,
                )

            if payload_name == "needs_more_request_data":
                policy = self._require_policy_interface(record.policy, StreamingRequestPolicy, payload_name)
                decision = policy.needs_more_request_data(
                    bytes(request.needs_more_request_data.accumulated)
                )
                return self._response_with_needs_more_decision(
                    request.request_id,
                    shared_ctx.metadata,
                    decision,
                )

            if payload_name == "request_chunk":
                policy = self._require_policy_interface(record.policy, StreamingRequestPolicy, payload_name)
                ctx = self._translator.to_python_request_stream_context(
                    request.request_chunk.context,
                    shared_ctx,
                )
                chunk = self._translator.to_python_stream_body(request.request_chunk.chunk)
                action = policy.on_request_body_chunk(execution_ctx, ctx, chunk, params)
                return self._response_with_streaming_request_action(
                    request.request_id,
                    shared_ctx.metadata,
                    action,
                )

            if payload_name == "needs_more_response_data":
                policy = self._require_policy_interface(record.policy, StreamingResponsePolicy, payload_name)
                decision = policy.needs_more_response_data(
                    bytes(request.needs_more_response_data.accumulated)
                )
                return self._response_with_needs_more_decision(
                    request.request_id,
                    shared_ctx.metadata,
                    decision,
                )

            if payload_name == "response_chunk":
                policy = self._require_policy_interface(record.policy, StreamingResponsePolicy, payload_name)
                ctx = self._translator.to_python_response_stream_context(
                    request.response_chunk.context,
                    shared_ctx,
                )
                chunk = self._translator.to_python_stream_body(request.response_chunk.chunk)
                action = policy.on_response_body_chunk(execution_ctx, ctx, chunk, params)
                return self._response_with_streaming_response_action(
                    request.request_id,
                    shared_ctx.metadata,
                    action,
                )

            raise ValueError(f"unsupported request payload: {payload_name}")
        finally:
            self._tracker.mark_worker_done(request.request_id)
            if self._store.release_execution(request.instance_id):
                try:
                    record.policy.close()
                    logger.info(
                        "Delayed DestroyPolicy OK: instance_id=%s",
                        request.instance_id,
                    )
                except Exception as exc:
                    logger.warning(
                        "DestroyPolicy: close() raised during delayed destruction for instance_id=%s: %s",
                        request.instance_id,
                        exc,
                    )

    async def HealthCheck(
        self,
        request: proto.HealthCheckRequest,
        context: grpc.ServicerContext,
    ) -> proto.HealthCheckResponse:
        loaded = self._loader.get_loaded_policy_count()
        return proto.HealthCheckResponse(ready=True, loaded_policies=loaded)

    def _build_execution_context(
        self,
        request: proto.StreamRequest,
        record: InstanceRecord,
    ) -> ExecutionContext:
        payload_name = request.WhichOneof("payload")
        if payload_name is None:
            raise ValueError("stream request payload is required")

        expected_phase = self._PAYLOAD_TO_PHASE[payload_name]
        actual_phase = (
            request.execution_metadata.phase
            if request.HasField("execution_metadata")
            else expected_phase
        )
        phase = self._translator.phase_from_proto(actual_phase)

        deadline = self._deadline_from_request(request)
        trace_id = None
        span_id = None
        route_name = record.metadata.route_name
        if request.HasField("execution_metadata"):
            metadata = request.execution_metadata
            if metadata.route_name:
                route_name = metadata.route_name
            if metadata.HasField("trace"):
                trace_id = metadata.trace.trace_id or None
                span_id = metadata.trace.span_id or None

        return ExecutionContext(
            request_id=request.request_id,
            phase=phase,
            deadline=deadline,
            route_name=route_name,
            policy_name=record.policy_name,
            policy_version=record.policy_version,
            trace_id=trace_id,
            span_id=span_id,
            _is_cancelled=lambda: self._tracker.is_cancelled(request.request_id),
        )

    def _validate_request_phase(self, request: proto.StreamRequest, payload_name: str):
        expected_phase = self._PAYLOAD_TO_PHASE[payload_name]
        if not request.HasField("execution_metadata"):
            return self._translator.phase_from_proto(expected_phase)
        if request.execution_metadata.phase != expected_phase:
            raise ValueError(
                "execution phase does not match payload: "
                f"payload={payload_name} metadata_phase={request.execution_metadata.phase}"
            )
        return self._translator.phase_from_proto(expected_phase)

    def _deadline_from_request(self, request: proto.StreamRequest) -> datetime | None:
        if not request.HasField("execution_metadata"):
            return None
        if not request.execution_metadata.HasField("deadline"):
            return None
        return self._translator.timestamp_to_datetime(request.execution_metadata.deadline)

    def _effective_timeout_seconds(self, request: proto.StreamRequest) -> float:
        timeout_seconds = float(self._timeout)
        deadline = self._deadline_from_request(request)
        if deadline is None:
            return timeout_seconds
        remaining = (deadline - datetime.now(timezone.utc)).total_seconds()
        return min(timeout_seconds, remaining)

    @staticmethod
    def _detect_policy_capabilities(instance: Policy) -> PolicyCapabilities:
        return PolicyCapabilities(
            request_headers=isinstance(instance, RequestHeaderPolicy),
            request_body=isinstance(instance, RequestPolicy),
            response_headers=isinstance(instance, ResponseHeaderPolicy),
            response_body=isinstance(instance, ResponsePolicy),
            streaming_request=isinstance(instance, StreamingRequestPolicy),
            streaming_response=isinstance(instance, StreamingResponsePolicy),
        )

    def _validate_policy_contract(
        self,
        instance: Policy,
        mode,
    ) -> PolicyCapabilities:
        capabilities = self._detect_policy_capabilities(instance)
        errors: list[str] = []

        if (mode.request_header_mode == HeaderProcessingMode.PROCESS) != capabilities.request_headers:
            errors.append(
                "request_header_mode=PROCESS requires request-header capability, "
                "and request_header_mode=SKIP requires it to be absent"
            )

        if (mode.response_header_mode == HeaderProcessingMode.PROCESS) != capabilities.response_headers:
            errors.append(
                "response_header_mode=PROCESS requires response-header capability, "
                "and response_header_mode=SKIP requires it to be absent"
            )

        if mode.request_body_mode == BodyProcessingMode.SKIP:
            if capabilities.request_body or capabilities.streaming_request:
                errors.append("request_body_mode=SKIP requires request-body capabilities to be absent")
        elif mode.request_body_mode == BodyProcessingMode.BUFFER:
            if not capabilities.request_body or capabilities.streaming_request:
                errors.append(
                    "request_body_mode=BUFFER requires buffered request-body capability "
                    "and no streaming-request capability"
                )
        elif mode.request_body_mode == BodyProcessingMode.STREAM:
            if not capabilities.request_body or not capabilities.streaming_request:
                errors.append(
                    "request_body_mode=STREAM requires both buffered request-body "
                    "and streaming-request capabilities"
                )

        if mode.response_body_mode == BodyProcessingMode.SKIP:
            if capabilities.response_body or capabilities.streaming_response:
                errors.append("response_body_mode=SKIP requires response-body capabilities to be absent")
        elif mode.response_body_mode == BodyProcessingMode.BUFFER:
            if not capabilities.response_body or capabilities.streaming_response:
                errors.append(
                    "response_body_mode=BUFFER requires buffered response-body capability "
                    "and no streaming-response capability"
                )
        elif mode.response_body_mode == BodyProcessingMode.STREAM:
            if not capabilities.response_body or not capabilities.streaming_response:
                errors.append(
                    "response_body_mode=STREAM requires both buffered response-body "
                    "and streaming-response capabilities"
                )

        if errors:
            raise ValueError("; ".join(errors))

        return capabilities

    @staticmethod
    def _require_policy_interface(instance: Policy, interface, payload_name: str):
        if not isinstance(instance, interface):
            raise ValueError(
                f"policy does not implement required interface for payload {payload_name}: "
                f"{interface.__name__}"
            )
        return instance

    def _response_with_request_header_action(self, request_id: str, metadata: dict, action) -> proto.StreamResponse:
        response = proto.StreamResponse(request_id=request_id)
        response.updated_metadata.CopyFrom(self._translator.dict_to_struct(metadata))
        response.request_header_action.CopyFrom(self._translator.to_proto_request_header_action(action))
        return response

    def _response_with_request_action(self, request_id: str, metadata: dict, action) -> proto.StreamResponse:
        response = proto.StreamResponse(request_id=request_id)
        response.updated_metadata.CopyFrom(self._translator.dict_to_struct(metadata))
        response.request_action.CopyFrom(self._translator.to_proto_request_action(action))
        return response

    def _response_with_response_header_action(self, request_id: str, metadata: dict, action) -> proto.StreamResponse:
        response = proto.StreamResponse(request_id=request_id)
        response.updated_metadata.CopyFrom(self._translator.dict_to_struct(metadata))
        response.response_header_action.CopyFrom(
            self._translator.to_proto_response_header_action(action)
        )
        return response

    def _response_with_response_action(self, request_id: str, metadata: dict, action) -> proto.StreamResponse:
        response = proto.StreamResponse(request_id=request_id)
        response.updated_metadata.CopyFrom(self._translator.dict_to_struct(metadata))
        response.response_action.CopyFrom(self._translator.to_proto_response_action(action))
        return response

    def _response_with_needs_more_decision(
        self,
        request_id: str,
        metadata: dict,
        needs_more: bool,
    ) -> proto.StreamResponse:
        response = proto.StreamResponse(request_id=request_id)
        response.updated_metadata.CopyFrom(self._translator.dict_to_struct(metadata))
        response.needs_more_decision.CopyFrom(
            self._translator.to_proto_needs_more_decision(needs_more)
        )
        return response

    def _response_with_streaming_request_action(
        self,
        request_id: str,
        metadata: dict,
        action,
    ) -> proto.StreamResponse:
        response = proto.StreamResponse(request_id=request_id)
        response.updated_metadata.CopyFrom(self._translator.dict_to_struct(metadata))
        response.streaming_request_action.CopyFrom(
            self._translator.to_proto_streaming_request_action(action)
        )
        return response

    def _response_with_streaming_response_action(
        self,
        request_id: str,
        metadata: dict,
        action,
    ) -> proto.StreamResponse:
        response = proto.StreamResponse(request_id=request_id)
        response.updated_metadata.CopyFrom(self._translator.dict_to_struct(metadata))
        response.streaming_response_action.CopyFrom(
            self._translator.to_proto_streaming_response_action(action)
        )
        return response

    @staticmethod
    def _error_response(
        request_id: str,
        error: Exception,
        policy_name: str,
        policy_version: str,
        error_type: str,
    ) -> proto.StreamResponse:
        response = proto.StreamResponse(request_id=request_id)
        response.error.CopyFrom(
            proto.ExecutionError(
                message=str(error),
                policy_name=policy_name,
                policy_version=policy_version,
                error_type=error_type,
            )
        )
        return response


class PythonExecutorServer:
    """Wrapper around the async gRPC server and executor dependencies."""

    def __init__(
        self,
        socket_path: str,
        worker_count: int = 4,
        max_concurrent: int = 100,
        timeout: int = 30,
    ):
        self.socket_path = socket_path
        self.worker_count = worker_count
        self.max_concurrent = max_concurrent
        self.timeout = timeout
        self.server: Optional[aio.Server] = None
        self._loader = PolicyLoader()
        self._store = PolicyInstanceStore()
        self._tracker = ExecutionTracker()
        self._servicer: Optional[PythonExecutorServicer] = None
        self._shared_executor: Optional[futures.ThreadPoolExecutor] = None

    async def start(self) -> None:
        """Start the gRPC server."""
        logger.info("Starting Python Executor on %s", self.socket_path)

        loaded_count = self._loader.load_policies()
        logger.info("Loaded %s policy factories", loaded_count)

        self._shared_executor = futures.ThreadPoolExecutor(
            max_workers=self.worker_count,
            thread_name_prefix="grpc_policy_worker",
        )
        self.server = aio.server(
            migration_thread_pool=self._shared_executor,
            options=[
                ("grpc.max_send_message_length", 50 * 1024 * 1024),
                ("grpc.max_receive_message_length", 50 * 1024 * 1024),
            ],
        )

        self._servicer = PythonExecutorServicer(
            policy_loader=self._loader,
            instance_store=self._store,
            execution_tracker=self._tracker,
            executor=self._shared_executor,
            max_concurrent=self.max_concurrent,
            timeout=self.timeout,
        )
        proto_grpc.add_PythonExecutorServiceServicer_to_server(self._servicer, self.server)

        if os.path.exists(self.socket_path):
            try:
                socket_stat = os.stat(self.socket_path)
                if stat.S_ISSOCK(socket_stat.st_mode):
                    os.remove(self.socket_path)
                else:
                    raise RuntimeError(
                        f"path exists but is not a socket: {self.socket_path}"
                    )
            except OSError as exc:
                logger.error("Error preparing socket path %s: %s", self.socket_path, exc)
                raise

        if self.server.add_insecure_port(f"unix:{self.socket_path}") == 0:
            error_message = f"failed to bind to UNIX domain socket at {self.socket_path}"
            logger.error(error_message)
            raise RuntimeError(error_message)

        await self.server.start()
        logger.info("Python Executor ready on %s", self.socket_path)

    async def wait_for_termination(self) -> None:
        if self.server:
            await self.server.wait_for_termination()

    async def shutdown(self) -> None:
        """Stop the server and close any remaining policy instances."""
        logger.info("Shutting down Python Executor...")

        if self.server:
            await self.server.stop(grace=5)

        if self._servicer:
            self._servicer.close()

        instances = self._store.clear()
        for instance in instances:
            try:
                instance.close()
            except Exception as exc:
                logger.warning("Error closing policy instance during shutdown: %s", exc)

        if self._shared_executor:
            self._shared_executor.shutdown(wait=True)

        logger.info("Python Executor shutdown complete")
