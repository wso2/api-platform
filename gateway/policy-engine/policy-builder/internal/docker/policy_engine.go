package docker

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/policy-engine/policy-builder/pkg/fsutil"
	"github.com/policy-engine/policy-builder/templates"
)

// PolicyEngineGenerator generates the policy engine Dockerfile and artifacts
type PolicyEngineGenerator struct {
	outputDir       string
	policyEngineBin string
	builderVersion  string
}

// NewPolicyEngineGenerator creates a new policy engine generator
func NewPolicyEngineGenerator(outputDir, policyEngineBin, builderVersion string) *PolicyEngineGenerator {
	return &PolicyEngineGenerator{
		outputDir:       outputDir,
		policyEngineBin: policyEngineBin,
		builderVersion:  builderVersion,
	}
}

// Generate generates the policy engine Dockerfile and copies the binary
func (g *PolicyEngineGenerator) Generate() (string, error) {
	slog.Info("Generating policy engine Dockerfile",
		"outputDir", g.outputDir)

	// Create output directory
	peDir := filepath.Join(g.outputDir, "policy-engine")
	if err := os.MkdirAll(peDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create policy-engine directory: %w", err)
	}

	// Copy binary to output directory
	binaryDest := filepath.Join(peDir, "policy-engine")
	if err := fsutil.CopyFile(g.policyEngineBin, binaryDest); err != nil {
		return "", fmt.Errorf("failed to copy binary: %w", err)
	}

	// Make binary executable
	if err := os.Chmod(binaryDest, 0755); err != nil {
		return "", fmt.Errorf("failed to make binary executable: %w", err)
	}

	// Generate Dockerfile
	dockerfilePath := filepath.Join(peDir, "Dockerfile")
	if err := g.generateDockerfile(dockerfilePath); err != nil {
		return "", fmt.Errorf("failed to generate Dockerfile: %w", err)
	}

	slog.Info("Successfully generated policy engine Dockerfile",
		"path", dockerfilePath)

	return dockerfilePath, nil
}

// generateDockerfile generates the Dockerfile for the policy engine
func (g *PolicyEngineGenerator) generateDockerfile(path string) error {
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
		BuilderVersion: g.builderVersion,
		Labels: map[string]string{
			"build.timestamp":       time.Now().UTC().Format(time.RFC3339),
			"build.builder-version": g.builderVersion,
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
