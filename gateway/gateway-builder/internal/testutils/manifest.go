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
	// BuildFileName is the default name for the build file.
	BuildFileName = "build.yaml"
	// BuildManifestFileName is the default name for the build manifest file.
	BuildManifestFileName = "build-manifest.yaml"
)

// CreateBuildFile writes a build file with the given raw YAML content.
func CreateBuildFile(t *testing.T, dir, content string) {
	t.Helper()
	WriteFile(t, filepath.Join(dir, BuildFileName), content)
}

// CreateBuildManifest writes a build manifest file with the given raw YAML content.
func CreateBuildManifest(t *testing.T, dir, content string) {
	t.Helper()
	WriteFile(t, filepath.Join(dir, BuildManifestFileName), content)
}

// BuildFilePolicy represents a policy entry for build file generation helpers.
type BuildFilePolicy struct {
	Name     string
	FilePath string // For local policies
	GoModule string // For remote policies (gomodule field value)
}

// CreateBuildFileWithPolicies creates a properly structured build file.
func CreateBuildFileWithPolicies(t *testing.T, dir string, policies []BuildFilePolicy) {
	t.Helper()

	entries := make([]types.BuildEntry, 0, len(policies))
	for _, p := range policies {
		entry := types.BuildEntry{Name: p.Name}
		if p.FilePath != "" {
			entry.FilePath = p.FilePath
		}
		if p.GoModule != "" {
			entry.Gomodule = p.GoModule
		}
		entries = append(entries, entry)
	}

	bf := types.BuildFile{
		Version:  "v1",
		Policies: entries,
	}

	data, err := yaml.Marshal(&bf)
	require.NoError(t, err, "failed to marshal build file")
	WriteFile(t, filepath.Join(dir, BuildFileName), string(data))
}

// CreateSimpleBuildFile creates a build file with a single local policy.
func CreateSimpleBuildFile(t *testing.T, dir, policyName, policyPath string) {
	t.Helper()
	CreateBuildFileWithPolicies(t, dir, []BuildFilePolicy{
		{Name: policyName, FilePath: policyPath},
	})
}

// CreateRemoteBuildFile creates a build file with a single remote (gomodule) policy.
func CreateRemoteBuildFile(t *testing.T, dir, policyName, modulePath string) {
	t.Helper()
	CreateBuildFileWithPolicies(t, dir, []BuildFilePolicy{
		{Name: policyName, GoModule: modulePath},
	})
}
