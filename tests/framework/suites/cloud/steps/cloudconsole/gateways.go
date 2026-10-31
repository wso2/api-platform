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
	"reflect"
	"strings"

	"github.com/wso2/api-platform/tests/framework/core/util/retry"
)

const (
	// maxGatewayHandleLength is the width of the platform's gateway handle column; a managed
	// gateway is registered as "<environment>-<handle>", so the environment's name uses part of it.
	maxGatewayHandleLength = 40
)

// gatewayHandleBudget is how many characters a handle may use in an environment.
func gatewayHandleBudget(environment string) int {
	if budget := maxGatewayHandleLength - len(environment) - 1; budget > 1 {
		return budget
	}
	return 1
}

// handleOfLength builds a handle of exactly the given length from a unique name: it keeps the
// name's tail, which is what makes it unique, and pads short names with a letter.
func handleOfLength(name string, length int) string {
	if length < 1 {
		return ""
	}
	if len(name) >= length {
		name = name[len(name)-length:]
	} else {
		name += strings.Repeat("x", length-len(name))
	}
	if name[0] == '-' {
		name = "a" + name[1:]
	}
	return name
}

// fetchGatewayList reads the managed gateways, narrowed to one kind when given.
func (s *Steps) fetchGatewayList(ctx context.Context, kind string) ([]map[string]any, error) {
	path := "/managed-gateways"
	if kind != "" {
		path += "?functionalityType=" + url.QueryEscape(kind)
	}
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, retry.Transient(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, retry.Transient(responseError(resp))
	}
	items, err := listItems(resp)
	if err != nil {
		return nil, retry.Transient(fmt.Errorf("decoding managed gateways: %w", err))
	}
	return items, nil
}

// sameValue compares a value read from JSON with the value a table cell stands for.
func sameValue(got, want any) bool {
	if reflect.DeepEqual(got, want) {
		return true
	}
	left, leftErr := json.Marshal(got)
	right, rightErr := json.Marshal(want)
	return leftErr == nil && rightErr == nil && string(left) == string(right)
}
