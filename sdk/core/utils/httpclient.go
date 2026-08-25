/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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

package utils

import (
	"net/http"
	"sync"
)

// sharedHTTPClient holds the process-wide outbound *http.Client that
// SharedHTTPClient retrieves. Guarded by sharedHTTPClientMu rather than an
// atomic.Pointer so SetSharedHTTPClient stays a plain, ordinary assignment —
// simplicity over micro-optimization, since this is set exactly once at
// startup and read (never written) from then on.
var (
	sharedHTTPClientMu sync.RWMutex
	sharedHTTPClient   *http.Client
)

// SetSharedHTTPClient installs the process-wide outbound *http.Client that
// SharedHTTPClient returns to every policy. Called exactly once by the
// policy engine at startup (see cmd/policy-engine/main.go), after building
// the client from its [policy_engine.http_client] configuration — mirrors
// the same startup-only contract as registry.PolicyRegistry.SetConfig.
//
// Not intended to be called by policies themselves.
func SetSharedHTTPClient(client *http.Client) {
	sharedHTTPClientMu.Lock()
	defer sharedHTTPClientMu.Unlock()
	sharedHTTPClient = client
}

// SharedHTTPClient retrieves the single, process-wide outbound *http.Client
// built once at policy-engine startup from the [policy_engine.http_client]
// configuration section (pooling, timeouts, TLS, forward-proxy, and
// SSRF-guard settings — see httpkit/httpclient.Config). A policy issuing an
// outbound HTTP call — to a guardrail/moderation backend, an external
// validation service, an LLM provider — should call this to retrieve the
// client to use, instead of building or caching one of its own, so that:
//
//   - every policy gets the same hardened defaults — bounded timeouts,
//     bounded response size, and an optional dial-time SSRF guard — without
//     reimplementing them;
//   - connections are pooled across policies and requests rather than a
//     fresh *http.Transport per policy instance;
//   - the operator has exactly one place (the policy-engine config file) to
//     tune outbound HTTP behavior for every policy at once.
//
// It is safe for concurrent use across goroutines and across policy
// instances. Returns nil if the engine has not installed a client yet (e.g.
// a test calling this before SetSharedHTTPClient, or a policy retrieving it
// too early) — callers must nil-check; falling back to http.DefaultClient is
// NOT an acceptable substitute in production code, since that bypasses the
// configured timeouts/SSRF guard. Treat a nil return as a configuration
// error instead.
func SharedHTTPClient() *http.Client {
	sharedHTTPClientMu.RLock()
	defer sharedHTTPClientMu.RUnlock()
	return sharedHTTPClient
}
