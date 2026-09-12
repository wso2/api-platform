/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package platformapi holds steps for both directions of the platform-api <-> gateway
// relationship: control_plane.go asserts against artifacts a gateway-controller pushes up to
// the control plane, and secret_deploy.go creates and deploys artifacts through the control
// plane's own REST API for the gateway to pull down.
package platformapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/actor"
	controlplane "github.com/wso2/api-platform/tests/framework/core/catalog/platformapi"
	"github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// apiBase is the control plane's versioned management API, matching what
// core/catalog/platformapi's provisioner authenticates against.
const apiBase = "/api/v0.9"

// artifactPaths maps a gateway artifact kind to its control-plane resource collection.
var artifactPaths = map[string]string{
	"LlmProviderTemplate": "llm-provider-templates",
	"LlmProvider":         "llm-providers",
	"LlmProxy":            "llm-proxies",
	"Mcp":                 "mcp-proxies",
	"RestApi":             "rest-apis",
}

// Steps holds what the control-plane steps need.
type Steps struct {
	topo   *runtime.Topology
	client *httpx.Client
}

// Register binds the control-plane assertion steps used by DP -> CP sync features. Requests
// go through the same httpx.Client every other suite request funnels through - client is the
// underlying client of the suite's shared funnel, not a step-owned http.Client.
func Register(sc *godog.ScenarioContext, topo *runtime.Topology, client *httpx.Client) {
	s := &Steps{topo: topo, client: client}
	sc.Step(`^the control plane should receive the "([^"]*)" artifact "([^"]*)"$`, s.shouldReceive)
	sc.Step(`^the control plane should not receive the "([^"]*)" artifact "([^"]*)"$`, s.shouldNotReceive)
	sc.Step(`^the control plane copy of the "([^"]*)" artifact "([^"]*)" configuration should contain "([^"]*)"$`,
		s.configurationShouldContain)
	sc.Step(`^the control plane copy of the "([^"]*)" artifact "([^"]*)" should be marked as gateway-originated$`,
		s.shouldBeGatewayOriginated)
	sc.Step(`^the control plane copy of the "LlmProvider" artifact "([^"]*)" should reference template "([^"]*)"$`,
		s.providerShouldReferenceTemplate)
	sc.Step(`^the control plane copy of the "LlmProxy" artifact "([^"]*)" should reference provider "([^"]*)"$`,
		s.proxyShouldReferenceProvider)
	sc.Step(`^the control plane should have deployed the "Mcp" artifact "([^"]*)"$`,
		func(ctx context.Context, name string) error { return s.mcpDeploymentStatus(ctx, name, "DEPLOYED") })
	sc.Step(`^the control plane should have undeployed the "Mcp" artifact "([^"]*)"$`,
		func(ctx context.Context, name string) error { return s.mcpDeploymentStatus(ctx, name, "UNDEPLOYED") })
	sc.Step(`^I create a project "([^"]*)" on the control plane$`, s.createProject)
	RegisterDeploy(sc, s)
}

// baseURL resolves the control plane's HTTPS base URL for this block.
func (s *Steps) baseURL() (string, error) {
	return s.topo.URL("platform-api", "https")
}

// bearer logs in as the fixed test administrator, exactly as the platform-api catalog
// component's own provisioner does, so a step and the provisioner can never disagree on
// how to authenticate against a running control plane.
func (s *Steps) bearer(ctx context.Context, base string) (string, error) {
	admin := actor.Administrator()
	return controlplane.ControlPlaneLogin(ctx, base, admin.Username, admin.Password)
}

// get issues one authenticated GET against the control plane's management API. Any status
// code is a valid result; only a transport failure is a retryable error, so 404 can be
// polled for as legitimately as 200.
func (s *Steps) get(ctx context.Context, base, bearer, path string) (*httpx.Response, error) {
	resp, err := s.client.Do(ctx, httpx.Request{
		Method:  http.MethodGet,
		URL:     base + apiBase + path,
		Headers: map[string]string{"Authorization": "Bearer " + bearer},
	}, 0, 0)
	if err != nil {
		return nil, retry.Transient(err)
	}
	return resp, nil
}

