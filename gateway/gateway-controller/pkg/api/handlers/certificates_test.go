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

package handlers

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/clientca"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption/aesgcm"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/lazyresourcexds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/testutil/pki"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

// Valid test certificate (generated with openssl)
const validTestCert = `-----BEGIN CERTIFICATE-----
MIIDkzCCAnugAwIBAgIUI92o4hdPPhGB4BFivBQnTe/RRjMwDQYJKoZIhvcNAQEL
BQAwWTELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAkNBMQswCQYDVQQHDAJTRjENMAsG
A1UECgwEVGVzdDELMAkGA1UECwwCSVQxFDASBgNVBAMMC2V4YW1wbGUuY29tMB4X
DTI2MDIwNjA5MzIwNloXDTI3MDIwNjA5MzIwNlowWTELMAkGA1UEBhMCVVMxCzAJ
BgNVBAgMAkNBMQswCQYDVQQHDAJTRjENMAsGA1UECgwEVGVzdDELMAkGA1UECwwC
SVQxFDASBgNVBAMMC2V4YW1wbGUuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A
MIIBCgKCAQEAiMGvSiOweFnDEfeyspV9BK/d/QXXGPey91qjtP3QkToIEbQQngM1
L8omo4dVoyqivbr5ngAGg1dSmwYC2EudyDg7fvERydIhjhCxLG6aN8Zn41AxmNzj
X0cZjM/o/38PI5QSYaC18J5cvz4er9ZtEiRGa0Jm5O22O7BlcOGDxy1FCENmsLvs
iVpLYg193j8gzFc1QrfBG3Fkpil5VVLcdIDeFyuXFOO4/nRLLefOCIsMVebmi7hx
6tFaMrmZ2jZV7nbVHFEJ6JKPpPg+4fWiG5bP0YkG/jGeGdVUAIr56z37ZKw7v2OK
iu4vA2YbKl8nO0VP4zbnk21bUU/xYTbGzwIDAQABo1MwUTAdBgNVHQ4EFgQU1tRl
0lD0zDHgIT4vJblGH6Q9hTswHwYDVR0jBBgwFoAU1tRl0lD0zDHgIT4vJblGH6Q9
hTswDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAQEAUxndGWNtdNPk
we7+UrZN8oZhE3bWdN6YB+R66dz6jDqjQxg7H5Nj/xoXrYlJ1Zxm67jpFCsZxOZc
xRGZVCp8vJEIPbMcbAxqbJTBTOjNIXdIwJ0ZQVPdT56eJPTPNgvdcI2y2cZ+IkZl
7iZ+PkQeoy0pI/P8aYShLdsJLeDxuFDFbSN7Y/a5Sm6nfwjlU6TABy5SdgfSbqKD
NbLeQy2E3Qy/SIsy/361VLbUNWyK5LyJLdIrDd2n+gsmzQ/cgV7b/fsDw22BmELB
RMVr21DnDN4l9BDDs8384GT2VOkW+6+Xl6co6gwNYSVRhsdOlDe8NkFtpe4BFg9H
/lNmxfnpPg==
-----END CERTIFICATE-----`

// Certificate chain with two certificates
const certChain = validTestCert + `
-----BEGIN CERTIFICATE-----
MIIDkzCCAnugAwIBAgIUI92o4hdPPhGB4BFivBQnTe/RRjMwDQYJKoZIhvcNAQEL
BQAwWTELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAkNBMQswCQYDVQQHDAJTRjENMAsG
A1UECgwEVGVzdDELMAkGA1UECwwCSVQxFDASBgNVBAMMC2V4YW1wbGUuY29tMB4X
DTI2MDIwNjA5MzIwNloXDTI3MDIwNjA5MzIwNlowWTELMAkGA1UEBhMCVVMxCzAJ
BgNVBAgMAkNBMQswCQYDVQQHDAJTRjENMAsG
A1UECgwEVGVzdDELMAkGA1UECwwCSVQxFDASBgNVBAMMC2V4YW1wbGUuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A
MIIBCgKCAQEAiMGvSiOweFnDEfeyspV9BK/d/QXXGPey91qjtP3QkToIEbQQngM1
L8omo4dVoyqivbr5ngAGg1dSmwYC2EudyDg7fvERydIhjhCxLG6aN8Zn41AxmNzj
X0cZjM/o/38PI5QSYaC18J5cvz4er9ZtEiRGa0Jm5O22O7BlcOGDxy1FCENmsLvs
iVpLYg193j8gzFc1QrfBG3Fkpil5VVLcdIDeFyuXFOO4/nRLLefOCIsMVebmi7hx
6tFaMrmZ2jZV7nbVHFEJ6JKPpPg+4fWiG5bP0YkG/jGeGdVUAIr56z37ZKw7v2OK
iu4vA2YbKl8nO0VP4zbnk21bUU/xYTbGzwIDAQABo1MwUTAdBgNVHQ4EFgQU1tRl
0lD0zDHgIT4vJblGH6Q9hTswHwYDVR0jBBgwFoAU1tRl0lD0zDHgIT4vJblGH6Q9
hTswDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAQEAUxndGWNtdNPk
we7+UrZN8oZhE3bWdN6YB+R66dz6jDqjQxg7H5Nj/xoXrYlJ1Zxm67jpFCsZxOZc
xRGZVCp8vJEIPbMcbAxqbJTBTOjNIXdIwJ0ZQVPdT56eJPTPNgvdcI2y2cZ+IkZl
7iZ+PkQeoy0pI/P8aYShLdsJLeDxuFDFbSN7Y/a5Sm6nfwjlU6TABy5SdgfSbqKD
NbLeQy2E3Qy/SIsy/361VLbUNWyK5LyJLdIrDd2n+gsmzQ/cgV7b/fsDw22BmELB
RMVr21DnDN4l9BDDs8384GT2VOkW+6+Xl6co6gwNYSVRhsdOlDe8NkFtpe4BFg9H
/lNmxfnpPg==
-----END CERTIFICATE-----`

// ============ Helper Function Tests ============
// These tests don't require mocking the snapshot manager

func TestExtractCertificateMetadata_Success(t *testing.T) {
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

	subject, issuer, notBefore, notAfter, err := server.extractCertificateMetadata([]byte(validTestCert))

	assert.NoError(t, err)
	assert.NotEmpty(t, subject)
	assert.NotEmpty(t, issuer)
	assert.False(t, notBefore.IsZero())
	assert.False(t, notAfter.IsZero())
	assert.Contains(t, subject, "example.com")
}

func TestExtractCertificateMetadata_MultipleCerts(t *testing.T) {
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

	// Should extract from first cert in chain
	subject, issuer, notBefore, notAfter, err := server.extractCertificateMetadata([]byte(certChain))

	assert.NoError(t, err)
	assert.NotEmpty(t, subject)
	assert.NotEmpty(t, issuer)
	assert.False(t, notBefore.IsZero())
	assert.False(t, notAfter.IsZero())
}

func TestExtractCertificateMetadata_InvalidPEM(t *testing.T) {
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

	_, _, _, _, err := server.extractCertificateMetadata([]byte("not a PEM"))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no valid certificate found")
}

func TestExtractCertificateMetadata_NoCertificate(t *testing.T) {
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

	pemWithoutCert := `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA...
-----END RSA PRIVATE KEY-----`

	_, _, _, _, err := server.extractCertificateMetadata([]byte(pemWithoutCert))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no valid certificate found")
}

func TestValidateCertificate_SingleCert(t *testing.T) {
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

	count, err := server.validateCertificate([]byte(validTestCert))

	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestValidateCertificate_CertChain(t *testing.T) {
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

	count, err := server.validateCertificate([]byte(certChain))

	assert.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestValidateCertificate_InvalidPEM(t *testing.T) {
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

	_, err := server.validateCertificate([]byte("not valid PEM"))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no valid certificates found")
}

func TestValidateCertificate_NoCerts(t *testing.T) {
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

	pemWithoutCert := `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA...
-----END RSA PRIVATE KEY-----`

	_, err := server.validateCertificate([]byte(pemWithoutCert))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no valid certificates found")
}

// ============ ListCertificates Tests ============
// These tests don't need snapshot manager mocking

// newCertListHandler wraps ListCertificates with CorrelationIDMiddleware for testing.
// newCertListHandler builds ListCertificatesParams from the query string,
// since these tests bypass the generated router.
func newCertListHandler(server *APIServer) http.Handler {
	listCertificates := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var params management.ListCertificatesParams
		if usage := r.URL.Query().Get("usage"); usage != "" {
			usageParam := management.ListCertificatesParamsUsage(usage)
			params.Usage = &usageParam
		}
		server.ListCertificates(w, r, params)
	})
	return middleware.CorrelationIDMiddleware(server.logger)(listCertificates)
}

