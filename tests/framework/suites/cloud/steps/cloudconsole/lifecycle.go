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
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	corecatalog "github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

func (s *Steps) obtainToken(ctx context.Context) error {
	token, err := bearerToken(ctx)
	if err != nil {
		return fmt.Errorf("obtaining APIP cloud console token: %w", err)
	}
	if err := tcontext.Set(ctx, keyToken, token); err != nil {
		return err
	}
	return nil
}

func (s *Steps) findDefaultProject(ctx context.Context) error {
	resp, err := s.doAuthenticated(ctx, http.MethodGet, "/projects", nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	items, err := listItems(resp)
	if err != nil {
		return fmt.Errorf("decoding APIP projects: %w", err)
	}
	for _, item := range items {
		if item["id"] == "default" {
			return tcontext.Set(ctx, keyProjectID, "default")
		}
	}
	return fmt.Errorf("APIP project list does not contain the default project")
}

func (s *Steps) findActiveGateway(ctx context.Context) error {
	resp, err := s.doAuthenticated(ctx, http.MethodGet, "/gateways", nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	items, err := listItems(resp)
	if err != nil {
		return fmt.Errorf("decoding APIP gateways: %w", err)
	}
	for _, item := range items {
		active, ok := item["isActive"].(bool)
		if !ok || !active {
			continue
		}
		id, ok := item["id"].(string)
		if !ok || strings.TrimSpace(id) == "" {
			continue
		}
		endpoints, ok := item["endpoints"].([]any)
		if !ok || len(endpoints) == 0 {
			continue
		}
		endpoint, ok := firstApprovedDataPlaneEndpoint(ctx, s, endpoints)
		if !ok {
			continue
		}
		if err := tcontext.Set(ctx, keyGatewayID, id); err != nil {
			return err
		}
		return tcontext.Set(ctx, keyGatewayEndpoint, endpoint)
	}
	return fmt.Errorf("APIP gateway list does not contain an active gateway with an endpoint")
}

func (s *Steps) createAPI(ctx context.Context) error {
	projectID, err := requiredValue(ctx, keyProjectID)
	if err != nil {
		return err
	}
	if err := generateResourceAndStore(ctx, "syn-api", keyAPIID); err != nil {
		return err
	}
	apiID, err := requiredValue(ctx, keyAPIID)
	if err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindAPI, ID: apiID, Actor: resourceActor, Description: "REST API " + apiID,
	}); err != nil {
		return fmt.Errorf("registering API %q for cleanup: %w", apiID, err)
	}
	body := map[string]any{
		"id": apiID, "displayName": "Synthetic " + apiID,
		"description": "Created and deleted by the APIP cloud integration test. Safe to delete.",
		"context":     "/" + apiID + "/v1", "version": "1.0.0", "projectId": projectID,
		"upstream": map[string]any{"main": map[string]string{"url": "https://jsonplaceholder.typicode.com"}},
		"security": map[string]bool{"enabled": false},
		"operations": []any{map[string]any{
			"name": "getPost", "request": map[string]string{"method": http.MethodGet, "path": "/posts/1"},
		}},
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, "/rest-apis", body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return responseError(resp)
	}
	createdID, err := stringField(resp, "id")
	if err != nil {
		return fmt.Errorf("creating APIP REST API: %w", err)
	}
	if createdID != apiID {
		deregister(ctx, cleanup.KindAPI, apiID)
		if err := cleanup.Register(ctx, cleanup.Resource{
			Kind: cleanup.KindAPI, ID: createdID, Actor: resourceActor, Description: "REST API " + createdID,
		}); err != nil {
			return fmt.Errorf("registering API %q for cleanup: %w", createdID, err)
		}
	}
	return tcontext.Set(ctx, keyAPIID, createdID)
}

func (s *Steps) deployAPI(ctx context.Context) error {
	apiID, err := requiredValue(ctx, keyAPIID)
	if err != nil {
		return err
	}
	gatewayID, err := requiredValue(ctx, keyGatewayID)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, "/rest-apis/"+url.PathEscape(apiID)+"/deployments", map[string]string{
		"name": apiID + "-deploy", "base": "current", "gatewayId": gatewayID,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return responseError(resp)
	}
	deploymentID, err := stringField(resp, "deploymentId")
	if err != nil {
		return fmt.Errorf("deploying APIP REST API: %w", err)
	}
	return tcontext.Set(ctx, keyDeploymentID, deploymentID)
}

