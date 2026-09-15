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

package tlsauth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

func generateTestCert(t *testing.T, cn string, uris []*url.URL) *x509.Certificate {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: cn},
		URIs:         uris,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert
}

func TestPeerIdentity(t *testing.T) {
	t.Run("prefers the first SAN URI over CommonName", func(t *testing.T) {
		spiffeID, err := url.Parse("spiffe://cluster.local/ns/gw/sa/envoy")
		require.NoError(t, err)
		cert := generateTestCert(t, "envoy-router", []*url.URL{spiffeID})
		assert.Equal(t, "spiffe://cluster.local/ns/gw/sa/envoy", PeerIdentity(cert))
	})

	t.Run("falls back to CommonName when there is no SAN URI", func(t *testing.T) {
		cert := generateTestCert(t, "policy-engine", nil)
		assert.Equal(t, "policy-engine", PeerIdentity(cert))
	})
}

func TestAllowedSet(t *testing.T) {
	set := AllowedSet([]string{"a", "b", "a"})
	assert.True(t, set["a"])
	assert.True(t, set["b"])
	assert.False(t, set["c"])
	assert.Len(t, set, 2)

	assert.Empty(t, AllowedSet(nil))
}

func TestVerifyStreamPeer(t *testing.T) {
	t.Run("no peer info in context is unauthenticated", func(t *testing.T) {
		err := VerifyStreamPeer(context.Background(), AllowedSet([]string{"x"}))
		assert.Error(t, err)
	})

	t.Run("peer info without TLS auth info is unauthenticated", func(t *testing.T) {
		p := &peer.Peer{Addr: &net.TCPAddr{}}
		ctx := peer.NewContext(context.Background(), p)
		err := VerifyStreamPeer(ctx, AllowedSet([]string{"x"}))
		assert.Error(t, err)
	})

	t.Run("TLS peer with no client certificate is unauthenticated", func(t *testing.T) {
		p := &peer.Peer{
			Addr:     &net.TCPAddr{},
			AuthInfo: credentials.TLSInfo{},
		}
		ctx := peer.NewContext(context.Background(), p)
		err := VerifyStreamPeer(ctx, AllowedSet([]string{"x"}))
		assert.Error(t, err)
	})

	t.Run("allowed identity passes", func(t *testing.T) {
		cert := generateTestCert(t, "envoy-router", nil)
		p := makeTLSPeer(cert)
		ctx := peer.NewContext(context.Background(), p)
		err := VerifyStreamPeer(ctx, AllowedSet([]string{"envoy-router"}))
		assert.NoError(t, err)
	})

	t.Run("unlisted identity is rejected even with a valid client cert", func(t *testing.T) {
		cert := generateTestCert(t, "unknown-caller", nil)
		p := makeTLSPeer(cert)
		ctx := peer.NewContext(context.Background(), p)
		err := VerifyStreamPeer(ctx, AllowedSet([]string{"envoy-router"}))
		assert.Error(t, err)
	})
}

func makeTLSPeer(cert *x509.Certificate) *peer.Peer {
	return &peer.Peer{
		Addr: &net.TCPAddr{},
		AuthInfo: credentials.TLSInfo{
			State: tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}},
		},
	}
}
