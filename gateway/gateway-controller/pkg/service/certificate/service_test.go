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

package certificate

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// certStore is a storage.Storage that implements only the certificate methods
// these tests exercise. storage.Storage is embedded as a nil interface, so any
// other database call panics rather than silently returning a zero value.
type certStore struct {
	storage.Storage

	certs map[string]*models.StoredCertificate
	calls *[]string

	saveErr   error
	getErr    error
	listErr   error
	deleteErr error
}

func newCertStore(calls *[]string) *certStore {
	return &certStore{certs: map[string]*models.StoredCertificate{}, calls: calls}
}

func (s *certStore) SaveCertificate(cert *models.StoredCertificate) error {
	*s.calls = append(*s.calls, "save_certificate")
	if s.saveErr != nil {
		return s.saveErr
	}
	stored := *cert
	s.certs[cert.UUID] = &stored
	return nil
}

func (s *certStore) GetCertificate(id string) (*models.StoredCertificate, error) {
	*s.calls = append(*s.calls, "get_certificate")
	if s.getErr != nil {
		return nil, s.getErr
	}
	cert, ok := s.certs[id]
	if !ok {
		return nil, fmt.Errorf("%w: id=%s", storage.ErrNotFound, id)
	}
	return cert, nil
}

func (s *certStore) GetCertificateByName(name string) (*models.StoredCertificate, error) {
	*s.calls = append(*s.calls, "get_certificate_by_name")
	if s.getErr != nil {
		return nil, s.getErr
	}
	for _, cert := range s.certs {
		if cert.Name == name {
			return cert, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (s *certStore) ListCertificates() ([]*models.StoredCertificate, error) {
	*s.calls = append(*s.calls, "list_certificates")
	if s.listErr != nil {
		return nil, s.listErr
	}
	out := make([]*models.StoredCertificate, 0, len(s.certs))
	for _, cert := range s.certs {
		out = append(out, cert)
	}
	return out, nil
}

func (s *certStore) DeleteCertificate(id string) error {
	*s.calls = append(*s.calls, "delete_certificate")
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.certs, id)
	return nil
}

// fakeCertStore stands in for certstore.CertStore.
type fakeCertStore struct {
	calls     *[]string
	combined  []byte
	reloadErr error
}

func (c *fakeCertStore) Reload() error {
	*c.calls = append(*c.calls, "reload")
	return c.reloadErr
}

func (c *fakeCertStore) GetCombinedCertificates() []byte { return c.combined }

// fakeSnapshot stands in for xds.SnapshotManager.
type fakeSnapshot struct {
	calls          *[]string
	correlationIDs []string
	err            error
}

func (s *fakeSnapshot) UpdateSnapshot(_ context.Context, correlationID string) error {
	*s.calls = append(*s.calls, "update_snapshot")
	s.correlationIDs = append(s.correlationIDs, correlationID)
	return s.err
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestService(t *testing.T) (*CertificateService, *certStore, *fakeCertStore, *fakeSnapshot, *[]string) {
	t.Helper()

	calls := &[]string{}
	db := newCertStore(calls)
	store := &fakeCertStore{calls: calls, combined: []byte("combined-bundle")}
	snapshot := &fakeSnapshot{calls: calls}
	resolver := func() *XDSTargets { return &XDSTargets{Store: store, Snapshot: snapshot} }

	return NewCertificateService(db, resolver, testLogger()), db, store, snapshot, calls
}

// testCertPEM mints a real self-signed certificate so ValidateChain and
// ExtractMetadata operate on genuine DER rather than a fixture that could drift
// from what x509 accepts.
func testCertPEM(t *testing.T, commonName string, notAfter time.Time) []byte {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: commonName},
		Issuer:       pkix.Name{CommonName: commonName},
		NotBefore:    notAfter.Add(-24 * time.Hour),
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestNewCertificateService_PanicsWithoutStorage(t *testing.T) {
	assert.PanicsWithValue(t, "CertificateService requires non-nil storage", func() {
		NewCertificateService(nil, nil, testLogger())
	})
}

func TestUpload_PersistsThenReloadsThenPushes(t *testing.T) {
	svc, db, _, snapshot, calls := newTestService(t)
	notAfter := time.Now().Add(365 * 24 * time.Hour).Truncate(time.Second)
	pemData := testCertPEM(t, "upstream.example.com", notAfter)

	result, err := svc.Upload(UploadParams{
		Name:           "  upstream  ",
		CertificatePEM: pemData,
		CorrelationID:  "corr-1",
		Logger:         testLogger(),
	})
	require.NoError(t, err)

	cert := result.Certificate
	assert.Equal(t, "upstream", cert.Name, "the name is trimmed before storage")
	assert.NotEmpty(t, cert.UUID)
	assert.Equal(t, 1, cert.CertCount)
	assert.Contains(t, cert.Subject, "upstream.example.com")
	assert.Contains(t, cert.Issuer, "upstream.example.com")
	assert.Equal(t, notAfter.UTC(), cert.NotAfter.UTC())
	assert.Equal(t, pemData, cert.Certificate)
	assert.False(t, cert.CreatedAt.IsZero())

	assert.Contains(t, db.certs, cert.UUID)

	// The row must land before the reload, and the reload before the SDS push:
	// the router reads the trust store from the database.
	assert.Equal(t, []string{"save_certificate", "reload", "update_snapshot"}, *calls)
	assert.Equal(t, []string{"corr-1"}, snapshot.correlationIDs)
}

func TestUpload_CountsEveryCertificateInAChain(t *testing.T) {
	svc, _, _, _, _ := newTestService(t)
	notAfter := time.Now().Add(24 * time.Hour)

	chain := append(testCertPEM(t, "leaf.example.com", notAfter), testCertPEM(t, "ca.example.com", notAfter)...)

	result, err := svc.Upload(UploadParams{Name: "chain", CertificatePEM: chain, Logger: testLogger()})
	require.NoError(t, err)

	assert.Equal(t, 2, result.Certificate.CertCount)
	// Metadata comes from the leaf, not the last block parsed.
	assert.Contains(t, result.Certificate.Subject, "leaf.example.com")
}

func TestUpload_RejectsBadInputBeforeTouchingStorage(t *testing.T) {
	valid := testCertPEM(t, "upstream.example.com", time.Now().Add(24*time.Hour))

	tests := []struct {
		name   string
		params UploadParams
		assert func(t *testing.T, err error)
	}{
		{
			name:   "missing name",
			params: UploadParams{Name: "   ", CertificatePEM: valid},
			assert: func(t *testing.T, err error) { assert.ErrorIs(t, err, ErrMissingFields) },
		},
		{
			name:   "missing certificate",
			params: UploadParams{Name: "upstream"},
			assert: func(t *testing.T, err error) { assert.ErrorIs(t, err, ErrMissingFields) },
		},
		{
			name:   "not PEM at all",
			params: UploadParams{Name: "upstream", CertificatePEM: []byte("not-a-pem")},
			assert: func(t *testing.T, err error) {
				var invalid *InvalidCertificateError
				require.ErrorAs(t, err, &invalid)
				assert.Contains(t, err.Error(), "no valid certificates found in PEM data")
			},
		},
		{
			name: "PEM block that is not a certificate",
			params: UploadParams{
				Name:           "upstream",
				CertificatePEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("garbage")}),
			},
			assert: func(t *testing.T, err error) {
				var invalid *InvalidCertificateError
				require.ErrorAs(t, err, &invalid)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _, _, calls := newTestService(t)
			tc.params.Logger = testLogger()

			_, err := svc.Upload(tc.params)

			require.Error(t, err)
			tc.assert(t, err)
			assert.Empty(t, *calls, "a rejected upload must not reach storage or the router")
		})
	}
}

func TestUpload_SaveFailureIsReportedAsAPersistError(t *testing.T) {
	svc, db, _, _, calls := newTestService(t)
	db.saveErr = errors.New("database error")

	_, err := svc.Upload(UploadParams{
		Name:           "upstream",
		CertificatePEM: testCertPEM(t, "upstream.example.com", time.Now().Add(24*time.Hour)),
		Logger:         testLogger(),
	})

	var persistErr *PersistError
	require.ErrorAs(t, err, &persistErr)
	assert.Equal(t, OpSave, persistErr.Op)
	assert.Equal(t, []string{"save_certificate"}, *calls, "the router is not touched after a failed write")
}

func TestUpload_ConflictRemainsDetectable(t *testing.T) {
	svc, db, _, _, _ := newTestService(t)
	db.saveErr = fmt.Errorf("%w: certificate with name 'upstream' already exists", storage.ErrConflict)

	_, err := svc.Upload(UploadParams{
		Name:           "upstream",
		CertificatePEM: testCertPEM(t, "upstream.example.com", time.Now().Add(24*time.Hour)),
		Logger:         testLogger(),
	})

	// PersistError must unwrap, so a caller can still classify the conflict even
	// though the REST handler currently reports every save failure as a 500.
	require.Error(t, err)
	assert.True(t, storage.IsConflictError(err))
}

// The certificate stays in the database when the router cannot be updated. That
// divergence is deliberate — a reload is idempotent and recoverable through
// Reload, whereas rolling back a committed write is not — so it is pinned here.
func TestUpload_SyncFailuresLeaveTheRowCommitted(t *testing.T) {
	tests := []struct {
		name  string
		arm   func(store *fakeCertStore, snapshot *fakeSnapshot)
		stage SyncStage
		calls []string
	}{
		{
			name:  "reload fails",
			arm:   func(store *fakeCertStore, _ *fakeSnapshot) { store.reloadErr = errors.New("reload boom") },
			stage: StageReload,
			calls: []string{"save_certificate", "reload"},
		},
		{
			name:  "snapshot push fails",
			arm:   func(_ *fakeCertStore, snapshot *fakeSnapshot) { snapshot.err = errors.New("sds boom") },
			stage: StageSnapshot,
			calls: []string{"save_certificate", "reload", "update_snapshot"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, db, store, snapshot, calls := newTestService(t)
			tc.arm(store, snapshot)

			_, err := svc.Upload(UploadParams{
				Name:           "upstream",
				CertificatePEM: testCertPEM(t, "upstream.example.com", time.Now().Add(24*time.Hour)),
				Logger:         testLogger(),
			})

			var syncErr *SyncError
			require.ErrorAs(t, err, &syncErr)
			assert.Equal(t, tc.stage, syncErr.Stage)
			assert.True(t, syncErr.Persisted, "the caller must be told the write already committed")
			assert.Len(t, db.certs, 1, "the certificate is not rolled back")
			assert.Equal(t, tc.calls, *calls)
		})
	}
}

func TestUpload_WithoutACertStoreStillCommits(t *testing.T) {
	calls := &[]string{}
	db := newCertStore(calls)
	svc := NewCertificateService(db, func() *XDSTargets { return nil }, testLogger())

	_, err := svc.Upload(UploadParams{
		Name:           "upstream",
		CertificatePEM: testCertPEM(t, "upstream.example.com", time.Now().Add(24*time.Hour)),
		Logger:         testLogger(),
	})

	assert.ErrorIs(t, err, ErrCertStoreNotConfigured)
	assert.Len(t, db.certs, 1, "matching the pre-service behaviour: the row is saved, then the sync fails")
}

func TestXDSResolution_NeverPanics(t *testing.T) {
	pemData := testCertPEM(t, "upstream.example.com", time.Now().Add(24*time.Hour))

	tests := []struct {
		name     string
		resolver XDSResolver
	}{
		{name: "no resolver at all", resolver: nil},
		{name: "resolver reports no xDS layer", resolver: func() *XDSTargets { return nil }},
		{name: "half-populated targets", resolver: func() *XDSTargets { return &XDSTargets{Store: &fakeCertStore{calls: &[]string{}}} }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			calls := &[]string{}
			svc := NewCertificateService(newCertStore(calls), tc.resolver, testLogger())

			_, err := svc.Upload(UploadParams{Name: "upstream", CertificatePEM: pemData, Logger: testLogger()})
			assert.ErrorIs(t, err, ErrCertStoreNotConfigured)

			_, err = svc.Reload(ReloadParams{Logger: testLogger()})
			assert.ErrorIs(t, err, ErrCertStoreNotConfigured)
		})
	}
}

