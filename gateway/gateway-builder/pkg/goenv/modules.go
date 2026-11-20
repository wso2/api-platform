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
)

// credentialsInURL matches the userinfo part of a URL ("scheme://user:token@").
var credentialsInURL = regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.\-]*://)[^/@\s]+@`)

// RedactCredentials replaces the userinfo of every URL in text with a placeholder,
// so a token embedded in a module proxy or repository URL never reaches the logs.
func RedactCredentials(text string) string {
	return credentialsInURL.ReplaceAllString(text, "${1}<redacted-credentials>@")
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
			fields = append(fields, key, RedactCredentials(v))
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
