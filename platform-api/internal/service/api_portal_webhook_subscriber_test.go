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

package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

type seenRequest struct {
	method string
	path   string
	auth   string
	body   webhookSubscriberBody
}

// newSeedTestServer records each request and answers with the next status
// from statuses (the last one repeats).
func newSeedTestServer(t *testing.T, tlsServer bool, statuses ...int) (*httptest.Server, *[]seenRequest) {
	t.Helper()
	seen := &[]seenRequest{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var b webhookSubscriberBody
		_ = json.Unmarshal(raw, &b)
		*seen = append(*seen, seenRequest{r.Method, r.URL.Path, r.Header.Get("Authorization"), b})
		idx := len(*seen) - 1
		if idx >= len(statuses) {
			idx = len(statuses) - 1
		}
		if statuses[idx] >= 300 && statuses[idx] < 400 {
			w.Header().Set("Location", "/redirected")
		}
		w.WriteHeader(statuses[idx])
		_, _ = w.Write([]byte(`{"detail":"from test server"}`))
	})
	var srv *httptest.Server
	if tlsServer {
		srv = httptest.NewTLSServer(handler)
	} else {
		srv = httptest.NewServer(handler)
	}
	t.Cleanup(srv.Close)
	return srv, seen
}

var seedTestWebhookCfg = config.Webhook{
	Enabled:             true,
	Secret:              "s3cret",
	AutoSeedSubscribers: true,
	ReceiverURL:         "https://platform-api.example.com/api/internal/v0.9/webhook/events",
}

// newSeedTestService wires a real auth registry so the shared-key header is
// built from the stored ciphertext, as in production.
func newSeedTestService(t *testing.T, repo *mockAPIPortalRepository, auditRepo *mockAPIPortalAuditRepository, client *http.Client) *APIPortalService {
	t.Helper()
	v := newTestVault(t)
	reg := NewAPIPortalAuthRegistry(repo, v)
	return NewAPIPortalService(repo, &mockAPIPortalOrgRepository{result: &model.Organization{}}, auditRepo,
		v, reg, nil, seedTestWebhookCfg, client, nil)
}

func seededPortal(t *testing.T, url string) *model.APIPortal {
	t.Helper()
	ct, err := newTestVault(t).Encrypt(context.Background(), testSharedKeyHex)
	if err != nil {
		t.Fatalf("seed encrypt: %v", err)
	}
	return &model.APIPortal{ID: "p-1", Handle: "acme", OrganizationID: "org-1", URL: url, InternalAuthKey: ct}
}

func TestEnsureWebhookSubscriberOnPortal_CreatedOnPost(t *testing.T) {
	srv, seen := newSeedTestServer(t, false, http.StatusCreated)
	repo := &mockAPIPortalRepository{}
	repo.getResult = seededPortal(t, srv.URL)
	svc := newSeedTestService(t, repo, &mockAPIPortalAuditRepository{}, srv.Client())

	if err := svc.EnsureWebhookSubscriberOnPortal(context.Background(), "acme", "org-1"); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if len(*seen) != 1 {
		t.Fatalf("want 1 request, got %d", len(*seen))
	}
	got := (*seen)[0]
	if got.method != http.MethodPost || got.path != webhookSubscribersPath {
		t.Errorf("want POST %s, got %s %s", webhookSubscribersPath, got.method, got.path)
	}
	if got.auth == "" {
		t.Error("shared-key Authorization header not sent")
	}
	if got.body.ID != platformAPISubscriberID || got.body.TargetURL != seedTestWebhookCfg.ReceiverURL ||
		got.body.Secret != seedTestWebhookCfg.Secret || !got.body.Enabled {
		t.Errorf("unexpected subscriber body: %+v", got.body)
	}
}

