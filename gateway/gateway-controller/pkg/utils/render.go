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

package utils

import (
	"fmt"
	"log/slog"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine/funcs"
)

// RenderAndCacheConfig renders template expressions in the spec of a StoredConfig.
// It renders cfg.Configuration in place, leaving cfg.SourceConfiguration untouched (as stored in DB).
// cfg.SensitiveValues is populated with tracked resolved secret values for redaction.
//
// Callers must ensure cfg.Configuration holds the value to render before calling. For the deployment
// flow, both Configuration and SourceConfiguration are equal at the call site. For LLM configs in the
// event listener, hydration runs first and sets Configuration to a RestAPI, which is then rendered here.
func RenderAndCacheConfig(cfg *models.StoredConfig, secretResolver funcs.SecretResolver, logger *slog.Logger) error {
	if cfg.Configuration == nil {
		return nil
	}

	renderResult, err := templateengine.RenderSpec(cfg.Configuration, secretResolver, logger)
	if err != nil {
		return fmt.Errorf("failed to render config %q (kind=%s): %w", cfg.Handle, cfg.Kind, err)
	}

	cfg.Configuration = renderResult.Config
	cfg.SensitiveValues = renderResult.Tracker.Values()
	return nil
}
