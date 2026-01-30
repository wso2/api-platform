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

package it

import (
	"path/filepath"

	"github.com/wso2/api-platform/common/testutils/coverage"
)

// DefaultCoverageConfig returns the default coverage configuration for cli/it
func DefaultCoverageConfig() *coverage.CoverageConfig {
	gatewayControllerDir, _ := filepath.Abs("../../gateway/gateway-controller")
	cliDir, _ := filepath.Abs("../src")
	return &coverage.CoverageConfig{
		OutputDir: "coverage",
		Services:  []string{"gateway-controller", "cli"},
		ServiceSourceDirs: map[string]string{
			"gateway-controller": gatewayControllerDir,
			"cli":                cliDir,
		},
		ContainerPath: "/build/",
		ModulePrefixes: []string{
			"github.com/wso2/api-platform/gateway/gateway-controller/",
			"github.com/wso2/api-platform/gateway/",
			"github.com/wso2/api-platform/cli/",
			"github.com/wso2/api-platform/",
		},
		ReportPrefix: "cli-integration-test-coverage",
	}
}

// CoverageCollector is an alias to the common coverage collector
type CoverageCollector = coverage.CoverageCollector

// NewCoverageCollector creates a new CoverageCollector
var NewCoverageCollector = coverage.NewCoverageCollector
