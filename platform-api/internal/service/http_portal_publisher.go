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
	"net/url"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/client"
	"github.com/wso2/api-platform/platform-api/internal/model"

	"gopkg.in/yaml.v3"
)

// HTTPPortalPublisher is the real PortalPublisher implementation for
// REST_Design.md §8 steps 1-2: an existence check (GET /apis/{handle})
// decides whether the metadata+definition push (step 2) uses create (POST
// /apis) or update (PUT /apis/{handle}), both addressed by the API's own
// handle — no portal-returned reference ID is stored locally.
//
// Step 3 (the content ZIP — thumbnail, landing page, documents) is NOT
// implemented here yet: its exact multipart/ZIP shape wasn't verified
// against API Portal's real asset-ingestion code in the session that built
// this, and this feature has no way to fetch a document's actual content in
// the first place (api_documents' content belongs to another team's
// not-yet-built feature — see REST_Design.md's "Not in scope"). Add it once
// both are confirmed rather than guess at the shape.
type HTTPPortalPublisher struct {
	client    *client.RetryableHTTPClient
	sharedKey string
}

// NewHTTPPortalPublisher returns a PortalPublisher that pushes to the real
// API Portal over HTTP, authenticating with sharedKey (the raw shared-key
// value — see config.PublicationPortalSharedKeyPath) via the shared-key S2S
// auth scheme.
func NewHTTPPortalPublisher(sharedKey string, retryClient *client.RetryableHTTPClient) PortalPublisher {
	return &HTTPPortalPublisher{client: retryClient, sharedKey: sharedKey}
}

func (p *HTTPPortalPublisher) authHeader() string {
	return "sharedkey " + p.sharedKey
}

// Publish implements PortalPublisher.
func (p *HTTPPortalPublisher) Publish(ctx context.Context, portal *model.PublicationAPIPortal, apiHandle string, pub *model.Publication, definition *model.PublicationContent) error {
	base := strings.TrimRight(portal.URL, "/")
	escapedHandle := url.PathEscape(apiHandle)

	exists, err := p.checkExists(ctx, base, escapedHandle)
	if err != nil {
		return err
	}

	method, path := http.MethodPost, base+"/apis"
	if exists {
		method, path = http.MethodPut, base+"/apis/"+escapedHandle
	}

	body, contentType, err := buildPortalMetadataMultipart(apiHandle, pub, definition)
	if err != nil {
		return err
	}

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, method, path, body)
	if err != nil {
		return fmt.Errorf("failed to build portal metadata request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", p.authHeader())

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("portal metadata push failed: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusConflict:
		return &PortalConflictError{Message: "the API Portal rejected this listing (conflicting handle or display name)"}
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	default:
		return fmt.Errorf("portal metadata push failed: unexpected status %d", resp.StatusCode)
	}
}

// checkExists is REST_Design.md §8 step 1: GET /apis/{handle} on the portal.
// 200 means it exists (update); 404 means it doesn't (create).
func (p *HTTPPortalPublisher) checkExists(ctx context.Context, base, escapedHandle string) (bool, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, base+"/apis/"+escapedHandle, nil)
	if err != nil {
		return false, fmt.Errorf("failed to build portal existence-check request: %w", err)
	}
	req.Header.Set("Authorization", p.authHeader())

	resp, err := p.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("portal existence check failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("portal existence check failed: unexpected status %d", resp.StatusCode)
	}
}

// portalMetadataEnvelope is REST_Design.md §8 step 2's "metadata" part — a
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
	Type                string                     `yaml:"type"`
	DisplayName         string                     `yaml:"displayName"`
	Version             string                     `yaml:"version"`
	Description         string                     `yaml:"description,omitempty"`
	Status              string                     `yaml:"status"`
	AgentVisibility     string                     `yaml:"agentVisibility"`
	Tags                []string                   `yaml:"tags"`
	Labels              []string                   `yaml:"labels"`
	ReferenceID         string                     `yaml:"referenceId"`
	Endpoints           portalMetadataEndpoints    `yaml:"endpoints"`
	BusinessInformation portalMetadataBusinessInfo `yaml:"businessInformation"`
	SubscriptionPlans   []portalMetadataPlanRef    `yaml:"subscriptionPlans"`
}

// portalMetadataEndpoints always sends both keys, even when empty — the
// portal requires the endpoints object to be present (REST_Design.md §8).
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

type portalMetadataPlanRef struct {
	ID string `yaml:"id"`
}

// buildPortalMetadataMultipart builds the two-part multipart body
// REST_Design.md §8 step 2 sends: a "metadata" part (YAML envelope) and a
// "definition" part (raw contract bytes, empty if the draft never stored
// one — the portal itself decides whether that's acceptable).
func buildPortalMetadataMultipart(apiHandle string, pub *model.Publication, definition *model.PublicationContent) (*bytes.Buffer, string, error) {
	envelope := portalMetadataEnvelope{
		APIVersion: "api-portal.api-platform.wso2.com/v1",
		Kind:       "RestApi",
		Metadata:   portalMetadataName{Name: apiHandle},
		Spec: portalMetadataSpecBody{
			Type:            "REST",
			DisplayName:     pub.DisplayName,
			Version:         pub.Version,
			Description:     pub.Description,
			Status:          "PUBLISHED",
			AgentVisibility: pub.AgentVisibility,
			Tags:            nonNilStringsForYAML(pub.Tags),
			Labels:          nonNilStringsForYAML(pub.Labels),
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
			SubscriptionPlans: portalPlanRefs(pub.SubscriptionPlanIds),
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

	defFileName, defContent := "definition.json", []byte{}
	if definition != nil {
		defContent = definition.Content
		if definition.FileName != "" {
			defFileName = definition.FileName
		}
	}
	defPart, err := mw.CreateFormFile("definition", defFileName)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create definition part: %w", err)
	}
	if _, err := defPart.Write(defContent); err != nil {
		return nil, "", fmt.Errorf("failed to write definition part: %w", err)
	}

	if err := mw.Close(); err != nil {
		return nil, "", fmt.Errorf("failed to finalize multipart body: %w", err)
	}
	return &buf, mw.FormDataContentType(), nil
}

// nonNilStringsForYAML returns s, or a non-nil empty slice when s is nil, so
// the field marshals as an empty YAML list rather than "null".
func nonNilStringsForYAML(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// portalPlanRefs converts subscription plan handles into the portal's
// {id: <handle>} shape.
func portalPlanRefs(handles []string) []portalMetadataPlanRef {
	refs := make([]portalMetadataPlanRef, 0, len(handles))
	for _, h := range handles {
		refs = append(refs, portalMetadataPlanRef{ID: h})
	}
	return refs
}
