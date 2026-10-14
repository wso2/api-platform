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

package xds

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/certstore"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption/aesgcm"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/testutil/pki"
)

// fakeSDSStorage implements only the certificate lookups GetSecrets reaches;
// any other method panics.
type fakeSDSStorage struct {
	storage.Storage
	certs []*models.StoredCertificate
	// listErr, when set, makes ListCertificates fail.
	listErr error
}

func (f *fakeSDSStorage) ListCertificates() ([]*models.StoredCertificate, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.certs, nil
}

func (f *fakeSDSStorage) ListCertificatesByUsage(usage string) ([]*models.StoredCertificate, error) {
	var filtered []*models.StoredCertificate
	for _, cert := range f.certs {
		effective := cert.Usage
		if effective == "" {
			effective = models.CertificateUsageUpstream
		}
		if effective == usage {
			filtered = append(filtered, cert)
		}
	}
	return filtered, nil
}

func (f *fakeSDSStorage) GetCertificateByName(name string) (*models.StoredCertificate, error) {
	for _, cert := range f.certs {
		if cert.Name == name {
			return cert, nil
		}
	}
	return nil, fmt.Errorf("certificate %q not found", name)
}

// testXDSEncryptionManager builds an AES-GCM provider backed by a temporary
// key.
func testXDSEncryptionManager(t *testing.T) *encryption.ProviderManager {
	t.Helper()
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "v1.key")
	key := make([]byte, aesgcm.AESKeySize)
	_, err := rand.Read(key)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyPath, key, 0600))

	logger := createTestLogger()
	provider, err := aesgcm.NewAESGCMProvider([]aesgcm.KeyConfig{{Version: "v1", FilePath: keyPath}}, logger)
	require.NoError(t, err)
	mgr, err := encryption.NewProviderManager([]encryption.EncryptionProvider{provider}, logger)
	require.NoError(t, err)
	return mgr
}

// encryptForStorage encrypts and marshals plaintext as stored in
// StoredCertificate.PrivateKeyCiphertext.
func encryptForStorage(t *testing.T, mgr *encryption.ProviderManager, plaintext []byte) string {
	t.Helper()
	payload, err := mgr.Encrypt(plaintext)
	require.NoError(t, err)
	return encryption.MarshalPayload(payload)
}

// secretsByName indexes a GetSecrets result by name.
func secretsByName(t *testing.T, secrets []types.Resource) map[string]*tlsv3.Secret {
	t.Helper()
	out := make(map[string]*tlsv3.Secret, len(secrets))
	for _, res := range secrets {
		s, ok := res.(*tlsv3.Secret)
		require.True(t, ok, "GetSecrets returned a non-Secret resource: %T", res)
		out[s.GetName()] = s
	}
	return out
}

// writeListenerCertFiles writes a freshly generated cert/key pair to temp
// files and returns their paths, for NewSDSSecretManager's listener-cert args.
func writeListenerCertFiles(t *testing.T) (certPath, keyPath string) {
	t.Helper()
	entity := pki.NewSelfSignedLeaf(t, "listener")
	dir := t.TempDir()
	certPath = filepath.Join(dir, "listener.crt")
	keyPath = filepath.Join(dir, "listener.key")
	require.NoError(t, os.WriteFile(certPath, entity.PEM(), 0o600))
	require.NoError(t, os.WriteFile(keyPath, entity.KeyPEM(), 0o600))
	return certPath, keyPath
}

func TestSDSSecretManager_GetSecrets_UpstreamAndClientRows_HTTPSEnabled(t *testing.T) {
	logger := createTestLogger()
	upstreamCert := pki.NewRootCA(t, "SDS Upstream CA")
	clientCert := pki.NewRootCA(t, "SDS Client CA")

	db := &fakeSDSStorage{certs: []*models.StoredCertificate{
		{UUID: "upstream-1", Name: "upstream-ca", Certificate: upstreamCert.PEM(), Usage: models.CertificateUsageUpstream},
		{UUID: "client-1", Name: "client-ca", Certificate: clientCert.PEM(), Usage: models.CertificateUsageDownstream},
	}}
	cs := certstore.NewCertStore(logger, db, "", "")
	_, err := cs.LoadCertificates()
	require.NoError(t, err)

	certPath, keyPath := writeListenerCertFiles(t)
	sm := NewSDSSecretManager(cs, nil, "test-node", logger, certPath, keyPath, true)

	secrets, err := sm.GetSecrets(nil)
	require.NoError(t, err)
	require.Len(t, secrets, 3, "expected upstream_ca_bundle, downstream_client_ca and downstream_listener_cert")

	byName := secretsByName(t, secrets)
	assert.Contains(t, byName, SecretNameUpstreamCA)
	assert.Contains(t, byName, SecretNameDownstreamClientCA)
	assert.Contains(t, byName, SecretNameDownstreamListenerCert)

	// downstream_client_ca carries only the client PEM, with ACCEPT_UNTRUSTED.
	clientSecret := byName[SecretNameDownstreamClientCA]
	validationCtx, ok := clientSecret.GetType().(*tlsv3.Secret_ValidationContext)
	require.True(t, ok, "downstream_client_ca must be a Secret_ValidationContext, got %T", clientSecret.GetType())
	trustedCA := string(validationCtx.ValidationContext.GetTrustedCa().GetInlineBytes())
	assert.Contains(t, trustedCA, string(clientCert.PEM()))
	assert.NotContains(t, trustedCA, string(upstreamCert.PEM()))
	assert.Equal(t, tlsv3.CertificateValidationContext_ACCEPT_UNTRUSTED, validationCtx.ValidationContext.GetTrustChainVerification())

	// upstream_ca_bundle must never carry the client-CA pool's certificate.
	upstreamSecret := byName[SecretNameUpstreamCA]
	upstreamValidationCtx, ok := upstreamSecret.GetType().(*tlsv3.Secret_ValidationContext)
	require.True(t, ok, "upstream_ca_bundle must be a Secret_ValidationContext, got %T", upstreamSecret.GetType())
	upstreamTrustedCA := string(upstreamValidationCtx.ValidationContext.GetTrustedCa().GetInlineBytes())
	assert.Contains(t, upstreamTrustedCA, string(upstreamCert.PEM()))
	assert.NotContains(t, upstreamTrustedCA, string(clientCert.PEM()))
}

