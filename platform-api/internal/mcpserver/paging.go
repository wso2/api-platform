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

package mcpserver

import (
	"encoding/base64"
	"strconv"
)

// Page sizes, the same as the Platform API's REST lists.
const (
	defaultPageLimit = 20
	maxPageLimit     = 100
)

// pageIn is embedded in every list tool's input.
type pageIn struct {
	Cursor string `json:"cursor,omitempty" jsonschema:"The nextCursor of the previous page. Leave it out for the first page."`
	Limit  int    `json:"limit,omitempty" jsonschema:"Items per page, 1 to 100. Default 20."`
}

// page is the Platform API's limit and offset. The cursor holds the offset.
type page struct{ limit, offset int }

func (in pageIn) page() (page, error) {
	p := page{limit: defaultPageLimit}
	if in.Limit != 0 {
		p.limit = min(max(in.Limit, 1), maxPageLimit)
	}
	if in.Cursor != "" {
		b, err := base64.RawURLEncoding.DecodeString(in.Cursor)
		n, err2 := strconv.Atoi(string(b))
		if err != nil || err2 != nil || n < 0 {
			return p, fail("the cursor is not valid; start again without it")
		}
		p.offset = n
	}
	return p, nil
}

// next is the cursor for the page after this one, or "" on the last page.
func (p page) next(got, total int) string {
	if got == 0 || p.offset+got >= total {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(p.offset + got)))
}
