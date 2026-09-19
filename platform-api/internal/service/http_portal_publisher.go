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
// The content ZIP (thumbnail, landing page, documents) is NOT implemented
// here yet: its exact multipart/ZIP shape hasn't been verified against API
// Portal's real asset-ingestion code, and this feature has no way to fetch a
// document's actual content in the first place — api_documents' content
// belongs to another team's not-yet-built feature. Add it once both are
// confirmed rather than guess at the shape.
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

// portalErrorBodyMaxBytes bounds how much of a 4xx response body is read
// when looking for a known conflict reason — this is always a small JSON
// error envelope (portalErrorEnvelope below), never user-facing configurable
// content, so a small fixed cap (rather than a config field) is appropriate.
const portalErrorBodyMaxBytes = 8 << 10 // 8 KiB

// portalErrorEnvelope is the shape of api-portal's own error responses
// (its util.js sendError/handleError helpers): {"code","message","errors":
// [{"message"}]}. The outer "message" field is the portal's own
// MACHINE-READABLE error code (e.g. "ERR_SUB_EXIST"), not free text —
// confirmed by reading apiMetadataService.js's CustomError call sites
// directly (new CustomError(409, constants.ERROR_MESSAGE.ERR_SUB_EXIST,
// "API has subscriptions.") — the human sentence is the third argument,
// nested under errors[0].message, which this deliberately never reads).
type portalErrorEnvelope struct {
	Message string `json:"message"`
}

// knownPortalConflictReasons maps a portal error CODE (never its raw
// message/errors[] text) to a short, pre-approved phrase this service owns
// and controls. error-handling.md forbids exposing raw downstream error
// text to the client — and api-portal's own code shows that's not
// paranoia: its duplicate-key conflict path deliberately keeps its message
// generic for the same reason ("raw driver messages can echo internal
// constraint/table names"). This allowlist preserves that guarantee: only
// codes individually verified against apiMetadataService.js's CustomError
// call sites get a specific reason; anything else falls back to
// defaultPortalConflictReason. Extend this map only after confirming a new
// code the same way, never by forwarding errors[].message directly.
var knownPortalConflictReasons = map[string]string{
	"ERR_SUB_EXIST": "active subscriptions are removed",
	"ERR_KEY_EXIST": "active API keys are removed",
}

// defaultPortalConflictReason is also this package's original, fixed wording
// for APIPublicationPortalConflict, kept as the fallback for any portal
// response that isn't one of the known codes above.
const defaultPortalConflictReason = "the conflict is resolved"

// portalConflictReason inspects a 4xx response body for one of the known
// portal error codes above, returning a curated, safe reason phrase — never
// the portal's own raw error text — or the generic fallback if the body is
// unparseable or names an unrecognized code.
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

// Publish implements PortalPublisher.
func (p *HTTPPortalPublisher) Publish(ctx context.Context, portal *model.APIPortal, apiHandle string, pub *model.Publication, definition *model.PublicationContent) error {
	base := strings.TrimRight(portal.URL, "/")
	escapedHandle := url.PathEscape(apiHandle)

	exists, err := p.checkExists(ctx, portal, base, escapedHandle)
	if err != nil {
		return err
	}

	method, path := http.MethodPost, base+"/apis"
	if exists {
		method, path = http.MethodPut, base+"/apis/"+escapedHandle
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
	path := strings.TrimRight(portal.URL, "/") + "/apis/" + url.PathEscape(apiHandle)
	return p.pushMetadata(ctx, portal, http.MethodPut, path, apiHandle, model.PublicationStatusDeprecated, live, nil)
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
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		// Any 4xx means the portal understood and rejected the request as-is —
		// a conflicting handle/display name (409), an unresolvable reference
		// like a subscription plan the portal doesn't recognize (404), or any
		// other validation failure. All of these are "will keep rejecting",
		// not "unreachable" — retrying without changing the listing or the portal can't help.
		body, _ := io.ReadAll(io.LimitReader(resp.Body, portalErrorBodyMaxBytes))
		return &PortalConflictError{
			Message: fmt.Sprintf("the API Portal rejected this listing (status %d)", resp.StatusCode),
			Reason:  portalConflictReason(body),
		}
	default:
		return fmt.Errorf("portal metadata push failed: unexpected status %d", resp.StatusCode)
	}
}

