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

package entra

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/wso2/api-platform/tests/framework/testbench"
)

// Port is the container port the service answers every route on over plain HTTP: the token and
// IMDS endpoints, their request logs and the protected resources.
const Port = 3027

// TLSPort is the port the same service answers on over HTTPS, for a token client that requires an
// https authority, as the azure-authentication policy does.
const TLSPort = 3028

// EnvTLSCert and EnvTLSKey name the variables holding the PEM certificate and key the service
// serves HTTPS with on TLSPort. When both are unset only the plain HTTP port is served.
const (
	EnvTLSCert = "TESTBENCH_ENTRA_TLS_CERT"
	EnvTLSKey  = "TESTBENCH_ENTRA_TLS_KEY"
)

// Tenant prefixes that select how the token endpoint answers. A tenant without one of them is
// answered with a token.
const (
	// PrefixInvalidClient answers every token request with 401 invalid_client, AADSTS700213.
	PrefixInvalidClient = "invalid-client-"
	// PrefixRejectOnce answers the tenant's first token request like PrefixInvalidClient and
	// issues a token for every later one.
	PrefixRejectOnce = "reject-once-"
	// PrefixThrottleOnce answers the tenant's first token request with 429 and Retry-After: 1.
	PrefixThrottleOnce = "throttle-once-"
	// PrefixErrorOnce answers the tenant's first token request with 500 and an empty body.
	PrefixErrorOnce = "error-once-"
	// PrefixRefreshSoon issues tokens, the tenant's first one with "refresh_in": 2.
	PrefixRefreshSoon = "refresh-soon-"
	// PrefixUnavailable answers every token request with 503.
	PrefixUnavailable = "unavailable-"
	// PrefixSlow issues tokens after the slow delay, so a short client timeout expires first.
	PrefixSlow = "slow-"
)

// defaultSlowDelay is longer than the one-second token timeouts scenarios set, and well inside the
// testbench's 30-second write timeout.
const defaultSlowDelay = 3 * time.Second

// maxPartitions bounds how many partitions the service holds: tenants, and each block's managed
// identities.
const maxPartitions = 4096

// maxRetainedRequests bounds the requests one partition keeps; the count stays exact beyond it.
const maxRetainedRequests = 256

// maxFormBytes bounds one token request body.
const maxFormBytes = 64 << 10

// Request is one recorded token request: a client credentials request to the Entra ID token
// endpoint, or a managed identity request to the IMDS endpoint.
type Request struct {
	ClientID            string `json:"clientId"`
	Scope               string `json:"scope"`
	GrantType           string `json:"grantType"`
	ClientAssertionType string `json:"clientAssertionType"`
	ClientAssertion     string `json:"clientAssertion"`
	// Resource, APIVersion and Metadata are the IMDS request's resource and api-version query
	// parameters and its Metadata header.
	Resource   string `json:"resource,omitempty"`
	APIVersion string `json:"apiVersion,omitempty"`
	Metadata   string `json:"metadata,omitempty"`
	// Status is the HTTP status the service answered the request with, or 0 when the caller left
	// before the answer was sent.
	Status int `json:"status"`
}

type partition struct {
	// count is every request the partition received; eligible is the well-formed ones, the
	// count that picks a one-time answer.
	count    int
	eligible int
	requests []Request
}

// Service implements testbench.Service and testbench.Partitioned. The first path segment of every
// route is its partition: the Entra tenant for the token endpoint, its log and the resources, and
// the block for the IMDS endpoint and its log, whose state is kept per block and identity.
type Service struct {
	mu         sync.Mutex
	partitions map[string]*partition
	slowDelay  time.Duration
}

// New returns a new Entra ID service.
func New() *Service { return newWithSlowDelay(defaultSlowDelay) }

// newWithSlowDelay returns a service whose slow- tenants wait delay before answering.
func newWithSlowDelay(delay time.Duration) *Service {
	return &Service{partitions: map[string]*partition{}, slowDelay: delay}
}

// Name returns the service registration name.
func (s *Service) Name() string { return "entra" }

// Port returns the service's listening port.
func (s *Service) Port() int { return Port }

// Stateful reports that the service retains the token requests it receives.
func (s *Service) Stateful() bool { return true }

// PartitionKey returns the partitioning strategy used by this stateful service.
func (s *Service) PartitionKey() string { return testbench.PartitionByBlock }

