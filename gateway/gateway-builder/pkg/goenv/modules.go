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

package goenv

import (
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// credentialsInURL matches a URL's userinfo up to the last "@" before the path, so a
	// raw "@" in the password ("https://user:p@ss@host") is redacted too.
	credentialsInURL = regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.\-]*://)[^/?#\s]+@`)
	// queryInURL matches a URL's query string, which can carry an access token.
	queryInURL = regexp.MustCompile(`://[^\s?#]*\?[^\s#]*`)
	// settingEntry matches one entry of a comma-, pipe- or space-separated Go setting.
	settingEntry = regexp.MustCompile(`[^,|\s]+`)
)

// RedactCredentials replaces the userinfo and query string of every URL in text with
// placeholders, so a token in a module proxy or repository URL never reaches the logs.
// Go's harmless "?go-get=1" lookups are kept.
func RedactCredentials(text string) string {
	text = credentialsInURL.ReplaceAllString(text, "${1}<redacted-credentials>@")
	return queryInURL.ReplaceAllStringFunc(text, func(m string) string {
		base, query, _ := strings.Cut(m, "?")
		suffix := ""
		if trimmed, ok := strings.CutSuffix(query, ":"); ok { // "...?token=x: 401 Unauthorized"
			query, suffix = trimmed, ":"
		}
		if query == "" || query == "go-get=1" {
			return m
		}
		return base + "?<redacted-query>" + suffix
	})
}

// redactSettingValue redacts every entry of a setting such as GOPROXY, including
// scheme-less entries ("user:token@proxy.example.com") that Go accepts by adding https://.
func redactSettingValue(value string) string {
	return settingEntry.ReplaceAllStringFunc(value, func(entry string) string {
		scheme, rest := "", entry
		if i := strings.Index(entry, "://"); i >= 0 {
			scheme, rest = entry[:i+3], entry[i+3:]
		}
		authority, tail := rest, ""
		if i := strings.IndexAny(rest, "/?#"); i >= 0 {
			authority, tail = rest[:i], rest[i:]
		}
		if at := strings.LastIndex(authority, "@"); at >= 0 {
			authority = "<redacted-credentials>" + authority[at:]
		}
		if i := strings.IndexAny(tail, "?#"); i >= 0 {
			tail = tail[:i+1] + "<redacted>"
		}
		return scheme + authority + tail
	})
}

// moduleSettingKeys are the Go settings that decide where and how modules are fetched.
var moduleSettingKeys = []string{
	"GOPROXY", "GOPRIVATE", "GONOPROXY", "GONOSUMDB", "GOSUMDB", "GOINSECURE", "GOVCS",
}

// ModuleSettingsLogFields returns slog fields describing how the go command will
// fetch modules, so a failed private-module fetch can be diagnosed from the log.
// Values are redacted; GOAUTH is reported only as set or unset because it may hold
// a command line carrying a token.
func ModuleSettingsLogFields(getenv func(string) string, fileExists func(string) bool) []any {
	fields := make([]any, 0, 2*(len(moduleSettingKeys)+2))
	for _, key := range moduleSettingKeys {
		if v := getenv(key); v != "" {
			fields = append(fields, key, redactSettingValue(v))
		}
	}
	if getenv("GOAUTH") != "" {
		fields = append(fields, "GOAUTH", "(set)")
	}

	// The go command reads $NETRC, else $HOME/.netrc; git reads $HOME/.netrc.
	netrc := getenv("NETRC")
	if netrc == "" && getenv("HOME") != "" {
		netrc = filepath.Join(getenv("HOME"), ".netrc")
	}
	fields = append(fields, "netrc_present", netrc != "" && fileExists(netrc))
	return fields
}
