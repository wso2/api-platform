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
	"net/http"
	"net/url"
	"strings"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
)

// The organisation is shared with other consoles, so the suite selects an environment only if it is
// the organisation's default one or its name starts with testResourcePrefix, and a deployment
// pipeline or a project only if its name starts with it (apip-test-environment,
// apip-test-pipeline-1, apip-test-project-1, ...). It never uses the default pipeline or the default
// project. Gateways and portals are not selected by name: the suite finds or builds them under any
// name.
const (
	testResourcePrefix     = "apip-test-"
	defaultEnvironmentName = "development"
)

// hasSuitePrefix reports whether a resource name starts with the suite's prefix.
func hasSuitePrefix(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.HasPrefix(name, testResourcePrefix) && len(name) > len(testResourcePrefix)
}

// ownedEnvironment reports whether the suite may select an environment: the default one, or one
// named with the suite's prefix.
func ownedEnvironment(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), defaultEnvironmentName) || hasSuitePrefix(name)
}

// ownedPipeline reports whether the suite may select a deployment pipeline: one named with the
// suite's prefix.
func ownedPipeline(name string) bool { return hasSuitePrefix(name) }

// minGatewayHandleBudget is the handle length an environment must allow for the suite's ordinary
// gateway names: syntheticGatewayBase and the 11 characters the unique generator adds. The
// platform limits a handle to 37 minus twice the environment name's length, so a long environment
// name (apip-test-environment allows 1 character) cannot host the gateways the scenarios build.
const minGatewayHandleBudget = len(syntheticGatewayBase) + 11

// gatewayHandleBudgetIn asks the platform how many characters a gateway handle may use in an
// environment: it requests a gateway whose handle is far too long, which the platform refuses before
// building anything, and reads the limit from the refusal. A request the platform does accept is
// deleted again and reported as an error.
func gatewayHandleBudgetIn(ctx context.Context, environment string,
	do func(ctx context.Context, method, path string, body []byte) (*httpx.Response, error)) (int, error) {
	body, err := json.Marshal(map[string]string{
		"displayName": "handle limit probe",
		"environment": environment,
		"handle":      strings.Repeat("a", handleProbeLength),
	})
	if err != nil {
		return 0, err
	}
	resp, err := do(ctx, http.MethodPost, "/managed-gateways", body)
	if err != nil {
		return 0, err
	}
	if gatewayRequestAccepted(resp.StatusCode) {
		if id, idErr := stringField(resp, "id"); idErr == nil {
			_, _ = do(ctx, http.MethodDelete, "/managed-gateways/"+url.PathEscape(id), nil)
		}
		return 0, fmt.Errorf("a %d-character handle was accepted in environment %q", handleProbeLength, environment)
	}
	if resp.StatusCode != http.StatusBadRequest {
		return 0, fmt.Errorf("probing the handle limit of environment %q: %w", environment, responseError(resp))
	}
	return handleBudgetOf(resp.Body)
}

// hostsSuiteGateways reports whether an environment allows the suite's ordinary gateway names. An
// environment whose limit cannot be read is reported as not hosting them.
func hostsSuiteGateways(ctx context.Context,
	do func(ctx context.Context, method, path string, body []byte) (*httpx.Response, error)) func(string) bool {
	return func(environment string) bool {
		budget, err := gatewayHandleBudgetIn(ctx, environment, do)
		return err == nil && budget >= minGatewayHandleBudget
	}
}
