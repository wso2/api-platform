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

package v1

// An operation can be expressed either with the simple top-level Method+Path fields or with the
// richer Match block (method + path{value,type} + headers). These helpers resolve the effective
// matching criteria: when Match is set it is authoritative and the top-level Method/Path are
// ignored; otherwise the top-level fields (the simple form) are used.

// EffectiveMethod returns the operation's effective HTTP method.
func (o Operation) EffectiveMethod() OperationMethod {
	if o.Match != nil {
		return o.Match.Method
	}
	return o.Method
}

// EffectivePath returns the operation's effective route path.
func (o Operation) EffectivePath() string {
	if o.Match != nil {
		return o.Match.Path.Value
	}
	return o.Path
}

// EffectivePathMatchType returns the effective path match type. For the match form it is the
// explicit type or "Exact" (the default) when omitted. For the simple form it is empty,
// preserving the legacy path-matching behavior for operations that never carried a type.
func (o Operation) EffectivePathMatchType() OperationPathMatchType {
	if o.Match != nil {
		if o.Match.Path.Type != "" {
			return o.Match.Path.Type
		}
		return OperationPathMatchExact
	}
	return ""
}

// EffectiveHeaders returns the operation's header matchers. Header matching is expressible only
// via the match form; the simple form has none.
func (o Operation) EffectiveHeaders() []OperationHeaderMatch {
	if o.Match != nil {
		return o.Match.Headers
	}
	return nil
}
