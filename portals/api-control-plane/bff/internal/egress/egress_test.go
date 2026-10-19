/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package egress

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// policyFor builds a development policy: most cases here are about host
// matching, and development is the mode that does not also refuse the
// loopback and private addresses those cases use.
func policyFor(t *testing.T, hosts ...string) *Policy {
	t.Helper()
	p, err := Parse(Spec{Environment: EnvDevelopment, AllowHosts: hosts})
	require.NoError(t, err)
	return p
}

// prodPolicyFor builds a production policy.
func prodPolicyFor(t *testing.T, hosts ...string) *Policy {
	t.Helper()
	p, err := Parse(Spec{Environment: EnvProduction, AllowHosts: hosts})
	require.NoError(t, err)
	return p
}

func check(t *testing.T, p *Policy, rawURL string) error {
	t.Helper()
	u, err := url.Parse(rawURL)
	require.NoError(t, err)
	return p.CheckTarget(u)
}

func TestEmptyAllowHostsReachesNothing(t *testing.T) {
	// The whole point of the model: a deployment that configures no policy
	// refuses every target, rather than dialing wherever a gateway record
	// happens to point.
	p := policyFor(t)

	for _, target := range []string{
		"https://gw.example.com/pizza",
		"http://10.1.2.3:8080/pizza",
		"https://anything.at.all",
	} {
		assert.ErrorIs(t, check(t, p, target), ErrTargetNotAllowed, target)
	}
}

func TestExactHostEntry(t *testing.T) {
	p := policyFor(t, "gw.example.com")

	assert.NoError(t, check(t, p, "https://gw.example.com/pizza"))
	assert.NoError(t, check(t, p, "http://gw.example.com:8080/pizza"), "no scheme or port named, so neither constrains")
	assert.NoError(t, check(t, p, "https://GW.Example.COM/pizza"), "host matching is case-insensitive")
	assert.NoError(t, check(t, p, "https://gw.example.com./pizza"), "a trailing root dot is the same host")

	assert.ErrorIs(t, check(t, p, "https://other.example.com/pizza"), ErrTargetNotAllowed)
	assert.ErrorIs(t, check(t, p, "https://gw.example.com.evil.net/pizza"), ErrTargetNotAllowed)
}

func TestSchemeConstrainsOnlyWhenWritten(t *testing.T) {
	pinned := policyFor(t, "https://gw.example.com")
	assert.NoError(t, check(t, pinned, "https://gw.example.com/x"))
	assert.ErrorIs(t, check(t, pinned, "http://gw.example.com/x"), ErrTargetNotAllowed)

	open := policyFor(t, "gw.example.com")
	assert.NoError(t, check(t, open, "http://gw.example.com/x"))
}

func TestPortConstrainsOnlyWhenWritten(t *testing.T) {
	pinned := policyFor(t, "gw.example.com:8443")
	assert.NoError(t, check(t, pinned, "https://gw.example.com:8443/x"))
	assert.ErrorIs(t, check(t, pinned, "https://gw.example.com:9200/x"), ErrTargetNotAllowed)

	open := policyFor(t, "gw.example.com")
	assert.NoError(t, check(t, open, "https://gw.example.com:9200/x"))
}

func TestPortDefaultsToTheSchemeDefault(t *testing.T) {
	// "https://gw.example.com" dials 443, so an entry pinning 443 must match
	// it even though the URL names no port.
	https := policyFor(t, "gw.example.com:443")
	assert.NoError(t, check(t, https, "https://gw.example.com/x"))
	assert.ErrorIs(t, check(t, https, "http://gw.example.com/x"), ErrTargetNotAllowed)

	http := policyFor(t, "gw.example.com:80")
	assert.NoError(t, check(t, http, "http://gw.example.com/x"))
}

func TestWildcardMatchesAnywhereInTheHost(t *testing.T) {
	for _, tc := range []struct {
		name    string
		entry   string
		target  string
		allowed bool
	}{
		{"leading wildcard, one label", "*.gw.example.com", "https://a.gw.example.com/x", true},
		{"leading wildcard, several labels", "*.gw.example.com", "https://a.b.gw.example.com/x", true},
		{"leading wildcard excludes the parent", "*.gw.example.com", "https://gw.example.com/x", false},
		{"leading wildcard is not a substring test", "*.gw.example.com", "https://evilgw.example.com/x", false},
		{"leading wildcard excludes a suffixed lookalike", "*.gw.example.com", "https://a.gw.example.com.evil.net/x", false},
		{"interior wildcard", "gw-*.example.com", "https://gw-eu.example.com/x", true},
		{"interior wildcard needs the prefix", "gw-*.example.com", "https://gw.example.com/x", false},
		{"trailing wildcard", "*.gw.*", "https://a.gw.internal/x", true},
		{"bare star matches everything", "*", "https://anything.at.all:9999/x", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := check(t, policyFor(t, tc.entry), tc.target)
			if tc.allowed {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, ErrTargetNotAllowed)
		})
	}
}

