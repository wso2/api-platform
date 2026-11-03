/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package aws

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

// Port is the container port. Every AWS endpoint the service offers is on it, addressed by the
// first two segments of the request path: http://testbench/<service>/<region>/...
//
// It is the HTTP default port so that an upstream URL names no port, like a real AWS endpoint on
// 443. The gateway signs the upstream URL's host and port, but its automatic host rewrite forwards
// the Host header without a non-default port, so a request to any other port would never verify.
const Port = 80

// serviceSTS is the service name that selects STS rather than the upstream. The gateway
// runtime's AWS SDK reaches STS through AWS_ENDPOINT_URL_STS.
const serviceSTS = "sts"

// Service is the testbench's AWS service: the upstream (upstream.go) and STS (sts.go), behind one
// port. It keeps no state, so concurrent callers never affect one another.
type Service struct {
	upstream *upstream
	sts      *sts
}

// New creates the AWS service.
func New() *Service {
	return newService(time.Now)
}

func newService(now func() time.Time) *Service {
	return &Service{
		upstream: &upstream{verifier: verifier{lookup: lookupCredential, now: now}},
		sts:      &sts{verifier: verifier{lookup: lookupFixtureCredential, now: now}, hold: unresponsiveHold},
	}
}

// Name returns the service name used by the testbench registry.
func (s *Service) Name() string { return "aws" }

// Port returns the service's listening port.
func (s *Service) Port() int { return Port }

// Stateful reports whether the service keeps request-specific state. STS-issued credentials are
// derived from their access key ID, not stored.
func (s *Service) Stateful() bool { return false }

// endpoint is where a request is addressed: the AWS service and region named by the first two
// path segments, and the rest of the path below them.
type endpoint struct {
	service string
	region  string
	rest    string
}

// endpointOf splits /<service>/<region>[/rest] into its parts.
func endpointOf(path string) (endpoint, bool) {
	segments := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 3)
	if len(segments) < 2 || segments[0] == "" || segments[1] == "" {
		return endpoint{}, false
	}
	ep := endpoint{service: segments[0], region: segments[1]}
	if len(segments) == 3 {
		ep.rest = "/" + segments[2]
	}
	return ep, true
}

// Handler routes each request to the AWS API its path names. The upstream and STS verify the
// request's signature before anything else, on the path exactly as it was signed.
func (s *Service) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			health(w, r)
			return
		}
		ep, ok := endpointOf(r.URL.Path)
		if !ok {
			writeJSONError(w, &awsError{status: http.StatusNotFound, code: "NotFound",
				message: "The request path must start with /<service>/<region>"})
			return
		}
		if ep.service == serviceSTS {
			s.sts.serve(w, r, ep)
			return
		}
		s.upstream.serve(w, r, ep)
	})
}

func health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "aws"})
}

// readBody reads the request body, writing an error response through fail when it cannot.
func readBody(w http.ResponseWriter, r *http.Request, fail func(http.ResponseWriter, *awsError)) ([]byte, bool) {
	body, err := io.ReadAll(r.Body)
	if err == nil {
		return body, true
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		fail(w, &awsError{status: http.StatusRequestEntityTooLarge, code: "RequestEntityTooLarge",
			message: "Request Entity Too Large"})
		return nil, false
	}
	fail(w, &awsError{status: http.StatusBadRequest, code: "InvalidRequest", message: "Failed to read request body"})
	return nil, false
}

// writeJSONError writes a rejection in the JSON shape of an AWS REST service, with the error
// code in X-Amzn-ErrorType, where the AWS SDKs read it.
func writeJSONError(w http.ResponseWriter, e *awsError) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Amzn-ErrorType", e.code)
	w.WriteHeader(e.status)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": e.code, "message": e.message})
}
