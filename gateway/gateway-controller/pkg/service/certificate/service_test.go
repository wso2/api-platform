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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/clientca"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption/aesgcm"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// Every write refreshes the certificate metrics, which exist only once
// initialised, as the controller does at startup.
func init() {
	metrics.Init()
}

// certStore is a storage.Storage that implements only the methods these tests
// exercise. storage.Storage is embedded as a nil interface, so any other
// database call panics rather than silently returning a zero value.
type certStore struct {
	storage.Storage

	certs   map[string]*models.StoredCertificate
	configs map[string][]*models.StoredConfig // by kind
	calls   *[]string

	saveErr    error
	getErr     error
	listErr    error
	deleteErr  error
	configsErr error
}

func newCertStore(calls *[]string) *certStore {
	return &certStore{
		certs:   map[string]*models.StoredCertificate{},
		configs: map[string][]*models.StoredConfig{},
		calls:   calls,
	}
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

func (s *certStore) UpdateCertificate(cert *models.StoredCertificate) error {
	*s.calls = append(*s.calls, "update_certificate")
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

func (s *certStore) ListCertificatesByUsage(usage string) ([]*models.StoredCertificate, error) {
	*s.calls = append(*s.calls, "list_certificates_by_usage")
	if s.listErr != nil {
		return nil, s.listErr
	}
	out := []*models.StoredCertificate{}
	for _, cert := range s.certs {
		if cert.EffectiveUsage() == usage {
			out = append(out, cert)
		}
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

func (s *certStore) GetAllConfigsByKind(kind string) ([]*models.StoredConfig, error) {
	*s.calls = append(*s.calls, "get_configs_"+kind)
	if s.configsErr != nil {
		return nil, s.configsErr
	}
	return s.configs[kind], nil
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

// recorder captures the replica events and client authority publishes the
// service makes.
type recorder struct {
	calls      *[]string
	events     []string // "ACTION:certID"
	publishes  int
	publishErr error
}

func (r *recorder) publishEvent(action, certID, _ string, _ *slog.Logger) {
	*r.calls = append(*r.calls, "event")
	r.events = append(r.events, action+":"+certID)
}

func (r *recorder) publishClientAuthorities(string) error {
	*r.calls = append(*r.calls, "publish_client_authorities")
	r.publishes++
	return r.publishErr
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type testHarness struct {
	svc      *CertificateService
	db       *certStore
	store    *fakeCertStore
	snapshot *fakeSnapshot
	rec      *recorder
	calls    *[]string
}

func newTestService(t *testing.T) *testHarness {
	t.Helper()

	calls := &[]string{}
	h := &testHarness{
		db:       newCertStore(calls),
		store:    &fakeCertStore{calls: calls, combined: []byte("combined-bundle")},
		snapshot: &fakeSnapshot{calls: calls},
		rec:      &recorder{calls: calls},
		calls:    calls,
	}
	h.svc = NewCertificateService(Dependencies{
		DB:                       h.db,
		ResolveXDS:               func() *XDSTargets { return &XDSTargets{Store: h.store, Snapshot: h.snapshot} },
		PublishEvent:             h.rec.publishEvent,
		PublishClientAuthorities: h.rec.publishClientAuthorities,
		Logger:                   testLogger(),
	})
	return h
}

// testCertPEM mints a real self-signed certificate so ValidateChain and
// ExtractMetadata operate on genuine DER rather than a fixture that could drift
// from what x509 accepts.
func testCertPEM(t *testing.T, commonName string, notAfter time.Time) []byte {
	t.Helper()
	return testCertPEMWith(t, commonName, notAfter, false)
}

func testCertPEMWith(t *testing.T, commonName string, notAfter time.Time, isCA bool) []byte {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: commonName},
		Issuer:                pkix.Name{CommonName: commonName},
		NotBefore:             notAfter.Add(-24 * time.Hour),
		NotAfter:              notAfter,
		IsCA:                  isCA,
		BasicConstraintsValid: isCA,
	}
	if isCA {
		tmpl.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

// testIdentityPair mints a self-signed leaf and its PKCS#8 private key, the
// shape a usage: identity upload takes.
func testIdentityPair(t *testing.T, commonName string) (string, string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return string(certPEM), string(keyPEM)
}

// testEncryptionManager builds an AES-GCM provider backed by a temporary key.
func testEncryptionManager(t *testing.T) *encryption.ProviderManager {
	t.Helper()

	keyPath := filepath.Join(t.TempDir(), "v1.key")
	key := make([]byte, aesgcm.AESKeySize)
	_, err := rand.Read(key)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyPath, key, 0600))

	provider, err := aesgcm.NewAESGCMProvider([]aesgcm.KeyConfig{{Version: "v1", FilePath: keyPath}}, testLogger())
	require.NoError(t, err)
	mgr, err := encryption.NewProviderManager([]encryption.EncryptionProvider{provider}, testLogger())
	require.NoError(t, err)
	return mgr
}

func fieldsOf(t *testing.T, err error) []FieldError {
	t.Helper()
	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, MsgUploadInvalid, ve.Message)
	return ve.Fields
}

func TestNewCertificateService_PanicsWithoutStorage(t *testing.T) {
	assert.PanicsWithValue(t, "CertificateService requires non-nil storage", func() {
		NewCertificateService(Dependencies{})
	})
}

func TestUpload_PersistsThenSignalsThenReloadsThenPushes(t *testing.T) {
	h := newTestService(t)
	notAfter := time.Now().Add(365 * 24 * time.Hour).Truncate(time.Second)
	pemData := testCertPEM(t, "upstream.example.com", notAfter)

	result, err := h.svc.Upload(UploadParams{
		Name:          "upstream",
		Certificate:   string(pemData),
		CorrelationID: "corr-1",
		Logger:        testLogger(),
	})
	require.NoError(t, err)

	cert := result.Certificate
	assert.Equal(t, "upstream", cert.Name)
	assert.NotEmpty(t, cert.UUID)
	assert.Equal(t, 1, cert.CertCount)
	assert.Contains(t, cert.Subject, "upstream.example.com")
	assert.Contains(t, cert.Issuer, "upstream.example.com")
	assert.Equal(t, notAfter.UTC(), cert.NotAfter.UTC())
	assert.Equal(t, pemData, cert.Certificate)
	assert.False(t, cert.CreatedAt.IsZero())
	assert.Equal(t, models.CertificateUsageUpstream, cert.Usage, "usage defaults to upstream")
	assert.Equal(t, models.CertificateUsageUpstream, result.Usage)
	assert.True(t, result.IsLeaf, "a non-CA certificate is a leaf")

	assert.Contains(t, h.db.certs, cert.UUID)

	// The row must land before the reload, and the reload before the SDS push:
	// the router reads the trust store from the database. The replica event
	// follows the committed write; metrics refresh last. An upstream
	// certificate does not touch the client authority pool.
	assert.Equal(t, []string{"save_certificate", "event", "reload", "update_snapshot", "list_certificates"}, *h.calls)
	assert.Equal(t, []string{"corr-1"}, h.snapshot.correlationIDs)
	assert.Equal(t, []string{"CREATE:" + cert.UUID}, h.rec.events)
	assert.Zero(t, h.rec.publishes)
}

func TestUpload_CountsEveryCertificateInAChain(t *testing.T) {
	h := newTestService(t)
	notAfter := time.Now().Add(24 * time.Hour)

	chain := append(testCertPEM(t, "leaf.example.com", notAfter), testCertPEM(t, "ca.example.com", notAfter)...)

	result, err := h.svc.Upload(UploadParams{Name: "chain", Certificate: string(chain), Logger: testLogger()})
	require.NoError(t, err)

	assert.Equal(t, 2, result.Certificate.CertCount)
	// Metadata comes from the leaf, not the last block parsed.
	assert.Contains(t, result.Certificate.Subject, "leaf.example.com")
}

func TestUpload_RejectsBadInputBeforeTouchingStorage(t *testing.T) {
	valid := string(testCertPEM(t, "upstream.example.com", time.Now().Add(24*time.Hour)))

	tests := []struct {
		name   string
		params UploadParams
		want   []FieldError
	}{
		{
			name:   "missing name",
			params: UploadParams{Certificate: valid},
			want:   []FieldError{{Field: "name", Message: "both name and certificate are required"}},
		},
		{
			name:   "missing certificate",
			params: UploadParams{Name: "upstream"},
			want:   []FieldError{{Field: "certificate", Message: "both name and certificate are required"}},
		},
		{
			name:   "name outside the allowed characters",
			params: UploadParams{Name: "my cert", Certificate: valid},
			want:   []FieldError{{Field: "name", Message: "name may contain only letters, digits, ., _ and -"}},
		},
		{
			name:   "not PEM at all",
			params: UploadParams{Name: "upstream", Certificate: "not-a-pem"},
			want:   []FieldError{{Field: "certificate", Message: clientca.MsgNotPEMCertificate}},
		},
		{
			name: "a private key pasted into certificate",
			params: UploadParams{
				Name:        "upstream",
				Certificate: valid + string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("key")})),
			},
			want: []FieldError{{Field: "certificate", Message: msgCertificateFieldCarriesKey}},
		},
		{
			name:   "a private key on a non-identity upload",
			params: UploadParams{Name: "upstream", Certificate: valid, PrivateKey: "key"},
			want:   []FieldError{{Field: "privateKey", Message: "privateKey applies only to usage: identity certificates"}},
		},
		{
			name:   "unknown usage",
			params: UploadParams{Name: "upstream", Certificate: valid, Usage: "sideways"},
			want:   []FieldError{{Field: "usage", Message: "usage must be upstream, downstream or identity"}},
		},
		{
			name:   "caller-reported shape errors come after validation errors",
			params: UploadParams{Name: "my cert", Certificate: valid, FieldErrors: []FieldError{{Field: "extra", Message: "unknown field extra"}}},
			want: []FieldError{
				{Field: "name", Message: "name may contain only letters, digits, ., _ and -"},
				{Field: "extra", Message: "unknown field extra"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestService(t)
			tc.params.Logger = testLogger()

			_, err := h.svc.Upload(tc.params)

			require.Error(t, err)
			assert.Equal(t, tc.want, fieldsOf(t, err))
			assert.Empty(t, *h.calls, "a rejected upload must not reach storage or the router")
		})
	}
}

func TestUpload_SaveFailureIsReportedAsAPersistError(t *testing.T) {
	h := newTestService(t)
	h.db.saveErr = errors.New("database error")

	_, err := h.svc.Upload(UploadParams{
		Name:        "upstream",
		Certificate: string(testCertPEM(t, "upstream.example.com", time.Now().Add(24*time.Hour))),
		Logger:      testLogger(),
	})

	var persistErr *PersistError
	require.ErrorAs(t, err, &persistErr)
	assert.Equal(t, OpSave, persistErr.Op)
	assert.Equal(t, []string{"save_certificate"}, *h.calls, "the router is not touched after a failed write")
}

func TestUpload_ConflictRemainsDetectable(t *testing.T) {
	h := newTestService(t)
	h.db.saveErr = fmt.Errorf("%w: certificate with name 'upstream' already exists", storage.ErrConflict)

	_, err := h.svc.Upload(UploadParams{
		Name:        "upstream",
		Certificate: string(testCertPEM(t, "upstream.example.com", time.Now().Add(24*time.Hour))),
		Logger:      testLogger(),
	})

	// PersistError must unwrap, so a caller can classify the conflict.
	require.Error(t, err)
	assert.True(t, storage.IsConflictError(err))
}

func TestUpload_DownstreamRepublishesTheClientAuthorityPool(t *testing.T) {
	h := newTestService(t)
	ca := testCertPEMWith(t, "Client CA", time.Now().Add(365*24*time.Hour), true)

	result, err := h.svc.Upload(UploadParams{
		Name:        "client-ca",
		Certificate: string(ca),
		Usage:       models.CertificateUsageDownstream,
		Logger:      testLogger(),
	})
	require.NoError(t, err)

	assert.Equal(t, models.CertificateUsageDownstream, result.Usage)
	assert.Equal(t, models.CertificateRoleClient, result.Role, "role defaults to client for downstream")
	assert.Equal(t, 1, h.rec.publishes)
	assert.Equal(t, "publish_client_authorities", (*h.calls)[len(*h.calls)-1], "the pool is republished last")
}

func TestUpload_PolicyEngineFailureIsAPersistedSyncError(t *testing.T) {
	h := newTestService(t)
	h.rec.publishErr = errors.New("policy engine down")
	ca := testCertPEMWith(t, "Client CA", time.Now().Add(365*24*time.Hour), true)

	_, err := h.svc.Upload(UploadParams{
		Name:        "client-ca",
		Certificate: string(ca),
		Usage:       models.CertificateUsageDownstream,
		Logger:      testLogger(),
	})

	var syncErr *SyncError
	require.ErrorAs(t, err, &syncErr)
	assert.Equal(t, StagePolicyEngine, syncErr.Stage)
	assert.True(t, syncErr.Persisted)
	assert.Len(t, h.db.certs, 1)
}

// A gateway identity's private key is never stored in clear: with no
// encryption provider the upload stops before anything is saved.
func TestUpload_IdentityWithoutEncryptionIsRefusedBeforeStorage(t *testing.T) {
	h := newTestService(t)
	certPEM, keyPEM := testIdentityPair(t, "gw.example.com")

	_, err := h.svc.Upload(UploadParams{
		Name:        "gw-identity",
		Certificate: certPEM,
		Usage:       models.CertificateUsageIdentity,
		PrivateKey:  keyPEM,
		Logger:      testLogger(),
	})

	assert.ErrorIs(t, err, ErrIdentityStorageUnavailable)
	assert.Empty(t, h.db.certs)
	assert.Empty(t, *h.calls)
}

func TestUpload_IdentityIsStoredEncrypted(t *testing.T) {
	h := newTestService(t)
	mgr := testEncryptionManager(t)
	h.svc.deps.EncryptionManager = func() *encryption.ProviderManager { return mgr }
	certPEM, keyPEM := testIdentityPair(t, "gw.example.com")

	result, err := h.svc.Upload(UploadParams{
		Name:        "gw-identity",
		Certificate: certPEM,
		Usage:       models.CertificateUsageIdentity,
		Role:        models.CertificateRoleDefault,
		PrivateKey:  keyPEM,
		Logger:      testLogger(),
	})
	require.NoError(t, err)

	stored := h.db.certs[result.Certificate.UUID]
	require.NotNil(t, stored)
	assert.NotEmpty(t, stored.PrivateKeyCiphertext)
	assert.NotContains(t, stored.PrivateKeyCiphertext, "PRIVATE KEY", "the key is never stored in clear")
	assert.Equal(t, models.CertificateRoleDefault, result.Role)
	assert.Equal(t, "RSA", result.KeyAlgorithm)
}

func TestUpload_SecondDefaultIdentityIsRefused(t *testing.T) {
	h := newTestService(t)
	mgr := testEncryptionManager(t)
	h.svc.deps.EncryptionManager = func() *encryption.ProviderManager { return mgr }
	h.db.certs["existing"] = &models.StoredCertificate{
		UUID: "existing", Name: "first-default",
		Usage: models.CertificateUsageIdentity, Role: models.CertificateRoleDefault,
	}
	certPEM, keyPEM := testIdentityPair(t, "gw.example.com")

	_, err := h.svc.Upload(UploadParams{
		Name:        "second-default",
		Certificate: certPEM,
		Usage:       models.CertificateUsageIdentity,
		Role:        models.CertificateRoleDefault,
		PrivateKey:  keyPEM,
		Logger:      testLogger(),
	})

	var defaultErr *DefaultIdentityExistsError
	require.ErrorAs(t, err, &defaultErr)
	assert.Equal(t,
		"gateway identity first-default already has role: default; delete it before uploading another default identity",
		err.Error())
	assert.Len(t, h.db.certs, 1, "nothing new is stored")
	assert.NotContains(t, *h.calls, "save_certificate")
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
			calls: []string{"save_certificate", "event", "reload"},
		},
		{
			name:  "snapshot push fails",
			arm:   func(_ *fakeCertStore, snapshot *fakeSnapshot) { snapshot.err = errors.New("sds boom") },
			stage: StageSnapshot,
			calls: []string{"save_certificate", "event", "reload", "update_snapshot"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestService(t)
			tc.arm(h.store, h.snapshot)

			_, err := h.svc.Upload(UploadParams{
				Name:        "upstream",
				Certificate: string(testCertPEM(t, "upstream.example.com", time.Now().Add(24*time.Hour))),
				Logger:      testLogger(),
			})

			var syncErr *SyncError
			require.ErrorAs(t, err, &syncErr)
			assert.Equal(t, tc.stage, syncErr.Stage)
			assert.True(t, syncErr.Persisted, "the caller must be told the write already committed")
			assert.Len(t, h.db.certs, 1, "the certificate is not rolled back")
			assert.Equal(t, tc.calls, *h.calls)
		})
	}
}

