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

import (
	"net/url"
	"regexp"
	"sync"
)

// pathPatternCache holds compiled patterns so a template's regex is compiled
// once rather than on every request.
var pathPatternCache sync.Map

// extractFromPath returns the first capture group of pattern applied to the
// request path, percent-decoded. An unparseable pattern, a pattern with no
// capture group, or no match yields an empty string, which callers treat the
// same as an absent field.
func extractFromPath(requestPath, pattern string) string {
	re := compilePathPattern(pattern)
	if re == nil {
		return ""
	}

	match := re.FindStringSubmatch(requestPath)
	if len(match) < 2 {
		return ""
	}

	// Values taken from a URL may be percent-encoded; AWS encodes ARN model
	// identifiers because their resource component can contain a slash.
	if decoded, err := url.PathUnescape(match[1]); err == nil {
		return decoded
	}
	return match[1]
}

// compilePathPattern compiles and caches a pattern, returning nil when it is
// not valid so a malformed template cannot break request handling.
func compilePathPattern(pattern string) *regexp.Regexp {
	if cached, ok := pathPatternCache.Load(pattern); ok {
		re, _ := cached.(*regexp.Regexp)
		return re
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		pathPatternCache.Store(pattern, (*regexp.Regexp)(nil))
		return nil
	}
	pathPatternCache.Store(pattern, re)
	return re
}
