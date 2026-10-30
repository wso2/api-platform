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

package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/authenticators"
	commonmodels "github.com/wso2/api-platform/common/models"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	auditservice "github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/audit"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/restapi"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// auditRecorder is a store that keeps every row the audit DB sink writes; every
// other Storage method is unused.
type auditRecorder struct {
	storage.Storage
	rows chan *models.AuditLog
}

// newAuditRecorder returns an enabled AuditService whose DB sink writes to the returned recorder.
func newAuditRecorder() (*auditservice.AuditService, *auditRecorder) {
	r := &auditRecorder{rows: make(chan *models.AuditLog, 32)}
	return auditservice.NewAuditService(config.AuditConfig{Enabled: true}, "test-gateway", r, slog.New(slog.NewTextHandler(io.Discard, nil))), r
}

func (r *auditRecorder) CreateAuditLog(row *models.AuditLog) error {
	r.rows <- row
	return nil
}

// next returns the next recorded row. Record writes asynchronously, so it waits briefly.
func (r *auditRecorder) next(t *testing.T) *models.AuditLog {
	t.Helper()
	select {
	case e := <-r.rows:
		return e
	case <-time.After(2 * time.Second):
		t.Fatal("expected an audit event, got none")
		return nil
	}
}

// none fails the test when an event is recorded within a short wait.
func (r *auditRecorder) none(t *testing.T) {
	t.Helper()
	select {
	case e := <-r.rows:
		t.Fatalf("expected no audit event, got %s %s %s", e.Action, e.ResourceType, e.Outcome)
	case <-time.After(200 * time.Millisecond):
	}
}

func auditedServer() (*APIServer, *MockStorage, *auditRecorder) {
	db := NewMockStorage()
	server := createTestAPIServerWithDB(db)
	audit, rec := newAuditRecorder()
	server.SetAuditService(audit)
	return server, db, rec
}

// asAlice adds the authenticated user the auth middleware would set.
func asAlice(r *http.Request) *http.Request {
	return r.WithContext(authenticators.WithAuthContext(r.Context(), commonmodels.AuthContext{UserID: "alice"}))
}

func TestAudit_SubscriptionPlan(t *testing.T) {
	server, db, rec := auditedServer()

	w, req := createTestContext(http.MethodPost, "/subscription-plans",
		[]byte(`{"planName":"gold","status":"ACTIVE","stopOnQuotaReach":true,"throttleLimitCount":100,"throttleLimitUnit":"Min"}`))
	server.CreateSubscriptionPlan(w, asAlice(req))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created struct{ ID string }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	e := rec.next(t)
	assert.Equal(t, models.AuditActionCreate, e.Action)
	assert.Equal(t, models.AuditResourceTypeSubscriptionPlan, e.ResourceType)
	assert.Equal(t, "gold", e.ResourceName)
	assert.Equal(t, "alice", e.PerformedBy)
	assert.JSONEq(t, `{"name":"gold","status":"ACTIVE","stopOnQuotaReach":true,
		"throttleLimitCount":100,"throttleLimitUnit":"Min","billingPlan":null,"expiryTime":null}`, e.EventData)

	w, req = createTestContext(http.MethodPut, "/subscription-plans/"+created.ID,
		[]byte(`{"throttleLimitCount":200,"throttleLimitUnit":"Min","status":"INACTIVE"}`))
	server.UpdateSubscriptionPlan(w, asAlice(req), created.ID)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	e = rec.next(t)
	assert.Equal(t, models.AuditActionUpdate, e.Action)
	assert.JSONEq(t, `{
		"status":             {"from":"ACTIVE","to":"INACTIVE"},
		"throttleLimitCount": {"from":100,"to":200}
	}`, e.EventData)

	db.updateErr = errors.New("db down")
	w, req = createTestContext(http.MethodPut, "/subscription-plans/"+created.ID, []byte(`{"status":"ACTIVE"}`))
	server.UpdateSubscriptionPlan(w, asAlice(req), created.ID)
	require.Equal(t, http.StatusInternalServerError, w.Code)
	e = rec.next(t)
	assert.Equal(t, models.AuditActionUpdate, e.Action)
	assert.Equal(t, models.AuditOutcomeFailed, e.Outcome)

	db.deleteErr = errors.New("db down")
	w, req = createTestContext(http.MethodDelete, "/subscription-plans/"+created.ID, nil)
	server.DeleteSubscriptionPlan(w, asAlice(req), created.ID)
	require.Equal(t, http.StatusInternalServerError, w.Code)
	e = rec.next(t)
	assert.Equal(t, models.AuditActionDelete, e.Action)
	assert.Equal(t, models.AuditOutcomeFailed, e.Outcome)

	db.deleteErr = nil
	w, req = createTestContext(http.MethodDelete, "/subscription-plans/unknown", nil)
	server.DeleteSubscriptionPlan(w, asAlice(req), "unknown")
	require.Equal(t, http.StatusNotFound, w.Code)
	rec.none(t)
}

