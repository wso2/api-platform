package tests

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type testEntry struct {
	Name    string `yaml:"name"`
	Enabled bool   `yaml:"enabled"`
}

type testConfig struct {
	Tests []testEntry `yaml:"tests"`
}

// isTestEnabled reads ../test-config.yaml and returns whether the named test
// is enabled. If the config cannot be read or the test is not listed, it
// conservatively returns false (disabled).
func isTestEnabled(name string) bool {
	cfgPath := filepath.Join("..", "test-config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return false
	}
	var cfg testConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return false
	}
	for _, t := range cfg.Tests {
		if t.Name == name {
			return t.Enabled
		}
	}
	return false
}
