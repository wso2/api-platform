module github.com/wso2/api-platform/gateway/system-policies/budget

go 1.26.10

// Pinned to the versions the policy engine itself requires, never ahead of it: the
// gateway builder compiles this module into the engine's plugin registry, so a higher
// requirement here would raise the engine's own versions as a side effect.
require (
	cel.dev/cel-go v0.33.1
	github.com/google/uuid v1.6.0
	github.com/wso2/api-platform/sdk/core v0.5.0
)

require (
	cel.dev/expr v0.25.1 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/exp v0.0.0-20240823005443-9b4947da3948 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240826202546-f6391c0de4c7 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240826202546-f6391c0de4c7 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)
