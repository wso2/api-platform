package types

// DockerBuildOptions contains options for building Docker images
type DockerBuildOptions struct {
	TempDir                string
	BinaryPath             string
	Policies               []*DiscoveredPolicy
	PolicyEngineImage      string
	GatewayControllerImage string
	RouterImage            string
	ImageTag               string
	BuilderVersion         string
}

// DockerBuildResult contains the results of building Docker images
type DockerBuildResult struct {
	PolicyEngineImage      string
	GatewayControllerImage string
	RouterImage            string
	ManifestPath           string
	Success                bool
	Errors                 []error
}
