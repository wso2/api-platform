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

package config

import "strings"

// validateUpstreamAuthHeader checks that the deprecated header/value upstream
// auth fields can be sent as an HTTP header (RFC 9110 section 5). Validators run
// after secret templates are resolved, so this covers a value that only
// becomes invalid once its secret is filled in. Absent or empty fields are left
// to the caller's "is required" checks.
//
// The messages never quote the value: it may be a resolved secret.
func validateUpstreamAuthHeader(fieldPrefix string, header, value *string) []ValidationError {
	var errors []ValidationError
	if header != nil && *header != "" && !isHTTPToken(*header) {
		errors = append(errors, ValidationError{
			Field:   fieldPrefix + ".header",
			Message: "Auth header must be a valid HTTP header name (letters, digits and !#$%&'*+-.^_`|~ only)",
		})
	}
	if value != nil && *value != "" && !isHTTPFieldValue(*value) {
		errors = append(errors, ValidationError{
			Field:   fieldPrefix + ".value",
			Message: "Auth value must be a valid HTTP header value (no control characters such as CR, LF or NUL)",
		})
	}
	return errors
}

// isHTTPToken reports whether s is an RFC 9110 token, the grammar of a header name.
func isHTTPToken(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		isAlnum := 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9'
		if !isAlnum && !strings.ContainsRune("!#$%&'*+-.^_`|~", rune(c)) {
			return false
		}
	}
	return true
}

// isHTTPFieldValue reports whether s holds only RFC 9110 field-value octets:
// visible characters, space, horizontal tab and obs-text, never other controls.
func isHTTPFieldValue(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < ' ' && c != '\t' || c == 0x7f {
			return false
		}
	}
	return true
}
