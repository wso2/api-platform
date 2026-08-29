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

package policyv1alpha2

// ResolutionAttributes provides read-only access to the protocol-derived facts a
// route's resolver captured about one request, in the same pass that identified
// the operation. The underlying data is managed by the policy engine kernel.
//
// They exist so a request payload is parsed once. A resolver on a multiplexed
// transport has already decoded the body to find the operation; without somewhere
// to put what else it saw, every later consumer re-parses the same bytes. They are
// also the only way a body-sourced value can reach a *request-header-phase*
// policy, since RequestHeaderContext carries no body of its own.
//
// # Why this is a type and not a map
//
// Read-only here is a guarantee, not a convention. A route whose resolution is
// fixed at deploy time builds its attributes once, at ingest, and shares them with
// every request on that route — so a policy writing into them would leak one
// request's data into the next. That failure is silent, appears only under
// traffic, and would be indistinguishable from a correlation bug in whatever read
// the attributes downstream. Keeping the map unexported removes the possibility
// rather than documenting against it, the same way Headers does.
//
// It also costs nothing: because nothing outside this package can reach the map,
// the kernel hands its own over directly and no per-request copy is made.
//
// # Naming
//
// Names are namespaced and dot-segmented by the protocol that produced them
// ("a2a.context.id"), because they share one set with every other producer of
// request facts. The set is small and closed per protocol; this is not a
// projection of the payload.
//
// # Trust
//
// Values are attacker-controlled in the general case — they come out of a request
// body — and are bounded in count and length by the engine before they get here.
// Bounded protocol facts and unbounded caller-supplied identifiers sit side by
// side, so a consumer using one as a metric label or a cache/limiter key must know
// which it has.
//
// The zero value is a valid, empty set. A route whose chain is fixed by the route
// itself inspected no request and carries no attributes, so every accessor below
// answers correctly without the caller testing for one.
type ResolutionAttributes struct {
	values map[string]string
}

// NewResolutionAttributes wraps a resolver's attributes for the policy layer.
//
// For the policy engine kernel: a policy receives a ResolutionAttributes, it never
// builds one. A nil or empty map yields the zero value, which every accessor
// already handles.
//
// The map is taken by reference rather than copied. Wrapping it is what puts it
// out of reach, so there is nothing left to defend against and no allocation to
// pay on a request path. The caller is expected not to mutate it afterwards; the
// kernel does not.
func NewResolutionAttributes(values map[string]string) ResolutionAttributes {
	return ResolutionAttributes{values: values}
}

// ============================================
// PUBLIC API - Safe for policies to use
// ============================================

// Get returns the value recorded under name, or the empty string if the request
// carried no such attribute.
//
// Absent and empty are not distinguished. Use Lookup where they differ — a
// protocol may record an attribute whose value is legitimately empty, and every
// identifier-shaped attribute is absent rather than empty when a request did not
// carry it.
//
// Example:
//
//	contextID := ctx.ResolutionAttributes.Get("a2a.context.id")
func (a ResolutionAttributes) Get(name string) string {
	return a.values[name]
}

// Lookup returns the value recorded under name and whether it was recorded at all.
//
// Example:
//
//	if taskID, ok := ctx.ResolutionAttributes.Lookup("a2a.task.id"); ok {
//	    // the request addressed a task
//	}
func (a ResolutionAttributes) Lookup(name string) (string, bool) {
	value, ok := a.values[name]
	return value, ok
}

// Len returns how many attributes this request carried. Zero for a route that
// inspected no request.
func (a ResolutionAttributes) Len() int {
	return len(a.values)
}

// Iterate calls fn once for every attribute, in unspecified order.
//
// Names and values are strings, so unlike Headers.Iterate there is nothing for a
// callback to mutate and nothing to copy defensively.
//
// Example:
//
//	ctx.ResolutionAttributes.Iterate(func(name, value string) {
//	    span.SetAttributes(attribute.String(name, value))
//	})
func (a ResolutionAttributes) Iterate(fn func(name, value string)) {
	if fn == nil {
		return
	}
	for name, value := range a.values {
		fn(name, value)
	}
}
