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
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/client"
	"github.com/wso2/api-platform/platform-api/internal/model"

	"gopkg.in/yaml.v3"
)

// HTTPPortalPublisher is the real PortalPublisher implementation: an
// existence check (GET /apis/{handle}) decides whether the
// metadata+definition push uses create (POST /apis) or update (PUT
// /apis/{handle}), both addressed by the API's own handle — no
// portal-returned reference ID is stored locally.
//
// Only the metadata and definition are pushed; the content ZIP (thumbnail,
// landing page, documents) is not implemented yet.
type HTTPPortalPublisher struct {
	client       *client.RetryableHTTPClient
	authRegistry *APIPortalAuthRegistry
}

// NewHTTPPortalPublisher returns a PortalPublisher that pushes to the real
// API Portal over HTTP, authenticating via the shared-key S2S auth scheme —
// the raw key is resolved per-portal through authRegistry (each api_portals
// row carries its own encrypted key), never a single server-wide secret.
func NewHTTPPortalPublisher(authRegistry *APIPortalAuthRegistry, retryClient *client.RetryableHTTPClient) PortalPublisher {
	return &HTTPPortalPublisher{client: retryClient, authRegistry: authRegistry}
}

// authHeader resolves the Authorization header for this specific portal via
// the shared-key auth registry — never a fixed value, since different
// api_portals rows (different deployed portal instances) each carry their
// own key.
func (p *HTTPPortalPublisher) authHeader(ctx context.Context, portal *model.APIPortal) (string, error) {
	provider, err := p.authRegistry.Get(portal.Handle, portal.OrganizationID)
	if err != nil {
		return "", fmt.Errorf("failed to resolve API Portal auth: %w", err)
	}
	return provider.AuthorizationHeader(ctx)
}

// portalErrorBodyMaxBytes bounds how much of a 4xx response body is read when
// looking for a known conflict reason; it is always a small JSON error envelope.
const portalErrorBodyMaxBytes = 8 << 10 // 8 KiB

// portalErrorEnvelope is the shape of the API Portal's error responses. Its
// "message" field is a machine-readable error code (e.g. "ERR_SUB_EXIST"); the
// human-readable text under errors[] is deliberately never read.
type portalErrorEnvelope struct {
	Message string `json:"message"`
}

// knownPortalConflictReasons maps a portal error code to a short, pre-approved
// phrase. Raw portal error text is never forwarded to the client
// (error-handling.md); any other code falls back to defaultPortalConflictReason.
var knownPortalConflictReasons = map[string]string{
	"ERR_SUB_EXIST": "active subscriptions are removed",
	"ERR_KEY_EXIST": "active API keys are removed",
}

// defaultPortalConflictReason is the fallback for any portal response that
// isn't one of the known codes above.
const defaultPortalConflictReason = "the conflict is resolved"

// portalConflictReason returns the reason phrase for the error code in a 4xx
// response body, or the generic fallback if the body is unparseable or the code
// is unknown.
func portalConflictReason(body []byte) string {
	var envelope portalErrorEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return defaultPortalConflictReason
	}
	if reason, ok := knownPortalConflictReasons[envelope.Message]; ok {
		return reason
	}
	return defaultPortalConflictReason
}

// isPortalAuthFailure reports whether the portal refused our credential
// (401/403). That is a platform-side setup problem, not a conflict the caller
// can resolve by changing the listing.
func isPortalAuthFailure(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusForbidden
}

// portalAuthError is deliberately not a *PortalConflictError, so the service
// layer surfaces it as PUBLICATION_PORTAL_UNAVAILABLE rather than a conflict.
func portalAuthError(status int) error {
	return fmt.Errorf("the API Portal rejected the configured credential (status %d)", status)
}

// isPortalRejection reports whether status is a 4xx the portal will keep
// returning for this request as-is — a conflicting handle or display name, an
// unknown subscription plan, or any other validation failure. Retrying
// without changing the listing can't help, so it is not a transient failure.
func isPortalRejection(status int) bool {
	return status >= 400 && status < 500
}

// portalRejection builds the conflict error for a rejected request, with a
// reason drawn only from the known portal error codes.
func portalRejection(resp *http.Response, message string) *PortalConflictError {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, portalErrorBodyMaxBytes))
	return &PortalConflictError{
		Message: fmt.Sprintf("%s (status %d)", message, resp.StatusCode),
		Reason:  portalConflictReason(body),
	}
}

// portalRESTBase is where the API Portal mounts its REST API. The registered portal URL is
// the portal root, without this prefix.
const portalRESTBase = "/api-portal/api/v0.9"

// apisURL is the portal's API collection URL.
func apisURL(portal *model.APIPortal) string {
	return strings.TrimRight(portal.URL, "/") + portalRESTBase + "/apis"
}

