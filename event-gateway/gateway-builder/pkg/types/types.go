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

// Package types defines the shared data structures used by the event-gateway builder.
package types

// BuildFile represents the build.yaml structure.
type BuildFile struct {
	Version  string       `yaml:"version"`
	Policies []BuildEntry `yaml:"policies"`
}

// BuildEntry is a single policy entry in the build file.
type BuildEntry struct {
	Name       string `yaml:"name"`
	FilePath   string `yaml:"filePath,omitempty"`
	Gomodule   string `yaml:"gomodule,omitempty"`
	PipPackage string `yaml:"pipPackage,omitempty"`
}

// DiscoveredPolicy holds all resolved information about a policy.
type DiscoveredPolicy struct {
	// Name is the policy name (from build.yaml or policy-definition.yaml).
	Name string
	// Version is the resolved semver version.
	Version string
	// Path is the absolute directory path (filePath entries only).
	Path string
	// GoModulePath is the Go module import path (e.g. github.com/wso2/...).
	GoModulePath string
	// GoModuleVersion is the canonical resolved version (e.g. v1.0.3).
	GoModuleVersion string
	// IsFilePathEntry is true for local path-based policies.
	IsFilePathEntry bool
	// Runtime is always "go" for event-gateway policies.
	Runtime string
	// SystemParameters are the system-parameter defaults from policy-definition.yaml.
	SystemParameters map[string]interface{}
}
