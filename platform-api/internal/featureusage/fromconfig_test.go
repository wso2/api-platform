/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package featureusage

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wso2/api-platform/httpkit/httpclient"
	"github.com/wso2/api-platform/httpkit/netguard"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// initSharedClient builds the process-wide outbound client the way the server
// does by default: SSRF-guarded, private addresses reachable, metadata not.
func initSharedClient(t *testing.T) {
	t.Helper()
	initSharedClientWithPolicy(t, netguard.PermitPrivateBlockMetadata())
}

// initSharedClientWithPolicy builds the process-wide outbound client under policy,
// e.g. netguard.PublicOnly() for platform_api.http_client.ssrf.preset = "public_only".
func initSharedClientWithPolicy(t *testing.T, policy netguard.Policy) {
	t.Helper()
	cfg := httpclient.DefaultConfig()
	cfg.SSRF.Enabled = true
	cfg.SSRF.Policy = policy
	cfg.SSRF.Policy.AllowedSchemes = []string{"http", "https"}
	cfg.SSRF.MaxRedirects = 5
	if err := utils.InitSharedHTTPClientFromConfig(cfg, 0); err != nil {
		t.Fatal(err)
	}
}

// wiring is a fake Platform IdP plus a fake collector proxy that accepts only
// the token the IdP issued.
type wiring struct {
	idpCalls       atomic.Int32
	collectorCalls atomic.Int32
	idpURL         string
	collectorURL   string
	collectorReply func(w http.ResponseWriter, r *http.Request)
}

func newWiring(t *testing.T) *wiring {
	t.Helper()
	wr := &wiring{}
	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wr.idpCalls.Add(1)
		if id, secret, ok := r.BasicAuth(); !ok || id != "platform-api" || secret != "m2m-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"idp-issued","token_type":"Bearer","expires_in":900}`))
	}))
	t.Cleanup(idp.Close)
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wr.collectorCalls.Add(1)
		if r.Header.Get("Authorization") != "Bearer idp-issued" || r.URL.Path != "/moesif-collector/v1/events" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if wr.collectorReply != nil {
			wr.collectorReply(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(collector.Close)
	wr.idpURL = idp.URL + "/oauth2/token"
	wr.collectorURL = collector.URL + "/moesif-collector"
	return wr
}

func (wr *wiring) config(collectorURL string) config.FeatureUsage {
	return config.FeatureUsage{
		Enabled:      true,
		CollectorURL: collectorURL,
		Token: config.FeatureUsageToken{
			TokenURL: wr.idpURL, ClientID: "platform-api", ClientSecret: "m2m-secret",
		},
		QueueSize:      4,
		Workers:        1,
		RequestTimeout: 2 * time.Second,
		MaxAttempts:    2,
		RetryBackoff:   10 * time.Millisecond,
		OverallTimeout: 5 * time.Second,
	}
}

func deliverOne(t *testing.T, cfg config.FeatureUsage) string {
	t.Helper()
	logs := &syncBuffer{}
	reporter, err := NewFromConfig(cfg, fakeAttributor{companyID: testCompanyUUID},
		slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	if err != nil {
		t.Fatal(err)
	}
	if _, isPublisher := reporter.(*Publisher); !isPublisher {
		t.Fatalf("enabled reporting returned %T, want *Publisher", reporter)
	}
	reporter.Record(validCreation())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := reporter.Close(ctx); err != nil {
		t.Fatal(err)
	}
	return logs.String()
}

// The full path a deployment takes: client-credentials token from the IdP,
// then the event posted with it through the shared, SSRF-guarded client.
func TestNewFromConfigDeliversThroughTheSharedClient(t *testing.T) {
	initSharedClient(t)
	wr := newWiring(t)
	logs := deliverOne(t, wr.config(wr.collectorURL))

	if wr.idpCalls.Load() != 1 || wr.collectorCalls.Load() != 1 {
		t.Fatalf("idp calls = %d, collector calls = %d; want 1 and 1", wr.idpCalls.Load(), wr.collectorCalls.Load())
	}
	if strings.Contains(logs, "REPORT_DROPPED") || strings.Contains(logs, "m2m-secret") || strings.Contains(logs, "idp-issued") {
		t.Errorf("unexpected logs: %s", logs)
	}
}

// The collector and token endpoint are operator-configured and usually in-cluster, so
// a "public_only" shared client, meant for tenant-supplied URLs, must not stop
// delivery to a private or loopback collector.
func TestNewFromConfigReachesAPrivateCollectorUnderPublicOnly(t *testing.T) {
	initSharedClientWithPolicy(t, netguard.PublicOnly())
	t.Cleanup(func() { initSharedClient(t) })
	wr := newWiring(t)
	logs := deliverOne(t, wr.config(wr.collectorURL))

	if wr.idpCalls.Load() != 1 || wr.collectorCalls.Load() != 1 {
		t.Fatalf("idp calls = %d, collector calls = %d; want 1 and 1", wr.idpCalls.Load(), wr.collectorCalls.Load())
	}
	if strings.Contains(logs, "REPORT_DROPPED") {
		t.Errorf("unexpected drop: %s", logs)
	}
}

// Relaxing the policy for operator destinations stops at private addresses:
// cloud metadata is still refused under "public_only".
func TestNewFromConfigRefusesMetadataAddressesUnderPublicOnly(t *testing.T) {
	initSharedClientWithPolicy(t, netguard.PublicOnly())
	t.Cleanup(func() { initSharedClient(t) })
	wr := newWiring(t)
	logs := deliverOne(t, wr.config("http://169.254.169.254/moesif-collector"))
	if !strings.Contains(logs, "REPORT_DROPPED") {
		t.Errorf("want the event dropped, got: %s", logs)
	}
}

// Both requests carry credentials, so a redirect is never followed.
func TestNewFromConfigDoesNotFollowRedirects(t *testing.T) {
	initSharedClient(t)
	wr := newWiring(t)
	var followed atomic.Int32
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { followed.Add(1) }))
	defer elsewhere.Close()
	wr.collectorReply = func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL+"/v1/events", http.StatusTemporaryRedirect)
	}

	logs := deliverOne(t, wr.config(wr.collectorURL))
	if followed.Load() != 0 {
		t.Fatal("the reporter followed a redirect with the bearer token")
	}
	if !strings.Contains(logs, "REPORT_DROPPED") || !strings.Contains(logs, "status 307") {
		t.Errorf("want a drop for the redirect, got: %s", logs)
	}
}

// A misconfigured collector URL pointing at cloud metadata is refused at dial
// time by the shared client's SSRF guard.
func TestNewFromConfigRefusesMetadataAddresses(t *testing.T) {
	initSharedClient(t)
	wr := newWiring(t)
	logs := deliverOne(t, wr.config("http://169.254.169.254/moesif-collector"))
	if !strings.Contains(logs, "REPORT_DROPPED") {
		t.Errorf("want the event dropped, got: %s", logs)
	}
	if strings.Contains(logs, "169.254.169.254") {
		t.Errorf("the drop log should not echo the dialed address: %s", logs)
	}
}
