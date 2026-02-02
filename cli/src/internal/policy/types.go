/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package policy

// PolicyManifest represents the build.yaml structure (policy manifest)
type PolicyManifest struct {
	Version           string           `yaml:"version"`
	VersionResolution string           `yaml:"versionResolution,omitempty"`
	Policies          []ManifestPolicy `yaml:"policies"`
	Gateway           GatewayConfig    `yaml:"gateway"`
}

// GatewayConfig represents the gateway configuration in the manifest
type GatewayConfig struct {
	Version string        `yaml:"version"`
	Images  GatewayImages `yaml:"images,omitempty"`
}

// GatewayImages represents optional custom image paths in the manifest
type GatewayImages struct {
	Builder    string `yaml:"builder,omitempty"`
	Controller string `yaml:"controller,omitempty"`
	Router     string `yaml:"router,omitempty"`
}

// ManifestPolicy represents a policy entry in the manifest
type ManifestPolicy struct {
	Name              string `yaml:"name"`
	Version           string `yaml:"version,omitempty"`
	VersionResolution string `yaml:"versionResolution,omitempty"`
	FilePath          string `yaml:"filePath,omitempty"`
}

// IsLocal returns true if the policy is a local policy (has filePath)
func (p *ManifestPolicy) IsLocal() bool {
	return p.FilePath != ""
}

// GetVersionResolution returns the version resolution strategy for this policy
func (p *ManifestPolicy) GetVersionResolution(rootResolution string) string {
	if p.VersionResolution != "" {
		return p.VersionResolution
	}
	if rootResolution != "" {
		return rootResolution
	}
	return "exact" // default
}

// PolicyLock represents the policy-manifest-lock.yaml structure
type PolicyLock struct {
	Version  string       `yaml:"version"`
	Policies []LockPolicy `yaml:"policies"`
}

// LockPolicy represents a policy entry in the lock file
type LockPolicy struct {
	Name     string `yaml:"name"`
	Version  string `yaml:"version"`
	Checksum string `yaml:"checksum"`
	Source   string `yaml:"source"`             // "hub" or "local"
	FilePath string `yaml:"filePath,omitempty"` // Workspace path (only in workspace lock file)
}

// ProcessedPolicy represents a policy after processing (downloading, verifying, etc.)
type ProcessedPolicy struct {
	Name      string
	Source    string // "hub" or "local"
	LocalPath string // Path to the policy (zip or directory)
	IsLocal   bool
	FilePath  string // Original filePath from manifest (for local policies)
}
