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

package models

import (
	"fmt"
	"strings"
)

// defaultDataVersion is used when the YAML apiVersion is missing or unparseable.
const defaultDataVersion = "1.0"

// dataMinorVersions holds the current data-shape minor version for each kind.
//
// BUMP the value here when you make a backward-compatible change to a kind's
// stored data shape WITHOUT bumping the YAML apiVersion. The major version is
// taken from the apiVersion (".../v1" -> "1"); the minor below is appended,
// yielding a stored data_version of "1.<minor>". This lets stored rows record
// exactly which data shape they were written with, enabling future data
// migrations.
//
// Keep this map exhaustive over the ArtifactKind constants — a missing entry
// silently falls back to minor 0 (see TestDataMinorVersionsExhaustive).
var dataMinorVersions = map[ArtifactKind]int{
	KindRestApi:      0,
	KindWebSubApi:    0,
	KindWebBrokerApi: 0,
	KindMcp:          0,
	KindLlmProxy:     0,
	KindLlmProvider:  0,
}

// majorFromApiVersion parses "<group>/v<N>..." and returns "N" (the leading
// numeric run after "/v"). Any alpha/beta qualifier is stripped, so
// "gateway.api-platform.wso2.com/v1alpha2" -> "1". Returns "" if unparseable.
func majorFromApiVersion(apiVersion string) string {
	idx := strings.LastIndex(apiVersion, "/v")
	if idx == -1 {
		return ""
	}
	suffix := apiVersion[idx+2:] // text after "/v"
	major := suffix
	for i, r := range suffix {
		if r < '0' || r > '9' {
			major = suffix[:i]
			break
		}
	}
	if major == "" {
		return ""
	}
	return major
}

// ComputeDataVersion derives the stored data_version "<major>.<minor>" from the
// YAML apiVersion (major) and the per-kind data-shape minor constant. It falls
// back to defaultDataVersion ("1.0") when the apiVersion is missing or
// unparseable.
func ComputeDataVersion(kind ArtifactKind, apiVersion string) string {
	major := majorFromApiVersion(apiVersion)
	if major == "" {
		return defaultDataVersion
	}
	minor := dataMinorVersions[kind] // missing kind -> 0 (guarded by exhaustiveness test)
	return fmt.Sprintf("%s.%d", major, minor)
}
