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

package platformapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"strconv"
	"strings"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
	"github.com/wso2/api-platform/tests/framework/testbench/services/specs"
)

// Multipart field values that upload a document instead of sending text: a testbench specs
// fixture by name, or a generated OpenAPI document of a given size in bytes.
const (
	specFieldPrefix          = "@spec:"
	generatedSpecFieldPrefix = "@generated-spec:"
)

// RegisterAPIDefinition binds the steps that send OpenAPI documents to the control plane as
// multipart forms, and that address the testbench specs service.
func RegisterAPIDefinition(sc *godog.ScenarioContext, s *Steps) {
	sc.Step(`^I send a multipart "([^"]*)" request to the control plane at "([^"]*)"(?: as "([^"]*)")? with fields:$`,
		s.sendMultipartControlPlane)
	sc.Step(`^I store the (http|https) URL of spec fixture "([^"]*)" as "([^"]*)"$`, storeSpecFixtureURL)
	sc.Step(`^the JSON response field "([^"]*)" should be spec fixture "([^"]*)"$`, jsonFieldIsSpecFixture)
}

// jsonFieldIsSpecFixture asserts a response field holds a fixture byte for byte, as the control
// plane stores and returns an uploaded definition unchanged.
func jsonFieldIsSpecFixture(ctx context.Context, field, name string) error {
	want, err := specs.Fixture(name)
	if err != nil {
		return err
	}
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	var doc map[string]any
	if err := json.Unmarshal(resp.Body, &doc); err != nil {
		return fmt.Errorf("response is not a JSON object: %w (%s)", err, resp.Describe())
	}
	got, ok := doc[field].(string)
	if !ok {
		return fmt.Errorf("JSON field %q is not a string: %s", field, resp.Describe())
	}
	if got != string(want) {
		return fmt.Errorf("JSON field %q differs from spec fixture %q (%d bytes, want %d)", field, name, len(got), len(want))
	}
	return nil
}

// sendMultipartControlPlane sends a form of "field | value" rows through the request funnel, so
// a created API is registered for cleanup like any other.
func (s *Steps) sendMultipartControlPlane(ctx context.Context, method, path, who string, table *godog.Table) error {
	body, contentType, err := multipartForm(ctx, table)
	if err != nil {
		return err
	}
	_, err = s.sendControlPlane(ctx, controlPlaneCall{
		method: method, path: path, who: who, body: body,
		headers: map[string]string{"Content-Type": contentType},
	})
	return err
}

// multipartForm builds the form a table describes, in row order. A value starting with
// @spec: or @generated-spec: is uploaded as a file; any other value is a text field.
func multipartForm(ctx context.Context, table *godog.Table) ([]byte, string, error) {
	if table == nil || len(table.Rows) == 0 {
		return nil, "", fmt.Errorf("a multipart request needs at least one \"field | value\" row")
	}
	buf := &bytes.Buffer{}
	form := multipart.NewWriter(buf)
	for i, row := range table.Rows {
		if len(row.Cells) != 2 {
			return nil, "", fmt.Errorf("multipart row %d must have two cells, field and value", i+1)
		}
		field := strings.TrimSpace(row.Cells[0].Value)
		value, err := stepscommon.Expand(ctx, row.Cells[1].Value)
		if err != nil {
			return nil, "", err
		}
		name, content, isFile, err := formFile(value)
		if err != nil {
			return nil, "", fmt.Errorf("multipart field %q: %w", field, err)
		}
		if !isFile {
			if err := form.WriteField(field, value); err != nil {
				return nil, "", err
			}
			continue
		}
		part, err := form.CreateFormFile(field, name)
		if err != nil {
			return nil, "", err
		}
		if _, err := part.Write(content); err != nil {
			return nil, "", err
		}
	}
	if err := form.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), form.FormDataContentType(), nil
}

// formFile resolves a file-upload value to its file name and content.
func formFile(value string) (name string, content []byte, isFile bool, err error) {
	switch {
	case strings.HasPrefix(value, specFieldPrefix):
		name = strings.TrimPrefix(value, specFieldPrefix)
		content, err = specs.Fixture(name)
		return name, content, true, err
	case strings.HasPrefix(value, generatedSpecFieldPrefix):
		size, convErr := strconv.Atoi(strings.TrimPrefix(value, generatedSpecFieldPrefix))
		if convErr != nil {
			return "", nil, true, fmt.Errorf("%q needs a byte count", value)
		}
		content, err = specs.Generate(size)
		return "generated.yaml", content, true, err
	default:
		return "", nil, false, nil
	}
}

// storeSpecFixtureURL stores the address a container on the block network fetches a specs
// fixture from. "large?bytes=<n>" names a generated document.
func storeSpecFixtureURL(ctx context.Context, scheme, name, key string) error {
	resolved, err := stepscommon.Expand(ctx, name)
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, key, specFixtureURL(scheme, resolved))
}

func specFixtureURL(scheme, name string) string {
	port := specs.Port
	if scheme == "https" {
		port = specs.TLSPort
	}
	return fmt.Sprintf("%s://testbench:%d/specs/%s", scheme, port, name)
}
