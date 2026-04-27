import asyncio
import time
import unittest
from concurrent import futures

from executor.execution_tracker import ExecutionTracker
from executor.instance_store import PolicyInstanceStore
from executor.server import PythonExecutorServicer
from executor.translator import Translator
import proto.python_executor_pb2 as proto
from wso2_gateway_policy_sdk import (
    BodyProcessingMode,
    DownstreamResponseHeaderModifications,
    DownstreamResponseModifications,
    ExecutionContext,
    ForwardRequestChunk,
    HeaderProcessingMode,
    ImmediateResponse,
    Policy,
    PolicyMetadata,
    ProcessingMode,
    RequestHeaderPolicy,
    RequestPolicy,
    StreamingRequestPolicy,
    StreamingResponsePolicy,
    TerminateResponseChunk,
    UpstreamRequestHeaderModifications,
    UpstreamRequestModifications,
)


class LoaderStub:
    def __init__(self, factory):
        self._factory = factory

    def get_factory(self, _policy_name, _policy_version):
        return self._factory

    def get_loaded_policy_count(self):
        return 1


class HeaderFixture(RequestHeaderPolicy):
    def mode(self) -> ProcessingMode:
        return ProcessingMode(request_header_mode=HeaderProcessingMode.PROCESS)

    def on_request_headers(self, execution_ctx, ctx, params):
        ctx.shared.metadata["phase"] = execution_ctx.phase.value
        ctx.shared.metadata["route"] = execution_ctx.route_name
        ctx.shared.metadata["vhost"] = ctx.vhost
        ctx.shared.metadata["trace"] = ctx.headers.get("x-trace")
        ctx.shared.metadata["param_enabled"] = params["enabled"]
        return UpstreamRequestHeaderModifications(headers_to_set={"x-added": "true"})


class StreamingFixture(StreamingRequestPolicy, StreamingResponsePolicy):
    def mode(self) -> ProcessingMode:
        return ProcessingMode(
            request_body_mode=BodyProcessingMode.STREAM,
            response_body_mode=BodyProcessingMode.STREAM,
        )

    def on_request_body(self, execution_ctx, ctx, params):
        return UpstreamRequestModifications()

    def needs_more_request_data(self, accumulated: bytes) -> bool:
        return len(accumulated) < 3

    def on_request_body_chunk(self, execution_ctx, ctx, chunk, params):
        ctx.shared.metadata["request_chunk_index"] = chunk.index
        return ForwardRequestChunk(body=chunk.chunk.upper())

    def on_response_body(self, execution_ctx, ctx, params):
        return DownstreamResponseModifications()

    def needs_more_response_data(self, accumulated: bytes) -> bool:
        return accumulated != b"done"

    def on_response_body_chunk(self, execution_ctx, ctx, chunk, params):
        ctx.shared.metadata["response_chunk_index"] = chunk.index
        return TerminateResponseChunk(body=chunk.chunk + b"!")


class SlowRequestPolicy(RequestPolicy):
    def __init__(self, delay_seconds: float):
        self._delay_seconds = delay_seconds

    def mode(self) -> ProcessingMode:
        return ProcessingMode(request_body_mode=BodyProcessingMode.BUFFER)

    def on_request_body(self, execution_ctx, ctx, params):
        time.sleep(self._delay_seconds)
        return ImmediateResponse(status_code=200, body=b"done")


class InvalidStreamingModePolicy(Policy):
    def mode(self) -> ProcessingMode:
        return ProcessingMode(request_body_mode=BodyProcessingMode.STREAM)


