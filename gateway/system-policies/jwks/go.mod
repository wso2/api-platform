module github.com/wso2/api-platform/gateway/system-policies/jwks

go 1.26.10

// Pinned to the version the policy engine itself requires, never ahead of it:
// the gateway builder compiles this module into the engine's plugin registry,
// so a higher requirement here would raise the engine's own SDK version as a
// side effect of adding a policy. v0.5.0 is the first sdk/core release with
// the signing package this policy relays.
require github.com/wso2/api-platform/sdk/core v0.5.0
