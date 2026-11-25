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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

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
	ComponentName           = corecatalog.Name
	keyToken                = "cloudConsoleToken"
	KeyProjectID            = "cloudProjectID"
	KeyGatewayID            = "cloudGatewayID"
	KeyGatewayEndpoint      = "cloudGatewayEndpoint"
	KeyGatewayEnv           = "cloudGatewayEnvironment"
	KeyAPIID                = "cloudAPIID"
	KeyDeploymentID         = "cloudDeploymentID"
	KeyEnvironmentName      = "cloudEnvironmentName"
	KeyAPIDefinition        = "cloudAPIDefinition"
	KeyAPIKey               = "cloudAPIKey"
	KeyRejectedAPIID        = "cloudRejectedAPIID"
	KeyVersionAPIID         = "cloudVersionAPIID"
	keyVersionDeploymentID  = "cloudVersionDeploymentID"
	keyVersionAPIDefinition = "cloudVersionAPIDefinition"
	KeyBuildID              = "cloudBuildID"
	KeyBuildIDs             = "cloudBuildIDs"
	KeyBuildsOwner          = "cloudBuildsOwner"
	KeyBuildMetadata        = "cloudBuildMetadata"
	// KeyBuildMetadataOf names the build the stored metadata was prepared with.
	KeyBuildMetadataOf         = "cloudBuildMetadataBuildID"
	KeyDeployedBuildID         = "cloudDeployedBuildID"
	keyGatewayDeleteStatus     = "cloudGatewayDeleteStatus"
	KeyDeployedTargets         = "cloudDeployedTargets"
	CloudConsoleAccessTokenEnv = "CLOUD_CONSOLE_ACCESS_TOKEN"
	CloudGatewayPinEnv         = "CLOUD_GATEWAY_ID"
	ResourceActor              = "apip-console"
)

// ManagedGatewaysCollection is the collection a managed gateway is listed in and deleted from.
const ManagedGatewaysCollection = "/managed-gateways"

var configureCloudExpansion sync.Once

// ExpandCloudValue expands the placeholders of a feature value against the scenario context.
func ExpandCloudValue(ctx context.Context, value string) (string, error) {
	configureCloudExpansion.Do(func() {
		unique.ContextValue = func(ctx context.Context, name string) (string, error) {
			return tcontext.ResolveString(ctx, name)
		}
	})
	return unique.Expand(ctx, value)
}

// Steps contains APIP cloud step bindings for one resolved block.
type Steps struct {
	Topo                 *frameworkruntime.Topology
	Funnel               *httpx.Funnel
	Client               *httpx.Client
	DataPlaneURLResolver func(context.Context, string) (string, error)
}

// StepDefinition pairs a Gherkin step pattern with its handler.
type StepDefinition struct {
	Pattern string
	Handler any
}

// StepRegistrar is the part of godog.ScenarioContext the step registration needs, so a test can
// record the registered patterns instead of running a scenario.
type StepRegistrar interface {
	Step(expr, stepFunc any)
}

// StepRecorder collects the steps registered through it.
type StepRecorder struct {
	Definitions []StepDefinition
}

// Step records a step pattern and its handler.
func (r *StepRecorder) Step(expr, stepFunc any) {
	r.Definitions = append(r.Definitions, StepDefinition{Pattern: expr.(string), Handler: stepFunc})
}

// CloudRequestDeleter sends the request toRequest builds from a resource id, and accepts the
// answers settled reports as leaving nothing behind.
func CloudRequestDeleter(
	topo *frameworkruntime.Topology, client *httpx.Client,
	toRequest func(id string) (method, path string, settled func(*httpx.Response) bool, err error),
) cleanup.Deleter {
	return func(ctx context.Context, resource cleanup.Resource) error {
		method, path, settled, err := toRequest(resource.ID)
		if err != nil {
			return err
		}
		base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
		if err != nil {
			return err
		}
		bearer, err := BearerToken(ctx)
		if err != nil {
			return err
		}
		resp, err := client.Do(ctx, httpx.Request{
			Method:  method,
			URL:     CloudAPIURL(base, path),
			Headers: map[string]string{"Authorization": "Bearer " + bearer},
		}, 0, 0)
		if err != nil {
			return err
		}
		if !settled(resp) {
			return ResponseError(resp)
		}
		return nil
	}
}

// CloudDeleter returns a cleanup deleter that deletes a resource from a collection of the cloud API.
func CloudDeleter(topo *frameworkruntime.Topology, client *httpx.Client, collection string) cleanup.Deleter {
	return func(ctx context.Context, resource cleanup.Resource) error {
		base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
		if err != nil {
			return err
		}
		bearer, err := BearerToken(ctx)
		if err != nil {
			return err
		}
		return DeleteCloudResource(ctx, client, base, bearer, collection, resource.ID)
	}
}

// DeleteCloudResource deletes one resource from a collection and treats an absent one as deleted.
func DeleteCloudResource(ctx context.Context, client *httpx.Client, base, bearer, collection, id string) error {
	resp, err := client.Do(ctx, httpx.Request{
		Method: http.MethodDelete,
		URL:    cloudResourceURL(base, collection, id),
		Headers: map[string]string{
			"Authorization": "Bearer " + bearer,
		},
	}, 0, 0)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound || resp.Succeeded() {
		return nil
	}
	return ResponseError(resp)
}

// BearerToken returns the console token of the run.
func BearerToken(ctx context.Context) (string, error) {
	if token, err := tcontext.ResolveString(ctx, keyToken); err == nil && strings.TrimSpace(token) != "" {
		return token, nil
	}
	token := strings.TrimSpace(os.Getenv(CloudConsoleAccessTokenEnv))
	if token == "" {
		return "", fmt.Errorf("environment variable %s is required", CloudConsoleAccessTokenEnv)
	}
	if err := tcontext.Set(ctx, keyToken, token); err != nil {
		return "", err
	}
	return token, nil
}

// ResponseError turns an unexpected cloud API response into an error.
func ResponseError(resp *httpx.Response) error {
	if resp == nil {
		return errors.New("cloud-console: no response")
	}
	var body struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(resp.Body, &body)
	return &StatusError{Status: resp.StatusCode, Code: body.Code, Message: "cloud-console: unexpected response: " + resp.Describe()}
}

// StatusError is an answer a caller did not expect. It keeps the status, so a caller can tell a
// resource that is absent from one the token may not touch.
type StatusError struct {
	Status int
	// code is the error code the platform's answer carries, when it carries one.
	Code    string
	Message string
}

// Error returns the message of the refused response.
func (e *StatusError) Error() string { return e.Message }

// errorStatus returns the status of the answer behind err, or zero when err is not one.
func errorStatus(err error) int {
	var answer *StatusError
	if errors.As(err, &answer) {
		return answer.Status
	}
	return 0
}

// CloudAPIURL resolves a path below the cloud API base URL.
func CloudAPIURL(base, path string) string {
	return strings.TrimRight(base, "/") + path
}

func cloudResourceURL(base, collection, id string) string {
	return CloudAPIURL(base, collection+"/"+url.PathEscape(id))
}

// PropagationTimeout returns how long a change may take to show up in the cloud API.
func PropagationTimeout(topo *frameworkruntime.Topology) time.Duration {
	if topo != nil && topo.PropagationTimeout > 0 {
		return topo.PropagationTimeout
	}
	return retry.PropagationCeiling
}
