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

package platformgateway

import (
	"path/filepath"

	"github.com/wso2/api-platform/tests/framework/core/builder"
)

// BuildSpec returns the source-build metadata for the gateway controller and runtime.
func BuildSpec(version string) (builder.Spec, error) {
	coveragePackages := []string{
		"github.com/wso2/api-platform/gateway/gateway-controller/...",
		"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/...",
		"github.com/wso2/gateway-controllers/policies/...",
	}
	return builder.Spec{
		Component: "platform-gateway",
		SourceDir: "gateway",
		Coverage: builder.CoverageSpec{
			Supported: true,
			Types:     []builder.CoverageType{builder.GoCoverage},
			Packages:  coveragePackages,
			OutputDir: "/coverage",
			BuildArgs: map[string]string{
				"ENABLE_COVERAGE": "true",
			},
			Environment: map[string]string{
				"GOCOVERDIR": "/coverage",
			},
		},
		Images: []builder.Image{
			{Name: "ghcr.io/wso2/api-platform/gateway-controller:" + version, Dockerfile: "gateway/gateway-controller/Dockerfile", Context: "gateway/gateway-controller"},
			{Name: "ghcr.io/wso2/api-platform/gateway-runtime:" + version, Dockerfile: "gateway/gateway-runtime/Dockerfile", Context: "gateway/gateway-runtime"},
		},
		Plan: func(repoRoot, v string, coverage builder.CoverageSpec) ([]builder.Command, error) {
			dir := filepath.Join(repoRoot, "gateway")
			runtimeDir := filepath.Join(dir, "gateway-runtime")
			controller := "ghcr.io/wso2/api-platform/gateway-controller:" + v
			runtime := "ghcr.io/wso2/api-platform/gateway-runtime:" + v
			coverageArg := []string{}
			if coverage.Supported {
				coverageArg = builder.CoverageBuildArgs(coverage)
			}
			buildBase := []string{"docker", "buildx", "build", "-f", "Dockerfile",
				"--build-context", "sdk=../../sdk",
				"--build-context", "sdk-python=../../sdk-python",
				"--build-context", "sdk-core=../../sdk/core",
				"--build-context", "common=../../common",
				"--build-context", "httpkit=../../httpkit",
				"--build-context", "gateway-builder=../gateway-builder",
				"--build-context", "system-policies=../system-policies",
				"--build-context", "dev-policies=../dev-policies",
				"--build-context", "target=target",
				"--build-arg", "VERSION=" + v,
				"--build-arg", "GIT_COMMIT=framework",
			}
			runtimeBuild := append(append([]string(nil), buildBase...), coverageArg...)
			runtimeBuild = append(runtimeBuild, "--target", "production", "--tag", runtime, "--load", ".")
			policyExport := append(append([]string(nil), buildBase...), "--target", "policy-export", "--output", "type=local,dest=../target/build/gateway-controller/policies", ".")
			controllerBuild := []string{"docker", "buildx", "build", "-f", "Dockerfile",
				"--build-context", "sdk=../../sdk",
				"--build-context", "sdk-core=../../sdk/core",
				"--build-context", "common=../../common",
				"--build-context", "httpkit=../../httpkit",
				"--build-context", "build-manifest=..",
				"--build-context", "policies=../target/build/gateway-controller/policies",
				"--build-context", "target=target",
				"--build-arg", "VERSION=" + v,
				"--build-arg", "GIT_COMMIT=framework",
			}
			controllerBuild = append(controllerBuild, coverageArg...)
			controllerBuild = append(controllerBuild, "--target", "production", "--tag", controller, "--load", ".")
			return []builder.Command{
				{Directory: runtimeDir, Args: []string{"mkdir", "-p", "target/configs"}},
				{Directory: runtimeDir, Args: []string{"cp", "../build.yaml", "target/build.yaml"}},
				{Directory: runtimeDir, Args: []string{"cp", "../../LICENSE", "target/LICENSE"}},
				{Directory: runtimeDir, Args: []string{"cp", "-R", "../configs/llm-pricing", "target/configs/llm-pricing"}},
				{Directory: runtimeDir, Args: runtimeBuild},
				{Directory: runtimeDir, Args: []string{"mkdir", "-p", "../target/build/gateway-controller/policies"}},
				{Directory: runtimeDir, Args: policyExport},
				{Directory: runtimeDir, Args: []string{"sh", "-c", "docker run --rm --entrypoint cat " + runtime + " /app/build-manifest.yaml > ../build-manifest.yaml"}},
				{Directory: filepath.Join(dir, "gateway-controller"), Args: []string{"mkdir", "-p", "target"}},
				{Directory: filepath.Join(dir, "gateway-controller"), Args: []string{"cp", "../../LICENSE", "target/LICENSE"}},
				{Directory: filepath.Join(dir, "gateway-controller"), Args: controllerBuild},
			}, nil
		},
	}, nil
}
