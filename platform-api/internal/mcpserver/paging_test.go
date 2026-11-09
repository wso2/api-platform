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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPageIn(t *testing.T) {
	cursor := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	tests := []struct {
		name    string
		in      pageIn
		want    page
		wantErr bool
	}{
		{"defaults", pageIn{}, page{limit: defaultPageLimit}, false},
		{"limit kept", pageIn{Limit: 5}, page{limit: 5}, false},
		{"limit raised to 1", pageIn{Limit: -3}, page{limit: 1}, false},
		{"limit capped", pageIn{Limit: 1000}, page{limit: maxPageLimit}, false},
		{"cursor is the offset", pageIn{Cursor: cursor("40")}, page{limit: defaultPageLimit, offset: 40}, false},
		{"cursor not base64", pageIn{Cursor: "%%"}, page{}, true},
		{"cursor not a number", pageIn{Cursor: cursor("abc")}, page{}, true},
		{"negative offset", pageIn{Cursor: cursor("-1")}, page{}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.in.page()
			if tc.wantErr {
				assert.ErrorContains(t, err, "cursor is not valid")
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestPageNext(t *testing.T) {
	p := page{limit: 20, offset: 20}
	next, err := pageIn{Cursor: p.next(20, 45)}.page()
	assert.NoError(t, err)
	assert.Equal(t, 40, next.offset, "the next cursor starts after this page")
	assert.Empty(t, p.next(5, 25), "no cursor on the last page")
	assert.Empty(t, p.next(0, 100), "no cursor on an empty page")
}
