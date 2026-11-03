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

package mcp

import (
	"context"
	"net/http"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestAddSumsTwoNumbers(t *testing.T) {
	result, _, err := add(context.Background(), &mcpsdk.CallToolRequest{}, addInput{A: 2, B: 3})
	require.NoError(t, err)
	require.Equal(t, "The sum of 2 and 3 is 5.", textOf(t, result))
}

func TestAddFormatsFractionalAndZeroOperands(t *testing.T) {
	result, _, err := add(context.Background(), &mcpsdk.CallToolRequest{}, addInput{A: 0.5, B: 0})
	require.NoError(t, err)
	require.Equal(t, "The sum of 0.5 and 0 is 0.5.", textOf(t, result))
}

func TestEchoReturnsTheMessage(t *testing.T) {
	result, _, err := echo(context.Background(), &mcpsdk.CallToolRequest{}, echoInput{Message: "hi"})
	require.NoError(t, err)
	require.Equal(t, "Echo: hi", textOf(t, result))
}

func TestHeadersReportsThePresentHeaderValue(t *testing.T) {
	req := &mcpsdk.CallToolRequest{Extra: &mcpsdk.RequestExtra{
		Header: http.Header{"Authorization": {"Bearer abc.def.ghi"}},
	}}

	result, _, err := headers(context.Background(), req, headersInput{Name: "Authorization"})

	require.NoError(t, err)
	require.Equal(t, "Authorization: Bearer abc.def.ghi", textOf(t, result))
}

func TestHeadersIsCaseInsensitiveOnTheHeaderName(t *testing.T) {
	req := &mcpsdk.CallToolRequest{Extra: &mcpsdk.RequestExtra{
		Header: http.Header{"X-Forwarded-Authorization": {"Bearer abc.def.ghi"}},
	}}

	result, _, err := headers(context.Background(), req, headersInput{Name: "x-forwarded-authorization"})

	require.NoError(t, err)
	require.Equal(t, "x-forwarded-authorization: Bearer abc.def.ghi", textOf(t, result))
}

func TestHeadersReportsEmptyWhenTheHeaderIsAbsent(t *testing.T) {
	req := &mcpsdk.CallToolRequest{Extra: &mcpsdk.RequestExtra{Header: http.Header{}}}

	result, _, err := headers(context.Background(), req, headersInput{Name: "X-Missing"})

	require.NoError(t, err)
	require.Equal(t, "X-Missing: ", textOf(t, result))
}

func TestHeadersReportsEmptyWhenExtraIsNil(t *testing.T) {
	result, _, err := headers(context.Background(), &mcpsdk.CallToolRequest{}, headersInput{Name: "Authorization"})

	require.NoError(t, err)
	require.Equal(t, "Authorization: ", textOf(t, result))
}

func textOf(t *testing.T, result *mcpsdk.CallToolResult) string {
	t.Helper()
	require.Len(t, result.Content, 1)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok, "expected a text content block")
	return text.Text
}
