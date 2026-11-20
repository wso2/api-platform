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

package cloudbase

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

const (
	defaultAPIOperations = `[{"name":"getPost","request":{"method":"GET","path":"/posts/1"}}]`
	newVersionAPILabel   = "new API version"
	// BackendEnv overrides the backend the synthetic REST APIs call.
	BackendEnv = "CLOUD_IT_BACKEND_URL"
	// DefaultBackend is the testbench backend service, which answers any path with the request it received.
	DefaultBackend = "https://development-wso2cloud.gateway.dev.cloud.wso2.com/apip-testbench-mocks/backend"
)

// APIRef names the scenario-state keys that describe one synthetic API, so a step can act on the
// REST API created first, on a second version of it, or on a GraphQL API.
type APIRef struct {
	ID         string
	deployment string
	definition string
	// kind is the artifact kind the project deployment routes are told; empty means a REST API.
	kind string
	// collection is the platform route that reads the API and its deployments.
	Collection string
	// builds lists the builds the scenario saw for the API, oldest first, and buildsOwner the API
	// they belong to. Both are empty for an API whose builds are not followed.
	builds      string
	BuildsOwner string
}

var (
	PrimaryAPI = APIRef{
		ID: KeyAPIID, deployment: KeyDeploymentID, definition: KeyAPIDefinition,
		Collection: RestAPIsCollection, builds: KeyBuildIDs, BuildsOwner: KeyBuildsOwner,
	}
	VersionAPI = APIRef{
		ID: KeyVersionAPIID, deployment: keyVersionDeploymentID, definition: keyVersionAPIDefinition,
		Collection: RestAPIsCollection,
	}
	GraphQLAPI = APIRef{
		ID: KeyGraphQLAPIID, deployment: KeyGraphQLDeploymentID, definition: KeyGraphQLDefinition,
		kind: KindGraphQLAPI, Collection: GraphQLAPIsCollection,
		builds: keyGraphQLBuildIDs, BuildsOwner: keyGraphQLBuildsOwner,
	}
)

// APIRefFor resolves the label used in step text to the state keys it refers to.
func APIRefFor(label string) APIRef {
	switch label {
	case newVersionAPILabel:
		return VersionAPI
	case GraphQLAPILabel:
		return GraphQLAPI
	default:
		return PrimaryAPI
	}
}

// WithKind adds the artifact kind to a project deployment route; a REST API needs none.
func WithKind(path, kind string) string {
	if kind == "" {
		return path
	}
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return path + separator + "kind=" + url.QueryEscape(kind)
}

// apiOverrideKeys lists the definition fields a values table may replace; the first three are
// plain strings and the rest carry JSON.
var apiOverrideKeys = map[string]bool{
	"displayName": true, "context": true, "version": true,
	"operations": true, "policies": true, "security": true,
}

// TableValues reads a two-column key/value Gherkin table.
func TableValues(table *godog.Table) (map[string]string, error) {
	values := map[string]string{}
	if table == nil {
		return values, nil
	}
	for _, row := range table.Rows {
		if len(row.Cells) != 2 {
			return nil, fmt.Errorf("values table rows must have exactly two cells, got %d", len(row.Cells))
		}
		key := strings.TrimSpace(row.Cells[0].Value)
		if key == "" {
			return nil, fmt.Errorf("values table contains an empty key")
		}
		if _, duplicate := values[key]; duplicate {
			return nil, fmt.Errorf("values table repeats key %q", key)
		}
		values[key] = row.Cells[1].Value
	}
	return values, nil
}

// applyAPIOverrides replaces fields of a REST API definition with the values a scenario supplies.
func applyAPIOverrides(ctx context.Context, definition map[string]any, overrides map[string]string) error {
	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !apiOverrideKeys[key] {
			return fmt.Errorf("unsupported REST API value %q", key)
		}
		value, err := ExpandCloudValue(ctx, overrides[key])
		if err != nil {
			return err
		}
		switch key {
		case "operations", "policies", "security":
			var parsed any
			if err := json.Unmarshal([]byte(value), &parsed); err != nil {
				return fmt.Errorf("REST API value %q is not valid JSON: %w", key, err)
			}
			definition[key] = parsed
		default:
			definition[key] = value
		}
	}
	return nil
}

