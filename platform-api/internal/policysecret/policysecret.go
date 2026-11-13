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

// Package policysecret carries a policy definition's write-only parameter
// declarations into the two places that have to act on them: the read path,
// which must not hand a stored credential back, and the update path, which must
// not lose one because the read path withheld it.
//
// A policy's parameter map is opaque on its own — nothing in `{"region": "...",
// "awsSecretAccessKey": "..."}` says which half is a secret. The policy
// definition says so, with the standard JSON Schema `writeOnly: true`, and this
// package is the translation from that declaration into parameter paths that
// the mappers can apply without knowing anything about any particular policy.
//
// The two halves are deliberately one package. A read path that blanks a
// parameter without an update path that preserves it turns every
// read-modify-write round trip into silent credential loss, which is the worse
// failure of the two: the value is not merely hidden, it is gone. Keeping
// Blank and Preserve next to each other, driven by the same resolved paths,
// is what keeps them symmetric.
package policysecret

// Path locates one parameter inside a policy's parameter map.
//
// Each element is a property name, except the array marker, which stands for
// "every element of the array at this position". A path is resolved against the
// parameter map, not against the schema: the schema is only how the path was
// discovered.
type Path []string

// ArrayMarker is the Path element standing for every element of an array.
//
// Nothing stops a schema naming a property "[]", so the marker is not resolved
// by spelling alone: it addresses elements only where the value it is applied
// to is actually an array, and anywhere else it is treated as the property name
// it reads as. Resolving on spelling would leave a property so named returned
// in full, which is the one outcome this package exists to prevent.
const ArrayMarker = "[]"

// Resolver answers which of a policy's parameters its definition declares
// write-only.
//
// Name and version identify the policy as an attachment names it. A resolver
// that knows nothing about the policy returns nil, which leaves its parameters
// untouched — see Nop for when that is the right answer and when it is not.
type Resolver interface {
	WriteOnlyPaths(policyName, version string) []Path
}

// nopResolver declares nothing write-only.
type nopResolver struct{}

// WriteOnlyPaths implements Resolver.
func (nopResolver) WriteOnlyPaths(string, string) []Path { return nil }

// Nop returns a Resolver that declares nothing write-only, so Blank copies and
// Preserve carries nothing forward.
//
// It belongs on paths that must see stored values in full — building the
// deployment artifact the gateway runs on, and the export of a definition the
// operator asked for — where the right answer is the whole value and a missing
// resolver would be a bug, not a safe default. It does not belong on a response
// path: passing it there is how a credential reaches a reader.
func Nop() Resolver { return nopResolver{} }

// PathsFromParameterSchema walks a policy definition's `parameters` schema and
// returns a path for every parameter the schema declares write-only.
//
// The schema is a JSON Schema object node, as the policy definition carries it,
// so the walk descends `properties` (adding the property name to the path) and
// `items` (adding the array marker). `writeOnly: true` on a node yields that
// node's path; the walk continues past it, because an object may be declared
// write-only and still have write-only members, and the narrower path costs
// nothing to carry.
//
// Combinator keywords — allOf, anyOf, oneOf, if/then/else — are not descended.
// A definition in the catalog uses them to constrain which parameters are
// required for a given authenticationType, never to introduce a parameter that
// `properties` does not already carry, so descending them would yield duplicate
// paths rather than new ones. A definition that did introduce one there would
// need this walk extended; see the catalog validation that keeps that from
// arriving unnoticed.
func PathsFromParameterSchema(schema any) []Path {
	var out []Path
	walkSchema(schema, nil, &out)
	return out
}

