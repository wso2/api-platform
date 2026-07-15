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

// Package normalizer up-converts a deployment artifact from whatever platform
// data version it was generated from up to the gateway-latest shape, so the
// rest of the translator (and any subsequent down-conversion) always starts
// from a canonical, fully-split artifact.
package normalizer

// shapeHandler up-converts artifact (a pointer to one of the *DeploymentYAML
// structs) to the gateway-latest shape, in place. Implementations are
// idempotent: an artifact already in gateway-latest shape is left unchanged.
type shapeHandler func(sourceDataVersion string, artifact any) error

// shapeHandlers maps kind -> its shape handler. Kinds absent here have no
// data shape that ever diverges from gateway-latest (their artifact only ever
// carries a single flat policies list), so they need no up-convert and fall
// through to the identity default. Populated by each kind file's init() —
// today only llm.go.
var shapeHandlers = map[string]shapeHandler{}

// Normalize brings artifact up to the gateway-latest shape for kind, given the
// platform data version it was generated from. Kinds with no registered
// handler are left untouched (identity).
func Normalize(kind string, sourceDataVersion string, artifact any) error {
	handler, ok := shapeHandlers[kind]
	if !ok {
		return nil
	}
	return handler(sourceDataVersion, artifact)
}
