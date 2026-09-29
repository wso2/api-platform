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

package platformgateway

import (
	"context"
	"testing"

	"github.com/cucumber/godog"
	"github.com/cucumber/messages/go/v34"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

func TestDecodeRawHeaderValue(t *testing.T) {
	tests := map[string]struct {
		in, want string
	}{
		"tabs":                {`\t\t`, "\t\t"},
		"spaces untouched":    {"   ", "   "},
		"carriage return":     {`\r\n`, "\r\n"},
		"escaped byte":        {`\x20\x20\x20`, "   "},
		"escaped byte upper":  {`\x4A`, "J"},
		"literal backslash":   {`a\\b`, `a\b`},
		"mixed":               {`abc\x01def`, "abc\x01def"},
		"crlf injection":      {`abc\r\nX-Injected: yes`, "abc\r\nX-Injected: yes"},
		"trailing backslash":  {`abc\`, `abc\`},
		"unknown escape kept": {`a\zb`, `a\zb`},
		"truncated hex kept":  {`a\x2`, `a\x2`},
		"invalid hex kept":    {`a\xZZb`, `a\xZZb`},
		"empty":               {"", ""},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tt.want, decodeRawHeaderValue(tt.in))
		})
	}
}

func TestRawHeaderLinesPreservesOrderAndDuplicates(t *testing.T) {
	table := &godog.Table{Rows: []*messages.PickleTableRow{
		{Cells: []*messages.PickleTableCell{{Value: "API-Key"}, {Value: "first"}}},
		{Cells: []*messages.PickleTableCell{{Value: "API-Key"}, {Value: "second"}}},
		{Cells: []*messages.PickleTableCell{{Value: "x-api-key"}, {Value: `\t`}}},
	}}

	headers, err := rawHeaderLines(context.Background(), table)
	require.NoError(t, err)
	require.Equal(t, []rawHeader{
		{name: "API-Key", value: "first"},
		{name: "API-Key", value: "second"},
		{name: "x-api-key", value: "\t"},
	}, headers)
}

func TestRawHeaderLinesExpandsContextValues(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("test-runner"))
	require.NoError(t, tcontext.Set(ctx, "akaKey", "secret-value"))
	table := &godog.Table{Rows: []*messages.PickleTableRow{
		{Cells: []*messages.PickleTableCell{{Value: "API-Key"}, {Value: "${CTX:akaKey}"}}},
	}}

	headers, err := rawHeaderLines(ctx, table)
	require.NoError(t, err)
	require.Equal(t, []rawHeader{{name: "API-Key", value: "secret-value"}}, headers)
}

func TestRawHeaderLinesRejectsEmptyTable(t *testing.T) {
	_, err := rawHeaderLines(context.Background(), &godog.Table{})
	require.Error(t, err)
}

func TestRawHeaderLinesRejectsMalformedRow(t *testing.T) {
	table := &godog.Table{Rows: []*messages.PickleTableRow{
		{Cells: []*messages.PickleTableCell{{Value: "API-Key"}}},
	}}
	_, err := rawHeaderLines(context.Background(), table)
	require.Error(t, err)
}

func TestRawHeaderLinesRejectsEmptyHeaderName(t *testing.T) {
	table := &godog.Table{Rows: []*messages.PickleTableRow{
		{Cells: []*messages.PickleTableCell{{Value: "  "}, {Value: "value"}}},
	}}
	_, err := rawHeaderLines(context.Background(), table)
	require.Error(t, err)
}