// walkSchema appends to out the write-only paths at and below node, which sits
// at prefix.
func walkSchema(node any, prefix Path, out *[]Path) {
	obj, ok := node.(map[string]any)
	if !ok {
		return
	}
	writeOnly, _ := obj["writeOnly"].(bool)
	if writeOnly && len(prefix) > 0 {
		*out = append(*out, prefix)
	}
	if properties, ok := obj["properties"].(map[string]any); ok {
		for name, child := range properties {
			childPath := extend(prefix, name)
			// A declaration on the root says the whole parameter map is
			// credential material. There is no path that addresses the map
			// itself — an empty path would mean "everything" to Blank and
			// "nothing" to the traversal — so it is expanded into one path per
			// parameter, which blanks and preserves exactly the same values.
			//
			// This reaches what `properties` names. A root declared write-only
			// with no `properties` at all describes a free-form map whose keys
			// are not knowable from the schema; such a definition is not in the
			// catalog, and would need addressing here before one could be.
			if writeOnly && len(prefix) == 0 {
				*out = append(*out, childPath)
			}
			walkSchema(child, childPath, out)
		}
	}
	if items, ok := obj["items"]; ok {
		walkSchema(items, extend(prefix, ArrayMarker), out)
	}
}

// extend returns prefix with one more element, as a path of its own.
//
// It always allocates. Appending in place would hand every sibling of a branch
// the same backing array, and the paths recorded along the way would then be
// rewritten by whichever sibling was walked last.
func extend(prefix Path, element string) Path {
	out := make(Path, len(prefix)+1)
	copy(out, prefix)
	out[len(prefix)] = element
	return out
}

// Blank returns a copy of params with the value at every path replaced by its
// type's zero value, leaving the key in place.
//
// The key stays and the value is emptied rather than the key being dropped,
// because an absent key and an empty one mean different things to the update
// path: empty is what Preserve reads as "the reader never saw this, keep what
// is stored". Dropping the key would say the same thing for a parameter the
// caller had actually removed. Every write-only parameter in the catalog is a
// string, so in practice this is the empty string.
//
// params is never modified. The copy is deep along every branch it shares with
// a path, so a blanked response cannot reach into the stored configuration it
// was mapped from.
func Blank(params map[string]any, paths []Path) map[string]any {
	if params == nil {
		return nil
	}
	out := copyMap(params)
	if len(paths) == 0 {
		return out
	}
	for _, path := range paths {
		blankAt(out, path)
	}
	return out
}

// blankAt empties the value at path within container, if it is there. A path
// that does not resolve — a parameter the attachment never set, or set to
// something other than the schema's shape — is not an error: the attachment is
// free to omit any parameter.
func blankAt(container any, path Path) {
	if len(path) == 0 {
		return
	}
	head, rest := path[0], path[1:]

	// The array marker addresses an array's elements. A map reached by the same
	// element is a property whose name happens to be the marker's spelling, and
	// it is blanked as any other property would be: falling through rather than
	// returning here is what keeps such a property from being read back in full.
	if elements, ok := container.([]any); ok && head == ArrayMarker {
		for i, element := range elements {
			if len(rest) == 0 {
				elements[i] = zeroOf(element)
				continue
			}
			blankAt(element, rest)
		}
		return
	}

	obj, ok := container.(map[string]any)
	if !ok {
		return
	}
	value, present := obj[head]
	if !present {
		return
	}
	if len(rest) == 0 {
		obj[head] = zeroOf(value)
		return
	}
	blankAt(value, rest)
}

// zeroOf returns the zero value of v's dynamic type.
//
// Today every write-only parameter is a string and this is the empty string.
// The other cases are here so that a definition declaring a non-string
// parameter write-only is emptied rather than returned intact; a reader cannot
// tell a blanked false from a stored false, which is why the catalog should
// keep declaring write-only on strings.
func zeroOf(v any) any {
	switch v.(type) {
	case string:
		return ""
	case bool:
		return false
	case float64:
		return float64(0)
	case int:
		return 0
	case int64:
		return int64(0)
	case []any:
		return []any{}
	case map[string]any:
		return map[string]any{}
	default:
		return nil
	}
}

