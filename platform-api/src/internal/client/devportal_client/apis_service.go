/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package devportal_client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	dto "platform-api/src/internal/client/devportal_client/dto"

	"github.com/go-playground/validator/v10"
)

// APIsService defines operations for API metadata and template management.
type APIsService interface {
	Publish(orgID string, meta dto.APIMetadataRequest, apiDefinition io.Reader, apiDefFilename string, schemaDefinition io.Reader, schemaFilename string) (*dto.APIResponse, error)
	Update(orgID, apiID string, meta dto.APIMetadataRequest, apiDefinition io.Reader, apiDefFilename string, schemaDefinition io.Reader, schemaFilename string) (*dto.APIResponse, error)
	Delete(orgID, apiID string) error
	Get(orgID, apiID string) (*dto.APIResponse, error)
	List(orgID string) ([]dto.APIResponse, error)

	// Template file operations
	UploadTemplate(orgID, apiID string, r io.Reader, filename string) error
	UpdateTemplate(orgID, apiID string, r io.Reader, filename string) error
	GetTemplate(orgID, apiID string) ([]byte, error)
	DeleteTemplate(orgID, apiID string) error
}

type apisService struct {
	DevPortalClient *DevPortalClient
	validator       *validator.Validate
}

// Publish creates a new API (multipart/form-data)
func (s *apisService) Publish(orgID string, meta dto.APIMetadataRequest, apiDefinition io.Reader, apiDefFilename string, schemaDefinition io.Reader, schemaFilename string) (*dto.APIResponse, error) {
	if err := s.validator.Struct(meta); err != nil {
		return nil, fmt.Errorf("API publish validation failed for orgID=%s: %w", orgID, err)
	}

	// Validate filenames
	if apiDefinition == nil || apiDefFilename == "" {
		return nil, fmt.Errorf("API definition and filename are required for publish")
	}
	if schemaDefinition != nil && schemaFilename == "" {
		return nil, fmt.Errorf("schema filename cannot be empty when schema is provided")
	}
	if schemaFilename != "" && schemaDefinition == nil {
		return nil, fmt.Errorf("schema definition cannot be nil when schemaFilename is provided")
	}

	url := s.DevPortalClient.buildURL(devportalOrganizationsPath, orgID, apisPath)
	body, contentType, err := createAPIMultipart(meta, apiDefinition, apiDefFilename, schemaDefinition, schemaFilename)
	if err != nil {
		return nil, err
	}
	// Buffer payload to byte slice for retry support
	payload := body.Bytes()

	req, err := s.DevPortalClient.NewRequest(http.MethodPost, url).
		BuildMultipart(bytes.NewReader(payload), contentType)
	if err != nil {
		return nil, err
	}
	// Enable request body replay for retries
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(payload)), nil
	}
	resp, err := s.DevPortalClient.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, NewDevPortalError(resp.StatusCode, fmt.Sprintf("API publish failed: %s", string(b)), resp.StatusCode >= 500, nil)
	}
	var out dto.APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update updates API metadata and optional files
func (s *apisService) Update(orgID, apiID string, meta dto.APIMetadataRequest, apiDefinition io.Reader, apiDefFilename string, schemaDefinition io.Reader, schemaFilename string) (*dto.APIResponse, error) {
	if err := s.validator.Struct(meta); err != nil {
		return nil, fmt.Errorf("API update validation failed for orgID=%s, apiID=%s: %w", orgID, apiID, err)
	}

	// Validate filenames
	if apiDefinition != nil && apiDefFilename == "" {
		return nil, fmt.Errorf("API definition filename cannot be empty when apiDefinition is provided")
	}
	if apiDefFilename != "" && apiDefinition == nil {
		return nil, fmt.Errorf("API definition cannot be nil when apiDefFilename is provided")
	}
	if schemaDefinition != nil && schemaFilename == "" {
		return nil, fmt.Errorf("schema filename cannot be empty when schema is provided")
	}
	if schemaFilename != "" && schemaDefinition == nil {
		return nil, fmt.Errorf("schema definition cannot be nil when schemaFilename is provided")
	}

	url := s.DevPortalClient.buildURL(devportalOrganizationsPath, orgID, apisPath, apiID)
	body, contentType, err := createAPIMultipart(meta, apiDefinition, apiDefFilename, schemaDefinition, schemaFilename)
	if err != nil {
		return nil, err
	}
	// Buffer payload to byte slice for retry support
	payload := body.Bytes()
	req, err := s.DevPortalClient.NewRequest(http.MethodPost, url).
		BuildMultipart(bytes.NewReader(payload), contentType)
	if err != nil {
		return nil, err
	}
	// Enable request body replay for retries
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(payload)), nil
	}
	resp, err := s.DevPortalClient.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		devPortalErr := NewDevPortalError(resp.StatusCode, fmt.Sprintf("API update failed: %s", string(b)), resp.StatusCode >= 500, nil)
		return nil, handleAPIError(devPortalErr)
	}
	var out dto.APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete removes an API
