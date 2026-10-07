/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package faultformat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// The shipped default, and the one list in this package that is wire-visible for every client
// of a kind. Decision 5 from the third design review set the rule: a kind formats nothing
// unless its protocol leaves the caller unable to read anything else, and named SoapApi and
// A2A as the two that would qualify when they landed.
//
// A2A has landed, so Agent is here and nothing else is. The qualifying part is not that A2A
// is new — it is that an Agent route serving JSON-RPC has no other way to answer. MCP fixes a
// protocol too and is still absent, because its policies write the JSON-RPC envelope
// themselves; A2A has no such policies, so a rejection there comes from an ordinary policy
// writing ordinary JSON to a client that can only read JSON-RPC.
//
// Asserted rather than left to the declaration because adding a kind here changes bytes every
// client of that kind parses, and it should not be possible to do by accident.
func TestSupportedKinds_OnlyAgentFormats(t *testing.T) {
	set := SupportedKinds()

	assert.True(t, set.Enabled(policy.APIKindAgent),
		"an Agent route serving JSON-RPC cannot read a plain JSON error")
	for _, kind := range knownAPIKinds {
		if kind == policy.APIKindAgent {
			continue
		}
		assert.False(t, set.Enabled(kind),
			"%s must forward its errors unchanged; adding it here changes bytes existing clients parse", kind)
	}
}

func TestKindSet_NothingConfiguredEnablesNothing(t *testing.T) {
	set, unknown := NewKindSet(nil)
	assert.Empty(t, unknown)

	for _, kind := range knownAPIKinds {
		assert.False(t, set.Enabled(kind), "%s must not be enabled by default", kind)
	}
}

func TestKindSet_EnablesOnlyWhatWasListed(t *testing.T) {
	set, unknown := NewKindSet([]string{"Mcp"})
	require.Empty(t, unknown)

	assert.True(t, set.Enabled(policy.APIKindMCP))
	assert.False(t, set.Enabled(policy.APIKindRestApi),
		"enabling one kind must not enable another")
}

// Operators write config by hand, so matching is case-insensitive and tolerates padding.
func TestKindSet_MatchingIsCaseInsensitiveAndTrimmed(t *testing.T) {
	for _, spelling := range []string{"RestApi", "restapi", "RESTAPI", "  RestApi  "} {
		set, unknown := NewKindSet([]string{spelling})
		require.Empty(t, unknown, "%q should be recognized", spelling)
		assert.True(t, set.Enabled(policy.APIKindRestApi), "%q should enable RestApi", spelling)
	}
}

// A typo must be visible. It cannot silently enable something else, and it must be reported
// so the operator finds out from the log rather than from a client.
func TestKindSet_UnknownKindIsReportedAndEnablesNothing(t *testing.T) {
	set, unknown := NewKindSet([]string{"RestAPIs", "SoapApi"})

	assert.Equal(t, []string{"RestAPIs", "SoapApi"}, unknown,
		"both are unknown and both must be reported verbatim, not normalized")
	assert.False(t, set.Enabled(policy.APIKindRestApi),
		"a near-miss must not enable the kind it resembles")
}

// A good entry beside a bad one still works — one typo must not disable the rest of the list.
func TestKindSet_KeepsTheValidEntriesAlongsideATypo(t *testing.T) {
	set, unknown := NewKindSet([]string{"Mcp", "Nonsense"})

	assert.Equal(t, []string{"Nonsense"}, unknown)
	assert.True(t, set.Enabled(policy.APIKindMCP))
}

// A request that matched no route has no kind, so no configuration can have named it. Asked
// with an empty kind, the answer is always no — including when everything else is enabled.
func TestKindSet_EmptyKindIsNeverEnabled(t *testing.T) {
	names := make([]string, 0, len(knownAPIKinds))
	for _, k := range knownAPIKinds {
		names = append(names, string(k))
	}
	set, unknown := NewKindSet(names)
	require.Empty(t, unknown)

	assert.False(t, set.Enabled(""), "no route means no kind means nothing to enable")
}

// ShouldFormat's gate order, asserted through the Reason string an operator reads in the log:
// "not switched on" and "a policy wrote a body" are different problems with different fixes.
func TestShouldFormat_GateOrderIsVisibleInTheReason(t *testing.T) {
	r := NewRegistry()
	base := Input{
		APIKind: policy.APIKindMCP,
		Status:  401,
		Err:     policy.FaultDetails{Code: "900901", Message: "Denied"},
	}

	t.Run("disabled kind is named", func(t *testing.T) {
		in := base
		in.FormatterEnabled = false
		in.BodyAuthored = true // both gates would stop it; the kind must be the reason given
		d := ShouldFormat(r, in)

		assert.False(t, d.Format)
		assert.Contains(t, d.Reason, "not enabled")
		assert.Contains(t, d.Reason, string(policy.APIKindMCP))
	})

	t.Run("authored body is reported once the kind is enabled", func(t *testing.T) {
		in := base
		in.FormatterEnabled = true
		in.BodyAuthored = true
		d := ShouldFormat(r, in)

		assert.False(t, d.Format)
		assert.Contains(t, d.Reason, "authored")
	})

	t.Run("enabled and unauthored renders", func(t *testing.T) {
		in := base
		in.FormatterEnabled = true
		d := ShouldFormat(r, in)

		require.True(t, d.Format, "reason: %s", d.Reason)
		assert.Contains(t, string(d.Body), "jsonrpc")
	})

	t.Run("HEAD outranks an enabled kind", func(t *testing.T) {
		in := base
		in.FormatterEnabled = true
		in.RequestMethod = "HEAD"
		d := ShouldFormat(r, in)

		assert.False(t, d.Format, "a HEAD response carries no body whatever the config says")
	})
}

// The two signals the kernel combines into BodyAuthored, kept distinct here: a body is a
// body, and a description is a description. The kernel's rule for the PRODUCING policy is
// "a body counts only if nothing was described" — a body beside a description is a fallback
// for a gateway that cannot render — while a fault entry's body counts unconditionally.
func TestBodyAuthoredAndDescribedErrorAreIndependentSignals(t *testing.T) {
	body := []byte(`{"error":"Unauthorized"}`)
	err := &policy.FaultDetails{Code: "900901"}

	assert.True(t, BodyAuthored(policy.ImmediateResponse{Body: body}))
	assert.False(t, DescribedError(policy.ImmediateResponse{Body: body}))

	assert.True(t, BodyAuthored(policy.ImmediateResponse{Body: body, Fault: err}))
	assert.True(t, DescribedError(policy.ImmediateResponse{Body: body, Fault: err}),
		"both are set, and it is the caller that decides the fallback means render")

	assert.False(t, BodyAuthored(policy.ImmediateResponse{Fault: err}))
	assert.True(t, DescribedError(policy.ImmediateResponse{Fault: err}))

	assert.True(t, BodyAuthored(policy.ImmediateResponse{Body: []byte{}}),
		"an explicitly empty body is a decision that the client gets nothing")

	assert.True(t, BodyAuthored(policy.DownstreamResponseModifications{Body: body}))
	assert.True(t, DescribedError(policy.DownstreamResponseModifications{Fault: err}))
}