func (s *Steps) waitDeployment(ctx context.Context) error {
	apiID, err := requiredValue(ctx, keyAPIID)
	if err != nil {
		return err
	}
	gatewayID, err := requiredValue(ctx, keyGatewayID)
	if err != nil {
		return err
	}
	var last *httpx.Response
	last, err = retry.Until(ctx, retry.Options{Timeout: 3 * time.Minute, Interval: 5 * time.Second},
		func(ctx context.Context) (*httpx.Response, error) {
			return s.doAuthenticatedRaw(ctx, http.MethodGet,
				"/rest-apis/"+url.PathEscape(apiID)+"/deployments?gatewayId="+url.QueryEscape(gatewayID), nil)
		}, func(resp *httpx.Response) bool {
			if resp == nil || resp.StatusCode != http.StatusOK {
				return false
			}
			items, decodeErr := listItems(resp)
			return decodeErr == nil && len(items) > 0 && items[0]["status"] == "DEPLOYED"
		})
	if err != nil {
		return fmt.Errorf("waiting for APIP REST API deployment: %w", err)
	}
	if last == nil {
		return fmt.Errorf("waiting for APIP REST API deployment: no response")
	}
	return nil
}

// sendUntilStatus invokes an absolute cloud data-plane URL until it returns the exact status.
func (s *Steps) sendUntilStatus(ctx context.Context, method, target string, want int) error {
	if s.funnel == nil {
		return fmt.Errorf("sending %s request: HTTP funnel is required", strings.ToUpper(method))
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		return fmt.Errorf("sending request: HTTP method is required")
	}
	if want < 100 || want > 599 {
		return fmt.Errorf("sending %s request: invalid HTTP status %d", method, want)
	}
	target, err := expandCloudValue(ctx, target)
	if err != nil {
		return err
	}
	target, err = s.resolveDataPlaneURL(ctx, target)
	if err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{Timeout: propagationTimeout(s.topo)},
		func(ctx context.Context) (*httpx.Response, error) {
			response, sendErr := s.funnel.Send(ctx, httpx.Request{Method: method, URL: target})
			if sendErr != nil {
				return nil, retry.Transient(sendErr)
			}
			return response, nil
		},
		func(response *httpx.Response) bool {
			return response != nil && response.StatusCode == want
		},
		fmt.Sprintf("waiting for %s %s to return %d", method, target, want))
}

func (s *Steps) assertAPIResponse(ctx context.Context) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body, &body); err != nil {
		return fmt.Errorf("decoding the synthetic REST API response: %w (%s)", err, response.Describe())
	}
	if body["id"] != float64(1) {
		return fmt.Errorf("expected the synthetic REST API response to contain post 1, got %s", response.Describe())
	}
	return nil
}

var approvedCloudDataPlaneHost = regexp.MustCompile(
	`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?-wso2cloud\.gateway(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*\.wso2\.com$`,
)

func (s *Steps) resolveDataPlaneURL(ctx context.Context, target string) (string, error) {
	if s != nil && s.dataPlaneURLResolver != nil {
		return s.dataPlaneURLResolver(ctx, target)
	}
	return resolveCloudDataPlaneURL(ctx, target)
}

func firstApprovedDataPlaneEndpoint(ctx context.Context, s *Steps, endpoints []any) (string, bool) {
	for _, rawEndpoint := range endpoints {
		endpoint, ok := rawEndpoint.(string)
		if !ok || strings.TrimSpace(endpoint) == "" {
			continue
		}
		endpoint, err := s.resolveDataPlaneURL(ctx, strings.TrimSpace(endpoint))
		if err != nil {
			continue
		}
		return strings.TrimRight(endpoint, "/"), true
	}
	return "", false
}

