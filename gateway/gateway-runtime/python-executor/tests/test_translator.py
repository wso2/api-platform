import unittest

from executor.translator import Translator
import proto.python_executor_pb2 as proto
from wso2_gateway_policy_sdk import (
    BodyProcessingMode,
    DownstreamResponseModifications,
    DropHeaderAction,
    ForwardResponseChunk,
    HeaderProcessingMode,
    ImmediateResponse,
    ProcessingMode,
    TerminateResponseChunk,
    UpstreamRequestModifications,
)


class TranslatorTest(unittest.TestCase):
    def setUp(self):
        self.translator = Translator()

    def test_shared_and_request_context_translation_preserves_auth_headers_and_vhost(self):
        shared_proto = proto.SharedContext(
            project_id="project-1",
            request_id="request-1",
            api_id="api-1",
            api_name="PetStore",
            api_version="v1",
            api_kind="RestApi",
            api_context="/petstore",
            operation_path="/pets/{id}",
            auth_context=proto.AuthContext(
                authenticated=True,
                authorized=True,
                auth_type="jwt",
                subject="alice",
                audience=["client-a"],
                credential_id="cred-1",
                previous=proto.AuthContext(
                    authenticated=True,
                    auth_type="apikey",
                    subject="legacy-client",
                ),
            ),
        )
        shared_proto.metadata.CopyFrom(Translator.dict_to_struct({"flag": True}))
        shared_proto.auth_context.scopes["read:pets"] = True
        shared_proto.auth_context.properties["tenant"] = "demo"

        shared = self.translator.to_python_shared_context(shared_proto)

        request_proto = proto.RequestContext(
            path="/petstore/v1/pets/123",
            method="POST",
            authority="gateway.example.com",
            scheme="https",
            vhost="public.example.com",
            body=proto.Body(content=b"payload", end_of_stream=True, present=True),
        )
        request_proto.headers.values["x-trace"].values.extend(["one", "two"])

        request_ctx = self.translator.to_python_request_context(request_proto, shared)

        self.assertTrue(shared.auth_context.authenticated)
        self.assertEqual("apikey", shared.auth_context.previous.auth_type)
        self.assertEqual(["one", "two"], request_ctx.headers.get("X-Trace"))
        self.assertEqual("public.example.com", request_ctx.vhost)
        self.assertEqual(b"payload", request_ctx.body.content)

    def test_action_translation_preserves_current_fields(self):
        request_action = UpstreamRequestModifications(
            body=b"rewritten",
            headers_to_set={"x-added": "yes"},
            headers_to_remove=["x-remove"],
            upstream_name="blue",
            path="/rewritten",
            host="backend.internal",
            method="PATCH",
            query_parameters_to_add={"foo": ["bar", "baz"]},
            query_parameters_to_remove=["drop"],
            analytics_metadata={"tokens": 12},
            dynamic_metadata={"ns": {"value": "ok"}},
            analytics_header_filter=DropHeaderAction(
                action="deny",
                headers=["authorization"],
            ),
        )
        request_payload = self.translator.to_proto_request_action(request_action)

        self.assertEqual(b"rewritten", request_payload.upstream_request_modifications.body.value)
        self.assertEqual("blue", request_payload.upstream_request_modifications.upstream_name.value)
        self.assertEqual(
            ["bar", "baz"],
            list(
                request_payload.upstream_request_modifications.query_parameters_to_add[
                    "foo"
                ].values
            ),
        )
        self.assertEqual(
            proto.DROP_HEADER_ACTION_TYPE_DENY,
            request_payload.upstream_request_modifications.analytics_header_filter.action,
        )

        response_action = DownstreamResponseModifications(
            body=b"final",
            status_code=202,
            headers_to_set={"x-cache": "hit"},
            headers_to_remove=["x-remove"],
            analytics_metadata={"cached": True},
            dynamic_metadata={"resp": {"source": "policy"}},
            analytics_header_filter=DropHeaderAction(
                action="allow",
                headers=["x-keep"],
            ),
        )
        response_payload = self.translator.to_proto_response_action(response_action)

        self.assertEqual(202, response_payload.downstream_response_modifications.status_code.value)
        self.assertEqual(
            "hit",
            response_payload.downstream_response_modifications.headers_to_set["x-cache"],
        )
        self.assertEqual(
            proto.DROP_HEADER_ACTION_TYPE_ALLOW,
            response_payload.downstream_response_modifications.analytics_header_filter.action,
        )

        streaming_payload = self.translator.to_proto_streaming_response_action(
            TerminateResponseChunk(
                body=b"done",
                analytics_metadata={"final": True},
                dynamic_metadata={"stream": {"end": True}},
            )
        )
        self.assertEqual(b"done", streaming_payload.terminate_response_chunk.body.value)
        self.assertTrue(
            streaming_payload.terminate_response_chunk.dynamic_metadata["stream"].fields["end"].bool_value
        )

        immediate_payload = self.translator.to_proto_request_header_action(
            ImmediateResponse(
                status_code=418,
                headers={"content-type": "text/plain"},
                body=b"teapot",
            )
        )
        self.assertEqual(418, immediate_payload.immediate_response.status_code)
        self.assertEqual(
            "text/plain",
            immediate_payload.immediate_response.headers["content-type"],
        )

        passthrough_payload = self.translator.to_proto_streaming_response_action(
            ForwardResponseChunk(body=b"chunk")
        )
        self.assertEqual(b"chunk", passthrough_payload.forward_response_chunk.body.value)

    def test_phase_and_needs_more_translation(self):
        mode = self.translator.to_proto_processing_mode(
            ProcessingMode(
                request_header_mode=HeaderProcessingMode.PROCESS,
                request_body_mode=BodyProcessingMode.BUFFER,
                response_header_mode=HeaderProcessingMode.SKIP,
                response_body_mode=BodyProcessingMode.STREAM,
            )
        )
        self.assertEqual(proto.HEADER_PROCESSING_MODE_PROCESS, mode.request_header_mode)
        self.assertEqual(proto.BODY_PROCESSING_MODE_BUFFER, mode.request_body_mode)
        self.assertEqual(proto.BODY_PROCESSING_MODE_STREAM, mode.response_body_mode)
        self.assertTrue(self.translator.to_proto_needs_more_decision(True).needs_more)
        self.assertEqual(
            "request_body_chunk",
            self.translator.phase_from_proto(proto.PHASE_REQUEST_BODY_CHUNK).value,
        )
