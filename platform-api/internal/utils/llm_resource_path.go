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

package utils

import (
	"regexp"
	"strings"
)

var llmResourcePathPattern = regexp.MustCompile(`^/(?:[A-Za-z0-9._~-]+|\*|\{[A-Za-z0-9._~-]+\})(?:/(?:[A-Za-z0-9._~-]+|\*|\{[A-Za-z0-9._~-]+\}))*$|^/$`)

func NormalizeAndValidateLLMResourcePath(resource string) (string, bool) {
	normalized := strings.TrimSpace(resource)
	if normalized == "" || !strings.HasPrefix(normalized, "/") {
		return "", false
	}
	if !llmResourcePathPattern.MatchString(normalized) {
		return "", false
	}
	return normalized, true
}