func resolveCloudDataPlaneURL(_ context.Context, target string) (string, error) {
	parsed, err := url.Parse(target)
	if err != nil {
		return "", fmt.Errorf("invalid cloud data-plane URL %q: %w", target, err)
	}
	if parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return "", fmt.Errorf("cloud data-plane target must be an approved HTTPS URL, got %q", target)
	}
	host := strings.ToLower(parsed.Hostname())
	if ip := net.ParseIP(host); ip != nil {
		return "", fmt.Errorf("cloud data-plane target has an unapproved IP destination %q", target)
	}
	if parsed.Port() != "" && parsed.Port() != "443" {
		return "", fmt.Errorf("cloud data-plane target has an unapproved port %q", target)
	}
	if !approvedCloudDataPlaneHost.MatchString(host) {
		return "", fmt.Errorf("cloud data-plane target has an unapproved destination %q", target)
	}
	return target, nil
}

func (s *Steps) deleteAPI(ctx context.Context) error {
	id, err := requiredValue(ctx, keyAPIID)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodDelete, "/rest-apis/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return responseError(resp)
	}
	if resp.StatusCode == http.StatusNotFound {
		deregister(ctx, cleanup.KindAPI, id)
	}
	return nil
}

func (s *Steps) verifyAPIDeleted(ctx context.Context) error {
	id, err := requiredValue(ctx, keyAPIID)
	if err != nil {
		return err
	}
	if err := s.awaitNotFound(ctx, "/rest-apis/"+url.PathEscape(id), "REST API "+id); err != nil {
		return err
	}
	deregister(ctx, cleanup.KindAPI, id)
	return nil
}

func (s *Steps) createProject(ctx context.Context) error {
	if err := generateResourceAndStore(ctx, "syn-project", keyProjectID); err != nil {
		return err
	}
	id, err := requiredValue(ctx, keyProjectID)
	if err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindProject, ID: id, Actor: resourceActor, Description: "project " + id,
	}); err != nil {
		return fmt.Errorf("registering project %q for cleanup: %w", id, err)
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, "/projects", map[string]string{
		"id": id, "displayName": "Synthetic " + id,
		"description": "Created and deleted by the APIP cloud integration test. Safe to delete.",
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return responseError(resp)
	}
	createdID, err := stringField(resp, "id")
	if err != nil {
		return fmt.Errorf("creating APIP project: %w", err)
	}
	if createdID != id {
		deregister(ctx, cleanup.KindProject, id)
		if err := cleanup.Register(ctx, cleanup.Resource{
			Kind: cleanup.KindProject, ID: createdID, Actor: resourceActor, Description: "project " + createdID,
		}); err != nil {
			return fmt.Errorf("registering project %q for cleanup: %w", createdID, err)
		}
	}
	return tcontext.Set(ctx, keyProjectID, createdID)
}

func (s *Steps) getProject(ctx context.Context) error {
	id, err := requiredValue(ctx, keyProjectID)
	if err != nil {
		return err
	}
	err = retry.Await(ctx, retry.Options{Timeout: propagationTimeout(s.topo), Interval: 3 * time.Second},
		func(ctx context.Context) (*httpx.Response, error) {
			return s.doAuthenticatedRaw(ctx, http.MethodGet, "/projects/"+url.PathEscape(id), nil)
		}, func(resp *httpx.Response) bool {
			if resp == nil || resp.StatusCode != http.StatusOK {
				return false
			}
			value, fieldErr := stringField(resp, "id")
			return fieldErr == nil && value == id
		}, "waiting for APIP project "+id+" to be retrievable")
	return err
}

func (s *Steps) deleteProject(ctx context.Context) error {
	id, err := requiredValue(ctx, keyProjectID)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodDelete, "/projects/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return responseError(resp)
	}
	if resp.StatusCode == http.StatusNotFound {
		deregister(ctx, cleanup.KindProject, id)
	}
	return nil
}

func (s *Steps) verifyProjectDeleted(ctx context.Context) error {
	id, err := requiredValue(ctx, keyProjectID)
	if err != nil {
		return err
	}
	if err := s.awaitNotFound(ctx, "/projects/"+url.PathEscape(id), "project "+id); err != nil {
		return err
	}
	deregister(ctx, cleanup.KindProject, id)
	return nil
}

