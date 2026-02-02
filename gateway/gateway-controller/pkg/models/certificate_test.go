/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStoredCertificate_Fields(t *testing.T) {
	now := time.Now()
	notBefore := now.Add(-24 * time.Hour)
	notAfter := now.Add(365 * 24 * time.Hour)

	cert := &StoredCertificate{
		ID:          "cert-123",
		Name:        "test-cert",
		Certificate: []byte("-----BEGIN CERTIFICATE-----\nMIIB..."),
		Subject:     "CN=test.example.com",
		Issuer:      "CN=Test CA",
		NotBefore:   notBefore,
		NotAfter:    notAfter,
		CertCount:   1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	assert.Equal(t, "cert-123", cert.ID)
	assert.Equal(t, "test-cert", cert.Name)
	assert.NotEmpty(t, cert.Certificate)
	assert.Equal(t, "CN=test.example.com", cert.Subject)
	assert.Equal(t, "CN=Test CA", cert.Issuer)
	assert.Equal(t, notBefore, cert.NotBefore)
	assert.Equal(t, notAfter, cert.NotAfter)
	assert.Equal(t, 1, cert.CertCount)
	assert.Equal(t, now, cert.CreatedAt)
	assert.Equal(t, now, cert.UpdatedAt)
}

func TestStoredCertificate_EmptyCertificate(t *testing.T) {
	cert := &StoredCertificate{
		ID:          "empty-cert",
		Name:        "empty",
		Certificate: []byte{},
		CertCount:   0,
	}

	assert.Empty(t, cert.Certificate)
	assert.Equal(t, 0, cert.CertCount)
}

func TestStoredCertificate_MultipleCerts(t *testing.T) {
	cert := &StoredCertificate{
		ID:          "bundle-cert",
		Name:        "cert-bundle",
		Certificate: []byte("-----BEGIN CERTIFICATE-----\ncert1\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\ncert2\n-----END CERTIFICATE-----"),
		CertCount:   2,
	}

	assert.Equal(t, 2, cert.CertCount)
	assert.Contains(t, string(cert.Certificate), "cert1")
	assert.Contains(t, string(cert.Certificate), "cert2")
}
