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

package models

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

func storedBackend(mutate func(*StoredBackend)) *StoredBackend {
	basePath := "/v1"
	labels := map[string]string{"team": "orders"}
	displayName := "Orders"
	weight := 80
	b := &StoredBackend{UUID: "u-1", Configuration: api.BackendRequest{
		Metadata: api.Metadata{Name: "tenant-a", Labels: &labels},
		Spec: api.BackendConfiguration{DisplayName: &displayName, BasePath: &basePath, Endpoints: []struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{{Url: "http://orders-a:9080", Weight: &weight}}},
	}}
	if mutate != nil {
		mutate(b)
	}
	return b
}

// Each revision of a Backend has its own cluster. The revision changes with everything that
// shapes the cluster or the base path in the record, and with the UUID; it ignores labels and the
// display name, which change neither.
func TestStoredBackend_ClusterNamePerRevision(t *testing.T) {
	base := storedBackend(nil)
	assert.Regexp(t, regexp.MustCompile(`^backend_tenant-a_[0-9a-f]{12}$`), base.ClusterName())
	assert.Equal(t, base.ClusterName(), storedBackend(nil).ClusterName(), "stable for the same revision")

	newRevision := map[string]func(*StoredBackend){
		"endpoint": func(b *StoredBackend) { b.Configuration.Spec.Endpoints[0].Url = "http://orders-b:9080" },
		"weight": func(b *StoredBackend) {
			w := 20
			b.Configuration.Spec.Endpoints[0].Weight = &w
		},
		"base path": func(b *StoredBackend) {
			p := "/v2"
			b.Configuration.Spec.BasePath = &p
		},
		"tls": func(b *StoredBackend) {
			tls := map[string]interface{}{"verifyHostName": false}
			b.Configuration.Spec.Tls = &tls
		},
		"timeout": func(b *StoredBackend) {
			connect := "2s"
			b.Configuration.Spec.Timeout = &api.UpstreamTimeout{Connect: &connect}
		},
		"uuid": func(b *StoredBackend) { b.UUID = "u-2" },
	}
	for name, mutate := range newRevision {
		assert.NotEqual(t, base.ClusterName(), storedBackend(mutate).ClusterName(), "%s gives a new revision", name)
	}

	sameRevision := map[string]func(*StoredBackend){
		"labels": func(b *StoredBackend) {
			labels := map[string]string{"team": "billing"}
			b.Configuration.Metadata.Labels = &labels
		},
		"display name": func(b *StoredBackend) {
			d := "Billing"
			b.Configuration.Spec.DisplayName = &d
		},
	}
	for name, mutate := range sameRevision {
		assert.Equal(t, base.ClusterName(), storedBackend(mutate).ClusterName(), "%s keeps the revision", name)
	}
}