func TestAudit_Subscription(t *testing.T) {
	server, db, rec := auditedServer()
	require.NoError(t, db.SaveConfig(&models.StoredConfig{UUID: "api-1", Kind: "RestApi", Handle: "orders-api"}))

	create := func() (string, int) {
		w, req := createTestContext(http.MethodPost, "/subscriptions",
			[]byte(`{"apiId":"orders-api","subscriptionToken":"tok-0123456789abcdef0123456789abcdef","status":"ACTIVE"}`))
		server.CreateSubscription(w, asAlice(req))
		var created struct{ ID string }
		_ = json.Unmarshal(w.Body.Bytes(), &created)
		return created.ID, w.Code
	}

	id, code := create()
	require.Equal(t, http.StatusCreated, code)
	e := rec.next(t)
	assert.Equal(t, models.AuditActionCreate, e.Action)
	assert.Equal(t, models.AuditResourceTypeSubscription, e.ResourceType)
	assert.Equal(t, id, e.ResourceUUID)
	assert.JSONEq(t, `{"apiHandle":"orders-api","status":"ACTIVE"}`, e.EventData)

	w, req := createTestContext(http.MethodPut, "/subscriptions/"+id, []byte(`{"status":"INACTIVE"}`))
	server.UpdateSubscription(w, asAlice(req), id)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	e = rec.next(t)
	assert.Equal(t, models.AuditActionUpdate, e.Action)
	assert.JSONEq(t, `{"status":{"from":"ACTIVE","to":"INACTIVE"}}`, e.EventData)

	db.saveErr = errors.New("db down")
	_, code = create()
	require.Equal(t, http.StatusInternalServerError, code)
	e = rec.next(t)
	assert.Equal(t, models.AuditActionCreate, e.Action)
	assert.Equal(t, models.AuditOutcomeFailed, e.Outcome)
}

func TestAudit_CertificateUploadFailure(t *testing.T) {
	server, db, rec := auditedServer()
	db.saveErr = errors.New("db down")

	body, err := json.Marshal(map[string]string{"name": "partner-ca", "certificate": validTestCert})
	require.NoError(t, err)
	w, req := createTestContext(http.MethodPost, "/certificates", body)
	server.UploadCertificate(w, asAlice(req))
	require.Equal(t, http.StatusInternalServerError, w.Code)

	e := rec.next(t)
	assert.Equal(t, models.AuditActionCreate, e.Action)
	assert.Equal(t, models.AuditResourceTypeCertificate, e.ResourceType)
	assert.Equal(t, models.AuditOutcomeFailed, e.Outcome)
	assert.Equal(t, "partner-ca", e.ResourceName)
	assert.Equal(t, "alice", e.PerformedBy)
}

