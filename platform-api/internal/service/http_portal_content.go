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

package service

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"

	"github.com/wso2/api-platform/platform-api/internal/model"
)

// portalContentFileName must end in .zip, or the portal refuses the upload.
const portalContentFileName = "content.zip"

// PublishContent implements PortalPublisher with PUT on the listing's /assets. The portal only
// adds or replaces files, so the call is safe to repeat.
func (p *HTTPPortalPublisher) PublishContent(ctx context.Context, portal *model.APIPortal, apiType, apiHandle string, content *PortalContent) error {
	listing, err := portalListingFor(apiType)
	if err != nil {
		return err
	}
	body, contentType, err := buildPortalContentMultipart(content)
	if err != nil {
		return err
	}

	authHeader, err := p.authHeader(ctx, portal)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, listingURL(portal, listing, apiHandle)+"/assets", body)
	if err != nil {
		return fmt.Errorf("failed to build portal content request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", authHeader)

	resp, err := p.contentClient.Do(req)
	if err != nil {
		return fmt.Errorf("portal content push failed: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case isPortalAuthFailure(resp.StatusCode):
		return portalAuthError(resp.StatusCode)
	case isPortalRejection(resp.StatusCode):
		return portalRejection(resp, "the API Portal rejected the content of this listing")
	default:
		return fmt.Errorf("portal content push failed: unexpected status %d", resp.StatusCode)
	}
}

// buildPortalContentMultipart wraps the ZIP as the "content" part.
func buildPortalContentMultipart(content *PortalContent) (*bytes.Buffer, string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	part, err := mw.CreateFormFile("content", portalContentFileName)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create content part: %w", err)
	}
	if _, err := part.Write(content.Archive); err != nil {
		return nil, "", fmt.Errorf("failed to write content part: %w", err)
	}

	if err := mw.Close(); err != nil {
		return nil, "", fmt.Errorf("failed to finalize multipart body: %w", err)
	}
	return &buf, mw.FormDataContentType(), nil
}
