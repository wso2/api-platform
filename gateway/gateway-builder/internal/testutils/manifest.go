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

package testutils

import (
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
)

const (
	// ManifestFileName is the default name for the policy manifest file.
	ManifestFileName = "policy-manifest.yaml"
	// ManifestLockFileName is the default name for the policy manifest lock file.
	ManifestLockFileName = "policy-manifest-lock.yaml"
)

// CreateManifest writes a manifest file with the given raw YAML content.
func CreateManifest(t *testing.T, dir, content string) {
	t.Helper()
	WriteFile(t, filepath.Join(dir, ManifestFileName), content)
}

// CreateManifestLock writes a manifest lock file with the given raw YAML content.
func CreateManifestLock(t *testing.T, dir, content string) {
	t.Helper()
	WriteFile(t, filepath.Join(dir, ManifestLockFileName), content)
}

// ManifestPolicy represents a policy entry for manifest generation helpers.
type ManifestPolicy struct {
	Name     string
	FilePath string // For local policies
	GoModule string // For remote policies (gomodule field value)
}

// CreateManifestWithPolicies creates a properly structured manifest file.
func CreateManifestWithPolicies(t *testing.T, dir string, policies []ManifestPolicy) {
	t.Helper()

	entries := make([]types.ManifestEntry, 0, len(policies))
	for _, p := range policies {
		entry := types.ManifestEntry{Name: p.Name}
		if p.FilePath != "" {
			entry.FilePath = p.FilePath
		}
		if p.GoModule != "" {
			entry.Gomodule = p.GoModule
		}
		entries = append(entries, entry)
	}

	manifest := types.PolicyManifest{
		Version:  "v1",
		Policies: entries,
	}

	data, err := yaml.Marshal(&manifest)
	require.NoError(t, err, "failed to marshal manifest")
	WriteFile(t, filepath.Join(dir, ManifestFileName), string(data))
}

// CreateSimpleManifest creates a manifest with a single local policy.
func CreateSimpleManifest(t *testing.T, dir, policyName, policyPath string) {
	t.Helper()
	CreateManifestWithPolicies(t, dir, []ManifestPolicy{
		{Name: policyName, FilePath: policyPath},
	})
}

// CreateRemoteManifest creates a manifest with a single remote (gomodule) policy.
func CreateRemoteManifest(t *testing.T, dir, policyName, modulePath string) {
	t.Helper()
	CreateManifestWithPolicies(t, dir, []ManifestPolicy{
		{Name: policyName, GoModule: modulePath},
	})
}
