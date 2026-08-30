module github.com/wso2/api-platform/gateway/system-policies/a2a

go 1.26.5

// Pinned to the version the policy engine itself requires, never ahead of it.
// The gateway builder compiles this module into the engine's plugin registry,
// so a higher requirement here would raise the engine's own SDK version as a
// side effect of adding a policy.
//
// v0.4.1 is what the engine requires today, and it is the first version to
// expose SharedContext.ResolutionAttributes — which is how the protected Agent
// Card responder learns which A2A binding it is answering, from the resolver
// that already decided it, rather than re-deriving it from the request.
require github.com/wso2/api-platform/sdk/core v0.4.1