// newUploadCertHandler wraps UploadCertificate with CorrelationIDMiddleware for testing.
func newUploadCertHandler(server *APIServer) http.Handler {
	return middleware.CorrelationIDMiddleware(server.logger)(http.HandlerFunc(server.UploadCertificate))
}

// certificateTestConfig is the system config a bare test server needs for
// the certificate handlers: the default certificate upload body limit.
func certificateTestConfig() *config.Config {
	return &config.Config{Controller: config.Controller{Server: config.ServerConfig{MaxCertificateUploadBytes: 1 << 20}}}
}

// newDeleteCertHandler wraps DeleteCertificate with CorrelationIDMiddleware for testing.
func newDeleteCertHandler(server *APIServer, certID string) http.Handler {
	return middleware.CorrelationIDMiddleware(server.logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.DeleteCertificate(w, r, certID)
	}))
}

func TestListCertificates_Success(t *testing.T) {
	mockDB := NewMockStorage()

	// Pre-populate with certificates
	cert1 := &models.StoredCertificate{
		UUID:        "0000-cert-1-0000-000000000000",
		Name:        "test-cert-1",
		Certificate: []byte(validTestCert),
		Subject:     "CN=example.com",
		Issuer:      "CN=example.com",
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		CertCount:   1,
	}
	cert2 := &models.StoredCertificate{
		UUID:        "0000-cert-2-0000-000000000000",
		Name:        "test-cert-2",
		Certificate: []byte(validTestCert),
		Subject:     "CN=test.com",
		Issuer:      "CN=test.com",
		NotAfter:    time.Now().Add(180 * 24 * time.Hour),
		CertCount:   1,
	}
	mockDB.certs = []*models.StoredCertificate{cert1, cert2}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

	handler := newCertListHandler(server)
	req := httptest.NewRequest(http.MethodGet, "/certificates", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListCertificatesResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "success", resp.Status)
	assert.Equal(t, 2, resp.TotalCount)
	assert.Equal(t, len(validTestCert)*2, resp.TotalBytes)
	assert.Len(t, resp.Certificates, 2)
}

func TestListCertificates_UnknownUsageFilter_Rejected(t *testing.T) {
	for _, usage := range []string{"client", "backend"} {
		t.Run(usage, func(t *testing.T) {
			mockDB := NewMockStorage()
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

			handler := newCertListHandler(server)
			req := httptest.NewRequest(http.MethodGet, "/certificates?usage="+usage, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			var resp management.ErrorResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, "invalid usage filter", resp.Message)
			require.NotNil(t, resp.Errors)
			require.Len(t, *resp.Errors, 1)
			fieldErr := (*resp.Errors)[0]
			require.NotNil(t, fieldErr.Field)
			require.NotNil(t, fieldErr.Message)
			assert.Equal(t, "usage", *fieldErr.Field)
			assert.Equal(t, "usage must be upstream, downstream or identity", *fieldErr.Message)
		})
	}
}

func TestListCertificates_EmptyList(t *testing.T) {
	mockDB := NewMockStorage()
	mockDB.certs = []*models.StoredCertificate{}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

	handler := newCertListHandler(server)
	req := httptest.NewRequest(http.MethodGet, "/certificates", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListCertificatesResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "success", resp.Status)
	assert.Equal(t, 0, resp.TotalCount)
	assert.Equal(t, 0, resp.TotalBytes)
}

func TestListCertificates_DatabaseError(t *testing.T) {
	mockDB := NewMockStorage()
	mockDB.getErr = errors.New("database error")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

	handler := newCertListHandler(server)
	req := httptest.NewRequest(http.MethodGet, "/certificates", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "error", resp["status"])
	assert.Contains(t, resp["message"], "Failed to list certificates")
}

