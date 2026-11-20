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
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/wso2/api-platform/tests/framework/core/util/retry"
)

// HandleProbeLength is the length of the over-long handle a probe sends to learn how long a
// handle the platform accepts in an environment.
const HandleProbeLength = 60

// handleBudgetPattern finds the limit in the platform's refusal of a handle that is too long.
var handleBudgetPattern = regexp.MustCompile(`(\d+) characters or fewer`)

// HandleBudgetOf reads the number of characters a handle may use from the platform's refusal of one
// that is too long.
func HandleBudgetOf(body []byte) (int, error) {
	match := handleBudgetPattern.FindSubmatch(body)
	if match == nil {
		return 0, fmt.Errorf("the refusal does not say how long a handle may be: %s", strings.TrimSpace(string(body)))
	}
	budget, err := strconv.Atoi(string(match[1]))
	if err != nil || budget < 1 {
		return 0, fmt.Errorf("the refusal gives the handle limit as %q", match[1])
	}
	return budget, nil
}

// FetchGatewayList reads the managed gateways, narrowed to one kind when given. The list includes
// a gateway that is still being built, which a read by id does not until the gateway is bound.
func (s *Steps) FetchGatewayList(ctx context.Context, kind string) ([]map[string]any, error) {
	path := "/managed-gateways"
	if kind != "" {
		path += "?functionalityType=" + url.QueryEscape(kind)
	}
	resp, err := s.DoAuthenticatedRaw(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, retry.Transient(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, retry.Transient(ResponseError(resp))
	}
	items, err := ListItems(resp)
	if err != nil {
		return nil, retry.Transient(fmt.Errorf("decoding managed gateways: %w", err))
	}
	return items, nil
}
