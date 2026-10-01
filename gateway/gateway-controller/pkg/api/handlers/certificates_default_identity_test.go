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
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/testutil/pki"
)

// defaultIdentityRow is a stored role: default gateway identity.
func defaultIdentityRow(t *testing.T, id, name string) *models.StoredCertificate {
	t.Helper()
	leaf := gatewayIdentityLeaf(t, name)
	return &models.StoredCertificate{
		UUID: id, Name: name, Certificate: leaf.PEM(),
		Usage: models.CertificateUsageIdentity, Role: models.CertificateRoleDefault, KeyAlgorithm: "ECDSA",
		NotAfter: time.Now().Add(365 * 24 * time.Hour),
	}
}

func storedCertificateByName(t *testing.T, db *MockStorage, name string) *models.StoredCertificate {
	t.Helper()
	for _, cert := range db.certs {
		if cert.Name == name {
			return cert
		}
	}
	t.Fatalf("no stored certificate named %q", name)
	return nil
}

func TestUploadCertificate_RoleDefaultOnIdentity_Stored(t *testing.T) {
	mockDB := NewMockStorage()
	mockDB.certs = []*models.StoredCertificate{seedUpstreamCert(t)}
	server := createTestAPIServerWithIdentitySupport(t, mockDB)

	leaf := gatewayIdentityLeaf(t, "gateway-default")
	w := uploadCertificateBody(t, server, UploadCertificateRequest{
		Name: "gateway-default", Usage: models.CertificateUsageIdentity, Role: models.CertificateRoleDefault,
		Certificate: string(leaf.PEM()), PrivateKey: string(leaf.KeyPEM()),
	})

	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, models.CertificateRoleDefault, resp["role"])
	assert.True(t, storedCertificateByName(t, mockDB, "gateway-default").IsDefaultIdentity())
}

func TestUploadCertificate_IdentityWithoutRole_HasNoRole(t *testing.T) {
	mockDB := NewMockStorage()
	mockDB.certs = []*models.StoredCertificate{seedUpstreamCert(t)}
	server := createTestAPIServerWithIdentitySupport(t, mockDB)

	leaf := gatewayIdentityLeaf(t, "partner-identity")
	w := uploadCertificateBody(t, server, UploadCertificateRequest{
		Name: "partner-identity", Usage: models.CertificateUsageIdentity,
		Certificate: string(leaf.PEM()), PrivateKey: string(leaf.KeyPEM()),
	})

	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	_, hasRole := resp["role"]
	assert.False(t, hasRole, "an identity uploaded without a role reports none")
	assert.False(t, storedCertificateByName(t, mockDB, "partner-identity").IsDefaultIdentity())
}

func TestUploadCertificate_RoleAndUsageMismatch_Rejected(t *testing.T) {
	leaf := gatewayIdentityLeaf(t, "mismatch")
	authority := string(pki.NewRootCA(t, "Mismatch CA").PEM())

	tests := []struct {
		name    string
		req     UploadCertificateRequest
		message string
	}{
		{
			name:    "default on upstream",
			req:     UploadCertificateRequest{Name: "c", Usage: models.CertificateUsageUpstream, Role: models.CertificateRoleDefault, Certificate: authority},
			message: "role default applies only to usage: identity certificates",
		},
		{
			name:    "default on omitted usage",
			req:     UploadCertificateRequest{Name: "c", Role: models.CertificateRoleDefault, Certificate: authority},
			message: "role default applies only to usage: identity certificates",
		},
		{
			name:    "default on downstream",
			req:     UploadCertificateRequest{Name: "c", Usage: models.CertificateUsageDownstream, Role: models.CertificateRoleDefault, Certificate: authority},
			message: "role default applies only to usage: identity certificates",
		},
		{
			name: "client on identity",
			req: UploadCertificateRequest{Name: "c", Usage: models.CertificateUsageIdentity, Role: models.CertificateRoleClient,
				Certificate: string(leaf.PEM()), PrivateKey: string(leaf.KeyPEM())},
			message: "role client applies only to usage: downstream certificates",
		},
		{
			name: "relay on identity",
			req: UploadCertificateRequest{Name: "c", Usage: models.CertificateUsageIdentity, Role: models.CertificateRoleRelay,
				Certificate: string(leaf.PEM()), PrivateKey: string(leaf.KeyPEM())},
			message: "role relay applies only to usage: downstream certificates",
		},
		{
			name:    "relay on upstream",
			req:     UploadCertificateRequest{Name: "c", Usage: models.CertificateUsageUpstream, Role: models.CertificateRoleRelay, Certificate: authority},
			message: "role relay applies only to usage: downstream certificates",
		},
		{
			name:    "unknown role",
			req:     UploadCertificateRequest{Name: "c", Usage: models.CertificateUsageIdentity, Role: "primary", Certificate: string(leaf.PEM()), PrivateKey: string(leaf.KeyPEM())},
			message: "role must be client, relay or default",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createTestAPIServerWithIdentitySupport(t, NewMockStorage())
			w := uploadCertificateBody(t, server, tt.req)

			require.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
			assert.Equal(t, tt.message, firstFieldError(t, w.Body.Bytes(), "role")["message"])
		})
	}
}

