package utils

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

const (
	testSetHeadersVersion = "v9.9.9"
	testRespondVersion    = "v9.9.8"
)

func newTestPolicyVersionResolver() PolicyVersionResolver {
	return NewStaticPolicyVersionResolver(map[string]string{
		constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME: testSetHeadersVersion,
		constants.ACCESS_CONTROL_DENY_POLICY_NAME:  testRespondVersion,
	})
}

// TestPolicyDefinitionMissingError_Error verifies the error message format.
func TestPolicyDefinitionMissingError_Error(t *testing.T) {
	err := &PolicyDefinitionMissingError{PolicyName: "my-policy"}
	assert.Contains(t, err.Error(), "my-policy")
}

// TestIsPolicyDefinitionMissingError verifies detection of the sentinel error type.
func TestIsPolicyDefinitionMissingError(t *testing.T) {
	err := &PolicyDefinitionMissingError{PolicyName: "test"}
	assert.True(t, IsPolicyDefinitionMissingError(err))

	wrapped := errors.New("other error")
	assert.False(t, IsPolicyDefinitionMissingError(wrapped))

	wrappedMissing := fmt.Errorf("wrap: %w", err)
	assert.True(t, IsPolicyDefinitionMissingError(wrappedMissing))
}

// TestNewLoadedPolicyVersionResolver verifies that the highest semver is selected
// and converted to a major-only version string.
func TestNewLoadedPolicyVersionResolver(t *testing.T) {
	defs := map[string]models.PolicyDefinition{
		"rate-limit|v1.0.0": {Name: "rate-limit", Version: "v1.0.0"},
		"rate-limit|v2.3.0": {Name: "rate-limit", Version: "v2.3.0"},
		"rate-limit|v2.1.0": {Name: "rate-limit", Version: "v2.1.0"},
		"auth|v0.5.0":       {Name: "auth", Version: "v0.5.0"},
	}

	resolver := NewLoadedPolicyVersionResolver(defs)
	require.NotNil(t, resolver)

	// The highest version for "rate-limit" is v2.3.0, converted to major-only "v2"
	v, err := resolver.Resolve("rate-limit")
	require.NoError(t, err)
	assert.Equal(t, "v2", v)

	// auth has a single version v0.5.0, converted to "v0"
	v, err = resolver.Resolve("auth")
	require.NoError(t, err)
	assert.Equal(t, "v0", v)
}

// TestNewLoadedPolicyVersionResolver_EmptyDefs returns an error for missing policy.
func TestNewLoadedPolicyVersionResolver_EmptyDefs(t *testing.T) {
	resolver := NewLoadedPolicyVersionResolver(map[string]models.PolicyDefinition{})
	_, err := resolver.Resolve("missing-policy")
	require.Error(t, err)
	assert.True(t, IsPolicyDefinitionMissingError(err))
}

// TestStaticPolicyVersionResolver_Resolve covers success and not-found paths.
func TestStaticPolicyVersionResolver_Resolve(t *testing.T) {
	resolver := NewStaticPolicyVersionResolver(map[string]string{
		"my-policy": "v1",
	})

	v, err := resolver.Resolve("my-policy")
	require.NoError(t, err)
	assert.Equal(t, "v1", v)

	_, err = resolver.Resolve("missing")
	require.Error(t, err)
	assert.True(t, IsPolicyDefinitionMissingError(err))
}

// TestStaticPolicyVersionResolver_NilReceiver verifies nil safety.
func TestStaticPolicyVersionResolver_NilReceiver(t *testing.T) {
	var r *StaticPolicyVersionResolver
	_, err := r.Resolve("anything")
	require.Error(t, err)
	assert.True(t, IsPolicyDefinitionMissingError(err))
}
