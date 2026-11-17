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

package resolver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// locationRoute prepares a dispatch route whose requests carry the model where loc
// says, as an endpoint's inbound template declares it.
func locationRoute(t *testing.T, apiContext string, loc, stream *LLMValueLocation) (*PreparedRoute, error) {
	t.Helper()
	raw, err := json.Marshal(LLMModelDispatchConfig{
		Models:         map[string]string{"gemini-flash": "flash-model", "gpt-4o": "gpt4o-model"},
		ErrorOperation: modelsErrorOp,
		ModelLocation:  loc,
		Stream:         stream,
	})
	require.NoError(t, err)
	route := modelsRoute(raw)
	route.APIContext = apiContext
	return PrepareRoute(DefaultRegistry(), route)
}

func resolveView(t *testing.T, pr *PreparedRoute, view RequestView) Resolution {
	t.Helper()
	res, err := pr.Resolver.Resolve(context.Background(), view)
	require.NoError(t, err)
	require.Nil(t, pr.ValidateResolution(res))
	return res
}

var geminiModel = &LLMValueLocation{Location: "pathParam", Identifier: `models/([a-zA-Z0-9.\-]+)`}
var geminiStream = &LLMValueLocation{Location: "pathParam", Identifier: `:streamGenerateContent$`}

// A model carried in the path resolves at the header phase: the endpoint's
// authentication then runs before any body is buffered.
func TestLLMModelDispatch_PathLocation(t *testing.T) {
	pr, err := locationRoute(t, "/gemini", geminiModel, geminiStream)
	require.NoError(t, err)
	assert.Equal(t, RequestRequirements{Body: BodyNotRequired}, pr.Requirements, "nothing to buffer")

	res := resolveView(t, pr, RequestView{Method: "POST", Path: "/gemini/v1beta/models/gemini-flash:generateContent"})
	assert.Equal(t, flashModelKey(), res.ChainKey)
	assert.Equal(t, "gemini-flash", res.Attributes[AttrLLMModelName])
	assert.Equal(t, "false", res.Attributes[AttrLLMRequestStream])

	res = resolveView(t, pr, RequestView{Method: "POST", Path: "/gemini/v1beta/models/gemini-flash:streamGenerateContent?alt=sse"})
	assert.Equal(t, flashModelKey(), res.ChainKey)
	assert.Equal(t, "true", res.Attributes[AttrLLMRequestStream], "the method suffix is the stream signal")

	// An unknown model, no model, and a bad escape bind the error chain.
	res = resolveView(t, pr, RequestView{Method: "POST", Path: "/gemini/v1beta/models/gpt-9:generateContent"})
	assert.Equal(t, modelsErrorKey, res.ChainKey)
	assert.Equal(t, LLMDispatchModelNotFound, res.Attributes[AttrLLMDispatchError])
	res = resolveView(t, pr, RequestView{Method: "POST", Path: "/gemini/v1beta/models"})
	assert.Equal(t, LLMDispatchMissingModel, res.Attributes[AttrLLMDispatchError])
}

// The capture is URL-decoded, for a pattern that admits an escape — a Bedrock model
// id carries ':', which clients escape.
func TestLLMModelDispatch_PathCaptureIsDecoded(t *testing.T) {
	raw, err := json.Marshal(LLMModelDispatchConfig{
		Models:         map[string]string{"us.anthropic.claude:0": "claude-model"},
		ErrorOperation: modelsErrorOp,
		ModelLocation:  &LLMValueLocation{Location: "pathParam", Identifier: `model/([A-Za-z0-9.:%-]+)/`},
	})
	require.NoError(t, err)
	route := modelsRoute(raw)
	route.APIContext = "/bedrock"
	pr, err := PrepareRoute(DefaultRegistry(), route)
	require.NoError(t, err)
	res := resolveView(t, pr, RequestView{Method: "POST", Path: "/bedrock/model/us.anthropic.claude%3A0/converse"})
	assert.Equal(t, claudeModelKey, res.ChainKey)
	res = resolveView(t, pr, RequestView{Method: "POST", Path: "/bedrock/model/bad%zzescape/converse"})
	assert.Equal(t, LLMDispatchInvalidModel, res.Attributes[AttrLLMDispatchError])
}