// apiURL is the portal's URL for the listing with the given handle.
func apiURL(portal *model.APIPortal, apiHandle string) string {
	return apisURL(portal) + "/" + url.PathEscape(apiHandle)
}

// Publish implements PortalPublisher.
func (p *HTTPPortalPublisher) Publish(ctx context.Context, portal *model.APIPortal, apiHandle string, pub *model.Publication, definition *model.PublicationContent) error {
	exists, err := p.checkExists(ctx, portal, apiHandle)
	if err != nil {
		return err
	}

	method, path := http.MethodPost, apisURL(portal)
	if exists {
		method, path = http.MethodPut, apiURL(portal, apiHandle)
	}

	// The portal itself decides whether an empty definition is acceptable.
	if definition == nil {
		definition = &model.PublicationContent{}
	}
	return p.pushMetadata(ctx, portal, method, path, apiHandle, model.PublicationStatusPublished, pub, definition)
}

// Deprecate implements PortalPublisher. The portal has no status-only call, so the
// live metadata is re-sent unchanged apart from the status; no definition is sent.
func (p *HTTPPortalPublisher) Deprecate(ctx context.Context, portal *model.APIPortal, apiHandle string, live *model.Publication) error {
	return p.pushMetadata(ctx, portal, http.MethodPut, apiURL(portal, apiHandle), apiHandle, model.PublicationStatusDeprecated, live, nil)
}

// pushMetadata sends the metadata (and the definition, if non-nil) to path and maps
// the portal's response to the PortalPublisher error contract.
func (p *HTTPPortalPublisher) pushMetadata(ctx context.Context, portal *model.APIPortal, method, path, apiHandle, status string, pub *model.Publication, definition *model.PublicationContent) error {
	body, contentType, err := buildPortalMetadataMultipart(apiHandle, status, pub, definition)
	if err != nil {
		return err
	}

	authHeader, err := p.authHeader(ctx, portal)
	if err != nil {
		return err
	}

	reqCtx, cancel := context.WithTimeout(ctx, p.client.TotalTimeout())
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, method, path, body)
	if err != nil {
		return fmt.Errorf("failed to build portal metadata request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", authHeader)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("portal metadata push failed: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case isPortalAuthFailure(resp.StatusCode):
		return portalAuthError(resp.StatusCode)
	case isPortalRejection(resp.StatusCode):
		return portalRejection(resp, "the API Portal rejected this listing")
	default:
		return fmt.Errorf("portal metadata push failed: unexpected status %d", resp.StatusCode)
	}
}

// Unpublish implements PortalPublisher: DELETE /apis/{handle} on the portal. A
// 404 counts as already removed, so a retry after a successful removal
// converges. Any other 4xx (most commonly 409, when active subscriptions or API
// keys are still attached) is returned as a PortalConflictError.
func (p *HTTPPortalPublisher) Unpublish(ctx context.Context, portal *model.APIPortal, apiHandle string) error {
	authHeader, err := p.authHeader(ctx, portal)
	if err != nil {
		return err
	}

	reqCtx, cancel := context.WithTimeout(ctx, p.client.TotalTimeout())
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodDelete, apiURL(portal, apiHandle), nil)
	if err != nil {
		return fmt.Errorf("failed to build portal unpublish request: %w", err)
	}
	req.Header.Set("Authorization", authHeader)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("portal unpublish failed: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound, resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case isPortalAuthFailure(resp.StatusCode):
		return portalAuthError(resp.StatusCode)
	case isPortalRejection(resp.StatusCode):
		return portalRejection(resp, "the API Portal rejected removal of this listing")
	default:
		return fmt.Errorf("portal unpublish failed: unexpected status %d", resp.StatusCode)
	}
}

// checkExists issues GET /apis/{handle} on the portal. 200 means it exists
// (update); 404 means it doesn't (create).
func (p *HTTPPortalPublisher) checkExists(ctx context.Context, portal *model.APIPortal, apiHandle string) (bool, error) {
	authHeader, err := p.authHeader(ctx, portal)
	if err != nil {
		return false, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, p.client.TotalTimeout())
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, apiURL(portal, apiHandle), nil)
	if err != nil {
		return false, fmt.Errorf("failed to build portal existence-check request: %w", err)
	}
	req.Header.Set("Authorization", authHeader)

	resp, err := p.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("portal existence check failed: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		return true, nil
	case resp.StatusCode == http.StatusNotFound:
		return false, nil
	case isPortalAuthFailure(resp.StatusCode):
		return false, portalAuthError(resp.StatusCode)
	case isPortalRejection(resp.StatusCode):
		return false, portalRejection(resp, "the API Portal rejected the existence check")
	default:
		return false, fmt.Errorf("portal existence check failed: unexpected status %d", resp.StatusCode)
	}
}