func TestUpload_WithoutACertStoreStillCommits(t *testing.T) {
	calls := &[]string{}
	db := newCertStore(calls)
	svc := NewCertificateService(Dependencies{DB: db, ResolveXDS: func() *XDSTargets { return nil }, Logger: testLogger()})

	_, err := svc.Upload(UploadParams{
		Name:        "upstream",
		Certificate: string(testCertPEM(t, "upstream.example.com", time.Now().Add(24*time.Hour))),
		Logger:      testLogger(),
	})

	assert.ErrorIs(t, err, ErrCertStoreNotConfigured)
	assert.Len(t, db.certs, 1, "matching the REST behaviour: the row is saved, then the sync fails")
}

func TestXDSResolution_NeverPanics(t *testing.T) {
	pemData := string(testCertPEM(t, "upstream.example.com", time.Now().Add(24*time.Hour)))

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
			svc := NewCertificateService(Dependencies{DB: newCertStore(calls), ResolveXDS: tc.resolver, Logger: testLogger()})

			_, err := svc.Upload(UploadParams{Name: "upstream", Certificate: pemData, Logger: testLogger()})
			assert.ErrorIs(t, err, ErrCertStoreNotConfigured)

			_, err = svc.Reload(ReloadParams{Logger: testLogger()})
			assert.ErrorIs(t, err, ErrCertStoreNotConfigured)
		})
	}
}

