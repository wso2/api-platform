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

package platformapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
)

// platformCustomPolicyKind is a custom policy synced into the organization from a gateway's
// manifest. It is deleted after the APIs, since platform-api refuses to delete a policy in use.
var platformCustomPolicyKind = cleanup.Kind{Name: "platform-api-gateway-custom-policy", Order: 60}

const (
	customPolicyCollection = "/gateway-custom-policies"
	customPolicySyncPath   = apiBase + customPolicyCollection + "/sync"
	customPolicyVersionSep = "/versions/"
)

// customPolicyVersionPattern parses the path of one version of a custom policy.
var customPolicyVersionPattern = regexp.MustCompile(
	`^` + regexp.QuoteMeta(apiBase+customPolicyCollection) + `/([^/]+)/versions/([^/]+)$`)

// trackSyncedCustomPolicy registers the record a successful sync created or updated, as
// <uuid>/versions/<version> under the custom policy collection. A sync of a higher minor
// version updates the record in place, so the record's earlier version is dropped.
func (s *Steps) trackSyncedCustomPolicy(ctx context.Context, resp *httpx.Response) error {
	var policy struct {
		UUID    string `json:"uuid"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(resp.Body, &policy); err != nil {
		return fmt.Errorf("recording the synced custom policy for cleanup: %w", err)
	}
	if strings.TrimSpace(policy.UUID) == "" || strings.TrimSpace(policy.Version) == "" {
		return fmt.Errorf("recording the synced custom policy for cleanup: the response names no uuid and version: %s",
			resp.Describe())
	}
	id := policy.UUID + customPolicyVersionSep + policy.Version
	return s.registerTracked(ctx, platformCustomPolicyKind, id, func(ctx context.Context) error {
		return s.deletePlatformResource(ctx, platformCustomPolicyKind, customPolicyCollection, id)
	}, func(reg *cleanup.Registry) error {
		deregisterCustomPolicy(reg, policy.UUID)
		return s.installPlatformDeleter(reg, platformCustomPolicyKind, customPolicyCollection)
	})
}

// deregisterDeletedCustomPolicy drops the cleanup record of a custom policy the scenario
// deleted itself.
func deregisterDeletedCustomPolicy(ctx context.Context, rawURL string) error {
	match := customPolicyVersionPattern.FindStringSubmatch(locationPath(rawURL))
	if match == nil {
		return nil
	}
	reg, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	deregisterCustomPolicy(reg, match[1])
	return nil
}

// deregisterCustomPolicy drops every cleanup record of the custom policy uuid.
func deregisterCustomPolicy(reg *cleanup.Registry, uuid string) {
	for _, pending := range reg.Pending() {
		if pending.Kind.Name == platformCustomPolicyKind.Name && strings.HasPrefix(pending.ID, uuid+customPolicyVersionSep) {
			reg.Deregister(platformCustomPolicyKind, pending.ID)
		}
	}
}

// jsonArrayHasItem reports whether a 200 JSON response holds, at field, an array with an object
// whose key field renders as want.
func jsonArrayHasItem(resp *httpx.Response, field, key, want string) bool {
	if resp == nil || resp.StatusCode != http.StatusOK {
		return false
	}
	var doc any
	if json.Unmarshal(resp.Body, &doc) != nil {
		return false
	}
	items, ok := traverse(doc, field)
	if !ok {
		return false
	}
	list, ok := items.([]any)
	if !ok {
		return false
	}
	for _, item := range list {
		got, ok := traverse(item, key)
		if ok && fmt.Sprintf("%v", got) == want {
			return true
		}
	}
	return false
}
