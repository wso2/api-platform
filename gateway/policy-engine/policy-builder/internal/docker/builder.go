package docker

import (
	"fmt"
	"log/slog"

	"github.com/policy-engine/policy-builder/pkg/types"
)

// StackBuilder orchestrates building all three Docker images
type StackBuilder struct {
	TempDir    string
	BinaryPath string
	Policies   []*types.DiscoveredPolicy

	// Policy Engine
	PolicyEngineOutputImage string

	// Gateway Controller
	GatewayControllerBaseImage   string
	GatewayControllerOutputImage string

	// Router
	RouterBaseImage   string
	RouterOutputImage string

	// Common
	ImageTag       string
	BuildArch      string // Target architecture: arm64 or amd64
	BuilderVersion string
}

// BuildResult contains the results of building all Docker images
type BuildResult struct {
	PolicyEngineImage      string
	GatewayControllerImage string
	RouterImage            string
	Success                bool
	Errors                 []error
}

// BuildAll builds all three Docker images
func (sb *StackBuilder) BuildAll() (*BuildResult, error) {
	result := &BuildResult{Success: true}

	slog.Info("Starting Docker image builds", "phase", "docker-build")

	// 1. Build Policy Engine Image
	slog.Info("Building policy engine image",
		"outputImage", sb.PolicyEngineOutputImage,
		"tag", sb.ImageTag,
		"arch", sb.BuildArch)

	peBuilder := NewPolicyEngineBuilder(
		sb.TempDir,
		sb.PolicyEngineOutputImage,
		sb.ImageTag,
		sb.BuildArch,
		sb.BinaryPath,
		sb.BuilderVersion,
	)

	if err := peBuilder.Build(); err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("policy engine build failed: %w", err))
		result.Success = false
	} else {
		result.PolicyEngineImage = fmt.Sprintf("%s:%s",
			sb.PolicyEngineOutputImage, sb.ImageTag)
	}

	// 2. Build Gateway Controller Image
	slog.Info("Building gateway controller image",
		"baseImage", sb.GatewayControllerBaseImage,
		"outputImage", sb.GatewayControllerOutputImage,
		"tag", sb.ImageTag,
		"arch", sb.BuildArch)

	gcBuilder := NewGatewayControllerBuilder(
		sb.TempDir,
		sb.GatewayControllerBaseImage,
		sb.GatewayControllerOutputImage,
		sb.ImageTag,
		sb.BuildArch,
		sb.Policies,
		sb.BuilderVersion,
	)

	if err := gcBuilder.Build(); err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("gateway controller build failed: %w", err))
		result.Success = false
	} else {
		result.GatewayControllerImage = fmt.Sprintf("%s:%s",
			sb.GatewayControllerOutputImage, sb.ImageTag)
	}

	// 3. Tag Router Image
	slog.Info("Tagging router image",
		"baseImage", sb.RouterBaseImage,
		"outputImage", sb.RouterOutputImage,
		"tag", sb.ImageTag)

	routerTagger := NewRouterTagger(
		sb.RouterBaseImage,
		sb.RouterOutputImage,
		sb.ImageTag,
	)

	if err := routerTagger.Tag(); err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("router tag failed: %w", err))
		result.Success = false
	} else {
		result.RouterImage = fmt.Sprintf("%s:%s",
			sb.RouterOutputImage, sb.ImageTag)
	}

	if result.Success {
		slog.Info("Successfully built all Docker images", "phase", "docker-build")
	} else {
		slog.Error("Docker build completed with errors",
			"errorCount", len(result.Errors))
	}

	return result, nil
}
