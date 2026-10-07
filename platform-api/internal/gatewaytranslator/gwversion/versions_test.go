/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package gwversion

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every Min* constant must be a parseable LTS version or NoLTSRelease, or
// AtLeast would silently compare against 1.0.0.
func TestMinVersionsParse(t *testing.T) {
	for name, v := range map[string]string{
		"MinGatewayV1Version":                    MinGatewayV1Version,
		"MinSecretSyncGatewayVersion":            MinSecretSyncGatewayVersion,
		"MinVerbatimMCPUpstreamPathVersion":      MinVerbatimMCPUpstreamPathVersion,
		"MinLLMUpstreamAuthTypeNoneOtherVersion": MinLLMUpstreamAuthTypeNoneOtherVersion,
		"MinMCPUpstreamAuthTypeNoneOtherVersion": MinMCPUpstreamAuthTypeNoneOtherVersion,
		"MinWebBrokerKindGatewayVersion":         MinWebBrokerKindGatewayVersion,
		"MinMCPSpecVersionListGatewayVersion":    MinMCPSpecVersionListGatewayVersion,
		"MinAgentKindGatewayVersion":             MinAgentKindGatewayVersion,
	} {
		if v == NoLTSRelease {
			continue
		}
		parsed, ok := Parse(v)
		require.Truef(t, ok, "%s = %q must parse", name, v)
		require.Truef(t, parsed.IsLTS(), "%s = %q must be an LTS release", name, v)
	}
}

// AtLeast is the rule every translator predicate shares: a blank, non-semver or
// STS (date-named) gateway version is a current build and satisfies every
// minimum; only a positively reported older LTS release fails. A capability no
// LTS release has fails on every LTS version.
func TestAtLeast(t *testing.T) {
	tests := []struct {
		raw  string
		min  string
		want bool
	}{
		{"", "1.2.0", true},
		{"   ", "1.2.0", true},
		{"it-e2e", "1.2.0", true},
		{"not-a-version", NoLTSRelease, true},
		{"1.0.0", "1.2.0", false},
		{"1.1.0", "1.2.0", false},
		{"1.1.9", "1.2.0", false},
		{"1.2.0", "1.2.0", true},
		{"1.2.0-rc", "1.2.0", true},
		{"1.2", "1.2.0", true},
		{"1.3.0", "1.2.0", true},
		{"1.0.0", NoLTSRelease, false},
		{"1.2.0", NoLTSRelease, false},
		{"1.3.0", NoLTSRelease, false},
		{"", NoLTSRelease, true},
		// STS releases are not compared yet (issue #3681): treated as current.
		{"2026.05.13", "1.2.0", true},
		{"2026.05.13", NoLTSRelease, true},
	}
	for _, tt := range tests {
		t.Run(tt.raw+"_vs_"+tt.min, func(t *testing.T) {
			assert.Equal(t, tt.want, AtLeast(tt.raw, tt.min))
			assert.Equal(t, !tt.want, Below(tt.raw, tt.min))
		})
	}
}

func TestSecretPredicates(t *testing.T) {
	for _, raw := range []string{"1.0.0", "1.1.0", "1.1.9", "1.1.0-SNAPSHOT"} {
		assert.Falsef(t, SupportsSecretSync(raw), "%s cannot pull secrets from the control plane", raw)
		assert.Truef(t, RequiresInlineSecrets(raw), "%s needs plaintext secrets", raw)
	}
	for _, raw := range []string{"1.2.0", "1.2.0-rc", "1.3.0", "", "it-e2e"} {
		assert.Truef(t, SupportsSecretSync(raw), "%q syncs secrets itself", raw)
		assert.Falsef(t, RequiresInlineSecrets(raw), "%q must keep its placeholders", raw)
	}
}

func TestPhrases(t *testing.T) {
	assert.Equal(t, "gateways below 1.2.0", Gateways("1.2.0"))
	assert.Equal(t, "LTS gateways", Gateways(NoLTSRelease))
	assert.Equal(t, "gateway version 1.2.0 or newer is required", Requirement("1.2.0"))
	assert.Equal(t, "no LTS gateway release supports them yet", Requirement(NoLTSRelease))
}
