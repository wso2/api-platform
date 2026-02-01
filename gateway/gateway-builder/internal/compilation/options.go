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

package compilation

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
)

// BuildOptions creates compilation options for the policy engine binary
func BuildOptions(outputPath string, buildMetadata *types.BuildMetadata) *types.CompilationOptions {
	// Check for coverage mode from environment variable
	enableCoverage := false
	if coverageEnv := os.Getenv("COVERAGE"); strings.EqualFold(coverageEnv, "true") {
		enableCoverage = true
	}

	// Generate ldflags for build metadata injection
	// Pass enableCoverage to avoid stripping debug info needed for coverage
	ldflags := generateLDFlags(buildMetadata, enableCoverage)

	return &types.CompilationOptions{
		OutputPath:     outputPath,
		EnableUPX:      false, // Disabled by default for compatibility
		LDFlags:        ldflags,
		BuildTags:      []string{},
		CGOEnabled:     false, // Static binary
		TargetOS:       "linux",
		TargetArch:     runtime.GOARCH, // Use native architecture
		EnableCoverage: enableCoverage,
	}
}

// generateLDFlags creates ldflags string for embedding build metadata
// enableCoverage determines if coverage is enabled (skip -s -w flags if so)
func generateLDFlags(metadata *types.BuildMetadata, enableCoverage bool) string {
	var ldflags string
	
	// Only strip debug info if coverage is NOT enabled
	// -s and -w interfere with Go coverage instrumentation
	if !enableCoverage {
		ldflags = "-s -w" // Strip debug info and symbol table
	}

	// Add version information (matching policy-engine main.go variables)
	if ldflags != "" {
		ldflags += " "
	}
	ldflags += fmt.Sprintf("-X main.Version=%s", metadata.Version)
	ldflags += fmt.Sprintf(" -X main.GitCommit=%s", metadata.GitCommit)

	// Add build timestamp as BuildDate
	timestamp := metadata.Timestamp.Format(time.RFC3339)
	ldflags += fmt.Sprintf(" -X main.BuildDate=%s", timestamp)

	return ldflags
}