func TestUploadCertificate_SecondDefaultIdentity_Returns409(t *testing.T) {
	mockDB := NewMockStorage()
	mockDB.certs = []*models.StoredCertificate{seedUpstreamCert(t), defaultIdentityRow(t, "default-1", "gateway-default")}
	server := createTestAPIServerWithIdentitySupport(t, mockDB)

	leaf := gatewayIdentityLeaf(t, "another-default")
	w := uploadCertificateBody(t, server, UploadCertificateRequest{
		Name: "another-default", Usage: models.CertificateUsageIdentity, Role: models.CertificateRoleDefault,
		Certificate: string(leaf.PEM()), PrivateKey: string(leaf.KeyPEM()),
	})

	require.Equal(t, http.StatusConflict, w.Code, "body: %s", w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t,
		"gateway identity gateway-default already has role: default; delete it before uploading another default identity",
		resp["message"])
	assert.Len(t, mockDB.certs, 2, "the refused identity must not be stored")
	assert.Empty(t, certificateEvents(t, server))
}

func TestUpdateCertificate_DefaultIdentity_KeepsRole(t *testing.T) {
	mockDB := NewMockStorage()
	mockDB.certs = []*models.StoredCertificate{seedUpstreamCert(t), defaultIdentityRow(t, "default-1", "gateway-default")}
	server := createTestAPIServerWithIdentitySupport(t, mockDB)

	rotated := gatewayIdentityLeaf(t, "gateway-default-rotated")
	w := updateCertificateBody(t, server, "default-1", UploadCertificateRequest{
		Certificate: string(rotated.PEM()),
		PrivateKey:  string(rotated.KeyPEM()),
		Role:        models.CertificateRoleClient,
	})

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, models.CertificateRoleDefault, resp["role"])
	stored := storedCertificateByName(t, mockDB, "gateway-default")
	assert.True(t, stored.IsDefaultIdentity(), "rotation keeps role: default")
	assert.Equal(t, rotated.PEM(), stored.Certificate)
}

func TestDeleteCertificate_DefaultIdentity_Allowed(t *testing.T) {
	mockDB := NewMockStorage()
	target := defaultIdentityRow(t, "default-1", "gateway-default")
	mockDB.certs = []*models.StoredCertificate{seedUpstreamCert(t), target}
	server := createTestAPIServerWithIdentitySupport(t, mockDB)

	w := httptest.NewRecorder()
	newDeleteCertHandler(server, target.UUID).ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/certificates/"+target.UUID, nil))

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	requireOneCertificateEvent(t, server, "DELETE", target.UUID)
}

