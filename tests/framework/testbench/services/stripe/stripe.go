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

// Package stripe provides a block-partitioned, stateful Stripe service for the API Portal's billing.
//
// It answers the Stripe calls the API Portal makes (customers, Checkout Sessions, subscriptions,
// invoices, webhook endpoints, the billing page and refunds) and sends signed Stripe events to the
// webhook endpoints registered with it. Control endpoints under /test act as the customer and as
// Stripe: they pay a checkout or let it expire, change a subscription's payment status, add invoices,
// resend an event, fail chosen requests, and report the stored state.
package stripe

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/wso2/api-platform/tests/framework/testbench"
)

// Port is the container port used by the testbench.
const Port = 3022

const (
	maxRequestBodyBytes = 1 << 20
	maxRetainedRequests = 2048
	maxRetainedEvents   = 1024
	deliveryTimeout     = 15 * time.Second
)

var apiKeyPattern = regexp.MustCompile(`^(sk|rk)_test_[A-Za-z0-9]+$`)

// Service is the Stripe service. Each block has its own account.
type Service struct {
	mu       sync.Mutex
	accounts map[string]*account
	client   *http.Client
	now      func() time.Time
	// sending counts the event deliveries still running in the background.
	sending sync.WaitGroup
}

// New returns a Stripe service with no accounts.
func New() *Service {
	return &Service{
		accounts: map[string]*account{},
		client: &http.Client{
			Timeout: deliveryTimeout,
			// The API Portal serves a self-signed certificate inside the test network.
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, //nolint:gosec
		},
		now: time.Now,
	}
}

func (s *Service) Name() string { return "stripe" }

func (s *Service) Port() int { return Port }

func (s *Service) Stateful() bool { return true }

func (s *Service) PartitionKey() string { return testbench.PartitionByBlock }

func (s *Service) Handler() http.Handler {
	routes := http.NewServeMux()
	routes.HandleFunc("GET /v1/webhook_endpoints", s.api(s.listWebhookEndpoints))
	routes.HandleFunc("POST /v1/webhook_endpoints", s.api(s.createWebhookEndpoint))
	routes.HandleFunc("POST /v1/webhook_endpoints/{id}", s.api(s.updateWebhookEndpoint))
	routes.HandleFunc("DELETE /v1/webhook_endpoints/{id}", s.api(s.deleteWebhookEndpoint))
	routes.HandleFunc("POST /v1/customers", s.api(s.createCustomer))
	routes.HandleFunc("POST /v1/checkout/sessions", s.api(s.createCheckoutSession))
	routes.HandleFunc("GET /v1/checkout/sessions/{id}", s.api(s.getCheckoutSession))
	routes.HandleFunc("POST /v1/checkout/sessions/{id}/expire", s.api(s.expireCheckoutSession))
	routes.HandleFunc("GET /v1/subscriptions/{id}", s.api(s.getSubscription))
	routes.HandleFunc("POST /v1/subscriptions/{id}", s.api(s.updateSubscription))
	routes.HandleFunc("DELETE /v1/subscriptions/{id}", s.api(s.cancelSubscription))
	routes.HandleFunc("GET /v1/invoices", s.api(s.listInvoices))
	routes.HandleFunc("GET /v1/invoices/{id}", s.api(s.getInvoice))
	routes.HandleFunc("GET /v1/invoices/{id}/lines", s.api(s.listInvoiceLines))
	routes.HandleFunc("POST /v1/invoices/create_preview", s.api(s.previewInvoice))
	routes.HandleFunc("POST /v1/refunds", s.api(s.createRefund))
	routes.HandleFunc("GET /v1/billing_portal/configurations", s.api(s.listPortalConfigurations))
	routes.HandleFunc("POST /v1/billing_portal/configurations", s.api(s.createPortalConfiguration))
	routes.HandleFunc("POST /v1/billing_portal/sessions", s.api(s.createPortalSession))

	routes.HandleFunc("POST /test/checkout-sessions/{id}/complete", s.control(s.completeCheckout))
	routes.HandleFunc("POST /test/checkout-sessions/{id}/expire", s.control(s.abandonCheckout))
	routes.HandleFunc("POST /test/subscriptions/{id}", s.control(s.setSubscriptionStatus))
	routes.HandleFunc("POST /test/subscriptions/{id}/period-end", s.control(s.endPeriod))
	routes.HandleFunc("POST /test/invoices", s.control(s.addInvoice))
	routes.HandleFunc("POST /test/events/resend", s.control(s.resendEvent))
	routes.HandleFunc("POST /test/fail", s.control(s.addFailure))
	routes.HandleFunc("GET /test/state", s.control(s.state))
	routes.HandleFunc("POST /test/reset", s.scoped(s.reset))
	routes.HandleFunc("GET /test/health", s.scoped(s.health))
	return testbench.NormalizeMethod(testbench.PartitionRouter(routes))
}