func (s *apisService) Delete(orgID, apiID string) error {
	url := s.DevPortalClient.buildURL(devportalOrganizationsPath, orgID, apisPath, apiID)
	req, err := s.DevPortalClient.NewRequest(http.MethodDelete, url).
		Build()
	if err != nil {
		return err
	}
	if err := s.DevPortalClient.doNoContent(req, []int{200, 204}); err != nil {
		return handleAPIError(err)
	}
	return nil
}

// Get retrieves an API
func (s *apisService) Get(orgID, apiID string) (*dto.APIResponse, error) {
	url := s.DevPortalClient.buildURL(devportalOrganizationsPath, orgID, apisPath, apiID)
	req, err := s.DevPortalClient.NewRequest(http.MethodGet, url).Build()
	if err != nil {
		return nil, err
	}
	var out dto.APIResponse
	if err := s.DevPortalClient.doAndDecode(req, []int{200}, &out); err != nil {
		return nil, handleAPIError(err)
	}
	return &out, nil
}

// List retrieves all APIs for an organization
func (s *apisService) List(orgID string) ([]dto.APIResponse, error) {
	url := s.DevPortalClient.buildURL(devportalOrganizationsPath, orgID, apisPath)
	req, err := s.DevPortalClient.NewRequest(http.MethodGet, url).Build()
	if err != nil {
		return nil, err
	}
	var out dto.APIListResponse
	if err := s.DevPortalClient.doAndDecode(req, []int{200}, &out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

// Template operations
func (s *apisService) UploadTemplate(orgID, apiID string, r io.Reader, filename string) error {
	if filename == "" || strings.ContainsAny(filename, "\x00") || len(filename) > 255 {
		return fmt.Errorf("invalid filename")
	}
	if r == nil {
		return fmt.Errorf("template reader cannot be nil")
	}
	url := s.DevPortalClient.buildURL(devportalOrganizationsPath, orgID, apisPath, apiID, templatePath)
	buf, contentType, err := createTemplateMultipart(r, filename)
	if err != nil {
		return err
	}
	// Buffer payload to byte slice for retry support
	payload := buf.Bytes()
	req, err := s.DevPortalClient.NewRequest(http.MethodPost, url).
		BuildMultipart(bytes.NewReader(payload), contentType)
	if err != nil {
		return err
	}
	// Enable request body replay for retries
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(payload)), nil
	}
	resp, err := s.DevPortalClient.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		devPortalErr := NewDevPortalError(resp.StatusCode, fmt.Sprintf("template upload failed: %s", string(b)), resp.StatusCode >= 500, nil)
		return handleAPIError(devPortalErr)
	}
	return nil
}

func (s *apisService) UpdateTemplate(orgID, apiID string, r io.Reader, filename string) error {
	if filename == "" || strings.ContainsAny(filename, "\x00") || len(filename) > 255 {
		return fmt.Errorf("invalid filename")
	}
	if r == nil {
		return fmt.Errorf("template reader cannot be nil")
	}
	url := s.DevPortalClient.buildURL(devportalOrganizationsPath, orgID, apisPath, apiID, templatePath)
	buf, contentType, err := createTemplateMultipart(r, filename)
	if err != nil {
		return err
	}
	// Buffer payload to byte slice for retry support
	payload := buf.Bytes()
	req, err := s.DevPortalClient.NewRequest(http.MethodPut, url).
		BuildMultipart(bytes.NewReader(payload), contentType)
	if err != nil {
		return err
	}
	// Enable request body replay for retries
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(payload)), nil
	}
	resp, err := s.DevPortalClient.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		devPortalErr := NewDevPortalError(resp.StatusCode, fmt.Sprintf("template update failed: %s", string(b)), resp.StatusCode >= 500, nil)
		return handleAPIError(devPortalErr)
	}
	return nil
}

