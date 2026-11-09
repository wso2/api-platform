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
	"encoding/hex"
	"encoding/json"
	"strings"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
)

// Span is a received span, flattened for assertions.
type Span struct {
	TraceID      string         `json:"traceId"`
	SpanID       string         `json:"spanId"`
	ParentSpanID string         `json:"parentSpanId"`
	Name         string         `json:"name"`
	Kind         string         `json:"kind"`
	Service      string         `json:"service"`
	StatusCode   string         `json:"statusCode"`
	Attributes   map[string]any `json:"attributes"`
	Links        []Link         `json:"links"`
}

// Link is a span link.
type Link struct {
	TraceID string `json:"traceId"`
	SpanID  string `json:"spanId"`
}

// LogRecord is a received log record, flattened for assertions.
type LogRecord struct {
	TraceID    string         `json:"traceId"`
	SpanID     string         `json:"spanId"`
	EventName  string         `json:"eventName"`
	Attributes map[string]any `json:"attributes"`
}

// Flatten turns an export request into one Span per span, with the resource's
// service.name on each.
func Flatten(req *coltracepb.ExportTraceServiceRequest) []Span {
	var out []Span
	for _, rs := range req.GetResourceSpans() {
		service, _ := attributes(rs.GetResource().GetAttributes())["service.name"].(string)
		for _, ss := range rs.GetScopeSpans() {
			for _, s := range ss.GetSpans() {
				span := Span{
					TraceID:      hex.EncodeToString(s.GetTraceId()),
					SpanID:       hex.EncodeToString(s.GetSpanId()),
					ParentSpanID: hex.EncodeToString(s.GetParentSpanId()),
					Name:         s.GetName(),
					Kind:         strings.TrimPrefix(s.GetKind().String(), "SPAN_KIND_"),
					Service:      service,
					StatusCode:   strings.TrimPrefix(s.GetStatus().GetCode().String(), "STATUS_CODE_"),
					Attributes:   attributes(s.GetAttributes()),
					Links:        []Link{},
				}
				for _, l := range s.GetLinks() {
					span.Links = append(span.Links, Link{TraceID: hex.EncodeToString(l.GetTraceId()), SpanID: hex.EncodeToString(l.GetSpanId())})
				}
				out = append(out, span)
			}
		}
	}
	return out
}

func attributes(kvs []*commonpb.KeyValue) map[string]any {
	out := make(map[string]any, len(kvs))
	for _, kv := range kvs {
		out[kv.GetKey()] = value(kv.GetValue())
	}
	return out
}

func value(v *commonpb.AnyValue) any {
	switch x := v.GetValue().(type) {
	case *commonpb.AnyValue_StringValue:
		return x.StringValue
	case *commonpb.AnyValue_IntValue:
		return x.IntValue
	case *commonpb.AnyValue_DoubleValue:
		return x.DoubleValue
	case *commonpb.AnyValue_BoolValue:
		return x.BoolValue
	case *commonpb.AnyValue_ArrayValue:
		items := []any{}
		for _, item := range x.ArrayValue.GetValues() {
			items = append(items, value(item))
		}
		return items
	}
	return nil
}

// FlattenLogs reads an OTLP/HTTP JSON logs export, the encoding the gateway's OTel
// analytics publisher sends.
func FlattenLogs(body []byte) ([]LogRecord, error) {
	var req struct {
		ResourceLogs []struct {
			ScopeLogs []struct {
				LogRecords []struct {
					TraceID    string `json:"traceId"`
					SpanID     string `json:"spanId"`
					EventName  string `json:"eventName"`
					Attributes []struct {
						Key   string          `json:"key"`
						Value json.RawMessage `json:"value"`
					} `json:"attributes"`
				} `json:"logRecords"`
			} `json:"scopeLogs"`
		} `json:"resourceLogs"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	var out []LogRecord
	for _, rl := range req.ResourceLogs {
		for _, sl := range rl.ScopeLogs {
			for _, lr := range sl.LogRecords {
				rec := LogRecord{TraceID: lr.TraceID, SpanID: lr.SpanID, EventName: lr.EventName, Attributes: map[string]any{}}
				for _, a := range lr.Attributes {
					var v map[string]any
					if json.Unmarshal(a.Value, &v) == nil {
						for _, x := range v {
							rec.Attributes[a.Key] = x
						}
					}
				}
				out = append(out, rec)
			}
		}
	}
	return out, nil
}