func TestEnsureWebhookSubscriberOnPortal_ConflictFallsBackToPut(t *testing.T) {
	srv, seen := newSeedTestServer(t, false, http.StatusConflict, http.StatusOK)
	repo := &mockAPIPortalRepository{}
	repo.getResult = seededPortal(t, srv.URL)
	svc := newSeedTestService(t, repo, &mockAPIPortalAuditRepository{}, srv.Client())

	if err := svc.EnsureWebhookSubscriberOnPortal(context.Background(), "acme", "org-1"); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if len(*seen) != 2 {
		t.Fatalf("want POST then PUT, got %d requests", len(*seen))
	}
	put := (*seen)[1]
	if put.method != http.MethodPut || put.path != webhookSubscribersPath+"/"+platformAPISubscriberID {
		t.Errorf("want PUT %s/%s, got %s %s", webhookSubscribersPath, platformAPISubscriberID, put.method, put.path)
	}
}

func TestEnsureWebhookSubscriberOnPortal_NonSuccessIsError(t *testing.T) {
	srv, seen := newSeedTestServer(t, false, http.StatusForbidden)
	repo := &mockAPIPortalRepository{}
	repo.getResult = seededPortal(t, srv.URL)
	svc := newSeedTestService(t, repo, &mockAPIPortalAuditRepository{}, srv.Client())

	err := svc.EnsureWebhookSubscriberOnPortal(context.Background(), "acme", "org-1")
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("want error carrying 403, got %v", err)
	}
	if len(*seen) != 1 {
		t.Errorf("non-409 failure must not retry with PUT, got %d requests", len(*seen))
	}
}

func TestEnsureWebhookSubscriberOnPortal_FailedPutIsError(t *testing.T) {
	srv, _ := newSeedTestServer(t, false, http.StatusConflict, http.StatusInternalServerError)
	repo := &mockAPIPortalRepository{}
	repo.getResult = seededPortal(t, srv.URL)
	svc := newSeedTestService(t, repo, &mockAPIPortalAuditRepository{}, srv.Client())

	err := svc.EnsureWebhookSubscriberOnPortal(context.Background(), "acme", "org-1")
	if err == nil || !strings.Contains(err.Error(), "PUT") || !strings.Contains(err.Error(), "500") {
		t.Fatalf("want PUT 500 error, got %v", err)
	}
}

func TestEnsureWebhookSubscriberOnPortal_NoOpWhenDisabled(t *testing.T) {
	repo := &mockAPIPortalRepository{}
	svc := NewAPIPortalService(repo, nil, nil, newTestVault(t), nil, nil, config.Webhook{Enabled: true}, nil, nil)
	// repo has no stubbed portal; reaching the lookup would return nil and error.
	if err := svc.EnsureWebhookSubscriberOnPortal(context.Background(), "acme", "org-1"); err != nil {
		t.Fatalf("want no-op when auto-seed is off, got %v", err)
	}
}

func TestCreateAPIPortal_SeedFailureRollsBackAndAudits(t *testing.T) {
	srv, _ := newSeedTestServer(t, true, http.StatusInternalServerError)
	repo := &mockAPIPortalRepository{}
	repo.getResult = seededPortal(t, srv.URL)
	auditRepo := &mockAPIPortalAuditRepository{}
	svc := newSeedTestService(t, repo, auditRepo, srv.Client())

	_, err := svc.CreateAPIPortal(testCreateReq{
		Handle: "acme", Name: "Acme", URL: srv.URL, SharedKey: testSharedKeyHex,
	}.build(), "org-1", "user-1")
	if err == nil {
		t.Fatal("want Create to fail closed when the seed fails")
	}
	if repo.createCapturedInput == nil || repo.deleteCalledWith[0] != repo.createCapturedInput.ID {
		t.Errorf("want rollback delete of the created row, got %v", repo.deleteCalledWith)
	}
	if len(auditRepo.records) != 2 || auditRepo.records[0].action != "CREATE" || auditRepo.records[1].action != "DELETE" {
		t.Errorf("want CREATE then DELETE audit records, got %+v", auditRepo.records)
	}
}

