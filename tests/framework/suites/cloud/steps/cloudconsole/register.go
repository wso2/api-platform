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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"

	corecatalog "github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
)

const (
	// ComponentName is the catalog component used by APIP cloud runners.
	ComponentName = corecatalog.Name

	keyToken            = "cloudConsoleToken"
	keyProjectID        = "cloudProjectID"
	keyGatewayID        = "cloudGatewayID"
	keyGatewayEndpoint  = "cloudGatewayEndpoint"
	keyAPIID            = "cloudAPIID"
	keyDeploymentID     = "cloudDeploymentID"
	keyEnvironmentName  = "cloudEnvironmentName"
	keyManagedGatewayID = "cloudManagedGatewayID"

	cloudConsoleAccessTokenEnv = "CLOUD_CONSOLE_ACCESS_TOKEN"

	resourceActor = "apip-console"
)

var configureCloudExpansion sync.Once

func expandCloudValue(ctx context.Context, value string) (string, error) {
	configureCloudExpansion.Do(func() {
		unique.ContextValue = func(ctx context.Context, name string) (string, error) {
			return tcontext.ResolveString(ctx, name)
		}
	})
	return unique.Expand(ctx, value)
}

// Steps contains APIP cloud step bindings for one resolved block.
type Steps struct {
	topo                 *frameworkruntime.Topology
	funnel               *httpx.Funnel
	client               *httpx.Client
	dataPlaneURLResolver func(context.Context, string) (string, error)
}

// Register binds APIP cloud-specific Gherkin steps.
func Register(sc *godog.ScenarioContext, topo *frameworkruntime.Topology, funnel *httpx.Funnel) {
	s := &Steps{topo: topo, funnel: funnel, client: funnel.Client()}
	sc.Step(`^I obtain an APIP cloud console token$`, s.obtainToken)
	sc.Step(`^I find the default APIP project$`, s.findDefaultProject)
	sc.Step(`^I find an active APIP gateway$`, s.findActiveGateway)
	sc.Step(`^I create a synthetic REST API in the default project$`, s.createAPI)
	sc.Step(`^I deploy the synthetic REST API to the active gateway$`, s.deployAPI)
	sc.Step(`^the synthetic REST API deployment should become DEPLOYED$`, s.waitDeployment)
	sc.Step(`^I send a "([^"]*)" request to "([^"]*)" until status (\d+)$`, s.sendUntilStatus)
	sc.Step(`^the synthetic REST API data-plane response should contain post 1$`, s.assertAPIResponse)
	sc.Step(`^I delete the synthetic REST API$`, s.deleteAPI)
	sc.Step(`^the synthetic REST API should eventually be absent$`, s.verifyAPIDeleted)

	sc.Step(`^I create a synthetic APIP project$`, s.createProject)
	sc.Step(`^the synthetic APIP project should be retrievable by handle$`, s.getProject)
	sc.Step(`^I delete the synthetic APIP project$`, s.deleteProject)
	sc.Step(`^the synthetic APIP project should eventually be absent$`, s.verifyProjectDeleted)

	sc.Step(`^I create a synthetic non-production APIP environment$`, s.createEnvironment)
	sc.Step(`^creating the same APIP environment again should return conflict$`, s.duplicateEnvironment)
	sc.Step(`^I delete the synthetic APIP environment$`, s.deleteEnvironment)
	sc.Step(`^deleting the synthetic APIP environment should eventually return not found$`, s.verifyEnvironmentDeleted)

	sc.Step(`^I select an existing APIP environment for a managed gateway$`, s.selectGatewayEnvironment)
	sc.Step(`^I create a synthetic managed APIP gateway$`, s.createManagedGateway)
	sc.Step(`^the managed APIP gateway should be retrievable through the gateways endpoint$`, s.getManagedGateway)
	sc.Step(`^I delete the synthetic managed APIP gateway$`, s.deleteManagedGateway)
	sc.Step(`^the managed APIP gateway should eventually be absent$`, s.verifyManagedGatewayDeleted)
}

