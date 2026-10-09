import unittest

from executor.translator import Translator
import proto.python_executor_pb2 as proto
from google.protobuf.struct_pb2 import Value
from google.protobuf.wrappers_pb2 import Int32Value
from apip_sdk_core import (
    BodyProcessingMode,
    DownstreamResponseModifications,
    DropHeaderAction,
    FaultDetails,
    ForwardResponseChunk,
    HeaderProcessingMode,
    ImmediateResponse,
    GuardrailDetails,
    JSONRPCError,
    ProcessingMode,
    TerminateResponseChunk,
    SharedContext,
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
        shared_proto.auth_context.typed_properties.CopyFrom(
            Translator.dict_to_struct({"roles": ["admin", "dev"], "dept": "platform"})
        )

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
        # typed_properties must arrive with structure intact: the array claim stays a list.
        self.assertEqual(["admin", "dev"], shared.auth_context.typed_properties["roles"])
        self.assertEqual("platform", shared.auth_context.typed_properties["dept"])
        self.assertEqual(["one", "two"], request_ctx.headers.get("X-Trace"))
        self.assertEqual("public.example.com", request_ctx.vhost)
        self.assertEqual(b"payload", request_ctx.body.content)

    def test_downstream_and_upstream_snapshot_translation(self):
        shared = self.translator.to_python_shared_context(proto.SharedContext())

        request_proto = proto.RequestContext(
            downstream=proto.DownstreamContext(
                request=proto.DownstreamRequest(
                    path="/api/pets",
                    method="POST",
                    authority="gateway.example.com",
                    scheme="https",
                ),
            ),
        )
        request_ctx = self.translator.to_python_request_context(request_proto, shared)
        self.assertIsNotNone(request_ctx.downstream)
        self.assertEqual("/api/pets", request_ctx.downstream.request.path)
        self.assertEqual("POST", request_ctx.downstream.request.method)
        self.assertEqual("gateway.example.com", request_ctx.downstream.request.authority)
        self.assertEqual("https", request_ctx.downstream.request.scheme)

        response_proto = proto.ResponseContext(
            upstream=proto.UpstreamResponseContext(
                response=proto.UpstreamResponse(status_code=503),
            ),
        )
        response_ctx = self.translator.to_python_response_context(response_proto, shared)
        self.assertIsNotNone(response_ctx.upstream)
        self.assertIsNotNone(response_ctx.upstream.response)
        self.assertEqual(503, response_ctx.upstream.response.status_code)

    def test_fault_declaration_survives_serialisation(self):
        """A policy's account of its rejection must reach the gateway.

        The buffered actions no longer DECLARE faultness — the gateway reads the status — so
        what has to survive here is the description, and the distinction between "described
        nothing" and "described an empty error". The streaming action still declares, having
        no status to read, and is checked at the end.
        """
        declared = self.translator.to_proto_request_header_action(
            ImmediateResponse(
                status_code=401,
                fault=FaultDetails(
                    code="900902",
                    type="authentication",
                    direction="Request",
                    message="Valid credentials required",
                    description="detail a renderer may withhold",
                ),
            )
        ).immediate_response
        self.assertEqual("900902", declared.fault.code)
        self.assertEqual("authentication", declared.fault.type)
        self.assertEqual("Request", declared.fault.direction)
        # description crosses the bridge even though no renderer emits it — a fault handler
        # reporting to an audit sink is exactly who needs it.
        self.assertEqual("detail a renderer may withhold", declared.fault.description)

        # A description on a sub-400 response still has to arrive. It no longer routes the
        # response into the fault flow — 404 would, 302 would not — but it is how a
        # deliberate non-failure gets an error-shaped body for a client that expects one.
        described = self.translator.to_proto_request_header_action(
            ImmediateResponse(status_code=302, fault=FaultDetails(code="961000"))
        ).immediate_response
        self.assertTrue(described.HasField("fault"))
        self.assertEqual("961000", described.fault.code)

        # Nothing described: absent must stay distinguishable from an empty description, since
        # the gateway reads a present error as something to render.
        silent = self.translator.to_proto_request_header_action(
            ImmediateResponse(status_code=404)
        ).immediate_response
        self.assertFalse(silent.HasField("fault"))

        # A response modification carrying no description still crosses cleanly. Whether it
        # is a fault is the gateway's read of the status, not anything on the wire here.
        relabelled = self.translator.to_proto_response_action(
            DownstreamResponseModifications(status_code=503)
        ).downstream_response_modifications
        self.assertFalse(relabelled.HasField("fault"))
        self.assertEqual(503, relabelled.status_code.value)

        # The streaming action is the one that still declares, because the status went out
        # with the headers and a mid-stream intervention has none of its own.
        terminated = self.translator.to_proto_streaming_response_action(
            TerminateResponseChunk(
                body=b'data: {"error":"blocked"}\n\n',
                is_fault=True,
                fault=FaultDetails(code="906000", type="guardrail"),
            )
        ).terminate_response_chunk
        self.assertTrue(terminated.is_fault)
        self.assertEqual("906000", terminated.fault.code)

    def test_jsonrpc_block_survives_serialisation(self):
        """The JSON-RPC block is how an MCP policy states what the engine cannot derive.

        The engine maps a status onto -32600/-32603 and has no request id at all. A policy that
        parsed the body knows both, so both have to cross the bridge intact — a flattened code
        turns "invalid params" into the generic "invalid request", and a lost id leaves a client
        with several calls in flight unable to tell which one failed.
        """
        declared = self.translator.to_proto_request_header_action(
            ImmediateResponse(
                status_code=400,
                fault=FaultDetails(
                    message="Invalid MCP request params",
                    jsonrpc=JSONRPCError(code=-32602, id="call-7"),
                ),
            )
        ).immediate_response
        self.assertTrue(declared.fault.HasField("jsonrpc"))
        self.assertEqual(-32602, declared.fault.jsonrpc.code.value)
        self.assertEqual("call-7", declared.fault.jsonrpc.id.string_value)

        # A numeric id must stay a number: JSON-RPC lets the client pick, and one matching on
        # the value it sent would not recognise "7".
        numeric = self.translator.to_proto_request_header_action(
            ImmediateResponse(status_code=400, fault=FaultDetails(jsonrpc=JSONRPCError(id=7)))
        ).immediate_response
        self.assertEqual(7, numeric.fault.jsonrpc.id.number_value)

        # An id with no code leaves the code unset, so the engine still derives it from the
        # status rather than being handed an invalid 0.
        id_only = self.translator.to_proto_request_header_action(
            ImmediateResponse(status_code=429, fault=FaultDetails(jsonrpc=JSONRPCError(id="x")))
        ).immediate_response
        self.assertTrue(id_only.fault.HasField("jsonrpc"))
        self.assertFalse(id_only.fault.jsonrpc.HasField("code"))

        # A code with no id still marks the block present — otherwise the code would vanish.
        code_only = self.translator.to_proto_request_header_action(
            ImmediateResponse(status_code=400, fault=FaultDetails(jsonrpc=JSONRPCError(code=-32700)))
        ).immediate_response
        self.assertTrue(code_only.fault.HasField("jsonrpc"))
        self.assertEqual(-32700, code_only.fault.jsonrpc.code.value)

        # No block stays absent: a non-MCP policy must not acquire an empty one just by
        # describing an error.
        plain = self.translator.to_proto_request_header_action(
            ImmediateResponse(status_code=500, fault=FaultDetails(message="boom"))
        ).immediate_response
        self.assertFalse(plain.fault.HasField("jsonrpc"))

        # Inbound: a fault handler sees what the failing policy supplied.
        received = Translator._to_python_error_response(
            proto.FaultDetails(
                message="Parse error",
                jsonrpc=proto.JSONRPCError(code=Int32Value(value=-32700), id=Value(string_value="abc")),
            )
        )
        self.assertIsNotNone(received.jsonrpc)
        self.assertEqual(-32700, received.jsonrpc.code)
        self.assertEqual("abc", received.jsonrpc.id)

    def test_guardrail_block_survives_serialisation(self):
        """A Python guardrail must be able to return an assessment.

        Before this the proto FaultDetails had no guardrail field at all, so a Python guardrail
        could describe a rejection but never say which guardrail acted or what it found.

        The distinction under test is absent-vs-present-but-empty. Absent means no guardrail was
        involved; present with no assessments means a guardrail acted and the operator did not
        opt into showing the evidence (showAssessment: false). Collapsing them would either hide
        every intervention or disclose every assessment.
        """
        full = self.translator.to_proto_request_header_action(
            ImmediateResponse(
                status_code=422,
                fault=FaultDetails(
                    code="906000",
                    type="guardrail",
                    message="Violation of applied word count constraints detected",
                    guardrail=GuardrailDetails(
                        intervening_guardrail="word-count-guardrail-py",
                        action_reason="too many words",
                        assessments={"assessments": "Expected 10 to 500 words."},
                    ),
                ),
            )
        ).immediate_response
        self.assertTrue(full.fault.HasField("guardrail"))
        self.assertEqual("word-count-guardrail-py", full.fault.guardrail.intervening_guardrail)
        # The default the dataclass supplies, so a guardrail need not restate it.
        self.assertEqual("GUARDRAIL_INTERVENED", full.fault.guardrail.action)
        self.assertEqual("too many words", full.fault.guardrail.action_reason)
        self.assertEqual(
            "Expected 10 to 500 words.",
            full.fault.guardrail.assessments["assessments"],
        )

        # showAssessment: false — block present, evidence withheld.
        gated = self.translator.to_proto_request_header_action(
            ImmediateResponse(
                status_code=422,
                fault=FaultDetails(
                    guardrail=GuardrailDetails(intervening_guardrail="regex-guardrail-py")
                ),
            )
        ).immediate_response
        self.assertTrue(gated.fault.HasField("guardrail"))
        self.assertFalse(gated.fault.guardrail.HasField("assessments"))

        # A non-guardrail policy must not acquire an empty block by describing an error.
        plain = self.translator.to_proto_request_header_action(
            ImmediateResponse(status_code=401, fault=FaultDetails(code="900902"))
        ).immediate_response
        self.assertFalse(plain.fault.HasField("guardrail"))

        # Inbound: a fault handler sees which guardrail acted and why.
        received = Translator._to_python_error_response(
            proto.FaultDetails(
                message="Blocked",
                guardrail=proto.GuardrailDetails(
                    intervening_guardrail="url-guardrail",
                    action="GUARDRAIL_INTERVENED",
                    action_reason="disallowed host",
                ),
            )
        )
        self.assertIsNotNone(received.guardrail)
        self.assertEqual("url-guardrail", received.guardrail.intervening_guardrail)
        self.assertEqual("disallowed host", received.guardrail.action_reason)
        self.assertIsNone(
            received.guardrail.assessments,
            "an unset Struct must stay None, not become an empty dict",
        )

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
        self.assertEqual(proto.HEADER_PROCESSING_MODE_SKIP, mode.response_header_mode)
        self.assertEqual(proto.BODY_PROCESSING_MODE_STREAM, mode.response_body_mode)
        self.assertTrue(self.translator.to_proto_needs_more_decision(True).needs_more)
        self.assertEqual(
            "request_body_chunk",
            self.translator.phase_from_proto(proto.PHASE_REQUEST_BODY_CHUNK).value,
        )

    def test_shared_context_translation_carries_the_resolved_operation(self):
        """The Go engine resolves the operation and its request facts; without this
        the bridge drops both and a Python policy reads empty defaults while the
        values exist upstream."""
        shared_proto = proto.SharedContext(
            api_kind="Agent",
            api_name="WeatherAgent",
            operation_path="/",
            resolved_operation="SendMessage",
        )
        shared_proto.resolution_attributes["a2a.operation"] = "SendMessage"
        shared_proto.resolution_attributes["a2a.transport"] = "JSONRPC"
        shared_proto.resolution_attributes["a2a.context.id"] = "ctx-1"

        shared = Translator.to_python_shared_context(shared_proto)

        self.assertEqual("SendMessage", shared.resolved_operation)
        self.assertEqual(
            {
                "a2a.operation": "SendMessage",
                "a2a.transport": "JSONRPC",
                "a2a.context.id": "ctx-1",
            },
            shared.resolution_attributes,
        )

    def test_shared_context_resolution_attributes_are_a_plain_detached_dict(self):
        """A policy should get an ordinary dict, not the proto message's own map
        container: the container's lifetime is the message's, and its type would
        surprise a policy author expecting a dict."""
        shared_proto = proto.SharedContext(resolved_operation="GetTask")
        shared_proto.resolution_attributes["a2a.task.id"] = "task-1"

        shared = Translator.to_python_shared_context(shared_proto)

        self.assertIs(type(shared.resolution_attributes), dict)
        shared.resolution_attributes["a2a.task.id"] = "tampered"
        self.assertEqual("task-1", shared_proto.resolution_attributes["a2a.task.id"])

    def test_shared_context_direct_route_reports_no_resolution(self):
        """Every API kind released before Agent resolves its chain from the route,
        so it sends neither value and a policy has to read the defaults as "not
        applicable" rather than as a failure."""
        shared = Translator.to_python_shared_context(
            proto.SharedContext(api_kind="RestApi", operation_path="/pets/{id}")
        )

        self.assertEqual("", shared.resolved_operation)
        self.assertEqual({}, shared.resolution_attributes)
    def test_error_context_carries_the_policy_phase(self):
        """policy_phase completes the attribution policy/policy_version starts.

        Asserted on the way IN, because that is the direction a fault handler depends on:
        the gateway names the failing policy and the phase it was in, and a handler reading
        one without the other cannot say what failed.
        """
        ctx = Translator.to_python_error_context(
            proto.FaultContext(
                policy="word-count-guardrail",
                policy_version="v1",
                policy_phase="response_body",
                response_status=446,
            ),
            SharedContext(request_id="r1"),
        )
        self.assertEqual("word-count-guardrail", ctx.policy)
        self.assertEqual("v1", ctx.policy_version)
        self.assertEqual("response_body", ctx.policy_phase)

    def test_error_context_carries_the_source(self):
        """source is what tells a handler whose failure it is looking at.

        Every source is checked rather than one, because the values are configuration surface
        — an execution condition tests them by string — so a spelling that does not survive
        the wire silently stops a deployment's condition from ever matching.
        """
        for source in ("gateway", "backend", "router", "noRoute", "unknown"):
            with self.subTest(source=source):
                ctx = Translator.to_python_error_context(
                    proto.FaultContext(source=source, response_status=503),
                    SharedContext(request_id="r1"),
                )
                self.assertEqual(source, ctx.source)

    def test_error_context_for_a_backend_failure_has_a_source_but_no_description(self):
        """The case source exists for.

        The gateway describes nothing for a backend error — another service's 500 is not the
        gateway's to classify — so source carries the entire signal. A handler seeing neither
        would have a bare 502 it could not tell from an engine failure.
        """
        ctx = Translator.to_python_error_context(
            proto.FaultContext(source="backend", response_status=502),
            SharedContext(request_id="r1"),
        )
        self.assertEqual("backend", ctx.source)
        self.assertIsNone(ctx.fault)

    def test_error_context_without_a_policy_has_no_phase(self):
        """A router failure names no policy, so it must name no phase.

        The gateway enforces this when it records the attribution; this is the wire half of
        the same invariant — an empty field must arrive empty rather than defaulting to some
        phase a handler would then report as fact.
        """
        ctx = Translator.to_python_error_context(
            proto.FaultContext(response_status=503),
            SharedContext(request_id="r2"),
        )
        self.assertEqual("", ctx.policy)
        self.assertEqual("", ctx.policy_phase)