func TestDelete(t *testing.T) {
	t.Run("removes the row then republishes", func(t *testing.T) {
		svc, db, _, _, calls := newTestService(t)
		db.certs["cert-1"] = &models.StoredCertificate{UUID: "cert-1", Name: "upstream"}

		result, err := svc.Delete(DeleteParams{ID: "cert-1", CorrelationID: "corr-2", Logger: testLogger()})
		require.NoError(t, err)

		assert.Equal(t, "cert-1", result.ID)
		assert.NotContains(t, db.certs, "cert-1")
		assert.Equal(t, []string{"delete_certificate", "reload", "update_snapshot"}, *calls)
	})

	t.Run("a storage failure is reported as a delete failure", func(t *testing.T) {
		svc, db, _, _, _ := newTestService(t)
		db.deleteErr = storage.ErrNotFound

		_, err := svc.Delete(DeleteParams{ID: "missing", Logger: testLogger()})

		var persistErr *PersistError
		require.ErrorAs(t, err, &persistErr)
		assert.Equal(t, OpDelete, persistErr.Op)
	})

	t.Run("blank id", func(t *testing.T) {
		svc, _, _, _, calls := newTestService(t)

		_, err := svc.Delete(DeleteParams{ID: "  ", Logger: testLogger()})

		assert.ErrorIs(t, err, ErrMissingFields)
		assert.Empty(t, *calls)
	})

	// The cert store is checked BEFORE the row is removed, so an unconfigured
	// gateway cannot delete a certificate it is then unable to stop serving.
	t.Run("without a cert store the row survives", func(t *testing.T) {
		calls := &[]string{}
		db := newCertStore(calls)
		db.certs["cert-1"] = &models.StoredCertificate{UUID: "cert-1"}
		svc := NewCertificateService(db, func() *XDSTargets { return nil }, testLogger())

		_, err := svc.Delete(DeleteParams{ID: "cert-1", Logger: testLogger()})

		assert.ErrorIs(t, err, ErrCertStoreNotConfigured)
		assert.Contains(t, db.certs, "cert-1")
		assert.Empty(t, *calls, "the delete is refused before it reaches storage")
	})
}