func TestListCertificates_DefaultIdentity_ShowsRole(t *testing.T) {
	mockDB := NewMockStorage()
	named := identityRowForUpdate(t, "named-1")
	named.Role = models.CertificateRoleClient // how a row stored without a role reads back
	mockDB.certs = []*models.StoredCertificate{defaultIdentityRow(t, "default-1", "gateway-default"), named}
	server := createTestAPIServerWithDB(mockDB)

	w := httptest.NewRecorder()
	newCertListHandler(server).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/certificates?usage=identity", nil))

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Certificates []map[string]any `json:"certificates"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	roles := map[string]any{}
	for _, item := range resp.Certificates {
		roles[item["name"].(string)] = item["role"]
	}
	assert.Equal(t, map[string]any{"gateway-default": models.CertificateRoleDefault, "out-identity-a": nil}, roles)
}

// lockedCertificateStorage serialises the certificate calls of a
// MockStorage so concurrent handlers can share it, and refuses a taken name
// as the database does.
type lockedCertificateStorage struct {
	*MockStorage
	mu sync.Mutex
}

func (l *lockedCertificateStorage) SaveCertificate(cert *models.StoredCertificate) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, existing := range l.certs {
		if existing.Name == cert.Name {
			return storage.ErrConflict
		}
	}
	return l.MockStorage.SaveCertificate(cert)
}

func (l *lockedCertificateStorage) GetCertificate(id string) (*models.StoredCertificate, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.MockStorage.GetCertificate(id)
}

func (l *lockedCertificateStorage) GetCertificateByName(name string) (*models.StoredCertificate, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.MockStorage.GetCertificateByName(name)
}

func (l *lockedCertificateStorage) ListCertificatesByUsage(usage string) ([]*models.StoredCertificate, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.MockStorage.ListCertificatesByUsage(usage)
}

func (l *lockedCertificateStorage) ListCertificates() ([]*models.StoredCertificate, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	certs, err := l.MockStorage.ListCertificates()
	return append([]*models.StoredCertificate(nil), certs...), err
}

// Concurrent role: default uploads on one replica store exactly one.
func TestUploadCertificate_ConcurrentDefaultIdentities_OneStored(t *testing.T) {
	mockDB := NewMockStorage()
	mockDB.certs = []*models.StoredCertificate{seedUpstreamCert(t)}
	db := &lockedCertificateStorage{MockStorage: mockDB}
	server := createTestAPIServerWithIdentitySupport(t, db)

	const uploads = 8
	requests := make([]UploadCertificateRequest, uploads)
	for i := range requests {
		name := fmt.Sprintf("gateway-default-%d", i)
		leaf := gatewayIdentityLeaf(t, name)
		requests[i] = UploadCertificateRequest{
			Name: name, Usage: models.CertificateUsageIdentity, Role: models.CertificateRoleDefault,
			Certificate: string(leaf.PEM()), PrivateKey: string(leaf.KeyPEM()),
		}
	}

	codes := make([]int, uploads)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range requests {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			codes[i] = uploadCertificateBody(t, server, requests[i]).Code
		}(i)
	}
	close(start)
	wg.Wait()

	counts := map[int]int{}
	for _, code := range codes {
		counts[code]++
	}
	assert.Equal(t, map[int]int{http.StatusCreated: 1, http.StatusConflict: uploads - 1}, counts)
	var defaults int
	for _, cert := range mockDB.certs {
		if cert.IsDefaultIdentity() {
			defaults++
		}
	}
	assert.Equal(t, 1, defaults, "exactly one default identity is stored")
}

// A second role: default upload whose name is also taken reports the role
// conflict, which is checked first.
func TestUploadCertificate_SecondDefaultIdentityWithTakenName_ReportsRoleConflict(t *testing.T) {
	mockDB := NewMockStorage()
	mockDB.certs = []*models.StoredCertificate{
		seedUpstreamCert(t),
		defaultIdentityRow(t, "default-1", "gateway-default"),
		identityRowForUpdate(t, "named-1"),
	}
	server := createTestAPIServerWithIdentitySupport(t, &lockedCertificateStorage{MockStorage: mockDB})
	takenName := storedCertificateByName(t, mockDB, "out-identity-a").Name
	leaf := gatewayIdentityLeaf(t, takenName)
	upload := func(role string) map[string]any {
		t.Helper()
		w := uploadCertificateBody(t, server, UploadCertificateRequest{
			Name: takenName, Usage: models.CertificateUsageIdentity, Role: role,
			Certificate: string(leaf.PEM()), PrivateKey: string(leaf.KeyPEM()),
		})
		require.Equal(t, http.StatusConflict, w.Code, "body: %s", w.Body.String())
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		return resp
	}

	assert.Equal(t, "a gateway identity named "+takenName+" already exists", upload("")["message"],
		"without role: default the name conflict is reported")
	assert.Equal(t,
		"gateway identity gateway-default already has role: default; delete it before uploading another default identity",
		upload(models.CertificateRoleDefault)["message"])
	assert.Len(t, mockDB.certs, 3, "the refused identities must not be stored")
}
