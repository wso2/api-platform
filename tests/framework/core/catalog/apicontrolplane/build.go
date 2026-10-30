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

package apicontrolplane

import (
	"path/filepath"

	"github.com/wso2/api-platform/tests/framework/core/builder"
)

// BuildSpec returns the source-build metadata for the Management Portal.
func BuildSpec(version string) (builder.Spec, error) {
	image := "ghcr.io/wso2/api-platform/api-control-plane:" + version
	return builder.Spec{
		Component: svcAPIControlPlane,
		SourceDir: "portals/api-control-plane",
		Coverage: builder.CoverageSpec{
			Supported: true, Types: []builder.CoverageType{builder.GoCoverage, builder.BrowserJSCoverage},
			Packages: []string{"api-control-plane-bff/..."}, OutputDir: "/coverage",
			BuildArgs:   map[string]string{"ENABLE_COVERAGE": "true"},
			Environment: map[string]string{"GOCOVERDIR": "/coverage"},
			Browser: builder.BrowserCoverageSpec{
				Supported:  true,
				SourceRoot: "portals/api-control-plane/src",
				Include:    []string{"**/*.ts", "**/*.tsx"},
				Exclude:    []string{"**/*.test.*", "**/node_modules/**"},
				OutputDir:  "/coverage/browser",
			},
		},
		Images: []builder.Image{{
			Name:       image,
			Dockerfile: "portals/api-control-plane/Dockerfile",
			Context:    "portals/api-control-plane",
		}},
		Plan: func(repoRoot, v string, coverage builder.CoverageSpec) ([]builder.Command, error) {
			args := []string{"docker", "buildx", "build", "--build-context", "httpkit=../../httpkit", "--tag", "ghcr.io/wso2/api-platform/api-control-plane:" + v, "--load"}
			if coverage.Supported {
				args = append(args, builder.CoverageBuildArgs(coverage)...)
			}
			args = append(args, ".")
			return []builder.Command{{
				Directory: filepath.Join(repoRoot, "portals/api-control-plane"),
				Args:      args,
			}}, nil
		},
	}, nil
}
