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
	"os"
	"path/filepath"
)

// RouterGenerator generates a placeholder Dockerfile for the router
type RouterGenerator struct {
	outputDir      string
	baseImage      string
	builderVersion string
}

// NewRouterGenerator creates a new router generator
func NewRouterGenerator(outputDir, baseImage, builderVersion string) *RouterGenerator {
	return &RouterGenerator{
		outputDir:      outputDir,
		baseImage:      baseImage,
		builderVersion: builderVersion,
	}
}

// Generate generates the router Dockerfile
func (g *RouterGenerator) Generate() (string, error) {
	slog.Info("Generating router Dockerfile",
		"outputDir", g.outputDir)

	// Create output directory
	routerDir := filepath.Join(g.outputDir, "router")
	if err := os.MkdirAll(routerDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create router directory: %w", err)
	}

	// Generate Dockerfile
	dockerfilePath := filepath.Join(routerDir, "Dockerfile")
	if err := g.generateDockerfile(dockerfilePath); err != nil {
		return "", fmt.Errorf("failed to generate Dockerfile: %w", err)
	}

	slog.Info("Successfully generated router Dockerfile",
		"path", dockerfilePath)

	return dockerfilePath, nil
}

// generateDockerfile generates a simple placeholder Dockerfile for the router
func (g *RouterGenerator) generateDockerfile(path string) error {
	slog.Debug("Generating router Dockerfile", "path", path)

	// Simple Dockerfile that references a base image as an ARG
	dockerfileContent := `# Router Dockerfile
# This Dockerfile uses the specified base router image
# Build with: docker build -t <output-image:tag> .

FROM ` + g.baseImage + `

# Add any custom router configurations here if needed
# COPY config/ /etc/router/config/

# Labels
LABEL build.builder-version="` + g.builderVersion + `"
LABEL build.component="router"
`

	// Write Dockerfile
	if err := os.WriteFile(path, []byte(dockerfileContent), 0644); err != nil {
		return fmt.Errorf("failed to write Dockerfile: %w", err)
	}

	slog.Debug("Generated router Dockerfile", "path", path)
	return nil
}
