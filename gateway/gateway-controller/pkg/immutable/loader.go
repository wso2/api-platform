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

package immutable

import (
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// ImmutableGW manages immutable gateway mode: artifact loading on startup and
// read-only enforcement on the management API at runtime.
// When cfg.Enabled is false, all methods are no-ops.
type ImmutableGW struct {
	cfg               config.ImmutableGatewayConfig
	deploymentService *utils.APIDeploymentService
	llmService        *utils.LLMDeploymentService
	mcpService        *utils.MCPDeploymentService
	parser            *config.Parser
}

// NewImmutableGW creates an ImmutableGW. All methods are no-ops when cfg.Enabled is false.
func NewImmutableGW(
	cfg config.ImmutableGatewayConfig,
	deploymentService *utils.APIDeploymentService,
	llmService *utils.LLMDeploymentService,
	mcpService *utils.MCPDeploymentService,
) *ImmutableGW {
	return &ImmutableGW{
		cfg:               cfg,
		deploymentService: deploymentService,
		llmService:        llmService,
		mcpService:        mcpService,
		parser:            config.NewParser(),
	}
}

// LoadArtifacts walks the configured artifacts directory and applies all YAML resources
// via the service layer in dependency order. It is a no-op when immutable mode is disabled.
// Returns an error on the first artifact that fails; the caller should treat this as fatal.
func (g *ImmutableGW) LoadArtifacts(log *slog.Logger) error {
	if !g.cfg.Enabled {
		return nil
	}

	log.Info("Scanning artifacts directory for immutable gateway", slog.String("dir", g.cfg.ArtifactsDir))

	type artifact struct {
		path string
		data []byte
		kind string
	}

	// pass1: LlmProvider — no dependencies
	// pass2: everything else (RestApi, WebSubApi, LlmProxy, Mcp) — may depend on pass1
	var pass1, pass2 []artifact

	if err := filepath.WalkDir(g.cfg.ArtifactsDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("error accessing path %s: %w", path, walkErr)
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read artifact %s: %w", path, err)
		}

		var envelope struct {
			Kind string `yaml:"kind"`
		}
		if err := g.parser.Parse(data, "application/x-yaml", &envelope); err != nil {
			return fmt.Errorf("failed to parse kind from %s: %w", path, err)
		}
		if envelope.Kind == "" {
			return fmt.Errorf("artifact %s has no 'kind' field", path)
		}

		a := artifact{path: path, data: data, kind: envelope.Kind}
		switch envelope.Kind {
		case models.KindLlmProvider:
			pass1 = append(pass1, a)
		case models.KindRestApi, models.KindWebSubApi, models.KindLlmProxy, models.KindMcp:
			pass2 = append(pass2, a)
		default:
			return fmt.Errorf("artifact %s has unsupported kind %q", path, envelope.Kind)
		}
		return nil
	}); err != nil {
		return err
	}

	total := len(pass1) + len(pass2)
	log.Info("Discovered artifacts", slog.Int("total", total), slog.Int("llm_providers", len(pass1)))

	for _, a := range append(pass1, pass2...) {
		if err := g.applyArtifact(a.path, a.kind, a.data, log); err != nil {
			return err
		}
	}

	log.Info("All immutable gateway artifacts loaded", slog.Int("count", total))
	return nil
}

func (g *ImmutableGW) applyArtifact(path, kind string, data []byte, log *slog.Logger) error {
	log.Info("Applying artifact", slog.String("path", path), slog.String("kind", kind))
	const contentType = "application/x-yaml"

	switch kind {
	case models.KindRestApi, models.KindWebSubApi:
		if _, err := g.deploymentService.DeployAPIConfiguration(utils.APIDeploymentParams{
			Data:        data,
			ContentType: contentType,
			Origin:      models.OriginGatewayAPI,
			Logger:      log,
		}); err != nil {
			return fmt.Errorf("failed to apply %s %s: %w", kind, path, err)
		}
	case models.KindLlmProvider:
		if _, err := g.llmService.CreateLLMProvider(utils.LLMDeploymentParams{
			Data:        data,
			ContentType: contentType,
			Origin:      models.OriginGatewayAPI,
			Logger:      log,
		}); err != nil {
			return fmt.Errorf("failed to apply %s %s: %w", kind, path, err)
		}
	case models.KindLlmProxy:
		if _, err := g.llmService.CreateLLMProxy(utils.LLMDeploymentParams{
			Data:        data,
			ContentType: contentType,
			Origin:      models.OriginGatewayAPI,
			Logger:      log,
		}); err != nil {
			return fmt.Errorf("failed to apply %s %s: %w", kind, path, err)
		}
	case models.KindMcp:
		if _, err := g.mcpService.CreateMCPProxy(utils.MCPDeploymentParams{
			Data:        data,
			ContentType: contentType,
			Origin:      models.OriginGatewayAPI,
			Logger:      log,
		}); err != nil {
			return fmt.Errorf("failed to apply %s %s: %w", kind, path, err)
		}
	}
	return nil
}

// Middleware returns a Gin handler that rejects POST, PUT, and DELETE with 405
// when immutable mode is enabled. When disabled, it returns a passthrough handler.
func (g *ImmutableGW) Middleware() gin.HandlerFunc {
	if !g.cfg.Enabled {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodDelete:
			c.JSON(http.StatusMethodNotAllowed, api.ErrorResponse{
				Status:  "error",
				Message: "Gateway is in immutable mode. Mutating operations are not allowed.",
			})
			c.Abort()
		default:
			c.Next()
		}
	}
}
