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

// Package idp provides a SCIM2 client for syncing org membership claims back to
// the identity provider (Thunder, WSO2 IS, Asgardeo, etc.) after org changes.
package idp

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultSCIM2Schema is the Thunder / WSO2 IS custom-user extension schema
	// under which the organization claims are registered.
	DefaultSCIM2Schema = "urn:scim:schemas:extension:custom:User"

	// DefaultOrgAttr is the SCIM2 attribute name for the user's currently
	// selected (active) organization UUID.
	DefaultOrgAttr = "organization"

	// DefaultOrgsAttr is the SCIM2 attribute name for the full space-separated
	// list of all organization UUIDs the user belongs to.
	DefaultOrgsAttr = "organizations"
)

// ClaimUpdater syncs organization membership back to the IDP via SCIM2 /Me.
// It updates two attributes in one PATCH call:
//   - organizations  — space-separated list of ALL org UUIDs (always updated)
//   - organization   — the user's default (first) org UUID (updated only when
//     the user has no prior value, i.e. their very first org creation)
type ClaimUpdater struct {
	scim2BaseURL string
	schema       string
	orgAttr      string // attribute name for the active/default org
	orgsAttr     string // attribute name for the full org list
	httpClient   *http.Client
}

// NewClaimUpdater creates a ClaimUpdater.
// Returns nil when scim2BaseURL is empty — callers treat nil as "IDP sync disabled".
// Set insecureSkipVerify=true only for local dev with self-signed certs (Thunder,
// local WSO2 IS). Keep false for cloud IDPs like Asgardeo.
func NewClaimUpdater(scim2BaseURL, schema, orgsAttr string, insecureSkipVerify bool) *ClaimUpdater {
	if scim2BaseURL == "" {
		return nil
	}
	if schema == "" {
		schema = DefaultSCIM2Schema
	}
	if orgsAttr == "" {
		orgsAttr = DefaultOrgsAttr
	}
	return &ClaimUpdater{
		scim2BaseURL: strings.TrimRight(scim2BaseURL, "/"),
		schema:       schema,
		orgAttr:      DefaultOrgAttr,
		orgsAttr:     orgsAttr,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureSkipVerify}, //nolint:gosec
			},
		},
	}
}

// scim2PatchOp is the JSON body for a SCIM2 PATCH request.
type scim2PatchOp struct {
	Schemas    []string         `json:"schemas"`
	Operations []scim2Operation `json:"Operations"`
}

type scim2Operation struct {
	Op    string                 `json:"op"`
	Value map[string]interface{} `json:"value"`
}

// GetUserOrgClaims fetches the target user's current org list from the IDP via
// SCIM2 GET /Users/{targetUserID}. Used before merging a new org so existing
// memberships are not overwritten.
func (c *ClaimUpdater) GetUserOrgClaims(adminBearerToken, targetUserID string) ([]string, error) {
	url := c.scim2BaseURL + "/scim2/Users/" + targetUserID
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build SCIM2 GET request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminBearerToken)
	req.Header.Set("Accept", "application/scim+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("SCIM2 GET /Users/%s: %w", targetUserID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("SCIM2 GET /Users/%s returned HTTP %d", targetUserID, resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode SCIM2 response: %w", err)
	}

	// Navigate schema.orgsAttr to find the space-separated org list.
	if ext, ok := body[c.schema].(map[string]interface{}); ok {
		if raw, ok := ext[c.orgsAttr].(string); ok && raw != "" {
			return strings.Fields(raw), nil
		}
	}
	return nil, nil
}

// UpdateUserOrgClaims writes the target user's full org list to the IDP using
// an admin bearer token. Unlike UpdateOrgClaims (which patches /scim2/Me for the
// caller), this patches /scim2/Users/{targetUserID} so an admin can update
// another user's org membership claim.
func (c *ClaimUpdater) UpdateUserOrgClaims(adminBearerToken, targetUserID string, orgIDs []string) error {
	if len(orgIDs) == 0 || targetUserID == "" {
		return nil
	}
	attrs := map[string]interface{}{
		c.orgsAttr: orgIDs,
		c.orgAttr:  orgIDs[0],
	}
	body := scim2PatchOp{
		Schemas: []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		Operations: []scim2Operation{
			{
				Op: "replace",
				Value: map[string]interface{}{
					c.schema: attrs,
				},
			},
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal SCIM2 patch: %w", err)
	}
	url := c.scim2BaseURL + "/scim2/Users/" + targetUserID
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build SCIM2 request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminBearerToken)
	req.Header.Set("Content-Type", "application/scim+json")
	req.Header.Set("Accept", "application/scim+json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("SCIM2 PATCH /Users/%s: %w", targetUserID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("SCIM2 PATCH /Users/%s returned HTTP %d: %s", targetUserID, resp.StatusCode, string(body))
	}
	return nil
}

// UpdateOrgClaims writes the user's full org list to the IDP via SCIM2 /Me.
// Sets organizations to the full array and organization to orgIDs[0].
// bearerToken must be the user's own access token.
func (c *ClaimUpdater) UpdateOrgClaims(bearerToken string, orgIDs []string) error {
	if len(orgIDs) == 0 {
		return nil
	}

	attrs := map[string]interface{}{
		c.orgsAttr: orgIDs,
		c.orgAttr:  orgIDs[0],
	}

	body := scim2PatchOp{
		Schemas: []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		Operations: []scim2Operation{
			{
				Op: "replace",
				Value: map[string]interface{}{
					c.schema: attrs,
				},
			},
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal SCIM2 patch: %w", err)
	}

	url := c.scim2BaseURL + "/scim2/Me"
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build SCIM2 request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Set("Content-Type", "application/scim+json")
	req.Header.Set("Accept", "application/scim+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("SCIM2 PATCH /Me: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("SCIM2 PATCH /Me returned HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