func (s *Steps) createEnvironment(ctx context.Context) error {
	if err := generateResourceAndStore(ctx, "syn-env", keyEnvironmentName); err != nil {
		return err
	}
	id, err := requiredValue(ctx, keyEnvironmentName)
	if err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindEnvironment, ID: id, Actor: resourceActor, Description: "environment " + id,
	}); err != nil {
		return fmt.Errorf("registering environment %q for cleanup: %w", id, err)
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, "/environments", map[string]any{
		"name": id, "isProduction": false,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return responseError(resp)
	}
	createdID, err := stringField(resp, "id")
	if err != nil {
		return fmt.Errorf("creating APIP environment: %w", err)
	}
	if createdID != id {
		deregister(ctx, cleanup.KindEnvironment, id)
		if registerErr := cleanup.Register(ctx, cleanup.Resource{
			Kind: cleanup.KindEnvironment, ID: createdID, Actor: resourceActor, Description: "environment " + createdID,
		}); registerErr != nil {
			return fmt.Errorf("registering environment %q for cleanup: %w", createdID, registerErr)
		}
		return fmt.Errorf("created APIP environment id = %q, want %q", createdID, id)
	}
	return nil
}

func (s *Steps) duplicateEnvironment(ctx context.Context) error {
	id, err := requiredValue(ctx, keyEnvironmentName)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, "/environments", map[string]any{
		"name": id, "isProduction": false,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusConflict {
		return responseError(resp)
	}
	return nil
}

func (s *Steps) deleteEnvironment(ctx context.Context) error {
	id, err := requiredValue(ctx, keyEnvironmentName)
	if err != nil {
		return err
	}
	return s.deleteEnvironmentUntilGone(ctx, id)
}

func (s *Steps) verifyEnvironmentDeleted(ctx context.Context) error {
	id, err := requiredValue(ctx, keyEnvironmentName)
	if err != nil {
		return err
	}
	if err := s.awaitEnvironmentNotFound(ctx, id); err != nil {
		return err
	}
	deregister(ctx, cleanup.KindEnvironment, id)
	return nil
}

func (s *Steps) deleteEnvironmentUntilGone(ctx context.Context, id string) error {
	err := s.awaitEnvironmentNotFound(ctx, id)
	if err != nil {
		return err
	}
	deregister(ctx, cleanup.KindEnvironment, id)
	return nil
}

func (s *Steps) awaitEnvironmentNotFound(ctx context.Context, id string) error {
	return retry.Await(ctx, retry.Options{Timeout: 2 * time.Minute, Interval: 3 * time.Second},
		func(ctx context.Context) (*httpx.Response, error) {
			return s.doAuthenticatedRaw(ctx, http.MethodDelete, "/environments/"+url.PathEscape(id), nil)
		}, func(resp *httpx.Response) bool {
			return resp != nil && resp.StatusCode == http.StatusNotFound
		}, "waiting for APIP environment "+id+" deletion")
}

func (s *Steps) selectGatewayEnvironment(ctx context.Context) error {
	resp, err := s.doAuthenticated(ctx, http.MethodGet, "/environments", nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	items, err := listItems(resp)
	if err != nil {
		return fmt.Errorf("decoding APIP environments: %w", err)
	}
	for _, item := range items {
		name, ok := item["name"].(string)
		if ok && strings.TrimSpace(name) != "" {
			return tcontext.Set(ctx, keyEnvironmentName, name)
		}
	}
	return fmt.Errorf("APIP environment list contains no usable environment")
}

func (s *Steps) createManagedGateway(ctx context.Context) error {
	environment, err := requiredValue(ctx, keyEnvironmentName)
	if err != nil {
		return err
	}
	if err := generateResourceAndStore(ctx, "syn-gateway", keyManagedGatewayID); err != nil {
		return err
	}
	name, err := requiredValue(ctx, keyManagedGatewayID)
	if err != nil {
		return err
	}
	// The managed-gateway API normally derives its registration id from the
	// environment and display name. Keep that deterministic fallback registered so
	// cleanup can still attempt deletion if a successful create has no decodable body.
	cleanupID := environment + "-" + name
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindGateway, ID: cleanupID, Actor: resourceActor, Description: "managed gateway " + cleanupID,
	}); err != nil {
		return fmt.Errorf("registering managed gateway %q for cleanup: %w", cleanupID, err)
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, "/managed-gateways", map[string]string{
		"displayName": name, "environment": environment,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return responseError(resp)
	}
	id, err := stringField(resp, "id")
	if err != nil {
		return fmt.Errorf("creating APIP managed gateway: %w", err)
	}
	if id != cleanupID {
		deregister(ctx, cleanup.KindGateway, cleanupID)
		if err := cleanup.Register(ctx, cleanup.Resource{
			Kind: cleanup.KindGateway, ID: id, Actor: resourceActor, Description: "managed gateway " + id,
		}); err != nil {
			return fmt.Errorf("registering managed gateway %q for cleanup: %w", id, err)
		}
	}
	return tcontext.Set(ctx, keyManagedGatewayID, id)
}

func (s *Steps) getManagedGateway(ctx context.Context) error {
	id, err := requiredValue(ctx, keyManagedGatewayID)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodGet, "/gateways/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	got, err := stringField(resp, "id")
	if err != nil {
		return fmt.Errorf("retrieving APIP managed gateway: %w", err)
	}
	if got != id {
		return fmt.Errorf("retrieved APIP gateway id = %q, want %q", got, id)
	}
	return nil
}

func (s *Steps) deleteManagedGateway(ctx context.Context) error {
	id, err := requiredValue(ctx, keyManagedGatewayID)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodDelete, "/managed-gateways/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return responseError(resp)
	}
	if resp.StatusCode == http.StatusNotFound {
		deregister(ctx, cleanup.KindGateway, id)
	}
	return nil
}