// Preserve carries stored write-only values forward onto an update that did not
// supply its own, so a read-modify-write round trip keeps the credential the
// reader was never shown.
//
// For each path, the value in updated decides:
//
//   - absent, or present and empty — the caller is echoing back what Blank
//     emitted, or left the parameter alone. The stored value is carried forward.
//   - present and non-empty — the caller supplied a new value. It wins.
//   - present and explicitly null — the caller is clearing the parameter. The
//     stored value is dropped and the parameter is left empty.
//
// updated is modified in place and is the authoritative result; stored is only
// read. Nothing outside the resolved paths is touched, so a non-secret
// parameter the caller removed stays removed.
func Preserve(stored, updated map[string]any, paths []Path) {
	if stored == nil || updated == nil {
		return
	}
	for _, path := range paths {
		preserveAt(stored, updated, path)
	}
}

// preserveAt carries one path's value from stored into updated.
func preserveAt(stored, updated any, path Path) {
	if len(path) == 0 {
		return
	}
	head, rest := path[0], path[1:]

	// An array's elements carry no identity a parameter map can be matched on.
	// Position is not identity: a caller who reorders an array sends the same
	// number of elements in a different order, and carrying values across by
	// position then attaches one element's credential to another — silently,
	// and to an element the caller may not control. Nothing is carried forward
	// inside an array; a caller editing one re-supplies its secrets. Blanking
	// is unaffected, so the values are still withheld on read.
	//
	// A map reached by the marker is a property named for the marker's
	// spelling, not an array, and falls through to the map handling below.
	if _, isArray := stored.([]any); isArray && head == ArrayMarker {
		return
	}
	if _, isArray := updated.([]any); isArray && head == ArrayMarker {
		return
	}

	storedObj, okStored := stored.(map[string]any)
	updatedObj, okUpdated := updated.(map[string]any)
	if !okStored || !okUpdated {
		return
	}
	storedValue, storedPresent := storedObj[head]
	if !storedPresent {
		return
	}
	if len(rest) == 0 {
		updatedValue, updatedPresent := updatedObj[head]
		updatedObj[head] = preservedValue(storedValue, updatedValue, updatedPresent)
		return
	}
	updatedValue, updatedPresent := updatedObj[head]
	if !updatedPresent {
		// The caller omitted the whole object on the way to the secret. Carrying
		// the stored branch forward wholesale would reinstate non-secret
		// parameters the caller meant to drop, so only the secret is restored,
		// into a branch rebuilt as far as it needs to be.
		rebuilt := map[string]any{}
		preserveAt(storedValue, rebuilt, rest)
		if len(rebuilt) > 0 {
			updatedObj[head] = rebuilt
		}
		return
	}
	preserveAt(storedValue, updatedValue, rest)
}

// preservedValue decides one leaf. See Preserve for the rules.
func preservedValue(storedValue, updatedValue any, updatedPresent bool) any {
	if !updatedPresent {
		return storedValue
	}
	if updatedValue == nil {
		// Explicit null: the caller is clearing the parameter. Emptied rather
		// than deleted so the shape of the stored parameters does not change
		// under a clear, and so a later update reads it as "nothing stored".
		return zeroOf(storedValue)
	}
	if isEmpty(updatedValue) {
		return storedValue
	}
	return updatedValue
}

// isEmpty reports whether v is its type's zero value — what Blank emitted.
func isEmpty(v any) bool {
	switch typed := v.(type) {
	case string:
		return typed == ""
	case bool:
		return !typed
	case float64:
		return typed == 0
	case int:
		return typed == 0
	case int64:
		return typed == 0
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	case nil:
		return true
	default:
		return false
	}
}

// copyMap deep-copies a parameter map, so that blanking a copy cannot reach the
// stored configuration it was taken from.
func copyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = copyValue(value)
	}
	return out
}

// copyValue deep-copies the container types a decoded parameter map holds.
// Scalars are returned as they are; they are immutable in this shape.
func copyValue(in any) any {
	switch typed := in.(type) {
	case map[string]any:
		return copyMap(typed)
	case []any:
		out := make([]any, len(typed))
		for i, element := range typed {
			out[i] = copyValue(element)
		}
		return out
	default:
		return in
	}
}
