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
	"log"
	"net/http"
	"net/url"
	"strings"
)

// buildURL joins base URL with path segments ensuring single slashes.
func (c *DevPortalClient) buildURL(parts ...string) string {
	base := strings.TrimRight(c.cfg.BaseURL, "/")
	// Split each part by "/" and collect non-empty segments
	segments := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.Trim(p, "/")
		if trimmed == "" {
			continue
		}
		// Split by "/" to handle parts like "devportal/organizations"
		subParts := strings.Split(trimmed, "/")
		for _, subPart := range subParts {
			if subPart != "" {
				segments = append(segments, url.PathEscape(subPart))
			}
		}
	}
	if len(segments) == 0 {
		return base
	}
	return base + "/" + strings.Join(segments, "/")
}

// newJSONRequest marshals v to JSON (if non-nil) and returns an *http.Request with Content-Type set.
func (c *DevPortalClient) newJSONRequest(method, url string, v interface{}) (*http.Request, error) {
	var body io.Reader
	if v != nil {
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	if v != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// doAndDecode executes the request, checks the status against expectedCodes,
// and decodes the response JSON into out. If out is nil, the body is discarded.
func (c *DevPortalClient) doAndDecode(req *http.Request, expectedCodes []int, out interface{}) error {
	// Execute the request first. Only use resp after confirming err==nil.
	resp, err := c.do(req)
	if err != nil {
		// do returned an error; log and return
		log.Printf("doAndDecode: request failed: %v", err)
		return err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("doAndDecode: reading response body failed: %v", err)
		return err
	}

	log.Printf("doAndDecode: status=%d", resp.StatusCode)

	ok := false
	for _, code := range expectedCodes {
		if resp.StatusCode == code {
			ok = true
			break
		}
	}
	if !ok {
		log.Printf("doAndDecode: unexpected status=%d body=%s", resp.StatusCode, string(b))
		return NewDevPortalError(resp.StatusCode, fmt.Sprintf("unexpected status %d: %s", resp.StatusCode, string(b)), resp.StatusCode >= 500, nil)
	}

	if out == nil {
		// caller doesn't want a response body
		return nil
	}

	// Decode from the buffered bytes
	decoder := json.NewDecoder(bytes.NewReader(b))
	if err := decoder.Decode(out); err != nil {
		log.Printf("doAndDecode: decode failed: %v; body=%s", err, string(b))
		return err
	}
	return nil
}

// doNoContent executes the request and expects one of expectedCodes; otherwise returns DevPortalError.
func (c *DevPortalClient) doNoContent(req *http.Request, expectedCodes []int) error {
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	for _, code := range expectedCodes {
		if resp.StatusCode == code {
			io.Copy(io.Discard, resp.Body)
			return nil
		}
	}
	b, _ := io.ReadAll(resp.Body)
	return NewDevPortalError(resp.StatusCode, fmt.Sprintf("unexpected status %d: %s", resp.StatusCode, string(b)), resp.StatusCode >= 500, nil)
}