// newAPIDefinition returns the default synthetic REST API with the overrides applied.
func newAPIDefinition(ctx context.Context, id, projectID string, overrides map[string]string) (map[string]any, error) {
	var operations any
	if err := json.Unmarshal([]byte(defaultAPIOperations), &operations); err != nil {
		return nil, err
	}
	backend, err := TestbenchURL(BackendEnv, DefaultBackend)
	if err != nil {
		return nil, err
	}
	definition := map[string]any{
		"id": id, "displayName": "Synthetic " + id,
		"description": "Created and deleted by the APIP cloud integration test. Safe to delete.",
		"context":     "/" + id + "/v1", "version": "1.0.0", "projectId": projectID,
		"upstream":   map[string]any{"main": map[string]string{"url": backend}},
		"security":   map[string]bool{"enabled": false},
		"operations": operations,
	}
	if err := applyAPIOverrides(ctx, definition, overrides); err != nil {
		return nil, err
	}
	return definition, nil
}

// RegisterAPIForCleanup records an API so the cleanup registry deletes it if the scenario does not.
func RegisterAPIForCleanup(ctx context.Context, id string) error {
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindAPI, ID: id, Actor: ResourceActor, Description: "REST API " + id,
	}); err != nil {
		return fmt.Errorf("registering API %q for cleanup: %w", id, err)
	}
	return nil
}

// CreateDefaultAPI creates a REST API with the default definition in the suite project.
func (s *Steps) CreateDefaultAPI(ctx context.Context) error {
	return s.CreateAPI(ctx, PrimaryAPI, nil)
}

// CreateAPIWithValues creates a REST API whose definition a values table overrides.
func (s *Steps) CreateAPIWithValues(ctx context.Context, table *godog.Table) error {
	overrides, err := TableValues(table)
	if err != nil {
		return err
	}
	return s.CreateAPI(ctx, PrimaryAPI, overrides)
}

// CreateNewAPIVersion creates a second API that shares the first API's display name and differs
// in version, which is how the platform models a new version of an API.
func (s *Steps) CreateNewAPIVersion(ctx context.Context, version string, table *godog.Table) error {
	overrides, err := TableValues(table)
	if err != nil {
		return err
	}
	definition, err := s.StoredDefinition(ctx, PrimaryAPI)
	if err != nil {
		return err
	}
	if _, set := overrides["displayName"]; !set {
		name, _ := definition["displayName"].(string)
		overrides["displayName"] = name
	}
	overrides["version"] = version
	if _, set := overrides["context"]; !set {
		overrides["context"] = "/${CTX:" + KeyVersionAPIID + "}/" + version
	}
	return s.CreateAPI(ctx, VersionAPI, overrides)
}

// CreateAPI creates the REST API a ref names, applying the overrides to the default definition.
func (s *Steps) CreateAPI(ctx context.Context, ref APIRef, overrides map[string]string) error {
	projectID, err := RequiredValue(ctx, KeyProjectID)
	if err != nil {
		return err
	}
	if err := GenerateResourceAndStore(ctx, "syn-api", ref.ID); err != nil {
		return err
	}
	apiID, err := RequiredValue(ctx, ref.ID)
	if err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindAPI, ID: apiID, Actor: ResourceActor, Description: "REST API " + apiID,
	}); err != nil {
		return fmt.Errorf("registering API %q for cleanup: %w", apiID, err)
	}
	definition, err := newAPIDefinition(ctx, apiID, projectID, overrides)
	if err != nil {
		return err
	}
	resp, err := s.DoAuthenticated(ctx, http.MethodPost, "/rest-apis", definition)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return ResponseError(resp)
	}
	createdID, err := StringField(resp, "id")
	if err != nil {
		return fmt.Errorf("creating APIP REST API: %w", err)
	}
	if createdID != apiID {
		Deregister(ctx, cleanup.KindAPI, apiID)
		if err := RegisterAPIForCleanup(ctx, createdID); err != nil {
			return err
		}
		definition["id"] = createdID
	}
	if err := StoreDefinition(ctx, ref, definition); err != nil {
		return err
	}
	return tcontext.Set(ctx, ref.ID, createdID)
}

