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

// defaultProjectID is the project every organization is given at onboarding, and
// the one the REST API scenarios deploy into.
const defaultProjectID = "default"

// findDefaultProject asks for the project by id rather than scanning the list.
//
// /projects is paginated and answers with 20 of them by default, so on an
// organization with more projects than that the default is simply on a later
// page. Scanning the first page reported it missing and sent the reader looking
// for a broken organization: one real organization held 213 projects, with the
// default among them the whole time.
func (s *Steps) findDefaultProject(ctx context.Context) error {
	resp, err := s.doAuthenticated(ctx, http.MethodGet, "/projects/"+defaultProjectID, nil)
	if err != nil {
		return err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return tcontext.Set(ctx, keyProjectID, defaultProjectID)
	case http.StatusNotFound:
		return fmt.Errorf("the organization has no %q project", defaultProjectID)
	default:
		return responseError(resp)
	}
}

// findActiveGateway reads through the managed-gateway endpoint rather than the
// generic gateway one. A cloud scenario should exercise the same route a console
// user drives, so the managed-gateway view is part of what is under test here;
// reading the generic endpoint would skip it.
func (s *Steps) findActiveGateway(ctx context.Context) error {
	resp, err := s.doAuthenticated(ctx, http.MethodGet, "/managed-gateways", nil)
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
	choice, err := s.chooseActiveGateway(ctx, items, "APIP gateway")
	if err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keyGatewayID, choice.id); err != nil {
		return err
	}
	if choice.environment != "" {
		if err := tcontext.Set(ctx, keyGatewayEnv, choice.environment); err != nil {
			return err
		}
	}
	return tcontext.Set(ctx, keyGatewayEndpoint, choice.endpoint)
}

// gatewayChoice is a listed gateway a scenario can deploy to and send traffic through.
type gatewayChoice struct {
	id          string
	environment string
	endpoint    string
}

// chooseActiveGateway picks the first listed gateway that is active and has an approved data-plane
// endpoint. label names the kind of gateway in the failure.
func (s *Steps) chooseActiveGateway(ctx context.Context, items []map[string]any, label string) (gatewayChoice, error) {
	// Record why each candidate was rejected. Skipping silently and reporting
	// only "none found" hides the difference between an organization with no
	// gateways and one whose gateways were all filtered out here.
	skipped := make([]string, 0, len(items))
	for _, item := range items {
		id, _ := item["id"].(string)
		id = strings.TrimSpace(id)
		if id == "" {
			skipped = append(skipped, "<no id>")
			continue
		}
		if active, ok := item["isActive"].(bool); !ok || !active {
			skipped = append(skipped, id+": not active")
			continue
		}
		endpoints, ok := item["endpoints"].([]any)
		if !ok || len(endpoints) == 0 {
			skipped = append(skipped, id+": no endpoints")
			continue
		}
		endpoint, ok := firstApprovedDataPlaneEndpoint(ctx, s, endpoints)
		if !ok {
			skipped = append(skipped, id+": no endpoint passed the data-plane host check")
			continue
		}
		environment, _ := item["environment"].(string)
		return gatewayChoice{id: id, environment: strings.TrimSpace(environment), endpoint: endpoint}, nil
	}
	if len(items) == 0 {
		return gatewayChoice{}, fmt.Errorf("the organization has no %ss", label)
	}
	return gatewayChoice{}, fmt.Errorf("no usable %s among %d listed: %s", label, len(items), strings.Join(skipped, "; "))
}

// sendUntilStatus invokes an absolute cloud data-plane URL until it returns the exact status.
func (s *Steps) sendUntilStatus(ctx context.Context, method, target string, want int) error {
	return s.sendUntilStatusWith(ctx, method, target, want, nil)
}

// sendUntilStatusWith is sendUntilStatus with extra request headers.
func (s *Steps) sendUntilStatusWith(ctx context.Context, method, target string, want int, headers map[string]string) error {
	return s.sendRequestUntilStatus(ctx, httpx.Request{Method: method, URL: target, Headers: headers}, want)
}