func TestEnsureWebhookSubscriberOnPortal_RedirectIsNotFollowed(t *testing.T) {
	srv, seen := newSeedTestServer(t, true, http.StatusTemporaryRedirect)
	repo := &mockAPIPortalRepository{}
	repo.getResult = seededPortal(t, srv.URL)
	// nil client exercises the service's own fallback.
	svc := newSeedTestService(t, repo, &mockAPIPortalAuditRepository{}, nil)
	svc.webhookHTTPClient.Transport = srv.Client().Transport

	err := svc.EnsureWebhookSubscriberOnPortal(context.Background(), "acme", "org-1")
	if err == nil || !strings.Contains(err.Error(), "307") {
		t.Fatalf("want redirect surfaced as a 307 failure, got %v", err)
	}
	if len(*seen) != 1 {
		t.Errorf("redirect must not be followed, got %d requests", len(*seen))
	}
}

func TestNewWebhookHTTPClient_RefusesRedirectsAndIsBounded(t *testing.T) {
	c := NewWebhookHTTPClient(false)
	if c.Timeout != webhookHTTPTimeout {
		t.Errorf("want timeout %v, got %v", webhookHTTPTimeout, c.Timeout)
	}
	if err := c.CheckRedirect(nil, nil); err != http.ErrUseLastResponse {
		t.Errorf("want redirects refused with ErrUseLastResponse, got %v", err)
	}
}

// A portal with a self-signed cert is reachable only when the operator opts in
// through platform_api.http_client.tls.insecure_skip_verify.
func TestNewWebhookHTTPClient_TLSVerificationIsOptIn(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	if _, err := NewWebhookHTTPClient(false).Get(srv.URL); err == nil {
		t.Error("want certificate verification failure against a self-signed portal by default")
	}
	resp, err := NewWebhookHTTPClient(true).Get(srv.URL)
	if err != nil {
		t.Fatalf("want self-signed portal reachable with insecure_skip_verify, got %v", err)
	}
	_ = resp.Body.Close()
}

// Each guard runs before any request is sent, so a misconfigured or missing
// portal never receives the shared key or the webhook secret.
func TestEnsureWebhookSubscriberOnPortal_PreconditionFailures(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.Webhook
		portal  func(t *testing.T, url string) *model.APIPortal
		getErr  error
		wantErr string
	}{
		{
			name:    "receiver url unset",
			cfg:     config.Webhook{Enabled: true, Secret: "s3cret", AutoSeedSubscribers: true},
			portal:  seededPortal,
			wantErr: "receiver_url is not configured",
		},
		{
			name:    "portal lookup fails",
			cfg:     seedTestWebhookCfg,
			getErr:  errors.New("db down"),
			wantErr: "db down",
		},
		{
			name:    "portal not found",
			cfg:     seedTestWebhookCfg,
			wantErr: "API_PORTAL_NOT_FOUND",
		},
		{
			name: "portal has no url",
			cfg:  seedTestWebhookCfg,
			portal: func(t *testing.T, _ string) *model.APIPortal {
				return seededPortal(t, "")
			},
			wantErr: "has no URL",
		},
		{
			name: "shared key cannot be decrypted",
			cfg:  seedTestWebhookCfg,
			portal: func(t *testing.T, url string) *model.APIPortal {
				p := seededPortal(t, url)
				p.InternalAuthKey = []byte("not-a-ciphertext")
				return p
			},
			wantErr: "shared-key auth header",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, seen := newSeedTestServer(t, false, http.StatusCreated)
			repo := &mockAPIPortalRepository{getErr: tc.getErr}
			if tc.portal != nil {
				repo.getResult = tc.portal(t, srv.URL)
			}
			v := newTestVault(t)
			svc := NewAPIPortalService(repo, nil, nil, v, NewAPIPortalAuthRegistry(repo, v), nil, tc.cfg, srv.Client(), nil)

			err := svc.EnsureWebhookSubscriberOnPortal(context.Background(), "acme", "org-1")
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
			if len(*seen) != 0 {
				t.Errorf("want no request sent to the portal, got %d", len(*seen))
			}
		})
	}
}

