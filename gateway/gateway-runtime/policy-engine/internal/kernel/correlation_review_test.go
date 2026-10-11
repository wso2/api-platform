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

package kernel

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/correlation"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
)

// A later phase that filters every captured header out (an analytics header
// filter in the body phase) must replace the headers an earlier phase stored, or
// the unfiltered set would be logged.
func TestBuildAnalyticsStruct_LaterEmptyHeadersReplaceStoredOnes(t *testing.T) {
	store := correlation.NewStore(100, time.Minute, 4)
	execCtx := correlatedExecCtx(newTestServerWithStore(t, store), "req-1")

	_, err := buildAnalyticsStruct(map[string]any{
		"request_headers": map[string]string{"authorization": "Bearer secret", "host": "example.com"},
	}, execCtx)
	require.NoError(t, err)

	st, err := buildAnalyticsStruct(map[string]any{"request_headers": map[string][]string{}}, execCtx)
	require.NoError(t, err)
	assert.NotContains(t, st.GetFields(), "request_headers", "the empty set is recorded in the store")

	payload, ok := store.Take(execCtx.correlationToken)
	require.True(t, ok)
	require.NotNil(t, payload.RequestHeaders)
	assert.Empty(t, payload.RequestHeaders, "the filtered (empty) set replaced the captured one")
}

// The token key is reserved: a policy-supplied value must never reach Envoy, or it
// could point this request's access-log entry at another request's stored fields.
func TestBuildAnalyticsStruct_DropsPolicySuppliedToken(t *testing.T) {
	store := correlation.NewStore(100, time.Minute, 4)
	server := newTestServerWithStore(t, store)

	victim := correlatedExecCtx(server, "victim")
	stVictim, err := buildAnalyticsStruct(map[string]any{"request_headers": map[string]string{"a": "b"}}, victim)
	require.NoError(t, err)
	victimToken := stVictim.GetFields()[analytics.CorrelationTokenKey].GetStringValue()
	require.NotEmpty(t, victimToken)

	// A stream with nothing to store has no token of its own.
	attacker := correlatedExecCtx(server, "attacker")
	st, err := buildAnalyticsStruct(map[string]any{analytics.CorrelationTokenKey: victimToken, "source": "policy"}, attacker)
	require.NoError(t, err)
	assert.NotContains(t, st.GetFields(), analytics.CorrelationTokenKey)

	// A stream with its own token keeps its own.
	owner := correlatedExecCtx(server, "owner")
	st, err = buildAnalyticsStruct(map[string]any{
		analytics.CorrelationTokenKey: victimToken,
		"request_headers":             map[string]string{"c": "d"},
	}, owner)
	require.NoError(t, err)
	assert.Equal(t, owner.correlationToken, st.GetFields()[analytics.CorrelationTokenKey].GetStringValue())
	assert.True(t, store.Has(victimToken), "the victim's entry is untouched")
}

// Once the ALS handler took a stream's entry, a later phase (for example after an
// Envoy message timeout) must not create a new entry nobody will read.
func TestBuildAnalyticsStruct_DoesNotRecreateTakenEntry(t *testing.T) {
	store := correlation.NewStore(100, time.Minute, 4)
	execCtx := correlatedExecCtx(newTestServerWithStore(t, store), "req-1")

	_, err := buildAnalyticsStruct(map[string]any{"request_headers": map[string]string{"a": "b"}}, execCtx)
	require.NoError(t, err)
	_, ok := store.Take(execCtx.correlationToken)
	require.True(t, ok, "the ALS handler read the entry")

	st, err := buildAnalyticsStruct(map[string]any{"response_headers": map[string]string{"c": "d"}}, execCtx)
	require.NoError(t, err)
	assert.Contains(t, st.GetFields(), "response_headers", "not stored, so it stays in metadata")
	assert.False(t, store.Has(execCtx.correlationToken), "no orphan entry was created")
}

