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

package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"gopkg.in/yaml.v3"
)

func commonContext() context.Context {
	return tcontext.WithLocal(
		tcontext.WithShared(context.Background(), tcontext.NewShared("block")),
		tcontext.NewLocal("runner"),
	)
}

func TestGenerateAndStore(t *testing.T) {
	ctx := commonContext()
	require.NoError(t, GenerateAndStore(ctx, "My API", "apiName"))

	value, err := StoredValue(ctx, "apiName")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(value, "My_API_"))

	expanded, err := Expand(ctx, "${CTX:apiName}")
	require.NoError(t, err)
	require.Equal(t, value, expanded)
}

func TestGenerateContextAndStore(t *testing.T) {
	ctx := commonContext()
	require.NoError(t, GenerateContextAndStore(ctx, "My API", "apiContext"))
	value, err := StoredValue(ctx, "apiContext")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(value, "/my-api-"))
}

func TestGenerateAPIVersionAndStore(t *testing.T) {
	ctx := commonContext()
	require.NoError(t, GenerateAPIVersionAndStore(ctx, "health-router", "apiVersion"))
	value, err := StoredValue(ctx, "apiVersion")
	require.NoError(t, err)
	require.Regexp(t, `^v1\.0\.[1-9][0-9]{0,5}$`, value)
}

func TestGenerateAndStoreRejectsInvalidContext(t *testing.T) {
	require.ErrorContains(t, GenerateAndStore(context.Background(), "api", "name"), "runner context")
	require.ErrorContains(t, GenerateAndStore(commonContext(), "api", " "), "empty key")
	_, err := StoredValue(commonContext(), "missing")
	require.ErrorContains(t, err, "no value in context")
}

func TestExpansionResolvesScopedValuesConcurrently(t *testing.T) {
	ConfigureExpansion()
	ctx := commonContext()
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, err := Expand(ctx, `${UNIQUE:api}`)
			if err != nil {
				t.Errorf("Expand() error = %v", err)
				return
			}
			if !strings.HasPrefix(value, "api_") {
				t.Errorf("Expand() = %q, want generated api name", value)
			}
		}()
	}
	wg.Wait()
}

func TestAwaitResponse(t *testing.T) {
	t.Run("retries transient errors and accepts the matching response", func(t *testing.T) {
		var calls int
		err := AwaitResponse(context.Background(), func(context.Context) (*httpx.Response, error) {
			calls++
			if calls < 2 {
				return nil, retry.Transient(errors.New("route is warming up"))
			}
			return &httpx.Response{StatusCode: http.StatusOK}, nil
		}, func(response *httpx.Response) bool {
			return response != nil && response.StatusCode == http.StatusOK
		}, "waiting for route")
		require.NoError(t, err)
		require.Equal(t, 2, calls)
	})

	t.Run("rejects missing polling functions before starting a wait", func(t *testing.T) {
		require.ErrorContains(t, AwaitResponse(context.Background(), nil,
			func(*httpx.Response) bool { return true }, "waiting"), "attempt is required")
		require.ErrorContains(t, AwaitResponse(context.Background(), func(context.Context) (*httpx.Response, error) {
			return nil, nil
		}, nil, "waiting"), "condition is required")
	})

	t.Run("does not retry a programming error", func(t *testing.T) {
		calls := 0
		err := AwaitResponse(context.Background(), func(context.Context) (*httpx.Response, error) {
			calls++
			return nil, retryError("invalid request")
		}, func(*httpx.Response) bool { return true }, "waiting")
		require.ErrorContains(t, err, "invalid request")
		require.Equal(t, 1, calls)
	})
}

type retryError string

func (e retryError) Error() string { return string(e) }

func TestAwaitResponseHonoursCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	err := AwaitResponse(ctx, func(context.Context) (*httpx.Response, error) {
		return nil, retry.Transient(errors.New("not ready"))
	}, func(*httpx.Response) bool { return false }, "waiting")
	require.ErrorIs(t, err, context.Canceled)
	require.Less(t, time.Since(start), time.Second)
}

func TestSubstituteTemplateValues(t *testing.T) {
	table := templateTable("apiName", "${UNIQUE:api}", "context", "${CTX:apiContext}")

	got, err := substituteTemplateValues("name: ${VALUE:apiName}\ncontext: ${VALUE:context}\n", table)
	require.NoError(t, err)
	require.Equal(t, "name: ${UNIQUE:api}\ncontext: ${CTX:apiContext}\n", got)

	_, err = substituteTemplateValues("name: ${VALUE:missing}", table)
	require.ErrorContains(t, err, `no value supplied for "missing"`)
	_, err = substituteTemplateValues("name: ${VALUE:apiName", table)
	require.ErrorContains(t, err, "unterminated")
}

func TestTemplateValuesRejectsMalformedTables(t *testing.T) {
	for name, table := range map[string]*godog.Table{
		"nil":                nil,
		"empty":              {},
		"wrong column count": tableFromJSON(`{"rows":[{"cells":[{"value":"key"}]}]}`),
		"empty key":          tableFromJSON(`{"rows":[{"cells":[{"value":" "},{"value":"value"}]}]}`),
		"duplicate key": tableFromJSON(`{"rows":[
			{"cells":[{"value":"key"},{"value":"one"}]},
			{"cells":[{"value":"key"},{"value":"two"}]}
		]}`),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := templateValues(table)
			require.Error(t, err)
		})
	}
}

