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

	"github.com/policy-engine/policy-builder/pkg/types"
	"github.com/policy-engine/policy-builder/templates"
)

// GatewayControllerBuilder builds the gateway controller Docker image
type GatewayControllerBuilder struct {
	tempDir        string
	baseImage      string
	outputImage    string
	imageTag       string
	buildArch      string
	policies       []*types.DiscoveredPolicy
	builderVersion string
}

// NewGatewayControllerBuilder creates a new gateway controller builder
func NewGatewayControllerBuilder(tempDir, baseImage, outputImage, imageTag, buildArch string, policies []*types.DiscoveredPolicy, builderVersion string) *GatewayControllerBuilder {
	return &GatewayControllerBuilder{
		tempDir:        tempDir,
		baseImage:      baseImage,
		outputImage:    outputImage,
		imageTag:       imageTag,
		buildArch:      buildArch,
		policies:       policies,
		builderVersion: builderVersion,
	}
}

// Build builds the gateway controller Docker image
func (b *GatewayControllerBuilder) Build() error {
	slog.Info("Building gateway controller image",
		"baseImage", b.baseImage,
		"outputImage", b.outputImage,
		"tag", b.imageTag)

	// Create policies directory
	policiesDir := filepath.Join(b.tempDir, "gc-policies")
	if err := os.MkdirAll(policiesDir, 0755); err != nil {
		return fmt.Errorf("failed to create policies directory: %w", err)
	}

	// Copy and rename policy.yaml files
	if err := b.copyPolicyFiles(policiesDir); err != nil {
		return fmt.Errorf("failed to copy policy files: %w", err)
	}

	// Generate Dockerfile
	dockerfilePath := filepath.Join(b.tempDir, "Dockerfile.gateway-controller")
	if err := b.generateDockerfile(dockerfilePath); err != nil {
		return fmt.Errorf("failed to generate Dockerfile: %w", err)
	}

	// Build image
	fullImageName := fmt.Sprintf("%s:%s", b.outputImage, b.imageTag)
	platform := fmt.Sprintf("linux/%s", b.buildArch)
	if err := ExecuteDockerCommand("build",
		"-f", dockerfilePath,
		"-t", fullImageName,
		"--platform", platform,
		b.tempDir,
	); err != nil {
		return fmt.Errorf("failed to build gateway controller image: %w", err)
	}

	slog.Info("Successfully built gateway controller image",
		"image", fullImageName)

	return nil
}

// copyPolicyFiles copies policy.yaml files and renames them to {name}-{version}.yaml
func (b *GatewayControllerBuilder) copyPolicyFiles(destDir string) error {
	slog.Info("Copying policy definition files",
		"count", len(b.policies),
		"destDir", destDir)

	for _, policy := range b.policies {
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
		if err := copyFile(srcPath, destPath); err != nil {
			return fmt.Errorf("failed to copy policy file %s: %w", policy.Name, err)
		}
	}

	slog.Info("Successfully copied all policy definition files",
		"count", len(b.policies))

	return nil
}

// generateDockerfile generates the Dockerfile for the gateway controller
func (b *GatewayControllerBuilder) generateDockerfile(path string) error {
	slog.Debug("Generating gateway controller Dockerfile", "path", path)

	// Parse template
	tmpl, err := template.New("dockerfile").Parse(templates.DockerfileGatewayControllerTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse Dockerfile template: %w", err)
	}

	// Prepare template data
	data := struct {
		BaseImage      string
		BuildTimestamp string
		BuilderVersion string
		Labels         map[string]string
	}{
		BaseImage:      b.baseImage,
		BuildTimestamp: time.Now().UTC().Format(time.RFC3339),
		BuilderVersion: b.builderVersion,
		Labels: map[string]string{
			"build.timestamp":      time.Now().UTC().Format(time.RFC3339),
			"build.builder-version": b.builderVersion,
			"build.policy-count":   fmt.Sprintf("%d", len(b.policies)),
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

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	return nil
}