func TestEnsureWebhookSubscriberOnPortal_UnreachablePortalIsError(t *testing.T) {
	srv, _ := newSeedTestServer(t, false, http.StatusCreated)
	repo := &mockAPIPortalRepository{}
	repo.getResult = seededPortal(t, srv.URL)
	svc := newSeedTestService(t, repo, &mockAPIPortalAuditRepository{}, srv.Client())
	srv.Close()

	err := svc.EnsureWebhookSubscriberOnPortal(context.Background(), "acme", "org-1")
	if err == nil || !strings.Contains(err.Error(), "POST") {
		t.Fatalf("want POST transport error, got %v", err)
	}
}

func TestEnsureWebhookSubscriberOnPortal_InvalidPortalURLIsError(t *testing.T) {
	repo := &mockAPIPortalRepository{}
	repo.getResult = seededPortal(t, "http://bad host")
	svc := newSeedTestService(t, repo, &mockAPIPortalAuditRepository{}, nil)

	err := svc.EnsureWebhookSubscriberOnPortal(context.Background(), "acme", "org-1")
	if err == nil || !strings.Contains(err.Error(), "POST") {
		t.Fatalf("want POST request-build error, got %v", err)
	}
}

// The portal accepts the POST as a conflict and then drops the connection on the
// PUT; the update must surface as an error rather than a silent success.
func TestEnsureWebhookSubscriberOnPortal_PutTransportErrorIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusConflict)
			return
		}
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = conn.Close()
		}
	}))
	t.Cleanup(srv.Close)
	repo := &mockAPIPortalRepository{}
	repo.getResult = seededPortal(t, srv.URL)
	svc := newSeedTestService(t, repo, &mockAPIPortalAuditRepository{}, srv.Client())

	err := svc.EnsureWebhookSubscriberOnPortal(context.Background(), "acme", "org-1")
	if err == nil || !strings.Contains(err.Error(), "PUT") {
		t.Fatalf("want PUT transport error, got %v", err)
	}
}

func TestEnsureWebhookSubscriberOnPortal_LongErrorBodyIsTruncated(t *testing.T) {
	long := strings.Repeat("x", 2048)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(long))
	}))
	t.Cleanup(srv.Close)
	repo := &mockAPIPortalRepository{}
	repo.getResult = seededPortal(t, srv.URL)
	svc := newSeedTestService(t, repo, &mockAPIPortalAuditRepository{}, srv.Client())

	err := svc.EnsureWebhookSubscriberOnPortal(context.Background(), "acme", "org-1")
	if err == nil || !strings.Contains(err.Error(), "...(truncated)") {
		t.Fatalf("want truncated body in error, got %v", err)
	}
	if strings.Contains(err.Error(), long) {
		t.Error("full response body leaked into the error")
	}
}

// When the rollback delete itself fails the row is orphaned; the Create still
// fails, and no DELETE audit entry is written for a delete that did not happen.
func TestCreateAPIPortal_SeedFailureWithFailedRollbackKeepsAuditHonest(t *testing.T) {
	srv, _ := newSeedTestServer(t, true, http.StatusInternalServerError)
	repo := &mockAPIPortalRepository{deleteErr: errors.New("db down")}
	repo.getResult = seededPortal(t, srv.URL)
	auditRepo := &mockAPIPortalAuditRepository{}
	v := newTestVault(t)
	svc := NewAPIPortalService(repo, &mockAPIPortalOrgRepository{result: &model.Organization{}}, auditRepo,
		v, NewAPIPortalAuthRegistry(repo, v), nil, seedTestWebhookCfg, srv.Client(), slog.New(slog.NewTextHandler(io.Discard, nil)))

	_, err := svc.CreateAPIPortal(testCreateReq{
		Handle: "acme", Name: "Acme", URL: srv.URL, SharedKey: testSharedKeyHex,
	}.build(), "org-1", "user-1")
	if err == nil {
		t.Fatal("want Create to fail when the seed fails")
	}
	if len(auditRepo.records) != 1 || auditRepo.records[0].action != "CREATE" {
		t.Errorf("want only the CREATE audit record, got %+v", auditRepo.records)
	}
}
