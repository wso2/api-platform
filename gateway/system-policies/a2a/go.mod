module github.com/wso2/api-platform/gateway/system-policies/a2a

go 1.26.5

// Pinned to the version the policy engine itself requires rather than the
// latest (v0.3.5). The gateway builder compiles this module into the engine's
// plugin registry, so a higher requirement here would raise the engine's own
// SDK version as a side effect of adding a policy.
require github.com/wso2/api-platform/sdk/core v0.3.3
