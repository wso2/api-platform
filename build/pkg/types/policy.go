package types

import "time"

// DiscoveredPolicy represents a policy found during the discovery phase
type DiscoveredPolicy struct {
	Name        string
	Version     string
	Path        string
	YAMLPath    string
	GoModPath   string
	SourceFiles []string
	Definition  *PolicyDefinition
}

// PolicyDefinition mirrors the structure from policy.yaml files
type PolicyDefinition struct {
	Name        string                 `yaml:"name"`
	Version     string                 `yaml:"version"`
	Description string                 `yaml:"description"`
	Category    string                 `yaml:"category"` // TODO: (renuka) change this to Tags an make this an array of string. This is only for documentation purpose.
	Parameters  map[string]interface{} `yaml:"parameters"`
	Condition   *ConditionDef          `yaml:"executionCondition,omitempty"`
	Body        *BodyRequirements      `yaml:"bodyRequirements,omitempty"`
	Examples    []interface{}          `yaml:"examples,omitempty"`
}

// ConditionDef represents execution conditions
type ConditionDef struct {
	Supported bool `yaml:"supported"`
}

// BodyRequirements specifies body processing needs
type BodyRequirements struct {
	Request  bool `yaml:"request"`
	Response bool `yaml:"response"`
}

// ValidationResult contains validation errors and warnings
type ValidationResult struct {
	Valid    bool
	Errors   []ValidationError
	Warnings []ValidationWarning
}

// ValidationError represents a validation failure
type ValidationError struct {
	PolicyName    string
	PolicyVersion string
	FilePath      string
	LineNumber    int
	Message       string
}

// ValidationWarning represents a non-blocking validation issue
type ValidationWarning struct {
	PolicyName    string
	PolicyVersion string
	FilePath      string
	Message       string
}

// BuildMetadata contains information about the build
type BuildMetadata struct {
	Timestamp      time.Time
	BuilderVersion string
	Policies       []PolicyInfo
}

// PolicyInfo contains basic policy information for build metadata
type PolicyInfo struct {
	Name    string
	Version string
	Path    string
}

// CompilationOptions contains settings for the compilation phase
type CompilationOptions struct {
	OutputPath string
	EnableUPX  bool
	LDFlags    string
	BuildTags  []string
	CGOEnabled bool
	TargetOS   string
	TargetArch string
}

// PackagingMetadata contains Docker image metadata
type PackagingMetadata struct {
	BaseImage      string
	Labels         map[string]string
	BuildTimestamp time.Time
	Policies       []PolicyInfo
}