// apiHandler serves a Stripe API call for an account; form holds the request's parameters, from
// the form-encoded body and the query string.
type apiHandler func(acct *account, w http.ResponseWriter, r *http.Request, form map[string]any)

// controlHandler serves a /test call for an account with its JSON body.
type controlHandler func(acct *account, w http.ResponseWriter, r *http.Request, body []byte)

func (s *Service) scoped(fn func(string, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key, ok := testbench.PartitionKeyFromContext(r.Context())
		if !ok || key == "" {
			http.Error(w, "stripe: request has no partition", http.StatusInternalServerError)
			return
		}
		fn(key, w, r)
	}
}

// api checks the key, records the call, applies a failure set for it, and parses its parameters.
func (s *Service) api(fn apiHandler) http.HandlerFunc {
	return s.scoped(func(key string, w http.ResponseWriter, r *http.Request) {
		body, ok := readBody(w, r)
		if !ok {
			return
		}
		acct := s.account(key)
		acct.record(r.Method, r.URL.Path, r.URL.RawQuery)
		if f := acct.takeFailure(r.Method, r.URL.Path, r.URL.RawQuery, body); f != nil {
			writeError(w, f.Status, "api_error", "", "Request failed by the testbench.")
			return
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || !apiKeyPattern.MatchString(strings.TrimPrefix(auth, "Bearer ")) {
			writeError(w, http.StatusUnauthorized, "invalid_request_error", "", "Invalid API Key provided.")
			return
		}
		form, err := parseParams(r.URL.RawQuery, body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request_error", "parameter_invalid", err.Error())
			return
		}
		fn(acct, w, r, form)
	})
}

func (s *Service) control(fn controlHandler) http.HandlerFunc {
	return s.scoped(func(key string, w http.ResponseWriter, r *http.Request) {
		body, ok := readBody(w, r)
		if !ok {
			return
		}
		if len(body) > 0 && !json.Valid(body) {
			http.Error(w, "stripe: control request body is not JSON", http.StatusBadRequest)
			return
		}
		fn(s.account(key), w, r, body)
	})
}

func (s *Service) account(key string) *account {
	s.mu.Lock()
	defer s.mu.Unlock()
	acct := s.accounts[key]
	if acct == nil {
		acct = newAccount()
		s.accounts[key] = acct
	}
	return acct
}

func (s *Service) reset(key string, w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	delete(s.accounts, key)
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) health(_ string, w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok","service":"stripe"}`))
}

func readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	defer func() { _ = r.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBodyBytes+1))
	if err != nil {
		http.Error(w, "stripe: reading request body failed", http.StatusBadRequest)
		return nil, false
	}
	if len(body) > maxRequestBodyBytes {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return nil, false
	}
	return body, true
}

// writeError answers in Stripe's error format.
func writeError(w http.ResponseWriter, status int, kind, code, message string) {
	detail := map[string]any{"type": kind, "message": message}
	if code != "" {
		detail["code"] = code
	}
	writeJSON(w, status, map[string]any{"error": detail})
}

func missing(w http.ResponseWriter, kind, id string) {
	writeError(w, http.StatusNotFound, "invalid_request_error", "resource_missing",
		"No such "+kind+": '"+id+"'")
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// newID returns a Stripe-style identifier with the given prefix.
func newID(prefix string) string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		panic("stripe: reading random bytes: " + err.Error())
	}
	return prefix + "_test_" + hex.EncodeToString(buf)
}

// failure fails matching requests with a status, a number of times.
type failure struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Match  string `json:"match"`
	Status int    `json:"status"`
	Times  int    `json:"times"`
}

func (f *failure) matches(method, path, query string, body []byte) bool {
	if f.Method != "" && !strings.EqualFold(f.Method, method) {
		return false
	}
	if f.Path != "" && f.Path != path {
		return false
	}
	if f.Match == "" {
		return true
	}
	return strings.Contains(path, f.Match) || strings.Contains(query, f.Match) || strings.Contains(string(body), f.Match)
}

func (f *failure) validate() error {
	switch {
	case f.Method == "" && f.Path == "" && f.Match == "":
		return errors.New("a failure needs a method, a path or a match")
	case f.Status < 400 || f.Status > 599:
		return errors.New("a failure status is 400 to 599")
	case f.Times < 0:
		return errors.New("a failure's times is not negative")
	}
	if f.Times == 0 {
		f.Times = 1
	}
	return nil
}
