package deploymenttransform

import (
	"testing"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// ParseVersion / GTE
// ---------------------------------------------------------------------------

func TestParseVersion(t *testing.T) {
	minVer := ParseVersion(MinSplitPoliciesVersion)
	tests := []struct {
		version string
		wantGTE bool // whether ParseVersion(version).GTE(minVer)
	}{
		// Old / empty → treated as 1.0.0
		{"", false},
		{"1.0.0", false},
		{"1.1.0", false},
		{"v1.1.0", false},
		{"1.1.9", false},
		// Boundary — 1.2.x is new
		{"1.2.0", true},
		{"v1.2.0", true},
		{"1.2.0-SNAPSHOT", true},
		{"1.2.1", true},
		{"1.3.0", true},
		{"2.0.0", true},
		// Unparseable → treated as 1.0.0 (old)
		{"not-a-version", false},
		{"abc", false},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			assert.Equal(t, tt.wantGTE, ParseVersion(tt.version).GTE(minVer))
		})
	}
}

// ---------------------------------------------------------------------------
// Helper spec builders
// ---------------------------------------------------------------------------

func newProviderArtifact(global []api.Policy, operation []api.OperationPolicy, legacy []api.LLMPolicy) *dto.LLMProviderDeploymentYAML {
	return &dto.LLMProviderDeploymentYAML{
		ApiVersion: constants.GatewayApiVersion,
		Kind:       constants.LLMProvider,
		Spec: dto.LLMProviderDeploymentSpec{
			DisplayName:       "test",
			GlobalPolicies:    global,
			OperationPolicies: operation,
			Policies:          legacy,
		},
	}
}

func newProxyArtifact(global []api.Policy, operation []api.OperationPolicy, legacy []api.LLMPolicy) *dto.LLMProxyDeploymentYAML {
	return &dto.LLMProxyDeploymentYAML{
		ApiVersion: constants.GatewayApiVersion,
		Kind:       constants.LLMProxy,
		Spec: dto.LLMProxyDeploymentSpec{
			DisplayName:       "test",
			GlobalPolicies:    global,
			OperationPolicies: operation,
			Policies:          legacy,
		},
	}
}

func sampleGlobal() []api.Policy {
	return []api.Policy{{Name: "basic-ratelimit", Version: "v1", Params: &map[string]interface{}{"requests": 10}}}
}

func sampleOperation() []api.OperationPolicy {
	return []api.OperationPolicy{{
		Name:    "basic-ratelimit",
		Version: "v1",
		Paths:   []api.OperationPolicyPath{{Path: "/chat/completions", Methods: []api.OperationPolicyPathMethods{api.OperationPolicyPathMethodsGET}}},
	}}
}

func sampleLegacy() []api.LLMPolicy {
	return []api.LLMPolicy{{Name: "basic-ratelimit", Version: "v1",
		Paths: []api.LLMPolicyPath{{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}}}}}
}

// ---------------------------------------------------------------------------
// Registry.Transform — new gateway (≥ 1.2.0): canonical spec passes through
//
// In normal operation the generator (Phase 8a) always produces the canonical
// shape (globalPolicies + operationPolicies, policies=nil) before Transform is
// called. Transform is a no-op for new gateways — it has no registered
// Transformation for them — so the canonical payload is stored unchanged.
// ---------------------------------------------------------------------------

func TestTransform_Provider_NewGateway_CanonicalPassthrough(t *testing.T) {
	artifact := newProviderArtifact(sampleGlobal(), sampleOperation(), nil)
	err := Default().Transform(constants.LLMProvider, ParseVersion(MinSplitPoliciesVersion), artifact)
	require.NoError(t, err)
	// New gateway: no-op — canonical shape and apiVersion preserved as-is.
	assert.Equal(t, constants.GatewayApiVersion, artifact.ApiVersion)
	assert.Len(t, artifact.Spec.GlobalPolicies, 1)
	assert.Len(t, artifact.Spec.OperationPolicies, 1)
	assert.Nil(t, artifact.Spec.Policies)
}

func TestTransform_Proxy_NewGateway_CanonicalPassthrough(t *testing.T) {
	artifact := newProxyArtifact(sampleGlobal(), sampleOperation(), nil)
	err := Default().Transform(constants.LLMProxy, ParseVersion("1.2.0-SNAPSHOT"), artifact)
	require.NoError(t, err)
	assert.Equal(t, constants.GatewayApiVersion, artifact.ApiVersion)
	assert.Len(t, artifact.Spec.GlobalPolicies, 1)
	assert.Len(t, artifact.Spec.OperationPolicies, 1)
	assert.Nil(t, artifact.Spec.Policies)
}

// ---------------------------------------------------------------------------
// Registry.Transform — old gateway (< 1.2.0): flattens to legacy policies
// ---------------------------------------------------------------------------