// SubmitAPICreation sends a REST API creation request and publishes the response without
// requiring success, so a scenario can assert on a rejection.
func (s *Steps) SubmitAPICreation(ctx context.Context, table *godog.Table) error {
	overrides, err := TableValues(table)
	if err != nil {
		return err
	}
	projectID, err := RequiredValue(ctx, KeyProjectID)
	if err != nil {
		return err
	}
	if err := GenerateResourceAndStore(ctx, "syn-api", KeyRejectedAPIID); err != nil {
		return err
	}
	id, err := RequiredValue(ctx, KeyRejectedAPIID)
	if err != nil {
		return err
	}
	if err := RegisterAPIForCleanup(ctx, id); err != nil {
		return err
	}
	definition, err := newAPIDefinition(ctx, id, projectID, overrides)
	if err != nil {
		return err
	}
	resp, err := s.DoAuthenticated(ctx, http.MethodPost, "/rest-apis", definition)
	if err != nil {
		return err
	}
	if RefusedCreate(resp) {
		Deregister(ctx, cleanup.KindAPI, id)
	}
	return nil
}

// AssertCloudResponseStatus checks the status code of the published cloud API response.
func (s *Steps) AssertCloudResponseStatus(ctx context.Context, want int) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	if response.StatusCode != want {
		return fmt.Errorf("expected response status %d, got %s", want, response.Describe())
	}
	return nil
}

// StoreDefinition keeps the definition of an API in the scenario context.
func StoreDefinition(ctx context.Context, ref APIRef, definition map[string]any) error {
	encoded, err := json.Marshal(definition)
	if err != nil {
		return fmt.Errorf("encoding REST API definition: %w", err)
	}
	return tcontext.Set(ctx, ref.definition, string(encoded))
}

// StoredDefinition returns the definition kept for an API.
func (s *Steps) StoredDefinition(ctx context.Context, ref APIRef) (map[string]any, error) {
	encoded, err := RequiredValue(ctx, ref.definition)
	if err != nil {
		return nil, err
	}
	var definition map[string]any
	if err := json.Unmarshal([]byte(encoded), &definition); err != nil {
		return nil, fmt.Errorf("decoding stored REST API definition: %w", err)
	}
	return definition, nil
}

// UpdateAPI replaces the synthetic REST API's definition with the stored one plus the overrides.
func (s *Steps) UpdateAPI(ctx context.Context, table *godog.Table) error {
	overrides, err := TableValues(table)
	if err != nil {
		return err
	}
	id, err := RequiredValue(ctx, PrimaryAPI.ID)
	if err != nil {
		return err
	}
	definition, err := s.StoredDefinition(ctx, PrimaryAPI)
	if err != nil {
		return err
	}
	if err := applyAPIOverrides(ctx, definition, overrides); err != nil {
		return err
	}
	resp, err := s.DoAuthenticated(ctx, http.MethodPut, "/rest-apis/"+url.PathEscape(id), definition)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return ResponseError(resp)
	}
	return StoreDefinition(ctx, PrimaryAPI, definition)
}

// DeployAPI deploys an API to the active gateway.
func (s *Steps) DeployAPI(ctx context.Context, label string) error {
	return s.deployAPIAs(ctx, APIRefFor(label))
}

// RedeployAPI deploys the scenario's REST API again to the active gateway.
func (s *Steps) RedeployAPI(ctx context.Context) error {
	return s.deployAPIAs(ctx, PrimaryAPI)
}

// deployAPIAs deploys the current definition of the API a ref points at to the active gateway,
// keeps the id of the deployment the platform created for it, and adds the build it rendered to
// the builds the scenario follows for the API.
func (s *Steps) deployAPIAs(ctx context.Context, ref APIRef) error {
	deployed, err := s.DeployRefToActiveGateway(ctx, ref, "")
	if err != nil {
		return err
	}
	if ref.builds == "" || deployed.Build == "" {
		return nil
	}
	return AppendBuild(ctx, ref, deployed.Build)
}

