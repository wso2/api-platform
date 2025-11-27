package docker

import (
	"fmt"
	"log/slog"
)

// RouterTagger tags the router Docker image
type RouterTagger struct {
	baseImage   string
	outputImage string
	imageTag    string
}

// NewRouterTagger creates a new router tagger
func NewRouterTagger(baseImage, outputImage, imageTag string) *RouterTagger {
	return &RouterTagger{
		baseImage:   baseImage,
		outputImage: outputImage,
		imageTag:    imageTag,
	}
}

// Tag tags the router image with a new name
func (t *RouterTagger) Tag() error {
	newImageName := fmt.Sprintf("%s:%s", t.outputImage, t.imageTag)

	slog.Info("Tagging router image",
		"baseImage", t.baseImage,
		"newImage", newImageName)

	if err := ExecuteDockerCommand("tag", t.baseImage, newImageName); err != nil {
		return fmt.Errorf("failed to tag router image: %w", err)
	}

	slog.Info("Successfully tagged router image",
		"image", newImageName)

	return nil
}
