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

// Package moesif provides a block-partitioned, stateful Moesif management API for the API Portal's
// billing.
//
// It answers the Moesif calls the API Portal makes: the billing catalog (plans and prices), billing
// meters, the connection to Stripe, and the usage search. Catalog prices are created in the
// testbench's Stripe service, as Moesif syncs its catalog to Stripe, and the usage a billing meter
// counts is reported to Stripe against the meter's price. It also receives the Stripe events sent to
// Moesif's Stripe endpoint. Control endpoints under /test record counted usage, fail chosen
// requests, and report the stored state.
package moesif

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wso2/api-platform/tests/framework/testbench"
	"github.com/wso2/api-platform/tests/framework/testbench/services/stripe"
)

// Port is the container port used by the testbench.
const Port = 3023

const (
	maxRequestBodyBytes = 1 << 20
	maxRetainedRequests = 2048
	maxRetainedRecords  = 4096
)

// Billing is the Stripe account Moesif syncs its catalog to and reports counted usage to.
type Billing interface {
	// AddPrice adds a catalog price to the account of the partition key.
	AddPrice(key string, spec stripe.PriceSpec) error
	// ArchivePrice stops new subscriptions to a price.
	ArchivePrice(key, priceID string) error
	// RecordUsage adds usage to a subscription's metered price. The subscription is named by its
	// id or by a value in its metadata; its id is returned.
	RecordUsage(key, subscriptionRef, priceID string, quantity int64) (string, error)
}

// Service is the Moesif service. Each block has its own Moesif application.
type Service struct {
	mu           sync.Mutex
	applications map[string]*application
	billing      Billing
	now          func() time.Time
}

// New returns a Moesif service that syncs its catalog to billing.
func New(billing Billing) (*Service, error) {
	if billing == nil {
		return nil, errors.New("moesif: a billing account is required")
	}
	return &Service{applications: map[string]*application{}, billing: billing, now: time.Now}, nil
}

func (s *Service) Name() string { return "moesif" }

func (s *Service) Port() int { return Port }

func (s *Service) Stateful() bool { return true }

func (s *Service) PartitionKey() string { return testbench.PartitionByBlock }

func (s *Service) Handler() http.Handler {
	routes := http.NewServeMux()
	routes.HandleFunc("GET /v1/~/billing/catalog/plans", s.api(s.listPlans))
	routes.HandleFunc("POST /v1/~/billing/catalog/plans", s.api(s.createPlan))
	routes.HandleFunc("PUT /v1/~/billing/catalog/plans/{id}", s.api(s.archivePlan))
	routes.HandleFunc("POST /v1/~/billing/catalog/prices", s.api(s.createPrice))
	routes.HandleFunc("PUT /v1/~/billing/catalog/prices/{id}", s.api(s.archivePrice))
	routes.HandleFunc("DELETE /v1/~/billing/catalog/prices/{id}", s.api(s.deletePrice))
	routes.HandleFunc("GET /v1/~/billing/meters", s.api(s.listMeters))
	routes.HandleFunc("POST /v1/~/billing/meters", s.api(s.createMeter))
	routes.HandleFunc("DELETE /v1/~/billing/meters/{id}", s.api(s.deleteMeter))
	routes.HandleFunc("GET /v1/~/billing/partneraccount/{provider}", s.api(s.getPartnerAccount))
	routes.HandleFunc("POST /v1/~/billing/partneraccount/{provider}", s.api(s.savePartnerAccount))
	routes.HandleFunc("DELETE /v1/alerting/~/partners/{provider}", s.api(s.deletePartnerAccount))
	routes.HandleFunc("POST /v1/search/~/search/events", s.api(s.searchEvents))
	routes.HandleFunc("POST /partners/stripe", s.scoped(s.receiveStripeEvent))

	routes.HandleFunc("POST /test/usage", s.scoped(s.recordUsage))
	routes.HandleFunc("POST /test/fail", s.scoped(s.addFailure))
	routes.HandleFunc("GET /test/state", s.scoped(s.state))
	routes.HandleFunc("POST /test/reset", s.scoped(s.reset))
	routes.HandleFunc("GET /test/health", s.scoped(s.health))
	return testbench.NormalizeMethod(testbench.PartitionRouter(routes))
}

// apiHandler serves a Moesif API call for an application with its JSON body, nil when empty.
type apiHandler func(key string, app *application, w http.ResponseWriter, r *http.Request, body map[string]any)

func (s *Service) scoped(fn func(string, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key, ok := testbench.PartitionKeyFromContext(r.Context())
		if !ok || key == "" {
			http.Error(w, "moesif: request has no partition", http.StatusInternalServerError)
			return
		}
		fn(key, w, r)
	}
}

// api records the call, applies a failure set for it, checks the management key and reads the
// JSON body.
func (s *Service) api(fn apiHandler) http.HandlerFunc {
	return s.scoped(func(key string, w http.ResponseWriter, r *http.Request) {
		raw, ok := readBody(w, r)
		if !ok {
			return
		}
		app := s.application(key)
		app.record(r.Method, r.URL.Path, r.URL.RawQuery)
		if f := app.takeFailure(r.Method, r.URL.Path, r.URL.RawQuery, raw); f != nil {
			writeError(w, f.Status, "Request failed by the testbench.")
			return
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || !validManagementKey(strings.TrimPrefix(auth, "Bearer ")) {
			writeError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		var body map[string]any
		if len(bytes.TrimSpace(raw)) > 0 {
			if err := json.Unmarshal(raw, &body); err != nil || body == nil {
				writeError(w, http.StatusBadRequest, "The request body is not a JSON object.")
				return
			}
		}
		fn(key, app, w, r, body)
	})
}

// validManagementKey reports whether a key is shaped as a Moesif management key: three parts, the
// middle one a base64url JSON object naming its application in "app".
func validManagementKey(key string) bool {
	parts := strings.Split(key, ".")
	return len(parts) == 3 && applicationClaim(parts[1]) != ""
}

// applicationClaim is the "app" claim of a base64url JSON segment, or "" when it has none.
func applicationClaim(segment string) string {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(segment, "="))
	if err != nil {
		return ""
	}
	var claims map[string]any
	if json.Unmarshal(raw, &claims) != nil {
		return ""
	}
	app, _ := claims["app"].(string)
	return app
}

func (s *Service) application(key string) *application {
	s.mu.Lock()
	defer s.mu.Unlock()
	app := s.applications[key]
	if app == nil {
		app = newApplication()
		s.applications[key] = app
	}
	return app
}

func (s *Service) reset(key string, w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	delete(s.applications, key)
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) health(_ string, w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok","service":"moesif"}`))
}

func readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	defer func() { _ = r.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBodyBytes+1))
	if err != nil {
		http.Error(w, "moesif: reading request body failed", http.StatusBadRequest)
		return nil, false
	}
	if len(body) > maxRequestBodyBytes {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return nil, false
	}
	return body, true
}

// writeError answers in Moesif's error format.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"message": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// newID returns a random identifier with the given prefix.
func newID(prefix string) string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		panic("moesif: reading random bytes: " + err.Error())
	}
	return prefix + hex.EncodeToString(buf)
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