func deployedRestAPITrusting(handle, certName string) *models.StoredConfig {
	tls := map[string]interface{}{"trustedCAs": []interface{}{certName}}
	defs := []api.UpstreamDefinition{{Tls: &tls}}
	return &models.StoredConfig{
		UUID:         "cfg-" + handle,
		Handle:       handle,
		Kind:         models.KindRestApi,
		DesiredState: models.StateDeployed,
		Configuration: api.RestAPI{
			Spec: api.APIConfigData{UpstreamDefinitions: &defs},
		},
	}
}

func TestDelete(t *testing.T) {
	t.Run("reads, removes the row, then signals and republishes", func(t *testing.T) {
		h := newTestService(t)
		h.db.certs["cert-1"] = &models.StoredCertificate{UUID: "cert-1", Name: "upstream", Usage: models.CertificateUsageUpstream}

		result, err := h.svc.Delete(DeleteParams{ID: "cert-1", CorrelationID: "corr-2", Logger: testLogger()})
		require.NoError(t, err)

		assert.Equal(t, "cert-1", result.ID)
		assert.NotContains(t, h.db.certs, "cert-1")
		assert.Equal(t, []string{
			"get_certificate", "get_configs_RestApi", "get_configs_Agent",
			"delete_certificate", "event", "reload", "update_snapshot", "list_certificates",
		}, *h.calls)
		assert.Equal(t, []string{"DELETE:cert-1"}, h.rec.events)
		assert.Zero(t, h.rec.publishes, "only a downstream delete republishes the pool")
	})

	t.Run("refused while a deployed API trusts the certificate", func(t *testing.T) {
		h := newTestService(t)
		h.db.certs["cert-1"] = &models.StoredCertificate{UUID: "cert-1", Name: "upstream-ca", Usage: models.CertificateUsageUpstream}
		h.db.configs[models.KindRestApi] = []*models.StoredConfig{deployedRestAPITrusting("orders", "upstream-ca")}

		_, err := h.svc.Delete(DeleteParams{ID: "cert-1", Logger: testLogger()})

		var inUse *InUseError
		require.ErrorAs(t, err, &inUse)
		assert.Equal(t, "certificate 'upstream-ca' is named by 1 deployed API; remove those references first", inUse.Message)
		assert.Equal(t, []Reference{{Field: "spec.upstreamDefinitions[0].tls.trustedCAs[0]", API: "orders"}}, inUse.References)
		assert.Contains(t, h.db.certs, "cert-1", "nothing is deleted")
		assert.Empty(t, h.rec.events)
	})

	t.Run("refused when the references cannot be read", func(t *testing.T) {
		h := newTestService(t)
		h.db.certs["cert-1"] = &models.StoredCertificate{UUID: "cert-1", Name: "upstream-ca", Usage: models.CertificateUsageUpstream}
		h.db.configsErr = errors.New("database unavailable")

		_, err := h.svc.Delete(DeleteParams{ID: "cert-1", Logger: testLogger()})

		var refCheck *ReferenceCheckError
		require.ErrorAs(t, err, &refCheck)
		assert.Equal(t, "Failed to verify certificate references", refCheck.Message)
		assert.Contains(t, h.db.certs, "cert-1")
	})

	t.Run("a read failure stops the delete", func(t *testing.T) {
		h := newTestService(t)
		h.db.getErr = errors.New("database unavailable")

		_, err := h.svc.Delete(DeleteParams{ID: "cert-1", Logger: testLogger()})

		var persistErr *PersistError
		require.ErrorAs(t, err, &persistErr)
		assert.Equal(t, OpLoad, persistErr.Op)
		assert.NotContains(t, *h.calls, "delete_certificate")
	})

	t.Run("a storage failure is reported as a delete failure", func(t *testing.T) {
		h := newTestService(t)
		h.db.deleteErr = storage.ErrNotFound

		_, err := h.svc.Delete(DeleteParams{ID: "missing", Logger: testLogger()})

		var persistErr *PersistError
		require.ErrorAs(t, err, &persistErr)
		assert.Equal(t, OpDelete, persistErr.Op)
	})

	t.Run("a downstream delete republishes the pool", func(t *testing.T) {
		h := newTestService(t)
		h.db.certs["relay-1"] = &models.StoredCertificate{
			UUID: "relay-1", Name: "relay", Usage: models.CertificateUsageDownstream, Role: models.CertificateRoleRelay,
		}

		_, err := h.svc.Delete(DeleteParams{ID: "relay-1", Logger: testLogger()})
		require.NoError(t, err)

		assert.Equal(t, 1, h.rec.publishes)
	})

	t.Run("blank id", func(t *testing.T) {
		h := newTestService(t)

		_, err := h.svc.Delete(DeleteParams{ID: "", Logger: testLogger()})

		assert.ErrorIs(t, err, ErrMissingID)
		assert.Empty(t, *h.calls)
	})

	// The cert store is checked BEFORE the row is read or removed, so an
	// unconfigured gateway cannot delete a certificate it is then unable to
	// stop serving.
	t.Run("without a cert store the row survives", func(t *testing.T) {
		calls := &[]string{}
		db := newCertStore(calls)
		db.certs["cert-1"] = &models.StoredCertificate{UUID: "cert-1"}
		svc := NewCertificateService(Dependencies{DB: db, ResolveXDS: func() *XDSTargets { return nil }, Logger: testLogger()})

		_, err := svc.Delete(DeleteParams{ID: "cert-1", Logger: testLogger()})

		assert.ErrorIs(t, err, ErrCertStoreNotConfigured)
		assert.Contains(t, db.certs, "cert-1")
		assert.Empty(t, *calls, "the delete is refused before it reaches storage")
	})
}