// Unpublish implements PortalPublisher: DELETE /apis/{handle} on the portal
// — verified against the portal's own OpenAPI spec
// (portals/api-portal/docs/api-portal-openapi-spec-v0.9.yaml) and
// apiMetadataService.js's deleteAPIMetadata. A 200 means removed; a 404 is
// treated as already-removed (success), which is what makes a retry after an
// already-successful removal converge rather than error. Any other 4xx —
// most commonly 409, when the portal still has active subscriptions/API
// keys attached to the listing (a force-delete-with-listing capability
// isn't implemented on the portal yet) — surfaces as the same
// PortalConflictError Publish uses for a rejection the portal will keep
// making.
func (p *HTTPPortalPublisher) Unpublish(ctx context.Context, portal *model.APIPortal, apiHandle string) error {
	base := strings.TrimRight(portal.URL, "/")
	escapedHandle := url.PathEscape(apiHandle)

	authHeader, err := p.authHeader(ctx, portal)
	if err != nil {
		return err
	}

	reqCtx, cancel := context.WithTimeout(ctx, p.client.TotalTimeout())
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodDelete, base+"/apis/"+escapedHandle, nil)
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
	case resp.StatusCode == http.StatusNotFound:
		return nil
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case isPortalAuthFailure(resp.StatusCode):
		return portalAuthError(resp.StatusCode)
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		// Same 4xx-is-non-retryable reasoning as Publish: a 409 (active
		// subscriptions/API keys) is the documented case, but any other 4xx
		// the portal returns here means it understood and rejected the
		// removal, not that it's unreachable — won't clear on its own retry.
		body, _ := io.ReadAll(io.LimitReader(resp.Body, portalErrorBodyMaxBytes))
		return &PortalConflictError{
			Message: fmt.Sprintf("the API Portal rejected removal of this listing (status %d)", resp.StatusCode),
			Reason:  portalConflictReason(body),
		}
	default:
		return fmt.Errorf("portal unpublish failed: unexpected status %d", resp.StatusCode)
	}
}

// checkExists issues GET /apis/{handle} on the portal. 200 means it exists
// (update); 404 means it doesn't (create).
func (p *HTTPPortalPublisher) checkExists(ctx context.Context, portal *model.APIPortal, base, escapedHandle string) (bool, error) {
	authHeader, err := p.authHeader(ctx, portal)
	if err != nil {
		return false, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, p.client.TotalTimeout())
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, base+"/apis/"+escapedHandle, nil)
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
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		// Same 4xx-is-non-retryable reasoning as Publish/Unpublish.
		body, _ := io.ReadAll(io.LimitReader(resp.Body, portalErrorBodyMaxBytes))
		return false, &PortalConflictError{
			Message: fmt.Sprintf("the API Portal rejected the existence check (status %d)", resp.StatusCode),
			Reason:  portalConflictReason(body),
		}
	default:
		return false, fmt.Errorf("portal existence check failed: unexpected status %d", resp.StatusCode)
	}
}

// portalMetadataEnvelope is the "metadata" part of the portal push — a
// k8s-style envelope. apiVersion/kind are sent but unread by the portal's
// parser (verified against the portal's own apiMetadataService.js); kept for
// shape-consistency with the portal's own sample files.
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
	// omitempty: an explicit empty list here isn't "no labels" to the portal —
	// its apiMetadataService.js only falls back to the "default" label when
	// the "labels" key is absent from the request entirely (`if
	// (apiMetadata.labels) ... else labelDao.createApiMapping(..., ['default'],
	// ...)`); sending `labels: []` is treated as an explicit "zero labels"
	// override and the API silently never gets attached to any view.
	Labels              []string                   `yaml:"labels,omitempty"`
	ReferenceID         string                     `yaml:"referenceId"`
	Endpoints           portalMetadataEndpoints    `yaml:"endpoints"`
	BusinessInformation portalMetadataBusinessInfo `yaml:"businessInformation"`
	// SubscriptionPlans is a plain string array of plan handles in this YAML
	// envelope — NOT the {id: <handle>} object-array shape. Per the portal's
	// own OpenAPI spec (api-portal-openapi-spec-v0.9.yaml, the requestBody
	// description on ApiMetadataMultipartBody): "subscriptionPlans links
	// existing org-level plans to this API by name... In YAML it is a string
	// array (["Gold", "Silver"]). In the JSON metadata field it is an object
	// array where only id is used" — the object-array shape is for a
	// different upload path (a plain JSON metadata field, not this YAML
	// file part) and is never valid here. Sending {id: ...} objects in this
	// field silently breaks: the portal's YAML parser JS-stringifies each
	// entry (`String({id:"Gold"})` -> "[object Object]") before looking it
	// up, so every plan reference 404s as "Subscription plan not found"
	// regardless of whether the handle is otherwise correct.
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
