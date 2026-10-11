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
	"io"
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