// DeployRefToActiveGateway deploys the API a ref points at to the active gateway, naming a build
// when one is given, and keeps the id of the deployment created on that gateway.
func (s *Steps) DeployRefToActiveGateway(ctx context.Context, ref APIRef, buildID string) (deployment, error) {
	apiID, err := RequiredValue(ctx, ref.ID)
	if err != nil {
		return deployment{}, err
	}
	gatewayID, err := RequiredValue(ctx, KeyGatewayID)
	if err != nil {
		return deployment{}, err
	}
	environment, err := RequiredValue(ctx, KeyGatewayEnv)
	if err != nil {
		return deployment{}, fmt.Errorf("the active gateway does not report its environment: %w", err)
	}
	deployed, err := s.DeployArtifact(ctx, ref.kind, apiID, environment, gatewayID, buildID)
	if err != nil {
		return deployment{}, fmt.Errorf("deploying APIP API %s: %w", apiID, err)
	}
	return deployed, tcontext.Set(ctx, ref.deployment, deployed.ID)
}

// deployment is what a deployment request created on one gateway: the deployment and the build
// it runs.
type deployment struct {
	ID    string
	Build string
}

// DeployArtifact deploys an API of a kind to one gateway of an environment through the API's
// project. The platform renders a build of the definition as it is now when no build is named.
func (s *Steps) DeployArtifact(ctx context.Context, kind, apiID, environment, gatewayID, buildID string) (deployment, error) {
	projectID, err := RequiredValue(ctx, KeyProjectID)
	if err != nil {
		return deployment{}, err
	}
	body := map[string]any{"environment": environment, "gateways": []map[string]string{{"gatewayId": gatewayID}}}
	if buildID != "" {
		body["buildId"] = buildID
	}
	resp, err := s.DoAuthenticated(ctx, http.MethodPost,
		WithKind("/projects/"+url.PathEscape(projectID)+"/apis/"+url.PathEscape(apiID)+"/deployments", kind), body)
	if err != nil {
		return deployment{}, err
	}
	if resp.StatusCode != http.StatusCreated {
		return deployment{}, ResponseError(resp)
	}
	items, err := ListItems(resp)
	if err != nil {
		return deployment{}, fmt.Errorf("decoding the deployment response: %w", err)
	}
	for _, item := range items {
		if item["gatewayId"] != gatewayID {
			continue
		}
		if id, _ := item["deploymentId"].(string); id != "" {
			build, _ := item["buildId"].(string)
			return deployment{ID: id, Build: build}, nil
		}
	}
	return deployment{}, fmt.Errorf("the deployment response lists no deployment on gateway %q", gatewayID)
}

// UndeployAPI stops the deployment of the labelled API on the active gateway.
func (s *Steps) UndeployAPI(ctx context.Context, label string) error {
	resp, err := s.SendUndeploy(ctx, APIRefFor(label), "")
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent {
		return ResponseError(resp)
	}
	return nil
}

// SendUndeploy sends the undeploy of the deployment a ref points at on the active gateway through
// the API's project, in the active gateway's environment unless another is given.
func (s *Steps) SendUndeploy(ctx context.Context, ref APIRef, environment string) (*httpx.Response, error) {
	deploymentID, err := RequiredValue(ctx, ref.deployment)
	if err != nil {
		return nil, err
	}
	gatewayID, err := RequiredValue(ctx, KeyGatewayID)
	if err != nil {
		return nil, err
	}
	if environment == "" {
		if environment, err = RequiredValue(ctx, KeyGatewayEnv); err != nil {
			return nil, fmt.Errorf("the active gateway does not report its environment: %w", err)
		}
	}
	scope, err := RefScopePath(ctx, ref)
	if err != nil {
		return nil, err
	}
	return s.DoAuthenticated(ctx, http.MethodPost, WithKind(
		scope+"/deployments/"+url.PathEscape(deploymentID)+"/undeploy?environment="+url.QueryEscape(environment)+
			"&gatewayId="+url.QueryEscape(gatewayID), ref.kind), nil)
}