func (s *Steps) verifyManagedGatewayDeleted(ctx context.Context) error {
	id, err := requiredValue(ctx, keyManagedGatewayID)
	if err != nil {
		return err
	}
	if err := s.awaitNotFound(ctx, "/gateways/"+url.PathEscape(id), "managed gateway "+id); err != nil {
		return err
	}
	deregister(ctx, cleanup.KindGateway, id)
	return nil
}

func (s *Steps) awaitNotFound(ctx context.Context, path, what string) error {
	return retry.Await(ctx, retry.Options{Timeout: propagationTimeout(s.topo), Interval: 3 * time.Second},
		func(ctx context.Context) (*httpx.Response, error) {
			return s.doAuthenticatedRaw(ctx, http.MethodGet, path, nil)
		}, func(resp *httpx.Response) bool {
			return resp != nil && resp.StatusCode == http.StatusNotFound
		}, "waiting for "+what+" to be absent")
}

func (s *Steps) doAuthenticated(ctx context.Context, method, path string, body any) (*httpx.Response, error) {
	resp, err := s.doAuthenticatedRaw(ctx, method, path, body)
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

func (s *Steps) doAuthenticatedRaw(ctx context.Context, method, path string, body any) (*httpx.Response, error) {
	token, err := bearerToken(ctx)
	if err != nil {
		return nil, err
	}
	base, err := s.topo.URL(corecatalog.Name, corecatalog.EndpointAPIPBML)
	if err != nil {
		return nil, err
	}
	var payload []byte
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding request body for %s: %w", path, err)
		}
	}
	return s.client.Do(ctx, httpx.Request{
		Method: method,
		URL:    cloudAPIURL(base, path),
		Headers: map[string]string{
			"Authorization": "Bearer " + token,
			"Content-Type":  "application/json",
		},
		Body: payload,
	}, 0, 0)
}

func deregister(ctx context.Context, kind cleanup.Kind, id string) {
	if registry, err := cleanup.Of(ctx); err == nil {
		registry.Deregister(kind, id)
	}
}

func requiredValue(ctx context.Context, key string) (string, error) {
	value, err := tcontext.ResolveString(ctx, key)
	if err != nil || strings.TrimSpace(value) == "" {
		if err != nil {
			return "", err
		}
		return "", fmt.Errorf("context key %q is empty", key)
	}
	return value, nil
}

func listItems(resp *httpx.Response) ([]map[string]any, error) {
	var document struct {
		List []map[string]any `json:"list"`
	}
	if err := json.Unmarshal(resp.Body, &document); err != nil {
		return nil, err
	}
	return document.List, nil
}

func stringField(resp *httpx.Response, field string) (string, error) {
	var document map[string]any
	if err := json.Unmarshal(resp.Body, &document); err != nil {
		return "", err
	}
	value, ok := document[field].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("response field %q is missing or empty", field)
	}
	return value, nil
}
