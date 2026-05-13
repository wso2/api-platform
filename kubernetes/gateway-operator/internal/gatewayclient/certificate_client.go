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

package gatewayclient

import (
	"context"
	"encoding/json"
	"fmt"
)

// CertificateUploadPayload mirrors the management-API
// CertificateUploadRequest body expected at POST /certificates.
type CertificateUploadPayload struct {
	Name        string `json:"name"`
	Certificate string `json:"certificate"`
}

// CertificateCreateResponse captures the gateway-issued id (UUID) returned
// from POST /certificates and is parsed to populate Status.Id.
type CertificateCreateResponse struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// UploadCertificate POSTs a CertificateUploadRequest payload and returns
// the parsed response (in particular the gateway-issued id).
func UploadCertificate(ctx context.Context, gatewayEndpoint string, payload CertificateUploadPayload, auth AuthHeaderFunc) (*CertificateCreateResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal certificate payload: %w", err)
	}
	respBody, err := CreateResource(ctx, gatewayEndpoint, certificatesPath, body, PayloadContentTypeJSON, auth)
	if err != nil {
		return nil, err
	}
	var out CertificateCreateResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("decode certificate response: %w", err)
	}
	if out.Id == "" {
		return nil, fmt.Errorf("empty certificate id in gateway response")
	}
	return &out, nil
}

// DeleteCertificate DELETEs /certificates/{id}.
func DeleteCertificate(ctx context.Context, gatewayEndpoint, id string, auth AuthHeaderFunc) error {
	if id == "" {
		return fmt.Errorf("certificate id is required")
	}
	return DeleteResource(ctx, gatewayEndpoint, certificatesPath, id, auth)
}
