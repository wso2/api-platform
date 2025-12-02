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
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"go.uber.org/zap"
)

// UploadCertificateRequest represents the request body for certificate upload
type UploadCertificateRequest struct {
	Certificate string `json:"certificate" binding:"required"` // PEM-encoded certificate
	Name        string `json:"name" binding:"required"`        // Unique certificate name
}

// CertificateResponse represents a certificate information response
type CertificateResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Subject  string `json:"subject,omitempty"`
	Issuer   string `json:"issuer,omitempty"`
	NotAfter string `json:"notAfter,omitempty"`
	Count    int    `json:"count"` // Number of certs in file
	Message  string `json:"message,omitempty"`
	Status   string `json:"status"` // success, error
}

// ListCertificatesResponse represents the response for listing certificates
type ListCertificatesResponse struct {
	Certificates []CertificateResponse `json:"certificates"`
	TotalCount   int                   `json:"totalCount"`
	TotalBytes   int                   `json:"totalBytes"`
	Status       string                `json:"status"`
}

// UploadCertificate handles certificate upload via REST API
// POST /certificates
func (s *APIServer) UploadCertificate(c *gin.Context) {
	correlationID := middleware.GetCorrelationID(c)
	log := s.logger.With(zap.String("correlation_id", correlationID))

	var req UploadCertificateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warn("Invalid certificate upload request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request body: " + err.Error(),
		})
		return
	}

	// Validate certificate format
	certData := []byte(req.Certificate)
	count, err := s.validateCertificate(certData)
	if err != nil {
		log.Warn("Invalid certificate provided", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid certificate: " + err.Error(),
		})
		return
	}

	// Extract certificate metadata
	subject, issuer, notBefore, notAfter, err := s.extractCertificateMetadata(certData)
	if err != nil {
		log.Warn("Failed to extract certificate metadata", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Failed to parse certificate metadata: " + err.Error(),
		})
		return
	}

	// Generate unique ID
	certID := uuid.New().String()

	// Create certificate model
	cert := &models.StoredCertificate{
		ID:          certID,
		Name:        req.Name,
		Certificate: certData,
		Subject:     subject,
		Issuer:      issuer,
		NotBefore:   notBefore,
		NotAfter:    notAfter,
		CertCount:   count,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Save to database
	if err := s.db.SaveCertificate(cert); err != nil {
		log.Error("Failed to save certificate to database",
			zap.String("name", req.Name),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to save certificate: " + err.Error(),
		})
		return
	}

	log.Info("Certificate saved to database successfully",
		zap.String("id", certID),
		zap.String("name", req.Name),
		zap.Int("cert_count", count))

	// Get cert store from snapshot manager
	translator := s.snapshotManager.GetTranslator()
	if translator == nil || translator.GetCertStore() == nil {
		log.Error("Certificate store not available")
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Certificate store not configured",
		})
		return
	}

	certStore := translator.GetCertStore()

	// Reload certificates from database
	if err := certStore.Reload(); err != nil {
		log.Error("Failed to reload certificates", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Certificate saved but failed to reload: " + err.Error(),
		})
		return
	}

	// Trigger SDS update by regenerating the snapshot
	if err := s.snapshotManager.UpdateSnapshot(context.Background(), correlationID); err != nil {
		log.Error("Failed to update SDS snapshot", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Certificate reloaded but failed to update SDS: " + err.Error(),
		})
		return
	}

	log.Info("SDS snapshot updated with new certificate",
		zap.String("id", certID),
		zap.String("name", req.Name))

	c.JSON(http.StatusCreated, CertificateResponse{
		ID:       certID,
		Name:     req.Name,
		Subject:  subject,
		Issuer:   issuer,
		NotAfter: notAfter.Format("2006-01-02 15:04:05"),
		Count:    count,
		Message:  "Certificate uploaded and SDS updated successfully",
		Status:   "success",
	})
}

// ListCertificates lists all custom certificates
// GET /certificates
func (s *APIServer) ListCertificates(c *gin.Context) {
	correlationID := middleware.GetCorrelationID(c)
	log := s.logger.With(zap.String("correlation_id", correlationID))

	// Get certificates from database
	certs, err := s.db.ListCertificates()
	if err != nil {
		log.Error("Failed to list certificates from database", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to list certificates: " + err.Error(),
		})
		return
	}

	var certificates []CertificateResponse
	totalBytes := 0

	for _, cert := range certs {
		totalBytes += len(cert.Certificate)

		certificates = append(certificates, CertificateResponse{
			ID:       cert.ID,
			Name:     cert.Name,
			Subject:  cert.Subject,
			Issuer:   cert.Issuer,
			NotAfter: cert.NotAfter.Format("2006-01-02 15:04:05"),
			Count:    cert.CertCount,
			Status:   "success",
		})
	}

	c.JSON(http.StatusOK, ListCertificatesResponse{
		Certificates: certificates,
		TotalCount:   len(certificates),
		TotalBytes:   totalBytes,
		Status:       "success",
	})
}

