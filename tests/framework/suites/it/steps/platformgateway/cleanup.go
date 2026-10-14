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

package platformgateway

import (
	"context"
	"net/http"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
)

// managementCollections maps each resource kind a gateway scenario creates to the collection of
// the gateway controller's management API that deletes it.
var managementCollections = map[cleanup.Kind]string{
	cleanup.KindAPI:                 "/rest-apis",
	cleanup.KindLLMProvider:         "/llm-providers",
	cleanup.KindLLMProxy:            "/llm-proxies",
	cleanup.KindLLMProviderTemplate: "/llm-provider-templates",
	cleanup.KindMCPProxy:            "/mcp-proxies",
	cleanup.KindAgent:               "/agents",
	cleanup.KindCertificate:         certificatesCollection,
	cleanup.KindSecret:              "/secrets",
}

// RegisterDeleters registers how the cleanup sweep deletes each resource kind a gateway
// scenario creates, through the management API of the topology's platform-gateway. A resource
// that is already gone counts as deleted. On releases where the next scenario's resource could
// lose its route to the controller's snapshot race, the sweep also waits until the controller
// has applied the deletion.
func RegisterDeleters(reg *cleanup.Registry, topo *runtime.Topology) {
	client := httpx.NewClient(httpx.Options{MaxRetries: 1, InsecureSkipVerify: true})
	for kind, collection := range managementCollections {
		reg.RegisterDeleter(kind, func(ctx context.Context, res cleanup.Resource) error {
			endpoint, err := managementEndpointOf(topo)
			if err != nil {
				return err
			}
			return endpoint.delete(ctx, client, collection, res.ID)
		})
	}
}

// managementEndpoint addresses the gateway controller of one topology.
type managementEndpoint struct {
	restURL       string
	adminURL      string
	version       string
	authorization string
}

func managementEndpointOf(topo *runtime.Topology) (managementEndpoint, error) {
	rest, err := topo.URL("platform-gateway", "rest")
	if err != nil {
		return managementEndpoint{}, err
	}
	admin, err := topo.URL("platform-gateway", "admin")
	if err != nil {
		return managementEndpoint{}, err
	}
	version, err := topo.ComponentVersion("platform-gateway")
	if err != nil {
		return managementEndpoint{}, err
	}
	return managementEndpoint{
		restURL:       rest,
		adminURL:      admin,
		version:       version,
		authorization: BasicAuthHeader(topo.Admin.Username, topo.Admin.Password),
	}, nil
}

// certificatesCollection is deleted waiting out 409 Conflict: the controller refuses to delete a
// certificate while it still counts an API that was just removed as a reference, and a
// certificate left behind changes the pool the next scenario sees.
const certificatesCollection = "/certificates"

// delete removes one resource. A 404 means it is already gone, which is success for a sweep;
// any other failure is a leak signal and is reported.
func (e managementEndpoint) delete(ctx context.Context, client *httpx.Client, collection, id string) error {
	request := httpx.Request{
		Method:  http.MethodDelete,
		URL:     e.restURL + ManagementBasePathForVersion(e.version) + collection + "/" + id,
		Headers: map[string]string{"Authorization": e.authorization},
	}
	send := func(ctx context.Context) (*httpx.Response, error) { return client.Do(ctx, request, 1, 0) }
	var resp *httpx.Response
	var err error
	if collection == certificatesCollection {
		resp, err = retry.Until(ctx, retry.Options{}, send,
			func(r *httpx.Response) bool { return r != nil && r.StatusCode != http.StatusConflict })
	} else {
		resp, err = send(ctx)
	}
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNotFound && !resp.Succeeded() {
		return &deleteError{resp: resp}
	}
	return AwaitDeletionApplied(ctx, client, e.adminURL, e.version, e.authorization, id)
}

type deleteError struct{ resp *httpx.Response }

func (e *deleteError) Error() string { return e.resp.Describe() }