func TestAudit_RestAPI(t *testing.T) {
	server, db, rec := auditedServer()
	server.restAPIService.SetAuditService(server.auditService)
	logger := server.logger
	yaml := func(displayName, operationPolicies string) []byte {
		return []byte(`
apiVersion: gateway.api-platform.wso2.com/v1
kind: RestApi
metadata:
  name: orders-api
spec:
  displayName: ` + displayName + `
  version: v1
  context: /orders
  upstream:
    main:
      url: http://localhost:5000
  operations:
    - method: GET
      path: /items` + operationPolicies + `
`)
	}
	rateLimit := `
      policies:
        - name: rate-limit
          version: v1
          params:
            limit: 10`

	_, err := server.restAPIService.Create(restapi.CreateParams{Body: yaml("Orders", ""), ContentType: "application/yaml", Actor: "alice", Logger: logger})
	require.NoError(t, err)
	e := rec.next(t)
	assert.Equal(t, models.AuditActionCreate, e.Action)
	assert.Equal(t, models.AuditResourceTypeRestAPI, e.ResourceType)
	assert.Equal(t, "orders-api", e.ResourceName)
	assert.JSONEq(t, `{"displayName":"Orders","version":"v1","desiredState":"deployed","policies":[],"faultPolicies":[]}`, e.EventData)

	_, err = server.restAPIService.Update(restapi.UpdateParams{Handle: "orders-api", Body: yaml("Orders 2", ""), ContentType: "application/yaml", Actor: "alice", Logger: logger})
	require.NoError(t, err)
	e = rec.next(t)
	assert.Equal(t, models.AuditActionUpdate, e.Action)
	assert.JSONEq(t, `{"displayName":{"from":"Orders","to":"Orders 2"}}`, e.EventData)

	// Attaching a policy is an update whose diff names the policy; its params are never recorded.
	_, err = server.restAPIService.Update(restapi.UpdateParams{Handle: "orders-api", Body: yaml("Orders 2", rateLimit), ContentType: "application/yaml", Actor: "alice", Logger: logger})
	require.NoError(t, err)
	e = rec.next(t)
	assert.JSONEq(t, `{"policies":{"from":[],"to":["rate-limit v1 (GET /items)"]}}`, e.EventData)

	_, err = server.restAPIService.Update(restapi.UpdateParams{Handle: "orders-api", Body: yaml("Orders 2", ""), ContentType: "application/yaml", Actor: "alice", Logger: logger})
	require.NoError(t, err)
	e = rec.next(t)
	assert.JSONEq(t, `{"policies":{"from":["rate-limit v1 (GET /items)"],"to":[]}}`, e.EventData)

	db.updateErr = errors.New("db down")
	_, err = server.restAPIService.Update(restapi.UpdateParams{Handle: "orders-api", Body: yaml("Orders 3", ""), ContentType: "application/yaml", Actor: "alice", Logger: logger})
	require.Error(t, err)
	e = rec.next(t)
	assert.Equal(t, models.AuditActionUpdate, e.Action)
	assert.Equal(t, models.AuditOutcomeFailed, e.Outcome)

	db.updateErr = nil
	_, err = server.restAPIService.Delete(restapi.DeleteParams{Handle: "orders-api", Actor: "alice", Logger: logger})
	require.NoError(t, err)
	e = rec.next(t)
	assert.Equal(t, models.AuditActionDelete, e.Action)
	assert.Equal(t, models.AuditOutcomeSucceeded, e.Outcome)
}

// auditListStorage serves a fixed page of audit rows with a separate total, so
// the list response's count (this page) and pagination.total (all matches) differ.
type auditListStorage struct {
	*MockStorage
	rows  []*models.AuditLog
	total int
}

func (s *auditListStorage) ListAuditLogs(_ storage.AuditLogFilter, _, _ int) ([]*models.AuditLog, error) {
	return s.rows, nil
}
func (s *auditListStorage) CountAuditLogs(_ storage.AuditLogFilter) (int, error) { return s.total, nil }

func TestListAuditLogs_ResponseShape(t *testing.T) {
	now := time.Now().UTC()
	db := &auditListStorage{MockStorage: NewMockStorage(), total: 5, rows: []*models.AuditLog{
		{UUID: "e2", Action: models.AuditActionUpdate, ResourceType: models.AuditResourceTypeRestAPI, Outcome: models.AuditOutcomeSucceeded, PerformedAt: now},
		{UUID: "e1", Action: models.AuditActionCreate, ResourceType: models.AuditResourceTypeRestAPI, Outcome: models.AuditOutcomeSucceeded, PerformedAt: now},
	}}
	server := createTestAPIServerWithDB(db)

	type page struct {
		Count      int               `json:"count"`
		Events     []json.RawMessage `json:"events"`
		Pagination struct {
			Total  int `json:"total"`
			Offset int `json:"offset"`
			Limit  int `json:"limit"`
		} `json:"pagination"`
	}
	list := func(limit, offset int32) page {
		w := httptest.NewRecorder()
		server.ListAuditLogs(w, httptest.NewRequest(http.MethodGet, "/audit-logs", nil), api.ListAuditLogsParams{Limit: &limit, Offset: &offset})
		require.Equal(t, http.StatusOK, w.Code)
		var p page
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &p))
		return p
	}

	p := list(2, 3)
	assert.Equal(t, 2, p.Count)
	assert.Len(t, p.Events, 2)
	assert.Equal(t, 5, p.Pagination.Total)
	assert.Equal(t, 3, p.Pagination.Offset)
	assert.Equal(t, 2, p.Pagination.Limit)

	// An oversized limit is capped, and the response says so.
	assert.Equal(t, auditMaxLimit, list(500, 0).Pagination.Limit)
}
