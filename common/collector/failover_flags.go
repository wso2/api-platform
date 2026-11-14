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

package collector

// FailoverGradingResponseFlags is the set of Envoy response flags that mean a failover attempt's
// upstream leg failed — the verdict the model-failover policy cannot reach any other way.
//
// The policy grades a target from its response headers, so an attempt that sends `200` and then
// dies mid-body reads as a success. Worse than recording nothing: the false success resets the
// consecutive-failure counter and immunises a provider that truncates every stream (FR-030n).
// Envoy finishes the exchange and stamps the access-log entry with a flag, which is how the
// verdict reaches the policy engine.
//
// This list lives in `common` because it is one agreement held by two components that must not
// drift: the gateway-controller filters the access-log sink down to exactly these flags, and the
// policy-engine decodes exactly these flags back out of the entries it receives. A flag the
// controller admits but the engine ignores is a silently dropped verdict; a flag the engine
// looks for but the controller filters out never arrives. Neither failure mode shows up as an
// error — hence one list, two consumers.
//
// Deliberately upstream-only. `DC` (downstream connection termination) means the *client* went
// away, which says nothing about the target and would suspend a healthy provider whenever a user
// closed a tab mid-stream. `UH`/`NR` mean no attempt reached an upstream at all, so there is
// nothing to grade.
//
// Envoy validates these strings against its own closed flag list when it loads the listener, so
// a typo here fails the config push rather than silently matching nothing.
var FailoverGradingResponseFlags = []string{
	FlagUpstreamProtocolError,
	FlagUpstreamConnectionTermination,
	FlagUpstreamRemoteReset,
	FlagUpstreamRequestTimeout,
	FlagUpstreamConnectionFailure,
	FlagUpstreamRetryLimitExceeded,
	FlagUpstreamMaxStreamDurationReached,
}

// Envoy's short names for the flags in FailoverGradingResponseFlags. Named constants rather than
// bare literals so the controller's filter and the engine's decoder reference the same symbol
// and a rename cannot leave one side spelling the old string.
const (
	// FlagUpstreamProtocolError is the flag observed on a target that commits `200` and then
	// resets mid-body — the failure mode FR-030n exists for (see poc-results.md, T030k0).
	FlagUpstreamProtocolError = "UPE"
	// FlagUpstreamConnectionTermination — the target closed the connection mid-exchange.
	FlagUpstreamConnectionTermination = "UC"
	// FlagUpstreamRemoteReset — the target reset the stream.
	FlagUpstreamRemoteReset = "UR"
	// FlagUpstreamRequestTimeout — the attempt's own bound elapsed against the target.
	FlagUpstreamRequestTimeout = "UT"
	// FlagUpstreamConnectionFailure — the attempt never established a connection.
	FlagUpstreamConnectionFailure = "UF"
	// FlagUpstreamRetryLimitExceeded — the chain was exhausted without a target answering.
	FlagUpstreamRetryLimitExceeded = "URX"
	// FlagUpstreamMaxStreamDurationReached — the target's stream was cut at its duration cap.
	FlagUpstreamMaxStreamDurationReached = "UMSDR"
)
