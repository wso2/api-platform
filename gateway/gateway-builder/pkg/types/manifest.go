package types

// PolicyManifest represents the policy.yaml manifest file
type PolicyManifest struct {
	Version  string          `yaml:"version"`
	Policies []ManifestEntry `yaml:"policies"`
}

// ManifestEntry represents a single policy entry in the manifest
type ManifestEntry struct {
	Name     string `yaml:"name"`
	Version  string `yaml:"version"`
	FilePath string `yaml:"filePath"`
}
