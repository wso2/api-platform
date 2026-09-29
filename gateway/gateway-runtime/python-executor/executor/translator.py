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

"""Proto <-> Python SDK translation helpers."""

from __future__ import annotations

from datetime import datetime, timezone
from typing import Any, Mapping

from google.protobuf.struct_pb2 import Struct, Value
from google.protobuf.wrappers_pb2 import BytesValue, Int32Value, StringValue

import proto.python_executor_pb2 as proto
from apip_sdk_core import (
    AuthContext,
    Body,
    DownstreamContext,
    DownstreamRequest,
    DownstreamResponseHeaderModifications,
    DownstreamResponseModifications,
    DropHeaderAction,
    FaultContext,
    FaultResponse,
    FaultDetails,
    ExecutionPhase,
    ForwardRequestChunk,
    ForwardResponseChunk,
    GuardrailDetails,
    Headers,
    ImmediateResponse,
    JSONRPCError,
    PolicyMetadata,
    ProcessingMode,
    RequestAction,
    RequestContext,
    RequestHeaderAction,
    RequestHeaderContext,
    RequestStreamContext,
    ResponseAction,
    ResponseContext,
    ResponseHeaderAction,
    ResponseHeaderContext,
    ResponseStreamContext,
    SharedContext,
    StreamBody,
    StreamingRequestAction,
    StreamingResponseAction,
    TerminateResponseChunk,
    UpstreamRequestContext,
    UpstreamRequestHeaderModifications,
    UpstreamRequestModifications,
    UpstreamResponse,
    UpstreamResponseContext,
)