// artifactPath resolves a gateway artifact kind to its control-plane collection path,
// keyed by the same handle the gateway pushed - platform-api matches gateway-originated
// artifacts by handle, never by a data-plane UUID (see internal/service/artifact_import.go).
func artifactPath(kind, handle string) (string, error) {
	collection, ok := artifactPaths[kind]
	if !ok {
		return "", fmt.Errorf("unknown DP -> CP artifact kind %q", kind)
	}
	return "/" + collection + "/" + handle, nil
}

// awaitArtifact polls the control plane for one artifact until accept holds. The push is
// asynchronous (an ADS-driven background sync, not a call the DP request waits on), so every
// control-plane assertion needs its own wait rather than assuming an earlier step's timing.
func (s *Steps) awaitArtifact(
	ctx context.Context, kind, name, what string, accept func(*httpx.Response) bool,
) error {
	resolvedKind, err := stepscommon.Expand(ctx, kind)
	if err != nil {
		return err
	}
	resolvedName, err := stepscommon.Expand(ctx, name)
	if err != nil {
		return err
	}
	path, err := artifactPath(resolvedKind, resolvedName)
	if err != nil {
		return err
	}
	base, err := s.baseURL()
	if err != nil {
		return err
	}
	bearer, err := s.bearer(ctx, base)
	if err != nil {
		return fmt.Errorf("authenticating to the control plane: %w", err)
	}

	return retry.Await(ctx, retry.Options{},
		func(ctx context.Context) (*httpx.Response, error) { return s.get(ctx, base, bearer, path) },
		accept, what)
}

func (s *Steps) shouldReceive(ctx context.Context, kind, name string) error {
	return s.awaitArtifact(ctx, kind, name,
		fmt.Sprintf("waiting for the control plane to receive the %s artifact %q", kind, name),
		func(r *httpx.Response) bool { return r != nil && r.StatusCode == http.StatusOK })
}

func (s *Steps) shouldNotReceive(ctx context.Context, kind, name string) error {
	resolvedKind, err := stepscommon.Expand(ctx, kind)
	if err != nil {
		return err
	}
	resolvedName, err := stepscommon.Expand(ctx, name)
	if err != nil {
		return err
	}
	path, err := artifactPath(resolvedKind, resolvedName)
	if err != nil {
		return err
	}
	base, err := s.baseURL()
	if err != nil {
		return err
	}
	bearer, err := s.bearer(ctx, base)
	if err != nil {
		return fmt.Errorf("authenticating to the control plane: %w", err)
	}

	what := fmt.Sprintf("waiting to confirm the control plane never receives the %s artifact %q", kind, name)
	settled, err := retry.SettledCount(ctx, retry.Options{Timeout: 12 * time.Second}, 3*time.Second,
		func(ctx context.Context) (int, error) {
			resp, getErr := s.get(ctx, base, bearer, path)
			if getErr != nil {
				return 0, getErr
			}
			if resp != nil && resp.StatusCode == http.StatusNotFound {
				return 0, nil
			}
			return 1, nil
		})
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	if !settled.Quiet || settled.Value != 0 {
		return fmt.Errorf("%s: artifact was observed during the quiet period", what)
	}
	return nil
}

func (s *Steps) configurationShouldContain(ctx context.Context, kind, name, want string) error {
	resolvedWant, err := stepscommon.Expand(ctx, want)
	if err != nil {
		return err
	}
	return s.awaitArtifact(ctx, kind, name,
		fmt.Sprintf("waiting for the control plane's copy of the %s artifact %q to contain %q", kind, name, resolvedWant),
		func(r *httpx.Response) bool {
			return r != nil && r.StatusCode == http.StatusOK && strings.Contains(r.Text(), resolvedWant)
		})
}

// shouldBeGatewayOriginated asserts the control plane marks the pushed artifact read-only,
// which is how it records "this came from a data-plane gateway, not the control plane
// itself" (see the readOnly doc comment on every artifact DTO in platform-api/api).
func (s *Steps) shouldBeGatewayOriginated(ctx context.Context, kind, name string) error {
	return s.awaitArtifact(ctx, kind, name,
		fmt.Sprintf("waiting for the control plane's copy of the %s artifact %q to be marked gateway-originated", kind, name),
		func(r *httpx.Response) bool {
			if r == nil || r.StatusCode != http.StatusOK {
				return false
			}
			var doc struct {
				ReadOnly bool `json:"readOnly"`
			}
			return json.Unmarshal(r.Body, &doc) == nil && doc.ReadOnly
		})
}

