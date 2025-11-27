package docker

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/policy-engine/policy-builder/templates"
)

// PolicyEngineBuilder builds the policy engine Docker image
type PolicyEngineBuilder struct {
	tempDir        string
	imageName      string
	imageTag       string
	buildArch      string
	binaryPath     string
	builderVersion string
}

// NewPolicyEngineBuilder creates a new policy engine builder
func NewPolicyEngineBuilder(tempDir, outputImageName, imageTag, buildArch, binaryPath, builderVersion string) *PolicyEngineBuilder {
	return &PolicyEngineBuilder{
		tempDir:        tempDir,
		imageName:      outputImageName,
		imageTag:       imageTag,
		buildArch:      buildArch,
		binaryPath:     binaryPath,
		builderVersion: builderVersion,
	}
}

// Build builds the policy engine Docker image
func (b *PolicyEngineBuilder) Build() error {
	slog.Info("Building policy engine image",
		"image", b.imageName,
		"tag", b.imageTag)

	// Generate Dockerfile
	dockerfilePath := filepath.Join(b.tempDir, "Dockerfile.policy-engine")
	if err := b.generateDockerfile(dockerfilePath); err != nil {
		return fmt.Errorf("failed to generate Dockerfile: %w", err)
	}

	// Build image
	fullImageName := fmt.Sprintf("%s:%s", b.imageName, b.imageTag)
	platform := fmt.Sprintf("linux/%s", b.buildArch)
	if err := ExecuteDockerCommand("build",
		"-f", dockerfilePath,
		"-t", fullImageName,
		"--platform", platform,
		b.tempDir,
	); err != nil {
		return fmt.Errorf("failed to build policy engine image: %w", err)
	}

	slog.Info("Successfully built policy engine image",
		"image", fullImageName)

	return nil
}

// generateDockerfile generates the Dockerfile for the policy engine
func (b *PolicyEngineBuilder) generateDockerfile(path string) error {
	slog.Debug("Generating policy engine Dockerfile", "path", path)

	// Parse template
	tmpl, err := template.New("dockerfile").Parse(templates.DockerfileRuntimeTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse Dockerfile template: %w", err)
	}

	// Prepare template data
	data := struct {
		BuildTimestamp string
		BuilderVersion string
		Labels         map[string]string
	}{
		BuildTimestamp: time.Now().UTC().Format(time.RFC3339),
		BuilderVersion: b.builderVersion,
		Labels: map[string]string{
			"build.timestamp":      time.Now().UTC().Format(time.RFC3339),
			"build.builder-version": b.builderVersion,
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

	slog.Debug("Generated policy engine Dockerfile", "path", path)
	return nil
}