// Response phases re-send the request-phase analytics unchanged; those fields are
// not merged again, and stay out of metadata.
func TestBuildAnalyticsStruct_SkipsUnchangedFields(t *testing.T) {
	store := correlation.NewStore(100, time.Minute, 4)
	execCtx := correlatedExecCtx(newTestServerWithStore(t, store), "req-1")
	reqHeaders := map[string]string{"a": "b"}

	_, err := buildAnalyticsStruct(map[string]any{"request_headers": reqHeaders}, execCtx)
	require.NoError(t, err)
	store.Discard(execCtx.correlationToken)

	st, err := buildAnalyticsStruct(map[string]any{"request_headers": reqHeaders}, execCtx)
	require.NoError(t, err)
	assert.NotContains(t, st.GetFields(), "request_headers")
	assert.False(t, store.Has(execCtx.correlationToken), "the unchanged field was not merged again")
}

// A stream whose token never reached Envoy has an entry no access-log entry can
// point to; it is discarded when the stream ends.
func TestCompleteCorrelationEntry_DiscardsEntryWhoseTokenWasNeverSent(t *testing.T) {
	store := correlation.NewStore(100, time.Minute, 4)
	server := newTestServerWithStore(t, store)
	execCtx := correlatedExecCtx(server, "req-1")
	_, err := buildAnalyticsStruct(map[string]any{"request_headers": map[string]string{"a": "b"}}, execCtx)
	require.NoError(t, err)
	require.True(t, store.Has(execCtx.correlationToken))

	server.completeCorrelationEntry(execCtx) // the response carrying the token was never sent
	assert.False(t, store.Has(execCtx.correlationToken))
}

func newIgnoringStore(prefixes ...string) *correlation.Store {
	return correlation.NewStoreFromConfig(config.CollectorConfig{
		IgnorePathPrefixes: prefixes,
		CorrelationStore: config.CorrelationStoreConfig{
			Capacity: 100, TTL: time.Minute, Shards: 4, MaxPayloadBytes: 1024, MaxBodyBytes: 1 << 20,
		},
	})
}

// A policy that rewrites the path into collector.ignore_path_prefixes may make
// Envoy skip the access-log entry, so nothing is stored for it.
func TestBuildAnalyticsStruct_RewrittenIntoIgnoredPathIsNotStored(t *testing.T) {
	store := newIgnoringStore("/health")
	execCtx := correlatedExecCtx(newTestServerWithStore(t, store), "req-1")
	execCtx.clientPath = "/api/v1/orders"
	rewritten := "/health/check"
	execCtx.noteRoutedPath(&rewritten)

	st, err := buildAnalyticsStruct(map[string]any{"request_headers": map[string]string{"a": "b"}}, execCtx)
	require.NoError(t, err)
	assert.Contains(t, st.GetFields(), "request_headers")
	assert.Empty(t, execCtx.correlationToken)
}

// When the rewrite into an ignored prefix comes after fields were stored, the entry
// is released and its fields go back into metadata, so a route that still logs the
// request loses nothing and no orphan is left.
func TestBuildAnalyticsStruct_LateRewriteIntoIgnoredPathReleasesEntry(t *testing.T) {
	store := newIgnoringStore("/health")
	execCtx := correlatedExecCtx(newTestServerWithStore(t, store), "req-1")
	execCtx.clientPath = "/api/v1/orders"

	_, err := buildAnalyticsStruct(map[string]any{
		"request_headers": map[string]string{"a": "b"},
		"request_payload": "body",
	}, execCtx)
	require.NoError(t, err)
	token := execCtx.correlationToken
	require.True(t, store.Has(token))

	rewritten := "/health/check"
	execCtx.noteRoutedPath(&rewritten)
	st, err := buildAnalyticsStruct(map[string]any{"source": "policy"}, execCtx)
	require.NoError(t, err)

	assert.False(t, store.Has(token), "the entry was released")
	assert.NotContains(t, st.GetFields(), analytics.CorrelationTokenKey)
	require.Contains(t, st.GetFields(), "request_headers")
	// Header maps travel through metadata as a JSON string, as on the pre-store path.
	assert.JSONEq(t, `{"a":"b"}`, st.GetFields()["request_headers"].GetStringValue())
	assert.Equal(t, "body", st.GetFields()["request_payload"].GetStringValue())
}

