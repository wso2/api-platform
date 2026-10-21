/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cloudconsole

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	corecatalog "github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
)

// internalDetailMarkers are fragments that only an unsanitised error body carries: a Go panic,
// a stack frame or the name of a hop behind the edge.
var internalDetailMarkers = []string{
	"panic", "goroutine", "runtime error", "stack trace", "traceback", ".go:", "upstream",
}

// cloudMethods are the HTTP methods a scenario may send to the cloud API.
var cloudMethods = map[string]bool{
	http.MethodGet: true, http.MethodPost: true, http.MethodPut: true,
	http.MethodPatch: true, http.MethodDelete: true,
}

// apiURL resolves a path below the cloud API base URL.
func (s *Steps) apiURL(path string) (string, error) {
	if !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("cloud API path %q must start with /", path)
	}
	base, err := s.topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	if err != nil {
		return "", err
	}
	return cloudAPIURL(base, path), nil
}

// sendCloudRequest sends a request with exactly the headers given, so a scenario controls
// whether and how the request authenticates, and returns the response without publishing it.
func (s *Steps) sendCloudRequest(ctx context.Context, method, path string, headers map[string]string) (*httpx.Response, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	if !cloudMethods[method] {
		return nil, fmt.Errorf("unsupported HTTP method %q", method)
	}
	target, err := s.apiURL(path)
	if err != nil {
		return nil, err
	}
	return s.client.Do(ctx, httpx.Request{Method: method, URL: target, Headers: headers}, 0, 0)
}

// sendAndPublish sends a request and publishes its response for the assertion steps.
func (s *Steps) sendAndPublish(ctx context.Context, method, path string, headers map[string]string) (*httpx.Response, error) {
	resp, err := s.sendCloudRequest(ctx, method, path, headers)
	if err != nil {
		return nil, err
	}
	if s.funnel != nil {
		if err := s.funnel.Publish(ctx, resp); err != nil {
			return nil, err
		}
	}
	return resp, nil
}

func bearerHeaders(ctx context.Context) (map[string]string, error) {
	token, err := bearerToken(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]string{"Authorization": "Bearer " + token}, nil
}

func (s *Steps) sendAuthenticatedRequest(ctx context.Context, method, path string) error {
	headers, err := bearerHeaders(ctx)
	if err != nil {
		return err
	}
	_, err = s.sendAndPublish(ctx, method, path, headers)
	return err
}

func (s *Steps) sendWithoutAuthorization(ctx context.Context, method, path string) error {
	_, err := s.sendAndPublish(ctx, method, path, nil)
	return err
}

func (s *Steps) sendWithAuthorizationHeader(ctx context.Context, method, path, value string) error {
	_, err := s.sendAndPublish(ctx, method, path, map[string]string{"Authorization": value})
	return err
}

// sendWithForgedToken sends a request bearing a token the platform must refuse.
func (s *Steps) sendWithForgedToken(ctx context.Context, method, path, kind string) error {
	var header, claims map[string]any
	if kind == forgeExpired {
		var err error
		if header, claims, err = tokenParts(ctx); err != nil {
			return fmt.Errorf("reading the console token the forgery is based on: %w", err)
		}
	}
	forged, err := forgeToken(kind, header, claims, time.Now())
	if err != nil {
		return err
	}
	_, err = s.sendAndPublish(ctx, method, path, map[string]string{"Authorization": "Bearer " + forged})
	return err
}

// assertForeignOrganisationHeaderIgnored sends the same authenticated request with and without
// a header that names an organisation nobody asked for, and requires the same answer: the
// caller's organisation comes from the token alone.
func (s *Steps) assertForeignOrganisationHeaderIgnored(ctx context.Context, method, path, header string) error {
	headers, err := bearerHeaders(ctx)
	if err != nil {
		return err
	}
	plain, err := s.sendCloudRequest(ctx, method, path, headers)
	if err != nil {
		return err
	}
	foreign, err := randomHex(16)
	if err != nil {
		return err
	}
	headers[header] = "forged-" + foreign
	withHeader, err := s.sendCloudRequest(ctx, method, path, headers)
	if err != nil {
		return err
	}
	if s.funnel != nil {
		if err := s.funnel.Publish(ctx, withHeader); err != nil {
			return err
		}
	}
	if plain.StatusCode != http.StatusOK {
		return fmt.Errorf("the request without the %s header was not accepted: %s", header, plain.Describe())
	}
	if withHeader.StatusCode != plain.StatusCode {
		return fmt.Errorf("the %s header changed the status from %d to %d", header, plain.StatusCode, withHeader.StatusCode)
	}
	if !sameJSON(plain.Body, withHeader.Body) {
		return fmt.Errorf("the %s header changed the response body: %s", header, withHeader.Describe())
	}
	return nil
}

// sameJSON compares two bodies as JSON documents, falling back to the raw bytes.
func sameJSON(left, right []byte) bool {
	var l, r any
	if json.Unmarshal(left, &l) != nil || json.Unmarshal(right, &r) != nil {
		return bytes.Equal(left, right)
	}
	return reflect.DeepEqual(l, r)
}

// assertNoInternalDetail checks that an error body names no panic, stack frame or upstream hop.
func (s *Steps) assertNoInternalDetail(ctx context.Context) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	body := strings.ToLower(string(response.Body))
	for _, marker := range internalDetailMarkers {
		if strings.Contains(body, marker) {
			return fmt.Errorf("the error response exposes internal detail (%q): %s", marker, response.Describe())
		}
	}
	return nil
}

func (s *Steps) assertCloudResponseClientError(ctx context.Context) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	if response.StatusCode < 400 || response.StatusCode > 499 {
		return fmt.Errorf("expected a client error status, got %s", response.Describe())
	}
	return nil
}

func (s *Steps) assertCloudResponseContains(ctx context.Context, want string) error {
	want, err := expandCloudValue(ctx, want)
	if err != nil {
		return err
	}
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	if !strings.Contains(string(response.Body), want) {
		return fmt.Errorf("expected the response body to contain %q, got %s", want, response.Describe())
	}
	return nil
}

func (s *Steps) assertCloudErrorCode(ctx context.Context, want string) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	code, err := stringField(response, "code")
	if err != nil {
		return fmt.Errorf("reading the error code: %w (%s)", err, response.Describe())
	}
	if code != want {
		return fmt.Errorf("expected error code %q, got %q (%s)", want, code, response.Describe())
	}
	return nil
}