// prepareDataPlaneRequest checks a data-plane request can be sent: a method, a funnel, and a
// target that expands and passes the data-plane host check. It returns the request ready to send.
func (s *Steps) prepareDataPlaneRequest(ctx context.Context, request httpx.Request) (httpx.Request, error) {
	if s.funnel == nil {
		return request, fmt.Errorf("sending %s request: HTTP funnel is required", strings.ToUpper(request.Method))
	}
	request.Method = strings.ToUpper(strings.TrimSpace(request.Method))
	if request.Method == "" {
		return request, fmt.Errorf("sending request: HTTP method is required")
	}
	target, err := expandCloudValue(ctx, request.URL)
	if err != nil {
		return request, err
	}
	request.URL, err = s.resolveDataPlaneURL(ctx, target)
	return request, err
}

// sendRequestUntilStatus sends a data-plane request until the gateway answers with the status.
func (s *Steps) sendRequestUntilStatus(ctx context.Context, request httpx.Request, want int) error {
	if want < 100 || want > 599 {
		return fmt.Errorf("sending %s request: invalid HTTP status %d", strings.ToUpper(request.Method), want)
	}
	request, err := s.prepareDataPlaneRequest(ctx, request)
	if err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{Timeout: propagationTimeout(s.topo)},
		func(ctx context.Context) (*httpx.Response, error) {
			response, sendErr := s.funnel.Send(ctx, request)
			if sendErr != nil {
				return nil, retry.Transient(sendErr)
			}
			return response, nil
		},
		func(response *httpx.Response) bool {
			return response != nil && response.StatusCode == want
		},
		fmt.Sprintf("waiting for %s %s to return %d", request.Method, request.URL, want))
}

func (s *Steps) assertAPIResponse(ctx context.Context, post int) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body, &body); err != nil {
		return fmt.Errorf("decoding the synthetic REST API response: %w (%s)", err, response.Describe())
	}
	if body["id"] != float64(post) {
		return fmt.Errorf("expected the synthetic REST API response to contain post %d, got %s", post, response.Describe())
	}
	return nil
}

var approvedCloudDataPlaneHost = regexp.MustCompile(
	`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.gateway(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*\.wso2\.com$`,
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
	return s.createProjectOf(ctx, cleanup.KindProject)
}

// createProjectOf creates a uniquely named project, registered for cleanup under the given kind
// before it is requested, and keeps it as the scenario's project.
func (s *Steps) createProjectOf(ctx context.Context, kind cleanup.Kind) error {
	id, resp, err := s.requestProjectOf(ctx, kind)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return responseError(resp)
	}
	return s.keepCreatedProject(ctx, kind, id, resp)
}

// requestProjectOf asks for a uniquely named project and returns the answer whatever it is. The
// project is registered for cleanup under the given kind before it is requested.
func (s *Steps) requestProjectOf(ctx context.Context, kind cleanup.Kind) (string, *httpx.Response, error) {
	if err := generateResourceAndStore(ctx, "syn-project", keyProjectID); err != nil {
		return "", nil, err
	}
	id, err := requiredValue(ctx, keyProjectID)
	if err != nil {
		return "", nil, err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: kind, ID: id, Actor: resourceActor, Description: "project " + id,
	}); err != nil {
		return "", nil, fmt.Errorf("registering project %q for cleanup: %w", id, err)
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, "/projects", map[string]string{
		"id": id, "displayName": "Synthetic " + id,
		"description": "Created and deleted by the APIP cloud integration test. Safe to delete.",
	})
	if err != nil {
		return "", nil, err
	}
	return id, resp, nil
}

