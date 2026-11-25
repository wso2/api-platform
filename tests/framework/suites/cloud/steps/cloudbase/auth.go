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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	corecatalog "github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
)

// APIURL resolves a path below the cloud API base URL.
func (s *Steps) APIURL(path string) (string, error) {
	if !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("cloud API path %q must start with /", path)
	}
	base, err := s.Topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	if err != nil {
		return "", err
	}
	return CloudAPIURL(base, path), nil
}

// BearerHeaders returns the headers that carry the scenario's console token.
func BearerHeaders(ctx context.Context) (map[string]string, error) {
	token, err := BearerToken(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]string{"Authorization": "Bearer " + token}, nil
}

// SameJSON compares two bodies as JSON documents, falling back to the raw bytes.
func SameJSON(left, right []byte) bool {
	var l, r any
	if json.Unmarshal(left, &l) != nil || json.Unmarshal(right, &r) != nil {
		return bytes.Equal(left, right)
	}
	return reflect.DeepEqual(l, r)
}
