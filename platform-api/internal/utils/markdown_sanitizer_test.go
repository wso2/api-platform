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

import "testing"

func TestStripEmbeddedHTML(t *testing.T) {
	cases := map[string]string{
		"# Title *bold* [link](https://example.com)":               "# Title *bold* [link](https://example.com)",
		"a <b>bold</b> word":                                       "a bold word",
		"x<script>alert(1)</script>y":                              "xy",
		"x<style>p{}</style>y":                                     "xy",
		"encoded &lt;script&gt;alert(1)&lt;/script&gt; stays text": "encoded &lt;script&gt;alert(1)&lt;/script&gt; stays text",
		"see <https://example.com> now":                            "see <https://example.com> now",
		"see <HTTP://Example.com/a?x=1&y=2#f> now":                 "see <HTTP://Example.com/a?x=1&y=2#f> now",
		"mail <user@example.com> now":                              "mail <user@example.com> now",
		"mail <mailto:user@example.com> now":                       "mail <mailto:user@example.com> now",
		"x<javascript:alert(1)>y":                                  "xy",
		"x<https://evil.com onmouseover=alert(1)>y":                "xy",
		"x<user@example.com onclick=alert(1)>y":                    "xy",
		"<script><https://example.com></script>ok":                 "ok",
		"x<script/><img src=x onerror=alert(1)>y":                  "x",
		"x<script/>a</script>y":                                    "xy",
		"x<style/><b>z</b></style>y":                               "xy",
		"a<br/>b <img src=x/>c":                                    "ab c",
	}
	for in, want := range cases {
		if got := StripEmbeddedHTML(in); got != want {
			t.Errorf("StripEmbeddedHTML(%q) = %q, want %q", in, got, want)
		}
	}
}
