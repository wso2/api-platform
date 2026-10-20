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

// Package opa provides a deterministic Open Policy Agent (OPA) data API for the opa-policy
// integration tests.
//
// It answers POST /v1/data/<package path>/<rule> the way OPA's data API does: the decision of a
// boolean rule is {"result": true|false}, and a document the server does not define is the
// empty object {}. The decision is a function of the request, so a scenario chooses the
// outcome by the policy and rule it configures and by what it sends:
//
//	authz/policy/allow     the request header X-Opa-Decision forces an outcome (allow, deny,
//	                       undefined, invalid, error); without it, GET is allowed and any other
//	                       method is allowed only with the header X-Test-User: admin
//	authz/context/allow    allow only when the input carries the request metadata the gateway is
//	                       expected to send (method, path, headers, apiContext.apiName)
//	authz/token/allow      allow only when the input carries apiContext.accessToken
//	au    allow only when the input carries no apiContext.accessToken
//	authz/secured/allow thz/notoken/allow   allow, but only for a caller presenting "Authorization: Bearer
//	                       opa-test-key"; any other caller is answered 401
//	authz/slow/allow       allow, after SlowDelay
//
// Any other path is an undefined document and is answered {}.
package opa

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/wso2/api-platform/tests/framework/testbench"
)

// Port is the container port used by the testbench. Features address http://testbench:3018.
const Port = 3018

const (
	// DataPath is the prefix of OPA's data API.
	DataPath = "/v1/data/"

	// HeaderDecision is the request header, read from the OPA input's transportHeaders, that
	// forces the outcome of authz/policy/allow.
	HeaderDecision = "x-opa-decision"

	// HeaderTestUser is the request header that lets a non-GET request through authz/policy/allow.
	HeaderTestUser = "x-test-user"

	// AccessKey is the bearer token authz/secured/allow requires from its caller.
	AccessKey = "opa-test-key"

	// SlowDelay is how long authz/slow/allow takes to answer.
	SlowDelay = 3 * time.Second

	maxRequestBodySize = 1 << 20
)

// Forced outcomes of authz/policy/allow, selected by the X-Opa-Decision request header.
const (
	DecisionAllow     = "allow"
	DecisionDeny      = "deny"
	DecisionUndefined = "undefined"
	DecisionInvalid   = "invalid"
	DecisionError     = "error"
)

// Service implements the stateless OPA test service.
type Service struct{}

// New returns a new OPA service.
func New() *Service { return &Service{} }

// Name returns the service registration name.
func (s *Service) Name() string { return "opa" }

// Port returns the service's listening port.
func (s *Service) Port() int { return Port }

// Stateful reports whether the service keeps request-specific state.
func (s *Service) Stateful() bool { return false }

// Handler returns the data API handler.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+DataPath, s.query)
	return mux
}

// input is the part of the OPA input document the rules read.
type input struct {
	RequestMethod    string            `json:"requestMethod"`
	RequestPath      string            `json:"requestPath"`
	TransportHeaders map[string]string `json:"transportHeaders"`
	APIContext       struct {
		APIName     string `json:"apiName"`
		AccessToken string `json:"accessToken"`
	} `json:"apiContext"`
}

func (s *Service) query(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBodySize))
	if err != nil {
		http.Error(w, "could not read request body", http.StatusBadRequest)
		return
	}
	var req struct {
		Input input `json:"input"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, `{"code":"invalid_parameter","message":"error(s) occurred while decoding request"}`, http.StatusBadRequest)
		return
	}
	in := req.Input

	rulePath := strings.Trim(strings.TrimPrefix(r.URL.Path, DataPath), "/")
	switch rulePath {
	case "authz/policy/allow":
		s.policy(w, in)
	case "authz/context/allow":
		decide(w, in.RequestMethod != "" && in.RequestPath != "" && in.TransportHeaders != nil && in.APIContext.APIName != "")
	case "authz/token/allow":
		decide(w, in.APIContext.AccessToken != "")
	case "authz/notoken/allow":
		decide(w, in.APIContext.AccessToken == "")
	case "authz/secured/allow":
		if r.Header.Get("Authorization") != "Bearer "+AccessKey {
			http.Error(w, `{"code":"unauthorized","message":"invalid or missing token"}`, http.StatusUnauthorized)
			return
		}
		decide(w, true)
	case "authz/slow/allow":
		select {
		case <-time.After(SlowDelay):
		case <-r.Context().Done():
			return
		}
		decide(w, true)
	default:
		undefined(w)
	}
}

// policy is authz/policy/allow.
func (s *Service) policy(w http.ResponseWriter, in input) {
	switch strings.ToLower(in.TransportHeaders[HeaderDecision]) {
	case DecisionAllow:
		decide(w, true)
	case DecisionDeny:
		decide(w, false)
	case DecisionUndefined:
		undefined(w)
	case DecisionInvalid:
		writeJSON(w, `{"result":"yes"}`)
	case DecisionError:
		http.Error(w, `{"code":"internal_error","message":"forced error"}`, http.StatusInternalServerError)
	default:
		decide(w, in.RequestMethod == http.MethodGet || strings.EqualFold(in.TransportHeaders[HeaderTestUser], "admin"))
	}
}

func decide(w http.ResponseWriter, allowed bool) {
	if allowed {
		writeJSON(w, `{"result":true}`)
		return
	}
	writeJSON(w, `{"result":false}`)
}

// undefined answers a document OPA does not define: the empty object.
func undefined(w http.ResponseWriter) { writeJSON(w, `{}`) }

func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, body)
}

var _ testbench.Service = (*Service)(nil)
