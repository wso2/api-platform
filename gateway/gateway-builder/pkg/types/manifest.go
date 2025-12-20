package types

// PolicyManifestLock represents the policy-manifest-lock.yaml file
type PolicyManifestLock struct {
	Version  string          `yaml:"version"`
	Policies []ManifestEntry `yaml:"policies"`
}

// ManifestEntry represents a single policy entry in the manifest lock
type ManifestEntry struct {
	Name     string `yaml:"name"`
	Version  string `yaml:"version"`
	FilePath string `yaml:"filePath"`
}
