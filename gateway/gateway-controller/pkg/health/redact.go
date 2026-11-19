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

package health

import "regexp"

var (
	// passwordSetting matches a password key-value in a connection string. A quoted
	// value may contain backslash-escaped quotes (libpq) or doubled quotes (SQL Server),
	// SQL Server also accepts a {braced} value with doubled closing braces, and libpq an
	// unquoted value with backslash-escaped characters such as an escaped space; each is
	// matched to its real end so no part of the password is left behind.
	passwordSetting = regexp.MustCompile(`(?i)(password|pwd)(\s*=\s*)('(?:[^'\\]|\\.|'')*'|"(?:[^"\\]|\\.|"")*"|\{(?:[^}]|\}\})*\}|(?:\\.|[^\s;\\])+)`)
	// urlUserinfoPassword matches the password in a URL's userinfo. The userinfo ends at
	// the last '@' before the path, query or fragment, so a password containing '@' is
	// matched whole.
	urlUserinfoPassword = regexp.MustCompile(`(://[^:/@\s]*:)[^/?#\s]*@`)
)

// RedactSecrets removes passwords from an error message. Database drivers can echo the
// connection string they failed to use, so every check error is passed through this
// before it is logged.
func RedactSecrets(msg string) string {
	// URL passwords first: a password inside userinfo can itself look like a key-value
	// setting ("pwd=..."), which would otherwise consume the '@' that ends the userinfo.
	msg = urlUserinfoPassword.ReplaceAllString(msg, "${1}xxxxx@")
	return passwordSetting.ReplaceAllString(msg, "${1}${2}xxxxx")
}