// portalMetadataEnvelope is the "metadata" part of the portal push. The portal
// ignores apiVersion and kind; they are sent to match its sample files.
type portalMetadataEnvelope struct {
	APIVersion string                 `yaml:"apiVersion"`
	Kind       string                 `yaml:"kind"`
	Metadata   portalMetadataName     `yaml:"metadata"`
	Spec       portalMetadataSpecBody `yaml:"spec"`
}

type portalMetadataName struct {
	Name string `yaml:"name"`
}

type portalMetadataSpecBody struct {
	Type            string   `yaml:"type"`
	DisplayName     string   `yaml:"displayName"`
	Version         string   `yaml:"version"`
	Description     string   `yaml:"description,omitempty"`
	Status          string   `yaml:"status"`
	AgentVisibility string   `yaml:"agentVisibility"`
	Tags            []string `yaml:"tags"`
	// omitempty: the portal applies its "default" label only when the key is
	// absent; an explicit empty list leaves the API attached to no view.
	Labels              []string                   `yaml:"labels,omitempty"`
	ReferenceID         string                     `yaml:"referenceId"`
	Endpoints           portalMetadataEndpoints    `yaml:"endpoints"`
	BusinessInformation portalMetadataBusinessInfo `yaml:"businessInformation"`
	// SubscriptionPlans is a plain array of plan handles in this YAML envelope,
	// not the {id: <handle>} objects the JSON metadata field takes; objects here
	// make every plan lookup fail.
	SubscriptionPlans []string `yaml:"subscriptionPlans"`
}

// portalMetadataEndpoints always sends both keys, even when empty — the
// portal requires the endpoints object to be present.
type portalMetadataEndpoints struct {
	ProductionURL string `yaml:"productionUrl"`
	SandboxURL    string `yaml:"sandboxUrl"`
}

type portalMetadataBusinessInfo struct {
	BusinessOwner       string `yaml:"businessOwner,omitempty"`
	BusinessOwnerEmail  string `yaml:"businessOwnerEmail,omitempty"`
	TechnicalOwner      string `yaml:"technicalOwner,omitempty"`
	TechnicalOwnerEmail string `yaml:"technicalOwnerEmail,omitempty"`
}

// buildPortalMetadataMultipart builds the request body: a "metadata" part and,
// when definition is non-nil, a "definition" part.
func buildPortalMetadataMultipart(apiHandle, status string, pub *model.Publication, definition *model.PublicationContent) (*bytes.Buffer, string, error) {
	envelope := portalMetadataEnvelope{
		APIVersion: "api-portal.api-platform.wso2.com/v1",
		Kind:       "RestApi",
		Metadata:   portalMetadataName{Name: apiHandle},
		Spec: portalMetadataSpecBody{
			Type:            "REST",
			DisplayName:     pub.DisplayName,
			Version:         pub.Version,
			Description:     pub.Description,
			Status:          status,
			AgentVisibility: pub.AgentVisibility,
			Tags:            nonNil(pub.Tags),
			Labels:          nonNil(pub.Labels),
			ReferenceID:     apiHandle,
			Endpoints: portalMetadataEndpoints{
				ProductionURL: pub.ProductionURL,
				SandboxURL:    pub.SandboxURL,
			},
			BusinessInformation: portalMetadataBusinessInfo{
				BusinessOwner:       pub.BusinessOwner,
				BusinessOwnerEmail:  pub.BusinessOwnerEmail,
				TechnicalOwner:      pub.TechnicalOwner,
				TechnicalOwnerEmail: pub.TechnicalOwnerEmail,
			},
			SubscriptionPlans: nonNil(pub.SubscriptionPlanIds),
		},
	}

	yamlBytes, err := yaml.Marshal(envelope)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal portal metadata: %w", err)
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	metaPart, err := mw.CreateFormFile("metadata", "metadata.yaml")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create metadata part: %w", err)
	}
	if _, err := metaPart.Write(yamlBytes); err != nil {
		return nil, "", fmt.Errorf("failed to write metadata part: %w", err)
	}

	if definition != nil {
		defFileName := "definition.json"
		if definition.FileName != "" {
			defFileName = definition.FileName
		}
		defPart, err := mw.CreateFormFile("definition", defFileName)
		if err != nil {
			return nil, "", fmt.Errorf("failed to create definition part: %w", err)
		}
		if _, err := defPart.Write(definition.Content); err != nil {
			return nil, "", fmt.Errorf("failed to write definition part: %w", err)
		}
	}

	if err := mw.Close(); err != nil {
		return nil, "", fmt.Errorf("failed to finalize multipart body: %w", err)
	}
	return &buf, mw.FormDataContentType(), nil
}
