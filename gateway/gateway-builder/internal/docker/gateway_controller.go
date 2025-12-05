package docker

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/policy-engine/gateway-builder/pkg/fsutil"
	"github.com/policy-engine/gateway-builder/pkg/types"
	"github.com/policy-engine/gateway-builder/templates"
)

// GatewayControllerGenerator generates the gateway controller Dockerfile and artifacts
type GatewayControllerGenerator struct {
	outputDir      string
	baseImage      string
	policies       []*types.DiscoveredPolicy
	builderVersion string
}

// NewGatewayControllerGenerator creates a new gateway controller generator
func NewGatewayControllerGenerator(outputDir, baseImage string, policies []*types.DiscoveredPolicy, builderVersion string) *GatewayControllerGenerator {
	return &GatewayControllerGenerator{
		outputDir:      outputDir,
		baseImage:      baseImage,
		policies:       policies,
		builderVersion: builderVersion,
	}
}

// Generate generates the gateway controller Dockerfile and copies policy files
func (g *GatewayControllerGenerator) Generate() (string, error) {
	slog.Info("Generating gateway controller Dockerfile",
		"outputDir", g.outputDir)

	// Create output directory
	gcDir := filepath.Join(g.outputDir, "gateway-controller")
	if err := os.MkdirAll(gcDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create gateway-controller directory: %w", err)
	}

	// Create policies directory within gateway-controller
	policiesDir := filepath.Join(gcDir, "policies")
	if err := os.MkdirAll(policiesDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create policies directory: %w", err)
	}

	// Copy and rename policy.yaml files
	if err := g.copyPolicyFiles(policiesDir); err != nil {
		return "", fmt.Errorf("failed to copy policy files: %w", err)
	}

	// Generate Dockerfile
	dockerfilePath := filepath.Join(gcDir, "Dockerfile")
	if err := g.generateDockerfile(dockerfilePath); err != nil {
		return "", fmt.Errorf("failed to generate Dockerfile: %w", err)
	}

	slog.Info("Successfully generated gateway controller Dockerfile",
		"path", dockerfilePath)

	return dockerfilePath, nil
}

// copyPolicyFiles copies policy.yaml files and renames them to {name}-{version}.yaml
func (g *GatewayControllerGenerator) copyPolicyFiles(destDir string) error {
	slog.Info("Copying policy definition files",
		"count", len(g.policies),
		"destDir", destDir)

	for _, policy := range g.policies {
		// Source: policy.yaml in policy directory
		srcPath := policy.YAMLPath

		// Destination: {PolicyName}-{Version}.yaml
		destFilename := fmt.Sprintf("%s-%s.yaml", policy.Name, policy.Version)
		destPath := filepath.Join(destDir, destFilename)

		slog.Debug("Copying policy file",
			"name", policy.Name,
			"version", policy.Version,
			"src", srcPath,
			"dest", destPath)

		// Copy file
		if err := fsutil.CopyFile(srcPath, destPath); err != nil {
			return fmt.Errorf("failed to copy policy file %s: %w", policy.Name, err)
		}
	}

	slog.Info("Successfully copied all policy definition files",
		"count", len(g.policies))

	return nil
}

// generateDockerfile generates the Dockerfile for the gateway controller
func (g *GatewayControllerGenerator) generateDockerfile(path string) error {
	slog.Debug("Generating gateway controller Dockerfile", "path", path)

	// Parse template
	tmpl, err := template.New("dockerfile").Parse(templates.DockerfileGatewayControllerTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse Dockerfile template: %w", err)
	}

	// Prepare template data (note: BaseImage will be set in the template as ARG)
	data := struct {
		BaseImage      string
		BuildTimestamp string
		BuilderVersion string
		Labels         map[string]string
	}{
		BaseImage:      g.baseImage,
		BuildTimestamp: time.Now().UTC().Format(time.RFC3339),
		BuilderVersion: g.builderVersion,
		Labels: map[string]string{
			"build.timestamp":       time.Now().UTC().Format(time.RFC3339),
			"build.builder-version": g.builderVersion,
			"build.policy-count":    fmt.Sprintf("%d", len(g.policies)),
		},
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

	slog.Debug("Generated gateway controller Dockerfile", "path", path)
	return nil
}