func (s *Steps) providerShouldReferenceTemplate(ctx context.Context, name, templateName string) error {
	resolvedTemplate, err := stepscommon.Expand(ctx, templateName)
	if err != nil {
		return err
	}
	return s.awaitArtifact(ctx, "LlmProvider", name,
		fmt.Sprintf("waiting for the control plane's copy of the LlmProvider artifact %q to reference template %q", name, resolvedTemplate),
		func(r *httpx.Response) bool {
			if r == nil || r.StatusCode != http.StatusOK {
				return false
			}
			var doc struct {
				Template string `json:"template"`
			}
			return json.Unmarshal(r.Body, &doc) == nil && doc.Template == resolvedTemplate
		})
}

func (s *Steps) proxyShouldReferenceProvider(ctx context.Context, name, providerName string) error {
	resolvedProvider, err := stepscommon.Expand(ctx, providerName)
	if err != nil {
		return err
	}
	return s.awaitArtifact(ctx, "LlmProxy", name,
		fmt.Sprintf("waiting for the control plane's copy of the LlmProxy artifact %q to reference provider %q", name, resolvedProvider),
		func(r *httpx.Response) bool {
			if r == nil || r.StatusCode != http.StatusOK {
				return false
			}
			var doc struct {
				Provider struct {
					Id string `json:"id"`
				} `json:"provider"`
			}
			return json.Unmarshal(r.Body, &doc) == nil && doc.Provider.Id == resolvedProvider
		})
}

// mcpDeploymentStatus polls the control plane's per-gateway deployment record for an MCP
// proxy. Unlike the artifact resource itself, deploy/undeploy is a lifecycle state platform-api
// tracks per gateway, not a field on the proxy - see api.DeploymentResponse.
func (s *Steps) mcpDeploymentStatus(ctx context.Context, name, want string) error {
	resolvedName, err := stepscommon.Expand(ctx, name)
	if err != nil {
		return err
	}
	base, err := s.baseURL()
	if err != nil {
		return err
	}
	bearer, err := s.bearer(ctx, base)
	if err != nil {
		return fmt.Errorf("authenticating to the control plane: %w", err)
	}
	what := fmt.Sprintf("waiting for the control plane to record the Mcp artifact %q as %s", resolvedName, want)

	return retry.Await(ctx, retry.Options{},
		func(ctx context.Context) (*httpx.Response, error) {
			return s.get(ctx, base, bearer, "/mcp-proxies/"+resolvedName+"/deployments")
		},
		func(r *httpx.Response) bool { return deploymentStatusMatches(r, want) }, what)
}

func deploymentStatusMatches(r *httpx.Response, want string) bool {
	if r == nil || r.StatusCode != http.StatusOK {
		return false
	}
	var doc struct {
		List []struct {
			Status string `json:"status"`
		} `json:"list"`
	}
	if json.Unmarshal(r.Body, &doc) != nil || len(doc.List) == 0 {
		return false
	}
	for _, deployment := range doc.List {
		if deployment.Status == want {
			return true
		}
	}
	return false
}

// createProject creates a real control-plane project, used to give a project-scoped
// artifact (MCP, RestApi, LlmProxy) a resolvable project-id annotation - or, by never being
// called, to leave one unresolvable so a push is genuinely rejected.
func (s *Steps) createProject(ctx context.Context, handle string) error {
	resolvedHandle, err := stepscommon.Expand(ctx, handle)
	if err != nil {
		return err
	}
	base, err := s.baseURL()
	if err != nil {
		return err
	}
	bearer, err := s.bearer(ctx, base)
	if err != nil {
		return fmt.Errorf("authenticating to the control plane: %w", err)
	}

	payload, err := json.Marshal(map[string]any{"id": resolvedHandle, "displayName": resolvedHandle})
	if err != nil {
		return err
	}
	resp, err := s.client.Do(ctx, httpx.Request{
		Method:      http.MethodPost,
		URL:         base + apiBase + "/projects",
		Headers:     map[string]string{"Authorization": "Bearer " + bearer},
		Body:        payload,
		ContentType: "application/json",
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("creating control-plane project %q: %w", resolvedHandle, err)
	}
	if !resp.Succeeded() {
		return fmt.Errorf("creating control-plane project %q: %s", resolvedHandle, resp.Describe())
	}
	return s.registerPlatformResource(ctx, platformProjectKind, resolvedHandle, "/projects")
}