func TestSDSSecretManager_GetSecrets_NoClientRows_ClientCASecretAbsent(t *testing.T) {
	logger := createTestLogger()
	upstreamCert := pki.NewRootCA(t, "SDS Upstream Only CA")

	db := &fakeSDSStorage{certs: []*models.StoredCertificate{
		{UUID: "upstream-1", Name: "upstream-ca", Certificate: upstreamCert.PEM(), Usage: models.CertificateUsageUpstream},
	}}
	cs := certstore.NewCertStore(logger, db, "", "")
	_, err := cs.LoadCertificates()
	require.NoError(t, err)

	certPath, keyPath := writeListenerCertFiles(t)
	sm := NewSDSSecretManager(cs, nil, "test-node", logger, certPath, keyPath, true)

	secrets, err := sm.GetSecrets(nil)
	require.NoError(t, err)

	byName := secretsByName(t, secrets)
	assert.Contains(t, byName, SecretNameUpstreamCA)
	assert.Contains(t, byName, SecretNameDownstreamListenerCert)
	assert.NotContains(t, byName, SecretNameDownstreamClientCA, "an empty client-CA pool must omit the secret, not error")
}

func TestSDSSecretManager_GetSecrets_HTTPSEnabled_UnreadableListenerCert_Errors(t *testing.T) {
	logger := createTestLogger()
	db := &fakeSDSStorage{}
	cs := certstore.NewCertStore(logger, db, "", "")

	sm := NewSDSSecretManager(cs, nil, "test-node", logger, "/nonexistent/does-not-exist.crt", "/nonexistent/does-not-exist.key", true)

	_, err := sm.GetSecrets(nil)
	assert.Error(t, err, "a failed listener-cert read must fail the whole snapshot, not be silently omitted")
}

func TestSDSSecretManager_GetSecrets_HTTPSDisabled_NoListenerSecretNoError(t *testing.T) {
	logger := createTestLogger()
	db := &fakeSDSStorage{}
	cs := certstore.NewCertStore(logger, db, "", "")

	// Deliberately invalid paths: httpsEnabled false must mean these are never read.
	sm := NewSDSSecretManager(cs, nil, "test-node", logger, "/nonexistent/does-not-exist.crt", "/nonexistent/does-not-exist.key", false)

	secrets, err := sm.GetSecrets(nil)
	require.NoError(t, err)

	byName := secretsByName(t, secrets)
	assert.NotContains(t, byName, SecretNameDownstreamListenerCert)
}

// ============================================================================
// Gateway-identity and per-upstream-trust SDS secrets (mTLS outbound)
// ============================================================================

