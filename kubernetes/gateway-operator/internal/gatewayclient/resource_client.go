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
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// PayloadContentTypeYAML is the default payload content type for envelope-
// shaped resources whose REST endpoints accept application/yaml (rest-apis,
// llm-providers, llm-proxies, llm-provider-templates, mcps, secrets).
const PayloadContentTypeYAML = "application/yaml"

// PayloadContentTypeJSON is used for resources whose REST endpoints accept
// application/json (subscriptions, subscription-plans, certificates).
const PayloadContentTypeJSON = "application/json"

// ResourceExists performs GET <gatewayEndpoint><resourcePath>/<handle> and
// reports whether the resource is present. Used to choose POST vs PUT for
// idempotent deploys.
func ResourceExists(ctx context.Context, gatewayEndpoint, resourcePath, handle string, auth AuthHeaderFunc) (bool, error) {
	endpoint := fmt.Sprintf("%s%s/%s", gatewayEndpoint, resourcePath, url.PathEscape(handle))
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
		return false, &RetryableError{Err: fmt.Errorf("send existence check: %w", err)}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	switch {
	case resp.StatusCode == http.StatusOK:
		return true, nil
	case resp.StatusCode == http.StatusNotFound:
		return false, nil
	case IsRetryableStatusCode(resp.StatusCode):
		return false, &RetryableError{
			Err:        fmt.Errorf("existence check returned status %d: %s", resp.StatusCode, string(body)),
			StatusCode: resp.StatusCode,
		}
	default:
		return false, &NonRetryableError{
			Err:        fmt.Errorf("existence check returned status %d: %s", resp.StatusCode, string(body)),
			StatusCode: resp.StatusCode,
		}
	}
}

// DeployResource POSTs (when exists is false) or PUTs (when exists is true)
// the body to the configured resource path. Used by all envelope-shaped
// management-API resources.
//
// The contentType selects which Content-Type header is sent (typically
// PayloadContentTypeYAML for envelope kinds and PayloadContentTypeJSON for
// resources whose payloads are JSON).
func DeployResource(ctx context.Context, gatewayEndpoint, resourcePath, handle string, body []byte, exists bool, contentType string, auth AuthHeaderFunc) error {
	var (
		endpoint string
		method   string
	)
	if exists {
		endpoint = fmt.Sprintf("%s%s/%s", gatewayEndpoint, resourcePath, url.PathEscape(handle))
		method = http.MethodPut
	} else {
		endpoint = gatewayEndpoint + resourcePath
		method = http.MethodPost
	}
	return sendBody(ctx, method, endpoint, body, contentType, auth)
}

// CreateResource POSTs the body to <gatewayEndpoint><resourcePath> and
// returns the raw response body so callers can extract gateway-issued ids
// (e.g. for Subscription/SubscriptionPlan/Certificate). Status-code
// classification matches DeployResource.
func CreateResource(ctx context.Context, gatewayEndpoint, resourcePath string, body []byte, contentType string, auth AuthHeaderFunc) ([]byte, error) {
	endpoint := gatewayEndpoint + resourcePath
	return sendBodyForResponse(ctx, http.MethodPost, endpoint, body, contentType, auth)
}

// UpdateResource PUTs the body to <gatewayEndpoint><resourcePath>/<id> for
// UUID-keyed resources whose update semantics differ from envelope kinds.
func UpdateResource(ctx context.Context, gatewayEndpoint, resourcePath, id string, body []byte, contentType string, auth AuthHeaderFunc) ([]byte, error) {
	endpoint := fmt.Sprintf("%s%s/%s", gatewayEndpoint, resourcePath, url.PathEscape(id))
	return sendBodyForResponse(ctx, http.MethodPut, endpoint, body, contentType, auth)
}

// DeleteResource DELETEs <gatewayEndpoint><resourcePath>/<handle>. The
// status-code mapping mirrors DeleteRestAPI: 404 is treated as success so
// finalizer removal is idempotent.
func DeleteResource(ctx context.Context, gatewayEndpoint, resourcePath, handle string, auth AuthHeaderFunc) error {
	endpoint := fmt.Sprintf("%s%s/%s", gatewayEndpoint, resourcePath, url.PathEscape(handle))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NonRetryableError{Err: fmt.Errorf("create HTTP request: %w", err)}
	}
	if auth != nil {
		if err := auth(ctx, req); err != nil {
			return err
		}
	}

	client := &http.Client{Timeout: defaultProbeTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return &RetryableError{Err: fmt.Errorf("send delete request: %w", err)}
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent, http.StatusAccepted, http.StatusNotFound:
		return nil
	default:
		body, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("gateway returned status %d: %s", resp.StatusCode, string(body))
		if IsRetryableStatusCode(resp.StatusCode) {
			return &RetryableError{Err: err, StatusCode: resp.StatusCode}
		}
		return &NonRetryableError{Err: err, StatusCode: resp.StatusCode}
	}
}

func sendBody(ctx context.Context, method, endpoint string, body []byte, contentType string, auth AuthHeaderFunc) error {
	if _, err := sendBodyForResponse(ctx, method, endpoint, body, contentType, auth); err != nil {
		return err
	}
	return nil
}

func sendBodyForResponse(ctx context.Context, method, endpoint string, body []byte, contentType string, auth AuthHeaderFunc) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, &RetryableError{Err: fmt.Errorf("create HTTP request: %w", err)}
	}
	if contentType == "" {
		contentType = PayloadContentTypeYAML
	}
	req.Header.Set("Content-Type", contentType)
	if auth != nil {
		if err := auth(ctx, req); err != nil {
			return nil, err
		}
	}

	client := &http.Client{Timeout: defaultDeployTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, &RetryableError{Err: fmt.Errorf("send request to gateway: %w", err)}
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	switch {
	case resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated:
		return respBody, nil
	case IsRetryableStatusCode(resp.StatusCode):
		return nil, &RetryableError{
			Err:        fmt.Errorf("gateway returned status %d: %s", resp.StatusCode, string(respBody)),
			StatusCode: resp.StatusCode,
		}
	default:
		return nil, &NonRetryableError{
			Err:        fmt.Errorf("gateway returned status %d: %s", resp.StatusCode, string(respBody)),
			StatusCode: resp.StatusCode,
		}
	}
}
