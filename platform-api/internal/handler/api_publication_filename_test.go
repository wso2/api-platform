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

package handler

import "testing"

func TestSanitizeUploadFileName(t *testing.T) {
	cases := map[string]string{
		"icon.png":        "icon.png",
		"../../icon.png":  "icon.png",
		"a/b/icon.png":    "icon.png",
		"":                "",
		".":               "",
		"..":              "",
		"/":               "",
		"../..":           "",
		"icon\x00.png":    "",
		"../x\x00/is.png": "",
	}
	for in, want := range cases {
		if got := sanitizeUploadFileName(in); got != want {
			t.Errorf("sanitizeUploadFileName(%q) = %q, want %q", in, got, want)
		}
	}
}