// No token is allocated while the store refuses a stream's fields, so its
// access-log entry is not counted as a store miss.
func TestBuildAnalyticsStruct_NoTokenWhenFirstFieldRefused(t *testing.T) {
	store := correlation.NewStore(1, time.Minute, 1)
	server := newTestServerWithStore(t, store)
	_, err := buildAnalyticsStruct(map[string]any{"request_headers": map[string]string{"a": "b"}}, correlatedExecCtx(server, "fills-the-store"))
	require.NoError(t, err)

	refused := correlatedExecCtx(server, "refused")
	st, err := buildAnalyticsStruct(map[string]any{"request_headers": map[string]string{"c": "d"}}, refused)
	require.NoError(t, err)
	assert.Contains(t, st.GetFields(), "request_headers")
	assert.NotContains(t, st.GetFields(), analytics.CorrelationTokenKey)
	assert.Empty(t, refused.correlationToken)
}

// When a newer value of an already stored field is refused (a streamed response
// body that outgrew max_payload_bytes), the older stored copy is cleared so the
// newer value in metadata is the one logged.
func TestBuildAnalyticsStruct_RefusedNewerBodyClearsStoredOne(t *testing.T) {
	store := correlation.NewStoreWithBodyLimits(10, time.Minute, 1, 8, 1024)
	execCtx := correlatedExecCtx(newTestServerWithStore(t, store), "req-1")

	_, err := buildAnalyticsStruct(map[string]any{"response_payload": "chunk1"}, execCtx)
	require.NoError(t, err)
	st, err := buildAnalyticsStruct(map[string]any{"response_payload": "chunk1chunk2"}, execCtx)
	require.NoError(t, err)
	assert.Equal(t, "chunk1chunk2", st.GetFields()["response_payload"].GetStringValue())

	payload, ok := store.Take(execCtx.correlationToken)
	require.True(t, ok)
	assert.Empty(t, payload.ResponseBody, "the truncated older copy is not logged")
}

// A header filter's repeated values reach the store unflattened.
func TestBuildAnalyticsStruct_KeepsMultiValueHeaders(t *testing.T) {
	store := correlation.NewStore(10, time.Minute, 1)
	execCtx := correlatedExecCtx(newTestServerWithStore(t, store), "req-1")
	_, err := buildAnalyticsStruct(map[string]any{
		"request_headers": map[string][]string{"x-multi": {"a=1", "b=2"}},
	}, execCtx)
	require.NoError(t, err)
	payload, ok := store.Take(execCtx.correlationToken)
	require.True(t, ok)
	assert.Equal(t, map[string][]string{"x-multi": {"a=1", "b=2"}}, payload.RequestHeaders)
}

// A policy cannot set the token as a top-level key of the ext_proc metadata
// namespace either.
func TestBuildDynamicMetadata_DropsPolicySuppliedToken(t *testing.T) {
	md := buildDynamicMetadata(nil, nil, map[string]map[string]interface{}{
		constants.ExtProcFilterName: {
			analytics.CorrelationTokenKey: "someone-elses-token",
			"policy-key":                  "kept",
		},
	})
	ns := md.GetFields()[constants.ExtProcFilterName].GetStructValue()
	require.NotNil(t, ns)
	assert.NotContains(t, ns.GetFields(), analytics.CorrelationTokenKey)
	assert.Equal(t, "kept", ns.GetFields()["policy-key"].GetStringValue())
}

// With the store off, an unchanged header map re-sent by a later phase reuses its
// encoding instead of being JSON-encoded again.
func TestBuildAnalyticsStruct_ReusesEncodingOfUnchangedFields(t *testing.T) {
	execCtx := correlatedExecCtx(newTestServerWithStore(t, nil), "req-1")
	reqHeaders := map[string][]string{"x-multi": {"a", "b"}}
	first, err := buildAnalyticsStruct(map[string]any{"request_headers": reqHeaders}, execCtx)
	require.NoError(t, err)
	second, err := buildAnalyticsStruct(map[string]any{"request_headers": reqHeaders}, execCtx)
	require.NoError(t, err)
	assert.Same(t, first.GetFields()["request_headers"], second.GetFields()["request_headers"])

	third, err := buildAnalyticsStruct(map[string]any{"request_headers": map[string][]string{"x-multi": {"c"}}}, execCtx)
	require.NoError(t, err)
	assert.NotSame(t, first.GetFields()["request_headers"], third.GetFields()["request_headers"], "a new value is encoded")
}