func TestListCertificates_CalculatesTotalBytes(t *testing.T) {
	mockDB := NewMockStorage()

	cert1 := &models.StoredCertificate{
		UUID:        "0000-cert-1-0000-000000000000",
		Name:        "test-cert-1",
		Certificate: []byte("small cert"),
		NotAfter:    time.Now(),
		CertCount:   1,
	}
	cert2 := &models.StoredCertificate{
		UUID:        "0000-cert-2-0000-000000000000",
		Name:        "test-cert-2",
		Certificate: []byte("another small cert"),
		NotAfter:    time.Now(),
		CertCount:   1,
	}
	mockDB.certs = []*models.StoredCertificate{cert1, cert2}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

	handler := newCertListHandler(server)
	req := httptest.NewRequest(http.MethodGet, "/certificates", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListCertificatesResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	expectedBytes := len("small cert") + len("another small cert")
	assert.Equal(t, expectedBytes, resp.TotalBytes)
}

// ============ Error Handling Tests ============
// These test various error conditions without needing full snapshot manager

func TestUploadCertificate_InvalidRequestBody(t *testing.T) {
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

	handler := newUploadCertHandler(server)

	req := httptest.NewRequest(http.MethodPost, "/certificates", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "error", resp["status"])
	assert.Contains(t, resp["message"], "Invalid request body")
}

func TestUploadCertificate_MissingRequiredFields(t *testing.T) {
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

	handler := newUploadCertHandler(server)

	tests := []struct {
		name    string
		reqBody UploadCertificateRequest
	}{
		{
			name:    "Missing certificate",
			reqBody: UploadCertificateRequest{Name: "test"},
		},
		{
			name:    "Missing name",
			reqBody: UploadCertificateRequest{Certificate: validTestCert},
		},
		{
			name:    "Both missing",
			reqBody: UploadCertificateRequest{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(tt.reqBody)
			req := httptest.NewRequest(http.MethodPost, "/certificates", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestUploadCertificate_InvalidPEMFormat(t *testing.T) {
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

	handler := newUploadCertHandler(server)

	reqBody := UploadCertificateRequest{
		Name:        "test-cert",
		Certificate: "not a valid PEM certificate",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/certificates", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "error", resp["status"])
	assert.Equal(t, certificateUploadInvalidMessage, resp["message"])
	entry := firstFieldError(t, w.Body.Bytes(), "certificate")
	assert.Equal(t, clientca.MsgNotPEMCertificate, entry["message"])
}

func TestUploadCertificate_DatabaseSaveError(t *testing.T) {
	mockDB := NewMockStorage()
	mockDB.saveErr = errors.New("database error")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

	handler := newUploadCertHandler(server)

	reqBody := UploadCertificateRequest{
		Name:        "test-cert",
		Certificate: validTestCert,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/certificates", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "error", resp["status"])
	assert.Contains(t, resp["message"], "Failed to save certificate")
}

func TestDeleteCertificate_EmptyID(t *testing.T) {
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

	handler := newDeleteCertHandler(server, "")

	req := httptest.NewRequest(http.MethodDelete, "/certificates", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "error", resp["status"])
	assert.Contains(t, resp["message"], "Certificate ID is required")
}

// ============================================================================
// Phase 3: Edge Cases and Boundary Tests for Certificates
// ============================================================================

// TestUploadCertificate_LargeCertificate tests handling of a very large certificate file
func TestUploadCertificate_LargeCertificate(t *testing.T) {
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

	// Create a large certificate by repeating the valid cert multiple times
	largeCert := ""
	for i := 0; i < 10; i++ {
		largeCert += validTestCert + "\n"
	}

	// Test validation of large cert (doesn't require snapshot manager)
	_, err := server.validateCertificate([]byte(largeCert))

	// A chain of 10 identical valid certs should parse successfully
	assert.NoError(t, err)
}

// TestUploadCertificate_EmptyPEMBlock tests certificate with empty PEM block
func TestUploadCertificate_EmptyPEMBlock(t *testing.T) {
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

	handler := newUploadCertHandler(server)

	reqBody := UploadCertificateRequest{
		Name:        "empty-cert",
		Certificate: "-----BEGIN CERTIFICATE-----\n-----END CERTIFICATE-----",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/certificates", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "error", resp["status"])
}

// TestUploadCertificate_MalformedPEMHeaders tests malformed PEM headers
func TestUploadCertificate_MalformedPEMHeaders(t *testing.T) {
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

	handler := newUploadCertHandler(server)

	tests := []struct {
		name string
		cert string
	}{
		{
			name: "Missing BEGIN header",
			cert: "MIIDkzCCAnugAwIBAgIUI92o...\n-----END CERTIFICATE-----",
		},
		{
			name: "Missing END header",
			cert: "-----BEGIN CERTIFICATE-----\nMIIDkzCCAnugAwIBAgIUI92o...",
		},
		{
			name: "Wrong header type",
			cert: "-----BEGIN RSA PRIVATE KEY-----\nMIIDkzCCAnugAwIBAgIUI92o...\n-----END RSA PRIVATE KEY-----",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := UploadCertificateRequest{
				Name:        "malformed-cert",
				Certificate: tt.cert,
			}
			bodyBytes, _ := json.Marshal(reqBody)

			req := httptest.NewRequest(http.MethodPost, "/certificates", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

// TestSaveCertificate_SpecialCharactersInName tests that certificate names
// with special characters are correctly stored in the database.
// Note: This tests direct database storage, not the full UploadCertificate handler flow.
func TestSaveCertificate_SpecialCharactersInName(t *testing.T) {
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

	// Test various special character names are accepted and stored correctly
	tests := []struct {
		name     string
		certName string
	}{
		{"Hyphen", "my-cert"},
		{"Underscore", "my_cert"},
		{"Dot", "my.cert"},
		{"Space", "my cert"},
		{"Unicode", "my-cert-日本語"},
		{"Special chars", "my@cert#123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a certificate with the special character name
			cert := &models.StoredCertificate{
				UUID:        uuid.New().String(),
				Name:        tt.certName, // ← ACTUALLY USES tt.certName NOW
				Certificate: []byte(validTestCert),
				Subject:     "CN=test.com",
				Issuer:      "CN=test.com",
				NotBefore:   time.Now(),
				NotAfter:    time.Now().Add(365 * 24 * time.Hour),
				CertCount:   1,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}

			// Save to database - this is what the handler does at line 119
			err := server.db.SaveCertificate(cert)
			assert.NoError(t, err)

			// Verify the certificate was saved with the correct name
			savedCerts, err := mockDB.ListCertificates()
			require.NoError(t, err)

			found := false
			for _, saved := range savedCerts {
				if saved.Name == tt.certName {
					found = true
					assert.Equal(t, tt.certName, saved.Name)
					break
				}
			}
			assert.True(t, found, "Certificate with name %q should be saved", tt.certName)

			// Clean up for next iteration
			mockDB.certs = []*models.StoredCertificate{}
		})
	}
}

// TestListCertificates_LargeResultSet tests listing many certificates
func TestListCertificates_LargeResultSet(t *testing.T) {
	mockDB := NewMockStorage()

	// Add 100 certificates
	for i := 0; i < 100; i++ {
		cert := &models.StoredCertificate{
			UUID:        fmt.Sprintf("0000-cert-%d-0000-000000000000", i),
			Name:        fmt.Sprintf("test-cert-%d", i),
			Certificate: []byte(validTestCert),
			Subject:     "CN=test.com",
			Issuer:      "CN=test.com",
			NotAfter:    time.Now().Add(365 * 24 * time.Hour),
			CertCount:   1,
		}
		mockDB.certs = append(mockDB.certs, cert)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

	handler := newCertListHandler(server)
	req := httptest.NewRequest(http.MethodGet, "/certificates", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListCertificatesResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 100, resp.TotalCount)
	assert.Len(t, resp.Certificates, 100)
}

// TestDeleteCertificate_SpecialCharactersInID tests deleting with special chars in ID
func TestDeleteCertificate_SpecialCharactersInID(t *testing.T) {
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

	// Test with various special character IDs - verify they can be looked up in DB
	specialIDs := []string{
		"cert-with-dashes",
		"cert_with_underscores",
		"cert.with.dots",
		"uuid-1234-5678-90ab-cdef",
	}

	for _, id := range specialIDs {
		// Try to get certificate with this ID (should return not found, not panic)
		_, err := mockDB.GetCertificate(id)

		// Should return an error (not found) but not panic
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	}

	// Verify empty ID handling
	t.Run("Empty ID", func(t *testing.T) {
		handler := newDeleteCertHandler(server, "")
		req := httptest.NewRequest(http.MethodDelete, "/certificates", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestValidateCertificate_BoundaryConditions tests certificate validation edge cases
func TestValidateCertificate_BoundaryConditions(t *testing.T) {
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

	tests := []struct {
		name        string
		certData    []byte
		expectError bool
	}{
		{
			name:        "Empty data",
			certData:    []byte{},
			expectError: true,
		},
		{
			name:        "Nil data",
			certData:    nil,
			expectError: true,
		},
		{
			name:        "Whitespace only",
			certData:    []byte("   \n\t  "),
			expectError: true,
		},
		{
			name:        "Single newline",
			certData:    []byte("\n"),
			expectError: true,
		},
		{
			name:        "Valid cert with extra whitespace",
			certData:    []byte("\n\n" + validTestCert + "\n\n"),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.validateCertificate(tt.certData)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestExtractCertificateMetadata_EdgeCases tests metadata extraction edge cases
func TestExtractCertificateMetadata_EdgeCases(t *testing.T) {
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

	tests := []struct {
		name        string
		certData    []byte
		expectError bool
	}{
		{
			name:        "Empty certificate data",
			certData:    []byte{},
			expectError: true,
		},
		{
			name:        "Certificate with extra padding",
			certData:    []byte("\n\n\n" + validTestCert + "\n\n\n"),
			expectError: false,
		},
		{
			name:        "Multiple certificates (chain)",
			certData:    []byte(certChain),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, _, err := server.extractCertificateMetadata(tt.certData)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestListCertificates_ConcurrentAccess tests concurrent listing (thread safety)
func TestListCertificates_ConcurrentAccess(t *testing.T) {
	mockDB := NewMockStorage()

	// Add some certificates
	for i := 0; i < 10; i++ {
		cert := &models.StoredCertificate{
			UUID:        fmt.Sprintf("0000-cert-%d-0000-000000000000", i),
			Name:        fmt.Sprintf("test-cert-%d", i),
			Certificate: []byte(validTestCert),
			NotAfter:    time.Now(),
			CertCount:   1,
		}
		mockDB.certs = append(mockDB.certs, cert)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger, systemConfig: certificateTestConfig(), clientAuthorities: testClientAuthorityPublisher(mockDB)}

	handler := newCertListHandler(server)

	// Launch concurrent requests
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/certificates", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}()
	}

	wg.Wait()
}

// TestUploadCertificate_JSONBoundaries tests JSON parsing edge cases
func TestUploadCertificate_JSONBoundaries(t *testing.T) {
	tests := []struct {
		name           string
		jsonBody       string
		expectParseErr bool
	}{
		{
			name:           "Empty JSON",
			jsonBody:       "{}",
			expectParseErr: false, // Parses OK, but would fail validation
		},
		{
			name:           "Null values",
			jsonBody:       `{"name":null,"certificate":null}`,
			expectParseErr: false, // Parses OK, but empty values
		},
		{
			name:           "Invalid JSON syntax",
			jsonBody:       `{"name":"test",}`,
			expectParseErr: true,
		},
		{
			name:           "Unclosed quote",
			jsonBody:       `{"name":"test}`,
			expectParseErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req UploadCertificateRequest
			err := json.Unmarshal([]byte(tt.jsonBody), &req)

			if tt.expectParseErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ============================================================================
// Client certificate authority pool (mTLS) tests
// ============================================================================

// createTestAPIServerWithCertStore builds a server with a real snapshot
// manager and certificate store, for flows that reload the store and update
// SDS.
func createTestAPIServerWithCertStore(t *testing.T, db storage.Storage) *APIServer {
	t.Helper()
	server := createTestAPIServerWithDB(db)
	server.routerConfig.Upstream.TLS.CustomCertsPath = t.TempDir()
	// The base listener loads this script; the default path only resolves
	// from the repo root.
	server.routerConfig.Lua.RequestTransformation.ScriptPath = "../../../lua/request_transformation.lua"
	snapshotManager, err := xds.NewSnapshotManager(server.store, server.logger, server.routerConfig, db, server.systemConfig)
	require.NoError(t, err)
	server.snapshotManager = snapshotManager
	return server
}

func seedUpstreamCert(t *testing.T) *models.StoredCertificate {
	t.Helper()
	seed := pki.NewRootCA(t, "Seed Upstream CA")
	return &models.StoredCertificate{
		UUID:        "seed-upstream-cert",
		Name:        "seed-upstream-cert",
		Certificate: seed.PEM(),
		Usage:       models.CertificateUsageUpstream,
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
	}
}

// uploadCertificateBody posts an upload for rejection tests, which fail
// validation before reaching the snapshot manager.
func uploadCertificateBody(t *testing.T, server *APIServer, reqBody UploadCertificateRequest) *httptest.ResponseRecorder {
	t.Helper()
	handler := newUploadCertHandler(server)
	bodyBytes, err := json.Marshal(reqBody)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/certificates", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func firstFieldError(t *testing.T, body []byte, field string) map[string]any {
	t.Helper()
	var resp map[string]any
	require.NoError(t, json.Unmarshal(body, &resp))
	errs, ok := resp["errors"].([]any)
	require.True(t, ok, "expected an errors array in response, got %v", resp)
	for _, raw := range errs {
		entry := raw.(map[string]any)
		if entry["field"] == field {
			return entry
		}
	}
	t.Fatalf("no validation error found for field %q in %v", field, errs)
	return nil
}

func TestUploadCertificate_BodyOverSizeLimit_Rejected(t *testing.T) {
	mockDB := NewMockStorage()
	server := createTestAPIServerWithDB(mockDB)
	server.systemConfig.Controller.Server.MaxCertificateUploadBytes = 4 << 10

	oversized := strings.Repeat("A", (4<<10)+1)
	w := uploadCertificateBody(t, server, UploadCertificateRequest{
		Name:        "pool-too-big",
		Usage:       models.CertificateUsageDownstream,
		Certificate: oversized,
	})

	require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)

	sizeStatedPattern := regexp.MustCompile(`[0-9]+ ?(bytes|KiB|MiB|kB|MB)`)
	assert.False(t, sizeStatedPattern.MatchString(w.Body.String()),
		"response body should not state the configured size limit: %s", w.Body.String())
}

// ============================================================================
// DELETE referential integrity for client-CA authorities
// ============================================================================

func clientAuthorityCert(name string) *models.StoredCertificate {
	return &models.StoredCertificate{
		UUID: name, Name: name, Usage: models.CertificateUsageDownstream, Role: models.CertificateRoleClient,
		NotAfter: time.Now().Add(365 * 24 * time.Hour),
	}
}

// restAPIConfigWithMtlsAuth builds a deployed RestApi attaching mtls-auth at
// API or operation level, accepting caNames, or inheriting the pool when
// caNames is empty.
func restAPIConfigWithMtlsAuth(handle string, apiLevel bool, caNames ...string) *models.StoredConfig {
	var params map[string]interface{}
	if len(caNames) > 0 {
		accept := make([]interface{}, 0, len(caNames))
		for _, name := range caNames {
			accept = append(accept, map[string]interface{}{"ca": name})
		}
		params = map[string]interface{}{"accept": accept}
	}
	policy := management.Policy{Name: "mtls-auth", Version: "v1"}
	if params != nil {
		policy.Params = &params
	}

	cfg := management.RestAPI{
		Kind:     management.RestAPIKindRestApi,
		Metadata: management.Metadata{Name: handle},
		Spec: management.APIConfigData{
			DisplayName: handle,
			Version:     "v1.0",
			Context:     "/" + handle,
			Operations: []management.Operation{
				{Method: management.Ptr(management.OperationMethodGET), Path: management.Ptr("/resource")},
			},
			Upstream: struct {
				Main    management.Upstream  `json:"main" yaml:"main"`
				Sandbox *management.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{
				Main: management.Upstream{Url: stringPtr("http://backend:8080")},
			},
		},
	}
	if apiLevel {
		cfg.Spec.Policies = &[]management.Policy{policy}
	} else {
		cfg.Spec.Operations[0].Policies = &[]management.Policy{policy}
	}

	return &models.StoredConfig{
		UUID:          handle,
		Kind:          models.KindRestApi,
		Handle:        handle,
		DisplayName:   handle,
		Version:       "v1.0",
		DesiredState:  models.StateDeployed,
		Configuration: cfg,
	}
}

func TestCheckClientAuthorityDeletable_NamedInAcceptList_OperationLevel_Blocks(t *testing.T) {
	mockDB := NewMockStorage()
	target := clientAuthorityCert("ref-partner-a")
	mockDB.certs = []*models.StoredCertificate{target}
	server := createTestAPIServerWithDB(mockDB)
	require.NoError(t, server.db.SaveConfig(restAPIConfigWithMtlsAuth("ref-naming-api", false, "ref-partner-a")))

	resp, _, blocked := server.checkClientAuthorityDeletable(target)

	require.True(t, blocked)
	require.NotNil(t, resp.Errors)
	errs := *resp.Errors
	require.Len(t, errs, 1)
	assert.Equal(t, "spec.operations[0].policies[0].params.accept[0].ca", *errs[0].Field)
}

func TestCheckClientAuthorityDeletable_NamedInAcceptList_TwoAPIs_PluralMessage(t *testing.T) {
	mockDB := NewMockStorage()
	target := clientAuthorityCert("ref-partner-a")
	mockDB.certs = []*models.StoredCertificate{target}
	server := createTestAPIServerWithDB(mockDB)
	require.NoError(t, server.db.SaveConfig(restAPIConfigWithMtlsAuth("ref-naming-api-1", true, "ref-partner-a")))
	require.NoError(t, server.db.SaveConfig(restAPIConfigWithMtlsAuth("ref-naming-api-2", true, "ref-partner-a")))

	resp, _, blocked := server.checkClientAuthorityDeletable(target)

	require.True(t, blocked)
	assert.Equal(t, "client-CA authority 'ref-partner-a' is named by 2 deployed APIs; remove those references first", resp.Message)
	require.NotNil(t, resp.Errors)
	assert.Len(t, *resp.Errors, 2)
}

func TestCheckClientAuthorityDeletable_LastNonRelayAuthority_Inheriting_Blocks(t *testing.T) {
	mockDB := NewMockStorage()
	target := clientAuthorityCert("ref-only-authority")
	mockDB.certs = []*models.StoredCertificate{target}
	server := createTestAPIServerWithDB(mockDB)
	require.NoError(t, server.db.SaveConfig(restAPIConfigWithMtlsAuth("ref-inheriting-api", true))) // no accept: inherits the pool

	resp, _, blocked := server.checkClientAuthorityDeletable(target)

	require.True(t, blocked)
	assert.Equal(t, "cannot remove the last client-CA authority while 1 deployed API attaches mtls-auth; add a replacement first or remove those APIs", resp.Message)
	require.NotNil(t, resp.Errors)
	errs := *resp.Errors
	require.Len(t, errs, 1)
	assert.Equal(t, "spec.policies[0]", *errs[0].Field)
}

func TestDeleteCertificate_NamedInAcceptList_Returns409(t *testing.T) {
	mockDB := NewMockStorage()
	target := clientAuthorityCert("ref-partner-a")
	mockDB.certs = []*models.StoredCertificate{target, seedUpstreamCert(t)}
	server := createTestAPIServerWithCertStore(t, mockDB)
	require.NoError(t, server.db.SaveConfig(restAPIConfigWithMtlsAuth("ref-naming-api", true, "ref-partner-a")))

	handler := newDeleteCertHandler(server, target.UUID)
	req := httptest.NewRequest(http.MethodDelete, "/certificates/"+target.UUID, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusConflict, w.Code, "body: %s", w.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "error", resp["status"])
	assert.Equal(t, "client-CA authority 'ref-partner-a' is named by 1 deployed API; remove those references first", resp["message"])

	// Still present afterwards: the delete never reached the database.
	_, err := mockDB.GetCertificate(target.UUID)
	assert.NoError(t, err)
}

// An unreferenced upstream certificate deletes even while a deployed API
// would block losing the last client authority.
func TestDeleteCertificate_UpstreamRow_UnaffectedByReferentialCheck(t *testing.T) {
	mockDB := NewMockStorage()
	upstream := &models.StoredCertificate{
		UUID: "ref-backend-trust", Name: "ref-backend-trust", Usage: models.CertificateUsageUpstream,
		NotAfter: time.Now().Add(365 * 24 * time.Hour), Certificate: []byte(validTestCert),
	}
	client := clientAuthorityCert("ref-only-authority")
	// A second upstream cert keeps the reload after the delete non-empty.
	mockDB.certs = []*models.StoredCertificate{upstream, client, seedUpstreamCert(t)}
	server := createTestAPIServerWithCertStore(t, mockDB)
	require.NoError(t, server.db.SaveConfig(restAPIConfigWithMtlsAuth("ref-inheriting-api", true)))

	handler := newDeleteCertHandler(server, upstream.UUID)
	req := httptest.NewRequest(http.MethodDelete, "/certificates/"+upstream.UUID, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
}

// ============================================================================
// Certificate upload validation: match narrowing
// ============================================================================

func uploadCertificateRawJSON(t *testing.T, server *APIServer, jsonBody string) *httptest.ResponseRecorder {
	t.Helper()
	handler := newUploadCertHandler(server)
	req := httptest.NewRequest(http.MethodPost, "/certificates", strings.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func TestUploadCertificate_MatchWithRoleClient_Rejected(t *testing.T) {
	mockDB := NewMockStorage()
	server := createTestAPIServerWithDB(mockDB)

	root := pki.NewRootCA(t, "Match Role Client CA")
	w := uploadCertificateBody(t, server, UploadCertificateRequest{
		Name:        "pool-match-role-client",
		Usage:       models.CertificateUsageDownstream,
		Role:        models.CertificateRoleClient,
		Certificate: string(root.PEM()),
		Match:       &models.CertificateMatch{DNSSANs: []string{"lb.corp.test"}},
	})

	require.Equal(t, http.StatusBadRequest, w.Code)
	entry := firstFieldError(t, w.Body.Bytes(), "match")
	assert.Equal(t, "match applies only to role: relay entries", entry["message"])
}

// client names a role, not a usage, so it is refused exactly like any other
// unknown usage value and nothing is stored.
func TestUploadCertificate_UnknownUsage_Rejected(t *testing.T) {
	for _, usage := range []string{"client", "backend"} {
		t.Run(usage, func(t *testing.T) {
			mockDB := NewMockStorage()
			server := createTestAPIServerWithDB(mockDB)

			root := pki.NewRootCA(t, "Unknown Usage CA")
			w := uploadCertificateBody(t, server, UploadCertificateRequest{
				Name:        "pool-unknown-usage",
				Usage:       usage,
				Certificate: string(root.PEM()),
			})

			require.Equal(t, http.StatusBadRequest, w.Code)
			entry := firstFieldError(t, w.Body.Bytes(), "usage")
			assert.Equal(t, "usage must be upstream, downstream or identity", entry["message"])
			assert.Empty(t, mockDB.certs, "a refused upload must not be stored")
		})
	}
}

// Raw JSON, because omitempty would drop an empty dnsSANs slice and test the
// omitted case instead.
func TestUploadCertificate_EmptyDNSSANsList_Rejected(t *testing.T) {
	mockDB := NewMockStorage()
	server := createTestAPIServerWithDB(mockDB)

	root := pki.NewRootCA(t, "Empty DNS SANs CA")
	certJSON, err := json.Marshal(string(root.PEM()))
	require.NoError(t, err)

	body := fmt.Sprintf(`{"name":"pool-empty-dns-sans","usage":"downstream","role":"relay","certificate":%s,"match":{"dnsSANs":[]}}`, certJSON)
	w := uploadCertificateRawJSON(t, server, body)

	require.Equal(t, http.StatusBadRequest, w.Code)
	entry := firstFieldError(t, w.Body.Bytes(), "match.dnsSANs")
	assert.Equal(t, "list at least one non-empty SAN", entry["message"])
}

func TestUploadCertificate_DNSSANsElementEmpty_RejectedAtIndexZero(t *testing.T) {
	mockDB := NewMockStorage()
	server := createTestAPIServerWithDB(mockDB)

	root := pki.NewRootCA(t, "DNS SANs Element Empty CA")
	certJSON, err := json.Marshal(string(root.PEM()))
	require.NoError(t, err)

	body := fmt.Sprintf(`{"name":"pool-dns-sans-empty-element","usage":"downstream","role":"relay","certificate":%s,"match":{"dnsSANs":[""]}}`, certJSON)
	w := uploadCertificateRawJSON(t, server, body)

	require.Equal(t, http.StatusBadRequest, w.Code)
	entry := firstFieldError(t, w.Body.Bytes(), "match.dnsSANs[0]")
	assert.Equal(t, "list at least one non-empty SAN", entry["message"])
}

func TestUploadCertificate_PrivateKeyInCertificateField_RejectedForEveryUsage(t *testing.T) {
	const certificateFieldCarriesKey = "the certificate field takes certificates only; the private key belongs in privateKey"
	for _, tc := range []struct {
		usage      string
		privateKey bool
		message    string
	}{
		{usage: models.CertificateUsageDownstream, message: "the upload contains a private key; a client-CA entry accepts certificates only"},
		{usage: models.CertificateUsageUpstream, message: certificateFieldCarriesKey},
		{usage: models.CertificateUsageIdentity, privateKey: true, message: certificateFieldCarriesKey},
	} {
		t.Run(tc.usage, func(t *testing.T) {
			mockDB := NewMockStorage()
			mockDB.certs = []*models.StoredCertificate{seedUpstreamCert(t)}
			server := createTestAPIServerWithIdentitySupport(t, mockDB)

			leaf := gatewayIdentityLeaf(t, "key-in-certificate-"+tc.usage)
			req := UploadCertificateRequest{
				Name:        "key-in-certificate-" + tc.usage,
				Usage:       tc.usage,
				Certificate: string(leaf.PEM()) + string(leaf.KeyPEM()),
			}
			if tc.privateKey {
				req.PrivateKey = string(leaf.KeyPEM())
			}
			w := uploadCertificateBody(t, server, req)

			require.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
			entry := firstFieldError(t, w.Body.Bytes(), "certificate")
			assert.Equal(t, tc.message, entry["message"])
			assert.Empty(t, certificateEvents(t, server))
		})
	}
}

// A relay entry's match appears in the upload response and the listing.
func TestUploadCertificate_RelayWithMatch_EchoedInResponseAndList(t *testing.T) {
	mockDB := NewMockStorage()
	mockDB.certs = []*models.StoredCertificate{seedUpstreamCert(t)}
	server := createTestAPIServerWithCertStore(t, mockDB)

	root := pki.NewRootCA(t, "Relay With Match CA")
	w := uploadCertificateBody(t, server, UploadCertificateRequest{
		Name:        "pool-relay-with-match",
		Usage:       models.CertificateUsageDownstream,
		Role:        models.CertificateRoleRelay,
		Certificate: string(root.PEM()),
		Match:       &models.CertificateMatch{DNSSANs: []string{"lb.corp.test"}},
	})
	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	matchRaw, ok := resp["match"].(map[string]any)
	require.True(t, ok, "expected match in the upload response, got %v", resp["match"])
	dnsSANs, ok := matchRaw["dnsSANs"].([]any)
	require.True(t, ok)
	require.Len(t, dnsSANs, 1)
	assert.Equal(t, "lb.corp.test", dnsSANs[0])

	listHandler := newCertListHandler(server)
	listReq := httptest.NewRequest(http.MethodGet, "/certificates?usage=downstream", nil)
	listW := httptest.NewRecorder()
	listHandler.ServeHTTP(listW, listReq)
	require.Equal(t, http.StatusOK, listW.Code)

	var listResp map[string]any
	require.NoError(t, json.Unmarshal(listW.Body.Bytes(), &listResp))
	items, ok := listResp["certificates"].([]any)
	require.True(t, ok)

	var found map[string]any
	for _, raw := range items {
		item := raw.(map[string]any)
		if item["name"] == "pool-relay-with-match" {
			found = item
		}
	}
	require.NotNil(t, found, "uploaded relay cert not found in listing")
	listMatch, ok := found["match"].(map[string]any)
	require.True(t, ok, "expected match on the listed relay item, got %v", found["match"])
	listDNSSANs, ok := listMatch["dnsSANs"].([]any)
	require.True(t, ok)
	require.Len(t, listDNSSANs, 1)
	assert.Equal(t, "lb.corp.test", listDNSSANs[0])
}

// ============================================================================
// Gateway identities: usage: identity certificates
// ============================================================================

// testEncryptionManager builds an AES-GCM provider backed by a temporary key.
func testEncryptionManager(t *testing.T) *encryption.ProviderManager {
	t.Helper()
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "v1.key")
	key := make([]byte, aesgcm.AESKeySize)
	_, err := rand.Read(key)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyPath, key, 0600))

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	provider, err := aesgcm.NewAESGCMProvider([]aesgcm.KeyConfig{{Version: "v1", FilePath: keyPath}}, logger)
	require.NoError(t, err)

	mgr, err := encryption.NewProviderManager([]encryption.EncryptionProvider{provider}, logger)
	require.NoError(t, err)
	return mgr
}

// createTestAPIServerWithIdentitySupport wires the certificate store and one
// encryption manager for both encrypting and decrypting identity keys.
func createTestAPIServerWithIdentitySupport(t *testing.T, db storage.Storage) *APIServer {
	t.Helper()
	server := createTestAPIServerWithCertStore(t, db)
	mgr := testEncryptionManager(t)
	server.encryptionManager = mgr
	if translator := server.snapshotManager.GetTranslator(); translator != nil && translator.GetCertStore() != nil {
		translator.GetCertStore().SetEncryptionManager(mgr)
	}
	return server
}

// gatewayIdentityLeaf builds a client-auth leaf under its own root CA.
func gatewayIdentityLeaf(t *testing.T, cn string) *pki.Entity {
	t.Helper()
	ca := pki.NewRootCA(t, cn+" Root CA")
	return pki.NewLeaf(t, ca, cn)
}

// newUpdateCertHandler wraps UpdateCertificate with CorrelationIDMiddleware for testing.
func newUpdateCertHandler(server *APIServer, certID string) http.Handler {
	return middleware.CorrelationIDMiddleware(server.logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.UpdateCertificate(w, r, certID)
	}))
}

func updateCertificateBody(t *testing.T, server *APIServer, certID string, reqBody UploadCertificateRequest) *httptest.ResponseRecorder {
	t.Helper()
	handler := newUpdateCertHandler(server, certID)
	bodyBytes, err := json.Marshal(reqBody)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPut, "/certificates/"+certID, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

// restAPIConfigWithUpstreamTLS builds a deployed RestApi whose "partner"
// definition carries a tls block with the given identity and trustedCAs.
func restAPIConfigWithUpstreamTLS(handle, identity string, trustedCAs ...string) *models.StoredConfig {
	tls := map[string]interface{}{}
	if identity != "" {
		tls["identity"] = identity
	}
	if len(trustedCAs) > 0 {
		caList := make([]interface{}, len(trustedCAs))
		for i, c := range trustedCAs {
			caList[i] = c
		}
		tls["trustedCAs"] = caList
	}

	def := management.UpstreamDefinition{
		Name: "partner",
		Tls:  &tls,
		Upstreams: []struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{{Url: "https://backend:8443"}},
	}

	cfg := management.RestAPI{
		Kind:     management.RestAPIKindRestApi,
		Metadata: management.Metadata{Name: handle},
		Spec: management.APIConfigData{
			DisplayName:         handle,
			Version:             "v1.0",
			Context:             "/" + handle,
			UpstreamDefinitions: &[]management.UpstreamDefinition{def},
			Operations: []management.Operation{
				{Method: management.Ptr(management.OperationMethodGET), Path: management.Ptr("/resource")},
			},
			Upstream: struct {
				Main    management.Upstream  `json:"main" yaml:"main"`
				Sandbox *management.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{
				Main: management.Upstream{Ref: stringPtr("partner")},
			},
		},
	}

	return &models.StoredConfig{
		UUID:          handle,
		Kind:          models.KindRestApi,
		Handle:        handle,
		DisplayName:   handle,
		Version:       "v1.0",
		DesiredState:  models.StateDeployed,
		Configuration: cfg,
	}
}

func TestListCertificates_UsageIdentity_NoPrivateKeyAndReferencedByApis(t *testing.T) {
	mockDB := NewMockStorage()
	identity := gatewayIdentityLeaf(t, "list-identity")
	mockDB.certs = []*models.StoredCertificate{
		{
			UUID: "list-identity-1", Name: "out-identity-a", Certificate: identity.PEM(),
			Usage: models.CertificateUsageIdentity, KeyAlgorithm: "ECDSA",
			PrivateKeyCiphertext: "aesgcm:v1:deadbeef", NotAfter: time.Now().Add(365 * 24 * time.Hour),
		},
	}
	server := createTestAPIServerWithDB(mockDB)

	handler := newCertListHandler(server)
	req := httptest.NewRequest(http.MethodGet, "/certificates?usage=identity", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "PrivateKeyCiphertext")
	assert.NotContains(t, w.Body.String(), "deadbeef")

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	items, ok := resp["certificates"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	item := items[0].(map[string]any)

	_, hasPrivateKey := item["privateKey"]
	assert.False(t, hasPrivateKey)
	referenced, ok := item["referencedByApis"].(float64)
	require.True(t, ok, "expected identity row to carry a numeric referencedByApis, got %v", item["referencedByApis"])
	assert.Equal(t, float64(0), referenced)
	assert.Equal(t, "ECDSA", item["keyAlgorithm"])
}

func identityRowForUpdate(t *testing.T, id string) *models.StoredCertificate {
	t.Helper()
	original := gatewayIdentityLeaf(t, "rotate-original")
	return &models.StoredCertificate{
		UUID: id, Name: "out-identity-a", Certificate: original.PEM(),
		Usage: models.CertificateUsageIdentity, KeyAlgorithm: "ECDSA",
		NotAfter: time.Now().Add(365 * 24 * time.Hour),
	}
}

func TestUpdateCertificate_UnknownField_Rejected(t *testing.T) {
	mockDB := NewMockStorage()
	mockDB.certs = []*models.StoredCertificate{seedUpstreamCert(t), identityRowForUpdate(t, "rotate-identity-1")}
	server := createTestAPIServerWithIdentitySupport(t, mockDB)

	rotated := gatewayIdentityLeaf(t, "rotate-new")
	body, err := json.Marshal(map[string]any{
		"certificate": string(rotated.PEM()),
		"privateKey":  string(rotated.KeyPEM()),
		"passphrase":  "secret",
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPut, "/certificates/rotate-identity-1", bytes.NewReader(body))
	w := httptest.NewRecorder()
	newUpdateCertHandler(server, "rotate-identity-1").ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
	entry := firstFieldError(t, w.Body.Bytes(), "passphrase")
	assert.Equal(t, "unknown field passphrase", entry["message"])

	stored, err := mockDB.GetCertificate("rotate-identity-1")
	require.NoError(t, err)
	assert.NotEqual(t, string(rotated.PEM()), string(stored.Certificate), "a refused update must not reach the database")
}

func TestUpdateCertificate_MissingRow_Returns404(t *testing.T) {
	mockDB := NewMockStorage()
	server := createTestAPIServerWithIdentitySupport(t, mockDB)

	rotated := gatewayIdentityLeaf(t, "rotate-new")
	w := updateCertificateBody(t, server, "no-such-id", UploadCertificateRequest{
		Certificate: string(rotated.PEM()),
		PrivateKey:  string(rotated.KeyPEM()),
	})

	require.Equal(t, http.StatusNotFound, w.Code, "body: %s", w.Body.String())
}

func TestUpdateCertificate_StoreReadFailure_Returns500(t *testing.T) {
	mockDB := NewMockStorage()
	mockDB.certs = []*models.StoredCertificate{identityRowForUpdate(t, "rotate-identity-1")}
	server := createTestAPIServerWithIdentitySupport(t, mockDB)
	mockDB.getErr = errors.New("connection reset")

	rotated := gatewayIdentityLeaf(t, "rotate-new")
	w := updateCertificateBody(t, server, "rotate-identity-1", UploadCertificateRequest{
		Certificate: string(rotated.PEM()),
		PrivateKey:  string(rotated.KeyPEM()),
	})

	require.Equal(t, http.StatusInternalServerError, w.Code, "body: %s", w.Body.String())
	assert.NotContains(t, w.Body.String(), "connection reset")
}

func TestUpdateCertificate_ExpiringIdentity_WarnsExpiresSoon(t *testing.T) {
	mockDB := NewMockStorage()
	mockDB.certs = []*models.StoredCertificate{seedUpstreamCert(t), identityRowForUpdate(t, "rotate-identity-1")}
	server := createTestAPIServerWithIdentitySupport(t, mockDB)

	ca := pki.NewRootCA(t, "rotate-soon Root CA")
	rotated := pki.NewLeaf(t, ca, "rotate-soon", pki.WithValidity(time.Now().Add(-time.Hour), time.Now().Add(5*24*time.Hour)))
	w := updateCertificateBody(t, server, "rotate-identity-1", UploadCertificateRequest{
		Certificate: string(rotated.PEM()),
		PrivateKey:  string(rotated.KeyPEM()),
	})

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	warnings, ok := resp["warnings"].([]any)
	require.True(t, ok, "expected a warnings array, got %v", resp["warnings"])
	var codes []any
	for _, raw := range warnings {
		codes = append(codes, raw.(map[string]any)["code"])
	}
	assert.Contains(t, codes, "CERT_EXPIRES_SOON")
}

func TestDeleteCertificate_IdentityNamedInUpstreamTLS_Returns409(t *testing.T) {
	identity := gatewayIdentityLeaf(t, "del-identity")
	target := &models.StoredCertificate{
		UUID: "del-identity-1", Name: "out-identity-a", Certificate: identity.PEM(),
		Usage: models.CertificateUsageIdentity, KeyAlgorithm: "ECDSA", NotAfter: time.Now().Add(365 * 24 * time.Hour),
	}
	mockDB := NewMockStorage()
	mockDB.certs = []*models.StoredCertificate{target, seedUpstreamCert(t)}
	server := createTestAPIServerWithCertStore(t, mockDB)
	require.NoError(t, server.db.SaveConfig(restAPIConfigWithUpstreamTLS("out-partner-api", "out-identity-a")))

	handler := newDeleteCertHandler(server, target.UUID)
	req := httptest.NewRequest(http.MethodDelete, "/certificates/"+target.UUID, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusConflict, w.Code, "body: %s", w.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "gateway identity 'out-identity-a' is named by 1 deployed API; remove those references first", resp["message"])
	errs, ok := resp["errors"].([]any)
	require.True(t, ok)
	require.Len(t, errs, 1)
	entry := errs[0].(map[string]any)
	assert.Equal(t, "spec.upstreamDefinitions[0].tls.identity", entry["field"])
	assert.Equal(t, "referenced by API 'out-partner-api'", entry["message"])

	// Still present afterwards: the delete never reached the database.
	_, err := mockDB.GetCertificate(target.UUID)
	assert.NoError(t, err)
}

// agentConfigWithUpstreamTLS builds a deployed Agent whose single upstream
// definition names identity and trustedCAs in its tls block.
func agentConfigWithUpstreamTLS(handle, identity string, trustedCAs ...string) *models.StoredConfig {
	rest := restAPIConfigWithUpstreamTLS(handle, identity, trustedCAs...).Configuration.(management.RestAPI)
	agent := management.AgentConfiguration{
		Kind:     management.AgentConfigurationKindAgent,
		Metadata: management.Metadata{Name: handle},
		Spec: management.AgentConfigData{
			DisplayName:         handle,
			Version:             "v1.0",
			Context:             stringPtr("/" + handle),
			UpstreamDefinitions: rest.Spec.UpstreamDefinitions,
			Upstream:            management.AgentConfigData_Upstream{Ref: stringPtr("partner")},
		},
	}
	return &models.StoredConfig{
		UUID:          handle,
		Kind:          models.KindAgent,
		Handle:        handle,
		DisplayName:   handle,
		Version:       "v1.0",
		DesiredState:  models.StateDeployed,
		Configuration: agent,
	}
}

func TestDeleteCertificate_NamedByDeployedAgent_Returns409(t *testing.T) {
	identity := gatewayIdentityLeaf(t, "agent-identity")
	identityRow := &models.StoredCertificate{
		UUID: "agent-identity-1", Name: "out-identity-a", Certificate: identity.PEM(),
		Usage: models.CertificateUsageIdentity, KeyAlgorithm: "ECDSA", NotAfter: time.Now().Add(365 * 24 * time.Hour),
	}
	trustRow := &models.StoredCertificate{
		UUID: "agent-backend-ca-1", Name: "out-backend-ca", Usage: models.CertificateUsageUpstream,
		Certificate: pki.NewRootCA(t, "Agent Backend CA").PEM(), NotAfter: time.Now().Add(365 * 24 * time.Hour),
	}
	mockDB := NewMockStorage()
	mockDB.certs = []*models.StoredCertificate{identityRow, trustRow, seedUpstreamCert(t)}
	server := createTestAPIServerWithCertStore(t, mockDB)
	require.NoError(t, server.db.SaveConfig(agentConfigWithUpstreamTLS("out-partner-agent", "out-identity-a", "out-backend-ca")))

	for _, tc := range []struct {
		row     *models.StoredCertificate
		field   string
		message string
	}{
		{identityRow, "spec.upstreamDefinitions[0].tls.identity", "gateway identity 'out-identity-a' is named by 1 deployed API; remove those references first"},
		{trustRow, "spec.upstreamDefinitions[0].tls.trustedCAs[0]", "certificate 'out-backend-ca' is named by 1 deployed API; remove those references first"},
	} {
		t.Run(tc.row.Name, func(t *testing.T) {
			w := httptest.NewRecorder()
			newDeleteCertHandler(server, tc.row.UUID).ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/certificates/"+tc.row.UUID, nil))
			require.Equal(t, http.StatusConflict, w.Code, "body: %s", w.Body.String())

			var resp map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, tc.message, resp["message"])
			errs, ok := resp["errors"].([]any)
			require.True(t, ok)
			require.Len(t, errs, 1)
			entry := errs[0].(map[string]any)
			assert.Equal(t, tc.field, entry["field"])
			assert.Equal(t, "referenced by API 'out-partner-agent'", entry["message"])

			_, err := mockDB.GetCertificate(tc.row.UUID)
			assert.NoError(t, err)
		})
	}
}

func TestDeleteCertificate_IdentityRow_AllowedWhenUnreferenced_DespiteMtlsAuthAPIDeployed(t *testing.T) {
	clientAuth := clientAuthorityCert("ref-only-authority")
	identity := gatewayIdentityLeaf(t, "unreferenced-identity")
	target := &models.StoredCertificate{
		UUID: "unreferenced-identity-1", Name: "out-identity-unreferenced", Certificate: identity.PEM(),
		Usage: models.CertificateUsageIdentity, KeyAlgorithm: "ECDSA", NotAfter: time.Now().Add(365 * 24 * time.Hour),
	}
	mockDB := NewMockStorage()
	mockDB.certs = []*models.StoredCertificate{clientAuth, target, seedUpstreamCert(t)}
	server := createTestAPIServerWithCertStore(t, mockDB)
	require.NoError(t, server.db.SaveConfig(restAPIConfigWithMtlsAuth("ref-inheriting-api", true)))

	handler := newDeleteCertHandler(server, target.UUID)
	req := httptest.NewRequest(http.MethodDelete, "/certificates/"+target.UUID, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
}

func TestDeleteCertificate_StoreReadFailure_Refuses(t *testing.T) {
	mockDB := NewMockStorage()
	authority := clientAuthorityCert("ref-partner-a")
	mockDB.certs = []*models.StoredCertificate{authority, seedUpstreamCert(t)}
	server := createTestAPIServerWithCertStore(t, mockDB)
	mockDB.getErr = errors.New("database unavailable")

	handler := newDeleteCertHandler(server, authority.UUID)
	req := httptest.NewRequest(http.MethodDelete, "/certificates/"+authority.UUID, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code, "body: %s", w.Body.String())
	assert.NotContains(t, w.Body.String(), "database unavailable")
	assert.Len(t, mockDB.certs, 2, "a delete must not proceed when the row could not be read")
}

// withClientAuthorityPublisher wires a real publisher over server's database
// into server and returns the lazy-resource manager it publishes through.
func withClientAuthorityPublisher(server *APIServer) *lazyresourcexds.LazyResourceStateManager {
	manager := newTestLazyResourceManager()
	server.clientAuthorities = utils.NewClientAuthorityPublisher(server.db, manager)
	return manager
}

func publishedClientAuthorities(manager *lazyresourcexds.LazyResourceStateManager) []string {
	var names []string
	for name := range manager.GetResourcesByType(utils.LazyResourceTypeClientCertificateAuthority) {
		names = append(names, name)
	}
	return names
}

func postCertificate(t *testing.T, server *APIServer, name, usage string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(UploadCertificateRequest{
		Name:        name,
		Usage:       usage,
		Certificate: string(pki.NewRootCA(t, name).PEM()),
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/certificates", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	newUploadCertHandler(server).ServeHTTP(w, req)
	return w
}

func TestReloadCertificates_PublishesClientAuthorities(t *testing.T) {
	mockDB := NewMockStorage()
	mockDB.certs = []*models.StoredCertificate{seedUpstreamCert(t)}
	server := createTestAPIServerWithCertStore(t, mockDB)
	manager := withClientAuthorityPublisher(server)

	// A row that reached the database without passing through this server's
	// upload handler becomes visible to the policy engine on reload.
	mockDB.certs = append(mockDB.certs, &models.StoredCertificate{
		UUID: "late-id", Name: "late", Certificate: pki.NewRootCA(t, "Late Root").PEM(),
		Usage: models.CertificateUsageDownstream, NotAfter: time.Now().Add(time.Hour),
	})

	handler := middleware.CorrelationIDMiddleware(server.logger)(http.HandlerFunc(server.ReloadCertificates))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/certificates/reload", nil))
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	assert.ElementsMatch(t, []string{"late"}, publishedClientAuthorities(manager))
}

func TestUploadCertificate_UsageDownstream_PublishFailureIsReported(t *testing.T) {
	mockDB := NewMockStorage()
	mockDB.certs = []*models.StoredCertificate{seedUpstreamCert(t)}
	server := createTestAPIServerWithCertStore(t, mockDB)
	server.clientAuthorities = utils.NewClientAuthorityPublisher(&failingCertificateList{MockStorage: mockDB}, nil)

	w := postCertificate(t, server, "partner-root", models.CertificateUsageDownstream)
	assert.Equal(t, http.StatusInternalServerError, w.Code, "body: %s", w.Body.String())
}

// failingCertificateList fails the pool listing the publisher performs, while
// every other storage call behaves as the embedded mock.
type failingCertificateList struct {
	*MockStorage
}

func (f *failingCertificateList) ListCertificatesByUsage(string) ([]*models.StoredCertificate, error) {
	return nil, storage.ErrDatabaseUnavailable
}

// ============ Replica propagation ============

// certificateEvents returns the certificate events server published.
func certificateEvents(t *testing.T, server *APIServer) []eventhub.Event {
	t.Helper()
	hub, ok := server.eventHub.(*mockEventHub)
	require.True(t, ok, "expected the test server to carry a mockEventHub, got %T", server.eventHub)
	var out []eventhub.Event
	for _, published := range hub.publishedEvents {
		if published.event.EventType == eventhub.EventTypeCertificate {
			out = append(out, published.event)
		}
	}
	return out
}

func requireOneCertificateEvent(t *testing.T, server *APIServer, action, entityID string) {
	t.Helper()
	events := certificateEvents(t, server)
	require.Len(t, events, 1, "expected exactly one certificate event, got %v", events)
	assert.Equal(t, action, events[0].Action)
	assert.Equal(t, entityID, events[0].EntityID)
	assert.Equal(t, "test-gateway", events[0].GatewayID)
	assert.Equal(t, eventhub.EmptyEventData, events[0].EventData, "the event must carry no certificate material")
	assert.NotEmpty(t, events[0].EventID, "the event carries the request's correlation id")
}

func TestUploadCertificate_PublishesCertificateCreateEvent(t *testing.T) {
	mockDB := NewMockStorage()
	mockDB.certs = []*models.StoredCertificate{seedUpstreamCert(t)}
	server := createTestAPIServerWithCertStore(t, mockDB)

	body, err := json.Marshal(UploadCertificateRequest{
		Name: "pool-partner", Usage: models.CertificateUsageDownstream,
		Certificate: string(pki.NewRootCA(t, "Replica Partner CA").PEM()),
	})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	newUploadCertHandler(server).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/certificates", bytes.NewReader(body)))
	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	requireOneCertificateEvent(t, server, "CREATE", resp["id"].(string))
}

func TestUploadCertificate_RefusedUploadPublishesNothing(t *testing.T) {
	mockDB := NewMockStorage()
	server := createTestAPIServerWithCertStore(t, mockDB)

	body, err := json.Marshal(UploadCertificateRequest{Name: "bad name", Certificate: "not a certificate"})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	newUploadCertHandler(server).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/certificates", bytes.NewReader(body)))
	require.Equal(t, http.StatusBadRequest, w.Code)

	assert.Empty(t, certificateEvents(t, server))
}

func TestUpdateCertificate_PublishesCertificateUpdateEvent(t *testing.T) {
	mockDB := NewMockStorage()
	mockDB.certs = []*models.StoredCertificate{seedUpstreamCert(t), identityRowForUpdate(t, "rotate-identity-1")}
	server := createTestAPIServerWithIdentitySupport(t, mockDB)

	rotated := gatewayIdentityLeaf(t, "rotate-new")
	w := updateCertificateBody(t, server, "rotate-identity-1", UploadCertificateRequest{
		Certificate: string(rotated.PEM()),
		PrivateKey:  string(rotated.KeyPEM()),
	})
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	requireOneCertificateEvent(t, server, "UPDATE", "rotate-identity-1")
}

func TestDeleteCertificate_PublishesCertificateDeleteEvent(t *testing.T) {
	target := &models.StoredCertificate{
		UUID: "del-unreferenced-1", Name: "unreferenced-backend-ca", Usage: models.CertificateUsageUpstream,
		Certificate: pki.NewRootCA(t, "Unreferenced CA").PEM(), NotAfter: time.Now().Add(365 * 24 * time.Hour),
	}
	mockDB := NewMockStorage()
	mockDB.certs = []*models.StoredCertificate{target, seedUpstreamCert(t)}
	server := createTestAPIServerWithCertStore(t, mockDB)

	w := httptest.NewRecorder()
	newDeleteCertHandler(server, target.UUID).ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/certificates/"+target.UUID, nil))
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	requireOneCertificateEvent(t, server, "DELETE", target.UUID)
}

func TestReloadCertificates_PublishesCertificateUpdateEvent(t *testing.T) {
	mockDB := NewMockStorage()
	mockDB.certs = []*models.StoredCertificate{seedUpstreamCert(t)}
	server := createTestAPIServerWithCertStore(t, mockDB)

	w := httptest.NewRecorder()
	middleware.CorrelationIDMiddleware(server.logger)(http.HandlerFunc(server.ReloadCertificates)).
		ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/certificates/reload", nil))
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	requireOneCertificateEvent(t, server, "UPDATE", "")
}