func TestWildcardCombinesWithSchemeAndPort(t *testing.T) {
	p := policyFor(t, "https://*.gw.svc.cluster.local:9443")

	assert.NoError(t, check(t, p, "https://a.gw.svc.cluster.local:9443/pizza"))
	assert.ErrorIs(t, check(t, p, "http://a.gw.svc.cluster.local:9443/pizza"), ErrTargetNotAllowed)
	assert.ErrorIs(t, check(t, p, "https://a.gw.svc.cluster.local:9200/pizza"), ErrTargetNotAllowed)
	assert.ErrorIs(t, check(t, p, "https://a.other.svc.cluster.local:9443/pizza"), ErrTargetNotAllowed)
}

func TestAnyMatchingEntryAllows(t *testing.T) {
	p := policyFor(t, "https://gw.eu.example.com:443", "https://gw.us.example.com:443")

	assert.NoError(t, check(t, p, "https://gw.eu.example.com/x"))
	assert.NoError(t, check(t, p, "https://gw.us.example.com/x"))
	assert.ErrorIs(t, check(t, p, "https://gw.ap.example.com/x"), ErrTargetNotAllowed)
}

func TestIPLiteralEntries(t *testing.T) {
	v4 := policyFor(t, "10.1.2.3:8080")
	assert.NoError(t, check(t, v4, "http://10.1.2.3:8080/x"))
	assert.ErrorIs(t, check(t, v4, "http://10.1.2.4:8080/x"), ErrTargetNotAllowed)

	// An IPv6 literal is bracketed in both the entry and the URL, so the
	// address's own colons are not read as a port separator.
	v6 := policyFor(t, "[fd00::1]:8443")
	assert.NoError(t, check(t, v6, "https://[fd00::1]:8443/x"))
	assert.ErrorIs(t, check(t, v6, "https://[fd00::2]:8443/x"), ErrTargetNotAllowed)
}

func TestCheckTargetRejectsAURLWithNoHost(t *testing.T) {
	p := policyFor(t, "*")
	u, err := url.Parse("/just/a/path")
	require.NoError(t, err)
	assert.ErrorIs(t, p.CheckTarget(u), ErrNoTargetHost)
	assert.ErrorIs(t, p.CheckTarget(nil), ErrNoTargetHost)
}

func TestParseRejectsEntriesThatWouldNotDoWhatTheyRead(t *testing.T) {
	for _, tc := range []struct {
		name  string
		entry string
		says  string
	}{
		{"empty", "", "empty entry"},
		{"whitespace only", "   ", "empty entry"},
		{"unsupported scheme", "ftp://gw.example.com", "only http and https"},
		{"carries a path", "gw.example.com/pizza", "path or query"},
		{"carries a query", "gw.example.com?a=1", "path or query"},
		{"port out of range", "gw.example.com:70000", "not a valid TCP port"},
		{"port not a number", "gw.example.com:https", "not a valid TCP port"},
		{"unbracketed ipv6", "fd00::1:8443", "bracket an IPv6 literal"},
		{"no host", "https://", "names no host"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(Spec{AllowHosts: []string{tc.entry}})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.says)
		})
	}
}

func TestAlwaysRefusedRangesAreNotConfigurable(t *testing.T) {
	// allow_hosts matches names. A name an operator trusts can still resolve
	// to the metadata endpoint, so the floor is checked against the resolved
	// address and no entry can re-open it.
	p := policyFor(t, "*")
	ng := p.NetguardPolicy()

	assert.True(t, ng.BlockLinkLocal, "link-local carries the cloud metadata endpoint")
	assert.True(t, ng.BlockUnspecified)
	assert.True(t, ng.BlockMulticastBroadcast)
	assert.Empty(t, ng.AllowCIDRs, "a widening list would let an entry re-open a refused range")

	var has6to4, hasNAT64 bool
	for _, cidr := range ng.DenyCIDRs {
		switch cidr.String() {
		case "2002::/16":
			has6to4 = true
		case "64:ff9b::/96":
			hasNAT64 = true
		}
	}
	assert.True(t, has6to4, "2002:a9fe:a9fe:: reaches 169.254.169.254 without being link-local")
	assert.True(t, hasNAT64)
}

func TestDevelopmentKeepsPrivateAndLoopbackDialable(t *testing.T) {
	// A gateway in development genuinely is on localhost or a ClusterIP.
	ng := policyFor(t, "*").NetguardPolicy()

	assert.False(t, ng.BlockPrivate)
	assert.False(t, ng.BlockLoopback)
	assert.False(t, ng.BlockCGNAT)
	assert.ElementsMatch(t, []string{"http", "https"}, ng.AllowedSchemes)
}

