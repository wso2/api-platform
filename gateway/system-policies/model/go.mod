module github.com/wso2/api-platform/gateway/system-policies/model

go 1.26.5

// Pinned to the version the policy engine itself requires, never ahead of it.
// The gateway builder compiles this module into the engine's plugin registry,
// so a higher requirement here would raise the engine's own SDK version as a
// side effect of adding a policy.
//
// v0.4.1 is the version the a2a system policy pins for the same reason, and
// every SDK surface this policy uses — SharedContext.ResolutionAttributes, a
// body-phase UpstreamName, the streaming response interface — is already in it.
require github.com/wso2/api-platform/sdk/core v0.4.1
