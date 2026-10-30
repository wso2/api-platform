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

package shared

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestGatewayVersionResolvesFromVERSIONFile verifies the gateway version source.
func TestGatewayVersionResolvesFromVERSIONFile(t *testing.T) {
	root, ok := RepoRootFromCallerFile()
	require.True(t, ok,
		"could not locate the repository root from this package's source path — the resolver "+
			"in version.go is broken, and gatewayVersion() is silently returning its fallback")

	raw, err := os.ReadFile(filepath.Join(root, "gateway", "VERSION"))
	require.NoError(t, err)
	onDisk := strings.TrimSpace(string(raw))
	require.NotEmpty(t, onDisk, "gateway/VERSION is empty")

	require.Equal(t, onDisk, gatewayVersion(),
		"gatewayVersion() returned %q but gateway/VERSION says %q", gatewayVersion(), onDisk)
}

// TestGatewayBuildYAMLAgreesWithVERSION verifies the product build version metadata.
func TestGatewayBuildYAMLAgreesWithVERSION(t *testing.T) {
	root, ok := RepoRootFromCallerFile()
	require.True(t, ok)

	buildFile := filepath.Join(root, "gateway", "build.yaml")
	raw, err := os.ReadFile(buildFile)
	if err != nil {
		t.Skipf("no readable gateway/build.yaml to cross-check (%v)", err)
	}

	var parsed struct {
		Gateway struct {
			Version string `yaml:"version"`
		} `yaml:"gateway"`
	}
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		t.Skipf("gateway/build.yaml is not in the expected shape (%v)", err)
	}
	if parsed.Gateway.Version == "" {
		t.Skip("gateway/build.yaml declares no gateway.version")
	}

	require.Equal(t, gatewayVersion(), parsed.Gateway.Version,
		"gateway/VERSION says %q but gateway/build.yaml says %q — `make build` tags images "+
			"from VERSION, so the suites would pin a tag build.yaml disagrees with",
		gatewayVersion(), parsed.Gateway.Version)
}

// TestGatewayRunImagesUseNormalNames verifies default gateway image references.
func TestGatewayRunImagesUseNormalNames(t *testing.T) {
	t.Setenv(EnvCoverageMode, "")
	for _, ref := range []string{GatewayControllerRunImage(), GatewayRuntimeRunImage()} {
		require.NotContains(t, ref, "-coverage",
			"%s is a coverage-instrumented build; a default run collects no runtime "+
				"coverage, so it would pay the slowest build in the pipeline for nothing", ref)
		require.Contains(t, ref, ":"+gatewayVersion(),
			"%s is not pinned to gateway/VERSION", ref)
		require.NotContains(t, ref, ":latest",
			"%s uses :latest, which also exists in the registry — a developer who had not run "+
				"`make build` would silently test a released image instead of their own code", ref)
	}
}

// TestGatewayRunImagesRemainStableInCoverageMode verifies coverage image references.
func TestGatewayRunImagesRemainStableInCoverageMode(t *testing.T) {
	t.Setenv(EnvCoverageMode, "true")
	require.Equal(t, GatewayControllerImage(), GatewayControllerRunImage())
	require.Equal(t, GatewayRuntimeImage(), GatewayRuntimeRunImage())
}

// The steps trust the listener certificate and the gateway serves it, so the two must be the
// same certificate and it must be valid for the names the suite reaches the gateway by.
func TestGatewayListenerTLSIsOneCertificateValidForLocalhost(t *testing.T) {
	pair := GatewayListenerTLS()

	require.Same(t, pair, GatewayListenerTLS())
	_, err := tls.X509KeyPair(pair.CertPEM, pair.PrivateKeyPEM)
	require.NoError(t, err, "the certificate and key must belong together")

	block, _ := pem.Decode(pair.CertPEM)
	require.NotNil(t, block)
	certificate, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	roots.AddCert(certificate)
	for _, name := range []string{"localhost", "api.localhost"} {
		_, err := certificate.Verify(x509.VerifyOptions{DNSName: name, Roots: roots})
		require.NoError(t, err, name)
	}
	_, err = certificate.Verify(x509.VerifyOptions{DNSName: "example.com", Roots: roots})
	require.Error(t, err)
}

// The portal serves its certificate and the steps trust it, so the two must be one certificate,
// valid for the name containers reach the portal by and distinct from the control plane's.
func TestAPIPortalTLSIsItsOwnCertificateValidForThePortalHost(t *testing.T) {
	pair := APIPortalTLS()

	require.Same(t, pair, APIPortalTLS())
	_, err := tls.X509KeyPair(pair.CertPEM, pair.PrivateKeyPEM)
	require.NoError(t, err, "the certificate and key must belong together")
	require.NotEqual(t, ControlPlaneCrypto()["certs/cert.pem"], pair.CertPEM,
		"the portal must not borrow the control plane's certificate")

	block, _ := pem.Decode(pair.CertPEM)
	require.NotNil(t, block)
	certificate, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	roots.AddCert(certificate)
	for _, name := range []string{APIPortalHost, "localhost"} {
		_, err := certificate.Verify(x509.VerifyOptions{DNSName: name, Roots: roots})
		require.NoError(t, err, name)
	}
	_, err = certificate.Verify(x509.VerifyOptions{DNSName: "platform-api", Roots: roots})
	require.Error(t, err)
}

func TestAPIPortalSharedKeyIsGeneratedOnceAndItsHashMatches(t *testing.T) {
	key := APIPortalSharedKey()

	require.Regexp(t, "^[0-9a-f]{64}$", key)
	require.Equal(t, key, APIPortalSharedKey())
	sum := sha256.Sum256([]byte(key))
	require.Equal(t, hex.EncodeToString(sum[:]), APIPortalSharedKeyHash())
	require.NotEqual(t, key, APIPortalSharedKeyHash())
}

func TestAPIPortalClientTLSTrustsOnlyThePortalCertificate(t *testing.T) {
	config := APIPortalClientTLS()
	require.Equal(t, APIPortalHost, config.ServerName)
	require.True(t, config.RootCAs.Equal(portalOnlyPool(t)), "the pool must hold the portal certificate and nothing else")
}

func portalOnlyPool(t *testing.T) *x509.CertPool {
	t.Helper()
	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM(APIPortalTLS().CertPEM))
	return pool
}

func TestGatewayEncryptionKeyIsA32ByteKeyAndEachCallGetsACopy(t *testing.T) {
	key := GatewayEncryptionKey()

	require.Len(t, key, 32)
	require.NotEqual(t, make([]byte, 32), key)
	key[0] ^= 0xff
	require.NotEqual(t, key, GatewayEncryptionKey(), "mutating a returned key must not change the next one")
}

func TestAPIPortalCertificateCoversEveryPortalInstance(t *testing.T) {
	block, _ := pem.Decode(APIPortalTLS().CertPEM)
	require.NotNil(t, block)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	for _, host := range []string{
		APIPortalHost, APIPortalMultiOrganizationHost, APIPortalOtherOrgHost, "localhost",
	} {
		require.NoError(t, cert.VerifyHostname(host), host)
	}
	require.Error(t, cert.VerifyHostname("some-other-host"))
}
