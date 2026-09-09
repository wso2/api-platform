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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

func (s *Steps) resourceCreationSucceeded(ctx context.Context) error {
	version, err := s.topo.ComponentVersion("platform-gateway")
	if err != nil {
		return err
	}
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	_, err = assertSuccessfulAPIResponse(response, version, "creation")
	return err
}

func (s *Steps) apiUpdateSucceeded(ctx context.Context) error {
	version, err := s.topo.ComponentVersion("platform-gateway")
	if err != nil {
		return err
	}
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	_, err = assertSuccessfulAPIResponse(response, version, "update")
	return err
}

func (s *Steps) apiRetrievalSucceeded(ctx context.Context, name, apiContext string) error {
	version, err := s.topo.ComponentVersion("platform-gateway")
	if err != nil {
		return err
	}
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	expectedName, err := stepscommon.Expand(ctx, name)
	if err != nil {
		return fmt.Errorf("expanding expected API name: %w", err)
	}
	expectedContext, err := stepscommon.Expand(ctx, apiContext)
	if err != nil {
		return fmt.Errorf("expanding expected API context: %w", err)
	}
	document, err := assertSuccessfulAPIResponse(response, version, "retrieval")
	if err != nil {
		return err
	}
	return assertRetrievedAPIDetails(document, version, expectedName, expectedContext, response)
}

func assertSuccessfulAPIResponse(response *httpx.Response, version, operation string) (map[string]any, error) {
	if response == nil {
		return nil, fmt.Errorf("gateway API %s response is missing", operation)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("gateway API %s was not successful for %s: %s", operation, version, response.Describe())
	}

	var document map[string]any
	if err := json.Unmarshal(response.Body, &document); err != nil {
		return nil, fmt.Errorf("gateway API %s response for %s is not valid JSON: %w", operation, version, err)
	}
	status, ok := document["status"]
	if !ok {
		return nil, fmt.Errorf("gateway API %s response for %s has no status: %s", operation, version, response.Describe())
	}

	if usesResourceStatus(version) {
		if err := assertResourceStatus(status, version, operation, response); err != nil {
			return nil, err
		}
	} else if statusString, ok := status.(string); !ok || statusString != "success" {
		return nil, fmt.Errorf("gateway API %s response for %s has status %v, want success: %s",
			operation, version, status, response.Describe())
	}
	return document, nil
}

func usesResourceStatus(version string) bool {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	parts := strings.SplitN(version, ".", 3)
	return len(parts) >= 2 && parts[0] == "1" && parts[1] == "2"
}

func assertResourceStatus(value any, version, operation string, response *httpx.Response) error {
	status, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("gateway API %s response for %s has status %T, want resource status: %s",
			operation, version, value, response.Describe())
	}
	for _, field := range []string{"id", "state", "createdAt", "updatedAt"} {
		text, ok := status[field].(string)
		if !ok || strings.TrimSpace(text) == "" {
			return fmt.Errorf("gateway API %s response for %s has invalid status.%s: %v: %s",
				operation, version, field, status[field], response.Describe())
		}
	}
	if status["state"] != "deployed" {
		return fmt.Errorf("gateway API %s response for %s has status.state %v, want deployed: %s",
			operation, version, status["state"], response.Describe())
	}
	return nil
}

func assertRetrievedAPIDetails(document map[string]any, version, expectedName, expectedContext string,
	response *httpx.Response,
) error {
	if got := nestedString(document, "metadata", "name"); got != expectedName {
		return fmt.Errorf("gateway API retrieval for %s returned metadata.name %q, want %q: %s",
			version, got, expectedName, response.Describe())
	}
	if got := nestedString(document, "spec", "context"); got != expectedContext {
		return fmt.Errorf("gateway API retrieval for %s returned spec.context %q, want %q: %s",
			version, got, expectedContext, response.Describe())
	}
	if got := nestedString(document, "kind"); got != "RestApi" {
		return fmt.Errorf("gateway API retrieval for %s returned kind %q, want %q: %s",
			version, got, "RestApi", response.Describe())
	}
	return nil
}

func nestedString(document map[string]any, path ...string) string {
	var value any = document
	for _, key := range path {
		object, ok := value.(map[string]any)
		if !ok {
			return ""
		}
		value = object[key]
	}
	text, _ := value.(string)
	return text
}
