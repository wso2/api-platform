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

// Package docker generates the Dockerfile used to build a custom event-gateway
// image that contains the user-compiled binary with embedded policies.
package docker

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/wso2/api-platform/event-gateway/gateway-builder/templates"
)

// GenerateResult holds the output of Dockerfile generation.
type GenerateResult struct {
	DockerfilePath  string
	EventGatewayBin string
	OutputDir       string
	Success         bool
	Errors          []error
}

// DockerfileGenerator generates the event-gateway Dockerfile and copies artifacts.
type DockerfileGenerator struct {
	EventGatewayBin string
	OutputDir       string
	BaseImage       string
	BuilderVersion  string
	Labels          map[string]string
}

// Generate generates the Dockerfile and copies the binary into the output directory.
func (g *DockerfileGenerator) Generate() (*GenerateResult, error) {
	result := &GenerateResult{
		Success:   true,
		OutputDir: g.OutputDir,
	}

	outDir := filepath.Join(g.OutputDir, "event-gateway")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Copy the compiled binary
	destBin := filepath.Join(outDir, "event-gateway")
	if err := copyFile(g.EventGatewayBin, destBin); err != nil {
		return nil, fmt.Errorf("failed to copy binary: %w", err)
	}
	result.EventGatewayBin = destBin

	// Render the Dockerfile template
	tmpl, err := template.New("Dockerfile").Parse(templates.DockerfileEventGatewayTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Dockerfile template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct {
		BaseImage      string
		BuildTimestamp string
		BuilderVersion string
		Labels         map[string]string
	}{
		BaseImage:      g.BaseImage,
		BuildTimestamp: time.Now().UTC().Format(time.RFC3339),
		BuilderVersion: g.BuilderVersion,
		Labels:         g.Labels,
	}); err != nil {
		return nil, fmt.Errorf("failed to render Dockerfile template: %w", err)
	}

	dockerfilePath := filepath.Join(outDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, buf.Bytes(), 0600); err != nil {
		return nil, fmt.Errorf("failed to write Dockerfile: %w", err)
	}
	result.DockerfilePath = dockerfilePath

	slog.Info("Generated event-gateway Dockerfile", "path", dockerfilePath)
	return result, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src) //nolint:gosec
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst) //nolint:gosec
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
