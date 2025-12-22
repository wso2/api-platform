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

package docker

import (
	"fmt"
	"log/slog"

	"github.com/policy-engine/gateway-builder/pkg/types"
)

// DockerfileGenerator orchestrates generating all Dockerfiles and artifacts
type DockerfileGenerator struct {
	PolicyEngineBin            string
	Policies                   []*types.DiscoveredPolicy
	OutputDir                  string
	GatewayControllerBaseImage string
	RouterBaseImage            string
	BuilderVersion             string
}

// GenerateResult contains the results of generating all Dockerfiles
type GenerateResult struct {
	PolicyEngineDockerfile      string
	GatewayControllerDockerfile string
	RouterDockerfile            string
	PolicyEngineBin             string
	OutputDir                   string
	Success                     bool
	Errors                      []error
}

// GenerateAll generates all Dockerfiles and copies necessary artifacts
func (sg *DockerfileGenerator) GenerateAll() (*GenerateResult, error) {
	result := &GenerateResult{
		Success:         true,
		OutputDir:       sg.OutputDir,
		PolicyEngineBin: sg.PolicyEngineBin,
	}

	slog.Info("Starting Dockerfile generation", "phase", "dockerfile-generation")

	// 1. Generate Policy Engine Dockerfile
	slog.Info("Generating policy engine Dockerfile",
		"outputDir", sg.OutputDir)

	peGenerator := NewPolicyEngineGenerator(
		sg.OutputDir,
		sg.PolicyEngineBin,
		sg.BuilderVersion,
	)

	dockerfilePath, err := peGenerator.Generate()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("policy engine generation failed: %w", err))
		result.Success = false
	} else {
		result.PolicyEngineDockerfile = dockerfilePath
	}

	// 2. Generate Gateway Controller Dockerfile
	slog.Info("Generating gateway controller Dockerfile",
		"outputDir", sg.OutputDir,
		"baseImage", sg.GatewayControllerBaseImage)

	gcGenerator := NewGatewayControllerGenerator(
		sg.OutputDir,
		sg.GatewayControllerBaseImage,
		sg.Policies,
		sg.BuilderVersion,
	)

	dockerfilePath, err = gcGenerator.Generate()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("gateway controller generation failed: %w", err))
		result.Success = false
	} else {
		result.GatewayControllerDockerfile = dockerfilePath
	}

	// 3. Generate Router Dockerfile
	slog.Info("Generating router Dockerfile",
		"outputDir", sg.OutputDir,
		"baseImage", sg.RouterBaseImage)

	routerGenerator := NewRouterGenerator(
		sg.OutputDir,
		sg.RouterBaseImage,
		sg.BuilderVersion,
	)

	dockerfilePath, err = routerGenerator.Generate()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("router generation failed: %w", err))
		result.Success = false
	} else {
		result.RouterDockerfile = dockerfilePath
	}

	if result.Success {
		slog.Info("Successfully generated all Dockerfiles", "phase", "dockerfile-generation")
	} else {
		slog.Error("Dockerfile generation completed with errors",
			"errorCount", len(result.Errors))
	}

	return result, nil
}
