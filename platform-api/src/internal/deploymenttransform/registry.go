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

// Package deploymenttransform provides a generic, version-keyed registry for
// adapting canonical deployment artifact specs to the capability of a target
// gateway. Each registered Transformation is scoped to one artifact kind and
// applied only when its AppliesWhen predicate matches the target gateway version.
//
// Generators produce the canonical (latest) artifact shape unconditionally.
// The registry's Transform call, invoked in the deploy orchestration layer
// before the artifact is marshalled and stored, rewrites the spec to whatever
// shape the target gateway understands.
//
// New version-boundary conversions are registered once, in a package init(),
// without touching the deploy services. The deploy services call only
// Default().Transform(kind, target, &spec).
package deploymenttransform

import "fmt"

// Transformation adapts a canonical deployment spec so a target gateway version
// can consume it. Scoped to one artifact kind; applied only when AppliesWhen
// returns true for the target Version.
type Transformation struct {
	// Name identifies the transformation in logs and error messages.
	Name string
	// Kind is the artifact kind this transformation applies to
	// (e.g. constants.LLMProvider, constants.LLMProxy).
	Kind string
	// AppliesWhen returns true when this transformation must be applied for
	// the given target gateway version.
	AppliesWhen func(target Version) bool
	// Apply mutates the payload (a *dto.*DeploymentSpec) in place.
	Apply func(payload any) error
}

// Registry holds registered Transformations and applies them in order.
type Registry struct {
	transformations []Transformation
}

// Register adds a Transformation to the registry. Transformations are applied
// in registration order, so order matters when multiple transformations target
// the same kind and version range.
func (r *Registry) Register(t Transformation) {
	r.transformations = append(r.transformations, t)
}

// Transform runs every registered Transformation whose Kind matches and
// AppliesWhen(target) is true, in registration order. An unknown kind or a
// target version that matches no Transformation is a no-op — the canonical
// payload passes through unchanged.
func (r *Registry) Transform(kind string, target Version, payload any) error {
	for _, t := range r.transformations {
		if t.Kind == kind && t.AppliesWhen(target) {
			if err := t.Apply(payload); err != nil {
				return fmt.Errorf("deploymenttransform %q: %w", t.Name, err)
			}
		}
	}
	return nil
}

// defaultRegistry is the package-level registry, pre-loaded by init() calls
// in other files of this package.
var defaultRegistry = &Registry{}

// Default returns the package-level Registry. All transformations registered
// via init() are available here.
func Default() *Registry {
	return defaultRegistry
}