// OverTLS returns the service served over HTTPS on TLSPort with the supplied PEM certificate and
// key. It shares this service's tenants, so a token requested over HTTPS is read back from
// /test/requests on either port.
func (s *Service) OverTLS(certPEM, keyPEM []byte) (testbench.Service, error) {
	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("entra: loading the TLS key pair: %w", err)
	}
	return &secure{
		Service:   s,
		tlsConfig: &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12},
	}, nil
}

// secure is the service registered on TLSPort; everything but its name, port and TLS
// configuration is the plain service's.
type secure struct {
	*Service
	tlsConfig *tls.Config
}

func (s *secure) Name() string           { return "entra-tls" }
func (s *secure) Port() int              { return TLSPort }
func (s *secure) TLSConfig() *tls.Config { return s.tlsConfig }

// Handler serves the tenant-partitioned routes.
func (s *Service) Handler() http.Handler {
	routes := http.NewServeMux()
	routes.HandleFunc("POST /oauth2/v2.0/token", s.scoped(s.token))
	routes.HandleFunc("GET /metadata/identity/oauth2/token", s.scoped(s.imdsToken))
	routes.HandleFunc("GET /test/requests", s.scoped(s.recorded))
	routes.HandleFunc("GET /test/managed-identity/{identity}", s.scoped(s.imdsRecorded))
	for path, audience := range map[string]string{"/foundry": FoundryAudience, "/cognitive": CognitiveAudience} {
		routes.HandleFunc(path, s.scoped(resource(audience)))
		routes.HandleFunc(path+"/", s.scoped(resource(audience)))
	}
	return testbench.NormalizeMethod(testbench.PartitionRouter(routes))
}

// scoped passes fn the tenant PartitionRouter took from the path, failing a request that has none.
func (s *Service) scoped(fn func(string, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenant, ok := testbench.PartitionKeyFromContext(r.Context())
		if !ok || tenant == "" {
			http.Error(w, "entra: request has no tenant", http.StatusInternalServerError)
			return
		}
		fn(tenant, w, r)
	}
}

// admission is an admitted request: its 1-based position in the partition, its 1-based position
// among the partition's well-formed requests (0 for a malformed one), its status, and its index in
// the partition's retained requests, or -1 when the retention cap left it out.
type admission struct {
	n, eligible, status, index int
}

// admit records a token request under key. A malformed request is answered 400; a well-formed one
// gets the status decide picks for its position among the partition's well-formed requests, so a
// malformed request never uses up a one-time answer. It reports false when the partition budget
// is exhausted.
func (s *Service) admit(key string, req Request, wellFormed bool, decide func(eligible int) int) (admission, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.partitions[key]
	if p == nil {
		if len(s.partitions) >= maxPartitions {
			return admission{}, false
		}
		p = &partition{}
		s.partitions[key] = p
	}
	p.count++
	a := admission{n: p.count, index: -1}
	if wellFormed {
		p.eligible++
		a.eligible = p.eligible
		req.Status = decide(p.eligible)
	} else {
		req.Status = http.StatusBadRequest
	}
	a.status = req.Status
	if len(p.requests) < maxRetainedRequests {
		a.index = len(p.requests)
		p.requests = append(p.requests, req)
	}
	return a, true
}

// abandon records that the caller of an admitted request left before it was answered: the
// request keeps its place in the log and count, with status 0.
func (s *Service) abandon(key string, a admission) {
	if a.index < 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if p := s.partitions[key]; p != nil {
		p.requests[a.index].Status = 0
	}
}

// recorded reports GET /test/requests as {"count": n, "requests": [...]}, oldest first.
func (s *Service) recorded(tenant string, w http.ResponseWriter, _ *http.Request) {
	s.writeLog(w, tenant)
}

// writeLog writes the request log kept under key as {"count": n, "requests": [...]}.
func (s *Service) writeLog(w http.ResponseWriter, key string) {
	s.mu.Lock()
	count, requests := 0, []Request{}
	if p := s.partitions[key]; p != nil {
		count = p.count
		requests = append(requests, p.requests...)
	}
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"count": count, "requests": requests})
}

// wait blocks for delay unless the request ends first, reporting whether the delay elapsed.
func wait(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// writeJSON writes payload as a JSON response with status.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("entra: failed to encode response: %v", err)
	}
}
