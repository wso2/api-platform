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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// APIKeyExpiry mirrors APIKeyCreationRequest.expiresIn.
type APIKeyExpiry struct {
	Duration int64  `json:"duration"`
	Unit     string `json:"unit"`
}

// APIKeyCreatePayload mirrors the management-API APIKeyCreationRequest body
// expected at POST/PUT /<parent>/{parentName}/api-keys[/{name}].
type APIKeyCreatePayload struct {
	Name          string        `json:"name,omitempty"`
	DisplayName   string        `json:"displayName,omitempty"`
	ApiKey        string        `json:"apiKey,omitempty"`
	MaskedApiKey  string        `json:"maskedApiKey,omitempty"`
	ExpiresIn     *APIKeyExpiry `json:"expiresIn,omitempty"`
	ExpiresAt     *time.Time    `json:"expiresAt,omitempty"`
	Issuer        string        `json:"issuer,omitempty"`
	ExternalRefId string        `json:"externalRefId,omitempty"`
}

// BuildAPIKeysPath returns the management-API URL fragment for an API key
// resource nested under its parent. When keyHandle is empty the returned
// path targets the collection (POST), otherwise it targets a single key.
func BuildAPIKeysPath(parentKind, parentHandle, keyHandle string) (string, error) {
	parentBase, err := apiKeyParentPath(parentKind)
	if err != nil {
		return "", err
	}
	if parentHandle == "" {
		return "", fmt.Errorf("apikey parent handle is required")
	}
	if keyHandle == "" {
		return fmt.Sprintf("%s/%s/api-keys", parentBase, url.PathEscape(parentHandle)), nil
	}
	return fmt.Sprintf("%s/%s/api-keys/%s", parentBase, url.PathEscape(parentHandle), url.PathEscape(keyHandle)), nil
}

// apiKeyResourcePathForExists returns the path used by ResourceExists/
// DeployResource where the trailing handle is appended automatically.
func apiKeyResourcePathForExists(parentKind, parentHandle string) (string, error) {
	parentBase, err := apiKeyParentPath(parentKind)
	if err != nil {
		return "", err
	}
	if parentHandle == "" {
		return "", fmt.Errorf("apikey parent handle is required")
	}
	return fmt.Sprintf("%s/%s/api-keys", parentBase, url.PathEscape(parentHandle)), nil
}

// APIKeyExists reports whether the named API key exists under the given
// parent.
//
// The gateway management API exposes GET /<parent>/{id}/api-keys as a list
// operation only; there is no GET for a single key by name. Probing
// .../api-keys/{name} therefore always returned 404 and forced POST on every
// deploy, which duplicated keys and broke enforcement. We detect existence by
// listing keys and matching the key name.
func APIKeyExists(ctx context.Context, gatewayEndpoint, parentKind, parentHandle, keyHandle string, auth AuthHeaderFunc) (bool, error) {
	rp, err := apiKeyResourcePathForExists(parentKind, parentHandle)
	if err != nil {
		return false, err
	}
	endpoint := gatewayEndpoint + rp
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, &RetryableError{Err: fmt.Errorf("create HTTP request: %w", err)}
	}
	if auth != nil {
		if err := auth(ctx, req); err != nil {
			return false, err
		}
	}

	client := &http.Client{Timeout: defaultProbeTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return false, &RetryableError{Err: fmt.Errorf("list api keys for existence check: %w", err)}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusOK:
		var parsed struct {
			ApiKeys *[]struct {
				Name *string `json:"name"`
			} `json:"apiKeys"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			return false, &NonRetryableError{
				Err:        fmt.Errorf("decode api key list: %w", err),
				StatusCode: resp.StatusCode,
			}
		}
		if parsed.ApiKeys == nil {
			return false, nil
		}
		for _, k := range *parsed.ApiKeys {
			if k.Name != nil && *k.Name == keyHandle {
				return true, nil
			}
		}
		return false, nil
	case http.StatusNotFound:
		return false, nil
	default:
		err := fmt.Errorf("list api keys returned status %d: %s", resp.StatusCode, string(body))
		if IsRetryableStatusCode(resp.StatusCode) {
			return false, &RetryableError{Err: err, StatusCode: resp.StatusCode}
		}
		return false, &NonRetryableError{Err: err, StatusCode: resp.StatusCode}
	}
}

// DeployAPIKey POSTs (when exists is false) or PUTs (when exists is true)
// the JSON payload for an API key under the configured parent.
func DeployAPIKey(ctx context.Context, gatewayEndpoint, parentKind, parentHandle, keyHandle string, payload APIKeyCreatePayload, exists bool, auth AuthHeaderFunc) error {
	rp, err := apiKeyResourcePathForExists(parentKind, parentHandle)
	if err != nil {
		return err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal apikey payload: %w", err)
	}
	if err := DeployResource(ctx, gatewayEndpoint, rp, keyHandle, body, exists, PayloadContentTypeJSON, auth); err != nil {
		// Some deployments can race or miss existence detection and hit a create
		// conflict even though the key already exists. Fall back to PUT update so
		// reconciliation remains idempotent for RestApi/LlmProvider/LlmProxy keys.
		if !exists {
			var nr *NonRetryableError
			if errors.As(err, &nr) && nr.StatusCode == 409 {
				return DeployResource(ctx, gatewayEndpoint, rp, keyHandle, body, true, PayloadContentTypeJSON, auth)
			}
		}
		return err
	}
	return nil
}

// DeleteAPIKey removes the named API key under the given parent.
func DeleteAPIKey(ctx context.Context, gatewayEndpoint, parentKind, parentHandle, keyHandle string, auth AuthHeaderFunc) error {
	rp, err := apiKeyResourcePathForExists(parentKind, parentHandle)
	if err != nil {
		return err
	}
	return DeleteResource(ctx, gatewayEndpoint, rp, keyHandle, auth)
}