class PythonExecutorServicerTest(unittest.IsolatedAsyncioTestCase):
    def _make_servicer(self, factory, timeout=30):
        executor = futures.ThreadPoolExecutor(max_workers=4)
        self.addCleanup(executor.shutdown, True)
        store = PolicyInstanceStore()
        tracker = ExecutionTracker()
        servicer = PythonExecutorServicer(
            policy_loader=LoaderStub(factory),
            instance_store=store,
            execution_tracker=tracker,
            executor=executor,
            timeout=timeout,
        )
        return servicer, store, tracker

    @staticmethod
    def _shared_context() -> proto.SharedContext:
        shared = proto.SharedContext(
            request_id="shared-1",
            api_id="api-1",
            api_name="PetStore",
            api_version="v1",
            api_kind="RestApi",
            api_context="/petstore",
            operation_path="/pets/{id}",
        )
        shared.metadata.CopyFrom(Translator.dict_to_struct({"seed": "value"}))
        return shared

    async def test_init_policy_returns_validated_capabilities(self):
        servicer, store, _ = self._make_servicer(lambda metadata, params: StreamingFixture())

        response = await servicer.InitPolicy(
            proto.InitPolicyRequest(
                policy_name="demo-policy",
                policy_version="v1.0.0",
                policy_metadata=proto.PolicyMetadata(route_name="route-a"),
            ),
            None,
        )

        self.assertTrue(response.success)
        self.assertTrue(response.capabilities.request_body)
        self.assertTrue(response.capabilities.response_body)
        self.assertTrue(response.capabilities.streaming_request)
        self.assertTrue(response.capabilities.streaming_response)
        self.assertEqual(1, store.count())

    def test_validate_policy_contract_rejects_invalid_streaming_mode(self):
        servicer, _, _ = self._make_servicer(lambda metadata, params: InvalidStreamingModePolicy())

        with self.assertRaisesRegex(ValueError, "request_body_mode=STREAM"):
            policy = InvalidStreamingModePolicy()
            servicer._validate_policy_contract(policy, policy.mode())

    def test_execute_request_dispatches_request_headers(self):
        servicer, store, _ = self._make_servicer(lambda metadata, params: HeaderFixture())
        store.put(
            instance_id="instance-1",
            instance=HeaderFixture(),
            policy_name="demo-policy",
            policy_version="v1.0.0",
            metadata=PolicyMetadata(route_name="route-a"),
        )

        request = proto.StreamRequest(
            request_id="req-1",
            instance_id="instance-1",
            policy_name="demo-policy",
            policy_version="v1.0.0",
            shared_context=self._shared_context(),
            params=Translator.dict_to_struct({"enabled": True}),
            execution_metadata=proto.ExecutionMetadata(
                phase=proto.PHASE_REQUEST_HEADERS,
                route_name="route-a",
            ),
            request_headers=proto.RequestHeadersPayload(
                context=proto.RequestHeaderContext(
                    path="/petstore/v1/pets/123",
                    method="GET",
                    authority="gateway.example.com",
                    scheme="https",
                    vhost="public.example.com",
                )
            ),
        )
        request.request_headers.context.headers.values["x-trace"].values.extend(["one", "two"])

        response = servicer._execute_request(request)
        metadata = Translator.struct_to_dict(response.updated_metadata)

        self.assertEqual(
            "true",
            response.request_header_action.upstream_request_header_modifications.headers_to_set[
                "x-added"
            ],
        )
        self.assertEqual("request_headers", metadata["phase"])
        self.assertEqual("route-a", metadata["route"])
        self.assertEqual("public.example.com", metadata["vhost"])
        self.assertEqual(["one", "two"], metadata["trace"])
        self.assertTrue(metadata["param_enabled"])

    def test_execute_request_dispatches_needs_more_and_streaming_response_chunk(self):
        servicer, store, _ = self._make_servicer(lambda metadata, params: StreamingFixture())
        store.put(
            instance_id="instance-2",
            instance=StreamingFixture(),
            policy_name="demo-policy",
            policy_version="v1.0.0",
            metadata=PolicyMetadata(route_name="route-stream"),
        )

        decision_response = servicer._execute_request(
            proto.StreamRequest(
                request_id="req-2",
                instance_id="instance-2",
                policy_name="demo-policy",
                policy_version="v1.0.0",
                shared_context=self._shared_context(),
                execution_metadata=proto.ExecutionMetadata(
                    phase=proto.PHASE_NEEDS_MORE_REQUEST_DATA,
                    route_name="route-stream",
                ),
                needs_more_request_data=proto.NeedsMoreRequestDataPayload(accumulated=b"ab"),
            )
        )
        self.assertTrue(decision_response.needs_more_decision.needs_more)

        chunk_response = servicer._execute_request(
            proto.StreamRequest(
                request_id="req-3",
                instance_id="instance-2",
                policy_name="demo-policy",
                policy_version="v1.0.0",
                shared_context=self._shared_context(),
                execution_metadata=proto.ExecutionMetadata(
                    phase=proto.PHASE_RESPONSE_BODY_CHUNK,
                    route_name="route-stream",
                ),
                response_chunk=proto.ResponseChunkPayload(
                    context=proto.ResponseStreamContext(
                        request_path="/petstore/v1/pets/123",
                        request_method="GET",
                        response_status=200,
                    ),
                    chunk=proto.StreamBody(chunk=b"tail", end_of_stream=True, index=4),
                ),
            )
        )
        metadata = Translator.struct_to_dict(chunk_response.updated_metadata)

        self.assertEqual(
            b"tail!",
            chunk_response.streaming_response_action.terminate_response_chunk.body.value,
        )
        self.assertEqual(4, metadata["response_chunk_index"])

    async def test_execute_stream_returns_timeout_error(self):
        servicer, store, _ = self._make_servicer(
            lambda metadata, params: SlowRequestPolicy(0.05),
            timeout=0.01,
        )
        store.put(
            instance_id="instance-3",
            instance=SlowRequestPolicy(0.05),
            policy_name="demo-policy",
            policy_version="v1.0.0",
            metadata=PolicyMetadata(route_name="route-timeout"),
        )

        request = proto.StreamRequest(
            request_id="req-timeout",
            instance_id="instance-3",
            policy_name="demo-policy",
            policy_version="v1.0.0",
            shared_context=self._shared_context(),
            execution_metadata=proto.ExecutionMetadata(
                phase=proto.PHASE_REQUEST_BODY,
                route_name="route-timeout",
            ),
            request_body=proto.RequestBodyPayload(
                context=proto.RequestContext(
                    path="/petstore/v1/pets/123",
                    method="POST",
                    vhost="timeout.example.com",
                    body=proto.Body(content=b"payload", end_of_stream=True, present=True),
                )
            ),
        )

        async def iterator():
            yield request

        responses = [response async for response in servicer.ExecuteStream(iterator(), None)]

        self.assertEqual(1, len(responses))
        self.assertEqual("timeout", responses[0].error.error_type)
        self.assertIn("timed out", responses[0].error.message)

    async def test_execute_stream_cancellation_drops_late_response(self):
        servicer, store, tracker = self._make_servicer(
            lambda metadata, params: SlowRequestPolicy(0.05),
            timeout=1,
        )
        store.put(
            instance_id="instance-4",
            instance=SlowRequestPolicy(0.05),
            policy_name="demo-policy",
            policy_version="v1.0.0",
            metadata=PolicyMetadata(route_name="route-cancel"),
        )

        request = proto.StreamRequest(
            request_id="req-cancel",
            instance_id="instance-4",
            policy_name="demo-policy",
            policy_version="v1.0.0",
            shared_context=self._shared_context(),
            execution_metadata=proto.ExecutionMetadata(
                phase=proto.PHASE_REQUEST_BODY,
                route_name="route-cancel",
            ),
            request_body=proto.RequestBodyPayload(
                context=proto.RequestContext(
                    path="/petstore/v1/pets/123",
                    method="POST",
                    body=proto.Body(content=b"payload", end_of_stream=True, present=True),
                )
            ),
        )
        cancel = proto.StreamRequest(
            request_id="req-cancel",
            instance_id="instance-4",
            policy_name="demo-policy",
            policy_version="v1.0.0",
            execution_metadata=proto.ExecutionMetadata(phase=proto.PHASE_CANCEL),
            cancel_execution=proto.CancelExecutionPayload(
                target_phase=proto.PHASE_REQUEST_BODY,
                reason="client cancelled",
            ),
        )

        async def iterator():
            yield request
            await asyncio.sleep(0.01)
            yield cancel

        responses = [response async for response in servicer.ExecuteStream(iterator(), None)]

        self.assertEqual([], responses)
        self.assertEqual({}, tracker._executions)

    async def test_execute_stream_missing_instance_cleans_tracker(self):
        servicer, _, tracker = self._make_servicer(lambda metadata, params: HeaderFixture())

        request = proto.StreamRequest(
            request_id="req-missing-instance",
            instance_id="missing-instance",
            policy_name="demo-policy",
            policy_version="v1.0.0",
            shared_context=self._shared_context(),
            execution_metadata=proto.ExecutionMetadata(
                phase=proto.PHASE_REQUEST_HEADERS,
                route_name="route-a",
            ),
            request_headers=proto.RequestHeadersPayload(
                context=proto.RequestHeaderContext(
                    path="/petstore/v1/pets/123",
                    method="GET",
                    authority="gateway.example.com",
                    scheme="https",
                    vhost="public.example.com",
                )
            ),
        )

        async def iterator():
            yield request

        responses = [response async for response in servicer.ExecuteStream(iterator(), None)]

        self.assertEqual(1, len(responses))
        self.assertEqual("execution_error", responses[0].error.error_type)
        self.assertIn("no policy instance", responses[0].error.message)
        self.assertEqual({}, tracker._executions)
