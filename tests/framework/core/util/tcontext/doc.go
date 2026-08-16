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

// Package tcontext carries per-block and per-runner test state.
//
// Named tcontext rather than context so it does not shadow the standard library
// package it is built on.
//
// Two scopes:
//
//	shared - per block. Populated during boot (accessors, container handles, actor
//	         registry) and read-only afterwards. Every test owns its resources and
//	         shares nothing mutable; that rule is what makes parallel runners safe.
//	local  - per runner. Mutable, and the handoff channel for a _setup_ feature's
//	         fixtures to the scenarios that follow it in the same runner.
//
// Both ride on context.Context, which godog already threads through hooks and steps.
// That is simpler and safer than the thread-local equivalent this design is ported
// from: there is no per-invocation set/clear to get wrong and no way for a pooled
// worker to carry stale scope.
//
// Go's failure mode is the inverse one. Scope cannot leak between blocks, but state
// parked in a package-level variable escapes both scopes entirely — which is exactly
// what the suite this framework replaces did with a single global TestState and a
// Reset method. Package-level mutable state is banned.
//
// Three read intents, and choosing one IS the statement of intent:
//
//	Resolve  - required. Returns an error when absent, so a mistyped step argument
//	           fails immediately with a clear message instead of surfacing later as a
//	           nil dereference. The default for anything a step receives.
//	Get      - nullable. For framework-managed keys the caller checks itself, or where
//	           absence is a legitimate branch.
//	Contains - presence only.
//
// Do not collapse them into one call with a flag.
package tcontext
