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
	"path/filepath"
	"strings"

	"github.com/wso2/api-platform/tests/framework/core/builder"
)

// BuildSpec returns the source-build metadata for Platform API.
func BuildSpec(version string) (builder.Spec, error) {
	coveragePackages := []string{"github.com/wso2/api-platform/platform-api/..."}
	return builder.Spec{
		Component: "platform-api",
		SourceDir: "platform-api",
		Coverage: builder.CoverageSpec{
			Supported: true, Types: []builder.CoverageType{builder.GoCoverage},
			Packages: coveragePackages, OutputDir: "/coverage",
			BuildArgs:   map[string]string{"ENABLE_COVERAGE": "true"},
			Environment: map[string]string{"GOCOVERDIR": "/coverage"},
		},
		Images: []builder.Image{{
			Name:       "ghcr.io/wso2/api-platform/platform-api:" + version,
			Dockerfile: "platform-api/Dockerfile",
			Context:    ".",
		}},
		// Builds from source with docker-bake.hcl, which compiles the binary with
		// Dockerfile.build and builds the image Dockerfile around it in one run. With
		// coverage, the binary is instrumented and the "coverage" target adds a
		// writable /coverage on top of the image.
		Plan: func(repoRoot, v string, coverage builder.CoverageSpec) ([]builder.Command, error) {
			target := "image"
			// Bake reads its variables from the environment, so every switch is set
			// explicitly; a value left in the caller's environment can't turn on
			// coverage or the experimental build.
			env := map[string]string{"VERSION": v, "PORT": "9243",
				"ENABLE_COVERAGE": "false", "EXPERIMENTAL": "false"}
			if coverage.Supported {
				target = "coverage"
				env["ENABLE_COVERAGE"] = "true"
				env["COVERAGE_PACKAGES"] = strings.Join(coverage.Packages, ",")
			}
			args := []string{"docker", "buildx", "bake", "-f", "docker-bake.hcl",
				"--allow=fs.read=..",
				"--set", target + ".tags=ghcr.io/wso2/api-platform/platform-api:" + v,
				"--load", target}
			return []builder.Command{{
				Directory: filepath.Join(repoRoot, "platform-api"),
				Args:      args,
				Env:       env,
			}}, nil
		},
	}, nil
}