func TestReload(t *testing.T) {
	t.Run("returns the combined bundle size", func(t *testing.T) {
		h := newTestService(t)
		h.store.combined = []byte("0123456789")

		result, err := h.svc.Reload(ReloadParams{CorrelationID: "corr-3", Logger: testLogger()})
		require.NoError(t, err)

		assert.Equal(t, 10, result.TotalBytes)
		assert.Equal(t, []string{"reload", "update_snapshot", "event", "publish_client_authorities", "list_certificates"}, *h.calls)
		assert.Equal(t, []string{"corr-3"}, h.snapshot.correlationIDs)
		assert.Equal(t, []string{"UPDATE:"}, h.rec.events, "a reload names no single row")
	})

	t.Run("a reload failure is not marked as persisted", func(t *testing.T) {
		h := newTestService(t)
		h.store.reloadErr = errors.New("reload boom")

		_, err := h.svc.Reload(ReloadParams{Logger: testLogger()})

		var syncErr *SyncError
		require.ErrorAs(t, err, &syncErr)
		assert.Equal(t, StageReload, syncErr.Stage)
		assert.False(t, syncErr.Persisted, "reload changes no stored certificate")
	})
}

func TestUpdate_OnlyIdentityCertificatesCanBeUpdated(t *testing.T) {
	h := newTestService(t)
	h.db.certs["cert-1"] = &models.StoredCertificate{UUID: "cert-1", Name: "upstream", Usage: models.CertificateUsageUpstream}

	_, err := h.svc.Update(UpdateParams{ID: "cert-1", Certificate: "x", PrivateKey: "y", Logger: testLogger()})

	assert.Equal(t, []FieldError{{
		Field:   "usage",
		Message: "only usage: identity certificates can be updated; delete and re-upload other certificates",
	}}, fieldsOf(t, err))
	assert.Equal(t, []string{"get_certificate"}, *h.calls)

	_, err = h.svc.Update(UpdateParams{ID: "missing", Logger: testLogger()})
	assert.ErrorIs(t, err, ErrNotFound)
}

