package templates

import (
	_ "embed"
)

// Embedded template files
// These files are embedded at compile time from the same directory
// The actual .tmpl files remain in the directory for easy editing

//go:embed plugin_registry.go.tmpl
var PluginRegistryTemplate string

//go:embed build_info.go.tmpl
var BuildInfoTemplate string

//go:embed Dockerfile.runtime.tmpl
var DockerfileRuntimeTemplate string