// DeleteCertificate deletes a certificate by ID
// DELETE /certificates/:id
func (s *APIServer) DeleteCertificate(c *gin.Context, id string) {
	correlationID := middleware.GetCorrelationID(c)
	log := s.logger.With(zap.String("correlation_id", correlationID))

	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Certificate ID is required",
		})
		return
	}

	translator := s.snapshotManager.GetTranslator()
	if translator == nil || translator.GetCertStore() == nil {
		log.Error("Certificate store not available")
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Certificate store not configured",
		})
		return
	}

	// Delete from database
	if err := s.db.DeleteCertificate(id); err != nil {
		log.Error("Failed to delete certificate",
			zap.String("id", id),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Certificate not found or failed to delete: " + err.Error(),
		})
		return
	}

	log.Info("Certificate deleted from database", zap.String("id", id))

	certStore := translator.GetCertStore()

	// Reload certificates from database
	if err := certStore.Reload(); err != nil {
		log.Error("Failed to reload certificates", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Certificate deleted but failed to reload: " + err.Error(),
		})
		return
	}

	// Trigger SDS update
	if err := s.snapshotManager.UpdateSnapshot(context.Background(), correlationID); err != nil {
		log.Error("Failed to update SDS snapshot", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Certificate deleted and reloaded but failed to update SDS: " + err.Error(),
		})
		return
	}

	log.Info("SDS snapshot updated after certificate deletion", zap.String("id", id))

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Certificate deleted and SDS updated successfully",
		"id":      id,
	})
}

// ReloadCertificates manually triggers certificate reload and SDS update
// POST /certificates/reload
func (s *APIServer) ReloadCertificates(c *gin.Context) {
	correlationID := middleware.GetCorrelationID(c)
	log := s.logger.With(zap.String("correlation_id", correlationID))

	translator := s.snapshotManager.GetTranslator()
	if translator == nil || translator.GetCertStore() == nil {
		log.Error("Certificate store not available")
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Certificate store not configured",
		})
		return
	}

	certStore := translator.GetCertStore()

	// Reload certificates from database
	if err := certStore.Reload(); err != nil {
		log.Error("Failed to reload certificates", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to reload certificates: " + err.Error(),
		})
		return
	}

	// Trigger SDS update
	if err := s.snapshotManager.UpdateSnapshot(context.Background(), correlationID); err != nil {
		log.Error("Failed to update SDS snapshot", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Certificates reloaded but failed to update SDS: " + err.Error(),
		})
		return
	}

	log.Info("Certificates reloaded and SDS snapshot updated")

	combinedCerts := certStore.GetCombinedCertificates()
	c.JSON(http.StatusOK, gin.H{
		"status":      "success",
		"message":     "Certificates reloaded and SDS updated successfully",
		"total_bytes": len(combinedCerts),
	})
}

// Helper functions

// extractCertificateMetadata extracts metadata from the first certificate in the chain
func (s *APIServer) extractCertificateMetadata(data []byte) (subject, issuer string, notBefore, notAfter time.Time, err error) {
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}

		if block.Type != "CERTIFICATE" {
			continue
		}

		cert, parseErr := x509.ParseCertificate(block.Bytes)
		if parseErr != nil {
			err = parseErr
			return
		}

		// Use first certificate for metadata
		subject = cert.Subject.String()
		issuer = cert.Issuer.String()
		notBefore = cert.NotBefore
		notAfter = cert.NotAfter
		return
	}

	err = fmt.Errorf("no valid certificate found")
	return
}

func (s *APIServer) validateCertificate(data []byte) (int, error) {
	count := 0
	rest := data

	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}

		if block.Type != "CERTIFICATE" {
			continue
		}

		_, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return 0, fmt.Errorf("invalid certificate: %w", err)
		}

		count++
	}

	if count == 0 {
		return 0, fmt.Errorf("no valid certificates found in PEM data")
	}

	return count, nil
}
