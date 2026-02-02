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

package types

import (
	"time"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// DiscoveredPolicy represents a policy found during the discovery phase
type DiscoveredPolicy struct {
	Name             string
	Version          string
	Path             string
	YAMLPath         string
	GoModPath        string
	SourceFiles      []string
	SystemParameters map[string]interface{}
	Definition       *policy.PolicyDefinition
}

// ConditionDef represents execution conditions
type ConditionDef struct {
	Supported bool `yaml:"supported"`
}

// BodyRequirements specifies body processing needs
type BodyRequirements struct {
	Request  bool `yaml:"request"`
	Response bool `yaml:"response"`
}

// ValidationResult contains validation errors and warnings
type ValidationResult struct {
	Valid    bool
	Errors   []ValidationError
	Warnings []ValidationWarning
}

// ValidationError represents a validation failure
type ValidationError struct {
	PolicyName    string
	PolicyVersion string
	FilePath      string
	LineNumber    int
	Message       string
}

// ValidationWarning represents a non-blocking validation issue
type ValidationWarning struct {
	PolicyName    string
	PolicyVersion string
	FilePath      string
	Message       string
}

// BuildMetadata contains information about the build
type BuildMetadata struct {
	Timestamp      time.Time
	BuilderVersion string
	Version        string // Policy Engine version
	GitCommit      string // Git commit hash
	Policies       []PolicyInfo
}

// PolicyInfo contains basic policy information for build metadata
type PolicyInfo struct {
	Name    string
	Version string
}

// CompilationOptions contains settings for the compilation phase
type CompilationOptions struct {
	OutputPath     string
	EnableUPX      bool
	LDFlags        string
	BuildTags      []string
	CGOEnabled     bool
	TargetOS       string
	TargetArch     string
	EnableCoverage bool // Enable coverage instrumentation for integration tests
}

// PackagingMetadata contains Docker image metadata
type PackagingMetadata struct {
	BaseImage      string
	Labels         map[string]string
	BuildTimestamp time.Time
	Policies       []PolicyInfo
}
