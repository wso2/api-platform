/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied.  See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

package llmusage

import "strings"

// pathsMatch reports whether a request path is covered by a resource pattern.
// A pattern may end in a wildcard to cover everything beneath a prefix. A
// specific pattern never matches a wildcard request path, so catch-all routes
// do not pick up resource-specific configuration.
func pathsMatch(requestPath, pattern string) bool {
	if pattern == "/*" {
		return true
	}
	if requestPath == pattern {
		return true
	}
	if strings.Contains(pattern, "*") {
		// Only a single trailing wildcard is supported. An embedded one such as
		// "/v1/*/usage" would otherwise reduce to the prefix "/v1/" and cover
		// every route beneath it, applying this resource's field locations to
		// requests it was never meant to describe.
		if !strings.HasSuffix(pattern, "*") || strings.Count(pattern, "*") != 1 {
			return false
		}
		prefix := pattern[:strings.LastIndex(pattern, "*")]
		return strings.HasPrefix(requestPath, prefix)
	}
	return false
}

// preferMoreSpecificPath reports whether candidate is a better match than
// current: a pattern without a wildcard beats one with a wildcard, and a longer
// pattern beats a shorter one.
func preferMoreSpecificPath(candidate, current string) bool {
	candidateHasWildcard := strings.Contains(candidate, "*")
	currentHasWildcard := strings.Contains(current, "*")

	if !candidateHasWildcard && currentHasWildcard {
		return true
	}
	if candidateHasWildcard && !currentHasWildcard {
		return false
	}

	return len(candidate) > len(current)
}