// The API context is stripped before the pattern is matched, so a context that
// looks like the pattern is never read as the model.
func TestLLMModelDispatch_PathLocationIgnoresTheContext(t *testing.T) {
	pr, err := locationRoute(t, "/models/gpt-4o", geminiModel, geminiStream)
	require.NoError(t, err)
	res := resolveView(t, pr, RequestView{Method: "POST", Path: "/models/gpt-4o/v1beta/models/gemini-flash:generateContent"})
	assert.Equal(t, flashModelKey(), res.ChainKey)
	assert.Equal(t, "gemini-flash", res.Attributes[AttrLLMModelName])
}

func TestLLMModelDispatch_HeaderAndQueryLocations(t *testing.T) {
	pr, err := locationRoute(t, "/wso2/v1", &LLMValueLocation{Location: "header", Identifier: "X-Model"}, nil)
	require.NoError(t, err)
	assert.Equal(t, RequestRequirements{Body: BodyNotRequired}, pr.Requirements)
	res := resolveView(t, pr, RequestView{Method: "POST", Path: "/wso2/v1/chat/completions",
		Headers: map[string][]string{"x-model": {"gpt-4o"}}})
	assert.Equal(t, gpt4oModelKey, res.ChainKey)
	res = resolveView(t, pr, RequestView{Method: "POST", Path: "/wso2/v1/chat/completions",
		Headers: map[string][]string{"x-model": {"gpt-4o", "gemini-flash"}}})
	assert.Equal(t, LLMDispatchInvalidModel, res.Attributes[AttrLLMDispatchError], "a repeated header is ambiguous")
	res = resolveView(t, pr, RequestView{Method: "POST", Path: "/wso2/v1/chat/completions"})
	assert.Equal(t, LLMDispatchMissingModel, res.Attributes[AttrLLMDispatchError])

	pr, err = locationRoute(t, "/wso2/v1", &LLMValueLocation{Location: "queryParam", Identifier: "model"}, nil)
	require.NoError(t, err)
	res = resolveView(t, pr, RequestView{Method: "POST", Path: "/wso2/v1/chat/completions?model=gpt%2D4o"})
	assert.Equal(t, gpt4oModelKey, res.ChainKey, "the value is URL-decoded")
	res = resolveView(t, pr, RequestView{Method: "POST", Path: "/wso2/v1/chat/completions?model=a&model=b"})
	assert.Equal(t, LLMDispatchInvalidModel, res.Attributes[AttrLLMDispatchError])
}

// A payload location names a top-level member; the stream signal follows the
// format.
func TestLLMModelDispatch_PayloadLocationMember(t *testing.T) {
	pr, err := locationRoute(t, "/wso2/v1", &LLMValueLocation{Location: "payload", Identifier: "$.model"},
		&LLMValueLocation{Location: "payload", Identifier: "$.stream"})
	require.NoError(t, err)
	assert.Equal(t, RequestRequirements{Body: BodyBuffered}, pr.Requirements, "the model is in the body")
	res := resolveView(t, pr, RequestView{Method: "POST", Body: []byte(`{"model":"gpt-4o","stream":true}`)})
	assert.Equal(t, gpt4oModelKey, res.ChainKey)
	assert.Equal(t, "true", res.Attributes[AttrLLMRequestStream])
}

func TestLLMModelDispatchPrepare_RejectsUnusableLocations(t *testing.T) {
	for name, loc := range map[string]*LLMValueLocation{
		"nested payload":        {Location: "payload", Identifier: "$.request.model"},
		"array payload":         {Location: "payload", Identifier: "$.models[0]"},
		"pattern with no group": {Location: "pathParam", Identifier: `models/[a-z]+`},
		"pattern with two":      {Location: "pathParam", Identifier: `(models)/([a-z]+)`},
		"bad pattern":           {Location: "pathParam", Identifier: `models/(`},
		"bad header":            {Location: "header", Identifier: "x model"},
		"bad query":             {Location: "queryParam", Identifier: "a&b"},
		"unknown location":      {Location: "cookie", Identifier: "m"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := locationRoute(t, "/wso2/v1", loc, nil)
			require.Error(t, err, "a location the controller generated wrongly drops the route")
		})
	}
}

func flashModelKey() string {
	return ChainKeyFor(modelsAPIID, modelsVhost, "flash-model")
}
