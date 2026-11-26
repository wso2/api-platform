// Throwaway ext_proc server for Probe E.
//
// Answers three questions the Lua probes could not:
//  1. Is ext_proc invoked once per RETRY ATTEMPT at the upstream position?
//  2. Do request_attributes carry PER-ATTEMPT target identity (xds.cluster_name,
//     upstream.address), or the first attempt's values?
//  3. What does the extra gRPC round trip cost per attempt?
//
// Question 2 decides whether the attempt-index-into-chain workaround that Probe D
// forced survives into the real design, or gets deleted.
package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sort"
	"strconv"
	"time"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocconfig "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extproc "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"google.golang.org/grpc"
)

type server struct {
	extproc.UnimplementedExternalProcessorServer
}

// contexts stands in for the resolved failover chain the downstream policy publishes.
var contexts = []string{"/ctx-a", "/ctx-b"}

func (s *server) Process(stream extproc.ExternalProcessor_ProcessServer) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		started := time.Now()

		// Catch-all: name every message type that arrives, before any handling, so a message
		// the probe fails to answer is distinguishable from one it never received.
		log.Printf("PROBE >>> message received: %T", req.GetRequest())

		// PROBE T4: does the UPSTREAM position deliver RESPONSES at all? The filter is
		// generated response-SKIP today, on the stated grounds that a response here carries
		// no cluster attribute. Answer these before anything else so an unexpected message
		// type is reported rather than falling through the request handling below and
		// replying with the wrong response type, which kills the stream.
		if rh := req.GetResponseHeaders(); rh != nil {
			status := ""
			for _, h := range rh.GetHeaders().GetHeaders() {
				if h.GetKey() == ":status" {
					status = h.GetValue()
					if status == "" {
						status = string(h.GetRawValue())
					}
				}
			}
			log.Printf("PROBE *** RECEIVED RESPONSE HEADERS at upstream position: status=%s ***", status)
			if err := stream.Send(&extproc.ProcessingResponse{
				Response: &extproc.ProcessingResponse_ResponseHeaders{
					ResponseHeaders: &extproc.HeadersResponse{Response: &extproc.CommonResponse{}},
				},
			}); err != nil {
				return err
			}
			continue
		}
		if rb := req.GetResponseBody(); rb != nil {
			log.Printf("PROBE *** RECEIVED RESPONSE BODY at upstream position: %d bytes eos=%v ***",
				len(rb.GetBody()), rb.GetEndOfStream())
			if err := stream.Send(&extproc.ProcessingResponse{
				Response: &extproc.ProcessingResponse_ResponseBody{
					ResponseBody: &extproc.BodyResponse{Response: &extproc.CommonResponse{}},
				},
			}); err != nil {
				return err
			}
			continue
		}

		// PROBE I: does the UPSTREAM position deliver a request BODY at all?
		if body := req.GetRequestBody(); body != nil {
			log.Printf("PROBE *** RECEIVED REQUEST BODY at upstream position: %d bytes ***", len(body.GetBody()))
			if err := stream.Send(&extproc.ProcessingResponse{
				Response: &extproc.ProcessingResponse_RequestBody{
					RequestBody: &extproc.BodyResponse{Response: &extproc.CommonResponse{}},
				},
			}); err != nil {
				return err
			}
			continue
		}

		headers := req.GetRequestHeaders()
		if headers == nil {
			// Not a phase we asked for; acknowledge and move on.
			if err := stream.Send(&extproc.ProcessingResponse{}); err != nil {
				return err
			}
			continue
		}

		path, attempt := "", 1
		for _, h := range headers.GetHeaders().GetHeaders() {
			value := h.GetValue()
			if value == "" && len(h.GetRawValue()) > 0 {
				value = string(h.GetRawValue())
			}
			switch h.GetKey() {
			case ":path":
				path = value
			case "x-envoy-attempt-count":
				if n, convErr := strconv.Atoi(value); convErr == nil && n > 0 {
					attempt = n
				}
			}
		}

		// Q2: what identity did Envoy actually send us for THIS attempt?
		log.Printf("PROBE call attempt=%d path=%s", attempt, path)
		keys := make([]string, 0, len(req.GetAttributes()))
		for k := range req.GetAttributes() {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if len(keys) == 0 {
			log.Printf("PROBE   attributes: NONE RECEIVED")
		}
		for _, k := range keys {
			for field, v := range req.GetAttributes()[k].GetFields() {
				log.Printf("PROBE   attribute %s.%s = %s", k, field, v.String())
			}
		}

		// Rewrite :path to this attempt's provider context, built from a fixed
		// source rather than the incoming path -- Probe D showed mutations
		// accumulate across attempts.
		resp := &extproc.ProcessingResponse{}
		if attempt >= 1 && attempt <= len(contexts) {
			rewritten := contexts[attempt-1] + "/chat/completions"
			log.Printf("PROBE   rewriting :path -> %s", rewritten)
			resp.Response = &extproc.ProcessingResponse_RequestHeaders{
				RequestHeaders: &extproc.HeadersResponse{
					Response: &extproc.CommonResponse{
						HeaderMutation: &extproc.HeaderMutation{
							SetHeaders: []*core.HeaderValueOption{{
								Header: &core.HeaderValue{
									Key:      ":path",
									RawValue: []byte(rewritten),
								},
							}},
						},
					},
				},
			}
		} else {
			log.Printf("PROBE   attempt %d beyond chain, leaving path alone", attempt)
			resp.Response = &extproc.ProcessingResponse_RequestHeaders{
				RequestHeaders: &extproc.HeadersResponse{Response: &extproc.CommonResponse{}},
			}
		}

		// PROBE I: ask Envoy to turn the body phase ON via mode_override,
		// exactly as the production upstream dispatcher does.
		//
		// Gated, because a probe that only asks "was this position invoked at all" does not
		// want it: requesting BUFFERED and then not answering the body message stalls the
		// attempt until the route timeout, which reads as a 504 and hides the very thing
		// being measured. Set PROBE_BODY_MODE=off for a headers-only run.
		if resp.GetRequestHeaders() != nil && os.Getenv("PROBE_BODY_MODE") != "off" {
			resp.ModeOverride = &extprocconfig.ProcessingMode{
				RequestHeaderMode:  extprocconfig.ProcessingMode_SEND,
				RequestBodyMode:    extprocconfig.ProcessingMode_BUFFERED,
				ResponseHeaderMode: extprocconfig.ProcessingMode_SKIP,
			}
			log.Printf("PROBE   sent mode_override requesting BUFFERED body")
		}

		if err := stream.Send(resp); err != nil {
			return err
		}
		log.Printf("PROBE   handled in %s", time.Since(started))
	}
}

func main() {
	listener, err := net.Listen("tcp", ":9000")
	if err != nil {
		panic(err)
	}
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(4*1024*1024),
		grpc.MaxSendMsgSize(4*1024*1024),
		grpc.MaxConcurrentStreams(100),
	)
	extproc.RegisterExternalProcessorServer(grpcServer, &server{})
	fmt.Println("PROBE ext_proc server listening on :9000")
	if err := grpcServer.Serve(listener); err != nil {
		panic(err)
	}
}
