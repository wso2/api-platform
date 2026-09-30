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

package it

import (
	"net/http"
	"time"
)

// pendingPropagationContextKey holds the scenario's pendingPropagation. It
// lives in TestState.Context so TestState.Reset clears it per scenario.
const pendingPropagationContextKey = "pendingPropagation"

// clientAuthorityPoolChangedContextKey is set while this scenario has
// changed the client authority pool and no step has yet seen the gateway
// apply the change.
const clientAuthorityPoolChangedContextKey = "clientAuthorityPoolChanged"

// mtlsScenarioContextKey is set for a scenario tagged @mtls, whose steps
// wait for Envoy's listeners to become active before sending to the gateway.
const mtlsScenarioContextKey = "mtlsScenario"

// preMutationContextKey holds the versions read just before the latest
// mutation sent to the gateway controller.
const preMutationContextKey = "preMutationVersions"

// listenerBaselineContextKey holds Envoy's listener version from before a
// mutation that can change the HTTPS listener, until a step sees Envoy move
// past it or the move grace passes.
const listenerBaselineContextKey = "listenerBaseline"

// lastRequestAtContextKey holds when the scenario's latest request was sent.
const lastRequestAtContextKey = "lastRequestAt"

// preMutationVersions are the snapshot controller's policy chain version and
// Envoy's listener version, read before a mutation is sent. An empty field
// could not be read.
type preMutationVersions struct {
	policyChain string
	listener    string
}

// pendingPropagation records the API mutations the controller accepted that
// no step has yet seen reach the gateway. policyBaseline is the snapshot
// controller's policy chain version before the latest of them.
type pendingPropagation struct {
	mutations      int
	lastAccepted   time.Time
	policyBaseline string
}

// beforeRequest runs before every request a step sends. It reads the
// pre-mutation versions before a mutation addressed to the gateway
// controller's management API, settles pending propagation before any other
// request, and then records the send time.
func beforeRequest(state *TestState, req *http.Request) error {
	if req.URL.Hostname() == "localhost" && req.URL.Port() == GatewayControllerPort {
		if req.Method != http.MethodGet && req.Method != http.MethodHead {
			recordPreMutationVersions(state)
		}
	} else if err := settlePendingPropagation(state); err != nil {
		return err
	}
	state.SetContextValue(lastRequestAtContextKey, time.Now())
	return nil
}

// recordPreMutationVersions reads the versions a later readiness wait
// compares against. Envoy's listener version is read only in an @mtls
// scenario, the only kind that waits on it.
func recordPreMutationVersions(state *TestState) {
	var pre preMutationVersions
	pre.policyChain, _ = snapshotControllerPolicyVersion(state)
	if isMTLSScenario(state) {
		pre.listener, _ = envoyListenersVersion(state)
	}
	state.SetContextValue(preMutationContextKey, pre)
}

func currentPreMutationVersions(state *TestState) preMutationVersions {
	raw, _ := state.GetContextValue(preMutationContextKey)
	pre, _ := raw.(preMutationVersions)
	return pre
}

// markPropagationPending records one accepted API mutation. changesListener
// marks a mutation that can change the HTTPS listener, such as deploying,
// updating or deleting an API that carries mtls-auth.
func markPropagationPending(state *TestState, changesListener bool) {
	pre := currentPreMutationVersions(state)
	pending, _ := currentPendingPropagation(state)
	pending.mutations++
	pending.lastAccepted = time.Now()
	pending.policyBaseline = pre.policyChain
	state.SetContextValue(pendingPropagationContextKey, pending)
	if changesListener {
		markListenerBaseline(state, pre)
	}
}

// markListenerBaseline records Envoy's pre-mutation listener version in an
// @mtls scenario.
func markListenerBaseline(state *TestState, pre preMutationVersions) {
	if isMTLSScenario(state) && pre.listener != "" {
		state.SetContextValue(listenerBaselineContextKey, pre.listener)
	}
}

func currentPendingPropagation(state *TestState) (pendingPropagation, bool) {
	raw, ok := state.GetContextValue(pendingPropagationContextKey)
	if !ok {
		return pendingPropagation{}, false
	}
	pending, ok := raw.(pendingPropagation)
	return pending, ok
}

// settlePendingPropagation waits until policyPropagationDelay has passed
// since the last accepted API mutation, then forgets the pending mutations.
// In an @mtls scenario it then waits for Envoy to settle, including after a
// client authority pool change with no API mutation pending. With nothing
// pending it returns at once.
func settlePendingPropagation(state *TestState) error {
	pending, ok := currentPendingPropagation(state)
	_, listenerPending := state.GetContextValue(listenerBaselineContextKey)
	if !ok && !(listenerPending && isMTLSScenario(state)) {
		return nil
	}
	if ok {
		if remaining := policyPropagationDelay - time.Since(pending.lastAccepted); remaining > 0 {
			time.Sleep(remaining)
		}
	}
	if isMTLSScenario(state) {
		if err := waitForEnvoySettled(state); err != nil {
			return err
		}
	}
	state.DeleteContextValue(pendingPropagationContextKey)
	return nil
}

// releaseObservedPropagation forgets the pending mutation once a step has
// polled its route and the policy snapshot into agreement. It releases only
// a single pending mutation: when several are pending, the route that was
// polled may belong to an earlier one, so the next gateway request still
// waits out the rest of policyPropagationDelay.
func releaseObservedPropagation(state *TestState) {
	if pending, ok := currentPendingPropagation(state); ok && pending.mutations == 1 {
		state.DeleteContextValue(pendingPropagationContextKey)
	}
}

// markClientAuthorityPoolChanged records that this scenario changed the
// client authority pool, which can change the HTTPS listener.
func markClientAuthorityPoolChanged(state *TestState) {
	state.SetContextValue(clientAuthorityPoolChangedContextKey, true)
	markListenerBaseline(state, currentPreMutationVersions(state))
}

// clientAuthorityPoolChanged reports whether this scenario changed the client
// authority pool since the gateway was last seen to apply it.
func clientAuthorityPoolChanged(state *TestState) bool {
	changed, _ := state.GetContextValue(clientAuthorityPoolChangedContextKey)
	return changed == true
}

// markMTLSScenario records that the current scenario is tagged @mtls.
func markMTLSScenario(state *TestState) {
	state.SetContextValue(mtlsScenarioContextKey, true)
}

// isMTLSScenario reports whether the current scenario is tagged @mtls.
func isMTLSScenario(state *TestState) bool {
	flag, _ := state.GetContextValue(mtlsScenarioContextKey)
	return flag == true
}

// lastRequestAt returns when the scenario's latest request was sent.
func lastRequestAt(state *TestState) (time.Time, bool) {
	raw, ok := state.GetContextValue(lastRequestAtContextKey)
	if !ok {
		return time.Time{}, false
	}
	at, ok := raw.(time.Time)
	return at, ok
}
