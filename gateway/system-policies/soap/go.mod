module github.com/wso2/api-platform/gateway/system-policies/soap

go 1.26.11

// Pinned to the version the policy engine itself requires, never ahead of it:
// the gateway builder compiles this module into the engine's plugin registry,
// so a higher requirement here would raise the engine's own SDK version as a
// side effect of adding a policy.
require github.com/wso2/api-platform/sdk/core v0.5.0

// Latest release. It sits above the v0.40.0 the policy engine resolves today, which
// carries GO-2026-6629 (x/text/secure/precis, fixed in v0.41.0), so building this policy
// into the engine also lifts the engine onto the fixed version.
require golang.org/x/text v0.43.0
