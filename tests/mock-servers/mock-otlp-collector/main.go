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

// mock-otlp-collector receives OpenTelemetry spans and log records and lets a test read
// them back. It accepts OTLP/gRPC traces on :4317, OTLP/HTTP protobuf traces and
// OTLP/HTTP JSON logs on :4318, and serves the test API on :8080:
//
//	GET  /test/spans?traceId=<hex>   spans received, flattened, optionally of one trace
//	GET  /test/logs?traceId=<hex>    log records received, optionally of one trace
//	POST /test/reset                 forget everything received
//	GET  /test/health
package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"sync"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type store struct {
	mu    sync.Mutex
	spans []Span
	logs  []LogRecord
}

func (s *store) addSpans(req *coltracepb.ExportTraceServiceRequest) {
	spans := Flatten(req)
	s.mu.Lock()
	s.spans = append(s.spans, spans...)
	s.mu.Unlock()
}

type traceServer struct {
	coltracepb.UnimplementedTraceServiceServer
	store *store
}

func (t *traceServer) Export(_ context.Context, req *coltracepb.ExportTraceServiceRequest) (*coltracepb.ExportTraceServiceResponse, error) {
	t.store.addSpans(req)
	return &coltracepb.ExportTraceServiceResponse{}, nil
}

func main() {
	st := &store{}

	lis, err := net.Listen("tcp", ":4317")
	if err != nil {
		log.Fatalf("listen 4317: %v", err)
	}
	grpcServer := grpc.NewServer()
	coltracepb.RegisterTraceServiceServer(grpcServer, &traceServer{store: st})
	go func() { log.Fatal(grpcServer.Serve(lis)) }()

	otlpHTTP := http.NewServeMux()
	otlpHTTP.HandleFunc("POST /v1/traces", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		req := &coltracepb.ExportTraceServiceRequest{}
		if err := proto.Unmarshal(body, req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		st.addSpans(req)
		w.Header().Set("Content-Type", "application/x-protobuf")
		out, _ := proto.Marshal(&coltracepb.ExportTraceServiceResponse{})
		_, _ = w.Write(out)
	})
	otlpHTTP.HandleFunc("POST /v1/logs", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		records, err := FlattenLogs(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		st.mu.Lock()
		st.logs = append(st.logs, records...)
		st.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{}"))
	})
	go func() { log.Fatal(http.ListenAndServe(":4318", otlpHTTP)) }()

	api := http.NewServeMux()
	api.HandleFunc("GET /test/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	api.HandleFunc("POST /test/reset", func(w http.ResponseWriter, _ *http.Request) {
		st.mu.Lock()
		st.spans, st.logs = nil, nil
		st.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	api.HandleFunc("GET /test/spans", func(w http.ResponseWriter, r *http.Request) {
		traceID := r.URL.Query().Get("traceId")
		st.mu.Lock()
		out := []Span{}
		for _, s := range st.spans {
			if traceID == "" || s.TraceID == traceID {
				out = append(out, s)
			}
		}
		st.mu.Unlock()
		writeJSON(w, out)
	})
	api.HandleFunc("GET /test/logs", func(w http.ResponseWriter, r *http.Request) {
		traceID := r.URL.Query().Get("traceId")
		st.mu.Lock()
		out := []LogRecord{}
		for _, l := range st.logs {
			if traceID == "" || l.TraceID == traceID {
				out = append(out, l)
			}
		}
		st.mu.Unlock()
		writeJSON(w, out)
	})
	log.Println("mock-otlp-collector: gRPC :4317, HTTP :4318, test API :8080")
	log.Fatal(http.ListenAndServe(":8080", api))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