func TestProductionRefusesPrivateLoopbackAndCGNAT(t *testing.T) {
	// Where everything else in a cluster lives, and never a gateway reachable
	// from a production control plane.
	ng := prodPolicyFor(t, "*").NetguardPolicy()

	assert.True(t, ng.BlockPrivate)
	assert.True(t, ng.BlockLoopback)
	assert.True(t, ng.BlockCGNAT)
}

func TestProductionRefusesAnIPLiteralTarget(t *testing.T) {
	// A literal sidesteps the naming allow_hosts is written in terms of, and
	// a public literal would not be caught by the address checks at all.
	p := prodPolicyFor(t, "*")

	for _, target := range []string{
		"http://10.0.0.5/x",
		"http://127.0.0.1:9780/x",
		"http://169.254.169.254/x",
		"https://203.0.113.5/x", // public, so only this rule refuses it
		"http://[fd00::1]:8443/x",
		"http://[::1]:8080/x",
	} {
		assert.ErrorIs(t, check(t, p, target), ErrIPLiteralTarget, target)
	}
}

func TestProductionStillAllowsNamedHosts(t *testing.T) {
	p := prodPolicyFor(t, "https://*.gw.example.com:443")

	assert.NoError(t, check(t, p, "https://a.gw.example.com/pizza"))
	assert.ErrorIs(t, check(t, p, "https://a.other.example.com/pizza"), ErrTargetNotAllowed)
}

func TestDevelopmentAllowsAnIPLiteralTarget(t *testing.T) {
	p := policyFor(t, "*")

	assert.NoError(t, check(t, p, "http://127.0.0.1:8080/x"))
	assert.NoError(t, check(t, p, "http://10.0.0.5/x"))
}

func TestEnvironmentDefaultsToProduction(t *testing.T) {
	// Absent and empty both mean production: the stricter stance has to be
	// what a deployment gets by forgetting to choose.
	for _, spec := range []Spec{
		{AllowHosts: []string{"*"}},
		{Environment: "", AllowHosts: []string{"*"}},
		{Environment: "  PRODUCTION  ", AllowHosts: []string{"*"}},
	} {
		p, err := Parse(spec)
		require.NoError(t, err)
		assert.True(t, p.NetguardPolicy().BlockPrivate)
		assert.ErrorIs(t, check(t, p, "http://10.0.0.5/x"), ErrIPLiteralTarget)
	}
}

func TestUnknownEnvironmentRefusesToStart(t *testing.T) {
	// A typo must not quietly land on the permissive mode.
	for _, env := range []string{"prod", "dev", "staging", "Production!"} {
		_, err := Parse(Spec{Environment: env, AllowHosts: []string{"*"}})
		assert.ErrorIs(t, err, ErrInvalidEnv, env)
	}
}

func TestMetadataFloorHoldsInBothEnvironments(t *testing.T) {
	// The one thing environment never relaxes.
	for _, p := range []*Policy{policyFor(t, "*"), prodPolicyFor(t, "*")} {
		ng := p.NetguardPolicy()
		assert.True(t, ng.BlockLinkLocal)
		assert.True(t, ng.BlockUnspecified)
		assert.True(t, ng.BlockMulticastBroadcast)
		assert.Empty(t, ng.AllowCIDRs)
	}
}

func TestDescribeNamesTheEffectivePolicy(t *testing.T) {
	empty := policyFor(t).Describe()
	assert.Contains(t, empty, "nothing is reachable", "an operator reading the log should see why every call fails")

	set := policyFor(t, "https://*.gw.internal:9443").Describe()
	assert.Contains(t, set, "https://*.gw.internal:9443")
	assert.Contains(t, set, "environment=development")
	assert.Contains(t, prodPolicyFor(t, "a.b.com").Describe(), "environment=production")
}

func TestGlobMatch(t *testing.T) {
	for _, tc := range []struct {
		pattern, input string
		want           bool
	}{
		{"*", "", true},
		{"*", "anything", true},
		{"a*c", "abc", true},
		{"a*c", "ac", true},
		{"a*c", "abdc", true},
		{"a*c", "abd", false},
		{"*.example.com", ".example.com", true},
		{"*.example.com", "a.b.example.com", true},
		{"*.example.com", "example.com", false},
		{"exact", "exact", true},
		{"exact", "exactly", false},
		{"**", "anything", true},
		{"*a*b*", "xaybz", true},
		{"*a*b*", "xbya", false},
	} {
		t.Run(tc.pattern+"/"+tc.input, func(t *testing.T) {
			assert.Equal(t, tc.want, globMatch(tc.pattern, tc.input))
		})
	}
}

func TestRefusalDoesNotNameTheCheckThatFired(t *testing.T) {
	// The handler answers generically; this is only about what the error
	// carries for the log. It should name the target, not the policy.
	err := check(t, policyFor(t, "gw.example.com"), "https://other.example.com/x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "other.example.com")
	assert.False(t, strings.Contains(err.Error(), "gw.example.com"),
		"the log line should not print the allowlist back on every refusal")
}