func TestReload(t *testing.T) {
	t.Run("returns the combined bundle size", func(t *testing.T) {
		svc, _, store, snapshot, calls := newTestService(t)
		store.combined = []byte("0123456789")

		result, err := svc.Reload(ReloadParams{CorrelationID: "corr-3", Logger: testLogger()})
		require.NoError(t, err)

		assert.Equal(t, 10, result.TotalBytes)
		assert.Equal(t, []string{"reload", "update_snapshot"}, *calls)
		assert.Equal(t, []string{"corr-3"}, snapshot.correlationIDs)
	})

	t.Run("a reload failure is not marked as persisted", func(t *testing.T) {
		svc, _, store, _, _ := newTestService(t)
		store.reloadErr = errors.New("reload boom")

		_, err := svc.Reload(ReloadParams{Logger: testLogger()})

		var syncErr *SyncError
		require.ErrorAs(t, err, &syncErr)
		assert.Equal(t, StageReload, syncErr.Stage)
		assert.False(t, syncErr.Persisted, "reload changes no stored certificate")
	})
}

func TestList(t *testing.T) {
	t.Run("sums the stored PEM bytes", func(t *testing.T) {
		svc, db, _, _, _ := newTestService(t)
		db.certs["a"] = &models.StoredCertificate{UUID: "a", Certificate: []byte("abc")}
		db.certs["b"] = &models.StoredCertificate{UUID: "b", Certificate: []byte("de")}

		result, err := svc.List()
		require.NoError(t, err)

		assert.Len(t, result.Certificates, 2)
		assert.Equal(t, 5, result.TotalBytes)
	})

	t.Run("empty store", func(t *testing.T) {
		svc, _, _, _, _ := newTestService(t)

		result, err := svc.List()
		require.NoError(t, err)

		assert.Empty(t, result.Certificates)
		assert.Equal(t, 0, result.TotalBytes)
	})

	t.Run("storage failure", func(t *testing.T) {
		svc, db, _, _, _ := newTestService(t)
		db.listErr = errors.New("database unavailable")

		_, err := svc.List()

		var persistErr *PersistError
		require.ErrorAs(t, err, &persistErr)
		assert.Equal(t, OpList, persistErr.Op)
	})
}

func TestGetAndGetByName(t *testing.T) {
	svc, db, _, _, _ := newTestService(t)
	db.certs["cert-1"] = &models.StoredCertificate{UUID: "cert-1", Name: "upstream"}

	byID, err := svc.Get("cert-1")
	require.NoError(t, err)
	assert.Equal(t, "upstream", byID.Certificate.Name)

	byName, err := svc.GetByName("upstream")
	require.NoError(t, err)
	assert.Equal(t, "cert-1", byName.Certificate.UUID)

	_, err = svc.Get("missing")
	assert.ErrorIs(t, err, ErrNotFound)

	_, err = svc.GetByName("missing")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestExtractMetadata_NoCertificate(t *testing.T) {
	_, _, _, _, err := ExtractMetadata([]byte("not-a-pem"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no valid certificate found")
}
