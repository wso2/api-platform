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
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	corecatalog "github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
)

const (
	// platformAPIRoute is where the control-plane gateway publishes the APIP platform API, and
	// coreAPIRoute where it publishes the WSO2 Cloud platform API next to it. The APIP platform
	// API closes the project delete ("Projects are deleted in the WSO2 Cloud console"), so a
	// synthetic project is deleted through the cloud API the console itself calls.
	platformAPIRoute = "/apip-platform-api-apip-platform-api-endpoint/api/v0.9"
	coreAPIRoute     = "/platform-api-service-platform-api-endpoint/api/v1"

	// projectDeleteTimeout bounds how long a project delete is retried when the cloud API
	// cannot be reached or answers with a server error.
	projectDeleteTimeout = 5 * time.Minute
)

// projectDeleteInterval is the pause between retries of a project delete.
var projectDeleteInterval = 5 * time.Second

// coreProjectsBase derives the cloud API's base from the APIP platform API's base, which carries
// the same host and token audience.
func coreProjectsBase(platformAPIBase string) (string, error) {
	trimmed := strings.TrimRight(platformAPIBase, "/")
	if !strings.HasSuffix(trimmed, platformAPIRoute) {
		return "", fmt.Errorf("the platform API endpoint does not end in %s, so the cloud API's project route cannot be derived from it", platformAPIRoute)
	}
	return strings.TrimSuffix(trimmed, platformAPIRoute) + coreAPIRoute, nil
}

// projectDeleter deletes the projects a scenario registered through the cloud API.
func projectDeleter(topo *frameworkruntime.Topology, client *httpx.Client) cleanup.Deleter {
	return func(ctx context.Context, resource cleanup.Resource) error {
		base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
		if err != nil {
			return err
		}
		bearer, err := bearerToken(ctx)
		if err != nil {
			return err
		}
		return deleteProjectThroughCloudAPI(ctx, client, base, bearer, resource.ID, projectDeleteTimeout)
	}
}

// deleteProjectThroughCloudAPI asks the cloud API to delete one project and does not wait for the
// project to be gone: the request is accepted at once and the project's removal, and its record
// in the APIP platform API, are the cloud's concern rather than the APIP's. An absent project
// counts as deleted. A delete the cloud API refuses or cannot take within timeout is
// logged and not reported as a failure, so it never fails a scenario or holds up the next runner.
func deleteProjectThroughCloudAPI(ctx context.Context, client *httpx.Client, platformAPIBase, bearer, id string, timeout time.Duration) error {
	logger := slog.Default()
	base, err := coreProjectsBase(platformAPIBase)
	if err != nil {
		logger.Warn("a synthetic project was left for the cloud console to delete", "project", id, "error", err)
		return nil
	}
	target := base + "/projects/" + url.PathEscape(id)
	err = retry.Await(ctx, retry.Options{Timeout: timeout, Interval: projectDeleteInterval},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, err := client.Do(ctx, httpx.Request{
				Method:  http.MethodDelete,
				URL:     target,
				Headers: map[string]string{"Authorization": "Bearer " + bearer},
			}, 0, 0)
			if err != nil {
				return nil, retry.Transient(err)
			}
			switch {
			case resp.StatusCode == http.StatusNotFound || resp.Succeeded():
				return resp, nil
			case resp.StatusCode >= http.StatusInternalServerError || resp.StatusCode == http.StatusRequestTimeout ||
				resp.StatusCode == http.StatusTooManyRequests:
				return nil, retry.Transient(responseError(resp))
			default:
				// A refusal will not change by asking again.
				return nil, responseError(resp)
			}
		},
		func(resp *httpx.Response) bool { return resp != nil },
		"deleting project "+id)
	if err != nil {
		logger.Warn("a synthetic project was left for the cloud console to delete", "project", id, "error", err)
	}
	return nil
}
