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

// Package goenv builds the environment used when the builder shells out to the
// `go` toolchain.
package goenv

import (
	"os"
	"strings"
)

// gotoolchainKey is the environment variable Go consults to decide whether it
// may switch to a different toolchain than the one it was invoked as.
const gotoolchainKey = "GOTOOLCHAIN"

// Env returns a copy of the current process environment with GOTOOLCHAIN set to
// "auto" so the `go` command may download and switch to the toolchain a module
// requires.
//
// The official golang base images pin GOTOOLCHAIN=local. With that setting, a
// `go` command refuses to build or download a module whose go.mod declares a
// newer `go` directive than the toolchain baked into the image — it fails with
// "requires go >= X (running go Y; GOTOOLCHAIN=local)" instead of fetching the
// newer toolchain. Policies are published independently of the builder and may
// declare a newer `go` directive than the builder image ships, so we default to
// "auto" to let `go` transparently download and switch to the required version.
//
// An explicit operator override (any GOTOOLCHAIN value other than the empty
// string or "local", e.g. a pinned "go1.26.6") is respected and left untouched.
func Env() []string {
	env := os.Environ()
	for i, e := range env {
		v, ok := strings.CutPrefix(e, gotoolchainKey+"=")
		if !ok {
			continue
		}
		if v != "" && v != "local" {
			// Explicit, non-local override — respect it.
			return env
		}
		// Replace the local/empty value in place. Appending a second entry is
		// not enough: os.Getenv in the child reads the first occurrence, so a
		// leading "local" would still win.
		env[i] = gotoolchainKey + "=auto"
		return env
	}
	// Not set at all — add it.
	return append(env, gotoolchainKey+"=auto")
}