// The identity secret carries the decrypted key, which appears in no other
// secret.
func TestSDSSecretManager_GetSecrets_GatewayIdentity_DecryptedKeyOnlyInSecret(t *testing.T) {
	logger := createTestLogger()
	identity := pki.NewSelfSignedLeaf(t, "gateway-a")
	mgr := testXDSEncryptionManager(t)

	db := &fakeSDSStorage{certs: []*models.StoredCertificate{
		{
			UUID: "identity-1", Name: "out-identity-a", Certificate: identity.PEM(),
			Usage: models.CertificateUsageIdentity, PrivateKeyCiphertext: encryptForStorage(t, mgr, identity.KeyPEM()),
		},
	}}
	cs := certstore.NewCertStore(logger, db, "", "")
	cs.SetEncryptionManager(mgr)

	sm := NewSDSSecretManager(cs, nil, "test-node", logger, "", "", false)

	secrets, err := sm.GetSecrets([]UpstreamTLSSecretRef{{IdentityName: "out-identity-a"}})
	require.NoError(t, err)

	byName := secretsByName(t, secrets)
	identitySecret, ok := byName[GatewayIdentitySecretName("out-identity-a")]
	require.True(t, ok, "expected a gateway_identity:out-identity-a secret, got %v", byName)

	tlsCert, ok := identitySecret.GetType().(*tlsv3.Secret_TlsCertificate)
	require.True(t, ok, "gateway identity secret must be a Secret_TlsCertificate, got %T", identitySecret.GetType())
	assert.Equal(t, identity.PEM(), tlsCert.TlsCertificate.GetCertificateChain().GetInlineBytes())
	assert.Equal(t, identity.KeyPEM(), tlsCert.TlsCertificate.GetPrivateKey().GetInlineBytes(),
		"the secret must carry the DECRYPTED key, not the ciphertext")

	for name, secret := range byName {
		if name == GatewayIdentitySecretName("out-identity-a") {
			continue
		}
		serialized := secret.String()
		assert.NotContains(t, serialized, string(identity.KeyPEM()),
			"secret %q must not carry the gateway identity's private key", name)
	}
}

// The per-upstream trust secret carries exactly the named trustedCAs.
func TestSDSSecretManager_GetSecrets_PerUpstreamTrust_ExactTrustedCAs(t *testing.T) {
	logger := createTestLogger()
	trusted1 := pki.NewRootCA(t, "Per-Upstream Trusted CA 1")
	trusted2 := pki.NewRootCA(t, "Per-Upstream Trusted CA 2")
	excluded := pki.NewRootCA(t, "Per-Upstream Excluded CA")

	db := &fakeSDSStorage{certs: []*models.StoredCertificate{
		{UUID: "trust-1", Name: "out-backend-ca-1", Certificate: trusted1.PEM(), Usage: models.CertificateUsageUpstream},
		{UUID: "trust-2", Name: "out-backend-ca-2", Certificate: trusted2.PEM(), Usage: models.CertificateUsageUpstream},
		{UUID: "trust-3", Name: "out-backend-ca-excluded", Certificate: excluded.PEM(), Usage: models.CertificateUsageUpstream},
	}}
	cs := certstore.NewCertStore(logger, db, "", "")

	sm := NewSDSSecretManager(cs, nil, "test-node", logger, "", "", false)

	secrets, err := sm.GetSecrets([]UpstreamTLSSecretRef{{
		APIHandle: "out-partner-api", DefinitionName: "partner-a",
		TrustedCANames: []string{"out-backend-ca-1", "out-backend-ca-2"},
	}})
	require.NoError(t, err)

	byName := secretsByName(t, secrets)
	secretName := UpstreamCAValidationContextSecretName("out-partner-api", "partner-a")
	trustSecret, ok := byName[secretName]
	require.True(t, ok, "expected secret %q, got %v", secretName, byName)

	validationCtx, ok := trustSecret.GetType().(*tlsv3.Secret_ValidationContext)
	require.True(t, ok, "expected a Secret_ValidationContext, got %T", trustSecret.GetType())
	bundle := string(validationCtx.ValidationContext.GetTrustedCa().GetInlineBytes())
	assert.Contains(t, bundle, string(trusted1.PEM()))
	assert.Contains(t, bundle, string(trusted2.PEM()))
	assert.NotContains(t, bundle, string(excluded.PEM()), "must carry exactly the named trustedCAs, not the whole upstream pool")
}

// Two refs to the same definition produce a single secret.
func TestSDSSecretManager_GetSecrets_PerUpstreamTrust_DedupedAcrossRefs(t *testing.T) {
	logger := createTestLogger()
	trusted := pki.NewRootCA(t, "Dedup Trusted CA")

	db := &fakeSDSStorage{certs: []*models.StoredCertificate{
		{UUID: "trust-1", Name: "out-backend-ca", Certificate: trusted.PEM(), Usage: models.CertificateUsageUpstream},
	}}
	cs := certstore.NewCertStore(logger, db, "", "")

	sm := NewSDSSecretManager(cs, nil, "test-node", logger, "", "", false)

	secrets, err := sm.GetSecrets([]UpstreamTLSSecretRef{
		{APIHandle: "out-partner-api", DefinitionName: "partner-a", TrustedCANames: []string{"out-backend-ca"}},
		{APIHandle: "out-partner-api", DefinitionName: "partner-a", TrustedCANames: []string{"out-backend-ca"}},
	})
	require.NoError(t, err)

	count := 0
	secretName := UpstreamCAValidationContextSecretName("out-partner-api", "partner-a")
	for _, res := range secrets {
		if s, ok := res.(*tlsv3.Secret); ok && s.GetName() == secretName {
			count++
		}
	}
	assert.Equal(t, 1, count, "expected exactly one secret resource named %q, got %d", secretName, count)
}