func (s *apisService) GetTemplate(orgID, apiID string) ([]byte, error) {
	url := s.DevPortalClient.buildURL(devportalOrganizationsPath, orgID, apisPath, apiID, templatePath)
	req, err := s.DevPortalClient.NewRequest(http.MethodGet, url).Build()
	if err != nil {
		return nil, err
	}
	resp, err := s.DevPortalClient.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		devPortalErr := NewDevPortalError(resp.StatusCode, fmt.Sprintf("template retrieval failed: %s", string(b)), resp.StatusCode >= 500, nil)
		return nil, handleAPIError(devPortalErr)
	}
	// Read the entire template into memory to simplify the API surface
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read template: %w", err)
	}
	return b, nil
}

func (s *apisService) DeleteTemplate(orgID, apiID string) error {
	url := s.DevPortalClient.buildURL(devportalOrganizationsPath, orgID, apisPath, apiID, templatePath)
	req, err := s.DevPortalClient.NewRequest(http.MethodDelete, url).
		Build()
	if err != nil {
		return err
	}
	resp, err := s.DevPortalClient.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resp.Body)
		devPortalErr := NewDevPortalError(resp.StatusCode, fmt.Sprintf("template deletion failed: %s", string(b)), resp.StatusCode >= 500, nil)
		return handleAPIError(devPortalErr)
	}
	return nil
}

// handleAPIError converts 404 DevPortalErrors to ErrAPINotFound for consistent error handling
func handleAPIError(err error) error {
	if devPortalErr, ok := err.(*DevPortalError); ok && devPortalErr.Code == http.StatusNotFound {
		return ErrAPINotFound
	}
	return err
}

// helper: create multipart body with apiMetadata JSON and optional files
func createAPIMultipart(meta dto.APIMetadataRequest, apiDef io.Reader, apiDefName string, schema io.Reader, schemaName string) (body *bytes.Buffer, contentType string, err error) {
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)

	// Add apiMetadata field as JSON with explicit content type
	metadataJSON, err := json.Marshal(meta)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal API metadata: %w", err)
	}

	// Create apiMetadata field with application/json content type
	metadataField, err := mw.CreateFormField("apiMetadata")
	if err != nil {
		return nil, "", ErrFormFieldCreationFailed
	}
	if _, err := metadataField.Write(metadataJSON); err != nil {
		return nil, "", fmt.Errorf("failed to write apiMetadata: %w", err)
	}

	// Add apiDefinition file field
	if apiDef != nil {
		fname := filepath.Base(apiDefName)
		if fname == "" || strings.ContainsAny(fname, "\x00") || len(fname) > 255 {
			return nil, "", fmt.Errorf("invalid api definition filename")
		}
		fileField, err := mw.CreateFormFile("apiDefinition", fname)
		if err != nil {
			return nil, "", ErrFormFieldCreationFailed
		}
		if _, err := io.Copy(fileField, apiDef); err != nil {
			return nil, "", ErrFileWriteFailed
		}
	}

	// optional schemaDefinition file
	if schema != nil {
		fname := filepath.Base(schemaName)
		if fname == "" || strings.ContainsAny(fname, "\x00") || len(fname) > 255 {
			return nil, "", fmt.Errorf("invalid schema filename")
		}
		fw, err := mw.CreateFormFile("schemaDefinition", fname)
		if err != nil {
			return nil, "", ErrFormFieldCreationFailed
		}
		if _, err := io.Copy(fw, schema); err != nil {
			return nil, "", ErrFileWriteFailed
		}
	}

	if err := mw.Close(); err != nil {
		return nil, "", ErrMultipartCreationFailed
	}
	return buf, mw.FormDataContentType(), nil
}

// helper: create multipart body for template upload/update
func createTemplateMultipart(r io.Reader, filename string) (*bytes.Buffer, string, error) {
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)
	fw, err := mw.CreateFormFile("apiContent", filepath.Base(filename))
	if err != nil {
		return nil, "", ErrFormFieldCreationFailed
	}
	if _, err := io.Copy(fw, r); err != nil {
		return nil, "", ErrFileWriteFailed
	}
	if err := mw.Close(); err != nil {
		return nil, "", ErrMultipartCreationFailed
	}
	return buf, mw.FormDataContentType(), nil
}

// Expose via DevPortalClient
func (c *DevPortalClient) APIs() APIsService {
	return &apisService{
		DevPortalClient: c,
		validator:       c.validator,
	}
}