func TestTransform_Provider_OldGateway_FlattensToLegacy(t *testing.T) {
	artifact := newProviderArtifact(sampleGlobal(), sampleOperation(), nil)
	err := Default().Transform(constants.LLMProvider, ParseVersion("1.1.0"), artifact)
	require.NoError(t, err)
	// Old gateway: apiVersion downgraded and policies flattened.
	assert.Equal(t, constants.GatewayApiVersionV1Alpha1, artifact.ApiVersion)
	assert.Nil(t, artifact.Spec.GlobalPolicies)
	assert.Nil(t, artifact.Spec.OperationPolicies)
	require.Len(t, artifact.Spec.Policies, 2)
	// Global policy → legacy entry with path "/*", methods ["*"]
	var wildcardEntry, chatEntry *api.LLMPolicy
	for i := range artifact.Spec.Policies {
		if len(artifact.Spec.Policies[i].Paths) > 0 && artifact.Spec.Policies[i].Paths[0].Path == "/*" {
			wildcardEntry = &artifact.Spec.Policies[i]
		} else {
			chatEntry = &artifact.Spec.Policies[i]
		}
	}
	require.NotNil(t, wildcardEntry, "global policy must produce a /* legacy entry")
	assert.Equal(t, api.LLMPolicyPathMethods("*"), wildcardEntry.Paths[0].Methods[0])
	require.NotNil(t, chatEntry, "operation policy must be preserved with its path")
	assert.Equal(t, "/chat/completions", chatEntry.Paths[0].Path)
}

func TestTransform_Provider_EmptyVersion_TreatedAsOld(t *testing.T) {
	// Empty version string → ParseVersion returns 1.0.0 → old gateway.
	artifact := newProviderArtifact(sampleGlobal(), nil, nil)
	err := Default().Transform(constants.LLMProvider, ParseVersion(""), artifact)
	require.NoError(t, err)
	assert.Equal(t, constants.GatewayApiVersionV1Alpha1, artifact.ApiVersion)
	assert.Nil(t, artifact.Spec.GlobalPolicies)
	require.Len(t, artifact.Spec.Policies, 1)
	assert.Equal(t, "/*", artifact.Spec.Policies[0].Paths[0].Path)
}

func TestTransform_Provider_OldGateway_AppendedToExistingLegacy(t *testing.T) {
	// If the spec already has legacy policies (e.g. security/consumer entries
	// that are present when the old-gateway path is tested in isolation),
	// down-convert appends to them then re-orders.
	existing := sampleLegacy()
	artifact := newProviderArtifact(sampleGlobal(), nil, existing)
	err := Default().Transform(constants.LLMProvider, ParseVersion("1.0.0"), artifact)
	require.NoError(t, err)
	assert.Equal(t, constants.GatewayApiVersionV1Alpha1, artifact.ApiVersion)
	assert.Nil(t, artifact.Spec.GlobalPolicies)
	// existing (1) + global flattened (1) = 2
	assert.Len(t, artifact.Spec.Policies, 2)
}

func TestTransform_Proxy_OldGateway_FlattensToLegacy(t *testing.T) {
	artifact := newProxyArtifact(sampleGlobal(), nil, nil)
	err := Default().Transform(constants.LLMProxy, ParseVersion("1.0.0"), artifact)
	require.NoError(t, err)
	assert.Equal(t, constants.GatewayApiVersionV1Alpha1, artifact.ApiVersion)
	assert.Nil(t, artifact.Spec.GlobalPolicies)
	require.Len(t, artifact.Spec.Policies, 1)
	assert.Equal(t, "/*", artifact.Spec.Policies[0].Paths[0].Path)
}

// ---------------------------------------------------------------------------
// Registry dispatch — no-op for unknown kind; type error for wrong payload
// ---------------------------------------------------------------------------

func TestTransform_UnknownKind_Noop(t *testing.T) {
	artifact := newProviderArtifact(sampleGlobal(), nil, nil)
	err := Default().Transform("UnknownKind", ParseVersion("1.0.0"), artifact)
	require.NoError(t, err)
	// Unknown kind: no-op — artifact unchanged.
	assert.Equal(t, constants.GatewayApiVersion, artifact.ApiVersion)
	assert.Len(t, artifact.Spec.GlobalPolicies, 1)
}

func TestTransform_WrongPayloadType_ReturnsError(t *testing.T) {
	// Passing a *dto.LLMProviderDeploymentYAML where LLMProxy is expected
	// triggers the type-assertion guard in Apply.
	artifact := newProviderArtifact(sampleGlobal(), nil, nil)
	err := Default().Transform(constants.LLMProxy, ParseVersion("1.0.0"), artifact)
	assert.Error(t, err)
}