// keepCreatedProject makes the project a create answered 201 for the scenario's project, and
// registers the id the platform gave it when that differs from the one requested.
func (s *Steps) keepCreatedProject(ctx context.Context, kind cleanup.Kind, id string, resp *httpx.Response) error {
	createdID, err := stringField(resp, "id")
	if err != nil {
		return fmt.Errorf("creating APIP project: %w", err)
	}
	if createdID != id {
		deregister(ctx, kind, id)
		if err := cleanup.Register(ctx, cleanup.Resource{
			Kind: kind, ID: createdID, Actor: resourceActor, Description: "project " + createdID,
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
	_, err = s.createManagedGatewayIn(ctx, environment)
	return err
}

// createManagedGatewayIn creates a uniquely named managed gateway in an environment, registers it
// for cleanup before the request, and keeps its id as the scenario's managed gateway. The platform
// registers a gateway as "<environment>-<handle>" within 40 characters, so a name too long for the
// environment is sent with a handle cut to fit.
func (s *Steps) createManagedGatewayIn(ctx context.Context, environment string) (string, error) {
	// "sgw", not "syn-gateway": a managed gateway's handle is bounded by the
	// release label the control plane renders from it, which spends the
	// environment name twice and leaves 15 characters in a "development"
	// environment. Unique() adds 11 ("-<suffix8>-<n>"), so the base has to fit in
	// the rest. A longer base is accepted at create time and then fails to apply,
	// leaving a gateway that never becomes ready.
	if err := generateResourceAndStore(ctx, "sgw", keyManagedGatewayID); err != nil {
		return "", err
	}
	name, err := requiredValue(ctx, keyManagedGatewayID)
	if err != nil {
		return "", err
	}
	request := map[string]string{"displayName": name, "environment": environment}
	handle := name
	if budget := gatewayHandleBudget(environment); len(name) > budget {
		handle = handleOfLength(name, budget)
		request["handle"] = handle
	}
	// The managed-gateway API normally derives its registration id from the
	// environment and handle. Keep that deterministic fallback registered so
	// cleanup can still attempt deletion if a successful create has no decodable body.
	cleanupID := environment + "-" + handle
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindGateway, ID: cleanupID, Actor: resourceActor, Description: "managed gateway " + cleanupID,
	}); err != nil {
		return "", fmt.Errorf("registering managed gateway %q for cleanup: %w", cleanupID, err)
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, "/managed-gateways", request)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusCreated {
		return "", responseError(resp)
	}
	id, err := stringField(resp, "id")
	if err != nil {
		return "", fmt.Errorf("creating APIP managed gateway: %w", err)
	}
	if id != cleanupID {
		deregister(ctx, cleanup.KindGateway, cleanupID)
		if err := cleanup.Register(ctx, cleanup.Resource{
			Kind: cleanup.KindGateway, ID: id, Actor: resourceActor, Description: "managed gateway " + id,
		}); err != nil {
			return "", fmt.Errorf("registering managed gateway %q for cleanup: %w", id, err)
		}
	}
	if err := tcontext.Set(ctx, keyManagedGatewayEnv, environment); err != nil {
		return "", err
	}
	return id, tcontext.Set(ctx, keyManagedGatewayID, id)
}

func (s *Steps) getManagedGateway(ctx context.Context) error {
	id, err := requiredValue(ctx, keyManagedGatewayID)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodGet, "/managed-gateways/"+url.PathEscape(id), nil)
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
	return tcontext.Set(ctx, keyGatewayDeleteStatus, fmt.Sprint(resp.StatusCode))
}

func (s *Steps) verifyManagedGatewayDeleted(ctx context.Context) error {
	id, err := requiredValue(ctx, keyManagedGatewayID)
	if err != nil {
		return err
	}
	if err := s.awaitNotFound(ctx, "/managed-gateways/"+url.PathEscape(id), "managed gateway "+id); err != nil {
		if status := optionalValue(ctx, keyGatewayDeleteStatus); status != "" {
			return fmt.Errorf("%w (the delete was answered %s)", err, status)
		}
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
	base, err := s.topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
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