// DeploymentRecord reads the record of the deployment a ref points at; it is nil when the list
// does not contain that deployment.
func (s *Steps) DeploymentRecord(ctx context.Context, ref APIRef) (map[string]any, error) {
	apiID, err := RequiredValue(ctx, ref.ID)
	if err != nil {
		return nil, err
	}
	deploymentID, err := RequiredValue(ctx, ref.deployment)
	if err != nil {
		return nil, err
	}
	gatewayID, err := RequiredValue(ctx, KeyGatewayID)
	if err != nil {
		return nil, err
	}
	resp, err := s.DoAuthenticatedRaw(ctx, http.MethodGet,
		ref.Collection+"/"+url.PathEscape(apiID)+"/deployments?gatewayId="+url.QueryEscape(gatewayID), nil)
	if err != nil {
		return nil, retry.Transient(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, retry.Transient(ResponseError(resp))
	}
	items, err := ListItems(resp)
	if err != nil {
		return nil, retry.Transient(fmt.Errorf("decoding APIP deployments: %w", err))
	}
	for _, item := range items {
		if item["deploymentId"] == deploymentID {
			return item, nil
		}
	}
	return nil, nil
}

// DeploymentStatus reads the status of the deployment a ref points at; it is empty when the list
// does not contain that deployment.
func (s *Steps) DeploymentStatus(ctx context.Context, ref APIRef) (string, error) {
	record, err := s.DeploymentRecord(ctx, ref)
	if err != nil || record == nil {
		return "", err
	}
	status, _ := record["status"].(string)
	return status, nil
}

// WaitDeployment waits until the deployment of an API reaches the given status.
func (s *Steps) WaitDeployment(ctx context.Context, label, status string) error {
	ref := APIRefFor(label)
	last, err := retry.Until(ctx, retry.Options{Timeout: 3 * time.Minute, Interval: 5 * time.Second},
		func(ctx context.Context) (string, error) { return s.DeploymentStatus(ctx, ref) },
		func(got string) bool { return got == status })
	if err != nil {
		return fmt.Errorf("waiting for the %s deployment to be %s: %w", label, status, err)
	}
	if last != status {
		return fmt.Errorf("the %s deployment never reached %s; last status %q", label, status, last)
	}
	return nil
}

// credentialKeys maps each credential a gateway step can send to its scenario-state key.
var credentialKeys = map[string]string{
	"API key":                              KeyAPIKey,
	"previous API key":                     KeyPreviousAPIKey,
	"subscription token":                   KeyConsumerSubscriptionToken,
	"previous subscription token":          KeyPreviousSubscriptionToken,
	"access token":                         KeyAccessToken,
	"access token with a forged signature": KeyAccessToken,
}

// SendWithCredentialUntilStatus invokes a data-plane URL with a stored credential in the named
// header. An access token is sent as a bearer token.
func (s *Steps) SendWithCredentialUntilStatus(ctx context.Context, method, target, header, which string, want int) error {
	stateKey, ok := credentialKeys[which]
	if !ok {
		return fmt.Errorf("unsupported credential %q", which)
	}
	key, err := RequiredValue(ctx, stateKey)
	if err != nil {
		return err
	}
	if stateKey == KeyAccessToken {
		if which == "access token with a forged signature" {
			if key, err = forgeSignature(key); err != nil {
				return err
			}
		}
		key = "Bearer " + key
	}
	header = strings.TrimSpace(header)
	if header == "" {
		return fmt.Errorf("sending request: header name is required")
	}
	return s.sendUntilStatusWith(ctx, method, target, want, map[string]string{header: key})
}

// forgeSignature returns the JWT with its signature changed.
func forgeSignature(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[2] == "" {
		return "", fmt.Errorf("the access token is not a signed JWT")
	}
	signature := []byte(parts[2])
	if signature[0] == 'A' {
		signature[0] = 'B'
	} else {
		signature[0] = 'A'
	}
	parts[2] = string(signature)
	return strings.Join(parts, "."), nil
}
