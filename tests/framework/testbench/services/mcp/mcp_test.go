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

// connect opens a client session on the server every MCP service builds, over the SDK's
// in-memory transport, so each test sees exactly what a client of the testbench would.
func connect(t *testing.T) *mcpsdk.ClientSession {
	t.Helper()
	ctx := context.Background()
	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	serverSession, err := newServer().Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestInitializeAdvertisesResourcesAndPrompts(t *testing.T) {
	caps := connect(t).InitializeResult().Capabilities
	require.NotNil(t, caps.Resources, "resources capability")
	require.NotNil(t, caps.Prompts, "prompts capability")
	require.NotNil(t, caps.Tools, "tools capability")
}

func TestResourcesListServesBothFixedResources(t *testing.T) {
	result, err := connect(t).ListResources(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, result.Resources, 2)

	byURI := map[string]*mcpsdk.Resource{}
	for _, r := range result.Resources {
		byURI[r.URI] = r
	}
	readme := byURI["file:///docs/readme.txt"]
	require.NotNil(t, readme)
	require.Equal(t, "readme", readme.Name)
	require.Equal(t, "Readme", readme.Title)
	require.Equal(t, "A short project readme", readme.Description)
	require.Equal(t, "text/plain", readme.MIMEType)
	changelog := byURI["file:///docs/changelog.txt"]
	require.NotNil(t, changelog)
	require.Equal(t, "changelog", changelog.Name)
}

func TestResourceReadReturnsTheFixedText(t *testing.T) {
	session := connect(t)
	for uri, want := range map[string]string{
		"file:///docs/readme.txt":    "This is the readme.",
		"file:///docs/changelog.txt": "v1.0.0: first release.",
	} {
		result, err := session.ReadResource(context.Background(), &mcpsdk.ReadResourceParams{URI: uri})
		require.NoError(t, err, uri)
		require.Len(t, result.Contents, 1, uri)
		require.Equal(t, uri, result.Contents[0].URI)
		require.Equal(t, "text/plain", result.Contents[0].MIMEType)
		require.Equal(t, want, result.Contents[0].Text)
	}
}

func TestResourceReadOfAnUnknownURIFails(t *testing.T) {
	_, err := connect(t).ReadResource(context.Background(), &mcpsdk.ReadResourceParams{URI: "file:///docs/missing.txt"})
	require.Error(t, err)
}

func TestPromptsListServesBothPrompts(t *testing.T) {
	result, err := connect(t).ListPrompts(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, result.Prompts, 2)

	byName := map[string]*mcpsdk.Prompt{}
	for _, p := range result.Prompts {
		byName[p.Name] = p
	}
	greetingPrompt := byName["greeting"]
	require.NotNil(t, greetingPrompt)
	require.Equal(t, "Greeting", greetingPrompt.Title)
	require.Equal(t, "Greets someone by name", greetingPrompt.Description)
	require.Len(t, greetingPrompt.Arguments, 1)
	require.Equal(t, "name", greetingPrompt.Arguments[0].Name)
	require.False(t, greetingPrompt.Arguments[0].Required)
	require.NotNil(t, byName["summary"])
}

func TestGreetingPromptGreetsTheNamedPersonOrWorld(t *testing.T) {
	session := connect(t)
	for args, want := range map[string]string{
		"":    "Say hello to World.",
		"Ada": "Say hello to Ada.",
	} {
		params := &mcpsdk.GetPromptParams{Name: "greeting"}
		if args != "" {
			params.Arguments = map[string]string{"name": args}
		}
		result, err := session.GetPrompt(context.Background(), params)
		require.NoError(t, err)
		require.Equal(t, "Greets someone by name", result.Description)
		require.Equal(t, want, promptText(t, result))
	}
}

func TestSummaryPromptQuotesTheText(t *testing.T) {
	result, err := connect(t).GetPrompt(context.Background(), &mcpsdk.GetPromptParams{
		Name: "summary", Arguments: map[string]string{"text": "a long story"},
	})
	require.NoError(t, err)
	require.Equal(t, "Summarize this text: a long story", promptText(t, result))
}

func TestGetOfAnUnknownPromptFails(t *testing.T) {
	_, err := connect(t).GetPrompt(context.Background(), &mcpsdk.GetPromptParams{Name: "missing"})
	require.Error(t, err)
}

// promptText returns the text of the single user message a fixed prompt answers with.
func promptText(t *testing.T, result *mcpsdk.GetPromptResult) string {
	t.Helper()
	require.Len(t, result.Messages, 1)
	require.Equal(t, mcpsdk.Role("user"), result.Messages[0].Role)
	text, ok := result.Messages[0].Content.(*mcpsdk.TextContent)
	require.True(t, ok, "message content is %T, not text", result.Messages[0].Content)
	return text.Text
}
