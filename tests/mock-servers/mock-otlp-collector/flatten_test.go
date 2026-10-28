// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package main

import (
	"testing"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func str(k, v string) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}}}
}

func TestFlatten(t *testing.T) {
	req := &coltracepb.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{{
		Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{str("service.name", "policy-engine")}},
		ScopeSpans: []*tracepb.ScopeSpans{{Spans: []*tracepb.Span{{
			TraceId:      []byte{0x4b, 0xf9, 0x2f, 0x35, 0x77, 0xb3, 0x4d, 0xa6, 0xa3, 0xce, 0x92, 0x9d, 0x0e, 0x0e, 0x47, 0x36},
			SpanId:       []byte{1, 2, 3, 4, 5, 6, 7, 8},
			ParentSpanId: []byte{8, 7, 6, 5, 4, 3, 2, 1},
			Name:         "chat gpt-4o",
			Kind:         tracepb.Span_SPAN_KIND_CLIENT,
			Attributes: []*commonpb.KeyValue{
				str("gen_ai.operation.name", "chat"),
				{Key: "gen_ai.usage.input_tokens", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: 19}}},
				{Key: "gen_ai.response.finish_reasons", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_ArrayValue{ArrayValue: &commonpb.ArrayValue{Values: []*commonpb.AnyValue{{Value: &commonpb.AnyValue_StringValue{StringValue: "stop"}}}}}}},
			},
		}}}},
	}}}
	spans := Flatten(req)
	if len(spans) != 1 {
		t.Fatalf("want 1 span, got %d", len(spans))
	}
	s := spans[0]
	if s.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" || s.SpanID != "0102030405060708" || s.ParentSpanID != "0807060504030201" {
		t.Errorf("ids = %s/%s/%s", s.TraceID, s.SpanID, s.ParentSpanID)
	}
	if s.Kind != "CLIENT" || s.Service != "policy-engine" || s.Name != "chat gpt-4o" {
		t.Errorf("kind/service/name = %s/%s/%s", s.Kind, s.Service, s.Name)
	}
	if s.Attributes["gen_ai.usage.input_tokens"] != int64(19) || s.Attributes["gen_ai.operation.name"] != "chat" {
		t.Errorf("attributes = %v", s.Attributes)
	}
	if reasons, _ := s.Attributes["gen_ai.response.finish_reasons"].([]any); len(reasons) != 1 || reasons[0] != "stop" {
		t.Errorf("finish reasons = %v", s.Attributes["gen_ai.response.finish_reasons"])
	}
}

func TestFlattenLogs(t *testing.T) {
	logs, err := FlattenLogs([]byte(`{"resourceLogs":[{"scopeLogs":[{"logRecords":[{"traceId":"abc","spanId":"def","eventName":"wso2.api.transaction","attributes":[{"key":"wso2.api.name","value":{"stringValue":"pets"}}]}]}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || logs[0].TraceID != "abc" || logs[0].SpanID != "def" || logs[0].Attributes["wso2.api.name"] != "pets" {
		t.Errorf("logs = %+v", logs)
	}
}
