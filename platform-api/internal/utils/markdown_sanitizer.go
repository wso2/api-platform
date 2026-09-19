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

	"golang.org/x/net/html"
)

// markdownAutolink matches the Markdown autolink forms <https://…>, <http://…>,
// <mailto:…> and <user@example.com>. Other schemes (javascript:, data:, …) are
// deliberately not matched, so they are stripped like any other tag.
var markdownAutolink = regexp.MustCompile(
	`(?i)^<(?:(?:https?://|mailto:)[^\s<>]+|[a-z0-9.!#$%&'*+/=?^_{|}~-]+@[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)+)>$`)

// StripEmbeddedHTML removes HTML markup embedded in Markdown text before
// storage — stricter than the portal's own handling for landing pages. This
// is defense-in-depth on top of, not a substitute for, whatever eventually
// renders this Markdown to HTML applying its own output encoding.
//
// Ordinary Markdown syntax (*bold*, # heading, [text](url), ...) passes
// through untouched — the HTML5 tokenizer only recognizes literal tag-like
// sequences, so plain text is never mistaken for markup. <script>/<style>
// element content is dropped entirely, not just the tags, so no
// still-executable-looking payload text survives; every other tag is simply
// removed, keeping its inner text. Markdown autolinks (see markdownAutolink)
// are kept.
func StripEmbeddedHTML(markdown string) string {
	if !strings.ContainsAny(markdown, "<>") {
		return markdown // fast path: nothing tag-like present at all
	}

	var b strings.Builder
	tokenizer := html.NewTokenizer(strings.NewReader(markdown))
	skipDepth := 0
	skipTag := ""
	for {
		tokenType := tokenizer.Next()
		if (tokenType == html.StartTagToken || tokenType == html.SelfClosingTagToken) && skipDepth == 0 {
			if raw := string(tokenizer.Raw()); markdownAutolink.MatchString(raw) {
				b.WriteString(raw)
				continue
			}
		}
		switch tokenType {
		case html.ErrorToken:
			return b.String()
		case html.TextToken:
			if skipDepth == 0 {
				b.Write(tokenizer.Raw())
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			// A self-closing <script/> still opens a raw-text block in HTML, so it
			// starts the skip like <script> does.
			name, _ := tokenizer.TagName()
			tag := string(name)
			if tag == "script" || tag == "style" {
				if skipDepth == 0 {
					skipTag = tag
				}
				skipDepth++
			}
		case html.EndTagToken:
			name, _ := tokenizer.TagName()
			if skipDepth > 0 && string(name) == skipTag {
				skipDepth--
			}
		default:
			// CommentToken, DoctypeToken: dropped from the output.
		}
	}
}