class Translator:
    """Translates between protobuf messages and Python SDK types."""

    _PHASE_MAP = {
        proto.PHASE_REQUEST_HEADERS: ExecutionPhase.REQUEST_HEADERS,
        proto.PHASE_REQUEST_BODY: ExecutionPhase.REQUEST_BODY,
        proto.PHASE_RESPONSE_HEADERS: ExecutionPhase.RESPONSE_HEADERS,
        proto.PHASE_RESPONSE_BODY: ExecutionPhase.RESPONSE_BODY,
        proto.PHASE_NEEDS_MORE_REQUEST_DATA: ExecutionPhase.NEEDS_MORE_REQUEST_DATA,
        proto.PHASE_REQUEST_BODY_CHUNK: ExecutionPhase.REQUEST_BODY_CHUNK,
        proto.PHASE_NEEDS_MORE_RESPONSE_DATA: ExecutionPhase.NEEDS_MORE_RESPONSE_DATA,
        proto.PHASE_RESPONSE_BODY_CHUNK: ExecutionPhase.RESPONSE_BODY_CHUNK,
        proto.PHASE_CANCEL: ExecutionPhase.CANCEL,
        proto.PHASE_FAULT: ExecutionPhase.FAULT,
    }

    _DROP_HEADER_ACTION_TO_PROTO = {
        "allow": proto.DROP_HEADER_ACTION_TYPE_ALLOW,
        "deny": proto.DROP_HEADER_ACTION_TYPE_DENY,
    }

    @staticmethod
    def phase_from_proto(value: int) -> ExecutionPhase:
        try:
            return Translator._PHASE_MAP[value]
        except KeyError as exc:
            raise ValueError(f"unsupported execution phase: {value}") from exc

    @staticmethod
    def timestamp_to_datetime(timestamp) -> datetime | None:
        if timestamp is None:
            return None
        if getattr(timestamp, "seconds", 0) == 0 and getattr(timestamp, "nanos", 0) == 0:
            return None
        return timestamp.ToDatetime(tzinfo=timezone.utc)

    @staticmethod
    def to_python_policy_metadata(proto_meta: proto.PolicyMetadata) -> PolicyMetadata:
        return PolicyMetadata(
            route_name=proto_meta.route_name,
            api_id=proto_meta.api_id,
            api_name=proto_meta.api_name,
            api_version=proto_meta.api_version,
            attached_to=proto_meta.attached_to,
        )

    @staticmethod
    def to_proto_processing_mode(mode: ProcessingMode) -> proto.ProcessingMode:
        return proto.ProcessingMode(
            request_header_mode=getattr(
                proto,
                f"HEADER_PROCESSING_MODE_{mode.request_header_mode.value}",
            ),
            request_body_mode=getattr(
                proto,
                f"BODY_PROCESSING_MODE_{mode.request_body_mode.value}",
            ),
            response_header_mode=getattr(
                proto,
                f"HEADER_PROCESSING_MODE_{mode.response_header_mode.value}",
            ),
            response_body_mode=getattr(
                proto,
                f"BODY_PROCESSING_MODE_{mode.response_body_mode.value}",
            ),
        )

    @staticmethod
    def to_python_shared_context(proto_ctx: proto.SharedContext) -> SharedContext:
        auth_context = None
        if proto_ctx.HasField("auth_context"):
            auth_context = Translator._to_python_auth_context(proto_ctx.auth_context)

        return SharedContext(
            project_id=proto_ctx.project_id,
            request_id=proto_ctx.request_id,
            metadata=Translator.struct_to_dict(proto_ctx.metadata),
            api_id=proto_ctx.api_id,
            api_name=proto_ctx.api_name,
            api_version=proto_ctx.api_version,
            api_kind=proto_ctx.api_kind,
            api_context=proto_ctx.api_context,
            operation_path=proto_ctx.operation_path,
            auth_context=auth_context,
            resolved_operation=proto_ctx.resolved_operation,
            # dict() rather than the ScalarMapContainer itself: a policy gets a
            # plain dict it can treat like any other, and one that is already its
            # own — the container is owned by the proto message this request
            # deserialized, and handing it out would tie the policy's view to the
            # lifetime of that message.
            resolution_attributes=dict(proto_ctx.resolution_attributes),
        )

    @staticmethod
    def to_python_request_header_context(
        proto_ctx: proto.RequestHeaderContext,
        shared: SharedContext,
    ) -> RequestHeaderContext:
        return RequestHeaderContext(
            shared=shared,
            headers=Translator._to_python_headers(proto_ctx.headers),
            path=proto_ctx.path,
            method=proto_ctx.method,
            authority=proto_ctx.authority,
            scheme=proto_ctx.scheme,
            vhost=proto_ctx.vhost,
            downstream=Translator._to_python_downstream(proto_ctx),
            upstream=Translator._to_python_request_upstream(proto_ctx),
        )

    @staticmethod
    def to_python_request_context(
        proto_ctx: proto.RequestContext,
        shared: SharedContext,
    ) -> RequestContext:
        body = Translator._to_python_body(proto_ctx.body) if proto_ctx.HasField("body") else None
        return RequestContext(
            shared=shared,
            headers=Translator._to_python_headers(proto_ctx.headers),
            body=body,
            path=proto_ctx.path,
            method=proto_ctx.method,
            authority=proto_ctx.authority,
            scheme=proto_ctx.scheme,
            vhost=proto_ctx.vhost,
            downstream=Translator._to_python_downstream(proto_ctx),
            upstream=Translator._to_python_request_upstream(proto_ctx),
        )

    @staticmethod
    def to_python_response_header_context(
        proto_ctx: proto.ResponseHeaderContext,
        shared: SharedContext,
    ) -> ResponseHeaderContext:
        request_body = (
            Translator._to_python_body(proto_ctx.request_body)
            if proto_ctx.HasField("request_body")
            else None
        )
        return ResponseHeaderContext(
            shared=shared,
            request_headers=Translator._to_python_headers(proto_ctx.request_headers),
            request_body=request_body,
            request_path=proto_ctx.request_path,
            request_method=proto_ctx.request_method,
            response_headers=Translator._to_python_headers(proto_ctx.response_headers),
            response_status=proto_ctx.response_status,
            downstream=Translator._to_python_downstream(proto_ctx),
            upstream=Translator._to_python_upstream(proto_ctx),
        )

    @staticmethod
    def to_python_response_context(
        proto_ctx: proto.ResponseContext,
        shared: SharedContext,
    ) -> ResponseContext:
        request_body = (
            Translator._to_python_body(proto_ctx.request_body)
            if proto_ctx.HasField("request_body")
            else None
        )
        response_body = (
            Translator._to_python_body(proto_ctx.response_body)
            if proto_ctx.HasField("response_body")
            else None
        )
        return ResponseContext(
            shared=shared,
            request_headers=Translator._to_python_headers(proto_ctx.request_headers),
            request_body=request_body,
            request_path=proto_ctx.request_path,
            request_method=proto_ctx.request_method,
            response_headers=Translator._to_python_headers(proto_ctx.response_headers),
            response_body=response_body,
            response_status=proto_ctx.response_status,
            downstream=Translator._to_python_downstream(proto_ctx),
            upstream=Translator._to_python_upstream(proto_ctx),
        )

    @staticmethod
    def to_python_error_context(
        proto_ctx: proto.FaultContext,
        shared: SharedContext,
    ) -> FaultContext:
        """Build the context a fault handler receives.

        Field-for-field now that both sides are flat: the proto message carries the response
        fields directly, and so does the SDK type, so there is no nested view to unwrap and
        no chance of reading a field off the wrong level.
        """
        request_body = (
            Translator._to_python_body(proto_ctx.request_body)
            if proto_ctx.HasField("request_body")
            else None
        )
        response_body = (
            Translator._to_python_body(proto_ctx.response_body)
            if proto_ctx.HasField("response_body")
            else None
        )
        return FaultContext(
            shared=shared,
            request_headers=Translator._to_python_headers(proto_ctx.request_headers),
            request_body=request_body,
            request_path=proto_ctx.request_path,
            request_method=proto_ctx.request_method,
            request_authority=proto_ctx.request_authority,
            request_scheme=proto_ctx.request_scheme,
            request_vhost=proto_ctx.request_vhost,
            response_headers=Translator._to_python_headers(proto_ctx.response_headers),
            response_body=response_body,
            response_status=proto_ctx.response_status,
            downstream=Translator._to_python_downstream(proto_ctx),
            upstream=Translator._to_python_upstream(proto_ctx),
            original_status=proto_ctx.original_status,
            policy=proto_ctx.policy,
            policy_version=proto_ctx.policy_version,
            policy_phase=proto_ctx.policy_phase,
            response_committed=proto_ctx.response_committed,
            route_key=proto_ctx.route_key,
            source=proto_ctx.source,
            # Absent stays None: a handler has to be able to tell "nothing described this
            # failure" from "described an empty one", since only the former means falling
            # back to the status.
            fault=(
                Translator._to_python_error_response(proto_ctx.fault)
                if proto_ctx.HasField("fault")
                else None
            ),
        )

    @staticmethod
    def _to_python_error_response(proto_error: proto.FaultDetails) -> FaultDetails:
        return FaultDetails(
            code=proto_error.code,
            type=proto_error.type,
            direction=proto_error.direction,
            message=proto_error.message,
            description=proto_error.description,
            jsonrpc=(
                Translator._to_python_jsonrpc_error(proto_error.jsonrpc)
                if proto_error.HasField("jsonrpc")
                else None
            ),
            guardrail=(
                Translator._to_python_guardrail_error(proto_error.guardrail)
                if proto_error.HasField("guardrail")
                else None
            ),
        )

    @staticmethod
    def _to_python_guardrail_error(proto_guardrail: proto.GuardrailDetails) -> GuardrailDetails:
        """Convert the guardrail block a failing policy supplied.

        An absent assessments Struct becomes None rather than an empty dict: "the operator did
        not opt into showing the assessment" and "the guardrail reported an empty assessment"
        are different statements, and only the former is what showAssessment: false means.
        """
        return GuardrailDetails(
            intervening_guardrail=proto_guardrail.intervening_guardrail,
            action=proto_guardrail.action,
            action_reason=proto_guardrail.action_reason,
            assessments=(
                Translator.struct_to_dict(proto_guardrail.assessments)
                if proto_guardrail.HasField("assessments")
                else None
            ),
        )

    @staticmethod
    def _to_python_jsonrpc_error(proto_jsonrpc: proto.JSONRPCError) -> JSONRPCError:
        """Convert the JSON-RPC block a failing policy supplied.

        An unset code stays None rather than becoming 0. The distinction is load-bearing: a
        policy that filled in an id but no code is asking for the id to be echoed and the code
        to be derived from the status, and 0 is not a valid JSON-RPC code anyway.
        """
        return JSONRPCError(
            code=proto_jsonrpc.code.value if proto_jsonrpc.HasField("code") else None,
            id=(
                Translator._proto_value_to_python(proto_jsonrpc.id)
                if proto_jsonrpc.HasField("id")
                else None
            ),
        )

    @staticmethod
    def to_python_request_stream_context(
        proto_ctx: proto.RequestStreamContext,
        shared: SharedContext,
    ) -> RequestStreamContext:
        return RequestStreamContext(
            shared=shared,
            headers=Translator._to_python_headers(proto_ctx.headers),
            path=proto_ctx.path,
            method=proto_ctx.method,
            authority=proto_ctx.authority,
            scheme=proto_ctx.scheme,
            vhost=proto_ctx.vhost,
            downstream=Translator._to_python_downstream(proto_ctx),
            upstream=Translator._to_python_request_upstream(proto_ctx),
        )

    @staticmethod
    def to_python_response_stream_context(
        proto_ctx: proto.ResponseStreamContext,
        shared: SharedContext,
    ) -> ResponseStreamContext:
        request_body = (
            Translator._to_python_body(proto_ctx.request_body)
            if proto_ctx.HasField("request_body")
            else None
        )
        return ResponseStreamContext(
            shared=shared,
            request_headers=Translator._to_python_headers(proto_ctx.request_headers),
            request_body=request_body,
            request_path=proto_ctx.request_path,
            request_method=proto_ctx.request_method,
            response_headers=Translator._to_python_headers(proto_ctx.response_headers),
            response_status=proto_ctx.response_status,
            downstream=Translator._to_python_downstream(proto_ctx),
            upstream=Translator._to_python_upstream(proto_ctx),
        )

    @staticmethod
    def to_python_stream_body(proto_body: proto.StreamBody) -> StreamBody:
        return StreamBody(
            chunk=bytes(proto_body.chunk),
            end_of_stream=proto_body.end_of_stream,
            index=int(proto_body.index),
        )

    @staticmethod
    def to_proto_request_header_action(action: RequestHeaderAction) -> proto.RequestHeaderActionPayload:
        payload = proto.RequestHeaderActionPayload()
        if action is None:
            return payload
        if isinstance(action, UpstreamRequestHeaderModifications):
            payload.upstream_request_header_modifications.CopyFrom(
                Translator._to_proto_upstream_request_header_modifications(action)
            )
            return payload
        if isinstance(action, ImmediateResponse):
            payload.immediate_response.CopyFrom(Translator._to_proto_immediate_response(action))
            return payload
        raise TypeError(f"unsupported request-header action type: {type(action)!r}")

    @staticmethod
    def to_proto_request_action(action: RequestAction) -> proto.RequestActionPayload:
        payload = proto.RequestActionPayload()
        if action is None:
            return payload
        if isinstance(action, UpstreamRequestModifications):
            payload.upstream_request_modifications.CopyFrom(
                Translator._to_proto_upstream_request_modifications(action)
            )
            return payload
        if isinstance(action, ImmediateResponse):
            payload.immediate_response.CopyFrom(Translator._to_proto_immediate_response(action))
            return payload
        raise TypeError(f"unsupported request action type: {type(action)!r}")

    @staticmethod
    def to_proto_response_header_action(action: ResponseHeaderAction) -> proto.ResponseHeaderActionPayload:
        payload = proto.ResponseHeaderActionPayload()
        if action is None:
            return payload
        if isinstance(action, DownstreamResponseHeaderModifications):
            payload.downstream_response_header_modifications.CopyFrom(
                Translator._to_proto_downstream_response_header_modifications(action)
            )
            return payload
        if isinstance(action, ImmediateResponse):
            payload.immediate_response.CopyFrom(Translator._to_proto_immediate_response(action))
            return payload
        raise TypeError(f"unsupported response-header action type: {type(action)!r}")

    @staticmethod
    def to_proto_response_action(action: ResponseAction) -> proto.ResponseActionPayload:
        payload = proto.ResponseActionPayload()
        if action is None:
            return payload
        if isinstance(action, DownstreamResponseModifications):
            payload.downstream_response_modifications.CopyFrom(
                Translator._to_proto_downstream_response_modifications(action)
            )
            return payload
        if isinstance(action, ImmediateResponse):
            payload.immediate_response.CopyFrom(Translator._to_proto_immediate_response(action))
            return payload
        raise TypeError(f"unsupported response action type: {type(action)!r}")

    @staticmethod
    def to_proto_fault_response(fault: FaultResponse | None) -> proto.FaultResponsePayload:
        """Serialize an ``on_fault`` return.

        ``None`` produces a payload with no action set, which the gateway reads as "changed
        nothing" — the same meaning nil carries in the Go contract. An empty FaultResponse is
        a different thing on the wire (an action that changes nothing) and stays
        distinguishable, though the outcome is identical.
        """
        payload = proto.FaultResponsePayload()
        if fault is None:
            return payload
        if not isinstance(fault, FaultResponse):
            raise TypeError(f"unsupported fault response type: {type(fault)!r}")
        message = payload.fault_response
        Translator._copy_int32_value(message.status_code, fault.status_code)
        Translator._copy_bytes_value(message.body, fault.body)
        message.headers_to_set.update(fault.headers_to_set)
        for name, values in fault.headers_to_append.items():
            message.headers_to_append[name].values.extend(values)
        message.headers_to_remove.extend(fault.headers_to_remove)
        Translator._copy_struct(message.analytics_metadata, fault.analytics_metadata)
        Translator._populate_struct_map(message.dynamic_metadata, fault.dynamic_metadata)
        Translator._copy_drop_header_action(
            message.analytics_header_filter,
            fault.analytics_header_filter,
        )
        message.final = fault.final
        if fault.fault is not None:
            Translator._copy_fault_details(message.fault, fault.fault)
        return payload

    @staticmethod
    def to_proto_streaming_request_action(
        action: StreamingRequestAction,
    ) -> proto.StreamingRequestActionPayload:
        payload = proto.StreamingRequestActionPayload()
        if action is None:
            return payload
        if not isinstance(action, ForwardRequestChunk):
            raise TypeError(f"unsupported streaming request action type: {type(action)!r}")
        payload.forward_request_chunk.CopyFrom(Translator._to_proto_forward_request_chunk(action))
        return payload

    @staticmethod
    def to_proto_streaming_response_action(
        action: StreamingResponseAction,
    ) -> proto.StreamingResponseActionPayload:
        payload = proto.StreamingResponseActionPayload()
        if action is None:
            return payload
        if isinstance(action, ForwardResponseChunk):
            payload.forward_response_chunk.CopyFrom(Translator._to_proto_forward_response_chunk(action))
            return payload
        if isinstance(action, TerminateResponseChunk):
            payload.terminate_response_chunk.CopyFrom(
                Translator._to_proto_terminate_response_chunk(action)
            )
            return payload
        raise TypeError(f"unsupported streaming response action type: {type(action)!r}")

    @staticmethod
    def to_proto_needs_more_decision(needs_more: bool) -> proto.NeedsMoreDecisionPayload:
        return proto.NeedsMoreDecisionPayload(needs_more=needs_more)

    @staticmethod
    def struct_to_dict(struct: Struct | None) -> dict[str, Any]:
        if struct is None:
            return {}
        return {key: Translator._proto_value_to_python(value) for key, value in struct.fields.items()}

    @staticmethod
    def dict_to_struct(data: Mapping[str, Any] | None) -> Struct:
        struct = Struct()
        if data:
            struct.update(dict(data))
        return struct

    @staticmethod
    def _proto_value_to_python(value: Value) -> Any:
        kind = value.WhichOneof("kind")
        if kind == "null_value":
            return None
        if kind == "number_value":
            return value.number_value
        if kind == "string_value":
            return value.string_value
        if kind == "bool_value":
            return value.bool_value
        if kind == "struct_value":
            return Translator.struct_to_dict(value.struct_value)
        if kind == "list_value":
            return [Translator._proto_value_to_python(v) for v in value.list_value.values]
        return None

    @staticmethod
    def _python_to_proto_value(value: Any) -> Value:
        """Wrap a JSON-RPC id for the wire. The inverse of _proto_value_to_python.

        The id is whatever the client sent, so the protocol permits a string, a number, or
        null. Anything else cannot have come from a parsed request; it is stringified rather
        than raising, since losing the exact type of a malformed id beats failing the whole
        error response.
        """
        if value is None:
            return Value(null_value=0)
        if isinstance(value, bool):
            return Value(bool_value=value)
        if isinstance(value, (int, float)):
            return Value(number_value=float(value))
        if isinstance(value, str):
            return Value(string_value=value)
        return Value(string_value=str(value))

    @staticmethod
    def _to_python_headers(proto_headers: proto.Headers) -> Headers:
        values: dict[str, list[str]] = {}
        for name, header_values in proto_headers.values.items():
            values[name] = list(header_values.values)
        return Headers(values)

    @staticmethod
    def _to_python_downstream(proto_ctx, field_name: str = "downstream") -> DownstreamContext | None:
        """Convert the optional downstream snapshot. Returns None when the field
        is unset (older gateways) so policies can detect absence and fall back."""
        if not proto_ctx.HasField(field_name):
            return None
        request = getattr(proto_ctx, field_name).request
        return DownstreamContext(
            request=DownstreamRequest(
                headers=Translator._to_python_headers(request.headers),
                path=request.path,
                method=request.method,
                authority=request.authority,
                scheme=request.scheme,
            ),
        )

    @staticmethod
    def _to_python_request_upstream(
        proto_ctx, field_name: str = "upstream"
    ) -> UpstreamRequestContext | None:
        """Convert the optional request-phase resolved upstream target. Returns
        None when the field is unset (older gateways); see _to_python_downstream."""
        if not proto_ctx.HasField(field_name):
            return None
        up = getattr(proto_ctx, field_name)
        return UpstreamRequestContext(
            name=up.name,
            url=up.url,
            base_path=up.base_path,
        )

    @staticmethod
    def _to_python_upstream(proto_ctx, field_name: str = "upstream") -> UpstreamResponseContext | None:
        """Convert the optional response-phase resolved upstream target and its
        response header snapshot. Returns None when the field is unset (older
        gateways); see _to_python_downstream."""
        if not proto_ctx.HasField(field_name):
            return None
        up = getattr(proto_ctx, field_name)
        response = None
        if up.HasField("response"):
            response = UpstreamResponse(
                headers=Translator._to_python_headers(up.response.headers),
                status_code=up.response.status_code,
            )
        return UpstreamResponseContext(
            name=up.name,
            url=up.url,
            base_path=up.base_path,
            response=response,
        )

    @staticmethod
    def _to_python_body(proto_body: proto.Body) -> Body:
        return Body(
            content=bytes(proto_body.content),
            end_of_stream=proto_body.end_of_stream,
            present=proto_body.present,
        )

    @staticmethod
    def _to_python_auth_context(proto_ctx: proto.AuthContext) -> AuthContext:
        previous = (
            Translator._to_python_auth_context(proto_ctx.previous)
            if proto_ctx.HasField("previous")
            else None
        )
        return AuthContext(
            authenticated=proto_ctx.authenticated,
            authorized=proto_ctx.authorized,
            auth_type=proto_ctx.auth_type,
            subject=proto_ctx.subject,
            issuer=proto_ctx.issuer,
            audience=list(proto_ctx.audience),
            scopes=dict(proto_ctx.scopes),
            credential_id=proto_ctx.credential_id,
            properties=dict(proto_ctx.properties),
            typed_properties=Translator.struct_to_dict(proto_ctx.typed_properties),
            previous=previous,
        )

    @staticmethod
    def _to_proto_headers(values: Mapping[str, list[str]] | None) -> proto.Headers:
        headers = proto.Headers()
        Translator._populate_string_list_map(headers.values, values)
        return headers

    @staticmethod
    def _copy_fault_declaration(message, action) -> None:
        """Copy a policy's fault declaration onto the outgoing action message.

        One helper for all three declaring actions — the two buffered ones and
        TerminateResponseChunk. They briefly diverged, while the buffered actions had their
        status read instead of a flag; they declare alike again.

        ``is_fault`` is a plain bool, so False and unset are the same wire value — which is
        exactly the intent: a policy that says nothing is not declaring a failure.

        ``fault`` is only set when the policy supplied one. Leaving the submessage absent
        keeps "described nothing" distinguishable from "described an empty fault", which the
        gateway relies on to decide whether there is anything to render.
        """
        message.is_fault = action.is_fault
        if action.fault is None:
            return
        Translator._copy_fault_details(message.fault, action.fault)

    @staticmethod
    def _copy_fault_details(target, declared) -> None:
        """Copy a FaultDetails onto an outgoing message's ``fault`` submessage.

        Split out of _copy_fault_declaration so the fault path can reuse it: a FaultResponse
        describes a fault but has no ``is_fault`` to declare, the fault flow being already
        under way. Keeping one copy of this means the guardrail and JSON-RPC presence rules
        below cannot drift between the two callers.
        """
        message_fault = target
        message_fault.code = declared.code
        message_fault.type = declared.type
        message_fault.direction = declared.direction
        message_fault.message = declared.message
        message_fault.description = declared.description
        guardrail = declared.guardrail
        if guardrail is not None:
            message_fault.guardrail.intervening_guardrail = guardrail.intervening_guardrail
            message_fault.guardrail.action = guardrail.action
            message_fault.guardrail.action_reason = guardrail.action_reason
            # Only set when the operator opted in. Touching the field would mark it present,
            # which on the far side reads as "an assessment was supplied and it was empty".
            if guardrail.assessments:
                Translator._copy_struct(message_fault.guardrail.assessments, guardrail.assessments)
            else:
                # A block with no assessment still has to exist — its metadata names which
                # guardrail acted, which is the part a fault handler needs most.
                message_fault.guardrail.SetInParent()

        jsonrpc = declared.jsonrpc
        if jsonrpc is None:
            return
        # Touching the submessage is what marks it present, so only do it when the policy
        # supplied one — an empty block would read as "an explicit JSON-RPC error with
        # nothing in it" rather than "not a JSON-RPC caller".
        Translator._copy_int32_value(message_fault.jsonrpc.code, jsonrpc.code)
        if jsonrpc.id is not None:
            message_fault.jsonrpc.id.CopyFrom(Translator._python_to_proto_value(jsonrpc.id))
        else:
            # An id-less block still has to exist if a code was set; setting the field
            # explicitly is what makes HasField("jsonrpc") true on the far side.
            message_fault.jsonrpc.SetInParent()

    @staticmethod
    def _to_proto_immediate_response(action: ImmediateResponse) -> proto.ImmediateResponse:
        message = proto.ImmediateResponse(status_code=action.status_code)
        message.headers.update(action.headers)
        Translator._copy_bytes_value(message.body, action.body)
        Translator._copy_struct(message.analytics_metadata, action.analytics_metadata)
        Translator._populate_struct_map(message.dynamic_metadata, action.dynamic_metadata)
        Translator._copy_drop_header_action(
            message.analytics_header_filter,
            action.analytics_header_filter,
        )
        Translator._copy_fault_declaration(message, action)
        return message

    @staticmethod
    def _to_proto_upstream_request_header_modifications(
        action: UpstreamRequestHeaderModifications,
    ) -> proto.UpstreamRequestHeaderModifications:
        message = proto.UpstreamRequestHeaderModifications()
        message.headers_to_set.update(action.headers_to_set)
        message.headers_to_remove.extend(action.headers_to_remove)
        Translator._copy_string_value(message.upstream_name, action.upstream_name)
        Translator._copy_string_value(message.path, action.path)
        Translator._copy_string_value(message.host, action.host)
        Translator._copy_string_value(message.method, action.method)
        Translator._populate_string_list_map(
            message.query_parameters_to_add,
            action.query_parameters_to_add,
        )
        message.query_parameters_to_remove.extend(action.query_parameters_to_remove)
        Translator._copy_struct(message.analytics_metadata, action.analytics_metadata)
        Translator._populate_struct_map(message.dynamic_metadata, action.dynamic_metadata)
        Translator._copy_drop_header_action(
            message.analytics_header_filter,
            action.analytics_header_filter,
        )
        return message

    @staticmethod
    def _to_proto_upstream_request_modifications(
        action: UpstreamRequestModifications,
    ) -> proto.UpstreamRequestModifications:
        message = proto.UpstreamRequestModifications()
        Translator._copy_bytes_value(message.body, action.body)
        message.headers_to_set.update(action.headers_to_set)
        message.headers_to_remove.extend(action.headers_to_remove)
        Translator._copy_string_value(message.upstream_name, action.upstream_name)
        Translator._copy_string_value(message.path, action.path)
        Translator._copy_string_value(message.host, action.host)
        Translator._copy_string_value(message.method, action.method)
        Translator._populate_string_list_map(
            message.query_parameters_to_add,
            action.query_parameters_to_add,
        )
        message.query_parameters_to_remove.extend(action.query_parameters_to_remove)
        Translator._copy_struct(message.analytics_metadata, action.analytics_metadata)
        Translator._populate_struct_map(message.dynamic_metadata, action.dynamic_metadata)
        Translator._copy_drop_header_action(
            message.analytics_header_filter,
            action.analytics_header_filter,
        )
        return message

    @staticmethod
    def _to_proto_downstream_response_header_modifications(
        action: DownstreamResponseHeaderModifications,
    ) -> proto.DownstreamResponseHeaderModifications:
        message = proto.DownstreamResponseHeaderModifications()
        message.headers_to_set.update(action.headers_to_set)
        message.headers_to_remove.extend(action.headers_to_remove)
        Translator._copy_struct(message.analytics_metadata, action.analytics_metadata)
        Translator._populate_struct_map(message.dynamic_metadata, action.dynamic_metadata)
        Translator._copy_drop_header_action(
            message.analytics_header_filter,
            action.analytics_header_filter,
        )
        return message

    @staticmethod
    def _to_proto_downstream_response_modifications(
        action: DownstreamResponseModifications,
    ) -> proto.DownstreamResponseModifications:
        message = proto.DownstreamResponseModifications()
        Translator._copy_bytes_value(message.body, action.body)
        Translator._copy_int32_value(message.status_code, action.status_code)
        message.headers_to_set.update(action.headers_to_set)
        message.headers_to_remove.extend(action.headers_to_remove)
        Translator._copy_struct(message.analytics_metadata, action.analytics_metadata)
        Translator._populate_struct_map(message.dynamic_metadata, action.dynamic_metadata)
        Translator._copy_drop_header_action(
            message.analytics_header_filter,
            action.analytics_header_filter,
        )
        Translator._copy_fault_declaration(message, action)
        return message

    @staticmethod
    def _to_proto_forward_request_chunk(action: ForwardRequestChunk) -> proto.ForwardRequestChunk:
        message = proto.ForwardRequestChunk()
        Translator._copy_bytes_value(message.body, action.body)
        Translator._copy_struct(message.analytics_metadata, action.analytics_metadata)
        Translator._populate_struct_map(message.dynamic_metadata, action.dynamic_metadata)
        return message

    @staticmethod
    def _to_proto_forward_response_chunk(action: ForwardResponseChunk) -> proto.ForwardResponseChunk:
        message = proto.ForwardResponseChunk()
        Translator._copy_bytes_value(message.body, action.body)
        Translator._copy_struct(message.analytics_metadata, action.analytics_metadata)
        Translator._populate_struct_map(message.dynamic_metadata, action.dynamic_metadata)
        return message

    @staticmethod
    def _to_proto_terminate_response_chunk(
        action: TerminateResponseChunk,
    ) -> proto.TerminateResponseChunk:
        message = proto.TerminateResponseChunk()
        Translator._copy_bytes_value(message.body, action.body)
        Translator._copy_struct(message.analytics_metadata, action.analytics_metadata)
        Translator._populate_struct_map(message.dynamic_metadata, action.dynamic_metadata)
        Translator._copy_fault_declaration(message, action)
        return message

    @staticmethod
    def _populate_string_list_map(destination, values: Mapping[str, list[str]] | None) -> None:
        for key, string_values in (values or {}).items():
            destination[key].values.extend(string_values)

    @staticmethod
    def _populate_struct_map(destination, values: Mapping[str, Mapping[str, Any]] | None) -> None:
        for key, data in (values or {}).items():
            destination[key].CopyFrom(Translator.dict_to_struct(data))

    @staticmethod
    def _copy_struct(field: Struct, data: Mapping[str, Any] | None) -> None:
        if data:
            field.CopyFrom(Translator.dict_to_struct(data))

    @staticmethod
    def _copy_bytes_value(field, value: bytes | None) -> None:
        if value is not None:
            field.CopyFrom(BytesValue(value=value))

    @staticmethod
    def _copy_string_value(field, value: str | None) -> None:
        if value is not None:
            field.CopyFrom(StringValue(value=value))

    @staticmethod
    def _copy_int32_value(field, value: int | None) -> None:
        if value is not None:
            field.CopyFrom(Int32Value(value=value))

    @staticmethod
    def _copy_drop_header_action(field, action: DropHeaderAction | None) -> None:
        if action is None or not action.headers:
            return
        try:
            action_type = Translator._DROP_HEADER_ACTION_TO_PROTO[action.action.lower()]
        except KeyError as exc:
            raise ValueError(f"unsupported analytics header filter action: {action.action}") from exc
        field.action = action_type
        field.headers.extend(action.headers)