func TestSubstituteTemplateValuesIsSafeForConcurrentCalls(t *testing.T) {
	table := templateTable("name", "api")
	const workers = 32
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := substituteTemplateValues(fmt.Sprintf("name: ${VALUE:name}-%d", i), table)
			if err != nil {
				t.Errorf("substitution failed: %v", err)
			}
		}(i)
	}
	wg.Wait()
}

func TestRenderCanonicalTemplateSupportsOptionalTypedFields(t *testing.T) {
	table := templateTable(
		"apiVersion", "gateway.api-platform.wso2.com/v1",
		"name", "proxy",
		"displayName", "Proxy",
		"version", "v1.0",
		"context", "/proxy",
		"provider.id", "provider",
		"spec.resilience.timeout", "6s",
	)
	definition, err := renderCanonicalTemplate(context.Background(), []byte(canonicalProxyTemplate), table)
	require.NoError(t, err)

	var document map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(definition), &document))
	spec := document["spec"].(map[string]any)
	provider := spec["provider"].(map[string]any)
	resilience := spec["resilience"].(map[string]any)
	require.Equal(t, "provider", provider["id"])
	require.Equal(t, "6s", resilience["timeout"])
}

func TestRenderCanonicalTemplateOmitsUnspecifiedOptionalFields(t *testing.T) {
	table := templateTable(
		"apiVersion", "gateway.api-platform.wso2.com/v1",
		"name", "proxy",
		"displayName", "Proxy",
		"version", "v1.0",
		"context", "/proxy",
		"provider.id", "provider",
	)
	definition, err := renderCanonicalTemplate(context.Background(), []byte(canonicalProxyTemplate), table)
	require.NoError(t, err)

	var document map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(definition), &document))
	spec := document["spec"].(map[string]any)
	_, present := spec["resilience"]
	require.False(t, present)
}

func TestCanonicalRestAPITemplateRemainsValidAfterRendering(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(source), "..", "..", "resources", "templates", "rest-api.yaml")
	template, err := os.ReadFile(path)
	require.NoError(t, err)
	table := templateTable(
		"apiVersion", "gateway.api-platform.wso2.com/v1",
		"name", "api",
		"spec", `{"displayName":"API","version":"v1.0","context":"/api/$version","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test"}]}`,
	)
	definition, err := RenderResourceTemplate(context.Background(), "resources/templates/rest-api.yaml", template, table)
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(definition), &document))
}

func TestCanonicalResourceTemplates(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	require.True(t, ok)

	root := filepath.Join(filepath.Dir(source), "..", "..", "resources", "templates")
	want := map[string]string{
		"llm-provider-template.yaml": "LlmProviderTemplate",
		"llm-provider.yaml":          "LlmProvider",
		"llm-proxy.yaml":             "LlmProxy",
		"mcp.yaml":                   "Mcp",
		"rest-api.yaml":              "RestApi",
	}

	paths, err := filepath.Glob(filepath.Join(root, "*.yaml"))
	require.NoError(t, err)
	require.Len(t, paths, len(want))
	for _, path := range paths {
		name := filepath.Base(path)
		expectedKind, expected := want[name]
		require.Truef(t, expected, "unexpected canonical template %q", name)

		content, err := os.ReadFile(path)
		require.NoError(t, err)
		var document map[string]any
		require.NoError(t, yaml.Unmarshal(content, &document))
		require.Equal(t, expectedKind, document["kind"], "template %q", name)
		require.NotEmpty(t, document["apiVersion"], "template %q", name)
		require.NotEmpty(t, document["metadata"], "template %q", name)
		spec, ok := document["spec"].(map[string]any)
		require.True(t, ok, "template %q must define a spec mapping", name)
		require.NotNil(t, spec, "template %q", name)
	}
}

func templateTable(values ...string) *godog.Table {
	rows := make([][]string, 0, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		rows = append(rows, []string{values[i], values[i+1]})
	}
	return tableFromRows(rows...)
}

func tableFromRows(rows ...[]string) *godog.Table {
	payload := struct {
		Rows []struct {
			Cells []struct {
				Value string `json:"value"`
			} `json:"cells"`
		} `json:"rows"`
	}{}
	for _, row := range rows {
		encodedRow := struct {
			Cells []struct {
				Value string `json:"value"`
			} `json:"cells"`
		}{Cells: make([]struct {
			Value string `json:"value"`
		}, len(row))}
		for i, value := range row {
			encodedRow.Cells[i].Value = value
		}
		payload.Rows = append(payload.Rows, encodedRow)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		panic(fmt.Sprintf("marshal table fixture: %v", err))
	}
	return tableFromJSON(string(raw))
}

func tableFromJSON(raw string) *godog.Table {
	var table godog.Table
	if err := json.Unmarshal([]byte(raw), &table); err != nil {
		panic(fmt.Sprintf("unmarshal table fixture: %v", err))
	}
	return &table
}

const canonicalProxyTemplate = `
apiVersion: ${VALUE:apiVersion}
kind: LlmProxy
metadata:
  name: ${VALUE:name}
spec:
  displayName: ${VALUE:displayName}
  version: ${VALUE:version}
  context: ${VALUE:context}
  provider:
    id: ${VALUE:provider.id}
`
