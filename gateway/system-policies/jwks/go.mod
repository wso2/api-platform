module github.com/wso2/api-platform/gateway/system-policies/jwks

go 1.26.5

// Pinned to the version the policy engine itself requires, never ahead of it:
// the gateway builder compiles this module into the engine's plugin registry,
// so a higher requirement here would raise the engine's own SDK version as a
// side effect of adding a policy. This policy needs the sdk/core signing
// package, so it moves with the engine to the first sdk/core release that
// ships it.
require github.com/wso2/api-platform/sdk/core v0.4.3
