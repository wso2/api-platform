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
	dto "platform-api/src/internal/client/devportal_client/dto"
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
	GetTemplate(orgID, apiID string) (io.ReadCloser, error)
	DeleteTemplate(orgID, apiID string) error
}

type apisService struct {
	DevPortalClient *DevPortalClient
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

// Publish creates a new API (multipart/form-data)
func (s *apisService) Publish(orgID string, meta dto.APIMetadataRequest, apiDefinition io.Reader, apiDefFilename string, schemaDefinition io.Reader, schemaFilename string) (*dto.APIResponse, error) {
	url := s.DevPortalClient.buildURL(devportalOrganizationsPath, orgID, apisPath)
	body, contentType, err := createAPIMultipart(meta, apiDefinition, apiDefFilename, schemaDefinition, schemaFilename)
	if err != nil {
		return nil, err
	}
	// Buffer payload to byte slice for retry support
	payload := append([]byte(nil), body.Bytes()...)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
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
	url := s.DevPortalClient.buildURL(devportalOrganizationsPath, orgID, apisPath, apiID)
	body, contentType, err := createAPIMultipart(meta, apiDefinition, apiDefFilename, schemaDefinition, schemaFilename)
	if err != nil {
		return nil, err
	}
	// Buffer payload to byte slice for retry support
	payload := append([]byte(nil), body.Bytes()...)
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	// Enable request body replay for retries
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(payload)), nil
	}
	resp, err := s.DevPortalClient.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusNotFound {
			return nil, ErrAPINotFound
		}
		return nil, NewDevPortalError(resp.StatusCode, fmt.Sprintf("API update failed: %s", string(b)), resp.StatusCode >= 500, nil)
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
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	resp, err := s.DevPortalClient.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusNotFound {
			return ErrAPINotFound
		}
		return NewDevPortalError(resp.StatusCode, fmt.Sprintf("API deletion failed: %s", string(b)), resp.StatusCode >= 500, nil)
	}
	return nil
}

// Get retrieves an API
func (s *apisService) Get(orgID, apiID string) (*dto.APIResponse, error) {
	url := s.DevPortalClient.buildURL(devportalOrganizationsPath, orgID, apisPath, apiID)
	req, err := s.DevPortalClient.newJSONRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.DevPortalClient.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrAPINotFound
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, NewDevPortalError(resp.StatusCode, fmt.Sprintf("failed to get API: %s", string(b)), resp.StatusCode >= 500, nil)
	}
	var out dto.APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// List retrieves all APIs for an organization
func (s *apisService) List(orgID string) ([]dto.APIResponse, error) {
	url := s.DevPortalClient.buildURL(devportalOrganizationsPath, orgID, apisPath)
	req, err := s.DevPortalClient.newJSONRequest("GET", url, nil)
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
		return nil, NewDevPortalError(resp.StatusCode, fmt.Sprintf("API list failed: %s", string(b)), resp.StatusCode >= 500, nil)
	}
	var out dto.APIListResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

// Template operations
func (s *apisService) UploadTemplate(orgID, apiID string, r io.Reader, filename string) error {
	url := s.DevPortalClient.buildURL(devportalOrganizationsPath, orgID, apisPath, apiID, templatePath)
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)
	fw, err := mw.CreateFormFile("apiContent", filepath.Base(filename))
	if err != nil {
		return ErrFormFieldCreationFailed
	}
	if _, err := io.Copy(fw, r); err != nil {
		return ErrFileWriteFailed
	}
	if err := mw.Close(); err != nil {
		return ErrMultipartCreationFailed
	}
	// Buffer payload to byte slice for retry support
	payload := append([]byte(nil), buf.Bytes()...)
	contentType := mw.FormDataContentType()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
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
		return NewDevPortalError(resp.StatusCode, fmt.Sprintf("template upload failed: %s", string(b)), resp.StatusCode >= 500, nil)
	}
	return nil
}

func (s *apisService) UpdateTemplate(orgID, apiID string, r io.Reader, filename string) error {
	url := s.DevPortalClient.buildURL(devportalOrganizationsPath, orgID, apisPath, apiID, templatePath)
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)
	fw, err := mw.CreateFormFile("apiContent", filepath.Base(filename))
	if err != nil {
		return ErrFormFieldCreationFailed
	}
	if _, err := io.Copy(fw, r); err != nil {
		return ErrFileWriteFailed
	}
	if err := mw.Close(); err != nil {
		return ErrMultipartCreationFailed
	}
	// Buffer payload to byte slice for retry support
	payload := append([]byte(nil), buf.Bytes()...)
	contentType := mw.FormDataContentType()
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
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
		return NewDevPortalError(resp.StatusCode, fmt.Sprintf("template update failed: %s", string(b)), resp.StatusCode >= 500, nil)
	}
	return nil
}

func (s *apisService) GetTemplate(orgID, apiID string) (io.ReadCloser, error) {
	url := s.DevPortalClient.buildURL(devportalOrganizationsPath, orgID, apisPath, apiID, templatePath)
	req, err := s.DevPortalClient.newJSONRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.DevPortalClient.do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, NewDevPortalError(resp.StatusCode, fmt.Sprintf("template retrieval failed: %s", string(b)), resp.StatusCode >= 500, nil)
	}
	// caller must close the returned ReadCloser
	return resp.Body, nil
}

func (s *apisService) DeleteTemplate(orgID, apiID string) error {
	url := s.DevPortalClient.buildURL(devportalOrganizationsPath, orgID, apisPath, apiID, templatePath)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	resp, err := s.DevPortalClient.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return NewDevPortalError(resp.StatusCode, fmt.Sprintf("template deletion failed: %s", string(b)), resp.StatusCode >= 500, nil)
	}
	return nil
}

// Expose via DevPortalClient
func (c *DevPortalClient) APIs() APIsService {
	return &apisService{DevPortalClient: c}
}
