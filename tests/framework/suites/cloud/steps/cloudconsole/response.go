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
	"sort"
	"strings"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
)

// documentPath reads a dotted path of a decoded JSON document.
func documentPath(document any, path string) (any, bool) {
	current := document
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		if current, ok = object[part]; !ok {
			return nil, false
		}
	}
	return current, true
}

// assertResponseReports checks fields of the published response against a table of dotted paths
// and expected values.
func (s *Steps) assertResponseReports(ctx context.Context, table *godog.Table) error {
	values, err := tableValues(table)
	if err != nil {
		return err
	}
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	var document any
	if err := json.Unmarshal(response.Body, &document); err != nil {
		return fmt.Errorf("the response is not JSON: %w (%s)", err, response.Describe())
	}
	var differing []string
	for path, raw := range values {
		want, err := expandCloudValue(ctx, raw)
		if err != nil {
			return fmt.Errorf("expanding %q: %w", path, err)
		}
		got, present := documentPath(document, path)
		if !present || !sameValue(got, parseConfigCell(want)) {
			differing = append(differing, fmt.Sprintf("%s is %v, want %s", path, got, want))
		}
	}
	sort.Strings(differing)
	if len(differing) > 0 {
		return fmt.Errorf("the response does not report the expected fields: %s", strings.Join(differing, "; "))
	}
	return nil
}
