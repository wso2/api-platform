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

package attemptverdict

import (
	"testing"

	accesslogv3 "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/collector"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func entry(code uint32, flags *accesslogv3.ResponseFlags) *accesslogv3.HTTPAccessLogEntry {
	return &accesslogv3.HTTPAccessLogEntry{
		CommonProperties: &accesslogv3.AccessLogCommon{
			ResponseFlags:   flags,
			UpstreamCluster: "llm_openai_primary",
		},
		Request:  &accesslogv3.HTTPRequestProperties{RequestId: "req-1"},
		Response: &accesslogv3.HTTPResponseProperties{ResponseCode: wrapperspb.UInt32(code)},
	}
}

// TestFlagReadersCoverEveryAdmittedFlag is the reason the flag list lives in `common`.
//
// The gateway-controller filters its access-log sink down to FailoverGradingResponseFlags; this
// package decodes entries back out. A flag admitted by the sink but missing a reader here is a
// verdict that arrives and is silently discarded — no error, no log line, just a provider that
// never gets suspended. This test is what makes that impossible to introduce.
func TestFlagReadersCoverEveryAdmittedFlag(t *testing.T) {
	for _, name := range collector.FailoverGradingResponseFlags {
		_, ok := flagReaders[name]
		assert.True(t, ok, "flag %q is admitted by the controller's sink but has no reader here", name)
	}
	assert.Len(t, flagReaders, len(collector.FailoverGradingResponseFlags),
		"a reader exists for a flag the sink never admits, so it can never fire")
}

// TestFromAccessLogEntry_DecodesEachAdmittedFlag drives every flag through the proto, so a reader
// wired to the wrong accessor (easy to do — the names are near-identical) is caught.
func TestFromAccessLogEntry_DecodesEachAdmittedFlag(t *testing.T) {
	cases := map[string]*accesslogv3.ResponseFlags{
		collector.FlagUpstreamProtocolError:            {UpstreamProtocolError: true},
		collector.FlagUpstreamConnectionTermination:    {UpstreamConnectionTermination: true},
		collector.FlagUpstreamRemoteReset:              {UpstreamRemoteReset: true},
		collector.FlagUpstreamRequestTimeout:           {UpstreamRequestTimeout: true},
		collector.FlagUpstreamConnectionFailure:        {UpstreamConnectionFailure: true},
		collector.FlagUpstreamRetryLimitExceeded:       {UpstreamRetryLimitExceeded: true},
		collector.FlagUpstreamMaxStreamDurationReached: {UpstreamMaxStreamDurationReached: true},
	}
	require.Len(t, cases, len(collector.FailoverGradingResponseFlags), "every admitted flag is exercised")

	for name, flags := range cases {
		t.Run(name, func(t *testing.T) {
			verdict, ok := FromAccessLogEntry(entry(200, flags))
			require.True(t, ok)
			assert.Equal(t, []string{name}, verdict.Flags)
			assert.Equal(t, "llm_openai_primary", verdict.UpstreamCluster,
				"the cluster names the target directly, where the response phase can only infer it")
			assert.Equal(t, "req-1", verdict.RequestID)
		})
	}
}

// TestFromAccessLogEntry_IgnoresEverythingElse covers the case that dominates by volume: when the
// collector is enabled the sink is unfiltered, so every healthy request on the gateway arrives
// here and must be dropped.
func TestFromAccessLogEntry_IgnoresEverythingElse(t *testing.T) {
	t.Run("healthy request", func(t *testing.T) {
		_, ok := FromAccessLogEntry(entry(200, &accesslogv3.ResponseFlags{}))
		assert.False(t, ok)
	})

	t.Run("no response flags block at all", func(t *testing.T) {
		_, ok := FromAccessLogEntry(entry(200, nil))
		assert.False(t, ok)
	})

	t.Run("nil entry", func(t *testing.T) {
		_, ok := FromAccessLogEntry(nil)
		assert.False(t, ok)
	})

	t.Run("downstream disconnect is not the target's fault", func(t *testing.T) {
		// The client closed the tab mid-stream. Grading this against the target would suspend
		// a provider that did nothing wrong.
		_, ok := FromAccessLogEntry(entry(200, &accesslogv3.ResponseFlags{
			DownstreamConnectionTermination: true,
		}))
		assert.False(t, ok)
	})

	t.Run("no healthy upstream means no attempt was made", func(t *testing.T) {
		_, ok := FromAccessLogEntry(entry(503, &accesslogv3.ResponseFlags{NoHealthyUpstream: true}))
		assert.False(t, ok)
	})
}

// TestCommittedBeforeFailing separates the failure the policy is blind to from the one it already
// grades correctly. Only the former justifies this whole path existing.
func TestCommittedBeforeFailing(t *testing.T) {
	committed, ok := FromAccessLogEntry(entry(200, &accesslogv3.ResponseFlags{UpstreamProtocolError: true}))
	require.True(t, ok)
	assert.True(t, committed.CommittedBeforeFailing(),
		"the client already has `200` and half a body; no phase the policy runs in can see this")

	preCommit, ok := FromAccessLogEntry(entry(502, &accesslogv3.ResponseFlags{UpstreamConnectionFailure: true}))
	require.True(t, ok)
	assert.False(t, preCommit.CommittedBeforeFailing(),
		"a reset before commitment surfaces as a 5xx the response phase already grades")
}

// TestFromAccessLogEntry_MultipleFlags pins the ordering, which follows the shared list rather
// than map iteration — otherwise a log line or assertion would flap between runs.
func TestFromAccessLogEntry_MultipleFlags(t *testing.T) {
	verdict, ok := FromAccessLogEntry(entry(200, &accesslogv3.ResponseFlags{
		UpstreamRemoteReset:   true,
		UpstreamProtocolError: true,
	}))
	require.True(t, ok)
	assert.Equal(t,
		[]string{collector.FlagUpstreamProtocolError, collector.FlagUpstreamRemoteReset},
		verdict.Flags, "ordered by the shared list, not by map iteration")
}

// TestConsumer_ForwardsOnlyVerdicts covers the fan-out contract: the consumer sees every entry
// and forwards only the ones that carry a verdict.
func TestConsumer_ForwardsOnlyVerdicts(t *testing.T) {
	sink := &recordingSink{}
	consumer := NewConsumer(sink)

	consumer.Consume(entry(200, &accesslogv3.ResponseFlags{}))
	consumer.Consume(entry(200, nil))
	consumer.Consume(entry(200, &accesslogv3.ResponseFlags{UpstreamProtocolError: true}))

	seen, forwarded := consumer.Stats()
	assert.Equal(t, uint64(3), seen)
	assert.Equal(t, uint64(1), forwarded)
	require.Len(t, sink.got, 1)
	assert.Equal(t, []string{collector.FlagUpstreamProtocolError}, sink.got[0].Flags)
}

// TestNewConsumer_NilSinkStillRuns — a nil sink must not panic the ALS stream. One bad consumer
// taking down the stream would take analytics down with it.
func TestNewConsumer_NilSinkStillRuns(t *testing.T) {
	consumer := NewConsumer(nil)
	require.NotPanics(t, func() {
		consumer.Consume(entry(200, &accesslogv3.ResponseFlags{UpstreamProtocolError: true}))
	})
	_, forwarded := consumer.Stats()
	assert.Equal(t, uint64(1), forwarded)
}

type recordingSink struct{ got []Verdict }

func (s *recordingSink) RecordAttemptVerdict(v Verdict) { s.got = append(s.got, v) }
