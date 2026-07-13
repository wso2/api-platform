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

package management

// Ptr returns a pointer to v. Convenience for populating the optional (pointer) fields of
// generated request/response types, e.g. Operation.Method / Operation.Path.
func Ptr[T any](v T) *T { return &v }

// An operation can be expressed either with the simple top-level method+path fields or with
// the richer `match` block (method + path{value,type} + headers). These helpers resolve the
// effective matching criteria: when `match` is present it is authoritative and the top-level
// method/path are ignored; otherwise the top-level fields (the simple form) are used. Every
// consumer should read matching criteria through these helpers so it does not need to know
// which form was authored.

// EffectiveMethod returns the operation's effective HTTP method.
func (o Operation) EffectiveMethod() string {
	if o.Match != nil {
		return string(o.Match.Method)
	}
	if o.Method != nil {
		return string(*o.Method)
	}
	return ""
}

// EffectivePath returns the operation's effective route path.
func (o Operation) EffectivePath() string {
	if o.Match != nil {
		return o.Match.Path.Value
	}
	if o.Path != nil {
		return *o.Path
	}
	return ""
}

// EffectivePathMatchType returns the effective path match type. For the match form it is the
// explicit type or "Exact" (the schema default) when omitted. For the simple form it is empty,
// preserving the legacy path-matching behavior for operations that never carried a type.
func (o Operation) EffectivePathMatchType() string {
	if o.Match != nil {
		if o.Match.Path.Type != nil {
			return string(*o.Match.Path.Type)
		}
		return "Exact"
	}
	return ""
}

// EffectiveHeaders returns the operation's header matchers. Header matching is expressible only
// via the match form; the simple form has none.
func (o Operation) EffectiveHeaders() []OperationHeaderMatch {
	if o.Match != nil && o.Match.Headers != nil {
		return *o.Match.Headers
	}
	return nil
}
