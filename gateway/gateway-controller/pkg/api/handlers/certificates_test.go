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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
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

// ============ Helper Function Tests ============
// These tests don't require mocking the snapshot manager

func TestExtractCertificateMetadata_Success(t *testing.T) {
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger}

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
	server := &APIServer{db: mockDB, logger: logger}

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
	server := &APIServer{db: mockDB, logger: logger}

	_, _, _, _, err := server.extractCertificateMetadata([]byte("not a PEM"))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no valid certificate found")
}

func TestExtractCertificateMetadata_NoCertificate(t *testing.T) {
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger}

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
	server := &APIServer{db: mockDB, logger: logger}

	count, err := server.validateCertificate([]byte(validTestCert))

	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestValidateCertificate_CertChain(t *testing.T) {
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger}

	count, err := server.validateCertificate([]byte(certChain))

	assert.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestValidateCertificate_InvalidPEM(t *testing.T) {
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger}

	_, err := server.validateCertificate([]byte("not valid PEM"))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no valid certificates found")
}

func TestValidateCertificate_NoCerts(t *testing.T) {
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger}

	pemWithoutCert := `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA...
-----END RSA PRIVATE KEY-----`

	_, err := server.validateCertificate([]byte(pemWithoutCert))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no valid certificates found")
}

// ============ ListCertificates Tests ============
// These tests don't need snapshot manager mocking

func TestListCertificates_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
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
	server := &APIServer{db: mockDB, logger: logger}

	router := gin.New()
	router.Use(middleware.CorrelationIDMiddleware(server.logger))
	router.GET("/certificates", server.ListCertificates)

	req := httptest.NewRequest(http.MethodGet, "/certificates", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListCertificatesResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "success", resp.Status)
	assert.Equal(t, 2, resp.TotalCount)
	assert.Equal(t, len(validTestCert)*2, resp.TotalBytes)
	assert.Len(t, resp.Certificates, 2)
}

func TestListCertificates_EmptyList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockDB := NewMockStorage()
	mockDB.certs = []*models.StoredCertificate{}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger}

	router := gin.New()
	router.Use(middleware.CorrelationIDMiddleware(server.logger))
	router.GET("/certificates", server.ListCertificates)

	req := httptest.NewRequest(http.MethodGet, "/certificates", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListCertificatesResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "success", resp.Status)
	assert.Equal(t, 0, resp.TotalCount)
	assert.Equal(t, 0, resp.TotalBytes)
}

func TestListCertificates_DatabaseError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockDB := NewMockStorage()
	mockDB.getErr = errors.New("database error")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger}

	router := gin.New()
	router.Use(middleware.CorrelationIDMiddleware(server.logger))
	router.GET("/certificates", server.ListCertificates)

	req := httptest.NewRequest(http.MethodGet, "/certificates", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "error", resp["status"])
	assert.Contains(t, resp["message"], "Failed to list certificates")
}

func TestListCertificates_CalculatesTotalBytes(t *testing.T) {
	gin.SetMode(gin.TestMode)
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
	server := &APIServer{db: mockDB, logger: logger}

	router := gin.New()
	router.Use(middleware.CorrelationIDMiddleware(server.logger))
	router.GET("/certificates", server.ListCertificates)

	req := httptest.NewRequest(http.MethodGet, "/certificates", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

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
	gin.SetMode(gin.TestMode)
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger}

	router := gin.New()
	router.Use(middleware.CorrelationIDMiddleware(server.logger))
	router.POST("/certificates", server.UploadCertificate)

	req := httptest.NewRequest(http.MethodPost, "/certificates", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "error", resp["status"])
	assert.Contains(t, resp["message"], "Invalid request body")
}

func TestUploadCertificate_MissingRequiredFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger}

	router := gin.New()
	router.Use(middleware.CorrelationIDMiddleware(server.logger))
	router.POST("/certificates", server.UploadCertificate)

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

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestUploadCertificate_InvalidPEMFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger}

	router := gin.New()
	router.Use(middleware.CorrelationIDMiddleware(server.logger))
	router.POST("/certificates", server.UploadCertificate)

	reqBody := UploadCertificateRequest{
		Name:        "test-cert",
		Certificate: "not a valid PEM certificate",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/certificates", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "error", resp["status"])
	assert.Contains(t, resp["message"], "Invalid certificate")
}

func TestUploadCertificate_DatabaseSaveError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockDB := NewMockStorage()
	mockDB.saveErr = errors.New("database error")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger}

	router := gin.New()
	router.Use(middleware.CorrelationIDMiddleware(server.logger))
	router.POST("/certificates", server.UploadCertificate)

	reqBody := UploadCertificateRequest{
		Name:        "test-cert",
		Certificate: validTestCert,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/certificates", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "error", resp["status"])
	assert.Contains(t, resp["message"], "Failed to save certificate")
}

func TestDeleteCertificate_EmptyID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger}

	router := gin.New()
	router.Use(middleware.CorrelationIDMiddleware(server.logger))
	router.DELETE("/certificates", func(c *gin.Context) {
		server.DeleteCertificate(c, "")
	})

	req := httptest.NewRequest(http.MethodDelete, "/certificates", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

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
	server := &APIServer{db: mockDB, logger: logger}

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
	gin.SetMode(gin.TestMode)
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger}

	router := gin.New()
	router.Use(middleware.CorrelationIDMiddleware(server.logger))
	router.POST("/certificates", server.UploadCertificate)

	reqBody := UploadCertificateRequest{
		Name:        "empty-cert",
		Certificate: "-----BEGIN CERTIFICATE-----\n-----END CERTIFICATE-----",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/certificates", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "error", resp["status"])
}

// TestUploadCertificate_MalformedPEMHeaders tests malformed PEM headers
func TestUploadCertificate_MalformedPEMHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger}

	router := gin.New()
	router.Use(middleware.CorrelationIDMiddleware(server.logger))
	router.POST("/certificates", server.UploadCertificate)

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

			router.ServeHTTP(w, req)

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
	server := &APIServer{db: mockDB, logger: logger}

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
	gin.SetMode(gin.TestMode)
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
	server := &APIServer{db: mockDB, logger: logger}

	router := gin.New()
	router.Use(middleware.CorrelationIDMiddleware(server.logger))
	router.GET("/certificates", server.ListCertificates)

	req := httptest.NewRequest(http.MethodGet, "/certificates", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

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
	server := &APIServer{db: mockDB, logger: logger}

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
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(middleware.CorrelationIDMiddleware(server.logger))
		router.DELETE("/certificates", func(c *gin.Context) {
			server.DeleteCertificate(c, "")
		})

		req := httptest.NewRequest(http.MethodDelete, "/certificates", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestValidateCertificate_BoundaryConditions tests certificate validation edge cases
func TestValidateCertificate_BoundaryConditions(t *testing.T) {
	mockDB := NewMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &APIServer{db: mockDB, logger: logger}

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
	server := &APIServer{db: mockDB, logger: logger}

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
	gin.SetMode(gin.TestMode)
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
	server := &APIServer{db: mockDB, logger: logger}

	router := gin.New()
	router.Use(middleware.CorrelationIDMiddleware(server.logger))
	router.GET("/certificates", server.ListCertificates)

	// Launch concurrent requests
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/certificates", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
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