// RegisterDeleters installs cloud-target cleanup handlers for a block containing cloud-console.
func RegisterDeleters(reg *cleanup.Registry, topo *frameworkruntime.Topology) error {
	if reg == nil {
		return fmt.Errorf("cloud-console: cleanup registry is required")
	}
	if topo == nil {
		return fmt.Errorf("cloud-console: topology is required")
	}
	if _, err := topo.Component(corecatalog.Name); err != nil {
		return nil
	}
	client := httpx.NewClient(httpx.Options{MaxRetries: 1})
	if err := reg.RegisterDeleter(cleanup.KindAPI, cloudDeleter(topo, client, "/rest-apis", false)); err != nil {
		return err
	}
	if err := reg.RegisterDeleter(cleanup.KindProject, cloudDeleter(topo, client, "/projects", false)); err != nil {
		return err
	}
	if err := reg.RegisterDeleter(cleanup.KindEnvironment, cloudDeleter(topo, client, "/environments", true)); err != nil {
		return err
	}
	return reg.RegisterDeleter(cleanup.KindGateway, cloudDeleter(topo, client, "/managed-gateways", false))
}

func cloudDeleter(topo *frameworkruntime.Topology, client *httpx.Client, collection string, waitForNotFound bool) cleanup.Deleter {
	return func(ctx context.Context, resource cleanup.Resource) error {
		base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPBML)
		if err != nil {
			return err
		}
		bearer, err := bearerToken(ctx)
		if err != nil {
			return err
		}
		deleteOne := func(ctx context.Context) (*httpx.Response, error) {
			return client.Do(ctx, httpx.Request{
				Method: http.MethodDelete,
				URL:    cloudResourceURL(base, collection, resource.ID),
				Headers: map[string]string{
					"Authorization": "Bearer " + bearer,
				},
			}, 0, 0)
		}
		if !waitForNotFound {
			resp, err := deleteOne(ctx)
			if err != nil {
				return err
			}
			if resp.StatusCode == http.StatusNotFound || resp.Succeeded() {
				return nil
			}
			return responseError(resp)
		}
		return retry.Await(ctx, retry.Options{Timeout: 2 * time.Minute, Interval: 3 * time.Second},
			func(ctx context.Context) (*httpx.Response, error) {
				resp, err := deleteOne(ctx)
				if err != nil {
					return nil, retry.Transient(err)
				}
				if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusConflict {
					return nil, responseError(resp)
				}
				return resp, nil
			},
			func(resp *httpx.Response) bool { return resp != nil && resp.StatusCode == http.StatusNotFound },
			"waiting for environment "+resource.ID+" to be deleted")
	}
}

func bearerToken(ctx context.Context) (string, error) {
	if token, err := tcontext.ResolveString(ctx, keyToken); err == nil && strings.TrimSpace(token) != "" {
		return token, nil
	}
	token := strings.TrimSpace(os.Getenv(cloudConsoleAccessTokenEnv))
	if token == "" {
		return "", fmt.Errorf("environment variable %s is required", cloudConsoleAccessTokenEnv)
	}
	if err := tcontext.Set(ctx, keyToken, token); err != nil {
		return "", err
	}
	return token, nil
}

func responseError(resp *httpx.Response) error {
	if resp == nil {
		return errors.New("cloud-console: no response")
	}
	return fmt.Errorf("cloud-console: unexpected response: %s", resp.Describe())
}

func cloudAPIURL(base, path string) string {
	return strings.TrimRight(base, "/") + path
}

func cloudResourceURL(base, collection, id string) string {
	return cloudAPIURL(base, collection+"/"+url.PathEscape(id))
}

func propagationTimeout(topo *frameworkruntime.Topology) time.Duration {
	if topo != nil && topo.PropagationTimeout > 0 {
		return topo.PropagationTimeout
	}
	return retry.PropagationCeiling
}
