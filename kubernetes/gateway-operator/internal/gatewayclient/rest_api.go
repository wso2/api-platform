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
	"time"
)

const (
	defaultDeployTimeout = 30 * time.Second
	defaultProbeTimeout  = 10 * time.Second
)

// AuthHeaderFunc sets auth headers on outbound requests (e.g. Basic auth).
type AuthHeaderFunc func(ctx context.Context, req *http.Request) error

// DeployRestAPI POSTs or PUTs YAML to the gateway-controller /rest-apis API.
func DeployRestAPI(ctx context.Context, gatewayEndpoint string, handle string, apiYAML []byte, exists bool, auth AuthHeaderFunc) error {
	var endpoint string
	var method string
	if exists {
		endpoint = fmt.Sprintf("%s/rest-apis/%s", gatewayEndpoint, url.PathEscape(handle))
		method = http.MethodPut
	} else {
		endpoint = gatewayEndpoint + "/rest-apis"
		method = http.MethodPost
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(apiYAML))
	if err != nil {
		return &RetryableError{Err: fmt.Errorf("create HTTP request: %w", err)}
	}
	req.Header.Set("Content-Type", "application/yaml")
	if auth != nil {
		if err := auth(ctx, req); err != nil {
			return err
		}
	}

	client := &http.Client{Timeout: defaultDeployTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return &RetryableError{Err: fmt.Errorf("send request to gateway: %w", err)}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	switch {
	case resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated:
		return nil
	case IsRetryableStatusCode(resp.StatusCode):
		return &RetryableError{
			Err:        fmt.Errorf("gateway returned status %d: %s", resp.StatusCode, string(body)),
			StatusCode: resp.StatusCode,
		}
	default:
		return &NonRetryableError{
			Err:        fmt.Errorf("gateway returned status %d: %s", resp.StatusCode, string(body)),
			StatusCode: resp.StatusCode,
		}
	}
}

// RestAPIExists performs GET /rest-apis/{handle}.
func RestAPIExists(ctx context.Context, gatewayEndpoint, handle string, auth AuthHeaderFunc) (bool, error) {
	endpoint := fmt.Sprintf("%s/rest-apis/%s", gatewayEndpoint, url.PathEscape(handle))
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

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	case http.StatusServiceUnavailable, http.StatusTooManyRequests, http.StatusBadGateway, http.StatusGatewayTimeout:
		body, _ := io.ReadAll(resp.Body)
		return false, &RetryableError{Err: fmt.Errorf("existence check returned status %d: %s", resp.StatusCode, string(body)), StatusCode: resp.StatusCode}
	default:
		body, _ := io.ReadAll(resp.Body)
		return false, &NonRetryableError{Err: fmt.Errorf("existence check returned status %d: %s", resp.StatusCode, string(body)), StatusCode: resp.StatusCode}
	}
}

// DeleteRestAPI DELETEs /rest-apis/{handle}.
func DeleteRestAPI(ctx context.Context, gatewayEndpoint, handle string, auth AuthHeaderFunc) error {
	endpoint := fmt.Sprintf("%s/rest-apis/%s", gatewayEndpoint, url.PathEscape(handle))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create HTTP request: %w", err)
	}
	if auth != nil {
		if err := auth(ctx, req); err != nil {
			return err
		}
	}

	client := &http.Client{Timeout: defaultProbeTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send delete request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent, http.StatusAccepted, http.StatusNotFound:
		return nil
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gateway returned status %d: %s", resp.StatusCode, string(body))
	}
}
