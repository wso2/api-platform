//go:build conformance

/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
 */

// Package conformance runs the upstream Gateway API conformance suite against the
// WSO2 API Platform gateway. The suite (and its test manifests, which are embedded
// in the module via //go:embed) is consumed as a Go module dependency, so no clone
// of the kubernetes-sigs/gateway-api repository is needed — `go test` resolves
// everything from the module cache. This mirrors how other implementations
// (kgateway, gloo, envoy-gateway) wire up their conformance runs.
//
// Build-tagged with `conformance` so it is excluded from normal `go test ./...`
// sweeps and only runs when explicitly requested (see run-conformance.sh).
package conformance_test

import (
	"os"
	"strconv"
	"testing"
	"time"

	"sigs.k8s.io/gateway-api/conformance"
)

// envDuration reads a duration-in-seconds override from the environment, falling
// back to def when unset or unparseable. Lets the run script tune timing without
// editing code (see run-conformance.sh).
func envDuration(name string, def time.Duration) time.Duration {
	if v := os.Getenv(name); v != "" {
		if secs, err := strconv.Atoi(v); err == nil {
			return time.Duration(secs) * time.Second
		}
	}
	return def
}

// TestConformance is the entrypoint. Options (gateway class, profile, report path,
// implementation metadata) are supplied as command-line flags after `-args`.
//
// We start from DefaultOptions (which parses those flags) and then relax the suite's
// timing. Two knobs help:
//   - TestIsolation: a quiet gap between test cases so resources from the previous
//     test settle/clean up before the next one provisions (this is the "delay
//     between tests").
//   - MaxTimeToConsistency / NamespacesMustBeReady: give each test longer to reach a
//     consistent routing state before it declares failure.
//
// All are overridable via env (seconds); defaults are conservative for local runs.
// On a stable cluster you can set them to 0 / the upstream defaults.
func TestConformance(t *testing.T) {
	opts := conformance.DefaultOptions(t)

	opts.TimeoutConfig.TestIsolation = envDuration("CONFORMANCE_TEST_ISOLATION", 5*time.Second)
	opts.TimeoutConfig.MaxTimeToConsistency = envDuration("CONFORMANCE_MAX_CONSISTENCY", 60*time.Second)
	opts.TimeoutConfig.NamespacesMustBeReady = envDuration("CONFORMANCE_NAMESPACES_READY", 5*time.Minute)

	conformance.RunConformanceWithOptions(t, opts)
}
