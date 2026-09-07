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

package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// otelConfig returns a Config with analytics enabled and only the otel publisher
// selected, so Validate exercises validateOTelPublisherConfig.
func otelConfig(mutate func(*OTelPublisherConfig)) *Config {
	cfg := defaultConfig()
	cfg.Analytics.Enabled = true
	cfg.Analytics.EnabledPublishers = []string{"otel"}
	mutate(&cfg.Analytics.Publishers.OTel)
	return cfg
}

// writeSelfSignedPair writes a throwaway self-signed certificate and its key.
func writeSelfSignedPair(t *testing.T) (certPath, keyPath string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "otel-config-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	dir := t.TempDir()
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(certPath,
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	require.NoError(t, os.WriteFile(keyPath,
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600))
	return certPath, keyPath
}

func TestValidate_OTelPublisher(t *testing.T) {
	t.Run("defaults pass", func(t *testing.T) {
		assert.NoError(t, otelConfig(func(*OTelPublisherConfig) {}).Validate())
	})

	t.Run("https endpoint passes", func(t *testing.T) {
		cfg := otelConfig(func(o *OTelPublisherConfig) {
			o.Endpoint = "https://otlp.vendor.example.com/v1/logs"
		})
		assert.NoError(t, cfg.Validate())
	})

	rejected := map[string]func(*OTelPublisherConfig){
		"missing endpoint":      func(o *OTelPublisherConfig) { o.Endpoint = "" },
		"endpoint without host": func(o *OTelPublisherConfig) { o.Endpoint = "/v1/logs" },
		"non-http scheme":       func(o *OTelPublisherConfig) { o.Endpoint = "grpc://collector:4317" },
		"missing service name":  func(o *OTelPublisherConfig) { o.ServiceName = "" },
		"zero batch size":       func(o *OTelPublisherConfig) { o.BatchSize = 0 },
		"zero queue size":       func(o *OTelPublisherConfig) { o.QueueSize = 0 },
		// A queue smaller than a batch can never fill one.
		"queue smaller than batch": func(o *OTelPublisherConfig) { o.QueueSize = 10; o.BatchSize = 100 },
		"zero flush interval":      func(o *OTelPublisherConfig) { o.FlushInterval = 0 },
		"negative timeout":         func(o *OTelPublisherConfig) { o.Timeout = -time.Second },
	}
	for name, mutate := range rejected {
		t.Run(name, func(t *testing.T) {
			err := otelConfig(mutate).Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "analytics.publishers.otel")
		})
	}

	// An unknown publisher name must fail rather than be silently skipped.
	t.Run("unknown publisher name is an error", func(t *testing.T) {
		cfg := otelConfig(func(*OTelPublisherConfig) {})
		cfg.Analytics.EnabledPublishers = []string{"otlp"}
		assert.Error(t, cfg.Validate())
	})

	// Nothing under the block is validated while the publisher is not enabled.
	t.Run("not enabled skips validation", func(t *testing.T) {
		cfg := defaultConfig()
		cfg.Analytics.Enabled = true
		cfg.Analytics.EnabledPublishers = nil
		cfg.Analytics.Publishers.OTel.Endpoint = ""
		cfg.Analytics.Publishers.OTel.TLS = OTelTLSConfig{CAFile: "/nonexistent/ca.pem"}
		assert.NoError(t, cfg.Validate())
	})
}

// TLS material is loaded at startup so a bad path fails there rather than on the
// first export, hours later.
func TestValidate_OTelPublisherTLS(t *testing.T) {
	certPath, keyPath := writeSelfSignedPair(t)

	t.Run("ca file plus mTLS pair passes", func(t *testing.T) {
		cfg := otelConfig(func(o *OTelPublisherConfig) {
			o.Endpoint = "https://collector.internal:4318/v1/logs"
			o.TLS = OTelTLSConfig{CAFile: certPath, CertFile: certPath, KeyFile: keyPath}
		})
		assert.NoError(t, cfg.Validate())
	})

	garbage := filepath.Join(t.TempDir(), "garbage.pem")
	require.NoError(t, os.WriteFile(garbage, []byte("not a certificate"), 0o600))
	absent := filepath.Join(t.TempDir(), "absent.pem")

	rejected := map[string]OTelTLSConfig{
		"unreadable ca file":  {CAFile: absent},
		"ca file with no PEM": {CAFile: garbage},
		"cert without key":    {CertFile: certPath},
		"key without cert":    {KeyFile: keyPath},
		"unloadable pair":     {CertFile: keyPath, KeyFile: certPath},
	}
	for name, tlsCfg := range rejected {
		t.Run(name, func(t *testing.T) {
			cfg := otelConfig(func(o *OTelPublisherConfig) {
				o.Endpoint = "https://collector.internal:4318/v1/logs"
				o.TLS = tlsCfg
			})
			err := cfg.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "analytics.publishers.otel.tls")
		})
	}

	// insecure_skip_verify only warns — it must not block startup, or an
	// operator debugging a cert problem has no way through.
	t.Run("insecure_skip_verify warns but passes", func(t *testing.T) {
		cfg := otelConfig(func(o *OTelPublisherConfig) {
			o.Endpoint = "https://collector.internal:4318/v1/logs"
			o.TLS = OTelTLSConfig{InsecureSkipVerify: true}
		})
		assert.NoError(t, cfg.Validate())
	})
}
