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

package templateengine

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

func TestRenderSpec_PostRenderCheck(t *testing.T) {
	t.Setenv("TEST_POST_RENDER_URL", "http://refused.example.com")
	t.Cleanup(func() { SetPostRenderCheck(nil) })

	errRefused := errors.New("refused")
	var gotKind string
	var gotURL string
	SetPostRenderCheck(func(kind string, rendered any) error {
		gotKind = kind
		gotURL = rendered.(testConfig).Spec.Upstream.URL
		if gotURL == "http://refused.example.com" {
			return errRefused
		}
		return nil
	})

	source := testConfig{Kind: "TestApi", Spec: testSpec{Upstream: testUpstream{URL: `{{ env "TEST_POST_RENDER_URL" }}`}}}
	cfg := &models.StoredConfig{Handle: "h", Kind: "LlmProxy", Configuration: source}
	err := RenderSpec(cfg, &mockResolver{}, nil)

	var renderErr *RenderError
	require.ErrorAs(t, err, &renderErr)
	assert.ErrorIs(t, err, errRefused)
	assert.Equal(t, "LlmProxy", gotKind, "the check receives the stored kind, not the rendered value's")
	assert.Equal(t, "http://refused.example.com", gotURL, "the check sees the rendered value")
	assert.Equal(t, source, cfg.Configuration, "a refused config is left unrendered")

	// Positive control: an accepted config is rendered in place.
	t.Setenv("TEST_POST_RENDER_URL", "http://ok.example.com")
	require.NoError(t, RenderSpec(cfg, &mockResolver{}, nil))
	assert.Equal(t, "http://ok.example.com", cfg.Configuration.(testConfig).Spec.Upstream.URL)

	// Clearing the check stops it from running.
	SetPostRenderCheck(nil)
	t.Setenv("TEST_POST_RENDER_URL", "http://refused.example.com")
	cfg.Configuration = source
	require.NoError(t, RenderSpec(cfg, &mockResolver{}, nil))
}
