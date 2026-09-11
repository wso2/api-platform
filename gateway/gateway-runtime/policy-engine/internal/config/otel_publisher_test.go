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
	// The default endpoint is plaintext http://, which now requires an explicit
	// opt-in. Granting it here keeps the transport question out of every case
	// that is really about batching, retries or TLS material; the gate itself is
	// covered by TestValidate_OTelPublisherPlaintextTransport, and a case can
	// still switch it back off via mutate.
	cfg.Analytics.Publishers.OTel.AllowInsecureTransport = true
	mutate(&cfg.Analytics.Publishers.OTel)
	return cfg
}

// The plaintext gate, matching traffic_logging.http.allow_insecure_transport:
// analytics records carry API keys and consumer identity, and every Headers
// value is an intake credential put on the wire on each export.
func TestValidate_OTelPublisherPlaintextTransport(t *testing.T) {
	const plaintext = "http://collector.example.com:4318/v1/logs"

	t.Run("http without the flag is refused", func(t *testing.T) {
		cfg := otelConfig(func(o *OTelPublisherConfig) {
			o.Endpoint = plaintext
			o.AllowInsecureTransport = false
		})
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "allow_insecure_transport is false")
	})

	t.Run("http with the flag is allowed", func(t *testing.T) {
		cfg := otelConfig(func(o *OTelPublisherConfig) {
			o.Endpoint = plaintext
			o.AllowInsecureTransport = true
		})
		assert.NoError(t, cfg.Validate())
	})

	t.Run("loopback is not exempt", func(t *testing.T) {
		// The sibling sink grants no loopback exemption, so neither does this:
		// "localhost" inside a container is not the operator's machine.
		for _, host := range []string{"http://127.0.0.1:4318/v1/logs", "http://localhost:4318/v1/logs"} {
			cfg := otelConfig(func(o *OTelPublisherConfig) {
				o.Endpoint = host
				o.AllowInsecureTransport = false
			})
			err := cfg.Validate()
			require.Error(t, err, "%s must still require the opt-in", host)
			assert.Contains(t, err.Error(), "allow_insecure_transport is false")
		}
	})

	t.Run("https needs no flag", func(t *testing.T) {
		cfg := otelConfig(func(o *OTelPublisherConfig) {
			o.Endpoint = "https://collector.example.com:4318/v1/logs"
			o.AllowInsecureTransport = false
		})
		assert.NoError(t, cfg.Validate())
	})

	t.Run("credentialed plaintext is still gated", func(t *testing.T) {
		// CWE-319: Headers authenticate to the intake, so plaintext exposes the
		// credential itself. Covered by the same gate rather than a second rule.
		cfg := otelConfig(func(o *OTelPublisherConfig) {
			o.Endpoint = plaintext
			o.AllowInsecureTransport = false
			o.Headers = map[string]string{"x-api-key": "s3cr3t"}
		})
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "allow_insecure_transport is false")
		assert.NotContains(t, err.Error(), "s3cr3t", "the error must not echo the credential")
	})

	t.Run("a non-http scheme names the opt-in", func(t *testing.T) {
		cfg := otelConfig(func(o *OTelPublisherConfig) { o.Endpoint = "ftp://collector:4318/v1/logs" })
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be https (or http with allow_insecure_transport)")
	})
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
		"zero queue capacity":   func(o *OTelPublisherConfig) { o.QueueCapacity = 0 },
		// A queue smaller than a batch can never fill one.
		"queue smaller than batch": func(o *OTelPublisherConfig) { o.QueueCapacity = 10; o.BatchSize = 100 },
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

	// The two accepted values are shared with traffic_logging.http via
	// QueueDropNew/QueueDropOldest, so both blocks must keep accepting both.
	t.Run("both drop policies are accepted", func(t *testing.T) {
		for _, policy := range []string{QueueDropNew, QueueDropOldest, " DROP_OLDEST "} {
			cfg := otelConfig(func(o *OTelPublisherConfig) { o.OnQueueFull = policy })
			assert.NoError(t, cfg.Validate(), "policy %q", policy)
		}
	})

	// Silently defaulting an unrecognised value would give an operator who asked
	// for drop_oldest the opposite behaviour.
	t.Run("unknown drop policy is an error", func(t *testing.T) {
		for _, policy := range []string{"", "drop", "evict_oldest", "drop_newest"} {
			cfg := otelConfig(func(o *OTelPublisherConfig) { o.OnQueueFull = policy })
			err := cfg.Validate()
			require.Error(t, err, "policy %q", policy)
			assert.Contains(t, err.Error(), "on_queue_full")
		}
	})

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

func TestValidate_OTelPublisherRetry(t *testing.T) {
	t.Run("defaults are retry-enabled", func(t *testing.T) {
		cfg := defaultConfig().Analytics.Publishers.OTel
		assert.Equal(t, 3, cfg.MaxRetries)
		assert.Equal(t, time.Second, cfg.RetryBackoff)
		assert.Equal(t, DefaultOTelRetryAbortQueueRatio, cfg.RetryAbortQueueRatio)
		assert.Equal(t, OTelCompressionNone, cfg.Compression)
	})

	// Retries off is a legitimate choice, so 0 must pass while negative fails.
	t.Run("zero retries is allowed", func(t *testing.T) {
		cfg := otelConfig(func(o *OTelPublisherConfig) { o.MaxRetries = 0; o.RetryBackoff = 0 })
		assert.NoError(t, cfg.Validate())
	})

	rejected := map[string]func(*OTelPublisherConfig){
		"negative retries":        func(o *OTelPublisherConfig) { o.MaxRetries = -1 },
		"retries without backoff": func(o *OTelPublisherConfig) { o.MaxRetries = 3; o.RetryBackoff = 0 },
		"negative backoff":        func(o *OTelPublisherConfig) { o.MaxRetries = 3; o.RetryBackoff = -time.Second },
		"abort ratio above one":   func(o *OTelPublisherConfig) { o.RetryAbortQueueRatio = 1.5 },
		"negative abort ratio":    func(o *OTelPublisherConfig) { o.RetryAbortQueueRatio = -0.1 },
		"unknown compression":     func(o *OTelPublisherConfig) { o.Compression = "zstd" },
	}
	for name, mutate := range rejected {
		t.Run(name, func(t *testing.T) {
			err := otelConfig(mutate).Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "analytics.publishers.otel")
		})
	}

	t.Run("compression values accepted", func(t *testing.T) {
		for _, c := range []string{"", OTelCompressionNone, OTelCompressionGzip, " GZIP "} {
			cfg := otelConfig(func(o *OTelPublisherConfig) { o.Compression = c })
			assert.NoError(t, cfg.Validate(), "compression %q", c)
		}
	})

	// The abort depth is a fraction of capacity, and must never round down to
	// zero for a non-zero ratio: that would silently disable the check.
	t.Run("abort depth", func(t *testing.T) {
		cases := []struct {
			capacity int
			ratio    float64
			want     int
		}{
			{10000, 0.5, 5000},
			{4, 0.5, 2},
			{1, 0.5, 1},   // rounds to 0, floored to 1
			{10000, 0, 0}, // explicitly disabled
		}
		for _, tc := range cases {
			cfg := OTelPublisherConfig{QueueCapacity: tc.capacity, RetryAbortQueueRatio: tc.ratio}
			assert.Equal(t, tc.want, cfg.EffectiveRetryAbortDepth(),
				"capacity %d ratio %v", tc.capacity, tc.ratio)
		}
	})
}
