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

// Package attemptverdict decodes Envoy's after-the-fact verdict on a failover attempt out of an
// access-log entry.
//
// The model-failover policy grades a target in its response-headers phase, which is the earliest
// point it can act and the only point it is given. That phase cannot see how the exchange ended:
// an attempt that sends `200` and then dies mid-body is indistinguishable there from one that
// streamed the whole answer. Recording it as a success is worse than recording nothing — the
// false success resets the consecutive-failure counter and immunises a provider that truncates
// every stream against the suspension its real failures would trigger (FR-030n).
//
// Envoy is the only component that sees the end of the exchange, and it reports what it saw as a
// response flag on the access-log entry. This package turns that entry back into a verdict the
// policy can act on.
package attemptverdict

import (
	accesslogv3 "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
	"github.com/wso2/api-platform/common/collector"
)

// Verdict is one attempt's outcome as Envoy finally observed it.
type Verdict struct {
	// RequestID is Envoy's x-request-id for the downstream request the attempt belongs to.
	RequestID string
	// UpstreamCluster names the target that served (or failed) the attempt. This is a direct
	// answer to "which target was this?", where the policy's response phase can only infer it
	// from the attempt count.
	UpstreamCluster string
	// Flags are the upstream-failure flags Envoy set, in the order of
	// collector.FailoverGradingResponseFlags. More than one can be set at once.
	Flags []string
	// ResponseCode is what the client was told, which is what makes the verdict interesting:
	// a `200` here means the failure happened after the response was already committed, and so
	// is invisible to every phase the policy runs in.
	ResponseCode uint32
}

// flagReaders maps each shared flag name to the proto accessor that reports it.
//
// Keyed by the same constants the gateway-controller builds its access-log filter from, so a
// flag can never be admitted by the sink and then silently ignored here. TestFlagReadersCoverAll
// asserts the two stay in step.
var flagReaders = map[string]func(*accesslogv3.ResponseFlags) bool{
	collector.FlagUpstreamProtocolError:            (*accesslogv3.ResponseFlags).GetUpstreamProtocolError,
	collector.FlagUpstreamConnectionTermination:    (*accesslogv3.ResponseFlags).GetUpstreamConnectionTermination,
	collector.FlagUpstreamRemoteReset:              (*accesslogv3.ResponseFlags).GetUpstreamRemoteReset,
	collector.FlagUpstreamRequestTimeout:           (*accesslogv3.ResponseFlags).GetUpstreamRequestTimeout,
	collector.FlagUpstreamConnectionFailure:        (*accesslogv3.ResponseFlags).GetUpstreamConnectionFailure,
	collector.FlagUpstreamRetryLimitExceeded:       (*accesslogv3.ResponseFlags).GetUpstreamRetryLimitExceeded,
	collector.FlagUpstreamMaxStreamDurationReached: (*accesslogv3.ResponseFlags).GetUpstreamMaxStreamDurationReached,
}

// FromAccessLogEntry decodes an entry into a Verdict, reporting false when the entry carries no
// upstream-failure flag.
//
// The false case is the common one and must stay cheap. When the collector is enabled its sink
// is unfiltered, so every request on the gateway arrives here — including all the healthy ones,
// which have nothing to say about target health and must cost no more than a few field reads.
func FromAccessLogEntry(entry *accesslogv3.HTTPAccessLogEntry) (Verdict, bool) {
	flags := entry.GetCommonProperties().GetResponseFlags()
	if flags == nil {
		return Verdict{}, false
	}

	// Ranged over the shared slice, not the map, so the order is stable and a log line or test
	// assertion does not flap between runs.
	var set []string
	for _, name := range collector.FailoverGradingResponseFlags {
		if read, ok := flagReaders[name]; ok && read(flags) {
			set = append(set, name)
		}
	}
	if len(set) == 0 {
		return Verdict{}, false
	}

	return Verdict{
		RequestID:       entry.GetRequest().GetRequestId(),
		UpstreamCluster: entry.GetCommonProperties().GetUpstreamCluster(),
		Flags:           set,
		ResponseCode:    entry.GetResponse().GetResponseCode().GetValue(),
	}, true
}

// CommittedBeforeFailing reports whether the client had already been told the request succeeded
// when the attempt fell over.
//
// This is the case the policy is blind to and this whole path exists for. A failure before
// commitment surfaces as a 5xx the policy's response phase already grades correctly; a failure
// after it looks, from every vantage point the policy has, like a success.
func (v Verdict) CommittedBeforeFailing() bool {
	return v.ResponseCode >= 200 && v.ResponseCode < 300
}
