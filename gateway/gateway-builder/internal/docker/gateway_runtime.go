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
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/fsutil"
	"github.com/wso2/api-platform/gateway/gateway-builder/templates"
)

// GatewayRuntimeGenerator generates the gateway runtime Dockerfile and artifacts
type GatewayRuntimeGenerator struct {
	outputDir       string
	policyEngineBin string
	baseImage       string
	builderVersion  string
}

// NewGatewayRuntimeGenerator creates a new gateway runtime generator
func NewGatewayRuntimeGenerator(outputDir, policyEngineBin, baseImage, builderVersion string) *GatewayRuntimeGenerator {
	return &GatewayRuntimeGenerator{
		outputDir:       outputDir,
		policyEngineBin: policyEngineBin,
		baseImage:       baseImage,
		builderVersion:  builderVersion,
	}
}

// Generate generates the gateway runtime Dockerfile and copies the binary
func (g *GatewayRuntimeGenerator) Generate() (string, error) {
	slog.Info("Generating gateway runtime Dockerfile",
		"outputDir", g.outputDir)

	// Create output directory
	runtimeDir := filepath.Join(g.outputDir, "gateway-runtime")
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create gateway-runtime directory: %w", err)
	}

	// Copy binary to output directory
	binaryDest := filepath.Join(runtimeDir, "policy-engine")
	if err := fsutil.CopyFile(g.policyEngineBin, binaryDest); err != nil {
		return "", fmt.Errorf("failed to copy binary: %w", err)
	}

	// Make binary executable
	if err := os.Chmod(binaryDest, 0755); err != nil {
		return "", fmt.Errorf("failed to make binary executable: %w", err)
	}

	// Generate Dockerfile
	dockerfilePath := filepath.Join(runtimeDir, "Dockerfile")
	if err := g.generateDockerfile(dockerfilePath); err != nil {
		return "", fmt.Errorf("failed to generate Dockerfile: %w", err)
	}

	slog.Info("Successfully generated gateway runtime Dockerfile",
		"path", dockerfilePath)

	return dockerfilePath, nil
}

// generateDockerfile generates the Dockerfile for the gateway runtime
func (g *GatewayRuntimeGenerator) generateDockerfile(path string) error {
	slog.Debug("Generating gateway runtime Dockerfile", "path", path)

	// Parse template
	tmpl, err := template.New("dockerfile").Parse(templates.DockerfileGatewayRuntimeTmpl)
	if err != nil {
		return fmt.Errorf("failed to parse Dockerfile template: %w", err)
	}

	// Prepare template data
	data := struct {
		BuildTimestamp string
		BuilderVersion string
		BaseImage      string
		Labels         map[string]string
	}{
		BuildTimestamp: time.Now().UTC().Format(time.RFC3339),
		BuilderVersion: g.builderVersion,
		BaseImage:      g.baseImage,
		Labels: map[string]string{},
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute Dockerfile template: %w", err)
	}

	// Write Dockerfile
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write Dockerfile: %w", err)
	}

	slog.Debug("Generated gateway runtime Dockerfile", "path", path)
	return nil
}
