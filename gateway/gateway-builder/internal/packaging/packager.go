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

package packaging

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/errors"
	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
	"github.com/wso2/api-platform/gateway/gateway-builder/templates"
)

// GenerateDockerfile generates the runtime Dockerfile
func GenerateDockerfile(outputDir string, policies []*types.DiscoveredPolicy, builderVersion string) error {
	slog.Info("Generating runtime Dockerfile", "phase", "packaging")

	// Create packaging metadata
	slog.Debug("Creating packaging metadata",
		"policyCount", len(policies),
		"phase", "packaging")

	metadata := &types.PackagingMetadata{
		BaseImage:      "alpine:3.19",
		BuildTimestamp: time.Now().UTC(),
		Policies:       make([]types.PolicyInfo, 0, len(policies)),
	}

	for _, p := range policies {
		metadata.Policies = append(metadata.Policies, types.PolicyInfo{
			Name:    p.Name,
			Version: p.Version,
		})
	}

	// Generate labels
	labels := GenerateDockerLabels(metadata)

	// Parse embedded template
	tmpl, err := template.New("dockerfile").Parse(templates.DockerfilePolicyEngineTmpl)
	if err != nil {
		return errors.NewPackagingError("failed to parse Dockerfile template", err)
	}

	// Execute template
	var buf bytes.Buffer
	data := struct {
		BuildTimestamp string
		BuilderVersion string
		Labels         map[string]string
	}{
		BuildTimestamp: metadata.BuildTimestamp.Format(time.RFC3339),
		BuilderVersion: builderVersion,
		Labels:         labels,
	}

	if err := tmpl.Execute(&buf, data); err != nil {
		return errors.NewPackagingError("failed to execute Dockerfile template", err)
	}

	// Write Dockerfile
	dockerfilePath := filepath.Join(outputDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, buf.Bytes(), 0644); err != nil {
		return errors.NewPackagingError("failed to write Dockerfile", err)
	}

	slog.Info("Generated Dockerfile",
		"path", dockerfilePath,
		"phase", "packaging")

	// Generate build instructions
	if err := generateBuildInstructions(outputDir, metadata); err != nil {
		return errors.NewPackagingError("failed to generate build instructions", err)
	}

	return nil
}

// generateBuildInstructions creates a README with Docker build instructions
func generateBuildInstructions(outputDir string, metadata *types.PackagingMetadata) error {
	instructions := `# Policy Engine Runtime Build Instructions

This directory contains the compiled policy engine binary and runtime Dockerfile.

## Contents

- policy-engine: Compiled binary with embedded policies
- Dockerfile: Runtime container image definition

## Compiled Policies

This binary includes the following policies:

`
	for i, p := range metadata.Policies {
		instructions += fmt.Sprintf("%d. %s %s\n", i+1, p.Name, p.Version)
	}

	instructions += fmt.Sprintf("\nBuild timestamp: %s\n", metadata.BuildTimestamp.Format(time.RFC3339))

	instructions += `
## Building the Docker Image

` + "```bash" + `
docker build -t policy-engine:custom .
` + "```" + `

## Running the Container

` + "```bash" + `
docker run -p 9001:9001 -p 9002:9002 policy-engine:custom
` + "```" + `

## Configuration

Mount your configuration files:

` + "```bash" + `
docker run -p 9001:9001 -p 9002:9002 \
  -v $(pwd)/configs:/etc/policy-engine \
  policy-engine:custom \
  --config-file=/etc/policy-engine/policy-engine.yaml
` + "```" + `

## Health Check

The container includes a health check endpoint. Check container health with:

` + "```bash" + `
docker ps
` + "```" + `

## Build Info

To see detailed build information:

` + "```bash" + `
docker run policy-engine:custom --build-info
` + "```" + `
`

	readmePath := filepath.Join(outputDir, "BUILD.md")
	if err := os.WriteFile(readmePath, []byte(instructions), 0644); err != nil {
		return fmt.Errorf("failed to write BUILD.md: %w", err)
	}

	slog.Info("Generated BUILD.md with instructions",
		"path", readmePath,
		"phase", "packaging")
	return nil
}