// A caller that already ran PrepareUpdate hands its result over, so the row
// is not read a second time.
func TestUpdate_UsesTheCertificateTheCallerAlreadyRead(t *testing.T) {
	h := newTestService(t)
	existing := &models.StoredCertificate{UUID: "id-1", Name: "gw", Usage: models.CertificateUsageIdentity}
	h.db.certs["id-1"] = existing

	_, err := h.svc.Update(UpdateParams{ID: "id-1", Existing: existing, Certificate: "x", PrivateKey: "y", Logger: testLogger()})

	require.Error(t, err, "the certificate and key are not valid PEM")
	assert.NotContains(t, *h.calls, "get_certificate")
}

func TestList(t *testing.T) {
	t.Run("sums the stored PEM bytes", func(t *testing.T) {
		h := newTestService(t)
		h.db.certs["a"] = &models.StoredCertificate{UUID: "a", Certificate: []byte("abc")}
		h.db.certs["b"] = &models.StoredCertificate{UUID: "b", Certificate: []byte("de")}

		result, err := h.svc.List(ListParams{Logger: testLogger()})
		require.NoError(t, err)

		assert.Len(t, result.Items, 2)
		assert.Equal(t, 5, result.TotalBytes)
	})

	t.Run("filters by usage", func(t *testing.T) {
		h := newTestService(t)
		h.db.certs["a"] = &models.StoredCertificate{UUID: "a", Usage: models.CertificateUsageUpstream}
		h.db.certs["b"] = &models.StoredCertificate{UUID: "b", Usage: models.CertificateUsageDownstream}

		result, err := h.svc.List(ListParams{Usage: models.CertificateUsageUpstream, Logger: testLogger()})
		require.NoError(t, err)

		require.Len(t, result.Items, 1)
		assert.Equal(t, "a", result.Items[0].Certificate.UUID)
		assert.Equal(t, models.CertificateUsageUpstream, result.Items[0].Usage)
	})

	t.Run("an unknown usage filter is refused", func(t *testing.T) {
		h := newTestService(t)

		_, err := h.svc.List(ListParams{Usage: "sideways", Logger: testLogger()})

		var ve *ValidationError
		require.ErrorAs(t, err, &ve)
		assert.Equal(t, MsgInvalidUsageFilter, ve.Message)
		assert.Equal(t, []FieldError{{Field: "usage", Message: "usage must be upstream, downstream or identity"}}, ve.Fields)
		assert.Empty(t, *h.calls)
	})

	t.Run("empty store", func(t *testing.T) {
		h := newTestService(t)

		result, err := h.svc.List(ListParams{Logger: testLogger()})
		require.NoError(t, err)

		assert.Empty(t, result.Items)
		assert.Equal(t, 0, result.TotalBytes)
	})

	t.Run("storage failure", func(t *testing.T) {
		h := newTestService(t)
		h.db.listErr = errors.New("database unavailable")

		_, err := h.svc.List(ListParams{Logger: testLogger()})

		var persistErr *PersistError
		require.ErrorAs(t, err, &persistErr)
		assert.Equal(t, OpList, persistErr.Op)
	})
}

func TestGetAndGetByName(t *testing.T) {
	h := newTestService(t)
	h.db.certs["cert-1"] = &models.StoredCertificate{UUID: "cert-1", Name: "upstream"}

	byID, err := h.svc.Get("cert-1")
	require.NoError(t, err)
	assert.Equal(t, "upstream", byID.Certificate.Name)

	byName, err := h.svc.GetByName("upstream")
	require.NoError(t, err)
	assert.Equal(t, "cert-1", byName.Certificate.UUID)

	_, err = h.svc.Get("missing")
	assert.ErrorIs(t, err, ErrNotFound)

	_, err = h.svc.GetByName("missing")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestExtractMetadata_NoCertificate(t *testing.T) {
	_, _, _, _, err := ExtractMetadata([]byte("not-a-pem"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no valid certificate found")
}
